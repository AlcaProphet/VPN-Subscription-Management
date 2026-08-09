// Package user 提供用户业务层：注册（含首管理员机制）、登录、快照查询。
package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/config"
	"vpn-sub/internal/store"
)

var (
	ErrEmailConflict   = errors.New("邮箱已被注册")
	ErrAuthFailed      = errors.New("邮箱或密码错误") // 统一措辞，防枚举
	ErrAccountInactive = errors.New("账号未激活或已被禁用")
	ErrBadRequest      = errors.New("参数错误")
)

// Service 用户服务
type Service struct {
	store *store.Store
	cfg   *config.Service
	log   *slog.Logger
	// sendWelcome 预留：新用户首次激活时发送欢迎邮件（Build3 Step 2 注入 mail 包；nil 时跳过）
	sendWelcome func(ctx context.Context, to, source string) error
}

func NewService(st *store.Store, cfg *config.Service, lg *slog.Logger) *Service {
	return &Service{store: st, cfg: cfg, log: lg}
}

// SetWelcomeSender 注入欢迎邮件发送函数（Build3 Step 2 SMTP 接通时调用）
func (s *Service) SetWelcomeSender(fn func(ctx context.Context, to, source string) error) {
	s.sendWelcome = fn
}

// defaultGroupIDTx 查询预置默认组 ID（Setup 事务内创建，理论恒存在）；
// 查不到或 groups 表未建立（单测/极端场景）返回 nil → 用户 group_id 存 NULL（单组互斥语义下不越权）
func defaultGroupIDTx(ctx context.Context, tx *sql.Tx) (any, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM groups WHERE is_default = 1 LIMIT 1`).Scan(&id)
	if err == nil {
		return id, nil
	}
	if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "no such table") {
		return nil, nil
	}
	return nil, err
}

// sendWelcomeIf 新用户首次激活时发送欢迎邮件（Design1 §3.4.6）；失败不阻断主流程
func (s *Service) sendWelcomeIf(ctx context.Context, email, source string) {
	if s.sendWelcome == nil || email == "" {
		return
	}
	if err := s.sendWelcome(ctx, email, source); err != nil {
		s.log.Warn("欢迎邮件发送失败", "email", email, "err", err)
	}
}

// User 对外用户信息
type User struct {
	ID                int64
	Username          string
	Email             string // 空串表示 NULL
	Role              string
	Status            string
	Source            string
	GroupID           int64 // 0 表示 NULL
	CredentialVersion int
	HasPassword       bool
	PasswordHash      string // 内部使用（登录校验）
	OidcSubject       string
}

// Register 自注册 + 首管理员机制（Design1 §2.5，关键约束）：
// 「邮箱唯一预查 → 空表判定 → 写入（首管理员）→ 置位标记」全程单个 BEGIN IMMEDIATE 事务（并发串行化）
func (s *Service) Register(ctx context.Context, username, emailRaw, password string) (*User, error) {
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
		// 1) 邮箱唯一冲突 → 409（基于规范化值）
		var dup int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE email = ?`, email).Scan(&dup); err != nil {
			return err
		}
		if dup > 0 {
			return ErrEmailConflict
		}
		// 2) 空表判定口径：任何用户记录（含待审批）存在即算非空
		var total int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
			return err
		}
		first := total == 0
		// 3) 默认状态：自注册审批开关本 Build 默认关闭 → 注册即 active；
		//    首管理员检查先于审批开关判定：空表时永远免审批直接激活（防死锁，Design1 §2.6）
		role, status, source := "user", "active", "selfreg"
		if !first && s.selfRegApprovalEnabledTx(ctx, tx) { // 开关读取路径预留（Build3 接通配置键）；事务内读取防连接池死锁
			status = "pending"
		}
		if first {
			role, status = "admin", "active" // 首管理员免审批，不受任何审批开关影响
		}
		// 新用户自动加入预置默认组（Design1 §2.2：OIDC/自注册/管理员创建统一口径）
		gid, err := defaultGroupIDTx(ctx, tx)
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO users (username, email, password_hash, role, user_source, status, group_id) VALUES (?,?,?,?,?,?,?)`,
			username, email, hash, role, source, status, gid)
		if err != nil {
			return ErrEmailConflict // 并发下 UNIQUE 约束失败同样按 409 处理
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		created = &User{ID: id, Username: username, Email: email, Role: role, Status: status, Source: source, HasPassword: true}
		// 4) 首管理员：同事务置位「已初始化」标记（用户表为空时忽略该标记）
		if first {
			if err := s.cfg.SetTx(ctx, tx, config.KeyAdminInitialized, "true"); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 欢迎邮件：直接激活（含首管理员）时发送；待审批不发（审批通过时由审批中心发送）
	if created.Status == "active" {
		s.sendWelcomeIf(ctx, created.Email, created.Source)
	}
	s.log.Info("用户注册成功", "user_id", created.ID, "role", created.Role, "first_admin", created.Role == "admin")
	return created, nil
}

// selfRegApprovalEnabledTx 事务内读配置键 selfreg_approval（默认 false；Build3 面板接通）。
// 事务内禁止经 store.DB() 二次取连接（MaxOpenConns=1 会死锁），必须走 tx 查询
func (s *Service) selfRegApprovalEnabledTx(ctx context.Context, tx *sql.Tx) bool {
	v, err := s.cfg.GetTx(ctx, tx, config.KeySelfRegApproval)
	if err != nil || v == "" {
		return false
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false
	}
	return b
}

// Login 邮箱 + 密码校验；失败提示统一措辞（不区分「邮箱不存在」与「密码错误」，防枚举）
func (s *Service) Login(ctx context.Context, emailRaw, password string) (*User, error) {
	email, err := auth.NormalizeEmail(emailRaw)
	if err != nil {
		return nil, ErrAuthFailed // 格式非法也归入统一措辞
	}
	u, err := s.getByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if u == nil || !u.HasPassword || !auth.CheckPassword(u.PasswordHash, password) {
		return nil, ErrAuthFailed
	}
	if u.Status != "active" { // 仅 active 可登录；待审批/已禁用统一提示
		return nil, ErrAccountInactive
	}
	return u, nil
}

// SnapshotByID 实现 auth.UserSource（供凭据校验中间件实时查库）
func (s *Service) SnapshotByID(ctx context.Context, id int64) (*auth.UserSnapshot, error) {
	var snap auth.UserSnapshot
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT id, role, status, credential_version FROM users WHERE id = ?`, id).
		Scan(&snap.ID, &snap.Role, &snap.Status, &snap.CredentialVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询用户快照失败: %w", err)
	}
	return &snap, nil
}

