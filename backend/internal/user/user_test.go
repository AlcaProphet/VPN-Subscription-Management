package user

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
)

// newTestUserService 创建临时库 + 用户服务（含 users 表迁移）
func newTestUserService(t *testing.T) (*store.Store, *Service) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	fsys := fstest.MapFS{
		"0001_init.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY, value TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"0002_users.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			oidc_subject TEXT UNIQUE,
			username TEXT NOT NULL,
			email TEXT UNIQUE,
			role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin','user')),
			group_id INTEGER,
			password_hash TEXT,
			user_source TEXT NOT NULL CHECK (user_source IN ('oidc','local','selfreg')),
			status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','active','disabled')),
			credential_version INTEGER NOT NULL DEFAULT 0,
			oidc_claims TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
	}
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	svc := NewService(st, cfg, log.New("error", "console"))
	return st, svc
}

// TestRegisterJoinsDefaultGroup 新用户（自注册/管理员创建/OIDC）自动加入预置默认组（Design1 §2.2）
func TestRegisterJoinsDefaultGroup(t *testing.T) {
	st, svc := newTestUserService(t)
	ctx := context.Background()
	// 模拟 Setup 预置：groups 表 + 默认组行
	if _, err := st.DB().ExecContext(ctx, `CREATE TABLE IF NOT EXISTS groups (
		id INTEGER PRIMARY KEY AUTOINCREMENT, slug TEXT UNIQUE, name TEXT NOT NULL,
		is_default INTEGER NOT NULL DEFAULT 0, needs_reselect INTEGER NOT NULL DEFAULT 0);
		INSERT INTO groups (slug, name, is_default) VALUES ('group-default', '默认组', 1);`); err != nil {
		t.Fatalf("预置默认组失败: %v", err)
	}
	// 自注册路径
	u, err := svc.Register(ctx, "kyle", "kyle@example.com", "password123")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	var gid any
	if err := st.DB().QueryRowContext(ctx, `SELECT group_id FROM users WHERE id = ?`, u.ID).Scan(&gid); err != nil {
		t.Fatalf("查询 group_id 失败: %v", err)
	}
	if gid == nil {
		t.Errorf("新注册用户应自动加入默认组，实际 group_id=NULL")
	}
}

// TestRegisterFirstAdmin 首个注册用户自动成为 admin 并置位初始化标记
func TestRegisterFirstAdmin(t *testing.T) {
	_, svc := newTestUserService(t)
	ctx := context.Background()
	u, err := svc.Register(ctx, "kyle", "kyle@example.com", "password123")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	if u.Role != "admin" || u.Status != "active" {
		t.Errorf("首用户应为 admin/active: role=%s status=%s", u.Role, u.Status)
	}
	// 第二个用户为普通 user
	u2, err := svc.Register(ctx, "bob", "bob@example.com", "password123")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	if u2.Role != "user" {
		t.Errorf("第二用户应为 user: %s", u2.Role)
	}
	// 邮箱冲突 409
	if _, err := svc.Register(ctx, "kyle2", "KYLE@example.com", "password123"); !errors.Is(err, ErrEmailConflict) {
		t.Errorf("大小写归一化后邮箱冲突应返回 ErrEmailConflict: %v", err)
	}
}

// TestRegisterConcurrentFirstAdmin 并发 N 个 Register 只产生一个 admin（BEGIN IMMEDIATE 串行化）
func TestRegisterConcurrentFirstAdmin(t *testing.T) {
	_, svc := newTestUserService(t)
	ctx := context.Background()
	const N = 6
	var wg sync.WaitGroup
	errs := make(chan error, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, err := svc.Register(ctx, fmt.Sprintf("user%d", n), fmt.Sprintf("user%d@example.com", n), "password123")
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil && !errors.Is(err, ErrEmailConflict) {
			t.Fatalf("并发注册失败: %v", err)
		}
	}
	// 统计 admin 数量
	var admins int
	if err := svc.store.DB().QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&admins); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if admins != 1 {
		t.Errorf("并发注册应只产生一个 admin: got %d", admins)
	}
}

// TestLoginUnifiedMessage 不存在邮箱与错误密码返回同一错误（防枚举）
func TestLoginUnifiedMessage(t *testing.T) {
	_, svc := newTestUserService(t)
	ctx := context.Background()
	if _, err := svc.Register(ctx, "kyle", "kyle@example.com", "password123"); err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	// 正确登录
	u, err := svc.Login(ctx, "kyle@example.com", "password123")
	if err != nil || u == nil {
		t.Fatalf("正确登录失败: %v", err)
	}
	// 错误密码
	if _, err := svc.Login(ctx, "kyle@example.com", "wrongpass"); !errors.Is(err, ErrAuthFailed) {
		t.Errorf("错误密码应返回 ErrAuthFailed: %v", err)
	}
	// 不存在邮箱
	if _, err := svc.Login(ctx, "ghost@example.com", "password123"); !errors.Is(err, ErrAuthFailed) {
		t.Errorf("不存在邮箱应返回 ErrAuthFailed: %v", err)
	}
	// 大小写不敏感登录
	if _, err := svc.Login(ctx, "KYLE@example.com", "password123"); err != nil {
		t.Errorf("大写邮箱应可登录: %v", err)
	}
}

// TestLoginInactive 待审批/禁用账号统一提示
func TestLoginInactive(t *testing.T) {
	st, svc := newTestUserService(t)
	ctx := context.Background()
	if _, err := svc.Register(ctx, "kyle", "kyle@example.com", "password123"); err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	// 直接改状态为 disabled
	if _, err := st.DB().Exec(`UPDATE users SET status = 'disabled' WHERE email = 'kyle@example.com'`); err != nil {
		t.Fatalf("更新状态失败: %v", err)
	}
	if _, err := svc.Login(ctx, "kyle@example.com", "password123"); !errors.Is(err, ErrAccountInactive) {
		t.Errorf("禁用账号应返回 ErrAccountInactive: %v", err)
	}
}

// TestSnapshotByID 快照查询（会话校验用）
func TestSnapshotByID(t *testing.T) {
	_, svc := newTestUserService(t)
	ctx := context.Background()
	u, err := svc.Register(ctx, "kyle", "kyle@example.com", "password123")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	snap, err := svc.SnapshotByID(ctx, u.ID)
	if err != nil || snap == nil {
		t.Fatalf("快照查询失败: %v", err)
	}
	if snap.Role != "admin" || snap.Status != "active" || snap.CredentialVersion != 0 {
		t.Errorf("快照内容异常: %+v", snap)
	}
	// 不存在用户返回 nil
	snap, err = svc.SnapshotByID(ctx, 99999)
	if err != nil || snap != nil {
		t.Errorf("不存在用户应返回 nil,nil: %v %v", snap, err)
	}
}
