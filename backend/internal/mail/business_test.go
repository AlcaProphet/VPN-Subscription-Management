package mail

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"strings"
	"testing"
)

func readCapturedBusinessMail(t *testing.T, raw []byte) (subject, textBody string) {
	t.Helper()
	parsed, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("解析业务邮件失败: %v", err)
	}
	media, params, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	if err != nil || media != "multipart/alternative" || params["boundary"] == "" {
		t.Fatalf("业务邮件应为 multipart/alternative: %q err=%v", parsed.Header.Get("Content-Type"), err)
	}
	mr := multipart.NewReader(parsed.Body, params["boundary"])
	first, err := mr.NextRawPart()
	if err != nil {
		t.Fatalf("读取纯文本正文失败: %v", err)
	}
	body, err := io.ReadAll(first)
	if err != nil {
		t.Fatal(err)
	}
	if got := parsePartMediaType(t, first.Header.Get("Content-Type")); got != "text/plain" {
		t.Fatalf("第一正文部分应为 text/plain: %q", got)
	}
	return decodeSubject(t, parsed.Header), readQP(t, bytes.NewReader(body))
}

// TestBusinessMailTemplateSelectionAndValues 五分支真正映射到固定模板与 RenderValues；
// 审批通过只走 approval_approved，拒绝走 approval_rejected，欢迎按来源分派 local/OIDC。
func TestBusinessMailTemplateSelectionAndValues(t *testing.T) {
	_, svc := newTestMail(t)
	ctx := context.Background()
	if err := svc.cfg.Set(ctx, KeyScopes, `["welcome","approval_notify","password_reset"]`); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		kind TemplateKind
		tpl  Template
	}{
		{TemplateWelcomeOIDC, Template{Subject: "OIDC自定义", Body: "OIDC欢迎 {{login_url}}"}},
		{TemplateWelcomeLocal, Template{Subject: "本地自定义", Body: "本地欢迎 {{login_url}}"}},
		{TemplateApprovalApproved, Template{Subject: "通过自定义", Body: "通过 {{login_url}}"}},
		{TemplateApprovalRejected, Template{Subject: "拒绝自定义", Body: "拒绝 {{site_name}}"}},
		{TemplatePasswordReset, Template{Subject: "重置自定义", Body: "重置 {{reset_url}}"}},
	} {
		if _, err := svc.SaveTemplate(ctx, item.kind, item.tpl); err != nil {
			t.Fatalf("保存模板 %s 失败: %v", item.kind, err)
		}
	}
	const (
		siteName = "测试站点"
		loginURL = "https://example.invalid/login?x=1"
		resetURL = "https://example.invalid/reset#token=abc"
	)
	cases := []struct {
		name        string
		invoke      func(to string) error
		wantSubject string
		wantContain string
	}{
		{
			name:        "welcome oidc",
			invoke:      func(to string) error { return svc.SendWelcome(ctx, to, siteName, loginURL, "oidc") },
			wantSubject: "OIDC自定义",
			wantContain: loginURL,
		},
		{
			name:        "welcome local",
			invoke:      func(to string) error { return svc.SendWelcome(ctx, to, siteName, loginURL, "selfreg") },
			wantSubject: "本地自定义",
			wantContain: loginURL,
		},
		{
			name:        "approval approved",
			invoke:      func(to string) error { return svc.SendApprovalApproved(ctx, to, siteName, loginURL) },
			wantSubject: "通过自定义",
			wantContain: loginURL,
		},
		{
			name:        "approval rejected",
			invoke:      func(to string) error { return svc.SendApprovalRejected(ctx, to, siteName) },
			wantSubject: "拒绝自定义",
			wantContain: siteName,
		},
		{
			name:        "password reset",
			invoke:      func(to string) error { return svc.SendPasswordReset(ctx, to, resetURL) },
			wantSubject: "重置自定义",
			wantContain: resetURL,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			addr, captured := captureSMTPOnce(t)
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				t.Fatal(err)
			}
			for k, v := range map[string]string{
				KeyHost: host, KeyPort: port, KeyFrom: "sender@example.com",
				KeySecurity: "plain", KeyAuth: "false",
			} {
				if err := svc.cfg.Set(ctx, k, v); err != nil {
					t.Fatal(err)
				}
			}
			if err := tc.invoke("recipient@example.com"); err != nil {
				t.Fatalf("%s 发送失败: %v", tc.name, err)
			}
			subject, textBody := readCapturedBusinessMail(t, <-captured)
			if subject != tc.wantSubject {
				t.Fatalf("模板选择/主题不符: got=%q want=%q", subject, tc.wantSubject)
			}
			if !strings.Contains(textBody, tc.wantContain) {
				t.Fatalf("实际值未进入纯文本正文: got=%q wantContain=%q", textBody, tc.wantContain)
			}
		})
	}
}

// TestBusinessMailInvalidRequiredURLDoesNotDial 必需链接为空/非法时必须在拨号前失败。
func TestBusinessMailInvalidRequiredURLDoesNotDial(t *testing.T) {
	st, svc := newTestMail(t)
	ctx := context.Background()
	if err := svc.cfg.Set(ctx, KeyScopes, `["password_reset"]`); err != nil {
		t.Fatal(err)
	}
	dialed := false
	cfg := svc.cfg
	svc = newServiceWithDialContext(cfg, svc.log, func(context.Context, string, string) (net.Conn, error) {
		dialed = true
		return nil, io.EOF
	})
	err := svc.SendPasswordReset(ctx, "recipient@example.com", "")
	if err == nil {
		t.Fatal("空重置链接应失败")
	}
	if dialed {
		t.Fatal("非法必需链接不得进入 SMTP 拨号阶段")
	}
	_ = st
}
