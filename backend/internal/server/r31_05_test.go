package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
)

// buildR3105ImportFile 构造真实 v2 加密导出文件（本地登录关闭、OIDC 参数完整，但 oidc_configured 可控）。
func buildR3105ImportFile(t *testing.T, oidcConfigured string) []byte {
	t.Helper()
	ctx := context.Background()
	dataDir := t.TempDir()
	st, err := store.Open(dataDir, "source.db")
	if err != nil {
		t.Fatalf("打开导出源库失败: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(ctx, downloadTestFS()); err != nil {
		t.Fatalf("迁移导出源库失败: %v", err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	signingKey := "r3105-http-test-signing-key"
	secretCipher, err := config.Encrypt([]byte("oidc-secret"), []byte(signingKey))
	if err != nil {
		t.Fatalf("加密测试 Secret 失败: %v", err)
	}
	for k, v := range map[string]string{
		config.KeyConfigured:      "true",
		config.KeyAllowLocalLogin: "false",
		config.KeySigningKey:      signingKey,
		"frontend_url":            "https://app.example.com",
		"oidc_configured":         oidcConfigured,
		"oidc_provider_type":      "generic",
		"oidc_params_generic": fmt.Sprintf(
			`{"base_url":"https://idp.example.com","client_id":"client","client_secret":%q}`, secretCipher),
	} {
		if err := cfg.Set(ctx, k, v); err != nil {
			t.Fatalf("写入导出源配置 %s 失败: %v", k, err)
		}
	}
	svc := config.NewExportService(st, cfg, dataDir, "prod", log.New("error", "console"))
	data, err := svc.Export(ctx, "export-pass-123")
	if err != nil {
		t.Fatalf("生成导入文件失败: %v", err)
	}
	return data
}

func multipartR3105ImportBody(t *testing.T, data []byte, confirmWord, disableWord string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", "r3105.enc")
	if err != nil {
		t.Fatalf("创建 file 字段失败: %v", err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatalf("写入文件字段失败: %v", err)
	}
	for k, v := range map[string]string{
		"password":             "export-pass-123",
		"confirm_word":         confirmWord,
		"disable_confirm_word": disableWord,
	} {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("写入字段 %s 失败: %v", k, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("关闭 multipart 失败: %v", err)
	}
	return buf.Bytes(), w.FormDataContentType()
}

func serverSystemConfigSnapshot(t *testing.T, st *store.Store) map[string]string {
	t.Helper()
	rows, err := st.DB().Query(`SELECT key, value FROM system_config`)
	if err != nil {
		t.Fatalf("读取 system_config 失败: %v", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			t.Fatalf("扫描 system_config 失败: %v", err)
		}
		out[k] = v
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("遍历 system_config 失败: %v", err)
	}
	return out
}

// TestR3105ImportHTTPRejectsBeforeOverwrite Setup/管理端 v2 导入都在覆盖前同步拒绝且配置不变。
func TestR3105ImportHTTPRejectsBeforeOverwrite(t *testing.T) {
	data := buildR3105ImportFile(t, "false")
	for _, tc := range []struct {
		name        string
		path        string
		needToken   bool
		disableWord string
	}{
		{name: "setup", path: "/api/setup/import", needToken: false, disableWord: ""},
		{name: "admin", path: "/api/admin/settings/import", needToken: true, disableWord: config.ConfirmWordDisable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := newImportTestServer(t)
			token := ""
			if tc.needToken {
				token = regUser(t, srv, "r3105-import-admin", "r3105-import-admin@example.com", "password123")
			}
			before := serverSystemConfigSnapshot(t, srv.store)
			body, ct := multipartR3105ImportBody(t, data, config.ConfirmWordImport, tc.disableWord)
			w := sendImportRequest(t, srv, tc.path, token, body, ct, nil)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("应在覆盖前返回 400: code=%d body=%s", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), "oidc_configured") {
				t.Fatalf("拒绝原因应指向 oidc_configured: %s", w.Body.String())
			}
			after := serverSystemConfigSnapshot(t, srv.store)
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("拒绝导入后配置应不变:\nbefore=%v\nafter=%v", before, after)
			}
		})
	}
}

// TestR3105AddressSaveImmediateAndClearHTTP 地址保存即时生效、独立回调优先、显式清除回退，且响应不再返回 need_restart。
func TestR3105AddressSaveImmediateAndClearHTTP(t *testing.T) {
	srv := newDownloadTestServer(t)
	token := regUser(t, srv, "r3105-address-admin", "r3105-address-admin@example.com", "password123")
	put := func(body map[string]any) *httptest.ResponseRecorder {
		return profileReq(t, srv, http.MethodPut, "/api/admin/settings/oidc", token, body)
	}
	get := func() map[string]any {
		w := profileReq(t, srv, http.MethodGet, "/api/admin/settings/oidc", token, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("GET OIDC 失败: %d %s", w.Code, w.Body.String())
		}
		var resp struct {
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("解析 OIDC 响应失败: %v", err)
		}
		return resp.Data
	}
	w := put(map[string]any{
		"provider_type": "generic", "base_url": "https://idp.example.com", "realm": "",
		"client_id": "client", "client_secret": "secret",
		"frontend_url": "https://app.example.com", "callback_url": "",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("保存 OIDC 失败: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "need_restart") {
		t.Fatalf("保存响应不应再返回 need_restart: %s", w.Body.String())
	}
	if got := get(); got["frontend_url"] != "https://app.example.com" {
		t.Fatalf("前端地址应即时生效: %+v", got)
	}

	independent := "https://callback.example.com/api/auth/oidc/callback"
	w = put(map[string]any{
		"provider_type": "generic", "base_url": "https://idp.example.com", "realm": "",
		"client_id": "client", "client_secret": "", "callback_url": independent,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("保存独立回调失败: %d %s", w.Code, w.Body.String())
	}
	if got := get(); got["callback_url"] != independent {
		t.Fatalf("独立回调应优先保存并回显: %+v", got)
	}

	// 错误路径拒绝且不改变已存独立回调。
	w = put(map[string]any{
		"provider_type": "generic", "base_url": "https://idp.example.com", "realm": "",
		"client_id": "client", "client_secret": "", "callback_url": "https://callback.example.com/wrong",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("错误回调路径应 400: %d %s", w.Code, w.Body.String())
	}
	if got := get(); got["callback_url"] != independent {
		t.Fatalf("拒绝后独立回调不应变化: %+v", got)
	}

	// 显式清除：空 callback_url 不修改，clear_callback_url=true 才清除并回退推导。
	w = put(map[string]any{
		"provider_type": "generic", "base_url": "https://idp.example.com", "realm": "",
		"client_id": "client", "client_secret": "", "frontend_url": "https://new.example.com",
		"callback_url": "", "clear_callback_url": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("清除独立回调失败: %d %s", w.Code, w.Body.String())
	}
	got := get()
	if got["callback_url"] != "" || got["frontend_url"] != "https://new.example.com" {
		t.Fatalf("清除后应回退推导且前端地址即时更新: %+v", got)
	}
}

// TestR3105CallbackRouteMatchesConstant 真实注册回调路由必须与地址校验使用的常量一致。
func TestR3105CallbackRouteMatchesConstant(t *testing.T) {
	srv := newDownloadTestServer(t)
	for _, route := range srv.Engine().Routes() {
		if route.Method == http.MethodGet && route.Path == config.OidcCallbackPath {
			return
		}
	}
	t.Fatalf("未找到真实回调路由 GET %s", config.OidcCallbackPath)
}

// TestR3105AddressSaveImmediateHomeConsumer 通过管理端保存 frontend_url 后，首页下载链接消费者立即使用新地址。
func TestR3105AddressSaveImmediateHomeConsumer(t *testing.T) {
	srv := newDownloadTestServer(t)
	token := regUser(t, srv, "r3105-home-admin", "r3105-home-admin@example.com", "password123")
	seedPlatformWithVersion(t, srv, "yaml")
	w := profileReq(t, srv, http.MethodPut, "/api/admin/settings/oidc", token, map[string]any{
		"provider_type": "generic", "base_url": "https://idp.example.com", "realm": "",
		"client_id": "client", "client_secret": "secret",
		"frontend_url": "https://new-site.example.com", "callback_url": "",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("保存 frontend_url 失败: %d %s", w.Code, w.Body.String())
	}
	if _, err := srv.store.DB().Exec(`UPDATE users SET role='user' WHERE id=1`); err != nil {
		t.Fatalf("降级测试用户失败: %v", err)
	}
	resp := profileReq(t, srv, http.MethodGet, "/api/home/platforms", token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("首页平台列表失败: %d %s", resp.Code, resp.Body.String())
	}
	var got struct {
		Data struct {
			List []map[string]any `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("解析首页响应失败: %v", err)
	}
	if len(got.Data.List) != 1 {
		t.Fatalf("首页平台数量异常: %+v", got.Data.List)
	}
	downloadURL, _ := got.Data.List[0]["download_url"].(string)
	if !strings.HasPrefix(downloadURL, "https://new-site.example.com/") {
		t.Fatalf("下载链接应立即使用新 frontend_url: %q", downloadURL)
	}
}
