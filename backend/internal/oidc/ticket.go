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

	"vpn-sub/internal/auth"
)

const loginTicketTTL = 60 * time.Second

// ErrLoginTicketInvalid 表示 ticket 缺失、已消费、过期或无效；接入层映射 401。
var ErrLoginTicketInvalid = errors.New("OIDC 登录 ticket 无效或已过期")

// insertLoginTicketTx 在调用方事务内清理过期 ticket 并写入带流程指纹的一次性 ticket。
func (s *Service) insertLoginTicketTx(ctx context.Context, tx *sql.Tx, sessionToken, flowHash string) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成 OIDC ticket 失败: %w", err)
	}
	ticket := base64.RawURLEncoding.EncodeToString(buf)
	if _, err := tx.ExecContext(ctx, `DELETE FROM oidc_login_tickets WHERE expires_at < ?`, time.Now()); err != nil {
		return "", fmt.Errorf("清理过期 OIDC ticket 失败: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO oidc_login_tickets (ticket, session_token, expires_at, flow_hash) VALUES (?,?,?,?)`,
		ticket, sessionToken, time.Now().Add(loginTicketTTL), flowHash); err != nil {
		return "", fmt.Errorf("写入 OIDC ticket 失败: %w", err)
	}
	return ticket, nil
}

// IssueLoginTicket 按调用时刻的当前流程签发 ticket（测试/兼容入口）。
// 生产 OIDC 回调必须使用 IssueLoginSessionForFlow，确保 session 签发与发起时固定流程一致。
func (s *Service) IssueLoginTicket(ctx context.Context, sessionToken string) (string, error) {
	if sessionToken == "" {
		return "", errors.New("会话凭据为空")
	}
	flowHash, err := s.currentFlowHash(ctx)
	if err != nil {
		return "", err
	}
	return s.IssueLoginTicketForFlow(ctx, sessionToken, flowHash)
}

// IssueLoginTicketForFlow 在单个写事务内校验启用状态与固定流程指纹后写入 ticket。
func (s *Service) IssueLoginTicketForFlow(ctx context.Context, sessionToken, flowHash string) (string, error) {
	if sessionToken == "" {
		return "", errors.New("会话凭据为空")
	}
	if flowHash == "" {
		return "", ErrOidcFlowInvalid
	}
	var ticket string
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if err := s.assertFlowCurrentTx(ctx, tx, flowHash); err != nil {
			return err
		}
		var err error
		ticket, err = s.insertLoginTicketTx(ctx, tx, sessionToken, flowHash)
		return err
	})
	if err != nil {
		return "", err
	}
	return ticket, nil
}

// IssueLoginSessionForFlow 在同一个写事务内完成：校验启用/流程指纹 → 签发 OIDC 会话 → 写入 ticket。
// 停用在检查之后提交、或快速重新启用时，均不会得到可用 ticket 或会话 token。
func (s *Service) IssueLoginSessionForFlow(ctx context.Context, userID int64, credVersion int, flowHash string) (string, time.Time, error) {
	if flowHash == "" {
		return "", time.Time{}, ErrOidcFlowInvalid
	}
	var ticket string
	var sessionToken string
	var expiresAt time.Time
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if err := s.assertFlowCurrentTx(ctx, tx, flowHash); err != nil {
			return err
		}
		token, exp, err := s.authSvc.IssueTx(ctx, tx, userID, credVersion, auth.OidcSession)
		if err != nil {
			return err
		}
		sessionToken = token
		expiresAt = exp
		var ierr error
		ticket, ierr = s.insertLoginTicketTx(ctx, tx, sessionToken, flowHash)
		return ierr
	})
	if err != nil {
		return "", time.Time{}, err
	}
	return ticket, expiresAt, nil
}

// IssueDirectSessionForFlow Dev mock 直连登录：在单个写事务内校验启用/流程指纹并签发会话。
func (s *Service) IssueDirectSessionForFlow(ctx context.Context, userID int64, credVersion int, flowHash string) (string, time.Time, error) {
	if flowHash == "" {
		return "", time.Time{}, ErrOidcFlowInvalid
	}
	var token string
	var expiresAt time.Time
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if err := s.assertFlowCurrentTx(ctx, tx, flowHash); err != nil {
			return err
		}
		var err error
		token, expiresAt, err = s.authSvc.IssueTx(ctx, tx, userID, credVersion, auth.OidcSession)
		return err
	})
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

// ConsumeLoginTicket 严格一次性读取并删除 ticket；过期记录清理失败或已消费记录删除失败均返回错误，
// 不把数据库失败静默当作 ticket 无效。R31-07：同时校验当前启用状态与 ticket 固定的完整流程指纹，
// 旧 Dev mock ticket、停用/清空后重新启用前的 ticket 均按无效处理并清理。
func (s *Service) ConsumeLoginTicket(ctx context.Context, ticket string) (string, error) {
	if ticket == "" {
		return "", ErrLoginTicketInvalid
	}
	var sessionToken string
	found := false
	expired := false
	invalid := false
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var expiresAt time.Time
		var flowHash string
		if err := tx.QueryRowContext(ctx,
			`SELECT session_token, expires_at, flow_hash FROM oidc_login_tickets WHERE ticket = ?`, ticket).
			Scan(&sessionToken, &expiresAt, &flowHash); err != nil {
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
		if err := s.assertFlowCurrentTx(ctx, tx, flowHash); err != nil {
			if errors.Is(err, ErrOidcDisabled) || errors.Is(err, ErrOidcFlowInvalid) {
				invalid = true
				if _, derr := tx.ExecContext(ctx, `DELETE FROM oidc_login_tickets WHERE ticket = ?`, ticket); derr != nil {
					return fmt.Errorf("清理失效 OIDC ticket 失败: %w", derr)
				}
				return nil // 提交失效清理，事务外统一返回无效
			}
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM oidc_login_tickets WHERE ticket = ?`, ticket); err != nil {
			return fmt.Errorf("删除已消费 OIDC ticket 失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if expired || invalid || !found {
		return "", ErrLoginTicketInvalid
	}
	return sessionToken, nil
}
