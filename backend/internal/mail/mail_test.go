package mail

import (
	"context"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
)

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

// TestConfigured SMTP 配置判定（三键口径）
func TestConfigured(t *testing.T) {
	st, svc := newTestMail(t)
	ctx := context.Background()
	if svc.Configured(ctx) {
		t.Error("未配置时 Configured 应为 false")
	}
	cfg := config.NewService(st, log.New("error", "console"))
	for k, v := range map[string]string{"smtp_host": "h", "smtp_user": "u", "smtp_password": "p"} {
		if err := cfg.Set(ctx, k, v); err != nil {
			t.Fatalf("配置失败: %v", err)
		}
	}
	if !svc.Configured(ctx) {
		t.Error("三键配置后 Configured 应为 true")
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
