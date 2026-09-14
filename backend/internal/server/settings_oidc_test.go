package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
