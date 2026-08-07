package captcha

import (
	"context"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
)

// newCaptchaEnv 创建临时库 + 验证码服务
func newCaptchaEnv(t *testing.T) (*Service, *config.Service) {
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
	return NewService(cfg, log.New("error", "console")), cfg
}

// TestEnforcedOff provider=off 时不强制
func TestEnforcedOff(t *testing.T) {
	svc, cfg := newCaptchaEnv(t)
	ctx := context.Background()
	_ = cfg.Set(ctx, KeyProvider, "off")
	if svc.Enforced(ctx, "login") {
		t.Error("provider=off 不应强制")
	}
	_ = cfg.Set(ctx, KeyProvider, "")
	if svc.Enforced(ctx, "login") {
		t.Error("provider 空不应强制")
	}
}

// TestEnforcedPageNotIncluded 页面不在 captcha_pages 时不强制
func TestEnforcedPageNotIncluded(t *testing.T) {
	svc, cfg := newCaptchaEnv(t)
	ctx := context.Background()
	_ = cfg.Set(ctx, KeyProvider, "recaptcha")
	_ = cfg.Set(ctx, KeyPages, `["register"]`)
	_ = cfg.Set(ctx, KeySecretKey, "secret-value")
	if svc.Enforced(ctx, "login") {
		t.Error("login 不在 captcha_pages 不应强制")
	}
	if !svc.Enforced(ctx, "register") {
		t.Error("register 在 captcha_pages 且密钥已配置应强制")
	}
}

// TestEnforcedSecretMissing 密钥缺失时跳过校验（兜底）
func TestEnforcedSecretMissing(t *testing.T) {
	svc, cfg := newCaptchaEnv(t)
	ctx := context.Background()
	_ = cfg.Set(ctx, KeyProvider, "turnstile")
	_ = cfg.Set(ctx, KeyPages, `["forgot"]`)
	// 密钥未配置 → Enforced 返回 false
	if svc.Enforced(ctx, "forgot") {
		t.Error("密钥缺失应跳过校验（兜底）")
	}
}

// TestVerifyEmptyToken 强制页面 + 空 token → 报错
func TestVerifyEmptyToken(t *testing.T) {
	svc, cfg := newCaptchaEnv(t)
	ctx := context.Background()
	_ = cfg.Set(ctx, KeyProvider, "recaptcha")
	_ = cfg.Set(ctx, KeyPages, `["login"]`)
	_ = cfg.Set(ctx, KeySecretKey, "secret-value")
	if err := svc.Verify(ctx, "login", ""); err == nil {
		t.Error("空 token 应报错")
	}
	// 非强制页面 → 直接放行
	if err := svc.Verify(ctx, "forgot", ""); err != nil {
		t.Errorf("非强制页面应放行: %v", err)
	}
}
