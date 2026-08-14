package download

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/internal/version"
)

// randStr 测试用随机 Token 值
func randStr() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// testMigrateFS 构造下载解析所需的完整表集
func testMigrateFS() fstest.MapFS {
	return fstest.MapFS{
		"0001_init.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY, value TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"0002_users.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT, oidc_subject TEXT UNIQUE,
			username TEXT NOT NULL, email TEXT UNIQUE,
			role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin','user')),
			group_id INTEGER, password_hash TEXT,
			user_source TEXT NOT NULL CHECK (user_source IN ('oidc','local','selfreg')),
			status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','active','disabled')),
			credential_version INTEGER NOT NULL DEFAULT 0, oidc_claims TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"0003_groups_platforms.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS groups (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL UNIQUE,
				is_default INTEGER NOT NULL DEFAULT 0, needs_reselect INTEGER NOT NULL DEFAULT 0,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS platforms (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '', schemes TEXT NOT NULL DEFAULT '[]',
				extra_headers TEXT NOT NULL DEFAULT '{}', installer_files TEXT NOT NULL DEFAULT '[]', installer_urls TEXT NOT NULL DEFAULT '[]',
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"1002_subscriptions_versions.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS subscriptions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
				platform_id INTEGER NOT NULL, current_version INTEGER NOT NULL DEFAULT 0,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS versions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				owner_type TEXT NOT NULL CHECK (owner_type IN ('subscription','rule','custom','share')),
				owner_id INTEGER NOT NULL, version_no INTEGER NOT NULL, file_path TEXT NOT NULL,
								file_name TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				UNIQUE (owner_type, owner_id, version_no));`)},
		"1003_groups.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS group_selections (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				group_id INTEGER NOT NULL,
				platform_id INTEGER NOT NULL,
				subscription_id INTEGER,
				UNIQUE (group_id, platform_id));`)},
		"1004_tokens.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS download_tokens (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				token TEXT NOT NULL UNIQUE,
				user_id INTEGER NOT NULL,
				platform_id INTEGER NOT NULL,
				custom_sub_id INTEGER,
				subscription_id INTEGER,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS access_logs (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id INTEGER, ip TEXT NOT NULL,
				download_type TEXT NOT NULL, platform TEXT,
				resource_slug TEXT NOT NULL, status TEXT NOT NULL,
				fail_reason TEXT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"1005_custom_share.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS custom_subscriptions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE,
				user_id INTEGER NOT NULL,
				platform_id INTEGER NOT NULL,
				current_version INTEGER NOT NULL DEFAULT 0,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				UNIQUE (user_id, platform_id));`)},
	}
}

// env 测试环境
type env struct {
	st   *store.Store
	ver  *version.Service
	svc  *Service
	cfg  *config.Service
	user int64
	plat int64
	sub  int64
}

// newTestDownload 构造完整测试环境：默认组+用户+平台+订阅（2 版内容）+ 组选定
func newTestDownload(t *testing.T) *env {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), testMigrateFS()); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	dataDir := t.TempDir()
	ver := version.NewService(st, dataDir, log.New("error", "console"))
	cfg := config.NewService(st, log.New("error", "console"))
	svc := NewService(st, ver, cfg, log.New("error", "console"))
	ctx := context.Background()
	// 默认组 + 平台
	if _, err := st.DB().Exec(`INSERT INTO groups (slug, name, is_default) VALUES ('group-1', '默认组', 1)`); err != nil {
		t.Fatalf("创建组失败: %v", err)
	}
	res, err := st.DB().Exec(`INSERT INTO platforms (slug, name, extra_headers) VALUES ('platform-x', '测试平台', '{"profile-web-page-url":"{frontend_url}"}')`)
	if err != nil {
		t.Fatalf("创建平台失败: %v", err)
	}
	plat, _ := res.LastInsertId()
	// 用户（普通）
	res, err = st.DB().Exec(`INSERT INTO users (username, email, role, group_id, user_source, status) VALUES ('u1','u1@x.com','user',1,'local','active')`)
	if err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	user, _ := res.LastInsertId()
	// 订阅 + 版本
	res, err = st.DB().Exec(`INSERT INTO subscriptions (slug, name, platform_id) VALUES ('sub-x', '订阅X', ?)`, plat)
	if err != nil {
		t.Fatalf("创建订阅失败: %v", err)
	}
	sub, _ := res.LastInsertId()
	if _, err := ver.CreateVersion(ctx, version.OwnerSubscription, sub, version.BytesContent([]byte("proxies: [x]"))); err != nil {
		t.Fatalf("创建版本失败: %v", err)
	}
	// 组选定
	if _, err := st.DB().Exec(`INSERT INTO group_selections (group_id, platform_id, subscription_id) VALUES (1, ?, ?)`, plat, sub); err != nil {
		t.Fatalf("组选定失败: %v", err)
	}
	_ = cfg.Set(ctx, config.KeyFrontendURL, "https://vpn.example.com")
	return &env{st: st, ver: ver, svc: svc, cfg: cfg, user: user, plat: plat, sub: sub}
}

// mkToken 直接插入 Token 记录并返回 Token 值
func (e *env) mkToken(t *testing.T, customID, subID int64) string {
	t.Helper()
	value := "tok-" + randStr()
	if _, err := e.st.DB().Exec(
		`INSERT INTO download_tokens (token, user_id, platform_id, custom_sub_id, subscription_id) VALUES (?,?,?,?,?)`,
		value, e.user, e.plat, nullID(customID), nullID(subID)); err != nil {
		t.Fatalf("插入 Token 失败: %v", err)
	}
	return value
}

func nullID(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

// TestResolveGroupToken 无标识 Token：组选定内容实时解析
func TestResolveGroupToken(t *testing.T) {
	e := newTestDownload(t)
	ctx := context.Background()
	tk := e.mkToken(t, 0, 0)
	res, entry, err := e.svc.ResolveUserDownload(ctx, tk, "platform-x")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if string(res.Content) != "proxies: [x]" {
		t.Errorf("内容异常: %s", res.Content)
	}
	if entry.Type != "subscription" || entry.ResourceID != e.sub {
		t.Errorf("访问日志参数异常: %+v", entry)
	}
	// 附加头注入（{frontend_url} 替换）
	if res.ExtraHeaders["profile-web-page-url"] != "https://vpn.example.com" {
		t.Errorf("附加头占位符替换异常: %v", res.ExtraHeaders)
	}
}

// TestResolveCustomToken 自定义 Token：直接返回自定义内容（覆盖组分配）
func TestResolveCustomToken(t *testing.T) {
	e := newTestDownload(t)
	ctx := context.Background()
	res, err := e.st.DB().Exec(`INSERT INTO custom_subscriptions (slug, user_id, platform_id) VALUES ('custom-1', ?, ?)`, e.user, e.plat)
	if err != nil {
		t.Fatalf("创建自定义失败: %v", err)
	}
	customID, _ := res.LastInsertId()
	if _, err := e.ver.CreateVersion(ctx, version.OwnerCustom, customID, version.BytesContent([]byte("custom-content"))); err != nil {
		t.Fatalf("创建自定义版本失败: %v", err)
	}
	tk := e.mkToken(t, customID, 0)
	res2, entry, err := e.svc.ResolveUserDownload(ctx, tk, "platform-x")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if string(res2.Content) != "custom-content" {
		t.Errorf("应返回自定义内容: %s", res2.Content)
	}
	if entry.Type != "custom" {
		t.Errorf("类型异常: %s", entry.Type)
	}
}

// TestResolveExplicitToken 显式 Token：实时校验管理员，降级后 404（ErrTokenInvalid）
func TestResolveExplicitToken(t *testing.T) {
	e := newTestDownload(t)
	ctx := context.Background()
	tk := e.mkToken(t, 0, e.sub)
	// 用户非管理员 → 拒绝
	if _, _, err := e.svc.ResolveUserDownload(ctx, tk, "platform-x"); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("非管理员显式 Token 应拒绝: %v", err)
	}
	// 提升为管理员 → 成功
	if _, err := e.st.DB().Exec(`UPDATE users SET role = 'admin' WHERE id = ?`, e.user); err != nil {
		t.Fatalf("提升管理员失败: %v", err)
	}
	if _, _, err := e.svc.ResolveUserDownload(ctx, tk, "platform-x"); err != nil {
		t.Errorf("管理员显式 Token 应成功: %v", err)
	}
	// 降级 → 立即失效
	if _, err := e.st.DB().Exec(`UPDATE users SET role = 'user' WHERE id = ?`, e.user); err != nil {
		t.Fatalf("降级失败: %v", err)
	}
	if _, _, err := e.svc.ResolveUserDownload(ctx, tk, "platform-x"); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("降级后显式 Token 应失效: %v", err)
	}
}

// TestURLSlugMismatch URL 平台标识与 Token 不一致 → 与无效 Token 同等对待
func TestURLSlugMismatch(t *testing.T) {
	e := newTestDownload(t)
	ctx := context.Background()
	tk := e.mkToken(t, 0, 0)
	if _, _, err := e.svc.ResolveUserDownload(ctx, tk, "other-platform"); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("标识不一致应返回 ErrTokenInvalid: %v", err)
	}
	if _, _, err := e.svc.ResolveUserDownload(ctx, "no-such-token", "platform-x"); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("无效 Token 应返回 ErrTokenInvalid: %v", err)
	}
}

// TestUnassigned 组未选定（或用户无组）→ ErrUnassigned
func TestUnassigned(t *testing.T) {
	e := newTestDownload(t)
	ctx := context.Background()
	// 移除组选定
	if _, err := e.st.DB().Exec(`DELETE FROM group_selections`); err != nil {
		t.Fatalf("清理选定失败: %v", err)
	}
	tk := e.mkToken(t, 0, 0)
	_, entry, err := e.svc.ResolveUserDownload(ctx, tk, "platform-x")
	if !errors.Is(err, ErrUnassigned) {
		t.Fatalf("未分配应返回 ErrUnassigned: %v", err)
	}
	if entry.FailReason != "unassigned" {
		t.Errorf("失败原因异常: %s", entry.FailReason)
	}
}

// TestWriteAccessLog 成功/失败均写入访问日志（含原因）
func TestWriteAccessLog(t *testing.T) {
	e := newTestDownload(t)
	ctx := context.Background()
	e.svc.WriteAccessLog(ctx, "1.2.3.4", &AccessEntry{UserID: e.user, Type: "subscription", ResourceID: e.sub}, true)
	e.svc.WriteAccessLog(ctx, "5.6.7.8", &AccessEntry{Platform: "platform-x", FailReason: "unassigned"}, false)
	var success, failed int
	if err := e.st.DB().QueryRow(`SELECT COUNT(*) FROM access_logs WHERE status = 'success'`).Scan(&success); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if err := e.st.DB().QueryRow(`SELECT COUNT(*) FROM access_logs WHERE status = 'fail' AND fail_reason = 'unassigned'`).Scan(&failed); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if success != 1 || failed != 1 {
		t.Errorf("访问日志写入异常: success=%d failed=%d", success, failed)
	}
	// resource_slug 转换（订阅标识）
	var slugVal string
	if err := e.st.DB().QueryRow(`SELECT resource_slug FROM access_logs WHERE status = 'success'`).Scan(&slugVal); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if slugVal != "sub-x" {
		t.Errorf("resource_slug 应记订阅标识: %s", slugVal)
	}
}
