package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/config"
	applog "vpn-sub/internal/log"
	"vpn-sub/internal/providertraffic"
	"vpn-sub/internal/store"
	"vpn-sub/internal/user"
	"vpn-sub/migrations"
)

// providerTrafficTestSettings 是管理端 Settings 响应。
type providerTrafficTestSettings struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Available bool                         `json:"available"`
		Enabled   bool                         `json:"enabled"`
		Providers []providerTrafficTestBinding `json:"providers"`
	} `json:"data"`
}

type providerTrafficTestBinding struct {
	Provider             string                     `json:"provider"`
	Enabled              bool                       `json:"enabled"`
	InstanceID           *string                    `json:"instance_id"`
	Revision             *string                    `json:"revision"`
	CredentialConfigured bool                       `json:"credential_configured"`
	Result               *providerTrafficTestResult `json:"result"`
}

type providerTrafficTestResult struct {
	Provider  string   `json:"provider"`
	Status    string   `json:"status"`
	Remaining *float64 `json:"remaining"`
	Unit      string   `json:"unit"`
	SampledAt string   `json:"sampled_at"`
	Message   string   `json:"message"`
	Hint      string   `json:"hint"`
}

type providerTrafficTestHome struct {
	Code int `json:"code"`
	Data struct {
		List  []providerTrafficTestResult `json:"list"`
		Total int                         `json:"total"`
	} `json:"data"`
}

