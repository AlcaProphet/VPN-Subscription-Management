package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestOidcSecretContractHTTP 面板 OIDC 接口合同：GET 不回显 Secret，仅返回配置状态；
// PUT 空值保持，显式新值替换，占位符被拒绝；Setup 入口同样拒绝占位符。
func TestOidcSecretContractHTTP(t *testing.T) {
	srv := newDownloadTestServer(t)
	token := regUser(t, srv, "admin", "admin@example.com", "password123")
	base := map[string]any{
		"provider_type": "generic",
		"base_url":      "https://idp.example.com",
		"realm":         "",
		"client_id":     "client-x",
		"client_secret": "sec123",
		"frontend_url":  "",
		"callback_url":  "",
	}
	w := profileReq(t, srv, http.MethodPut, "/api/admin/settings/oidc", token, base)
	if w.Code != http.StatusOK {
		t.Fatalf("保存 OIDC 失败: %d %s", w.Code, w.Body.String())
	}
	// GET 只回显状态，不回显 Secret
	w = profileReq(t, srv, http.MethodGet, "/api/admin/settings/oidc", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("读取 OIDC 失败: %d %s", w.Code, w.Body.String())
	}
	var got struct {
		Code int `json:"code"`
		Data struct {
			ClientSecret           string `json:"client_secret"`
			ClientSecretConfigured bool   `json:"client_secret_configured"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("解析 OIDC 响应失败: %v", err)
	}
	if got.Data.ClientSecret != "" || !got.Data.ClientSecretConfigured {
		t.Fatalf("GET 应空回显且标记已配置: %+v", got.Data)
	}
	// PUT 空值保持
	empty := map[string]any{
		"provider_type": "generic", "base_url": "https://idp.example.com", "realm": "",
		"client_id": "client-x", "client_secret": "", "frontend_url": "", "callback_url": "",
	}
	w = profileReq(t, srv, http.MethodPut, "/api/admin/settings/oidc", token, empty)
	if w.Code != http.StatusOK {
		t.Fatalf("空值保存应保持原 Secret: %d %s", w.Code, w.Body.String())
	}
	// PUT 占位符拒绝
	placeholder := map[string]any{
		"provider_type": "generic", "base_url": "https://idp.example.com", "realm": "",
		"client_id": "client-x", "client_secret": "***", "frontend_url": "", "callback_url": "",
	}
	w = profileReq(t, srv, http.MethodPut, "/api/admin/settings/oidc", token, placeholder)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("占位符应 400: %d %s", w.Code, w.Body.String())
	}
	// Setup 未配置入口同样拒绝占位符（避免任何入口把 *** 落库）
	body, err := json.Marshal(map[string]string{
		"provider_type": "generic", "base_url": "https://idp.example.com", "client_id": "c", "client_secret": "***",
	})
	if err != nil {
		t.Fatalf("序列化 Setup 请求失败: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/setup/oidc", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Engine().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Setup 占位符应 400: %d %s", rec.Code, rec.Body.String())
	}
}

// TestR3102OidcWriteEntrypointsRejectHTTPBaseURL Setup 与管理端写入口均不得接受 HTTP Base URL。
func TestR3102OidcWriteEntrypointsRejectHTTPBaseURL(t *testing.T) {
	t.Run("Setup", func(t *testing.T) {
		srv, _ := newDownloadTestServerWithDir(t)
		body, err := json.Marshal(map[string]string{
			"provider_type": "generic", "base_url": "http://idp.example.com", "client_id": "c", "client_secret": "s",
		})
		if err != nil {
			t.Fatalf("序列化 Setup 请求失败: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/setup/oidc", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.Engine().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "HTTPS") {
			t.Fatalf("Setup HTTP Base URL 应 400 且提示 HTTPS: %d %s", rec.Code, rec.Body.String())
		}
		var count int
		if err := srv.store.DB().QueryRow(
			`SELECT COUNT(*) FROM system_config WHERE key = 'oidc_provider_type' OR key = 'oidc_configured' OR key LIKE 'oidc_params_%'`).Scan(&count); err != nil {
			t.Fatalf("查询 OIDC 配置失败: %v", err)
		}
		if count != 0 {
			t.Fatalf("拒绝后不应写入 OIDC 配置，实际 %d 个键", count)
		}
	})

	t.Run("管理端", func(t *testing.T) {
		srv := newDownloadTestServer(t)
		token := regUser(t, srv, "r3102-admin", "r3102-admin@example.com", "password123")
		body := map[string]any{
			"provider_type": "generic",
			"base_url":      "http://idp.example.com",
			"realm":         "",
			"client_id":     "c",
			"client_secret": "s",
			"frontend_url":  "",
			"callback_url":  "",
		}
		w := profileReq(t, srv, http.MethodPut, "/api/admin/settings/oidc", token, body)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "HTTPS") {
			t.Fatalf("管理端 HTTP Base URL 应 400 且提示 HTTPS: %d %s", w.Code, w.Body.String())
		}
	})
}

// oidcRawParam 读取指定提供商参数原始 JSON（测试辅助）。
func oidcRawParam(t *testing.T, srv *Server, providerType string) string {
	t.Helper()
	var raw string
	if err := srv.store.DB().QueryRow(`SELECT value FROM system_config WHERE key = ?`, "oidc_params_"+providerType).Scan(&raw); err != nil {
		t.Fatalf("读取 %s 参数失败: %v", providerType, err)
	}
	return raw
}

// TestR3103TargetReadAndSaveIsolation HTTP 接口验收：目标读取不混字段、空 Secret 校验失败不写库、切回源提供商。
func TestR3103TargetReadAndSaveIsolation(t *testing.T) {
	srv := newDownloadTestServer(t)
	token := regUser(t, srv, "r3103-admin", "r3103-admin@example.com", "password123")

	put := func(body map[string]any) *httptest.ResponseRecorder {
		return profileReq(t, srv, http.MethodPut, "/api/admin/settings/oidc", token, body)
	}
	get := func(path string) (int, map[string]any) {
		w := profileReq(t, srv, http.MethodGet, path, token, nil)
		if w.Code != http.StatusOK {
			return w.Code, nil
		}
		var resp struct {
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("解析 OIDC GET 响应失败: %v", err)
		}
		return w.Code, resp.Data
	}

	// 源提供商 generic。
	w := put(map[string]any{
		"provider_type": "generic", "base_url": "https://source.example.com", "realm": "",
		"client_id": "source", "client_secret": "source-secret", "frontend_url": "", "callback_url": "",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("保存源 generic 失败: %d %s", w.Code, w.Body.String())
	}
	sourceRaw := oidcRawParam(t, srv, "generic")

	// 目标未配置：读取目标只返回空字段，不混入源字段。
	code, got := get("/api/admin/settings/oidc?provider_type=keycloak")
	if code != http.StatusOK || got["base_url"] != "" || got["client_id"] != "" || got["client_secret_configured"] != false {
		t.Fatalf("未配置目标读取异常: code=%d data=%+v", code, got)
	}
	if secret, ok := got["client_secret"]; ok && secret != "" {
		t.Fatalf("目标读取不得回显 Secret: %+v", got)
	}

	// 直接 API 尝试用源字段切换到目标：空 Secret 必须拒绝且不写 keycloak/不改 provider。
	w = put(map[string]any{
		"provider_type": "keycloak", "base_url": "https://source.example.com", "realm": "master",
		"client_id": "source", "client_secret": "", "frontend_url": "", "callback_url": "",
	})
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "Client Secret") {
		t.Fatalf("源字段 + 目标空 Secret 应 400 且要求新 Secret: %d %s", w.Code, w.Body.String())
	}
	var providerType string
	if err := srv.store.DB().QueryRow(`SELECT value FROM system_config WHERE key = 'oidc_provider_type'`).Scan(&providerType); err != nil {
		t.Fatalf("查询 provider_type 失败: %v", err)
	}
	if providerType != "generic" {
		t.Fatalf("拒绝后不得切换生效提供商: %s", providerType)
	}
	var targetCount int
	if err := srv.store.DB().QueryRow(`SELECT COUNT(*) FROM system_config WHERE key = 'oidc_params_keycloak'`).Scan(&targetCount); err != nil {
		t.Fatalf("查询 keycloak 参数失败: %v", err)
	}
	if targetCount != 0 {
		t.Fatal("拒绝后不得写入目标参数")
	}
	if gotRaw := oidcRawParam(t, srv, "generic"); gotRaw != sourceRaw {
		t.Fatalf("拒绝后源参数不应变化: before=%s after=%s", sourceRaw, gotRaw)
	}

	// 配置目标 keycloak。
	w = put(map[string]any{
		"provider_type": "keycloak", "base_url": "https://target.example.com", "realm": "master",
		"client_id": "target", "client_secret": "target-secret", "frontend_url": "", "callback_url": "",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("保存目标 keycloak 失败: %d %s", w.Code, w.Body.String())
	}
	targetRaw := oidcRawParam(t, srv, "keycloak")

	// 目标读取必须返回 keycloak 自己的字段与状态。
	code, got = get("/api/admin/settings/oidc?provider_type=keycloak")
	if code != http.StatusOK || got["provider_type"] != "keycloak" || got["base_url"] != "https://target.example.com" ||
		got["realm"] != "master" || got["client_id"] != "target" || got["client_secret_configured"] != true {
		t.Fatalf("目标读取异常: code=%d data=%+v", code, got)
	}
	if got["client_secret"] != "" {
		t.Fatalf("目标读取 Secret 应始终为空: %+v", got)
	}

	// 目标字段变化 + 空 Secret：拒绝且 raw JSON 不变。
	w = put(map[string]any{
		"provider_type": "keycloak", "base_url": "https://evil.example.com", "realm": "master",
		"client_id": "target", "client_secret": "", "frontend_url": "", "callback_url": "",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("目标字段变化 + 空 Secret 应 400: %d %s", w.Code, w.Body.String())
	}
	if gotRaw := oidcRawParam(t, srv, "keycloak"); gotRaw != targetRaw {
		t.Fatalf("拒绝后目标参数不应变化:\nbefore=%s\nafter=%s", targetRaw, gotRaw)
	}
	var currentProvider string
	if err := srv.store.DB().QueryRow(`SELECT value FROM system_config WHERE key = 'oidc_provider_type'`).Scan(&currentProvider); err != nil {
		t.Fatalf("查询 provider_type 失败: %v", err)
	}
	if currentProvider != "keycloak" {
		t.Fatalf("目标字段变化拒绝后不得切换生效提供商: %s", currentProvider)
	}

	// 目标字段一致 + 空 Secret：允许保留目标原密文。
	w = put(map[string]any{
		"provider_type": "keycloak", "base_url": "https://target.example.com", "realm": "master",
		"client_id": "target", "client_secret": "", "frontend_url": "", "callback_url": "",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("目标字段一致 + 空 Secret 应保留原值: %d %s", w.Code, w.Body.String())
	}
	if gotRaw := oidcRawParam(t, srv, "keycloak"); gotRaw != targetRaw {
		t.Fatalf("空 Secret 保存应保留目标原密文:\nbefore=%s\nafter=%s", targetRaw, gotRaw)
	}

	// 切回源提供商：源字段一致 + 空 Secret 应保留源原密文。
	w = put(map[string]any{
		"provider_type": "generic", "base_url": "https://source.example.com", "realm": "",
		"client_id": "source", "client_secret": "", "frontend_url": "", "callback_url": "",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("切回源提供商失败: %d %s", w.Code, w.Body.String())
	}
	if gotRaw := oidcRawParam(t, srv, "generic"); gotRaw != sourceRaw {
		t.Fatalf("切回源后源密文应保持:\nbefore=%s\nafter=%s", sourceRaw, gotRaw)
	}
	if gotRaw := oidcRawParam(t, srv, "keycloak"); gotRaw != targetRaw {
		t.Fatalf("切回源不应影响目标参数:\nbefore=%s\nafter=%s", targetRaw, gotRaw)
	}
}

// TestR3103CallbackAfterProviderSwitch 授权中切换提供商后，旧 state 回调不得用新提供商处理。
func TestR3103CallbackAfterProviderSwitch(t *testing.T) {
	srv := newDownloadTestServer(t)
	token := regUser(t, srv, "r3103-callback-admin", "r3103-callback-admin@example.com", "password123")

	put := func(body map[string]any) *httptest.ResponseRecorder {
		return profileReq(t, srv, http.MethodPut, "/api/admin/settings/oidc", token, body)
	}
	// 先配置 Dev mock 提供商并启动登录。
	w := put(map[string]any{
		"provider_type": "mock", "base_url": "", "realm": "", "client_id": "",
		"client_secret": "", "frontend_url": "https://app.example.com", "callback_url": "",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("保存 mock OIDC 失败: %d %s", w.Code, w.Body.String())
	}
	loginReq := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/login", nil)
	loginRec := httptest.NewRecorder()
	srv.Engine().ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusFound {
		t.Fatalf("mock 登录发起应 302: %d %s", loginRec.Code, loginRec.Body.String())
	}
	location := loginRec.Header().Get("Location")
	u, err := url.Parse(location)
	if err != nil {
		t.Fatalf("解析登录 Location 失败: %v", err)
	}
	state := u.Query().Get("state")
	if state == "" {
		t.Fatalf("登录 Location 缺少 state: %s", location)
	}
	var stateCookie *http.Cookie
	for _, ck := range loginRec.Result().Cookies() {
		if ck.Name == stateCookieName() {
			stateCookie = ck
			break
		}
	}
	if stateCookie == nil {
		t.Fatal("登录响应缺少 state Cookie")
	}

	// 切换到 generic；旧 state 属于 mock。
	w = put(map[string]any{
		"provider_type": "generic", "base_url": "https://idp.example.com", "realm": "",
		"client_id": "g", "client_secret": "g-secret", "frontend_url": "", "callback_url": "",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("切换 generic 失败: %d %s", w.Code, w.Body.String())
	}

	callback := func(stateValue string, cookie *http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?state="+url.QueryEscape(stateValue)+"&code=old-code", nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		srv.Engine().ServeHTTP(rec, req)
		return rec
	}
	rec := callback(state, stateCookie)
	if rec.Code != http.StatusFound || !strings.Contains(rec.Header().Get("Location"), "oidc_error=exchange_failed") {
		t.Fatalf("切换提供商后旧 state 回调应以 exchange_failed 拒绝: code=%d location=%s body=%s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	var ticketCount int
	if err := srv.store.DB().QueryRow(`SELECT COUNT(*) FROM oidc_login_tickets`).Scan(&ticketCount); err != nil {
		t.Fatalf("查询 ticket 失败: %v", err)
	}
	if ticketCount != 0 {
		t.Fatalf("旧 state 回调不得签发换票 ticket: %d", ticketCount)
	}

	// 迁移前旧 state（无 provider/config 固定标识）应保留行并以 state_expired 拒绝。
	if _, err := srv.store.DB().Exec(
		`INSERT INTO oidc_states (state, code_verifier, nonce, intent) VALUES ('legacy-callback','v','n','login')`); err != nil {
		t.Fatalf("写入旧 state 失败: %v", err)
	}
	legacyRec := callback("legacy-callback", &http.Cookie{Name: stateCookieName(), Value: "legacy-callback"})
	if legacyRec.Code != http.StatusFound || !strings.Contains(legacyRec.Header().Get("Location"), "oidc_error=state_expired") {
		t.Fatalf("旧 state 回调应以 state_expired 拒绝: code=%d location=%s", legacyRec.Code, legacyRec.Header().Get("Location"))
	}
	var legacyCount int
	if err := srv.store.DB().QueryRow(`SELECT COUNT(*) FROM oidc_states WHERE state='legacy-callback'`).Scan(&legacyCount); err != nil {
		t.Fatalf("查询旧 state 失败: %v", err)
	}
	if legacyCount != 1 {
		t.Fatalf("旧 state 行应在拒绝后保留，实际 %d", legacyCount)
	}
}

// stateCookieName 返回 OIDC state Cookie 名（避免测试直接依赖字符串）。
func stateCookieName() string {
	return stateCookie
}

// TestR3103LocalLoginOffSwitchProtection 本地登录关闭时，空 Secret 切换失败不得写库/切换 provider；显式新 Secret 才可完成切换。
func TestR3103LocalLoginOffSwitchProtection(t *testing.T) {
	srv := newDownloadTestServer(t)
	token := regUser(t, srv, "r3103-localoff-admin", "r3103-localoff-admin@example.com", "password123")
	w := profileReq(t, srv, http.MethodPut, "/api/admin/settings/oidc", token, map[string]any{
		"provider_type": "generic", "base_url": "https://source.example.com", "realm": "",
		"client_id": "source", "client_secret": "source-secret", "frontend_url": "https://app.example.com", "callback_url": "",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("保存源 generic 失败: %d %s", w.Code, w.Body.String())
	}
	sourceRaw := oidcRawParam(t, srv, "generic")
	w = profileReq(t, srv, http.MethodPut, "/api/admin/settings/local-auth", token, map[string]any{
		"allow_local_login": false, "allow_selfreg": false, "selfreg_approval": false,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("OIDC 可用时应允许关闭本地登录: %d %s", w.Code, w.Body.String())
	}

	// 源字段 + 目标空 Secret：拒绝且配置不变。
	w = profileReq(t, srv, http.MethodPut, "/api/admin/settings/oidc", token, map[string]any{
		"provider_type": "keycloak", "base_url": "https://source.example.com", "realm": "master",
		"client_id": "source", "client_secret": "", "frontend_url": "", "callback_url": "",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("本地登录关闭时源字段混入目标应 400: %d %s", w.Code, w.Body.String())
	}
	assertProvider := func(want string) {
		t.Helper()
		var got string
		if err := srv.store.DB().QueryRow(`SELECT value FROM system_config WHERE key='oidc_provider_type'`).Scan(&got); err != nil {
			t.Fatalf("查询 provider_type 失败: %v", err)
		}
		if got != want {
			t.Fatalf("provider_type 应为 %s，实际 %s", want, got)
		}
	}
	assertAllowLocal := func(want string) {
		t.Helper()
		var got string
		if err := srv.store.DB().QueryRow(`SELECT value FROM system_config WHERE key='allow_local_login'`).Scan(&got); err != nil {
			t.Fatalf("查询 allow_local_login 失败: %v", err)
		}
		if got != want {
			t.Fatalf("allow_local_login 应为 %s，实际 %s", want, got)
		}
	}
	assertProvider("generic")
	assertAllowLocal("false")
	if gotRaw := oidcRawParam(t, srv, "generic"); gotRaw != sourceRaw {
		t.Fatalf("拒绝后源参数不应变化: before=%s after=%s", sourceRaw, gotRaw)
	}
	var keycloakCount int
	if err := srv.store.DB().QueryRow(`SELECT COUNT(*) FROM system_config WHERE key='oidc_params_keycloak'`).Scan(&keycloakCount); err != nil {
		t.Fatalf("查询 keycloak 参数失败: %v", err)
	}
	if keycloakCount != 0 {
		t.Fatal("拒绝后不得写入 keycloak 参数")
	}

	// 显式新 Secret 完成切换；本地登录保持关闭且新目标可用。
	w = profileReq(t, srv, http.MethodPut, "/api/admin/settings/oidc", token, map[string]any{
		"provider_type": "keycloak", "base_url": "https://target.example.com", "realm": "master",
		"client_id": "target", "client_secret": "target-secret", "frontend_url": "", "callback_url": "",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("显式新 Secret 切换应成功: %d %s", w.Code, w.Body.String())
	}
	assertProvider("keycloak")
	assertAllowLocal("false")
}
