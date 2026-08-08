// user/admin.go：管理员用户管理服务（Build3 Step 1）——用户全生命周期操作与五重管理员保护（Design1 §2.5/3.4.5）。
package user

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/config"
	"vpn-sub/internal/store"
	"vpn-sub/internal/token"
	"vpn-sub/internal/version"
)

// 业务错误（接入层映射 HTTP 状态码）
var (
	ErrSelfOperation     = errors.New("不能对自己执行此操作")
	ErrLastAdmin         = errors.New("不能删除/降级/禁用最后一个活跃管理员")
	ErrPendingNotAllowed = errors.New("请先在审批中心处理待审批账号")
	ErrUserNotFound      = errors.New("用户不存在")
	ErrNoEmail           = errors.New("该用户无邮箱，请先补填邮箱")
	ErrSMTPNotConfigured = errors.New("SMTP 未配置")
)

// AdminService 管理员用户管理服务
type AdminService struct {
	store    *store.Store
	users    *Service
	tokens   *token.Service
	resetSvc *auth.ResetService
	cfg      *config.Service
	versions *version.Service
	log      *slog.Logger
}

func NewAdminService(st *store.Store, users *Service, tokens *token.Service, resetSvc *auth.ResetService, cfg *config.Service, versions *version.Service, lg *slog.Logger) *AdminService {
	return &AdminService{store: st, users: users, tokens: tokens, resetSvc: resetSvc, cfg: cfg, versions: versions, log: lg}
}

// --- 五重保护校验辅助（均在事务内实时查库，不缓存）---

// checkNotSelf 操作者不能操作自己（删自己/改自己角色/禁用自己/重置自己密码）
func (s *AdminService) checkNotSelf(operatorID, targetID int64) error {
	if operatorID == targetID {
		return ErrSelfOperation
	}
	return nil
}