// newProviderTrafficTestServer 构造含完整迁移的测试 server；采集器未启动，测试可按需 Start。
func newProviderTrafficTestServer(t *testing.T, level string) (*Server, *applog.RingBuffer) {
	t.Helper()
	dataDir := t.TempDir()
	st, err := store.Open(dataDir, "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), migrations.FS); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	baseLogger := applog.New("error", "console")
	cfg := config.NewService(st, baseLogger)
	if err := cfg.Set(context.Background(), config.KeySigningKey, "test-signing-key-0123456789abcdef"); err != nil {
		t.Fatalf("写入签名密钥失败: %v", err)
	}
	if err := cfg.Set(context.Background(), config.KeyConfigured, "true"); err != nil {
		t.Fatalf("置位 Setup 状态失败: %v", err)
	}
	users := user.NewService(st, cfg, baseLogger)
	buf := applog.NewRingBuffer()
	runtime := applog.NewRuntime(level, "json", buf)
	streamSvc := applog.NewStreamService(buf, runtime.Logger)
	srv, err := New(st, cfg, users, runtime, "dev", mustPolicy(t, "off"), "0", dataDir, streamSvc)
	if err != nil {
		t.Fatalf("装配 server 失败: %v", err)
	}
	t.Cleanup(func() {
		if srv.providerTraffic != nil {
			srv.providerTraffic.Stop()
		}
		srv.mailDispatcher.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.mailResultLog.Stop(ctx)
	})
	return srv, buf
}

// newProviderTrafficTestConfig 构造仅含 system_config 的轻量配置源，供 stub 路由测试实时读取模式。
func newProviderTrafficTestConfig(t *testing.T) *config.Service {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开配置测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	fsys := fstest.MapFS{
		"0001_config.sql": &fstest.MapFile{Data: []byte(
			`CREATE TABLE system_config (key TEXT PRIMARY KEY, value TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
	}
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("配置测试库迁移失败: %v", err)
	}
	cfg := config.NewService(st, applog.New("error", "console"))
	if err := cfg.Set(context.Background(), config.KeyConfigured, "true"); err != nil {
		t.Fatalf("置位 Setup 状态失败: %v", err)
	}
	return cfg
}

// providerTrafficStub 同时实现设置读取与采集器接口，用于可控结果与错误映射测试。
type providerTrafficStub struct {
	getSettings  func(context.Context) (providertraffic.Settings, error)
	results      func(context.Context) ([]providertraffic.Result, error)
	resultFor    func(providertraffic.Provider, string) *providertraffic.Result
	setEnabled   func(context.Context, bool) (providertraffic.Settings, error)
	saveBinding  func(context.Context, providertraffic.SaveBindingInput) (providertraffic.Settings, error)
	clearBinding func(context.Context, providertraffic.Provider, string) (providertraffic.Settings, error)
	manual       func(context.Context, providertraffic.Provider) (providertraffic.Result, error)
	manualCalls  int
}

func defaultProviderTrafficTestSettings() providertraffic.Settings {
	return providertraffic.Settings{Providers: []providertraffic.ProviderBinding{
		{Provider: providertraffic.ProviderKiwiVM},
		{Provider: providertraffic.ProviderVPS},
	}}
}

func (s *providerTrafficStub) GetSettings(ctx context.Context) (providertraffic.Settings, error) {
	if s.getSettings != nil {
		return s.getSettings(ctx)
	}
	return defaultProviderTrafficTestSettings(), nil
}

func (s *providerTrafficStub) Results(ctx context.Context) ([]providertraffic.Result, error) {
	if s.results != nil {
		return s.results(ctx)
	}
	return []providertraffic.Result{}, nil
}

func (s *providerTrafficStub) ResultForRevision(provider providertraffic.Provider, revision string) *providertraffic.Result {
	if s.resultFor != nil {
		return s.resultFor(provider, revision)
	}
	return nil
}

func (s *providerTrafficStub) SetEnabled(ctx context.Context, enabled bool) (providertraffic.Settings, error) {
	if s.setEnabled != nil {
		return s.setEnabled(ctx, enabled)
	}
	return defaultProviderTrafficTestSettings(), nil
}

func (s *providerTrafficStub) SaveBinding(ctx context.Context, in providertraffic.SaveBindingInput) (providertraffic.Settings, error) {
	if s.saveBinding != nil {
		return s.saveBinding(ctx, in)
	}
	return defaultProviderTrafficTestSettings(), nil
}

func (s *providerTrafficStub) ClearBinding(ctx context.Context, provider providertraffic.Provider, revision string) (providertraffic.Settings, error) {
	if s.clearBinding != nil {
		return s.clearBinding(ctx, provider, revision)
	}
	return defaultProviderTrafficTestSettings(), nil
}

func (s *providerTrafficStub) ManualRefresh(ctx context.Context, provider providertraffic.Provider) (providertraffic.Result, error) {
	s.manualCalls++
	if s.manual != nil {
		return s.manual(ctx, provider)
	}
	return providertraffic.Result{}, errors.New("未设置手动采集响应")
}

func newProviderTrafficStubEngine(t *testing.T, stub *providerTrafficStub, cfg *config.Service) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(bodyLimitMiddleware(cfg))
	noop := func(c *gin.Context) { c.Next() }
	RegisterProviderTrafficRoutes(engine, &ProviderTrafficHandler{
		settingsSvc: stub,
		collector:   stub,
		cfg:         cfg,
	}, noop, noop)
	return engine
}

func providerTrafficStubReq(t *testing.T, engine *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader = http.NoBody
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("序列化请求失败: %v", err)
		}
		reader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func providerTrafficGetSettings(t *testing.T, srv *Server, token string) *httptest.ResponseRecorder {
	t.Helper()
	return profileReq(t, srv, http.MethodGet, "/api/admin/settings/provider-traffic", token, nil)
}

func providerTrafficPutEnabled(t *testing.T, srv *Server, token string, enabled bool) *httptest.ResponseRecorder {
	t.Helper()
	return profileReq(t, srv, http.MethodPut, "/api/admin/settings/provider-traffic", token,
		map[string]any{"enabled": enabled})
}

func providerTrafficPutBinding(t *testing.T, srv *Server, token, provider string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return profileReq(t, srv, http.MethodPut, "/api/admin/settings/provider-traffic/"+provider, token, body)
}

func providerTrafficDeleteBinding(t *testing.T, srv *Server, token, provider string, revision any) *httptest.ResponseRecorder {
	t.Helper()
	return profileReq(t, srv, http.MethodDelete, "/api/admin/settings/provider-traffic/"+provider, token,
		map[string]any{"revision": revision})
}

func providerTrafficRefresh(t *testing.T, srv *Server, token, provider string) *httptest.ResponseRecorder {
	t.Helper()
	return profileReq(t, srv, http.MethodPost,
		"/api/admin/settings/provider-traffic/"+provider+"/refresh", token, nil)
}

func providerTrafficHome(t *testing.T, srv *Server, token string) *httptest.ResponseRecorder {
	t.Helper()
	return profileReq(t, srv, http.MethodGet, "/api/home/provider-traffic", token, nil)
}

func decodeProviderTrafficSettings(t *testing.T, w *httptest.ResponseRecorder) providerTrafficTestSettings {
	t.Helper()
	var resp providerTrafficTestSettings
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析 Settings 响应失败: %v body=%s", err, w.Body.String())
	}
	return resp
}

func decodeProviderTrafficHome(t *testing.T, w *httptest.ResponseRecorder) providerTrafficTestHome {
	t.Helper()
	var resp providerTrafficTestHome
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析首页响应失败: %v body=%s", err, w.Body.String())
	}
	return resp
}

func providerTrafficRequireStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("状态码 = %d，期望 %d: %s", w.Code, want, w.Body.String())
	}
}

func assertProviderTrafficNoStore(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q，期望 no-store", got)
	}
}

func providerTrafficRevisionFor(t *testing.T, resp providerTrafficTestSettings, provider string) string {
	t.Helper()
	for _, item := range resp.Data.Providers {
		if item.Provider == provider {
			if item.Revision == nil || *item.Revision == "" {
				t.Fatalf("%s 缺少绑定修订: %+v", provider, item)
			}
			return *item.Revision
		}
	}
	t.Fatalf("响应缺少平台 %s: %+v", provider, resp.Data.Providers)
	return ""
}

// TestProviderTrafficPermissions 覆盖管理员、普通用户、游客、停用账号及实时角色变化。
func TestProviderTrafficPermissions(t *testing.T) {
	srv, _ := newProviderTrafficTestServer(t, "error")
	adminToken := regUser(t, srv, "pt-admin", "pt-admin@example.com", "password123")
	if err := srv.cfg.Set(context.Background(), config.KeyAllowSelfreg, "true"); err != nil {
		t.Fatalf("开启自注册失败: %v", err)
	}
	userToken := regUser(t, srv, "pt-user", "pt-user@example.com", "password123")

	t.Run("guest", func(t *testing.T) {
		w := providerTrafficHome(t, srv, "")
		providerTrafficRequireStatus(t, w, http.StatusUnauthorized)
		assertProviderTrafficNoStore(t, w)

		w = providerTrafficGetSettings(t, srv, "")
		providerTrafficRequireStatus(t, w, http.StatusUnauthorized)
		assertProviderTrafficNoStore(t, w)
	})

	t.Run("normal user", func(t *testing.T) {
		w := providerTrafficHome(t, srv, userToken)
		providerTrafficRequireStatus(t, w, http.StatusOK)
		assertProviderTrafficNoStore(t, w)

		w = providerTrafficGetSettings(t, srv, userToken)
		providerTrafficRequireStatus(t, w, http.StatusForbidden)
		assertProviderTrafficNoStore(t, w)
	})

	t.Run("admin", func(t *testing.T) {
		w := providerTrafficGetSettings(t, srv, adminToken)
		providerTrafficRequireStatus(t, w, http.StatusOK)
		assertProviderTrafficNoStore(t, w)
	})

	t.Run("live role promotion", func(t *testing.T) {
		if _, err := srv.store.DB().Exec(`UPDATE users SET role = 'admin' WHERE email = 'pt-user@example.com'`); err != nil {
			t.Fatalf("提升测试用户失败: %v", err)
		}
		w := providerTrafficGetSettings(t, srv, userToken)
		providerTrafficRequireStatus(t, w, http.StatusOK)
	})

	t.Run("disabled account", func(t *testing.T) {
		if _, err := srv.store.DB().Exec(`UPDATE users SET status = 'disabled' WHERE email = 'pt-user@example.com'`); err != nil {
			t.Fatalf("停用测试用户失败: %v", err)
		}
		for _, path := range []string{"/api/home/provider-traffic", "/api/admin/settings/provider-traffic"} {
			w := profileReq(t, srv, http.MethodGet, path, userToken, nil)
			providerTrafficRequireStatus(t, w, http.StatusUnauthorized)
			assertProviderTrafficNoStore(t, w)
		}
	})
}

// TestProviderTrafficHomeVisibility 覆盖 Setup/高级模式、两级开关组合、等待行与两家顺序。
func TestProviderTrafficHomeVisibility(t *testing.T) {
	srv, _ := newProviderTrafficTestServer(t, "error")
	adminToken := regUser(t, srv, "pt-home-admin", "pt-home-admin@example.com", "password123")
	ctx := context.Background()

	home := providerTrafficHome(t, srv, adminToken)
	providerTrafficRequireStatus(t, home, http.StatusOK)
	if resp := decodeProviderTrafficHome(t, home); len(resp.Data.List) != 0 || resp.Data.Total != 0 {
		t.Fatalf("默认关闭应为空列表: %+v", resp)
	}

	providerTrafficRequireStatus(t, providerTrafficPutEnabled(t, srv, adminToken, true), http.StatusOK)
	if resp := decodeProviderTrafficHome(t, providerTrafficHome(t, srv, adminToken)); len(resp.Data.List) != 0 {
		t.Fatalf("总开关开启但未绑定时应为空列表: %+v", resp)
	}

	bound := decodeProviderTrafficSettings(t, providerTrafficPutBinding(t, srv, adminToken, "kiwivm", map[string]any{
		"enabled": true, "instance_id": "1001", "revision": nil, "credential": "synthetic-kiwivm-key",
	}))
	kiwiRevision := providerTrafficRevisionFor(t, bound, "kiwivm")
	home = providerTrafficHome(t, srv, adminToken)
	providerTrafficRequireStatus(t, home, http.StatusOK)
	homeResp := decodeProviderTrafficHome(t, home)
	if len(homeResp.Data.List) != 1 || homeResp.Data.Total != 1 {
		t.Fatalf("启用绑定后应返回一家等待行: %+v", homeResp)
	}
	waiting := homeResp.Data.List[0]
	if waiting.Provider != "kiwivm" || waiting.Status != "waiting" || waiting.Remaining != nil || waiting.Unit != "" ||
		waiting.SampledAt != "" || waiting.Message != "" || waiting.Hint != "" {
		t.Fatalf("等待行字段异常: %+v", waiting)
	}
	if strings.Contains(home.Body.String(), "1001") || strings.Contains(home.Body.String(), kiwiRevision) {
		t.Fatalf("首页不得包含实例或修订: %s", home.Body.String())
	}

	providerTrafficRequireStatus(t, providerTrafficPutBinding(t, srv, adminToken, "kiwivm", map[string]any{
		"enabled": false, "instance_id": "1001", "revision": kiwiRevision,
	}), http.StatusOK)
	if resp := decodeProviderTrafficHome(t, providerTrafficHome(t, srv, adminToken)); len(resp.Data.List) != 0 {
		t.Fatalf("单家关闭后应移除该行: %+v", resp)
	}

	providerTrafficRequireStatus(t, providerTrafficPutBinding(t, srv, adminToken, "kiwivm", map[string]any{
		"enabled": true, "instance_id": "1001", "revision": kiwiRevision,
	}), http.StatusOK)
	if resp := decodeProviderTrafficHome(t, providerTrafficHome(t, srv, adminToken)); len(resp.Data.List) != 1 {
		t.Fatalf("重新启用后应恢复等待行: %+v", resp)
	}

	providerTrafficRequireStatus(t, providerTrafficPutEnabled(t, srv, adminToken, false), http.StatusOK)
	if resp := decodeProviderTrafficHome(t, providerTrafficHome(t, srv, adminToken)); len(resp.Data.List) != 0 {
		t.Fatalf("总开关关闭后应为空列表: %+v", resp)
	}

	providerTrafficRequireStatus(t, providerTrafficPutEnabled(t, srv, adminToken, true), http.StatusOK)
	if resp := decodeProviderTrafficHome(t, providerTrafficHome(t, srv, adminToken)); len(resp.Data.List) != 1 {
		t.Fatalf("重新开启总开关后应恢复等待行: %+v", resp)
	}

	vpsBound := decodeProviderTrafficSettings(t, providerTrafficPutBinding(t, srv, adminToken, "vps", map[string]any{
		"enabled": true, "instance_id": "2002", "revision": nil, "credential": "synthetic-vps-refresh",
	}))
	vpsRevision := providerTrafficRevisionFor(t, vpsBound, "vps")
	homeResp = decodeProviderTrafficHome(t, providerTrafficHome(t, srv, adminToken))
	if len(homeResp.Data.List) != 2 || homeResp.Data.List[0].Provider != "kiwivm" || homeResp.Data.List[1].Provider != "vps" {
		t.Fatalf("两家固定顺序异常: %+v", homeResp.Data.List)
	}

	providerTrafficRequireStatus(t, providerTrafficDeleteBinding(t, srv, adminToken, "vps", vpsRevision), http.StatusOK)
	if resp := decodeProviderTrafficHome(t, providerTrafficHome(t, srv, adminToken)); len(resp.Data.List) != 1 {
		t.Fatalf("解除一家后应保留另一家: %+v", resp)
	}

	if err := srv.cfg.Set(ctx, config.KeyAdvancedMode, "true"); err != nil {
		t.Fatalf("开启高级模式失败: %v", err)
	}
	if resp := decodeProviderTrafficHome(t, providerTrafficHome(t, srv, adminToken)); len(resp.Data.List) != 0 {
		t.Fatalf("高级模式首页应为空: %+v", resp)
	}
	if err := srv.cfg.Set(ctx, config.KeyAdvancedMode, "false"); err != nil {
		t.Fatalf("关闭高级模式失败: %v", err)
	}
	if resp := decodeProviderTrafficHome(t, providerTrafficHome(t, srv, adminToken)); len(resp.Data.List) != 1 {
		t.Fatalf("切回基础模式后应恢复本地结果行: %+v", resp)
	}

	if err := srv.cfg.Set(ctx, config.KeyConfigured, "false"); err != nil {
		t.Fatalf("模拟 Setup 未完成失败: %v", err)
	}
	if resp := decodeProviderTrafficHome(t, providerTrafficHome(t, srv, adminToken)); len(resp.Data.List) != 0 {
		t.Fatalf("Setup 未完成首页应为空: %+v", resp)
	}
	if err := srv.cfg.Set(ctx, config.KeyConfigured, "true"); err != nil {
		t.Fatalf("恢复 Setup 状态失败: %v", err)
	}
}

// TestProviderTrafficSettingsRead 覆盖默认两家结构、总开关、绑定字段、凭据标记与解除绑定。
func TestProviderTrafficSettingsRead(t *testing.T) {
	srv, _ := newProviderTrafficTestServer(t, "error")
	adminToken := regUser(t, srv, "pt-settings-admin", "pt-settings-admin@example.com", "password123")

	w := providerTrafficGetSettings(t, srv, adminToken)
	providerTrafficRequireStatus(t, w, http.StatusOK)
	resp := decodeProviderTrafficSettings(t, w)
	if !resp.Data.Available || resp.Data.Enabled || len(resp.Data.Providers) != 2 {
		t.Fatalf("默认设置异常: %+v", resp.Data)
	}
	for i, provider := range []string{"kiwivm", "vps"} {
		item := resp.Data.Providers[i]
		if item.Provider != provider || item.Enabled || item.InstanceID != nil || item.Revision != nil ||
			item.CredentialConfigured || item.Result != nil {
			t.Fatalf("默认 %s 绑定异常: %+v", provider, item)
		}
	}

	providerTrafficRequireStatus(t, providerTrafficPutEnabled(t, srv, adminToken, true), http.StatusOK)
	resp = decodeProviderTrafficSettings(t, providerTrafficGetSettings(t, srv, adminToken))
	if !resp.Data.Enabled {
		t.Fatalf("总开关未保存: %+v", resp.Data)
	}

	bound := decodeProviderTrafficSettings(t, providerTrafficPutBinding(t, srv, adminToken, "kiwivm", map[string]any{
		"enabled": false, "instance_id": "1001", "revision": nil, "credential": "synthetic-kiwivm-key",
	}))
	rev := providerTrafficRevisionFor(t, bound, "kiwivm")
	resp = decodeProviderTrafficSettings(t, providerTrafficGetSettings(t, srv, adminToken))
	item := resp.Data.Providers[0]
	if item.Provider != "kiwivm" || item.Enabled || item.InstanceID == nil || *item.InstanceID != "1001" ||
		item.Revision == nil || *item.Revision != rev || !item.CredentialConfigured || item.Result != nil {
		t.Fatalf("绑定回读异常: %+v", item)
	}

	providerTrafficRequireStatus(t, providerTrafficDeleteBinding(t, srv, adminToken, "kiwivm", rev), http.StatusOK)
	resp = decodeProviderTrafficSettings(t, providerTrafficGetSettings(t, srv, adminToken))
	item = resp.Data.Providers[0]
	if item.InstanceID != nil || item.Revision != nil || item.CredentialConfigured || item.Enabled || item.Result != nil {
		t.Fatalf("解除绑定后字段异常: %+v", item)
	}
}

// TestProviderTrafficBindingValidation 覆盖必填布尔、实例、修订、凭据省略/null/空值与错误码。
func TestProviderTrafficBindingValidation(t *testing.T) {
	srv, _ := newProviderTrafficTestServer(t, "error")
	adminToken := regUser(t, srv, "pt-validate-admin", "pt-validate-admin@example.com", "password123")
	pathK := "/api/admin/settings/provider-traffic/kiwivm"

	for _, tc := range []struct {
		name string
		body map[string]any
	}{
		{
			name: "missing enabled",
			body: map[string]any{"instance_id": "1001", "revision": nil, "credential": "synthetic-kiwivm-key"},
		},
		{
			name: "null enabled",
			body: map[string]any{"enabled": nil, "instance_id": "1001", "revision": nil, "credential": "synthetic-kiwivm-key"},
		},
		{
			name: "string enabled",
			body: map[string]any{"enabled": "false", "instance_id": "1001", "revision": nil, "credential": "synthetic-kiwivm-key"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := providerTrafficPutBinding(t, srv, adminToken, "kiwivm", tc.body)
			providerTrafficRequireStatus(t, w, http.StatusBadRequest)
			assertProviderTrafficNoStore(t, w)
		})
	}

	for name, body := range map[string]map[string]any{
		"missing instance": {"enabled": true, "revision": nil, "credential": "synthetic-kiwivm-key"},
		"null instance":    {"enabled": true, "instance_id": nil, "revision": nil, "credential": "synthetic-kiwivm-key"},
		"invalid zero":     {"enabled": true, "instance_id": "0", "revision": nil, "credential": "synthetic-kiwivm-key"},
		"leading zero":     {"enabled": true, "instance_id": "0012", "revision": nil, "credential": "synthetic-kiwivm-key"},
		"non digit":        {"enabled": true, "instance_id": "12a", "revision": nil, "credential": "synthetic-kiwivm-key"},
	} {
		t.Run(name, func(t *testing.T) {
			w := providerTrafficPutBinding(t, srv, adminToken, "kiwivm", body)
			providerTrafficRequireStatus(t, w, http.StatusBadRequest)
		})
	}

	t.Run("missing revision", func(t *testing.T) {
		w := profileReq(t, srv, http.MethodPut, pathK, adminToken,
			map[string]any{"enabled": true, "instance_id": "1001", "credential": "synthetic-kiwivm-key"})
		providerTrafficRequireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("first bind requires null revision", func(t *testing.T) {
		w := providerTrafficPutBinding(t, srv, adminToken, "kiwivm", map[string]any{
			"enabled": true, "instance_id": "1001", "revision": "unexpected", "credential": "synthetic-kiwivm-key",
		})
		providerTrafficRequireStatus(t, w, http.StatusConflict)
	})

	for name, body := range map[string]map[string]any{
		"omitted credential": {"enabled": true, "instance_id": "1001", "revision": nil},
		"null credential":    {"enabled": true, "instance_id": "1001", "revision": nil, "credential": nil},
		"empty credential":   {"enabled": true, "instance_id": "1001", "revision": nil, "credential": ""},
		"blank credential":   {"enabled": true, "instance_id": "1001", "revision": nil, "credential": "   "},
	} {
		t.Run("first bind "+name, func(t *testing.T) {
			w := providerTrafficPutBinding(t, srv, adminToken, "kiwivm", body)
			providerTrafficRequireStatus(t, w, http.StatusBadRequest)
		})
	}

	first := decodeProviderTrafficSettings(t, providerTrafficPutBinding(t, srv, adminToken, "kiwivm", map[string]any{
		"enabled": true, "instance_id": "1001", "revision": nil, "credential": "synthetic-kiwivm-key",
	}))
	revision1 := providerTrafficRevisionFor(t, first, "kiwivm")
	if !first.Data.Providers[0].CredentialConfigured {
		t.Fatalf("首次绑定应标记凭据已配置: %+v", first.Data.Providers[0])
	}

	kept := decodeProviderTrafficSettings(t, providerTrafficPutBinding(t, srv, adminToken, "kiwivm", map[string]any{
		"enabled": false, "instance_id": "1001", "revision": revision1,
	}))
	if revisionKept := providerTrafficRevisionFor(t, kept, "kiwivm"); revisionKept != revision1 {
		t.Fatalf("同实例省略凭据应沿用修订: %s != %s", revisionKept, revision1)
	}

	t.Run("change instance requires credential", func(t *testing.T) {
		w := providerTrafficPutBinding(t, srv, adminToken, "kiwivm", map[string]any{
			"enabled": true, "instance_id": "1002", "revision": revision1,
		})
		providerTrafficRequireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("wrong revision", func(t *testing.T) {
		w := providerTrafficPutBinding(t, srv, adminToken, "kiwivm", map[string]any{
			"enabled": true, "instance_id": "1001", "revision": "wrong-revision",
		})
		providerTrafficRequireStatus(t, w, http.StatusConflict)
	})

	t.Run("null revision on existing binding", func(t *testing.T) {
		w := providerTrafficPutBinding(t, srv, adminToken, "kiwivm", map[string]any{
			"enabled": true, "instance_id": "1001", "revision": nil,
		})
		providerTrafficRequireStatus(t, w, http.StatusConflict)
	})

	rotated := decodeProviderTrafficSettings(t, providerTrafficPutBinding(t, srv, adminToken, "kiwivm", map[string]any{
		"enabled": true, "instance_id": "1001", "revision": revision1, "credential": "synthetic-kiwivm-key-2",
	}))
	revision2 := providerTrafficRevisionFor(t, rotated, "kiwivm")
	if revision2 == revision1 {
		t.Fatalf("换证应生成新修订: %s", revision2)
	}

	for name, revision := range map[string]any{
		"missing revision": nil,
		"null revision":    nil,
	} {
		t.Run("delete "+name, func(t *testing.T) {
			var w *httptest.ResponseRecorder
			if name == "missing revision" {
				w = profileReq(t, srv, http.MethodDelete, pathK, adminToken, map[string]any{})
			} else {
				w = providerTrafficDeleteBinding(t, srv, adminToken, "kiwivm", revision)
			}
			providerTrafficRequireStatus(t, w, http.StatusBadRequest)
		})
	}

	for name, revision := range map[string]any{
		"empty revision": "",
		"wrong revision": "wrong-revision",
		"old revision":   revision1,
	} {
		t.Run("delete "+name, func(t *testing.T) {
			w := providerTrafficDeleteBinding(t, srv, adminToken, "kiwivm", revision)
			providerTrafficRequireStatus(t, w, http.StatusConflict)
		})
	}

	providerTrafficRequireStatus(t, providerTrafficDeleteBinding(t, srv, adminToken, "kiwivm", revision2), http.StatusOK)
	providerTrafficRequireStatus(t, providerTrafficDeleteBinding(t, srv, adminToken, "kiwivm", revision2), http.StatusConflict)

	t.Run("unknown provider", func(t *testing.T) {
		for _, w := range []*httptest.ResponseRecorder{
			providerTrafficPutBinding(t, srv, adminToken, "unknown", map[string]any{
				"enabled": true, "instance_id": "1001", "revision": nil, "credential": "synthetic",
			}),
			providerTrafficDeleteBinding(t, srv, adminToken, "unknown", "rev"),
			providerTrafficRefresh(t, srv, adminToken, "unknown"),
		} {
			providerTrafficRequireStatus(t, w, http.StatusBadRequest)
		}
	})

	t.Run("malformed binding body", func(t *testing.T) {
		w := profileReq(t, srv, http.MethodPut, pathK, adminToken, "not-json")
		providerTrafficRequireStatus(t, w, http.StatusBadRequest)
	})

	vps := decodeProviderTrafficSettings(t, providerTrafficPutBinding(t, srv, adminToken, "vps", map[string]any{
		"enabled": false, "instance_id": "2002", "revision": nil, "credential": "synthetic-vps-refresh",
	}))
	if providerTrafficRevisionFor(t, vps, "vps") == "" {
		t.Fatal("V.P.S 首次绑定未返回修订")
	}
}

// TestProviderTrafficModeGates 覆盖高级模式与 Setup 未完成时的读取、写入、刷新和首页行为。
func TestProviderTrafficModeGates(t *testing.T) {
	srv, _ := newProviderTrafficTestServer(t, "error")
	adminToken := regUser(t, srv, "pt-mode-admin", "pt-mode-admin@example.com", "password123")
	ctx := context.Background()

	providerTrafficRequireStatus(t, providerTrafficPutEnabled(t, srv, adminToken, true), http.StatusOK)
	bound := decodeProviderTrafficSettings(t, providerTrafficPutBinding(t, srv, adminToken, "kiwivm", map[string]any{
		"enabled": true, "instance_id": "1001", "revision": nil, "credential": "synthetic-kiwivm-key",
	}))
	rev := providerTrafficRevisionFor(t, bound, "kiwivm")

	if err := srv.cfg.Set(ctx, config.KeyAdvancedMode, "true"); err != nil {
		t.Fatalf("开启高级模式失败: %v", err)
	}
	w := providerTrafficGetSettings(t, srv, adminToken)
	providerTrafficRequireStatus(t, w, http.StatusOK)
	resp := decodeProviderTrafficSettings(t, w)
	if resp.Data.Available || resp.Data.Enabled || len(resp.Data.Providers) != 0 {
		t.Fatalf("高级模式应返回不可用结构: %+v", resp.Data)
	}
	if strings.Contains(w.Body.String(), `"providers":null`) {
		t.Fatalf("不可用结构的 providers 必须为数组: %s", w.Body.String())
	}
	providerTrafficRequireStatus(t, providerTrafficPutEnabled(t, srv, adminToken, false), http.StatusForbidden)
	providerTrafficRequireStatus(t, providerTrafficPutBinding(t, srv, adminToken, "kiwivm", map[string]any{
		"enabled": false, "instance_id": "1001", "revision": rev,
	}), http.StatusForbidden)
	providerTrafficRequireStatus(t, providerTrafficDeleteBinding(t, srv, adminToken, "kiwivm", rev), http.StatusForbidden)
	providerTrafficRequireStatus(t, providerTrafficRefresh(t, srv, adminToken, "kiwivm"), http.StatusForbidden)
	if home := decodeProviderTrafficHome(t, providerTrafficHome(t, srv, adminToken)); len(home.Data.List) != 0 {
		t.Fatalf("高级模式首页应为空: %+v", home.Data)
	}

	if err := srv.cfg.Set(ctx, config.KeyAdvancedMode, "false"); err != nil {
		t.Fatalf("关闭高级模式失败: %v", err)
	}
	providerTrafficRequireStatus(t, providerTrafficGetSettings(t, srv, adminToken), http.StatusOK)
	if home := decodeProviderTrafficHome(t, providerTrafficHome(t, srv, adminToken)); len(home.Data.List) != 1 {
		t.Fatalf("切回基础模式应恢复一家等待行: %+v", home.Data)
	}

	if err := srv.cfg.Set(ctx, config.KeyConfigured, "false"); err != nil {
		t.Fatalf("模拟 Setup 未完成失败: %v", err)
	}
	w = providerTrafficGetSettings(t, srv, adminToken)
	providerTrafficRequireStatus(t, w, http.StatusOK)
	resp = decodeProviderTrafficSettings(t, w)
	if resp.Data.Available || resp.Data.Enabled || len(resp.Data.Providers) != 0 {
		t.Fatalf("Setup 未完成应返回不可用结构: %+v", resp.Data)
	}
	providerTrafficRequireStatus(t, providerTrafficPutEnabled(t, srv, adminToken, true), http.StatusForbidden)
	providerTrafficRequireStatus(t, providerTrafficRefresh(t, srv, adminToken, "kiwivm"), http.StatusForbidden)
	if home := decodeProviderTrafficHome(t, providerTrafficHome(t, srv, adminToken)); len(home.Data.List) != 0 {
		t.Fatalf("Setup 未完成首页应为空: %+v", home.Data)
	}
}

type providerTrafficBeforeRead struct {
	io.Reader
	before func()
}

func (r *providerTrafficBeforeRead) Read(p []byte) (int, error) {
	if r.before != nil {
		r.before()
		r.before = nil
	}
	return r.Reader.Read(p)
}

func TestProviderTrafficWriteRechecksMode(t *testing.T) {
	for _, lateState := range []string{"advanced", "setup"} {
		for _, operation := range []string{"total", "binding", "unbind"} {
			t.Run(lateState+"/"+operation, func(t *testing.T) {
				srv, _ := newProviderTrafficTestServer(t, "error")
				token := regUser(t, srv, "pt-mode-admin", "pt-mode-admin@example.com", "password123")
				ctx := context.Background()
				bound := providerTrafficPutBinding(t, srv, token, "kiwivm", map[string]any{
					"enabled": true, "instance_id": "1001", "revision": nil, "credential": "synthetic-initial",
				})
				providerTrafficRequireStatus(t, bound, http.StatusOK)
				revision := providerTrafficRevisionFor(t, decodeProviderTrafficSettings(t, bound), "kiwivm")
				traffic := providertraffic.NewService(srv.store, srv.cfg, srv.log)
				before, err := traffic.GetSettings(ctx)
				if err != nil {
					t.Fatal(err)
				}
				path := "/api/admin/settings/provider-traffic"
				method := http.MethodPut
				fields := map[string]any{"enabled": true}
				switch operation {
				case "binding":
					path += "/kiwivm"
					fields = map[string]any{
						"enabled": true, "instance_id": "1002", "revision": revision, "credential": "synthetic-replacement",
					}
				case "unbind":
					path += "/kiwivm"
					method = http.MethodDelete
					fields = map[string]any{"revision": revision}
				}
				payload, err := json.Marshal(fields)
				if err != nil {
					t.Fatal(err)
				}
				body := &providerTrafficBeforeRead{
					Reader: bytes.NewReader(payload),
					before: func() {
						if lateState == "advanced" {
							w := profileReq(t, srv, http.MethodPut, "/api/admin/settings/advanced", token, map[string]any{
								"advanced_mode": true, "collect_interval_minutes": 1,
							})
							providerTrafficRequireStatus(t, w, http.StatusOK)
						} else if err := srv.cfg.Set(ctx, config.KeyConfigured, "false"); err != nil {
							t.Fatal(err)
						}
					},
				}
				req := httptest.NewRequest(method, path, body)
				req.Header.Set("Authorization", "Bearer "+token)
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				srv.Engine().ServeHTTP(w, req)
				after, err := traffic.GetSettings(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(before, after) {
					t.Errorf("禁止写入时设置已改变: before=%+v after=%+v", before, after)
				}
				providerTrafficRequireStatus(t, w, http.StatusForbidden)
				assertProviderTrafficNoStore(t, w)
			})
		}
	}
}

// TestProviderTrafficManualRefreshGates 用真实采集器覆盖未启用、高级、Setup 与未知平台的 409/403/400。
func TestProviderTrafficManualRefreshGates(t *testing.T) {
	srv, _ := newProviderTrafficTestServer(t, "error")
	adminToken := regUser(t, srv, "pt-refresh-admin", "pt-refresh-admin@example.com", "password123")
	ctx := context.Background()

	providerTrafficRequireStatus(t, providerTrafficPutBinding(t, srv, adminToken, "kiwivm", map[string]any{
		"enabled": false, "instance_id": "1001", "revision": nil, "credential": "synthetic-kiwivm-key",
	}), http.StatusOK)
	if err := srv.providerTraffic.Start(context.Background()); err != nil {
		t.Fatalf("启动采集器失败: %v", err)
	}
	t.Cleanup(srv.providerTraffic.Stop)

	providerTrafficRequireStatus(t, providerTrafficRefresh(t, srv, adminToken, "kiwivm"), http.StatusConflict)

	providerTrafficRequireStatus(t, providerTrafficPutEnabled(t, srv, adminToken, true), http.StatusOK)
	providerTrafficRequireStatus(t, providerTrafficRefresh(t, srv, adminToken, "kiwivm"), http.StatusConflict)

	if err := srv.cfg.Set(ctx, config.KeyAdvancedMode, "true"); err != nil {
		t.Fatalf("开启高级模式失败: %v", err)
	}
	providerTrafficRequireStatus(t, providerTrafficRefresh(t, srv, adminToken, "kiwivm"), http.StatusForbidden)
	if err := srv.cfg.Set(ctx, config.KeyAdvancedMode, "false"); err != nil {
		t.Fatalf("关闭高级模式失败: %v", err)
	}

	if err := srv.cfg.Set(ctx, config.KeyConfigured, "false"); err != nil {
		t.Fatalf("模拟 Setup 未完成失败: %v", err)
	}
	providerTrafficRequireStatus(t, providerTrafficRefresh(t, srv, adminToken, "kiwivm"), http.StatusForbidden)
	if err := srv.cfg.Set(ctx, config.KeyConfigured, "true"); err != nil {
		t.Fatalf("恢复 Setup 状态失败: %v", err)
	}

	providerTrafficRequireStatus(t, providerTrafficRefresh(t, srv, adminToken, "unknown"), http.StatusBadRequest)
}

// TestProviderTrafficResultViews 覆盖 waiting/ok/error、合法零、UTC 时间、hint 隔离与首页只读。
func TestProviderTrafficResultViews(t *testing.T) {
	cfg := newProviderTrafficTestConfig(t)
	stub := &providerTrafficStub{}
	engine := newProviderTrafficStubEngine(t, stub, cfg)

	w := providerTrafficStubReq(t, engine, http.MethodGet, "/api/home/provider-traffic", nil)
	providerTrafficRequireStatus(t, w, http.StatusOK)
	assertProviderTrafficNoStore(t, w)
	home := decodeProviderTrafficHome(t, w)
	if home.Data.List == nil || len(home.Data.List) != 0 || home.Data.Total != 0 {
		t.Fatalf("空结果应返回 list=[] total=0: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), `"list":null`) {
		t.Fatalf("空列表不得序列化为 null: %s", w.Body.String())
	}

	stub.results = func(context.Context) ([]providertraffic.Result, error) {
		return []providertraffic.Result{{Provider: providertraffic.ProviderKiwiVM, Status: providertraffic.ResultWaiting}}, nil
	}
	w = providerTrafficStubReq(t, engine, http.MethodGet, "/api/home/provider-traffic", nil)
	providerTrafficRequireStatus(t, w, http.StatusOK)
	var waiting map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &waiting); err != nil {
		t.Fatalf("解析等待响应失败: %v", err)
	}
	item := waiting["data"].(map[string]any)["list"].([]any)[0].(map[string]any)
	if len(item) != 2 || item["provider"] != "kiwivm" || item["status"] != "waiting" {
		t.Fatalf("等待行字段异常: %+v", item)
	}

	sampledAt := time.Date(2026, 11, 24, 10, 15, 0, 0, time.FixedZone("UTC+8", 8*3600))
	stub.results = func(context.Context) ([]providertraffic.Result, error) {
		return []providertraffic.Result{{
			Provider:  providertraffic.ProviderKiwiVM,
			Status:    providertraffic.ResultOK,
			Remaining: 0,
			Unit:      providertraffic.UnitGiB,
			SampledAt: sampledAt,
		}}, nil
	}
	w = providerTrafficStubReq(t, engine, http.MethodGet, "/api/home/provider-traffic", nil)
	providerTrafficRequireStatus(t, w, http.StatusOK)
	var okBody map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &okBody); err != nil {
		t.Fatalf("解析成功响应失败: %v", err)
	}
	okItem := okBody["data"].(map[string]any)["list"].([]any)[0].(map[string]any)
	if len(okItem) != 5 || okItem["status"] != "ok" || okItem["remaining"] != float64(0) ||
		okItem["unit"] != "GiB" || okItem["sampled_at"] != "2026-11-24T02:15:00Z" {
		t.Fatalf("成功行字段异常: %+v", okItem)
	}

	stub.results = func(context.Context) ([]providertraffic.Result, error) {
		return []providertraffic.Result{{
			Provider: providertraffic.ProviderKiwiVM,
			Status:   providertraffic.ResultError,
			Message:  "无法连接服务商",
			Hint:     "请核对部署出网",
		}}, nil
	}
	w = providerTrafficStubReq(t, engine, http.MethodGet, "/api/home/provider-traffic", nil)
	providerTrafficRequireStatus(t, w, http.StatusOK)
	var errorBody map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &errorBody); err != nil {
		t.Fatalf("解析失败响应失败: %v", err)
	}
	errorItem := errorBody["data"].(map[string]any)["list"].([]any)[0].(map[string]any)
	if len(errorItem) != 3 || errorItem["status"] != "error" || errorItem["message"] != "无法连接服务商" {
		t.Fatalf("失败行字段异常: %+v", errorItem)
	}
	for _, key := range []string{"remaining", "unit", "sampled_at", "hint"} {
		if _, exists := errorItem[key]; exists {
			t.Fatalf("首页失败行不得包含 %s: %+v", key, errorItem)
		}
	}

	stub.getSettings = func(context.Context) (providertraffic.Settings, error) {
		return providertraffic.Settings{
			Enabled: true,
			Providers: []providertraffic.ProviderBinding{
				{
					Provider:             providertraffic.ProviderKiwiVM,
					Enabled:              true,
					InstanceID:           "1001",
					Revision:             "revision-1",
					CredentialConfigured: true,
				},
				{Provider: providertraffic.ProviderVPS},
			},
		}, nil
	}
	resultRevision := "revision-1"
	stub.resultFor = func(provider providertraffic.Provider, revision string) *providertraffic.Result {
		if provider != providertraffic.ProviderKiwiVM || revision != resultRevision {
			return nil
		}
		return &providertraffic.Result{
			Provider: providertraffic.ProviderKiwiVM,
			Status:   providertraffic.ResultError,
			Message:  "服务商凭据被拒绝",
			Hint:     "请重新填写对应凭据",
		}
	}
	w = providerTrafficStubReq(t, engine, http.MethodGet, "/api/admin/settings/provider-traffic", nil)
	providerTrafficRequireStatus(t, w, http.StatusOK)
	assertProviderTrafficNoStore(t, w)
	var settingsBody map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &settingsBody); err != nil {
		t.Fatalf("解析设置响应失败: %v", err)
	}
	settingsData := settingsBody["data"].(map[string]any)
	if len(settingsData) != 3 {
		t.Fatalf("Settings 顶层字段异常: %+v", settingsData)
	}
	providers := settingsData["providers"].([]any)
	if len(providers) != 2 {
		t.Fatalf("Settings 应固定两家: %+v", providers)
	}
	kiwi := providers[0].(map[string]any)
	if len(kiwi) != 6 || kiwi["provider"] != "kiwivm" || kiwi["instance_id"] != "1001" ||
		kiwi["revision"] != "revision-1" || kiwi["credential_configured"] != true {
		t.Fatalf("Settings 绑定字段异常: %+v", kiwi)
	}
	if _, exists := kiwi["credential"]; exists {
		t.Fatalf("Settings 不得包含凭据字段: %+v", kiwi)
	}
	adminResult := kiwi["result"].(map[string]any)
	if adminResult["status"] != "error" || adminResult["hint"] != "请重新填写对应凭据" {
		t.Fatalf("管理端错误结果应带 hint: %+v", adminResult)
	}
	if vps := providers[1].(map[string]any); vps["instance_id"] != nil || vps["result"] != nil {
		t.Fatalf("未绑定 V.P.S 字段异常: %+v", vps)
	}

	// 模拟响应已取得设置快照，结果槽随后发布新绑定的结果。
	resultRevision = "revision-2"
	stub.setEnabled = func(ctx context.Context, _ bool) (providertraffic.Settings, error) {
		return stub.GetSettings(ctx)
	}
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		w := providerTrafficStubReq(t, engine, method, "/api/admin/settings/provider-traffic", map[string]any{"enabled": true})
		providerTrafficRequireStatus(t, w, http.StatusOK)
		row := decodeProviderTrafficSettings(t, w).Data.Providers[0]
		if row.Revision == nil || *row.Revision != "revision-1" || row.Result != nil {
			t.Fatalf("%s 的旧绑定不得返回新修订的结果: %+v", method, row)
		}
	}
	if stub.manualCalls != 0 {
		t.Fatalf("首页/设置读取不得触发手动采集: %d", stub.manualCalls)
	}
}

// TestProviderTrafficBindingRequestDecoding 覆盖接入层对省略/null/字符串凭据与修订的精确传递。
func TestProviderTrafficBindingRequestDecoding(t *testing.T) {
	cfg := newProviderTrafficTestConfig(t)
	var captured providertraffic.SaveBindingInput
	stub := &providerTrafficStub{
		saveBinding: func(_ context.Context, in providertraffic.SaveBindingInput) (providertraffic.Settings, error) {
			captured = in
			return defaultProviderTrafficTestSettings(), nil
		},
	}
	engine := newProviderTrafficStubEngine(t, stub, cfg)

	w := providerTrafficStubReq(t, engine, http.MethodPut, "/api/admin/settings/provider-traffic/kiwivm", map[string]any{
		"enabled": false, "instance_id": "1001", "revision": nil,
	})
	providerTrafficRequireStatus(t, w, http.StatusOK)
	if captured.Provider != providertraffic.ProviderKiwiVM || captured.Enabled || captured.InstanceID != "1001" ||
		captured.Revision != nil || !captured.Credential.Omitted || captured.Credential.Null || captured.Credential.Value != "" {
		t.Fatalf("省略凭据解析异常: %+v", captured)
	}

	w = providerTrafficStubReq(t, engine, http.MethodPut, "/api/admin/settings/provider-traffic/kiwivm", map[string]any{
		"enabled": true, "instance_id": "1001", "revision": "rev-1", "credential": nil,
	})
	providerTrafficRequireStatus(t, w, http.StatusOK)
	if captured.Credential.Omitted || !captured.Credential.Null || captured.Revision == nil || *captured.Revision != "rev-1" {
		t.Fatalf("null 凭据/修订解析异常: %+v", captured)
	}

	w = providerTrafficStubReq(t, engine, http.MethodPut, "/api/admin/settings/provider-traffic/kiwivm", map[string]any{
		"enabled": true, "instance_id": "1001", "revision": "rev-1", "credential": "synthetic-key",
	})
	providerTrafficRequireStatus(t, w, http.StatusOK)
	if captured.Credential.Omitted || captured.Credential.Null || captured.Credential.Value != "synthetic-key" {
		t.Fatalf("字符串凭据解析异常: %+v", captured)
	}
}

// TestProviderTrafficManualRefreshMappings 用 stub 覆盖刷新成功、上游错误与 409/403/500 映射。
func TestProviderTrafficManualRefreshMappings(t *testing.T) {
	cfg := newProviderTrafficTestConfig(t)
	sampledAt := time.Date(2026, 11, 24, 2, 15, 0, 0, time.UTC)

	cases := []struct {
		name       string
		result     providertraffic.Result
		err        error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "success",
			result:     providertraffic.Result{Provider: providertraffic.ProviderKiwiVM, Status: providertraffic.ResultOK, Remaining: 12.5, Unit: providertraffic.UnitGiB, SampledAt: sampledAt},
			wantStatus: http.StatusOK,
		},
		{
			name:       "upstream credential rejection stays platform error",
			result:     providertraffic.Result{Provider: providertraffic.ProviderVPS, Status: providertraffic.ResultError, Message: "服务商凭据被拒绝", Hint: "请重新填写对应凭据"},
			wantStatus: http.StatusOK,
			wantBody:   "服务商凭据被拒绝",
		},
		{
			name:       "busy",
			err:        providertraffic.ErrCollectorBusy,
			wantStatus: http.StatusConflict,
			wantBody:   "正在采集，请稍后再试",
		},
		{
			name:       "not ready",
			err:        providertraffic.ErrNotReady,
			wantStatus: http.StatusConflict,
			wantBody:   "当前绑定未启用",
		},
		{
			name:       "discarded",
			err:        providertraffic.ErrDiscarded,
			wantStatus: http.StatusConflict,
			wantBody:   "本轮采集结果已丢弃",
		},
		{
			name:       "mode unavailable",
			err:        providertraffic.ErrModeUnavailable,
			wantStatus: http.StatusForbidden,
			wantBody:   "当前模式不可用",
		},
		{
			name:       "collector unavailable",
			err:        providertraffic.ErrCollectorUnavailable,
			wantStatus: http.StatusInternalServerError,
			wantBody:   "服务器内部错误",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &providerTrafficStub{
				manual: func(context.Context, providertraffic.Provider) (providertraffic.Result, error) {
					return tc.result, tc.err
				},
			}
			engine := newProviderTrafficStubEngine(t, stub, cfg)
			w := providerTrafficStubReq(t, engine, http.MethodPost,
				"/api/admin/settings/provider-traffic/kiwivm/refresh", nil)
			providerTrafficRequireStatus(t, w, tc.wantStatus)
			assertProviderTrafficNoStore(t, w)
			if tc.wantBody != "" && !strings.Contains(w.Body.String(), tc.wantBody) {
				t.Fatalf("响应缺少 %q: %s", tc.wantBody, w.Body.String())
			}
			if tc.wantStatus == http.StatusOK {
				var body providerTrafficTestResult
				var wrapper struct {
					Data providerTrafficTestResult `json:"data"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &wrapper); err != nil {
					t.Fatalf("解析刷新响应失败: %v", err)
				}
				body = wrapper.Data
				if body.Provider != string(tc.result.Provider) {
					t.Fatalf("刷新结果 provider 异常: %+v", body)
				}
				if tc.result.Status == providertraffic.ResultOK {
					if body.Remaining == nil || *body.Remaining != 12.5 || body.Unit != "GiB" ||
						body.SampledAt != "2026-11-24T02:15:00Z" || body.Hint != "" {
						t.Fatalf("刷新成功结果字段异常: %+v", body)
					}
				} else {
					if body.Status != "error" || body.Message != "服务商凭据被拒绝" ||
						body.Hint != "请重新填写对应凭据" || body.Remaining != nil || body.Unit != "" || body.SampledAt != "" {
						t.Fatalf("刷新错误结果字段异常: %+v", body)
					}
				}
			}
			if tc.err != nil && errors.Is(tc.err, providertraffic.ErrCollectorUnavailable) &&
				strings.Contains(w.Body.String(), "采集服务不可用") {
				t.Fatalf("500 不得泄露内部错误正文: %s", w.Body.String())
			}
		})
	}

	t.Run("unknown provider", func(t *testing.T) {
		stub := &providerTrafficStub{}
		engine := newProviderTrafficStubEngine(t, stub, cfg)
		w := providerTrafficStubReq(t, engine, http.MethodPost,
			"/api/admin/settings/provider-traffic/unknown/refresh", nil)
		providerTrafficRequireStatus(t, w, http.StatusBadRequest)
		if stub.manualCalls != 0 {
			t.Fatalf("未知平台不得进入采集器: %d", stub.manualCalls)
		}
	})
}

