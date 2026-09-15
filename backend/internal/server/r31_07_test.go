package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// R31-07 HTTP 层隔离测试：停用落库、公开状态、直接 API 阻断、旧 state/ticket 永久失效、已有会话保持。

func enableOidcForTest(t *testing.T, srv *Server, token, provider string) {
	t.Helper()
	body := map[string]any{
		"provider_type": provider,
		"base_url":      "",
		"realm":         "",
		"client_id":     "",
		"client_secret": "",
		"frontend_url":  "https://app.example.com",
		"callback_url":  "",
	}
	if provider != "mock" {
		body["base_url"] = "https://idp.example.com"
		body["client_id"] = "client-x"
		body["client_secret"] = "secret-x"
	}
	if w := profileReq(t, srv, http.MethodPut, "/api/admin/settings/oidc", token, body); w.Code != http.StatusOK {
		t.Fatalf("启用 OIDC %s 失败: %d %s", provider, w.Code, w.Body.String())
	}
}

func readSystemStatusForTest(t *testing.T, srv *Server) map[string]any {
	t.Helper()
	w := doReq(t, srv, http.MethodGet, "/api/system/status")
	if w.Code != http.StatusOK {
		t.Fatalf("系统状态应 200: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析系统状态失败: %v", err)
	}
	return resp.Data
}

// TestR3107HTTPDisablePersistsBlocksAndKeepsSession 停用经独立端点落库，公开状态不暴露保留 provider，
// 直接登录/绑定 API 被阻断，已有本地会话继续有效。
func TestR3107HTTPDisablePersistsBlocksAndKeepsSession(t *testing.T) {
	srv := newDownloadTestServer(t)
	token := regUser(t, srv, "r3107-http-admin", "r3107-http-admin@example.com", "password123")
	enableOidcForTest(t, srv, token, "generic")

	if w := profileReq(t, srv, http.MethodPost, "/api/admin/settings/oidc/disable", token, nil); w.Code != http.StatusOK {
		t.Fatalf("停用 OIDC 应 200: %d %s", w.Code, w.Body.String())
	}
	var configured, provider, params string
	if err := srv.store.DB().QueryRow(`SELECT COALESCE((SELECT value FROM system_config WHERE key='oidc_configured'),'')`).Scan(&configured); err != nil {
		t.Fatalf("查询 configured 失败: %v", err)
	}
	if err := srv.store.DB().QueryRow(`SELECT COALESCE((SELECT value FROM system_config WHERE key='oidc_provider_type'),'')`).Scan(&provider); err != nil {
		t.Fatalf("查询 provider 失败: %v", err)
	}
	if err := srv.store.DB().QueryRow(`SELECT COALESCE((SELECT value FROM system_config WHERE key='oidc_params_generic'),'')`).Scan(&params); err != nil {
		t.Fatalf("查询参数失败: %v", err)
	}
	if configured != "false" || provider != "generic" || params == "" {
		t.Fatalf("停用落库语义异常: configured=%q provider=%q params=%q", configured, provider, params)
	}

	// 管理端 GET：enabled=false 且保留 provider；公开状态：不把保留参数呈现为可用入口。
	admin := getOidcDataForTestForR3107(t, srv, token, "/api/admin/settings/oidc")
	if admin["enabled"] != false || admin["provider_type"] != "generic" || admin["client_secret"] != "" {
		t.Fatalf("管理端停用态异常: %+v", admin)
	}
	status := readSystemStatusForTest(t, srv)
	if status["oidc_configured"] != false || status["oidc_provider_type"] != "" {
		t.Fatalf("公开状态不得暴露停用/保留 OIDC: %+v", status)
	}

	// 直接 API：登录/绑定发起均 403，不得新增 state。
	if w := doReq(t, srv, http.MethodGet, "/api/auth/oidc/login"); w.Code != http.StatusForbidden {
		t.Fatalf("停用后登录发起应 403: %d %s", w.Code, w.Body.String())
	}
	if w := profileReq(t, srv, http.MethodPost, "/api/auth/oidc/bind", token, nil); w.Code != http.StatusForbidden {
		t.Fatalf("停用后绑定发起应 403: %d %s", w.Code, w.Body.String())
	}
	var stateCount int
	if err := srv.store.DB().QueryRow(`SELECT COUNT(*) FROM oidc_states`).Scan(&stateCount); err != nil {
		t.Fatalf("统计 state 失败: %v", err)
	}
	if stateCount != 0 {
		t.Fatalf("停用后不得新增 state，实际 %d", stateCount)
	}

	// 停用前已登录的本地会话继续有效。
	if w := profileReq(t, srv, http.MethodGet, "/api/auth/me", token, nil); w.Code != http.StatusOK {
		t.Fatalf("停用不得撤销已有会话: %d %s", w.Code, w.Body.String())
	}

	// 直接回调旧 state 也必须在停用态被拒。
	if _, err := srv.store.DB().Exec(
		`INSERT INTO oidc_states (state, code_verifier, nonce, intent, provider_type, config_hash, redirect_uri)
		 VALUES ('r3107-http-state','v','n','login','generic','hash','https://app.example.com/api/auth/oidc/callback')`); err != nil {
		t.Fatalf("写入回调 state 失败: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?state=r3107-http-state&code=code", nil)
	req.AddCookie(&http.Cookie{Name: stateCookieName(), Value: "r3107-http-state"})
	w := httptest.NewRecorder()
	srv.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusFound || !strings.Contains(w.Header().Get("Location"), "state_expired") {
		t.Fatalf("停用态回调应 state_expired 拒绝: code=%d location=%s", w.Code, w.Header().Get("Location"))
	}
	var ticketCount int
	if err := srv.store.DB().QueryRow(`SELECT COUNT(*) FROM oidc_login_tickets`).Scan(&ticketCount); err != nil {
		t.Fatalf("统计 ticket 失败: %v", err)
	}
	if ticketCount != 0 {
		t.Fatalf("停用态回调不得签发 ticket，实际 %d", ticketCount)
	}
}

func getOidcDataForTestForR3107(t *testing.T, srv *Server, token, path string) map[string]any {
	t.Helper()
	w := profileReq(t, srv, http.MethodGet, path, token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s 应 200: %d %s", path, w.Code, w.Body.String())
	}
	var resp struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析 %s 响应失败: %v", path, err)
	}
	return resp.Data
}

