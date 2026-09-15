package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	stdmail "net/mail"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/mail"
	"vpn-sub/internal/store"
	"vpn-sub/migrations"
)

// startSmokeSMTPServer 启动仅监听回环地址的本地 SMTP mock，捕获 DATA 报文字节。
func startSmokeSMTPServer(t *testing.T) (host, port string, messages <-chan []byte) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("启动 SMTP mock 失败: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	msgCh := make(chan []byte, 4)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				reader := bufio.NewReader(conn)
				_, _ = conn.Write([]byte("220 smoke SMTP\r\n"))
				inData := false
				var data bytes.Buffer
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					if inData {
						if line == ".\r\n" {
							inData = false
							msgCh <- data.Bytes()
							_, _ = conn.Write([]byte("250 queued\r\n"))
							continue
						}
						data.WriteString(line)
						continue
					}
					switch {
					case strings.HasPrefix(line, "EHLO "):
						_, _ = conn.Write([]byte("250 smoke\r\n"))
					case strings.HasPrefix(line, "MAIL FROM:"), strings.HasPrefix(line, "RCPT TO:"):
						_, _ = conn.Write([]byte("250 ok\r\n"))
					case line == "DATA\r\n":
						inData = true
						_, _ = conn.Write([]byte("354 send\r\n"))
					case line == "QUIT\r\n":
						_, _ = conn.Write([]byte("221 bye\r\n"))
						return
					default:
						_, _ = conn.Write([]byte("250 ok\r\n"))
					}
				}
			}(conn)
		}
	}()
	host, port, err = net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	return host, port, msgCh
}

func readSmokeTextPart(t *testing.T, msg []byte) (subject, textBody string) {
	t.Helper()
	parsed, err := stdmail.ReadMessage(bytes.NewReader(msg))
	if err != nil {
		t.Fatalf("解析 SMTP 报文失败: %v", err)
	}
	dec := new(mime.WordDecoder)
	subject, err = dec.DecodeHeader(parsed.Header.Get("Subject"))
	if err != nil {
		t.Fatalf("解码主题失败: %v", err)
	}
	media, params, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	if err != nil || media != "multipart/alternative" || params["boundary"] == "" {
		t.Fatalf("业务邮件应为 multipart/alternative: %q err=%v", parsed.Header.Get("Content-Type"), err)
	}
	mr := multipart.NewReader(parsed.Body, params["boundary"])
	part, err := mr.NextPart()
	if err != nil {
		t.Fatalf("读取 text/plain 部分失败: %v", err)
	}
	raw, err := io.ReadAll(part)
	if err != nil {
		t.Fatal(err)
	}
	return subject, string(raw)
}