// countActiveAdmins 活跃（未禁用）管理员数，排除指定用户
func (s *AdminService) countActiveAdmins(ctx context.Context, tx *sql.Tx, excludeID int64) (int, error) {
	var n int
	err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE role = 'admin' AND status = 'active' AND id != ?`, excludeID).Scan(&n)
	return n, err
}

// smtpConfigured SMTP 是否已配置（host+user+password 三键非空；与 mail 包判定口径一致，Step 2 接通）
func (s *AdminService) smtpConfigured(ctx context.Context) bool {
	host, _ := s.cfg.Get(ctx, "smtp_host")
	user, _ := s.cfg.Get(ctx, "smtp_user")
	pass, _ := s.cfg.Get(ctx, "smtp_password")
	return host != "" && user != "" && pass != ""
}

// --- 列表：后端分页（默认 20 条/页）+ 用户名/邮箱模糊搜索 ---

type ListQuery struct {
	Page, Size int
	Keyword    string // 用户名/邮箱模糊
}

// AdminUser 用户管理列表项
// CustomSubItem 用户的自定义订阅（版本管理跳转/删除用）
type CustomSubItem struct {
	ID           int64  `json:"id"`
	PlatformID   int64  `json:"platform_id"`
	PlatformName string `json:"platform_name"`
}

type AdminUser struct {
	ID          int64          `json:"id"`
	Username    string         `json:"username"`
	Email       string         `json:"email"`     // 空串 = 无邮箱（前端灰 tag）
	Role        string         `json:"role"`
	GroupID     int64          `json:"group_id"`  // 0 = 无组
	GroupName   string         `json:"group_name"` // 空串 = 无组
	Source      string         `json:"source"`    // oidc/local/selfreg
	Status      string          `json:"status"`        // pending/active/disabled
	HasPassword bool            `json:"has_password"` // 清 OIDC 绑定警告用
	HasOidcBind bool            `json:"has_oidc_binding"` // 是否已绑定 OIDC 身份（清绑定入口可见性）
	CustomSubs  []CustomSubItem `json:"custom_subs"`  // 自定义订阅列表（空 = 无）
}

func (s *AdminService) List(ctx context.Context, q ListQuery) ([]AdminUser, int64, error) {
	if q.Size <= 0 {
		q.Size = 20
	}
	if q.Size > 100 {
		q.Size = 100 // 上限防滥用
	}
	if q.Page <= 0 {
		q.Page = 1
	}
	kw := "%" + q.Keyword + "%"
	var total int64
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE (? = '' OR username LIKE ? OR email LIKE ?)`, q.Keyword, kw, kw).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计用户数失败: %w", err)
	}
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT u.id, u.username, COALESCE(u.email,''), u.role, COALESCE(u.group_id,0), COALESCE(g.name,''),
		        u.user_source, u.status, u.password_hash IS NOT NULL, u.oidc_subject IS NOT NULL
		 FROM users u LEFT JOIN groups g ON g.id = u.group_id
		 WHERE (? = '' OR u.username LIKE ? OR u.email LIKE ?)
		 ORDER BY u.id LIMIT ? OFFSET ?`,
		q.Keyword, kw, kw, q.Size, (q.Page-1)*q.Size)
	if err != nil {
		return nil, 0, fmt.Errorf("查询用户列表失败: %w", err)
	}
	defer rows.Close()
	out := make([]AdminUser, 0) // 空列表返回 [] 而非 null（前端 .map 安全）
	ids := make([]int64, 0, q.Size)
	for rows.Next() {
		var u AdminUser
		var hasPwd, hasOidc int
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.GroupID, &u.GroupName,
			&u.Source, &u.Status, &hasPwd, &hasOidc); err != nil {
			return nil, 0, fmt.Errorf("解析用户行失败: %w", err)
		}
		u.HasPassword = hasPwd == 1
		u.HasOidcBind = hasOidc == 1
		out = append(out, u)
		ids = append(ids, u.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	// 批量填充自定义订阅（单条查询分组，避免 N+1）
	if len(ids) > 0 {
		placeholders := strings.Repeat("?,", len(ids))
		placeholders = placeholders[:len(placeholders)-1]
		args := make([]any, len(ids))
		for i, id := range ids {
			args[i] = id
		}
		crows, err := s.store.DB().QueryContext(ctx,
			`SELECT c.user_id, c.id, c.platform_id, COALESCE(p.name,'')
			 FROM custom_subscriptions c LEFT JOIN platforms p ON p.id = c.platform_id
			 WHERE c.user_id IN (`+placeholders+`) ORDER BY c.id`, args...)
		if err != nil {
			return nil, 0, fmt.Errorf("查询自定义订阅失败: %w", err)
		}
		idx := map[int64]int{}
		for i, u := range out {
			idx[u.ID] = i
		}
		for crows.Next() {
			var userID int64
			var c CustomSubItem
			if err := crows.Scan(&userID, &c.ID, &c.PlatformID, &c.PlatformName); err != nil {
				_ = crows.Close()
				return nil, 0, fmt.Errorf("解析自定义订阅行失败: %w", err)
			}
			if i, ok := idx[userID]; ok {
				out[i].CustomSubs = append(out[i].CustomSubs, c)
			}
		}
		if err := crows.Err(); err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}

// --- 新建用户：用户名 + 邮箱 + 密码（邮箱唯一冲突 409；密码复杂度 ≥8；来源 local，默认直接激活）---

func (s *AdminService) Create(ctx context.Context, username, emailRaw, password string) (*User, error) {
	email, err := auth.NormalizeEmail(emailRaw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	if err := auth.ValidatePassword(password); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}
	var created *User
	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var dup int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE email = ?`, email).Scan(&dup); err != nil {
			return err
		}
		if dup > 0 {
			return ErrEmailConflict
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO users (username, email, password_hash, role, user_source, status) VALUES (?,?,?,?,?,?)`,
			username, email, hash, "user", "local", "active")
		if err != nil {
			return ErrEmailConflict // 并发下 UNIQUE 约束失败同样按 409 处理
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		created = &User{ID: id, Username: username, Email: email, Role: "user", Status: "active", Source: "local", HasPassword: true}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 欢迎邮件：管理员创建的本地用户默认直接激活（Design1 §3.4.6，Step 2 注入回调后发送）
	s.users.sendWelcomeIf(ctx, created.Email, created.Source)
	s.log.Info("管理员创建用户", "user_id", created.ID, "operator", "admin")
	return created, nil
}

// --- 编辑：调整所属组（换组无需清 Token，实时解析跟随，Design1 §3.4.5）---

func (s *AdminService) UpdateGroup(ctx context.Context, targetID, groupID int64) error {
	var n int
	if err := s.store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM groups WHERE id = ?`, groupID).Scan(&n); err != nil {
		return fmt.Errorf("校验用户组失败: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("%w: 用户组不存在", ErrBadRequest)
	}
	res, err := s.store.DB().ExecContext(ctx,
		`UPDATE users SET group_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, groupID, targetID)
	if err != nil {
		return fmt.Errorf("更新用户组失败: %w", err)
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// --- 角色变更（admin↔user）：仅可由其他管理员执行；降级级联清显式 Token ---

func (s *AdminService) ChangeRole(ctx context.Context, operatorID, targetID int64, newRole string) error {
	if newRole != "admin" && newRole != "user" {
		return fmt.Errorf("%w: 角色无效", ErrBadRequest)
	}
	if err := s.checkNotSelf(operatorID, targetID); err != nil { // 禁止改自己角色
		return err
	}
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var curRole string
		if err := tx.QueryRowContext(ctx, `SELECT role FROM users WHERE id = ?`, targetID).Scan(&curRole); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrUserNotFound
			}
			return err
		}
		if curRole == "admin" && newRole == "user" {
			remaining, err := s.countActiveAdmins(ctx, tx, targetID)
			if err != nil {
				return err
			}
			if remaining == 0 {
				return ErrLastAdmin // 降级最后一个活跃管理员
			}
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE users SET role = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, newRole, targetID); err != nil {
			return err
		}
		// 降级（admin→user）同事务级联清全部显式订阅 Token（Design1 §2.5）
		if curRole == "admin" && newRole == "user" {
			if _, err := tx.ExecContext(ctx,
				`DELETE FROM download_tokens WHERE user_id = ? AND subscription_id IS NOT NULL`, targetID); err != nil {
				return err
			}
		}
		return nil
	})
}

// --- 设置/重置密码（关键约束，Design1 §3.4.5）---

// directResetCharset 直接重置字符集：大小写字母 + 数字，去除易混淆 i I o O 0 l L，无特殊符号，8 位
const directResetCharset = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKMNPQRSTUVWXYZ23456789"

// genDirectResetPassword crypto/rand 取 8 字符
func genDirectResetPassword() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("生成随机密码失败: %w", err)
	}
	for i := range b {
		b[i] = directResetCharset[int(b[i])%len(directResetCharset)]
	}
	return string(b), nil
}

// ResetPasswordDirect 直接重置：待审批拒绝；重置后递增 credential_version（全部现有会话立即失效，同事务）。
// 二次确认由前端负责；返回明文密码供接入层展示复制（仅此一次）
func (s *AdminService) ResetPasswordDirect(ctx context.Context, operatorID, targetID int64) (string, error) {
	if operatorID == targetID {
		return "", ErrSelfOperation // 管理员不能通过本入口操作自己的密码
	}
	pwd, err := genDirectResetPassword()
	if err != nil {
		return "", err
	}
	hash, err := auth.HashPassword(pwd)
	if err != nil {
		return "", err
	}
	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM users WHERE id = ?`, targetID).Scan(&status); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrUserNotFound
			}
			return err
		}
		if status == "pending" {
			return ErrPendingNotAllowed
		}
		_, err := tx.ExecContext(ctx,
			`UPDATE users SET password_hash = ?, credential_version = credential_version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			hash, targetID) // 重置后全部现有会话立即失效
		return err
	})
	if err != nil {
		return "", err
	}
	s.log.Info("管理员直接重置用户密码", "user_id", targetID)
	return pwd, nil
}

// ResetPasswordByEmail 触发重置邮件：生成一次性重置令牌（1h TTL，复用 ResetService）并发送。
// 已配置 SMTP 时可选；待审批拒绝；无邮箱拒绝（提示先补填）
func (s *AdminService) ResetPasswordByEmail(ctx context.Context, operatorID, targetID int64) error {
	if err := s.checkNotSelf(operatorID, targetID); err != nil {
		return err
	}
	if !s.smtpConfigured(ctx) {
		return ErrSMTPNotConfigured
	}
	var status, email string
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT status, COALESCE(email,'') FROM users WHERE id = ?`, targetID).Scan(&status, &email); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUserNotFound
		}
		return err
	}
	if status == "pending" {
		return ErrPendingNotAllowed
	}
	if email == "" {
		return ErrNoEmail
	}
	return s.resetSvc.IssueForUser(ctx, targetID, email)
}

