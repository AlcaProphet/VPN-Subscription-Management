package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"vpn-sub/internal/config"
	"vpn-sub/internal/mail"
)

func TestSMTPPasswordResetAvailabilityField(t *testing.T) {
	srv := newTestServer(t)
	adminToken := regUser(t, srv, "smtp-avail-admin", "smtp-avail-admin@example.com", "password123")
	ctx := context.Background()

	read := func() bool {
		w := profileReq(t, srv, http.MethodGet, "/api/admin/settings/smtp", adminToken, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("读取 SMTP 设置应 200: %d %s", w.Code, w.Body.String())
		}
		var out struct {
			Data struct {
				PasswordResetAvailable bool `json:"password_reset_available"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}
		return out.Data.PasswordResetAvailable
	}

	if read() {
		t.Fatal("未配置时 password_reset_available 应为 false")
	}
	for k, v := range map[string]string{
		mail.KeyHost: "127.0.0.1", mail.KeyPort: "2525", mail.KeyFrom: "sender@example.com",
		mail.KeySecurity: config.SMTPSecurityPlain, mail.KeyAuth: "false",
		mail.KeyScopes: `["password_reset"]`,
	} {
		if err := srv.cfg.Set(ctx, k, v); err != nil {
			t.Fatalf("配置 %s 失败: %v", k, err)
		}
	}
	if !read() {
		t.Fatal("SMTP 完整且 password_reset 启用时可用性应为 true")
	}
}