// TestProviderTrafficNoStoreAndBodyLimit 覆盖鉴权失败、业务失败、成功及全局/处理器 413 的 no-store。
func TestProviderTrafficNoStoreAndBodyLimit(t *testing.T) {
	srv, _ := newProviderTrafficTestServer(t, "error")
	adminToken := regUser(t, srv, "pt-limit-admin", "pt-limit-admin@example.com", "password123")
	if err := srv.cfg.Set(context.Background(), config.KeyAllowSelfreg, "true"); err != nil {
		t.Fatalf("开启自注册失败: %v", err)
	}
	userToken := regUser(t, srv, "pt-limit-user", "pt-limit-user@example.com", "password123")

	for _, tc := range []struct {
		name   string
		token  string
		method string
		path   string
		body   any
		want   int
	}{
		{"guest home", "", http.MethodGet, "/api/home/provider-traffic", nil, http.StatusUnauthorized},
		{"guest admin", "", http.MethodGet, "/api/admin/settings/provider-traffic", nil, http.StatusUnauthorized},
		{"normal user admin", userToken, http.MethodGet, "/api/admin/settings/provider-traffic", nil, http.StatusForbidden},
		{"admin settings", adminToken, http.MethodGet, "/api/admin/settings/provider-traffic", nil, http.StatusOK},
		{"admin total", adminToken, http.MethodPut, "/api/admin/settings/provider-traffic", map[string]any{"enabled": false}, http.StatusOK},
		{"admin invalid binding", adminToken, http.MethodPut, "/api/admin/settings/provider-traffic/kiwivm", map[string]any{"enabled": true}, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := profileReq(t, srv, tc.method, tc.path, tc.token, tc.body)
			providerTrafficRequireStatus(t, w, tc.want)
			assertProviderTrafficNoStore(t, w)
		})
	}

	providerTrafficRequireStatus(t, providerTrafficPutBinding(t, srv, adminToken, "kiwivm", map[string]any{
		"enabled": true, "instance_id": "1001", "revision": nil, "credential": "synthetic-kiwivm-key",
	}), http.StatusOK)
	w := providerTrafficDeleteBinding(t, srv, adminToken, "kiwivm", "wrong-revision")
	providerTrafficRequireStatus(t, w, http.StatusConflict)
	assertProviderTrafficNoStore(t, w)

	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{"total root", http.MethodPut, "/api/admin/settings/provider-traffic"},
		{"provider", http.MethodPut, "/api/admin/settings/provider-traffic/vps"},
		{"refresh", http.MethodPost, "/api/admin/settings/provider-traffic/vps/refresh"},
		{"home", http.MethodGet, "/api/home/provider-traffic"},
	} {
		t.Run("global body limit "+tc.name, func(t *testing.T) {
			big := `{"credential":"` + strings.Repeat("a", (4<<20)+1) + `"}`
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(big))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+adminToken)
			w := httptest.NewRecorder()
			srv.Engine().ServeHTTP(w, req)
			providerTrafficRequireStatus(t, w, http.StatusRequestEntityTooLarge)
			assertProviderTrafficNoStore(t, w)
		})
	}

	t.Run("handler body limit", func(t *testing.T) {
		big := `{"credential":"` + strings.Repeat("a", (4<<20)+1) + `"}`
		// 包装 reader 使 ContentLength 未知，绕过全局 ContentLength 短路，验证处理器的 413 映射。
		unknownLength := struct{ io.Reader }{strings.NewReader(big)}
		req := httptest.NewRequest(http.MethodPut, "/api/admin/settings/provider-traffic/vps", unknownLength)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		w := httptest.NewRecorder()
		srv.Engine().ServeHTTP(w, req)
		providerTrafficRequireStatus(t, w, http.StatusRequestEntityTooLarge)
		assertProviderTrafficNoStore(t, w)
	})
}