// FillEmail 无邮箱用户补填邮箱（规范化 + 唯一预查；补填后获得设置密码/重置能力，Design1 §4.6）
func (s *AdminService) FillEmail(ctx context.Context, targetID int64, emailRaw string) error {
	email, err := auth.NormalizeEmail(emailRaw)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var cur string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(email,'') FROM users WHERE id = ?`, targetID).Scan(&cur); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrUserNotFound
			}
			return err
		}
		if cur != "" {
			return fmt.Errorf("%w: 该用户已有邮箱", ErrBadRequest)
		}
		var dup int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE email = ?`, email).Scan(&dup); err != nil {
			return err
		}
		if dup > 0 {
			return ErrEmailConflict
		}
		_, err := tx.ExecContext(ctx,
			`UPDATE users SET email = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, email, targetID)
		return err
	})
}

// --- 禁用/启用（禁用 = 同一事务内递增 credential_version + 物理删全部 Token）---

func (s *AdminService) SetStatus(ctx context.Context, operatorID, targetID int64, disable bool) error {
	if err := s.checkNotSelf(operatorID, targetID); err != nil { // 禁止禁用自己
		return err
	}
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if disable {
			var role string
			if err := tx.QueryRowContext(ctx, `SELECT role FROM users WHERE id = ?`, targetID).Scan(&role); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return ErrUserNotFound
				}
				return err
			}
			if role == "admin" {
				remaining, err := s.countActiveAdmins(ctx, tx, targetID)
				if err != nil {
					return err
				}
				if remaining == 0 {
					return ErrLastAdmin // 禁止禁用最后一个活跃管理员
				}
			}
			// 同一事务：递增 credential_version（会话立即失效）+ 物理删除全部 Token（防竞态窗口）
			if _, err := tx.ExecContext(ctx,
				`UPDATE users SET status = 'disabled', credential_version = credential_version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, targetID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM download_tokens WHERE user_id = ?`, targetID); err != nil {
				return err
			}
			return nil
		}
		// 启用：不恢复原 Token（用户下次访问首页重新生成）
		res, err := tx.ExecContext(ctx,
			`UPDATE users SET status = 'active', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, targetID)
		if err != nil {
			return err
		}
		if affected, _ := res.RowsAffected(); affected == 0 {
			return ErrUserNotFound
		}
		return nil
	})
}

