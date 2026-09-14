// ticket.go：OIDC 登录换票 ticket 的创建、一次性消费与过期清理（业务层）。
// 原 server/oidc.go 中的直接事务已迁入本服务，Handler 只负责 Cookie 与响应映射。
package oidc

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

const loginTicketTTL = 60 * time.Second

// ErrLoginTicketInvalid 表示 ticket 缺失、已消费、过期或无效；接入层映射 401。
var ErrLoginTicketInvalid = errors.New("OIDC 登录 ticket 无效或已过期")

// IssueLoginTicket 生成一次性 60 秒 ticket，并在同事务清理过期记录。
func (s *Service) IssueLoginTicket(ctx context.Context, sessionToken string) (string, error) {
	if sessionToken == "" {
		return "", errors.New("会话凭据为空")
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成 OIDC ticket 失败: %w", err)
	}
	ticket := base64.RawURLEncoding.EncodeToString(buf)
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM oidc_login_tickets WHERE expires_at < ?`, time.Now()); err != nil {
			return fmt.Errorf("清理过期 OIDC ticket 失败: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO oidc_login_tickets (ticket, session_token, expires_at) VALUES (?,?,?)`,
			ticket, sessionToken, time.Now().Add(loginTicketTTL)); err != nil {
			return fmt.Errorf("写入 OIDC ticket 失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return ticket, nil
}

// ConsumeLoginTicket 严格一次性读取并删除 ticket；过期记录清理失败或已消费记录删除失败均返回错误，
// 不把数据库失败静默当作 ticket 无效。
func (s *Service) ConsumeLoginTicket(ctx context.Context, ticket string) (string, error) {
	if ticket == "" {
		return "", ErrLoginTicketInvalid
	}
	var sessionToken string
	found := false
	expired := false
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var expiresAt time.Time
		if err := tx.QueryRowContext(ctx,
			`SELECT session_token, expires_at FROM oidc_login_tickets WHERE ticket = ?`, ticket).
			Scan(&sessionToken, &expiresAt); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrLoginTicketInvalid
			}
			return err
		}
		found = true
		if time.Now().After(expiresAt) {
			expired = true
			if _, err := tx.ExecContext(ctx, `DELETE FROM oidc_login_tickets WHERE ticket = ?`, ticket); err != nil {
				return fmt.Errorf("清理过期 OIDC ticket 失败: %w", err)
			}
			return nil // 提交过期清理，事务外再返回无效错误
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM oidc_login_tickets WHERE ticket = ?`, ticket); err != nil {
			return fmt.Errorf("删除已消费 OIDC ticket 失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if expired || !found {
		return "", ErrLoginTicketInvalid
	}
	return sessionToken, nil
}
