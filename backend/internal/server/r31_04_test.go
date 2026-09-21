package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/config"
	vpnlog "vpn-sub/internal/log"
)

// setOidcRawForTest 直接写入指定提供商的原始参数 JSON，用于构造 R31-04 隔离损坏数据。
func setOidcRawForTest(t *testing.T, srv *Server, providerType, raw string) {
	t.Helper()
	if _, err := srv.store.DB().Exec(
		`INSERT INTO system_config(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		"oidc_params_"+providerType, raw); err != nil {
		t.Fatalf("写入 %s raw 失败: %v", providerType, err)
	}
}

// encryptOidcForServerTest 用服务器当前签名密钥加密测试明文。
func encryptOidcForServerTest(t *testing.T, srv *Server, plain string) string {
	t.Helper()
	var key string
	if err := srv.store.DB().QueryRow(`SELECT value FROM system_config WHERE key = ?`, config.KeySigningKey).Scan(&key); err != nil {
		t.Fatalf("读取签名密钥失败: %v", err)
	}
	enc, err := config.Encrypt([]byte(plain), []byte(key))
	if err != nil {
		t.Fatalf("加密测试值失败: %v", err)
	}
	return enc
}

func getOidcDataForTest(t *testing.T, srv *Server, token, path string) (int, map[string]any, string) {
	t.Helper()
	w := profileReq(t, srv, http.MethodGet, path, token, nil)
	body := w.Body.String()
	var resp struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析 OIDC GET 响应失败: %v body=%s", err, body)
	}
	return w.Code, resp.Data, body
}

// TestR3104OidcStateHTTP 隔离数据覆盖六态 GET：字段回显、Secret 空、损坏专门标记、响应不泄露原始密文/占位符。
func TestR3104OidcStateHTTP(t *testing.T) {
	srv := newDownloadTestServer(t)
	token := regUser(t, srv, "r3104-admin", "r3104-admin@example.com", "password123")

	put := func(body map[string]any) *httptest.ResponseRecorder {
		return profileReq(t, srv, http.MethodPut, "/api/admin/settings/oidc", token, body)
	}
	base := map[string]any{
		"provider_type": "generic", "base_url": "https://idp.example.com", "realm": "",
		"client_id": "c", "client_secret": "normal-secret", "frontend_url": "", "callback_url": "",
	}
	if w := put(base); w.Code != http.StatusOK {
		t.Fatalf("首次保存 OIDC 失败: %d %s", w.Code, w.Body.String())
	}

	assertState := func(name, providerType, raw, wantState string, wantBaseURL, wantClientID string) {
		t.Helper()
		setOidcRawForTest(t, srv, providerType, raw)
		code, data, body := getOidcDataForTest(t, srv, token, "/api/admin/settings/oidc")
		if code != http.StatusOK {
			t.Fatalf("%s GET code=%d body=%s", name, code, body)
		}
		if data["params_state"] != wantState {
			t.Fatalf("%s params_state=%v want=%s data=%+v", name, data["params_state"], wantState, data)
		}
		if data["client_secret"] != "" {
			t.Fatalf("%s client_secret 必须为空: %+v", name, data)
		}
		if got := data["base_url"]; got != wantBaseURL {
			t.Fatalf("%s base_url=%v want=%q data=%+v", name, got, wantBaseURL, data)
		}
		if got := data["client_id"]; got != wantClientID {
			t.Fatalf("%s client_id=%v want=%q data=%+v", name, got, wantClientID, data)
		}
		if strings.Contains(body, "not-a-cipher") || strings.Contains(body, config.MaskedSecret) {
			t.Fatalf("%s 响应不得包含 Secret 明文/占位符或原始密文: %s", name, body)
		}
		if strings.Contains(body, "normal-secret") {
			t.Fatalf("%s 响应不得包含 Secret 明文: %s", name, body)
		}
	}

	normalCipher := encryptOidcForServerTest(t, srv, "normal-secret")
	assertState("空 JSON", "generic", "", string(config.OidcParamsNotConfigured), "", "")
	assertState("空对象", "generic", "{}", string(config.OidcParamsMissingSecret), "", "")
	assertState("坏 JSON", "generic", "{", string(config.OidcParamsJSONDamaged), "", "")
	assertState("空 Secret", "generic", `{"base_url":"https://idp.example.com","realm":"","client_id":"c","client_secret":""}`,
		string(config.OidcParamsMissingSecret), "https://idp.example.com", "c")
	assertState("字面 ***", "generic", `{"base_url":"https://idp.example.com","realm":"","client_id":"c","client_secret":"***"}`,
		string(config.OidcParamsSecretDamaged), "https://idp.example.com", "c")
	assertState("非法密文", "generic", `{"base_url":"https://idp.example.com","realm":"","client_id":"c","client_secret":"not-a-cipher"}`,
		string(config.OidcParamsSecretDamaged), "https://idp.example.com", "c")
	assertState("加密后 ***", "generic", `{"base_url":"https://idp.example.com","realm":"","client_id":"c","client_secret":"`+encryptOidcForServerTest(t, srv, config.MaskedSecret)+`"}`,
		string(config.OidcParamsSecretDamaged), "https://idp.example.com", "c")
	assertState("正常密文", "generic", `{"base_url":"https://idp.example.com","realm":"","client_id":"c","client_secret":"`+normalCipher+`"}`,
		string(config.OidcParamsUsable), "https://idp.example.com", "c")

	// 目标读取也必须只回显目标字段，不把源提供商字段混入。
	setOidcRawForTest(t, srv, "keycloak", `{"base_url":"https://kc.example.com","realm":"master","client_id":"kc","client_secret":"***"}`)
	code, data, body := getOidcDataForTest(t, srv, token, "/api/admin/settings/oidc?provider_type=keycloak")
	if code != http.StatusOK || data["params_state"] != string(config.OidcParamsSecretDamaged) ||
		data["base_url"] != "https://kc.example.com" || data["client_id"] != "kc" ||
		data["base_url"] == "https://idp.example.com" {
		t.Fatalf("目标读取字段隔离/损坏状态异常: code=%d data=%+v body=%s", code, data, body)
	}
}

// TestR3104SaveOidcTransactionRollback T1：参数已写入后 configured 写入失败，整体回滚不留半套配置。
func TestR3104SaveOidcTransactionRollback(t *testing.T) {
	srv := newDownloadTestServer(t)
	token := regUser(t, srv, "r3104-rollback", "r3104-rollback@example.com", "password123")

	put := func(body map[string]any) *httptest.ResponseRecorder {
		return profileReq(t, srv, http.MethodPut, "/api/admin/settings/oidc", token, body)
	}
	if w := put(map[string]any{
		"provider_type": "generic", "base_url": "https://source.example.com", "realm": "",
		"client_id": "source", "client_secret": "source-secret", "frontend_url": "", "callback_url": "",
	}); w.Code != http.StatusOK {
		t.Fatalf("初始保存失败: %d %s", w.Code, w.Body.String())
	}
	sourceRaw := oidcRawParam(t, srv, "generic")

	// 在事务已写入参数和 provider_type 后，于 configured 更新点注入失败。
	if _, err := srv.store.DB().Exec(`
		CREATE TRIGGER r3104_fail_oidc_configured
		BEFORE UPDATE ON system_config
		WHEN NEW.key = 'oidc_configured'
		BEGIN
			SELECT RAISE(ABORT, 'r3104 injected failure');
		END;`); err != nil {
		t.Fatalf("创建失败注入触发器失败: %v", err)
	}
	t.Cleanup(func() { _, _ = srv.store.DB().Exec(`DROP TRIGGER IF EXISTS r3104_fail_oidc_configured`) })

	w := put(map[string]any{
		"provider_type": "keycloak", "base_url": "https://target.example.com", "realm": "master",
		"client_id": "target", "client_secret": "target-secret",
		"frontend_url": "https://new-site.example.com", "callback_url": "https://new-site.example.com/api/auth/oidc/callback",
	})
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("注入失败应返回 500: code=%d body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "r3104 injected") {
		t.Fatalf("500 响应不得回显底层注入错误: %s", w.Body.String())
	}
	if got := oidcRawParam(t, srv, "generic"); got != sourceRaw {
		t.Fatalf("回滚后源参数不应变化: before=%s after=%s", sourceRaw, got)
	}
	var targetCount int
	if err := srv.store.DB().QueryRow(`SELECT COUNT(*) FROM system_config WHERE key = 'oidc_params_keycloak'`).Scan(&targetCount); err != nil {
		t.Fatalf("查询 target 参数失败: %v", err)
	}
	if targetCount != 0 {
		t.Fatal("回滚后不得写入目标参数")
	}
	var providerType, configured, frontendURL string
	if err := srv.store.DB().QueryRow(`SELECT value FROM system_config WHERE key = 'oidc_provider_type'`).Scan(&providerType); err != nil {
		t.Fatalf("查询 provider_type 失败: %v", err)
	}
	if err := srv.store.DB().QueryRow(`SELECT COALESCE(value, '') FROM system_config WHERE key = 'oidc_configured'`).Scan(&configured); err != nil {
		t.Fatalf("查询 oidc_configured 失败: %v", err)
	}
	if err := srv.store.DB().QueryRow(`SELECT COALESCE((SELECT value FROM system_config WHERE key = 'frontend_url'), '')`).Scan(&frontendURL); err != nil {
		t.Fatalf("查询 frontend_url 失败: %v", err)
	}
	if providerType != "generic" || configured != "true" || frontendURL != "" {
		t.Fatalf("失败后配置出现半套生效: provider=%q configured=%q frontend=%q", providerType, configured, frontendURL)
	}

	// 移除注入后重试，确认回滚不破坏后续正常写入。
	if _, err := srv.store.DB().Exec(`DROP TRIGGER IF EXISTS r3104_fail_oidc_configured`); err != nil {
		t.Fatalf("移除触发器失败: %v", err)
	}
	if w := put(map[string]any{
		"provider_type": "keycloak", "base_url": "https://target.example.com", "realm": "master",
		"client_id": "target", "client_secret": "target-secret", "frontend_url": "", "callback_url": "",
	}); w.Code != http.StatusOK {
		t.Fatalf("回滚后重试应成功: %d %s", w.Code, w.Body.String())
	}
}

// TestR3104SigningKeyFaultErrorMapping 直接验证 mapSettingsErr：signing_key 故障固定安全 503，响应与日志均脱敏。
func TestR3104SigningKeyFaultErrorMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logBuf bytes.Buffer
	lg := slog.New(vpnlog.NewRedactHandler(slog.NewTextHandler(&logBuf, nil)))
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPut, "/api/admin/settings/oidc", nil)
	c.Request = req.WithContext(vpnlog.WithLogger(req.Context(), lg))
	internalErr := fmt.Errorf("internal marker secret=VERY_SECRET_CIPHER: %w", config.ErrSigningKeyUnavailable)
	mapSettingsErr(c, internalErr)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("signing_key 故障应返回 503: code=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, config.OidcSigningKeyFaultPublicMessage) {
		t.Fatalf("503 响应应包含固定安全提示: %s", body)
	}
	if strings.Contains(body, "VERY_SECRET_CIPHER") || strings.Contains(body, "internal marker") {
		t.Fatalf("503 响应不得泄露内部错误详情: %s", body)
	}
	logs := logBuf.String()
	if strings.Contains(logs, "VERY_SECRET_CIPHER") {
		t.Fatalf("日志脱敏后不得出现凭据值: %s", logs)
	}
	if !strings.Contains(logs, "secret=***") {
		t.Fatalf("日志应经脱敏规则记录错误类别: %s", logs)
	}
	if !errors.Is(internalErr, config.ErrSigningKeyUnavailable) {
		t.Fatal("测试内部错误应保留 sentinel 包装")
	}
}