// TestMailTemplateIsolatedSmoke 使用隔离临时库 + 本地 SMTP mock 覆盖 API、实际 MIME、审批一封、
// 模板预览零写入和配置导出/导入往返。
func TestMailTemplateIsolatedSmoke(t *testing.T) {
	srv := newImportTestServer(t)
	ctx := context.Background()
	host, port, messages := startSmokeSMTPServer(t)
	for k, v := range map[string]string{
		mail.KeyHost: host, mail.KeyPort: port, mail.KeyFrom: "smoke@example.com",
		mail.KeySecurity: config.SMTPSecurityPlain, config.SMTPAuthRequiredKey: "false",
		mail.KeyScopes: `["approval_notify"]`,
		"site_name":    "Smoke 站点",
		"frontend_url": "https://app.example.com",
	} {
		if err := srv.cfg.Set(ctx, k, v); err != nil {
			t.Fatalf("配置 %s 失败: %v", k, err)
		}
	}
	adminToken := regUser(t, srv, "smoke-admin", "smoke-admin@example.com", "password123")

	custom := map[string]string{"subject": "审批通过自定义", "body": "请点击登录：{{login_url}}"}
	w := profileReq(t, srv, http.MethodPut, "/api/admin/settings/mail-templates/approval_approved", adminToken, custom)
	if w.Code != http.StatusOK {
		t.Fatalf("保存模板应 200: %d %s", w.Code, w.Body.String())
	}

	beforeCount := serverConfigCount(t, srv)
	w = profileReq(t, srv, http.MethodPost, "/api/admin/settings/mail-templates/password_reset/preview", adminToken,
		map[string]string{"subject": "重置预览", "body": "请打开 {{reset_url}}"})
	if w.Code != http.StatusOK {
		t.Fatalf("预览应 200: %d %s", w.Code, w.Body.String())
	}
	var previewResp struct {
		Data struct {
			TextBody string `json:"text_body"`
			HTMLBody string `json:"html_body"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &previewResp); err != nil {
		t.Fatalf("解析预览响应失败: %v", err)
	}
	if !strings.Contains(previewResp.Data.TextBody, "example.invalid/reset/example-token?source=preview") ||
		!strings.Contains(previewResp.Data.HTMLBody, "example.invalid/reset/example-token?source=preview") {
		t.Fatalf("预览应使用固定合成值: %+v", previewResp.Data)
	}
	if afterCount := serverConfigCount(t, srv); afterCount != beforeCount {
		t.Fatalf("预览不得写库: before=%d after=%d", beforeCount, afterCount)
	}

	res, err := srv.store.DB().ExecContext(ctx,
		`INSERT INTO users (username, email, role, user_source, status) VALUES ('smoke-user','smoke-user@example.com','user','selfreg','pending')`)
	if err != nil {
		t.Fatalf("写入待审批用户失败: %v", err)
	}
	userID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	w = profileReq(t, srv, http.MethodPost, fmt.Sprintf("/api/admin/approvals/%d/approve", userID), adminToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("审批通过应 200: %d %s", w.Code, w.Body.String())
	}
	var rawMsg []byte
	select {
	case rawMsg = <-messages:
	case <-time.After(2 * time.Second):
		t.Fatal("未在超时内收到审批通过邮件")
	}
	subject, textBody := readSmokeTextPart(t, rawMsg)
	if subject != "审批通过自定义" || !strings.Contains(textBody, "https://app.example.com") {
		t.Fatalf("实际 MIME 内容异常: subject=%q body=%q", subject, textBody)
	}
	select {
	case extra := <-messages:
		t.Fatalf("审批通过不得叠发第二封邮件: %q", extra)
	case <-time.After(200 * time.Millisecond):
	}

	// 配置往返：同一临时库导出，导入第二个临时全量库，模板覆盖必须保留。
	exportSvc := config.NewExportService(srv.store, srv.cfg, t.TempDir(), "prod", log.New("error", "console"))
	exportSvc.SetValidateConfig(mail.ValidateTemplateOverrides)
	data, err := exportSvc.Export(ctx, "export-pass-123")
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	targetDir := t.TempDir()
	targetSt, err := store.Open(targetDir, "test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = targetSt.Close() })
	if err := targetSt.Migrate(ctx, migrations.FS); err != nil {
		t.Fatalf("目标库迁移失败: %v", err)
	}
	targetCfg := config.NewService(targetSt, log.New("error", "console"))
	targetSvc := config.NewExportService(targetSt, targetCfg, targetDir, "prod", log.New("error", "console"))
	targetSvc.SetValidateConfig(mail.ValidateTemplateOverrides)
	if err := targetSvc.Import(ctx, data, "export-pass-123", config.ConfirmWordImport, false); err != nil {
		t.Fatalf("配置往返导入失败: %v", err)
	}
	targetMail := mail.NewService(targetCfg, log.New("error", "console"))
	tpl, state, err := targetMail.LoadTemplate(ctx, mail.TemplateApprovalApproved)
	if err != nil || state != mail.TemplateStateCustomized || tpl.Subject != custom["subject"] || tpl.Body != custom["body"] {
		t.Fatalf("配置往返后模板覆盖未保留: tpl=%+v state=%s err=%v", tpl, state, err)
	}
}
