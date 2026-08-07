package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/internal/user"
)

// newTestServer 构造测试用 server（临时库）
func newTestServer(t *testing.T) *Server {
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
	users := user.NewService(st, cfg, log.New("error", "console"))
	srv, err := New(st, cfg, users, log.New("error", "console"), "dev", "off", "0", t.TempDir())
	if err != nil {
		t.Fatalf("装配 server 失败: %v", err)
	}
	return srv
}

// doReq 执行测试请求并返回响应
func doReq(t *testing.T, srv *Server, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	srv.Engine().ServeHTTP(w, req)
	return w
}

// TestHealth 健康检查返回 {"status":"ok"}
func TestHealth(t *testing.T) {
	srv := newTestServer(t)
	w := doReq(t, srv, http.MethodGet, "/health")
	if w.Code != http.StatusOK {
		t.Fatalf("health 状态码异常: %d", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("health 响应异常: %v", body)
	}
}

// TestSystemStatus 系统状态返回 configured/app_mode/emergency
func TestSystemStatus(t *testing.T) {
	srv := newTestServer(t)
	w := doReq(t, srv, http.MethodGet, "/api/system/status")
	if w.Code != http.StatusOK {
		t.Fatalf("status 状态码异常: %d", w.Code)
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Configured bool   `json:"configured"`
			AppMode    string `json:"app_mode"`
			Emergency  bool   `json:"emergency"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("业务码异常: %d", resp.Code)
	}
	if resp.Data.Configured || resp.Data.Emergency {
		t.Error("全新库 configured/emergency 应为 false")
	}
	if resp.Data.AppMode != "dev" {
		t.Errorf("app_mode 异常: %s", resp.Data.AppMode)
	}
}

// TestResponseStructure 统一响应结构（错误响应）
func TestResponseStructure(t *testing.T) {
	// 直接构造 gin 上下文调 Fail 验证结构
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/system/status", nil)
	Fail(c, http.StatusBadRequest, "参数校验失败")
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if resp.Code != http.StatusBadRequest || resp.Message != "参数校验失败" {
		t.Errorf("错误响应结构异常: %+v", resp)
	}
}

// TestPanicRecovery panic 恢复返回 500 通用信息
func TestPanicRecovery(t *testing.T) {
	srv := newTestServer(t)
	srv.Engine().GET("/panic-test", func(c *gin.Context) {
		panic("boom")
	})
	w := doReq(t, srv, http.MethodGet, "/panic-test")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("panic 应返回 500: %d", w.Code)
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if resp.Message != "服务器内部错误" {
		t.Errorf("5xx 应对外脱敏: %+v", resp)
	}
}
