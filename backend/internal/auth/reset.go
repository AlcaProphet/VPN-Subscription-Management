// auth/reset.go：密码重置服务（一次性令牌、1 小时 TTL、用后即删、递增凭据版本号）。
package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"vpn-sub/internal/store"
)

var (
	ErrTokenInvalid = errors.New("重置链接无效或已过期")
	ErrBadRequest   = errors.New("参数错误")
)

const resetTokenTTL = time.Hour // 一次性、1 小时 TTL（关键设计参数，Design1 §4.6）

// ResetTarget 密码重置所需的用户最小信息（由 user 包实现来源接口注入，避免循环依赖）
type ResetTarget struct {
	ID          int64
	Email       string
	HasPassword bool
}

// ResetUserSource 用户来源接口（user 包实现）
type ResetUserSource interface {
	FindForReset(ctx context.Context, email string) (*ResetTarget, error)
}

// ResetService 密码重置服务
type ResetService struct {
	store *store.Store
	users ResetUserSource
	log   *slog.Logger
	// sendMail 预留：Build3 SMTP 接通前以日志记录代替（标注 Build3 接通）
	sendMail func(ctx context.Context, to, resetURL string) error
}

func NewResetService(st *store.Store, users ResetUserSource, lg *slog.Logger) *ResetService {
	return &ResetService{store: st, users: users, log: lg}
}

// SetSendMail 注入邮件发送函数（Build3 SMTP 接通时调用）
func (s *ResetService) SetSendMail(fn func(ctx context.Context, to, resetURL string) error) {
	s.sendMail = fn
}

// Request 生成一次性重置令牌；无论邮箱是否存在均返回统一提示（防枚举）
func (s *ResetService) Request(ctx context.Context, emailRaw string) error {
	email, err := NormalizeEmail(emailRaw)
	if err != nil {
		return nil // 格式非法也归入统一响应，不泄露信息
	}
	u, err := s.users.FindForReset(ctx, email)
	if err != nil {
		return err
	}
	if u != nil && u.HasPassword {
		buf := make([]byte, 32) // 256 位 ≥ 128 位熵（Design1 §4.2）
		if _, err := rand.Read(buf); err != nil {
			return fmt.Errorf("生成重置令牌失败: %w", err)
		}
		token := base64.RawURLEncoding.EncodeToString(buf)
		if _, err := s.store.DB().ExecContext(ctx,
			`INSERT INTO password_reset_tokens (token, user_id, expires_at) VALUES (?,?,?)`,
			token, u.ID, time.Now().Add(resetTokenTTL)); err != nil {
			return fmt.Errorf("写入重置令牌失败: %w", err)
		}
		// 已配置 SMTP 时发送（Build3 接通）；未配置 → 日志记录并提示联系管理员
		if s.sendMail != nil {
			if err := s.sendMail(ctx, email, resetLink(token)); err != nil {
				s.log.Warn("重置邮件发送失败", "err", err) // 不阻断主流程
			}
		} else {
			s.log.Info("重置令牌已生成（SMTP 未接通，Build3 替换）", "user_id", u.ID)
		}
	}
	return nil // 接入层统一返回「若该邮箱已注册，重置链接已发送」
}

// IssueForUser 管理员为指定用户生成一次性重置令牌并发送（Build3 用户管理调用，Design1 §3.4.5）：
// 令牌 1 小时 TTL、用后即删；邮件发送失败不阻断（sendMail 未注入时以日志记录代替，Step 2 接通）
func (s *ResetService) IssueForUser(ctx context.Context, userID int64, email string) error {
	buf := make([]byte, 32) // 256 位 ≥ 128 位熵（Design1 §4.2）
	if _, err := rand.Read(buf); err != nil {
		return fmt.Errorf("生成重置令牌失败: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	if _, err := s.store.DB().ExecContext(ctx,
		`INSERT INTO password_reset_tokens (token, user_id, expires_at) VALUES (?,?,?)`,
		token, userID, time.Now().Add(resetTokenTTL)); err != nil {
		return fmt.Errorf("写入重置令牌失败: %w", err)
	}
	if s.sendMail != nil {
		if err := s.sendMail(ctx, email, resetLink(token)); err != nil {
			s.log.Warn("重置邮件发送失败", "user_id", userID, "err", err) // 不阻断主流程
		}
	} else {
		s.log.Info("重置令牌已生成（SMTP 未接通，Build3 Step 2 替换）", "user_id", userID)
	}
	return nil
}

// resetLink 构造重置链接（新格式使用 URL fragment，避免 token 进入访问日志/Referer）
func resetLink(token string) string {
	return "/reset#token=" + url.QueryEscape(token)
}

// Complete 校验令牌（存在 + 未过期 + 未使用）→ 设新密码 → 用后即删 → 递增 credential_version
func (s *ResetService) Complete(ctx context.Context, token, newPassword string) error {
	if err := ValidatePassword(newPassword); err != nil {
		return fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error { // 先读后写：IMMEDIATE 防并发双消费
		var userID int64
		var expiresAt time.Time
		var used int
		err := tx.QueryRowContext(ctx,
			`SELECT user_id, expires_at, used FROM password_reset_tokens WHERE token = ?`, token).
			Scan(&userID, &expiresAt, &used)
		if errors.Is(err, sql.ErrNoRows) || used == 1 || time.Now().After(expiresAt) {
			return ErrTokenInvalid // 统一返回「重置链接无效或已过期」
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM password_reset_tokens WHERE token = ?`, token); err != nil { // 用后即删
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE users SET password_hash = ?, credential_version = credential_version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			hash, userID); err != nil { // 递增凭据版本号：全部现有会话立即失效
			return err
		}
		return nil
	})
}