// --- 吊销所有下载 Token（物理删除，无标记态；用户下次访问首页重新生成，Design1 §3.4.5）---

func (s *AdminService) RevokeAllTokens(ctx context.Context, targetID int64) error {
	if _, err := s.store.DB().ExecContext(ctx, `DELETE FROM download_tokens WHERE user_id = ?`, targetID); err != nil {
		return fmt.Errorf("吊销下载 Token 失败: %w", err)
	}
	return nil
}

// --- 清除 OIDC 绑定：清空 oidc_subject；返回 has_password 标记供前端警告（无密码时清除后无法登录）---

func (s *AdminService) ClearOidcBinding(ctx context.Context, targetID int64) (hasPassword bool, err error) {
	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var hp int
		if err := tx.QueryRowContext(ctx,
			`SELECT password_hash IS NOT NULL FROM users WHERE id = ?`, targetID).Scan(&hp); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrUserNotFound
			}
			return err
		}
		hasPassword = hp == 1
		if _, err := tx.ExecContext(ctx,
			`UPDATE users SET oidc_subject = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, targetID); err != nil {
			return err
		}
		return nil
	})
	return hasPassword, err
}

// --- 删除用户（级联 + 五重保护）---

func (s *AdminService) Delete(ctx context.Context, operatorID, targetID int64) error {
	if err := s.checkNotSelf(operatorID, targetID); err != nil { // 禁止删自己
		return err
	}
	var files []string
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var role, status string
		if err := tx.QueryRowContext(ctx, `SELECT role, status FROM users WHERE id = ?`, targetID).Scan(&role, &status); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrUserNotFound
			}
			return err
		}
		if role == "admin" && status == "active" {
			remaining, err := s.countActiveAdmins(ctx, tx, targetID)
			if err != nil {
				return err
			}
			if remaining == 0 {
				return ErrLastAdmin // 禁止删除最后一个管理员
			}
		}
		// 级联：全部 Token + 自定义订阅（含版本文件）；待审批账号删除与审批中心「拒绝」同效果（邮箱释放）
		if _, err := tx.ExecContext(ctx, `DELETE FROM download_tokens WHERE user_id = ?`, targetID); err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx, `SELECT id FROM custom_subscriptions WHERE user_id = ?`, targetID)
		if err != nil {
			return err
		}
		var ids []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for _, id := range ids {
			fs, err := s.versions.CollectVersionFiles(ctx, tx, version.OwnerCustom, id) // 事务内收集，提交后删除
			if err != nil {
				return err
			}
			files = append(files, fs...)
			if err := s.versions.DeleteVersionsTx(ctx, tx, version.OwnerCustom, id); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM custom_subscriptions WHERE user_id = ?`, targetID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, targetID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	// 事务提交后删自定义版本文件（失败记日志，不阻断）；路径必须位于数据卷 contents 目录内（防 DB 记录逃逸）
	contentsRoot := s.versions.ContentsRoot()
	for _, f := range files {
		clean := filepath.Clean(f)
		if !strings.HasPrefix(clean, contentsRoot+string(os.PathSeparator)) {
			s.log.Warn("跳过越界版本文件删除", "file", f)
			continue
		}
		if err := os.Remove(clean); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.log.Warn("删除自定义订阅版本文件失败", "file", clean, "err", err)
		}
	}
	s.log.Info("用户已删除", "user_id", targetID, "operator_id", operatorID)
	return nil
}

