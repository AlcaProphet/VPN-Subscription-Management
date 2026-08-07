// 外部测试包：避免 auth ↔ user 测试循环依赖（user 包 import auth 包）
package auth_test

import (
	"context"
	"database/sql"
	"testing"
	"testing/fstest"
	"time"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/internal/user"
)

// newResetEnv 创建临时库 + 重置服务环境
func newResetEnv(t *testing.T) (*store.Store, *auth.ResetService, *user.Service) {
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
		"0005_reset.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS password_reset_tokens (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at TIMESTAMP NOT NULL,
			used INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
	}
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	users := user.NewService(st, cfg, log.New("error", "console"))
	resetSvc := auth.NewResetService(st, users, log.New("error", "console"))
	return st, resetSvc, users
}

// helperInsertToken 直接插入令牌记录（测试 TTL/used 分支）
func helperInsertToken(t *testing.T, db *sql.DB, token string, userID int64, expiresAt time.Time, used int) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO password_reset_tokens (token, user_id, expires_at, used) VALUES (?,?,?,?)`,
		token, userID, expiresAt, used); err != nil {
		t.Fatalf("插入令牌失败: %v", err)
	}
}

// TestResetRequestAndComplete 请求生成令牌 → 重置成功 → 旧密码失效新密码可登录 → 二次 Complete 失败
func TestResetRequestAndComplete(t *testing.T) {
	st, resetSvc, users := newResetEnv(t)
	ctx := context.Background()
	if _, err := users.Register(ctx, "kyle", "kyle@example.com", "oldpass123"); err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	if err := resetSvc.Request(ctx, "kyle@example.com"); err != nil {
		t.Fatalf("Request 失败: %v", err)
	}
	// 从库中取令牌
	var token string
	if err := st.DB().QueryRow(`SELECT token FROM password_reset_tokens LIMIT 1`).Scan(&token); err != nil {
		t.Fatalf("查询令牌失败: %v", err)
	}
	if len(token) < 32 {
		t.Errorf("令牌熵不足: %d", len(token))
	}
	// 完成重置
	if err := resetSvc.Complete(ctx, token, "newpass123"); err != nil {
		t.Fatalf("Complete 失败: %v", err)
	}
	// 旧密码失败、新密码成功
	if _, err := users.Login(ctx, "kyle@example.com", "oldpass123"); err == nil {
		t.Error("旧密码应失效")
	}
	if _, err := users.Login(ctx, "kyle@example.com", "newpass123"); err != nil {
		t.Errorf("新密码应可登录: %v", err)
	}
	// 凭据版本号递增（旧会话失效）
	var cv int
	if err := st.DB().QueryRow(`SELECT credential_version FROM users WHERE email='kyle@example.com'`).Scan(&cv); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if cv != 1 {
		t.Errorf("凭据版本号应递增: %d", cv)
	}
	// 用后即删：二次 Complete 失败
	if err := resetSvc.Complete(ctx, token, "another123"); err == nil {
		t.Error("二次使用同令牌应失败（用后即删）")
	}
}

// TestResetTokenExpiredAndUsed 过期令牌/已用令牌拒绝
func TestResetTokenExpiredAndUsed(t *testing.T) {
	st, resetSvc, users := newResetEnv(t)
	ctx := context.Background()
	u, err := users.Register(ctx, "kyle", "kyle@example.com", "password123")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	// 过期令牌
	helperInsertToken(t, st.DB(), "expired-token", u.ID, time.Now().Add(-time.Minute), 0)
	if err := resetSvc.Complete(ctx, "expired-token", "newpass123"); err == nil {
		t.Error("过期令牌应拒绝")
	}
	// 已用令牌（used=1）
	helperInsertToken(t, st.DB(), "used-token", u.ID, time.Now().Add(time.Hour), 1)
	if err := resetSvc.Complete(ctx, "used-token", "newpass123"); err == nil {
		t.Error("已用令牌应拒绝")
	}
	// 不存在令牌
	if err := resetSvc.Complete(ctx, "ghost-token", "newpass123"); err == nil {
		t.Error("不存在令牌应拒绝")
	}
}

// TestResetAntiEnumeration 防枚举：存在/不存在邮箱的 forgot 响应一致（均成功无差异）
func TestResetAntiEnumeration(t *testing.T) {
	_, resetSvc, users := newResetEnv(t)
	ctx := context.Background()
	if _, err := users.Register(ctx, "kyle", "kyle@example.com", "password123"); err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	// 存在邮箱 → 生成令牌（无错误）
	if err := resetSvc.Request(ctx, "kyle@example.com"); err != nil {
		t.Errorf("存在邮箱 Request 失败: %v", err)
	}
	// 不存在邮箱 → 同样无错误
	if err := resetSvc.Request(ctx, "ghost@example.com"); err != nil {
		t.Errorf("不存在邮箱 Request 失败: %v", err)
	}
	// 格式非法 → 同样无错误
	if err := resetSvc.Request(ctx, "not-an-email"); err != nil {
		t.Errorf("非法邮箱 Request 失败: %v", err)
	}
}