// IsTableEmpty 注册入口可见性用（空表 = 0 行，含待审批）
func (s *Service) IsTableEmpty(ctx context.Context) (bool, error) {
	if s.store == nil { // 应急模式（DB 无法打开）下按非空处理，保证 status 端点可用不 panic
		return false, nil
	}
	var n int
	if err := s.store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return false, fmt.Errorf("查询用户表失败: %w", err)
	}
	return n == 0, nil
}

// GetByID 按 ID 查询用户
func (s *Service) GetByID(ctx context.Context, id int64) (*User, error) {
	return s.scanUser(ctx, `SELECT id, COALESCE(oidc_subject,''), username, COALESCE(email,''), role,
		COALESCE(group_id,0), COALESCE(password_hash,''), user_source, status, credential_version
		FROM users WHERE id = ?`, id)
}

// GroupNameByID 组名查询（首页顶栏所属组标签用；组不存在返回空串）
func (s *Service) GroupNameByID(ctx context.Context, groupID int64) (string, error) {
	var name string
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT name FROM groups WHERE id = ?`, groupID).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("读取组名失败: %w", err)
	}
	return name, nil
}

// FindForReset 实现 auth.ResetUserSource（密码重置场景：按规范化邮箱查最小信息）
func (s *Service) FindForReset(ctx context.Context, email string) (*auth.ResetTarget, error) {
	normalized, err := auth.NormalizeEmail(email)
	if err != nil {
		return nil, nil // 格式非法按未命中处理（不泄露差异）
	}
	u, err := s.getByEmail(ctx, normalized)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, nil
	}
	return &auth.ResetTarget{ID: u.ID, Email: u.Email, HasPassword: u.HasPassword}, nil
}

// getByEmail 按邮箱查询用户（登录/防枚举场景）
func (s *Service) getByEmail(ctx context.Context, email string) (*User, error) {
	return s.scanUser(ctx, `SELECT id, COALESCE(oidc_subject,''), username, COALESCE(email,''), role,
		COALESCE(group_id,0), COALESCE(password_hash,''), user_source, status, credential_version
		FROM users WHERE email = ?`, email)
}

// scanUser 通用行扫描
func (s *Service) scanUser(ctx context.Context, query string, args ...any) (*User, error) {
	var u User
	err := s.store.DB().QueryRowContext(ctx, query, args...).Scan(
		&u.ID, &u.OidcSubject, &u.Username, &u.Email, &u.Role,
		&u.GroupID, &u.PasswordHash, &u.Source, &u.Status, &u.CredentialVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	u.HasPassword = u.PasswordHash != ""
	return &u, nil
}