// --- 批量操作：为所有无密码用户发送密码设置链接 ---
// 仅面向已激活的无密码用户；待审批/已禁用/无邮箱自动排除并回执计数；依赖 SMTP（未配置返回错误，前端置灰）
func (s *AdminService) BatchSendPasswordLinks(ctx context.Context) (sent, skippedPending, skippedDisabled, skippedNoEmail int, err error) {
	if !s.smtpConfigured(ctx) {
		return 0, 0, 0, 0, ErrSMTPNotConfigured
	}
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT id, COALESCE(email,''), status FROM users WHERE password_hash IS NULL`)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("查询无密码用户失败: %w", err)
	}
	type rec struct {
		id     int64
		email  string
		status string
	}
	var recs []rec
	for rows.Next() {
		var r rec
		if err := rows.Scan(&r.id, &r.email, &r.status); err != nil {
			_ = rows.Close()
			return 0, 0, 0, 0, err
		}
		recs = append(recs, r)
	}
	if err := rows.Close(); err != nil {
		return 0, 0, 0, 0, err
	}
	for _, r := range recs {
		switch {
		case r.status == "pending":
			skippedPending++
		case r.status == "disabled":
			skippedDisabled++
		case r.email == "":
			skippedNoEmail++
		default:
			if err := s.resetSvc.IssueForUser(ctx, r.id, r.email); err != nil {
				s.log.Warn("批量发送密码设置链接失败", "user_id", r.id, "err", err) // 单项失败不阻断其余
				continue
			}
			sent++
		}
	}
	s.log.Info("批量发送密码设置链接完成", "sent", sent, "skipped_pending", skippedPending, "skipped_disabled", skippedDisabled, "skipped_no_email", skippedNoEmail)
	return sent, skippedPending, skippedDisabled, skippedNoEmail, nil
}