// TestR3107HTTPOldStateAndTicketCannotResume 快速停用/重启用后，旧 state 行与旧 ticket 均不得恢复。
func TestR3107HTTPOldStateAndTicketCannotResume(t *testing.T) {
	srv := newDownloadTestServer(t)
	token := regUser(t, srv, "r3107-http-resume", "r3107-http-resume@example.com", "password123")
	enableOidcForTest(t, srv, token, "generic")
	ctx := context.Background()

	// 旧 ticket：停用/重启用后仍不可兑换。
	oldTicket, err := srv.oidcSvc.IssueLoginTicket(ctx, "old-session-token")
	if err != nil {
		t.Fatalf("签发旧 ticket 失败: %v", err)
	}
	if w := profileReq(t, srv, http.MethodPost, "/api/admin/settings/oidc/disable", token, nil); w.Code != http.StatusOK {
		t.Fatalf("停用失败: %d %s", w.Code, w.Body.String())
	}
	enableOidcForTest(t, srv, token, "generic")
	if w := exchangeWithTicket(srv, oldTicket); w.Code != http.StatusUnauthorized {
		t.Fatalf("重新启用后旧 ticket 应 401: %d %s", w.Code, w.Body.String())
	}

	// 旧 state：停用事务已清 row；重启用后 callback 必须 state_expired，且不签发 ticket。
	oldState := "r3107-old-state"
	if _, err := srv.store.DB().Exec(
		`INSERT INTO oidc_states (state, code_verifier, nonce, intent, provider_type, config_hash, redirect_uri)
		 VALUES (?, 'v','n','login','generic','r3107-old-hash','https://app.example.com/api/auth/oidc/callback')`, oldState); err != nil {
		t.Fatalf("写入旧 state 失败: %v", err)
	}
	rec, err := srv.oidcSvc.ConsumeState(ctx, oldState)
	if err != nil {
		t.Fatalf("ConsumeState 失败: %v", err)
	}
	if w := profileReq(t, srv, http.MethodPost, "/api/admin/settings/oidc/disable", token, nil); w.Code != http.StatusOK {
		t.Fatalf("第二次停用失败: %d %s", w.Code, w.Body.String())
	}
	enableOidcForTest(t, srv, token, "generic")
	if _, err := srv.oidcSvc.Exchange(ctx, rec, "code"); err == nil {
		t.Fatal("已消费的旧 state 在重新启用后仍被 Exchange 接受")
	}
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?state="+oldState+"&code=code", nil)
	req.AddCookie(&http.Cookie{Name: stateCookieName(), Value: oldState})
	w := httptest.NewRecorder()
	srv.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusFound || !strings.Contains(w.Header().Get("Location"), "state_expired") {
		t.Fatalf("旧 state 回调应 state_expired: code=%d location=%s", w.Code, w.Header().Get("Location"))
	}
	var tickets int
	if err := srv.store.DB().QueryRow(`SELECT COUNT(*) FROM oidc_login_tickets`).Scan(&tickets); err != nil {
		t.Fatalf("统计 ticket 失败: %v", err)
	}
	if tickets != 0 {
		t.Fatalf("旧 state/票据不得恢复，实际 ticket=%d", tickets)
	}

	// R31-06 残余：历史空指纹 ticket（旧 Dev 已签发）在任何配置下都不可兑换。
	if _, err := srv.store.DB().Exec(
		`INSERT INTO oidc_login_tickets (ticket, session_token, expires_at, flow_hash) VALUES ('r3107-legacy-ticket','legacy-session',datetime('now','+1 minute'),'')`); err != nil {
		t.Fatalf("写入历史 ticket 失败: %v", err)
	}
	if w := exchangeWithTicket(srv, "r3107-legacy-ticket"); w.Code != http.StatusUnauthorized {
		t.Fatalf("历史空指纹 ticket 应 401: %d %s", w.Code, w.Body.String())
	}
}

