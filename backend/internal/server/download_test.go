package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/internal/user"
)

// downloadTestFS 下载限流测试迁移集（含全部相关表）
func downloadTestFS() fstest.MapFS {
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
				is_default INTEGER NOT NULL DEFAULT 0, default_quota REAL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS platforms (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
				product_type TEXT NOT NULL DEFAULT 'yaml',
				description TEXT NOT NULL DEFAULT '', schemes TEXT NOT NULL DEFAULT '[]',
				extra_headers TEXT NOT NULL DEFAULT '{}', installer_files TEXT NOT NULL DEFAULT '[]', installer_urls TEXT NOT NULL DEFAULT '[]',
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"1002_subscriptions_versions.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS subscriptions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
				platform_id INTEGER NOT NULL, current_version INTEGER NOT NULL DEFAULT 0,
				product_type TEXT NOT NULL DEFAULT 'yaml',
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS versions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				owner_type TEXT NOT NULL CHECK (owner_type IN ('subscription','rule','custom','share')),
				owner_id INTEGER NOT NULL, version_no INTEGER NOT NULL, file_path TEXT NOT NULL,
								file_name TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				UNIQUE (owner_type, owner_id, version_no));`)},
		"1009_xray.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS nodes (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				source TEXT NOT NULL, name TEXT NOT NULL UNIQUE,
				instance_id INTEGER);
			CREATE TABLE IF NOT EXISTS group_nodes (
				group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
				node_id INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
				sort_order INTEGER NOT NULL DEFAULT 0,
				PRIMARY KEY (group_id, node_id));
			CREATE TABLE IF NOT EXISTS assembly_blueprints (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				version_id INTEGER NOT NULL UNIQUE,
				target_syntax TEXT NOT NULL);`)},
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
		"1006_rules.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS rules (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE,
				name TEXT NOT NULL,
				current_version INTEGER NOT NULL DEFAULT 0,
				is_home_default INTEGER NOT NULL DEFAULT 0);
			CREATE TABLE IF NOT EXISTS rule_tokens (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				token TEXT NOT NULL UNIQUE,
				rule_id INTEGER NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				refreshed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
	}
}

// newDownloadTestServer 构造含下载路由的测试 server
func newDownloadTestServer(t *testing.T) *Server {
	t.Helper()
	dataDir := t.TempDir()
	st, err := store.Open(dataDir, "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), downloadTestFS()); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	users := user.NewService(st, cfg, log.New("error", "console"))
	streamSvc := log.NewStreamService(log.NewRingBuffer(), log.New("error", "console"))
	srv, err := New(st, cfg, users, log.New("error", "console"), "dev", mustPolicy(t, "off"), "0", dataDir, streamSvc)
	if err != nil {
		t.Fatalf("装配 server 失败: %v", err)
	}
	return srv
}

// TestDownloadRateLimit 下载限流：默认 20/min，第 21 次请求 429 + Retry-After
func TestDownloadRateLimit(t *testing.T) {
	srv := newDownloadTestServer(t)
	path := "/subscriptions/platform-x/download?token=bad"
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "192.168.1.10:1234"
		w := httptest.NewRecorder()
		srv.Engine().ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("第 %d 次请求应为 404（无效 Token）: %d", i+1, w.Code)
		}
	}
	// 第 21 次 → 429 + Retry-After
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = "192.168.1.10:1234"
	w := httptest.NewRecorder()
	srv.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("第 21 次请求应为 429: %d", w.Code)
	}
	if ra := w.Header().Get("Retry-After"); ra == "" {
		t.Error("429 响应应携带 Retry-After")
	}
	// 不同 IP 不受影响
	req2 := httptest.NewRequest(http.MethodGet, path, nil)
	req2.RemoteAddr = "192.168.1.20:1234"
	w2 := httptest.NewRecorder()
	srv.Engine().ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Errorf("不同 IP 应不受限流影响: %d", w2.Code)
	}
}

// TestPreviewRequiresSession 会话凭据预览：无凭据 401
func TestPreviewRequiresSession(t *testing.T) {
	srv := newDownloadTestServer(t)
	w := doReq(t, srv, http.MethodGet, "/api/subscriptions/preview?platform=platform-x")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("无凭据预览应 401: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "会话凭据") {
		t.Errorf("401 文案异常: %s", w.Body.String())
	}
}

// TestPreviewNoVersion 无激活版本订阅：预览返回 HTTP 200 注释块；有版本后回归 200 正文（Design2 §4.4）
func TestPreviewNoVersion(t *testing.T) {
	t.Helper()
	dataDir := t.TempDir()
	st, err := store.Open(dataDir, "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), downloadTestFS()); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	users := user.NewService(st, cfg, log.New("error", "console"))
	streamSvc := log.NewStreamService(log.NewRingBuffer(), log.New("error", "console"))
	srv, err := New(st, cfg, users, log.New("error", "console"), "dev", mustPolicy(t, "off"), "0", dataDir, streamSvc)
	if err != nil {
		t.Fatalf("装配 server 失败: %v", err)
	}
	ctx := context.Background()
	token := regUser(t, srv, "admin", "admin@x.com", "password123")
	// 播种平台 + 无版本订阅（current_version=0）
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO platforms (slug, name) VALUES ('platform-x','平台X')`); err != nil {
		t.Fatalf("播种平台失败: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO subscriptions (slug, name, platform_id) VALUES ('subscription-a','订阅A',1)`); err != nil {
		t.Fatalf("播种订阅失败: %v", err)
	}
	// 场景 1：无激活版本预览 → 200 + 纯文本注释块
	w := profileReq(t, srv, http.MethodGet, "/api/subscriptions/preview?platform=platform-x", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("无激活版本预览应 200: %d %s", w.Code, w.Body.String())
	}
	if strings.TrimSpace(w.Body.String()) != "# error: no active version" {
		t.Fatalf("无激活版本注释块异常: %s", w.Body.String())
	}
	// 访问日志应记 fail_reason=no_active_version
	var reason string
	if err := st.DB().QueryRowContext(ctx, `SELECT fail_reason FROM access_logs ORDER BY id DESC LIMIT 1`).Scan(&reason); err != nil {
		t.Fatalf("查询访问日志失败: %v", err)
	}
	if reason != "no_active_version" {
		t.Errorf("访问日志 fail_reason 应为 no_active_version: %q", reason)
	}
	// 场景 2：补建版本后预览回归 200 正文
	if err := os.MkdirAll(filepath.Join(dataDir, "contents", "subscription", "1"), 0o755); err != nil {
		t.Fatalf("建版本目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "contents", "subscription", "1", "v1"), []byte("proxies: []\n"), 0o644); err != nil {
		t.Fatalf("写版本文件失败: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO versions (owner_type, owner_id, version_no, file_path, file_name) VALUES ('subscription',1,1,'subscription/1/v1','sub.yaml')`); err != nil {
		t.Fatalf("插入版本记录失败: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx, `UPDATE subscriptions SET current_version = 1 WHERE id = 1`); err != nil {
		t.Fatalf("更新当前版本失败: %v", err)
	}
	w2 := profileReq(t, srv, http.MethodGet, "/api/subscriptions/preview?platform=platform-x", token, nil)
	if w2.Code != http.StatusOK {
		t.Fatalf("有版本订阅预览应 200: %d %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "proxies:") {
		t.Errorf("预览内容异常: %s", w2.Body.String())
	}
}
