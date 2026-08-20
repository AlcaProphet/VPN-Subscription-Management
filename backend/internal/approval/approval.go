// Package approval 提供审批中心业务层（Build3 Step 2）：待审批列表、通过/拒绝/批量通过与通知邮件。
// 设计要点：通过=激活+清 claims；拒绝=删号（邮箱释放）；邮件失败不阻断主流程（Design1 §3.4.6/4.6）。
package approval

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"vpn-sub/internal/config"
	"vpn-sub/internal/store"
	"vpn-sub/internal/xray"
)

// MailSender 邮件发送接口（mail.Service 实现；测试注入 mock）
type MailSender interface {
	SendWelcome(ctx context.Context, to, siteName, loginURL, source string) error
	SendApprovalNotify(ctx context.Context, to, siteName string, approved bool) error
}

// 业务错误
var (
	ErrNotFound = errors.New("待审批记录不存在")
)

// Service 审批服务
type Service struct {
	store *store.Store
	mail  MailSender
	cfg   *config.Service
	log   *slog.Logger

	onApproved func(ctx context.Context, userID int64)
	onRejected func(ctx context.Context, userID int64)

	onUserDeleting func(ctx context.Context, userID int64) ([]xray.Target, error)
	onUserDeleted  func(ctx context.Context, userID int64, targets []xray.Target)
}

func NewService(st *store.Store, mail MailSender, cfg *config.Service, lg *slog.Logger) *Service {
	return &Service{store: st, mail: mail, cfg: cfg, log: lg}
}

// SetOnApproved 注入审批通过后的 Xray 同步回调（Build6 Step3）。
func (s *Service) SetOnApproved(fn func(ctx context.Context, userID int64)) {
	s.onApproved = fn
}

// SetOnRejected 注入审批拒绝后的 Xray 清理回调（Build6 Step3）。
func (s *Service) SetOnRejected(fn func(ctx context.Context, userID int64)) {
	s.onRejected = fn
}

// SetOnUserDeleting 注入拒绝删除前收集 Xray 清理目标回调（Build6-2 补强）。
func (s *Service) SetOnUserDeleting(fn func(ctx context.Context, userID int64) ([]xray.Target, error)) {
	s.onUserDeleting = fn
}

// SetOnUserDeleted 注入拒绝删除后 Xray 清理回调（Build6-2 补强）。
func (s *Service) SetOnUserDeleted(fn func(ctx context.Context, userID int64, targets []xray.Target)) {
	s.onUserDeleted = fn
}

// PendingUser 待审批用户
type PendingUser struct {
	ID         int64     `json:"id"`
	Username   string    `json:"username"`
	Email      string    `json:"email"`       // 空串 = 无邮箱
	Source     string    `json:"source"`      // oidc/selfreg
	OidcClaims string    `json:"oidc_claims"` // JSON 快照（可空）
	CreatedAt  time.Time `json:"created_at"`  // UTC
}

// List 待审批列表（后端分页，默认 20 条/页）
func (s *Service) List(ctx context.Context, page, size int) ([]PendingUser, int64, error) {
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	if page <= 0 {
		page = 1
	}
	var total int64
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE status = 'pending'`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计待审批用户数失败: %w", err)
	}
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT id, username, COALESCE(email,''), user_source, COALESCE(oidc_claims,''), created_at
		 FROM users WHERE status = 'pending' ORDER BY created_at LIMIT ? OFFSET ?`,
		size, (page-1)*size)
	if err != nil {
		return nil, 0, fmt.Errorf("查询待审批用户失败: %w", err)
	}
	defer rows.Close()
	out := make([]PendingUser, 0) // 空列表返回 [] 而非 null
	for rows.Next() {
		var u PendingUser
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.Source, &u.OidcClaims, &u.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("解析待审批用户失败: %w", err)
		}
		out = append(out, u)
	}
	return out, total, rows.Err()
}

// siteContext 站点名称与登录链接（邮件模板用）
func (s *Service) siteContext(ctx context.Context) (siteName, loginURL string) {
	siteName, _ = s.cfg.Get(ctx, "site_name")
	loginURL, _ = s.cfg.Get(ctx, config.KeyFrontendURL)
	return
}

// Approve 通过：激活账号（status→active）+ 清空 oidc_claims；发欢迎邮件（按来源区分文案，失败不阻断）
func (s *Service) Approve(ctx context.Context, id int64) error {
	var email, source string
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(email,''), user_source FROM users WHERE id = ? AND status = 'pending'`, id).
			Scan(&email, &source); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE users SET status = 'active', oidc_claims = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	// 欢迎邮件（事务提交后发送，失败不阻断——记 warn 日志，Design1 §4.6）
	if email != "" {
		siteName, loginURL := s.siteContext(ctx)
		if err := s.mail.SendWelcome(ctx, email, siteName, loginURL, source); err != nil {
			s.log.Warn("欢迎邮件发送失败", "user_id", id, "err", err)
		}
	}
	if s.onApproved != nil {
		s.onApproved(ctx, id)
	}
	s.log.Info("审批通过", "user_id", id)
	return nil
}

// Reject 拒绝：删除账号（邮箱释放可重新注册）；claims 随账号删除；拒绝通知在动作时触发发送
func (s *Service) Reject(ctx context.Context, id int64) error {
	var cleanupTargets []xray.Target
	if s.onUserDeleting != nil {
		var err error
		cleanupTargets, err = s.onUserDeleting(ctx, id)
		if err != nil {
			return err
		}
	}
	var email string
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(email,'') FROM users WHERE id = ? AND status = 'pending'`, id).Scan(&email); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id); err != nil { // 账号删除、邮箱释放
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if email != "" {
		siteName, _ := s.siteContext(ctx)
		if err := s.mail.SendApprovalNotify(ctx, email, siteName, false); err != nil {
			s.log.Warn("拒绝通知邮件发送失败", "user_id", id, "err", err) // 不阻断
		}
	}
	if s.onRejected != nil {
		s.onRejected(ctx, id)
	}
	if s.onUserDeleted != nil && len(cleanupTargets) > 0 {
		s.onUserDeleted(ctx, id, cleanupTargets)
	}
	s.log.Info("审批拒绝", "user_id", id)
	return nil
}

// BatchApprove 批量通过（逐个走 Approve 语义；单个失败不阻断其余，回执成功/失败计数）
func (s *Service) BatchApprove(ctx context.Context, ids []int64) (succeeded, failed int, err error) {
	for _, id := range ids {
		if err := s.Approve(ctx, id); err != nil {
			failed++
			s.log.Warn("批量审批单项失败", "user_id", id, "err", err)
		} else {
			succeeded++
		}
	}
	return succeeded, failed, nil
}
