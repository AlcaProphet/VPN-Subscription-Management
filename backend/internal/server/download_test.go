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
				is_default INTEGER NOT NULL DEFAULT 0, needs_reselect INTEGER NOT NULL DEFAULT 0,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS platforms (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '', schemes TEXT NOT NULL DEFAULT '[]',
				extra_headers TEXT NOT NULL DEFAULT '{}', installer_file TEXT, installer_url TEXT,
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
	}
}

// newDownloadTestServer 构造含下载路由的测试 server
func newDownloadTestServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
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
	srv, err := New(st, cfg, users, log.New("error", "console"), "dev", "off", "0", t.TempDir(), streamSvc)
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

// TestPreviewNoVersion 无版本订阅：管理员预览返回 404 且访问日志记 version_missing；有版本后回归 200（R07-05）
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
	srv, err := New(st, cfg, users, log.New("error", "console"), "dev", "off", "0", dataDir, streamSvc)
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
	// 场景 1：管理员预览无版本订阅 → 404（原 500，R07-05）
	w := profileReq(t, srv, http.MethodGet, "/api/subscriptions/preview?platform=platform-x&subscription_id=1", token, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("无版本订阅预览应 404: %d %s", w.Code, w.Body.String())
	}
	// 访问日志应记 fail_reason=version_missing
	var reason string
	if err := st.DB().QueryRowContext(ctx, `SELECT fail_reason FROM access_logs ORDER BY id DESC LIMIT 1`).Scan(&reason); err != nil {
		t.Fatalf("查询访问日志失败: %v", err)
	}
	if reason != "version_missing" {
		t.Errorf("访问日志 fail_reason 应为 version_missing: %q", reason)
	}
	// 场景 2：补建版本后预览回归 200
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
	w2 := profileReq(t, srv, http.MethodGet, "/api/subscriptions/preview?platform=platform-x&subscription_id=1", token, nil)
	if w2.Code != http.StatusOK {
		t.Fatalf("有版本订阅预览应 200: %d %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "proxies:") {
		t.Errorf("预览内容异常: %s", w2.Body.String())
	}
}