func TestProviderTrafficJSONRequestEnd(t *testing.T) {
	cfg := newProviderTrafficTestConfig(t)
	if err := cfg.Set(context.Background(), "http_max_body_mb", "1"); err != nil {
		t.Fatal(err)
	}
	const object = `{"enabled":true}`
	const side = `{"enabled":true,"instance_id":"1001","revision":null,"credential":"synthetic"}`
	const unbind = `{"revision":"revision-1"}`
	const limit = 1 << 20
	const path = "/api/admin/settings/provider-traffic"
	whitespace := strings.Repeat(" ", limit-len(object))
	for _, tc := range []struct {
		name, method, path, body string
		want                     int
	}{
		{"valid exact limit", http.MethodPut, path, object + whitespace, http.StatusOK},
		{"second object", http.MethodPut, path, object + " " + object, http.StatusBadRequest},
		{"trailing byte", http.MethodPut, path + "/kiwivm", side + "x", http.StatusBadRequest},
		{"trailing non-JSON whitespace", http.MethodDelete, path + "/kiwivm", unbind + "\u00a0", http.StatusBadRequest},
		{"oversize whitespace", http.MethodPut, path, object + whitespace + " ", http.StatusRequestEntityTooLarge},
		{"oversize after invalid tail", http.MethodPut, path, object + "x" + whitespace, http.StatusRequestEntityTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writes := 0
			save := func() (providertraffic.Settings, error) {
				writes++
				return defaultProviderTrafficTestSettings(), nil
			}
			stub := &providerTrafficStub{
				setEnabled: func(context.Context, bool) (providertraffic.Settings, error) { return save() },
				saveBinding: func(context.Context, providertraffic.SaveBindingInput) (providertraffic.Settings, error) {
					return save()
				},
				clearBinding: func(context.Context, providertraffic.Provider, string) (providertraffic.Settings, error) {
					return save()
				},
			}
			engine := newProviderTrafficStubEngine(t, stub, cfg)
			req := httptest.NewRequest(tc.method, tc.path, struct{ io.Reader }{strings.NewReader(tc.body)})
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)
			wantWrites := 0
			if tc.want == http.StatusOK {
				wantWrites = 1
			}
			if writes != wantWrites {
				t.Errorf("写入次数 = %d，期望 %d", writes, wantWrites)
			}
			providerTrafficRequireStatus(t, w, tc.want)
			assertProviderTrafficNoStore(t, w)
		})
	}
}

