package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/config"
	"vpn-sub/internal/dataclear"
	"vpn-sub/internal/emergency"
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
	buf := log.NewRingBuffer()
	streamSvc := log.NewStreamService(buf, log.New("error", "console"))
	srv, err := New(st, cfg, users, log.New("error", "console"), "dev", "off", "0", t.TempDir(), streamSvc)
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

// TestTrustProxyClientIPTiers TRUST_PROXY 三档真实客户端 IP 解析（Design1 §6.4，Build1 Step 7 验收项）：
// auto 信任回环/私有网段转发头；off 忽略转发头；on 全信任
func TestTrustProxyClientIPTiers(t *testing.T) {
	// 构造带回显 ClientIP 的引擎
	newEcho := func(trustProxy string) *gin.Engine {
		e := gin.New()
		if err := applyTrustProxy(e, trustProxy); err != nil {
			t.Fatalf("applyTrustProxy 失败: %v", err)
		}
		e.GET("/ip", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })
		return e
	}
	doIP := func(e *gin.Engine, remoteAddr, xff string) string {
		req := httptest.NewRequest(http.MethodGet, "/ip", nil)
		req.RemoteAddr = remoteAddr
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
		return w.Body.String()
	}

	// auto：回环来源 → 信任转发头
	if got := doIP(newEcho("auto"), "127.0.0.1:1234", "1.2.3.4"); got != "1.2.3.4" {
		t.Errorf("auto+回环应信任转发头: got %s", got)
	}
	// auto：公网来源 → 忽略转发头（防伪造）
	if got := doIP(newEcho("auto"), "203.0.113.5:1234", "1.2.3.4"); got != "203.0.113.5" {
		t.Errorf("auto+公网应忽略转发头: got %s", got)
	}
	// off：回环来源也忽略转发头
	if got := doIP(newEcho("off"), "127.0.0.1:1234", "1.2.3.4"); got != "127.0.0.1" {
		t.Errorf("off 应忽略转发头: got %s", got)
	}
	// on：全信任（公网来源也取转发头）
	if got := doIP(newEcho("on"), "203.0.113.5:1234", "1.2.3.4"); got != "1.2.3.4" {
		t.Errorf("on 应全信任: got %s", got)
	}
}

// TestLocalLoginSwitch 本地登录开关：关闭后 login/register 返回 403（Design1 §3.2）
func TestLocalLoginSwitch(t *testing.T) {
	doLogin := func(srv *Server) int {
		body := strings.NewReader(`{"email":"kyle@example.com","password":"password123"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.Engine().ServeHTTP(w, req)
		return w.Code
	}
	doRegister := func(srv *Server) int {
		body := strings.NewReader(`{"username":"bob","email":"bob@example.com","password":"password123"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.Engine().ServeHTTP(w, req)
		return w.Code
	}

	// 默认开启：登录 401（无该用户，统一措辞）；注册 200（表空例外）
	srv := newTestServer(t)
	if code := doLogin(srv); code != http.StatusUnauthorized {
		t.Errorf("默认开启时登录应走正常校验（401）: %d", code)
	}
	if code := doRegister(srv); code != http.StatusOK {
		t.Errorf("默认开启时表空注册应 200: %d", code)
	}
	// 关闭：login/register 均 403
	srv2 := newTestServer(t)
	cfg2 := srv2.cfg
	if err := cfg2.Set(context.Background(), config.KeyAllowLocalLogin, "false"); err != nil {
		t.Fatalf("写配置失败: %v", err)
	}
	if code := doLogin(srv2); code != http.StatusForbidden {
		t.Errorf("关闭后登录应 403: %d", code)
	}
	if code := doRegister(srv2); code != http.StatusForbidden {
		t.Errorf("关闭后注册应 403: %d", code)
	}
}

// TestEmergencyServer 应急装配级验证（Build3 Step 6）：业务 API 503、/health 503、
// 系统状态返回 emergency 标记、站点信息/应急端点/白名单路径正常
func TestEmergencyServer(t *testing.T) {
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
			id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT NOT NULL, email TEXT UNIQUE,
			role TEXT NOT NULL DEFAULT 'user', user_source TEXT NOT NULL DEFAULT 'local',
			status TEXT NOT NULL DEFAULT 'active');`)},
	}
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	clearSvc := dataclear.NewService(st, t.TempDir(), log.New("error", "console"))
	emSvc := emergency.NewService(emergency.TriggerManual, true, st, cfg, clearSvc, t.TempDir(), "test.db", log.New("error", "console"))
	srv, err := NewEmergency(st, cfg, emSvc, log.New("error", "console"), "dev", "off", "0", t.TempDir())
	if err != nil {
		t.Fatalf("装配应急 server 失败: %v", err)
	}
	e := srv.Engine()
	req := func(method, path string) int {
		r := httptest.NewRequest(method, path, nil)
		w := httptest.NewRecorder()
		e.ServeHTTP(w, r)
		return w.Code
	}
	// 业务 API 503
	if code := req(http.MethodGet, "/api/admin/users"); code != http.StatusServiceUnavailable {
		t.Errorf("业务 API 应 503: %d", code)
	}
	if code := req(http.MethodGet, "/subscriptions/x/download?token=bad"); code != http.StatusServiceUnavailable {
		t.Errorf("下载端点应 503: %d", code)
	}
	// /health 503
	if code := req(http.MethodGet, "/health"); code != http.StatusServiceUnavailable {
		t.Errorf("/health 应 503: %d", code)
	}
	// 系统状态：emergency 标记 + 触发原因
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/system/status", nil))
	var status struct {
		Code int `json:"code"`
		Data struct {
			Emergency      bool   `json:"emergency"`
			EmergencyReason string `json:"emergency_reason"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("解析状态失败: %v", err)
	}
	if !status.Data.Emergency || status.Data.EmergencyReason != "manual" {
		t.Errorf("应急标记异常: %+v", status.Data)
	}
	// 站点信息正常
	if code := req(http.MethodGet, "/api/site/info"); code != http.StatusOK {
		t.Errorf("站点信息应 200: %d", code)
	}
	// SPA 回退（/emergency 前端路由）可加载
	if code := req(http.MethodGet, "/emergency"); code != http.StatusOK {
		t.Errorf("SPA 回退应 200: %d", code)
	}
	// 白名单判定（单元级）
	if isEmergencyAllowed("GET", "/api/system/status") != true || isEmergencyAllowed("POST", "/api/emergency/verify") != true ||
		isEmergencyAllowed("GET", "/assets/index.js") != true || isEmergencyAllowed("GET", "/api/auth/login") != false {
		t.Error("白名单判定异常")
	}
}
