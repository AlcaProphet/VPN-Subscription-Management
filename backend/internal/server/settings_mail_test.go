package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vpn-sub/internal/mail"
)

// TestMailTemplateRoutesAuthAndNoStore 四类路由必须 session+admin 双中间件且全部响应 no-store。
func TestMailTemplateRoutesAuthAndNoStore(t *testing.T) {
	srv := newTestServer(t)

	// 匿名 401 + no-store（no-store 中间件必须先于 session 校验执行）。
	w := doReq(t, srv, http.MethodGet, "/api/admin/settings/mail-templates")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("匿名应 401: %d %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("匿名 401 应带 no-store: %q", got)
	}

	adminToken := regUser(t, srv, "mail-admin", "mail-admin@example.com", "password123")
	w = profileReq(t, srv, http.MethodGet, "/api/admin/settings/mail-templates", adminToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("管理员 GET 应 200: %d %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("管理员 GET 应带 no-store: %q", got)
	}
	var getResp struct {
		Data struct {
			Templates []struct {
				ID    string `json:"id"`
				State string `json:"state"`
			} `json:"templates"`
			Limits struct {
				Subject int `json:"subject"`
				Body    int `json:"body"`
			} `json:"limits"`
			PreviewValues struct {
				SiteName string `json:"site_name"`
				LoginURL string `json:"login_url"`
				ResetURL string `json:"reset_url"`
			} `json:"preview_values"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("解析 GET 响应失败: %v", err)
	}
	if len(getResp.Data.Templates) != 5 {
		t.Fatalf("应返回 5 个固定模板: %+v", getResp.Data.Templates)
	}
	wantIDs := []string{"password_reset", "approval_approved", "approval_rejected", "welcome_local", "welcome_oidc"}
	for i, want := range wantIDs {
		if getResp.Data.Templates[i].ID != want || getResp.Data.Templates[i].State != "default" {
			t.Fatalf("模板顺序/状态异常: i=%d got=%+v want=%s", i, getResp.Data.Templates[i], want)
		}
	}
	if getResp.Data.Limits.Subject != 200 || getResp.Data.Limits.Body != 10000 {
		t.Fatalf("limits 元数据异常: %+v", getResp.Data.Limits)
	}
	if getResp.Data.PreviewValues.SiteName != "示例站点" ||
		getResp.Data.PreviewValues.LoginURL != "https://example.invalid/login?source=preview" ||
		getResp.Data.PreviewValues.ResetURL != "https://example.invalid/reset/example-token?source=preview" {
		t.Fatalf("preview_values 必须是固定合成值: %+v", getResp.Data.PreviewValues)
	}

	// 原 SMTP GET API 不受新增模板路由影响。
	w = profileReq(t, srv, http.MethodGet, "/api/admin/settings/smtp", adminToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("原 SMTP GET API 应保持 200: %d %s", w.Code, w.Body.String())
	}

	if err := srv.cfg.Set(context.Background(), "allow_selfreg", "true"); err != nil {
		t.Fatalf("开启自注册失败: %v", err)
	}
	userToken := regUser(t, srv, "mail-user", "mail-user@example.com", "password123")
	w = profileReq(t, srv, http.MethodGet, "/api/admin/settings/mail-templates", userToken, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("非管理员应 403: %d %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("403 应带 no-store: %q", got)
	}
}

// TestMailTemplateCRUDRestoreAndPreview 保存/恢复只影响目标键，preview 使用固定合成值且不应写库。
func TestMailTemplateCRUDRestoreAndPreview(t *testing.T) {
	srv := newTestServer(t)
	adminToken := regUser(t, srv, "mail-crud", "mail-crud@example.com", "password123")

	// 未知 kind → 400 + no-store。
	w := profileReq(t, srv, http.MethodPut, "/api/admin/settings/mail-templates/bogus", adminToken,
		map[string]string{"subject": "主题", "body": "正文"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("未知 kind 应 400: %d %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("400 应带 no-store: %q", got)
	}

	// 非法 JSON → 400。
	w = profileReq(t, srv, http.MethodPut, "/api/admin/settings/mail-templates/password_reset", adminToken, "not-json")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法 JSON 应 400: %d %s", w.Code, w.Body.String())
	}

	// 64 KiB 上限 → 413。
	big := strings.Repeat("a", 65<<10)
	w = profileReq(t, srv, http.MethodPut, "/api/admin/settings/mail-templates/password_reset", adminToken,
		map[string]string{"subject": "主题", "body": big})
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("超 64 KiB 应 413: %d %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("413 应带 no-store: %q", got)
	}
	// preview 同样受 64 KiB 限制。
	w = profileReq(t, srv, http.MethodPost, "/api/admin/settings/mail-templates/password_reset/preview", adminToken,
		map[string]string{"subject": "主题", "body": big})
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("preview 超 64 KiB 应 413: %d %s", w.Code, w.Body.String())
	}

	// 合法保存 → 200 customized；恢复 → 200 default；重复 DELETE 幂等。
	custom := map[string]string{"subject": "自定义主题", "body": "自定义正文 {{reset_url}}"}
	w = profileReq(t, srv, http.MethodPut, "/api/admin/settings/mail-templates/password_reset", adminToken, custom)
	if w.Code != http.StatusOK {
		t.Fatalf("合法保存应 200: %d %s", w.Code, w.Body.String())
	}
	var saved struct {
		Data struct {
			ID      string `json:"id"`
			Subject string `json:"subject"`
			Body    string `json:"body"`
			State   string `json:"state"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil {
		t.Fatalf("解析保存响应失败: %v", err)
	}
	if saved.Data.ID != "password_reset" || saved.Data.State != "customized" || saved.Data.Subject != custom["subject"] {
		t.Fatalf("保存响应异常: %+v", saved.Data)
	}

	// preview 使用固定合成值；不应写库。
	before := serverConfigCount(t, srv)
	w = profileReq(t, srv, http.MethodPost, "/api/admin/settings/mail-templates/password_reset/preview", adminToken, custom)
	if w.Code != http.StatusOK {
		t.Fatalf("preview 应 200: %d %s", w.Code, w.Body.String())
	}
	after := serverConfigCount(t, srv)
	if before != after {
		t.Fatalf("preview 不应写数据库: before=%d after=%d", before, after)
	}
	var preview struct {
		Data struct {
			Subject  string `json:"subject"`
			TextBody string `json:"text_body"`
			HTMLBody string `json:"html_body"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &preview); err != nil {
		t.Fatalf("解析 preview 响应失败: %v", err)
	}
	if preview.Data.Subject != custom["subject"] ||
		!strings.Contains(preview.Data.TextBody, "example.invalid/reset/example-token?source=preview") ||
		!strings.Contains(preview.Data.HTMLBody, "example.invalid/reset/example-token?source=preview") {
		t.Fatalf("preview 应使用固定合成值: %+v", preview.Data)
	}

	w = profileReq(t, srv, http.MethodDelete, "/api/admin/settings/mail-templates/password_reset", adminToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("恢复默认应 200: %d %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("DELETE 应带 no-store: %q", got)
	}
	w = profileReq(t, srv, http.MethodDelete, "/api/admin/settings/mail-templates/password_reset", adminToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("重复恢复应幂等 200: %d %s", w.Code, w.Body.String())
	}
}

// TestMailTemplateDamagedAndTargetKeyIsolation 损坏单项 200+warning+默认值，且保存/恢复只改目标键。
func TestMailTemplateDamagedAndTargetKeyIsolation(t *testing.T) {
	srv := newTestServer(t)
	adminToken := regUser(t, srv, "mail-damaged", "mail-damaged@example.com", "password123")
	ctx := context.Background()
	if _, err := srv.store.DB().ExecContext(ctx,
		`INSERT INTO system_config (key, value) VALUES ('mail_template_password_reset', '{broken-json')`); err != nil {
		t.Fatalf("写入损坏模板失败: %v", err)
	}

	w := profileReq(t, srv, http.MethodGet, "/api/admin/settings/mail-templates", adminToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("单项损坏时 GET 仍应 200: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Templates []struct {
				ID      string `json:"id"`
				Subject string `json:"subject"`
				Body    string `json:"body"`
				State   string `json:"state"`
				Warning string `json:"warning"`
			} `json:"templates"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析 GET 响应失败: %v", err)
	}
	if len(resp.Data.Templates) != 5 {
		t.Fatalf("应返回 5 个模板: %+v", resp.Data.Templates)
	}
	damaged := resp.Data.Templates[0]
	if damaged.ID != "password_reset" || damaged.State != "damaged" || damaged.Subject != "密码重置" ||
		!strings.Contains(damaged.Warning, "损坏") || strings.Contains(damaged.Body, "broken-json") {
		t.Fatalf("损坏项应返回默认值+warning 且不回显坏值: %+v", damaged)
	}

	before := serverConfigCount(t, srv)
	w = profileReq(t, srv, http.MethodPut, "/api/admin/settings/mail-templates/approval_approved", adminToken,
		map[string]string{"subject": "通过主题", "body": "通过正文 {{login_url}}"})
	if w.Code != http.StatusOK {
		t.Fatalf("保存 approval_approved 应 200: %d %s", w.Code, w.Body.String())
	}
	if after := serverConfigCount(t, srv); after != before+1 {
		t.Fatalf("保存应只新增目标键: before=%d after=%d", before, after)
	}
	var damagedRaw string
	if err := srv.store.DB().QueryRowContext(ctx,
		`SELECT value FROM system_config WHERE key = 'mail_template_password_reset'`).Scan(&damagedRaw); err != nil {
		t.Fatalf("读取损坏键失败: %v", err)
	}
	if damagedRaw != "{broken-json" {
		t.Fatalf("保存其他分支不得改写损坏键: %q", damagedRaw)
	}

	w = profileReq(t, srv, http.MethodDelete, "/api/admin/settings/mail-templates/approval_approved", adminToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("恢复 approval_approved 应 200: %d %s", w.Code, w.Body.String())
	}
	var approvalCount int
	if err := srv.store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM system_config WHERE key = 'mail_template_approval_approved'`).Scan(&approvalCount); err != nil {
		t.Fatalf("统计目标键失败: %v", err)
	}
	if approvalCount != 0 {
		t.Fatalf("恢复默认应删除目标键: %d", approvalCount)
	}
	if err := srv.store.DB().QueryRowContext(ctx,
		`SELECT value FROM system_config WHERE key = 'mail_template_password_reset'`).Scan(&damagedRaw); err != nil {
		t.Fatalf("恢复其他分支后损坏键应仍在: %v", err)
	}
}

func serverConfigCount(t *testing.T, srv *Server) int {
	t.Helper()
	var n int
	if err := srv.store.DB().QueryRow(`SELECT COUNT(*) FROM system_config`).Scan(&n); err != nil {
		t.Fatalf("统计 system_config 失败: %v", err)
	}
	return n
}

func rawProfileReq(srv *Server, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	srv.Engine().ServeHTTP(w, req)
	return w
}

// TestMailTemplateRequestStrictJSONAndLimit PUT/preview 必须在严格 JSON 和 64 KiB 边界前失败。
func TestMailTemplateRequestStrictJSONAndLimit(t *testing.T) {
	srv := newTestServer(t)
	adminToken := regUser(t, srv, "mail-strict", "mail-strict@example.com", "password123")

	invalidBodies := []struct {
		name string
		raw  string
	}{
		{"大小写 subject", `{"Subject":"x","Body":"{{reset_url}}"}`},
		{"大小写 body", `{"subject":"x","Body":"{{reset_url}}"}`},
		{"重复 subject", `{"subject":"x","body":"{{reset_url}}","subject":"y"}`},
		{"重复 body", `{"subject":"x","body":"{{reset_url}}","body":"y"}`},
		{"未知字段", `{"subject":"x","body":"{{reset_url}}","extra":1}`},
		{"非字符串", `{"subject":1,"body":"{{reset_url}}"}`},
		{"缺 body", `{"subject":"x"}`},
	}
	for _, tc := range invalidBodies {
		t.Run(tc.name, func(t *testing.T) {
			for _, path := range []string{
				"/api/admin/settings/mail-templates/password_reset",
				"/api/admin/settings/mail-templates/password_reset/preview",
			} {
				method := http.MethodPut
				if strings.HasSuffix(path, "/preview") {
					method = http.MethodPost
				}
				w := rawProfileReq(srv, method, path, adminToken, tc.raw)
				if w.Code != http.StatusBadRequest {
					t.Fatalf("%s %s 应 400: %d %s", method, path, w.Code, w.Body.String())
				}
			}
		})
	}

	validTpl, err := json.Marshal(mail.Template{
		Subject: "边界",
		Body:    strings.Repeat("a", 10000-len("{{reset_url}}")) + "{{reset_url}}",
	})
	if err != nil {
		t.Fatal(err)
	}
	const limit = 64 << 10
	exact := string(validTpl) + strings.Repeat(" ", limit-len(validTpl))
	if len(exact) != limit {
		t.Fatalf("构造边界请求失败: len=%d", len(exact))
	}
	w := rawProfileReq(srv, http.MethodPut, "/api/admin/settings/mail-templates/password_reset", adminToken, exact)
	if w.Code != http.StatusOK {
		t.Fatalf("64 KiB 精确边界应接受: %d %s", w.Code, w.Body.String())
	}
	over := exact + " "
	w = rawProfileReq(srv, http.MethodPut, "/api/admin/settings/mail-templates/password_reset", adminToken, over)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("64 KiB+1 应 413: %d %s", w.Code, w.Body.String())
	}
}