// TestProviderTrafficSafeErrorResponse 覆盖调试模式下 500 仍返回通用正文且带 no-store。
func TestProviderTrafficSafeErrorResponse(t *testing.T) {
	cfg := newProviderTrafficTestConfig(t)
	if err := cfg.Set(context.Background(), "debug_mode", "true"); err != nil {
		t.Fatalf("开启调试模式失败: %v", err)
	}
	stub := &providerTrafficStub{
		getSettings: func(context.Context) (providertraffic.Settings, error) {
			return providertraffic.Settings{}, errors.New("database is locked")
		},
	}
	engine := newProviderTrafficStubEngine(t, stub, cfg)
	w := providerTrafficStubReq(t, engine, http.MethodGet, "/api/admin/settings/provider-traffic", nil)
	providerTrafficRequireStatus(t, w, http.StatusInternalServerError)
	assertProviderTrafficNoStore(t, w)
	if !strings.Contains(w.Body.String(), "服务器内部错误") || strings.Contains(w.Body.String(), "database is locked") {
		t.Fatalf("500 响应正文不安全: %s", w.Body.String())
	}
}

// TestProviderTrafficLogsDoNotLeakCredential 覆盖保存/读取/首页响应和调试日志均不出现合成凭据。
func TestProviderTrafficLogsDoNotLeakCredential(t *testing.T) {
	srv, buf := newProviderTrafficTestServer(t, "debug")
	adminToken := regUser(t, srv, "pt-log-admin", "pt-log-admin@example.com", "password123")
	const credential = "synthetic-pt-credential-9f7c"

	settings := decodeProviderTrafficSettings(t, providerTrafficPutBinding(t, srv, adminToken, "kiwivm", map[string]any{
		"enabled": true, "instance_id": "1001", "revision": nil, "credential": credential,
	}))
	rev := providerTrafficRevisionFor(t, settings, "kiwivm")
	providerTrafficRequireStatus(t, providerTrafficPutEnabled(t, srv, adminToken, true), http.StatusOK)

	settingsBody := providerTrafficGetSettings(t, srv, adminToken)
	providerTrafficRequireStatus(t, settingsBody, http.StatusOK)
	homeBody := providerTrafficHome(t, srv, adminToken)
	providerTrafficRequireStatus(t, homeBody, http.StatusOK)

	var ciphertext string
	if err := srv.store.DB().QueryRow(
		`SELECT kiwivm_credential_ciphertext FROM provider_traffic_settings WHERE id = 1`).Scan(&ciphertext); err != nil {
		t.Fatalf("读取功能表密文失败: %v", err)
	}
	if ciphertext == "" || ciphertext == credential {
		t.Fatalf("凭据密文异常: %q", ciphertext)
	}
	for name, body := range map[string]string{
		"settings": settingsBody.Body.String(),
		"home":     homeBody.Body.String(),
	} {
		if strings.Contains(body, credential) || strings.Contains(body, ciphertext) {
			t.Fatalf("%s 响应泄露凭据或密文", name)
		}
	}
	if strings.Contains(homeBody.Body.String(), "1001") || strings.Contains(homeBody.Body.String(), rev) {
		t.Fatalf("首页泄露实例或修订: %s", homeBody.Body.String())
	}

	for _, entry := range buf.History() {
		text := entry.Message + " " + entry.Attrs
		if strings.Contains(text, credential) {
			t.Fatalf("调试日志泄露凭据: %s", text)
		}
	}
}

