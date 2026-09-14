package mail

import (
	"bufio"
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
)

func TestSendErrorRedactsServerReply(t *testing.T) {
	err := &sendError{stage: "认证", err: errors.New("535 password=secret@example.com")}
	if got := err.Error(); got != "SMTP 认证失败" {
		t.Fatalf("服务商响应不应直接回显: %q", got)
	}
	if !strings.Contains(err.Unwrap().Error(), "535") {
		t.Fatal("内部诊断原因应保留")
	}
}

// TestStartTLSRequired 未宣告 STARTTLS 的服务器不能继续进入认证或发送阶段。
func TestStartTLSRequired(t *testing.T) {
	st, svc := newTestMail(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err.Error()
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		_, _ = conn.Write([]byte("220 mock SMTP\r\n"))
		line, err := reader.ReadString('\n')
		if err != nil {
			done <- err.Error()
			return
		}
		if !strings.HasPrefix(line, "EHLO ") {
			done <- line
			return
		}
		_, _ = conn.Write([]byte("250 mock\r\n"))
		line, _ = reader.ReadString('\n')
		done <- line
	}()
	ctx := context.Background()
	cfg := config.NewService(st, log.New("error", "console"))
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	for k, v := range map[string]string{KeyHost: host, KeyPort: port, KeyUser: "sender@example.com", KeyPassword: "secret", KeyFrom: "sender@example.com", KeySecurity: "starttls", KeyAuth: "true"} {
		if err := cfg.Set(ctx, k, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.SendTest(ctx, "recipient@example.com"); err == nil || !strings.Contains(err.Error(), "未提供 STARTTLS") {
		t.Fatalf("应拒绝未升级连接: %v", err)
	}
	if line := <-done; line != "" {
		t.Fatalf("拒绝后仍发送了 SMTP 命令: %q", line)
	}
}

// TestPlainLoopbackRelay 即使本地中继宣告 STARTTLS，也不发送 AUTH 或自动升级。
func TestPlainLoopbackRelay(t *testing.T) {
	st, svc := newTestMail(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err.Error()
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		_, _ = conn.Write([]byte("220 mock SMTP\r\n"))
		var commands strings.Builder
		inData := false
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				done <- err.Error()
				return
			}
			commands.WriteString(line)
			if inData {
				if line == ".\r\n" {
					inData = false
					_, _ = conn.Write([]byte("250 queued\r\n"))
				}
				continue
			}
			switch {
			case strings.HasPrefix(line, "EHLO "):
				_, _ = conn.Write([]byte("250-mock\r\n250-STARTTLS\r\n250 AUTH PLAIN\r\n"))
			case strings.HasPrefix(line, "MAIL FROM:"), strings.HasPrefix(line, "RCPT TO:"):
				_, _ = conn.Write([]byte("250 ok\r\n"))
			case line == "DATA\r\n":
				inData = true
				_, _ = conn.Write([]byte("354 send\r\n"))
			case line == "QUIT\r\n":
				_, _ = conn.Write([]byte("221 bye\r\n"))
				done <- commands.String()
				return
			default:
				done <- "unexpected: " + line
				return
			}
		}
	}()
	ctx := context.Background()
	cfg := config.NewService(st, log.New("error", "console"))
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	for k, v := range map[string]string{KeyHost: host, KeyPort: port, KeyFrom: "sender@example.com", KeySecurity: config.SMTPSecurityPlain, KeyAuth: "false"} {
		if err := cfg.Set(ctx, k, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.SendTest(ctx, "recipient@example.com"); err != nil {
		t.Fatalf("本地中继发送失败: %v", err)
	}
	commands := <-done
	if strings.Contains(commands, "STARTTLS\r\n") || strings.Contains(commands, "AUTH ") || !strings.Contains(commands, "DATA\r\n") {
		t.Fatalf("本地中继命令不符: %q", commands)
	}
}

// newTestMail 创建临时库 + 邮件服务（不连真实 SMTP）
func newTestMail(t *testing.T) (*store.Store, *Service) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	fsys := fstest.MapFS{
		"0001_init.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY, value TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
	}
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	return st, NewService(cfg, log.New("error", "console"))
}

// TestConfigured SMTP 配置判定包含连接方式、发件人与认证模式。
func TestConfigured(t *testing.T) {
	st, svc := newTestMail(t)
	ctx := context.Background()
	if svc.Configured(ctx) {
		t.Error("未配置时 Configured 应为 false")
	}
	cfg := config.NewService(st, log.New("error", "console"))
	for k, v := range map[string]string{KeyHost: "h", KeyPort: "587", KeyFrom: "f@example.com", KeySecurity: "starttls", KeyAuth: "true", KeyUser: "u", KeyPassword: "p"} {
		if err := cfg.Set(ctx, k, v); err != nil {
			t.Fatalf("配置失败: %v", err)
		}
	}
	if !svc.Configured(ctx) {
		t.Error("完整认证配置后 Configured 应为 true")
	}
}

// TestSensitiveEncrypted smtp_password 加密落库（密文非明文）
func TestSensitiveEncrypted(t *testing.T) {
	st, svc := newTestMail(t)
	ctx := context.Background()
	cfg := config.NewService(st, log.New("error", "console"))
	if err := cfg.Set(ctx, KeyPassword, "plain-secret"); err != nil {
		t.Fatalf("配置失败: %v", err)
	}
	var raw string
	if err := st.DB().QueryRow(`SELECT value FROM system_config WHERE key = ?`, KeyPassword).Scan(&raw); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if raw == "plain-secret" {
		t.Error("smtp_password 应以密文落库")
	}
	// 读取自动解密
	got, err := cfg.Get(ctx, KeyPassword)
	if err != nil || got != "plain-secret" {
		t.Errorf("读取应解密: %q err=%v", got, err)
	}
	_ = svc
}

// TestScopeEnabled JSON 数组解析与包含判定
func TestScopeEnabled(t *testing.T) {
	st, svc := newTestMail(t)
	ctx := context.Background()
	if svc.ScopeEnabled(ctx, ScopeWelcome) {
		t.Error("未配置 scope 时应为 false")
	}
	cfg := config.NewService(st, log.New("error", "console"))
	if err := cfg.Set(ctx, KeyScopes, `["welcome","approval_notify"]`); err != nil {
		t.Fatalf("配置失败: %v", err)
	}
	if !svc.ScopeEnabled(ctx, ScopeWelcome) || !svc.ScopeEnabled(ctx, ScopeApprovalNotify) {
		t.Error("JSON 数组包含判定失败")
	}
	if svc.ScopeEnabled(ctx, ScopePasswordReset) {
		t.Error("未启用项应为 false")
	}
}

// TestSendUnconfigured SMTP 未配置时 Send 返回明确错误（不 panic）
func TestSendUnconfigured(t *testing.T) {
	_, svc := newTestMail(t)
	ctx := context.Background()
	if err := svc.Send(ctx, "a@example.com", "主题", "内容"); err == nil {
		t.Error("未配置 SMTP 发送应返回错误")
	}
	if err := svc.SendWelcome(ctx, "a@example.com", "站点", "https://x", "local"); err != nil {
		t.Errorf("scope 未启用时 SendWelcome 应返回 nil（不发送）: %v", err)
	}
	if err := svc.SendApprovalNotify(ctx, "a@example.com", "站点", true); err != nil {
		t.Errorf("scope 未启用时 SendApprovalNotify 应返回 nil: %v", err)
	}
	if err := svc.SendPasswordReset(ctx, "a@example.com", "https://x/reset/t"); err != nil {
		t.Errorf("scope 未启用时 SendPasswordReset 应返回 nil: %v", err)
	}
}

// TestSanitizeHeader 邮件头换行清洗（防头注入）
func TestSanitizeHeader(t *testing.T) {
	got := sanitizeHeader("站点\r\nBcc: evil@example.com")
	if got != "站点 Bcc: evil@example.com" {
		t.Errorf("换行应清洗: %q", got)
	}
}
