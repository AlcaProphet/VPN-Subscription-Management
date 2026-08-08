package config

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
)

// mockOidcOps 模拟 oidc.Service（config 包避免循环依赖的接口注入）
type mockOidcOps struct {
	configured bool
	secret     string // 库内已存密文（模拟）
}

func (m *mockOidcOps) SaveParams(ctx context.Context, providerType, baseURL, realm, clientID, clientSecret string) error {
	m.secret = clientSecret
	return nil
}

func (m *mockOidcOps) LoadParams(ctx context.Context, providerType string) (string, string, string, string, error) {
	return "https://idp.example.com", "realm", "client-x", m.secret, nil
}

func (m *mockOidcOps) IsConfigured(ctx context.Context) bool { return m.configured }
func (m *mockOidcOps) ClearDiscCache()                       {}

// newTestAdmin 创建临时库 + 面板配置服务
func newTestAdmin(t *testing.T, oidcOps OidcOps) (*store.Store, *AdminService) {
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
	cfg := NewService(st, log.New("error", "console"))
	svc := NewAdminService(cfg, st, oidcOps, t.TempDir(), log.New("error", "console"))
	return st, svc
}

// TestSensitiveMasked 敏感字段：加密落库 + GET 脱敏 + PUT 空串不修改
func TestSensitiveMasked(t *testing.T) {
	st, svc := newTestAdmin(t, &mockOidcOps{})
	ctx := context.Background()
	// smtp_password 敏感登记由 mail 包 init 完成；测试内显式登记（同 config_test.go 模式）
	RegisterSensitive("smtp_password")
	t.Cleanup(func() { delete(sensitiveKeys, "smtp_password") })

	if err := svc.SaveSMTP(ctx, SMTPSettings{Host: "smtp.example.com", Port: "587", User: "u",
		Password: "plain-pass", From: "f@example.com", TLS: true}); err != nil {
		t.Fatalf("保存 SMTP 失败: %v", err)
	}
	// 库内为密文
	var raw string
	if err := st.DB().QueryRow(`SELECT value FROM system_config WHERE key = 'smtp_password'`).Scan(&raw); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if raw == "plain-pass" {
		t.Error("smtp_password 应以密文落库")
	}
	// GET 脱敏回显
	got := svc.GetSMTP(ctx)
	if got.Password != "***" || got.Host != "smtp.example.com" {
		t.Errorf("回显应脱敏: %+v", got)
	}
	// PUT 空串不修改（仍为 ***）
	if err := svc.SaveSMTP(ctx, SMTPSettings{Host: "smtp.example.com", Password: ""}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	if got := svc.GetSMTP(ctx); got.Password != "***" {
		t.Errorf("空串不应修改密码: %+v", got)
	}
}

// TestAuthDeadlock 死锁防护：本地登录关 + OIDC 不可用 → 三入口均 ErrAuthDeadlock
func TestAuthDeadlock(t *testing.T) {
	mock := &mockOidcOps{configured: true} // 先允许保存本地登录关
	_, svc := newTestAdmin(t, mock)
	ctx := context.Background()
	if err := svc.SaveLocalAuth(ctx, LocalAuthSettings{AllowLocalLogin: false}); err != nil {
		t.Fatalf("OIDC 可用时保存本地登录关应成功: %v", err)
	}
	mock.configured = false // 模拟 OIDC 不可用
	if err := svc.SaveLocalAuth(ctx, LocalAuthSettings{AllowLocalLogin: false}); !errors.Is(err, ErrAuthDeadlock) {
		t.Errorf("SaveLocalAuth 应拒绝: %v", err)
	}
	if err := svc.SaveOidc(ctx, OidcSettings{ProviderType: "generic", BaseURL: "", ClientID: ""}); !errors.Is(err, ErrAuthDeadlock) {
		t.Errorf("SaveOidc 应拒绝: %v", err)
	}
	if err := svc.ClearOidc(ctx); !errors.Is(err, ErrAuthDeadlock) {
		t.Errorf("ClearOidc 应拒绝: %v", err)
	}
}

// TestWhitelistWarning 白名单空警告：approvalOn=true 且白名单空 → warning 标记
func TestWhitelistWarning(t *testing.T) {
	_, svc := newTestAdmin(t, &mockOidcOps{})
	ctx := context.Background()
	warning, err := svc.SaveOidcRules(ctx, true, WhitelistConfig{})
	if err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	if warning == "" {
		t.Error("审批开且白名单空应返回 warning")
	}
	// 白名单非空 → 无 warning
	warning, err = svc.SaveOidcRules(ctx, true, WhitelistConfig{RoleClaimPath: "roles", RoleValues: []string{"admin"}})
	if err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	if warning != "" {
		t.Errorf("白名单非空不应有 warning: %q", warning)
	}
	// 回显一致
	on, wl, err := svc.GetOidcRules(ctx)
	if err != nil || !on || len(wl.RoleValues) != 1 || wl.RoleValues[0] != "admin" {
		t.Errorf("回显异常: on=%v wl=%+v err=%v", on, wl, err)
	}
}

// TestCaptchaKeyMissing 验证码拦截：勾选启用页面但密钥缺失 → ErrCaptchaKeyMissing
func TestCaptchaKeyMissing(t *testing.T) {
	_, svc := newTestAdmin(t, &mockOidcOps{})
	ctx := context.Background()
	err := svc.SaveCaptcha(ctx, CaptchaSettings{Provider: "recaptcha", Pages: []string{"login"}})
	if !errors.Is(err, ErrCaptchaKeyMissing) {
		t.Errorf("密钥缺失应拦截: %v", err)
	}
	// 配置密钥后可保存
	if err := svc.SaveCaptcha(ctx, CaptchaSettings{Provider: "recaptcha", SiteKey: "sk", SecretKey: "sec", Pages: []string{"login"}}); err != nil {
		t.Fatalf("配置密钥后保存失败: %v", err)
	}
	// 关闭提供商可保存（off 不校验密钥）
	if err := svc.SaveCaptcha(ctx, CaptchaSettings{Provider: "off", Pages: nil}); err != nil {
		t.Fatalf("off 保存失败: %v", err)
	}
}

// TestLogLevelSwitch 日志级别：持久化 + LevelVar 立即生效
func TestLogLevelSwitch(t *testing.T) {
	logger := log.New("error", "console")
	log.SetDefault(logger) // 测试内接管默认 logger（生产由 main 装配）
	_, svc := newTestAdmin(t, &mockOidcOps{})
	ctx := context.Background()
	if err := svc.SetLogLevel(ctx, "debug"); err != nil {
		t.Fatalf("设置日志级别失败: %v", err)
	}
	if got := svc.GetLogLevel(ctx); got != "debug" {
		t.Errorf("持久化异常: %s", got)
	}
	if !log.Default().Enabled(ctx, slog.LevelDebug) {
		t.Error("debug 级别应已生效（LevelVar 切换）")
	}
	if err := svc.SetLogLevel(ctx, "bogus"); !errors.Is(err, ErrBadRequest) {
		t.Errorf("非法级别应拒绝: %v", err)
	}
}

// TestIconValidation ICON：>2MB 拒绝、svg/gif 拒绝、png 通过且版本号递增
func TestIconValidation(t *testing.T) {
	_, svc := newTestAdmin(t, &mockOidcOps{})
	ctx := context.Background()
	if err := svc.SaveSiteInfo(ctx, "站点", bytes.NewReader(make([]byte, MaxIconSize+1)), "icon.png"); !errors.Is(err, ErrBadRequest) {
		t.Errorf("超 2MB 应拒绝: %v", err)
	}
	if err := svc.SaveSiteInfo(ctx, "站点", strings.NewReader("x"), "icon.svg"); !errors.Is(err, ErrBadRequest) {
		t.Errorf("svg 应拒绝: %v", err)
	}
	if err := svc.SaveSiteInfo(ctx, "站点", strings.NewReader("x"), "icon.gif"); !errors.Is(err, ErrBadRequest) {
		t.Errorf("gif 应拒绝: %v", err)
	}
	if err := svc.SaveSiteInfo(ctx, "站点", strings.NewReader("png-data"), "icon.PNG"); err != nil {
		t.Fatalf("png 应通过: %v", err)
	}
	info := svc.GetSiteInfo(ctx)
	if !strings.Contains(info.IconURL, "/public/site/icon.png?v=1") {
		t.Errorf("ICON URL 应带版本参数: %s", info.IconURL)
	}
	// 再次上传版本号递增
	if err := svc.SaveSiteInfo(ctx, "站点", strings.NewReader("png2"), "icon.png"); err != nil {
		t.Fatalf("再次上传失败: %v", err)
	}
	if !strings.Contains(svc.GetSiteInfo(ctx).IconURL, "?v=2") {
		t.Errorf("版本号应递增: %s", svc.GetSiteInfo(ctx).IconURL)
	}
	// 名称超长拒绝
	if err := svc.SaveSiteInfo(ctx, strings.Repeat("长", 51), nil, ""); !errors.Is(err, ErrBadRequest) {
		t.Errorf("名称超 50 字符应拒绝: %v", err)
	}
	// 删除恢复默认
	if err := svc.DeleteSiteIcon(ctx); err != nil {
		t.Fatalf("删除 ICON 失败: %v", err)
	}
	if svc.GetSiteInfo(ctx).IconURL != "" {
		t.Errorf("删除后 icon_url 应为空: %s", svc.GetSiteInfo(ctx).IconURL)
	}
}

// TestFrontendURLCached 前端地址/回调地址：手动保存后沿用（库驱动缓存语义，重启不推导覆盖）
func TestFrontendURLCached(t *testing.T) {
	_, svc := newTestAdmin(t, &mockOidcOps{})
	ctx := context.Background()
	if err := svc.SaveOidc(ctx, OidcSettings{ProviderType: "generic", BaseURL: "https://idp.example.com",
		ClientID: "c", ClientSecret: "sec123", FrontendURL: "https://app.example.com", CallbackURL: "https://app.example.com/cb"}); err != nil {
		t.Fatalf("保存 OIDC 失败: %v", err)
	}
	got, err := svc.GetOidc(ctx)
	if err != nil {
		t.Fatalf("回显失败: %v", err)
	}
	if got.FrontendURL != "https://app.example.com" || got.CallbackURL != "https://app.example.com/cb" {
		t.Errorf("手动值应优先沿用: %+v", got)
	}
	if got.ClientSecret != "***" {
		t.Errorf("Secret 应脱敏: %+v", got)
	}
}

// TestSaveRateLimit 限流：非正数拒绝
func TestSaveRateLimit(t *testing.T) {
	_, svc := newTestAdmin(t, &mockOidcOps{})
	ctx := context.Background()
	if err := svc.SaveRateLimit(ctx, RateLimitSettings{Login: 10, Register: 5, Forgot: 5, Download: 0}); !errors.Is(err, ErrBadRequest) {
		t.Errorf("非正数应拒绝: %v", err)
	}
	if err := svc.SaveRateLimit(ctx, RateLimitSettings{Login: 10, Register: 5, Forgot: 5, Download: 20}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	got := svc.GetRateLimit(ctx)
	if got.Login != 10 || got.Download != 20 {
		t.Errorf("限流值异常: %+v", got)
	}
}

// TestAnnouncementLen 公告：>2000 字符拒绝
func TestAnnouncementLen(t *testing.T) {
	_, svc := newTestAdmin(t, &mockOidcOps{})
	ctx := context.Background()
	if err := svc.SaveAnnouncement(ctx, strings.Repeat("告", 2001)); !errors.Is(err, ErrBadRequest) {
		t.Errorf("超 2000 字符应拒绝: %v", err)
	}
	if err := svc.SaveAnnouncement(ctx, "欢迎使用"); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	if svc.GetAnnouncement(ctx) != "欢迎使用" {
		t.Errorf("公告异常: %q", svc.GetAnnouncement(ctx))
	}
}

// TestOidcUsableWithStoredSecret 库内已有密文视为可用（防认证死锁判定）
func TestOidcUsableWithStoredSecret(t *testing.T) {
	mock := &mockOidcOps{configured: true, secret: "cipher"}
	_, svc := newTestAdmin(t, mock)
	ctx := context.Background()
	// 先保存本地登录关（OIDC 可用时允许）
	if err := svc.SaveLocalAuth(ctx, LocalAuthSettings{AllowLocalLogin: false}); err != nil {
		t.Fatalf("保存本地登录关失败: %v", err)
	}
	mock.configured = false // 模拟 OIDC 配置状态不可知（以入参+库内密文判定）
	// 新 secret 为空但库内已有密文 + base_url/client_id 非空 → 视为可用，允许保存
	if err := svc.SaveOidc(ctx, OidcSettings{ProviderType: "generic", BaseURL: "https://idp.example.com", ClientID: "c"}); err != nil {
		t.Errorf("库内已有密文时应可保存: %v", err)
	}
	_ = io.Discard // 占位避免未使用（io 供后续扩展）
}