// TestR3107HTTPDisableRejectedWhenLocalLoginOff 本地登录关闭时停用端点必须 400 且配置不变。
func TestR3107HTTPDisableRejectedWhenLocalLoginOff(t *testing.T) {
	srv := newDownloadTestServer(t)
	token := regUser(t, srv, "r3107-http-deadlock", "r3107-http-deadlock@example.com", "password123")
	enableOidcForTest(t, srv, token, "generic")
	if w := profileReq(t, srv, http.MethodPut, "/api/admin/settings/local-auth", token, map[string]any{
		"allow_local_login": false, "allow_selfreg": false, "selfreg_approval": false,
	}); w.Code != http.StatusOK {
		t.Fatalf("关闭本地登录失败: %d %s", w.Code, w.Body.String())
	}
	if w := profileReq(t, srv, http.MethodPost, "/api/admin/settings/oidc/disable", token, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("本地登录关闭时停用应 400: %d %s", w.Code, w.Body.String())
	}
	var allow, configured string
	_ = srv.store.DB().QueryRow(`SELECT COALESCE((SELECT value FROM system_config WHERE key='allow_local_login'),'')`).Scan(&allow)
	_ = srv.store.DB().QueryRow(`SELECT COALESCE((SELECT value FROM system_config WHERE key='oidc_configured'),'')`).Scan(&configured)
	if allow != "false" || configured != "true" {
		t.Fatalf("停用拒绝后配置不得变化: allow=%q configured=%q", allow, configured)
	}
}

// TestR3107HTTPMockDirectLoginBlockedByDisable Dev mock 直连登录也必须受停用守卫。
func TestR3107HTTPMockDirectLoginBlockedByDisable(t *testing.T) {
	srv := newDownloadTestServer(t)
	token := regUser(t, srv, "r3107-http-mock", "r3107-http-mock@example.com", "password123")
	enableOidcForTest(t, srv, token, "mock")
	if w := profileReq(t, srv, http.MethodPost, "/api/admin/settings/oidc/disable", token, nil); w.Code != http.StatusOK {
		t.Fatalf("停用 mock OIDC 失败: %d %s", w.Code, w.Body.String())
	}
	body := map[string]any{"email": "r3107-mock@example.com", "email_verified": true}
	w := profileReq(t, srv, http.MethodPost, "/api/auth/oidc/mock/login", "", body)
	if w.Code != http.StatusForbidden {
		t.Fatalf("停用后 mock 直连登录应 403: %d %s", w.Code, w.Body.String())
	}
	var tickets int
	if err := srv.store.DB().QueryRow(`SELECT COUNT(*) FROM oidc_login_tickets`).Scan(&tickets); err != nil {
		t.Fatalf("统计 ticket 失败: %v", err)
	}
	if tickets != 0 {
		t.Fatalf("停用后 mock 直连登录不得签发 ticket，实际 %d", tickets)
	}
}