// TestProviderTrafficRunLifecycle 覆盖正常服务先启动采集器、退出停止及监听失败停止。
func TestProviderTrafficRunLifecycle(t *testing.T) {
	t.Run("starts before serving and stops on cancel", func(t *testing.T) {
		srv, _ := newProviderTrafficTestServer(t, "error")
		adminToken := regUser(t, srv, "pt-life-admin", "pt-life-admin@example.com", "password123")
		addr := freeProviderTrafficLoopbackAddr(t)
		srv.httpSrv.Addr = addr

		ctx, cancel := context.WithCancel(context.Background())
		runErr := make(chan error, 1)
		go func() { runErr <- srv.Run(ctx) }()

		waitForProviderTrafficHealth(t, "http://"+addr+"/health")
		// 采集器已启动（总开关关闭 -> 409 未启用，而不是未启动的 500）。
		w := providerTrafficRefresh(t, srv, adminToken, "kiwivm")
		providerTrafficRequireStatus(t, w, http.StatusConflict)

		cancel()
		select {
		case err := <-runErr:
			if err != nil {
				t.Fatalf("正常退出返回错误: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("等待正常退出超时")
		}
		if err := srv.providerTraffic.Start(context.Background()); !errors.Is(err, providertraffic.ErrCollectorStopped) {
			t.Fatalf("退出后采集器未停止: %v", err)
		}
	})

	t.Run("listen failure stops collector", func(t *testing.T) {
		srv, _ := newProviderTrafficTestServer(t, "error")
		srv.httpSrv.Addr = ":-1"
		if err := srv.Run(context.Background()); err == nil {
			t.Fatal("非法监听地址应返回错误")
		}
		if err := srv.providerTraffic.Start(context.Background()); !errors.Is(err, providertraffic.ErrCollectorStopped) {
			t.Fatalf("监听失败后采集器未停止: %v", err)
		}
	})
}

func freeProviderTrafficLoopbackAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("分配回环端口失败: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("释放回环端口失败: %v", err)
	}
	return addr
}

func waitForProviderTrafficHealth(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("等待健康检查超时: %s", url)
}
