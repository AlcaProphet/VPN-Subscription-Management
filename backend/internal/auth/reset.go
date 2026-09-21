// auth/reset.go：密码重置服务（一次性令牌、1 小时 TTL、用后标记已使用、递增凭据版本号）。
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

	"vpn-sub/internal/mail"
	"vpn-sub/internal/store"
)

var (
	ErrTokenInvalid = errors.New("重置链接无效或已过期")
	ErrBadRequest   = errors.New("参数错误")
)

const resetTokenTTL = time.Hour // 一次性、1 小时 TTL（关键设计参数，Design1 §4.6）

// ResetTokenStatus 重置链接状态（供 /reset 页面初始化判定，不消费 token）。
type ResetTokenStatus string

const (
	ResetTokenMissing ResetTokenStatus = "missing"
	ResetTokenUsed    ResetTokenStatus = "used"
	ResetTokenExpired ResetTokenStatus = "expired"
	ResetTokenValid   ResetTokenStatus = "valid"
)

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

// ResetMailer 密码重置邮件统一可用性与派发接口（mail.Dispatcher 实现；测试注入 fake）。
type ResetMailer interface {
	CheckAvailability(ctx context.Context, scope string) (mail.Availability, error)
	DispatchPasswordReset(ctx context.Context, userID int64, to, resetURL, source string) mail.DispatchResult
}

// ResetService 密码重置服务
type ResetService struct {
	store  *store.Store
	users  ResetUserSource
	log    *slog.Logger
	mailer ResetMailer
}

func NewResetService(st *store.Store, users ResetUserSource, lg *slog.Logger) *ResetService {
	return &ResetService{store: st, users: users, log: lg}
}

// SetMailer 注入统一邮件派发器；server.New 在提供请求前完成注入。
func (s *ResetService) SetMailer(m ResetMailer) {
	s.mailer = m
}

// PasswordResetAvailable 统一密码重置可用性查询：SMTP 完整配置且 password_reset scope 启用。
func (s *ResetService) PasswordResetAvailable(ctx context.Context) (bool, error) {
	if s.mailer == nil {
		return false, errors.New("密码重置邮件派发器未装配")
	}
	avail, err := s.mailer.CheckAvailability(ctx, mail.ScopePasswordReset)
	if err != nil {
		return false, err
	}
	return avail.Available, nil
}

// Request 公共忘记密码：邮箱不存在/无本地密码/邮件不可用/入队拒绝均返回统一 nil；
// 仅随机数、token 写库、派发前严格配置读取等内部故障返回 error（接入层 500）。
func (s *ResetService) Request(ctx context.Context, emailRaw string) error {
	email, err := NormalizeEmail(emailRaw)
	if err != nil {
		return nil // 格式非法也归入统一响应，不泄露信息
	}
	u, err := s.users.FindForReset(ctx, email)
	if err != nil {
		return err
	}
	if u == nil || !u.HasPassword {
		return nil
	}
	res, err := s.issue(ctx, u.ID, email, mail.SourcePublicForgot)
	if err != nil {
		return err
	}
	// 可用性预检成功后仍发生严格配置读取失败：属于内部故障，按用户决策返回 500。
	if res.Status == mail.DispatchRejected && res.Reason == mail.ReasonConfigReadFailed {
		return errors.New("密码重置邮件派发前读取配置失败")
	}
	return nil
}

// IssueForUser 管理员为指定用户生成一次性重置令牌并提交派发（Design1 §3.4.5）。
// 返回 queued/skipped/rejected 封闭结果；error 仅表示随机数/token 写库等准备失败。
func (s *ResetService) IssueForUser(ctx context.Context, userID int64, email, source string) (mail.DispatchResult, error) {
	return s.issue(ctx, userID, email, source)
}

// issue 统一生成 token + 派发 + 精确补偿：写库失败不创建任务；非 queued 精确删除本次 token。
func (s *ResetService) issue(ctx context.Context, userID int64, email, source string) (mail.DispatchResult, error) {
	if s.mailer == nil {
		return mail.DispatchResult{}, errors.New("密码重置邮件派发器未装配")
	}
	avail, err := s.mailer.CheckAvailability(ctx, mail.ScopePasswordReset)
	if err != nil {
		return mail.DispatchResult{}, err
	}
	if !avail.Available {
		return mail.DispatchResult{Status: mail.DispatchSkipped, Reason: avail.Reason}, nil
	}
	token, err := s.createToken(ctx, userID)
	if err != nil {
		return mail.DispatchResult{}, err
	}
	res := s.mailer.DispatchPasswordReset(ctx, userID, email, resetLink(token), source)
	if res.Status != mail.DispatchQueued {
		if delErr := s.deleteToken(ctx, token); delErr != nil {
			s.log.Warn("重置令牌补偿删除失败", "user_id", userID, "err", delErr)
		}
	}
	return res, nil
}

// createToken 生成 256 位一次性 token 并写入 1 小时 TTL。
func (s *ResetService) createToken(ctx context.Context, userID int64) (string, error) {
	buf := make([]byte, 32) // 256 位 ≥ 128 位熵（Design1 §4.2）
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成重置令牌失败: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	if _, err := s.store.DB().ExecContext(ctx,
		`INSERT INTO password_reset_tokens (token, user_id, expires_at) VALUES (?,?,?)`,
		token, userID, time.Now().Add(resetTokenTTL)); err != nil {
		return "", fmt.Errorf("写入重置令牌失败: %w", err)
	}
	return token, nil
}

// deleteToken 按本次 token 精确 best-effort 删除，绝不按 user_id 宽泛删除历史令牌。
func (s *ResetService) deleteToken(ctx context.Context, token string) error {
	_, err := s.store.DB().ExecContext(ctx, `DELETE FROM password_reset_tokens WHERE token = ?`, token)
	if err != nil {
		return fmt.Errorf("删除重置令牌失败: %w", err)
	}
	return nil
}

// resetLink 构造重置链接（新格式使用 URL fragment，避免 token 进入访问日志/Referer）
func resetLink(token string) string {
	return "/reset#token=" + url.QueryEscape(token)
}

// Validate 只读校验重置链接状态，不消费、不删除 token；供 /reset 页面初始化判定。
func (s *ResetService) Validate(ctx context.Context, token string) (ResetTokenStatus, error) {
	var expiresAt time.Time
	var used int
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT expires_at, used FROM password_reset_tokens WHERE token = ?`, token).
		Scan(&expiresAt, &used)
	if errors.Is(err, sql.ErrNoRows) {
		return ResetTokenMissing, nil
	}
	if err != nil {
		return "", err
	}
	if used == 1 {
		return ResetTokenUsed, nil
	}
	if time.Now().After(expiresAt) {
		return ResetTokenExpired, nil
	}
	return ResetTokenValid, nil
}

// Complete 校验令牌（存在 + 未过期 + 未使用）→ 设新密码 → 标记已使用 → 递增 credential_version
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
		if _, err := tx.ExecContext(ctx,
			`UPDATE password_reset_tokens SET used = 1 WHERE token = ?`, token); err != nil { // 用后标记已使用，保留记录供状态判定与清理
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
