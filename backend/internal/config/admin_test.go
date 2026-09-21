package config

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
)

// mockOidcSaveCall 记录一次 SaveParams 入参，供测试断言“空值保持”与 provider 隔离。
type mockOidcSaveCall struct {
	providerType string
	baseURL      string
	realm        string
	clientID     string
	clientSecret string
}

// mockOidcParams 模拟某个提供商的参数存储（secret 为已解密明文）。
type mockOidcParams struct {
	baseURL     string
	realm       string
	clientID    string
	secret      string
	jsonDamaged bool
}

// mockOidcOps 模拟 oidc.Service（config 包避免循环依赖的接口注入）
type mockOidcOps struct {
	configured bool
	secret     string // 单提供商兼容字段：库内已存明文（模拟）
	params     map[string]mockOidcParams
	saveCalls  []mockOidcSaveCall

	// describeErr / describeState 用于注入 signing_key_fault 等 R31-04 状态。
	describeErr   error
	describeState OidcParamsState
}

func (m *mockOidcOps) SaveParams(ctx context.Context, providerType, baseURL, realm, clientID, clientSecret string) error {
	m.saveCalls = append(m.saveCalls, mockOidcSaveCall{
		providerType: providerType, baseURL: baseURL, realm: realm, clientID: clientID, clientSecret: clientSecret,
	})
	if m.params == nil {
		if clientSecret != "" { // 空值保留原值
			m.secret = clientSecret
		}
		return nil
	}
	p := m.params[providerType]
	p.baseURL, p.realm, p.clientID = baseURL, realm, clientID
	if clientSecret != "" {
		p.secret = clientSecret
	}
	m.params[providerType] = p
	return nil
}

func (m *mockOidcOps) LoadParams(ctx context.Context, providerType string) (string, string, string, string, error) {
	if m.params != nil {
		p, ok := m.params[providerType]
		if !ok {
			return "", "", "", "", errors.New("OIDC 参数未配置")
		}
		return p.baseURL, p.realm, p.clientID, p.secret, nil
	}
	return "https://idp.example.com", "realm", "client-x", m.secret, nil
}

func (m *mockOidcOps) DescribeParams(ctx context.Context, providerType string) (OidcParamsState, error) {
	if m.describeErr != nil {
		return m.describeState, m.describeErr
	}
	if m.params != nil {
		p, ok := m.params[providerType]
		if !ok {
			return OidcParamsState{}, nil
		}
		return OidcParamsState{
			Present: true, BaseURL: p.baseURL, Realm: p.realm, ClientID: p.clientID,
			SecretUsable:  p.secret != "" && p.secret != MaskedSecret,
			SecretDamaged: p.secret == MaskedSecret,
			JSONDamaged:   p.jsonDamaged,
		}, nil
	}
	return OidcParamsState{
		Present: true, BaseURL: "https://idp.example.com", Realm: "realm", ClientID: "client-x",
		SecretUsable:  m.secret != "" && m.secret != MaskedSecret,
		SecretDamaged: m.secret == MaskedSecret,
	}, nil
}

// SaveParamsTx / DescribeParamsTx 与只读测试替身同语义；测试不模拟事务可见性。
func (m *mockOidcOps) SaveParamsTx(ctx context.Context, tx *sql.Tx, providerType, baseURL, realm, clientID, clientSecret string) error {
	return m.SaveParams(ctx, providerType, baseURL, realm, clientID, clientSecret)
}

func (m *mockOidcOps) DescribeParamsTx(ctx context.Context, tx *sql.Tx, providerType string) (OidcParamsState, error) {
	return m.DescribeParams(ctx, providerType)
}

func (m *mockOidcOps) IsConfigured(ctx context.Context) bool { return m.configured }
func (m *mockOidcOps) ClearDiscCache()                       {}

// newTestAdmin 创建临时库 + 面板配置服务（默认 Dev；需要 Production 语义时用 newTestAdminWithMode）
func newTestAdmin(t *testing.T, oidcOps OidcOps) (*store.Store, *AdminService) {
	t.Helper()
	return newTestAdminWithMode(t, oidcOps, "dev")
}

// newTestAdminWithMode 创建指定启动 mode 的面板配置服务，用于验证不读取 system_config.app_mode。
func newTestAdminWithMode(t *testing.T, oidcOps OidcOps, mode string) (*store.Store, *AdminService) {
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
			key TEXT PRIMARY KEY, value TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS oidc_states (
			state TEXT PRIMARY KEY, code_verifier TEXT NOT NULL, nonce TEXT NOT NULL DEFAULT '',
			intent TEXT NOT NULL, bind_user_id INTEGER, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			provider_type TEXT NOT NULL DEFAULT '', config_hash TEXT NOT NULL DEFAULT '', redirect_uri TEXT NOT NULL DEFAULT '');
			CREATE TABLE IF NOT EXISTS oidc_login_tickets (
			ticket TEXT PRIMARY KEY, session_token TEXT NOT NULL, expires_at TIMESTAMP NOT NULL, flow_hash TEXT NOT NULL DEFAULT '');
			CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY, oidc_subject TEXT);`)},
	}
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := NewService(st, log.New("error", "console"))
	svc := NewAdminService(cfg, st, oidcOps, t.TempDir(), mode, log.New("error", "console"), new(slog.LevelVar))
	return st, svc
}

// TestSensitiveMasked 敏感字段：加密落库 + GET 脱敏 + PUT 空串不修改
func TestSensitiveMasked(t *testing.T) {
	st, svc := newTestAdmin(t, &mockOidcOps{})
	ctx := context.Background()

	auth := true
	if err := svc.SaveSMTP(ctx, SMTPSettings{Host: "smtp.example.com", Port: "587", User: "u",
		Password: "plain-pass", From: "f@example.com", Security: SMTPSecurityImplicitTLS, AuthRequired: &auth}); err != nil {
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
	// GET 只回显配置状态，输入值始终为空
	got := svc.GetSMTP(ctx)
	if got.Password != "" || !got.PasswordConfigured || got.Host != "smtp.example.com" {
		t.Errorf("回显应脱敏: %+v", got)
	}
	// PUT 空串不修改密码或加密方式
	if err := svc.SaveSMTP(ctx, SMTPSettings{Host: "smtp.example.com", Port: "587", User: "u", From: "f@example.com", Password: "", Security: got.Security, AuthRequired: &auth}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	if got := svc.GetSMTP(ctx); got.Password != "" || !got.PasswordConfigured || got.Security != "implicit_tls" {
		t.Errorf("空串不应修改密码: %+v", got)
	}
	if password, err := svc.cfg.Get(ctx, "smtp_password"); err != nil || password != "plain-pass" {
		t.Errorf("已保存密码被改写: %q, %v", password, err)
	}
	if err := svc.SaveSMTP(ctx, SMTPSettings{Host: "smtp.example.com", Password: "***", Security: "implicit_tls"}); !errors.Is(err, ErrBadRequest) {
		t.Errorf("占位符应被拒绝: %v", err)
	}
}

// TestSMTPNewContract 验证旧配置不发送、无认证中继清除凭据及非法保存不部分写入。
func TestSMTPNewContract(t *testing.T) {
	st, svc := newTestAdmin(t, &mockOidcOps{})
	ctx := context.Background()
	for k, v := range map[string]string{
		"smtp_host": "smtp.example.com", "smtp_port": "587", "smtp_from": "sender@example.com",
		"smtp_user": "sender@example.com", "smtp_password": "old-secret", "smtp_security": "legacy",
	} {
		if err := svc.cfg.Set(ctx, k, v); err != nil {
			t.Fatal(err)
		}
	}
	if got := svc.GetSMTP(ctx); got.Configured || got.Security != "" {
		t.Fatalf("旧连接方式应等待重新选择: %+v", got)
	}
	noAuth := false
	plain := SMTPSettings{Host: "127.0.0.1", Port: "2525", From: "sender@example.com", Security: SMTPSecurityPlain, AuthRequired: &noAuth}
	if err := svc.SaveSMTP(ctx, plain); err != nil {
		t.Fatal(err)
	}
	if got := svc.GetSMTP(ctx); !got.Configured || got.PasswordConfigured || got.User != "" || got.Security != SMTPSecurityPlain {
		t.Fatalf("本地无认证中继应可用且凭据已清除: %+v", got)
	}
	var n int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM system_config WHERE key IN ('smtp_password', 'smtp_tls')`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("旧密钥未清除: n=%d err=%v", n, err)
	}
	plain.Host = "smtp.example.com"
	if err := svc.SaveSMTP(ctx, plain); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("远端明文中继应拒绝: %v", err)
	}
	if got := svc.GetSMTP(ctx); got.Host != "127.0.0.1" || !got.Configured {
		t.Fatalf("失败保存不应改变有效配置: %+v", got)
	}
}

// TestSMTPAuthValidation 验证认证密码、地址、端口与连接方式必须同时有效。
func TestSMTPAuthValidation(t *testing.T) {
	_, svc := newTestAdmin(t, &mockOidcOps{})
	ctx := context.Background()
	auth := true
	base := SMTPSettings{Host: "smtp.example.com", Port: "587", From: "sender@example.com", User: "sender", Security: SMTPSecurityStartTLS, AuthRequired: &auth}
	for _, tc := range []struct {
		name string
		edit func(*SMTPSettings)
	}{
		{"missing password", func(*SMTPSettings) {}},
		{"bad port", func(s *SMTPSettings) { s.Port = "65536"; s.Password = "secret" }},
		{"bad from", func(s *SMTPSettings) { s.From = "Name <sender@example.com>"; s.Password = "secret" }},
		{"old mode", func(s *SMTPSettings) { s.Security = "legacy"; s.Password = "secret" }},
		{"plain auth", func(s *SMTPSettings) { s.Host = "127.0.0.1"; s.Security = SMTPSecurityPlain; s.Password = "secret" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := base
			tc.edit(&in)
			if err := svc.SaveSMTP(ctx, in); !errors.Is(err, ErrBadRequest) {
				t.Fatalf("应拒绝无效配置: %v", err)
			}
		})
	}
	base.Password = "secret"
	if err := svc.SaveSMTP(ctx, base); err != nil {
		t.Fatalf("有效 STARTTLS 配置应保存: %v", err)
	}
	if got := svc.GetSMTP(ctx); !got.Configured || got.Security != SMTPSecurityStartTLS {
		t.Fatalf("配置状态错误: %+v", got)
	}
	if err := svc.cfg.Set(ctx, "smtp_password", "***"); err != nil {
		t.Fatal(err)
	}
	if got := svc.GetSMTP(ctx); got.Configured {
		t.Fatal("历史占位符密码不应被判为可发送")
	}
	base.Password = ""
	if err := svc.SaveSMTP(ctx, base); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("不能沿用已损坏的占位符密码: %v", err)
	}
	base.Password = "new-secret"
	if err := svc.SaveSMTP(ctx, base); err != nil || !svc.GetSMTP(ctx).Configured {
		t.Fatalf("重新填写后应恢复发送配置: %v", err)
	}
}

// TestAuthDeadlock 死锁防护：本地登录关 + OIDC 不可用 → 三入口均 ErrAuthDeadlock
func TestAuthDeadlock(t *testing.T) {
	mock := &mockOidcOps{configured: true, secret: "cipher"} // 先允许保存本地登录关
	_, svc := newTestAdmin(t, mock)
	ctx := context.Background()
	if err := svc.cfg.Set(ctx, oidcKeyConfigured, "true"); err != nil {
		t.Fatalf("设置 OIDC 启用标记失败: %v", err)
	}
	if err := svc.cfg.Set(ctx, oidcKeyProviderType, "generic"); err != nil {
		t.Fatalf("设置当前提供商失败: %v", err)
	}
	if err := svc.cfg.Set(ctx, KeyFrontendURL, "https://app.example.com"); err != nil {
		t.Fatalf("设置前端地址失败: %v", err)
	}
	if err := svc.SaveLocalAuth(ctx, LocalAuthSettings{AllowLocalLogin: false}); err != nil {
		t.Fatalf("OIDC 可用时保存本地登录关应成功: %v", err)
	}
	mock.configured = false // 模拟 OIDC 不可用
	if err := svc.cfg.Set(ctx, oidcKeyConfigured, "false"); err != nil {
		t.Fatalf("设置 OIDC 停用标记失败: %v", err)
	}
	if err := svc.SaveLocalAuth(ctx, LocalAuthSettings{AllowLocalLogin: false}); !errors.Is(err, ErrAuthDeadlock) {
		t.Errorf("SaveLocalAuth 应拒绝: %v", err)
	}
	if err := svc.SaveOidc(ctx, OidcSettings{ProviderType: "generic", BaseURL: "", ClientID: "", ClientSecret: "fresh-secret"}); !errors.Is(err, ErrAuthDeadlock) {
		t.Errorf("SaveOidc 应拒绝: %v", err)
	}
	if err := svc.ClearOidc(ctx); !errors.Is(err, ErrAuthDeadlock) {
		t.Errorf("ClearOidc 应拒绝: %v", err)
	}
}

// TestSaveLocalAuthRejectsDamagedOidcSecret 损坏的历史占位符不能被 oidc_configured 标记掩盖；
// 本地登录关闭必须被拒绝，直至管理员重新填写可用 Secret。
func TestSaveLocalAuthRejectsDamagedOidcSecret(t *testing.T) {
	mock := &mockOidcOps{configured: true, secret: MaskedSecret}
	_, svc := newTestAdmin(t, mock)
	ctx := context.Background()
	if err := svc.cfg.Set(ctx, oidcKeyConfigured, "true"); err != nil {
		t.Fatalf("设置 OIDC 启用标记失败: %v", err)
	}
	if err := svc.cfg.Set(ctx, oidcKeyProviderType, "generic"); err != nil {
		t.Fatalf("设置当前提供商失败: %v", err)
	}
	if err := svc.cfg.Set(ctx, KeyFrontendURL, "https://app.example.com"); err != nil {
		t.Fatalf("设置前端地址失败: %v", err)
	}
	if err := svc.SaveLocalAuth(ctx, LocalAuthSettings{AllowLocalLogin: false}); !errors.Is(err, ErrAuthDeadlock) {
		t.Fatalf("损坏 Secret 时关闭本地登录应返回 ErrAuthDeadlock: %v", err)
	}
	mock.secret = "fresh-secret" // 模拟管理员重填后恢复
	if err := svc.SaveLocalAuth(ctx, LocalAuthSettings{AllowLocalLogin: false}); err != nil {
		t.Fatalf("重填 Secret 后应允许关闭本地登录: %v", err)
	}
}

// TestSaveLocalAuthRejectsEmptyOidcSecret 空 Secret 视为未配置，不能作为关闭本地登录的依据。
func TestSaveLocalAuthRejectsEmptyOidcSecret(t *testing.T) {
	mock := &mockOidcOps{configured: true, secret: ""}
	_, svc := newTestAdmin(t, mock)
	ctx := context.Background()
	if err := svc.cfg.Set(ctx, oidcKeyConfigured, "true"); err != nil {
		t.Fatalf("设置 OIDC 启用标记失败: %v", err)
	}
	if err := svc.cfg.Set(ctx, oidcKeyProviderType, "generic"); err != nil {
		t.Fatalf("设置当前提供商失败: %v", err)
	}
	if err := svc.cfg.Set(ctx, KeyFrontendURL, "https://app.example.com"); err != nil {
		t.Fatalf("设置前端地址失败: %v", err)
	}
	if err := svc.SaveLocalAuth(ctx, LocalAuthSettings{AllowLocalLogin: false}); !errors.Is(err, ErrAuthDeadlock) {
		t.Fatalf("空 Secret 时关闭本地登录应返回 ErrAuthDeadlock: %v", err)
	}
	mock.secret = "fresh-secret" // 模拟管理员填写后恢复
	if err := svc.SaveLocalAuth(ctx, LocalAuthSettings{AllowLocalLogin: false}); err != nil {
		t.Fatalf("填写 Secret 后应允许关闭本地登录: %v", err)
	}
}

// TestSaveLocalAuthMockOnlyAvailableInDev mock OIDC 仅在 Dev 模式有登录能力；
// 运行模式取启动注入值，不信任可被导入覆盖的 system_config.app_mode。
func TestSaveLocalAuthMockOnlyAvailableInDev(t *testing.T) {
	ctx := context.Background()

	// Production 启动：即使 DB app_mode 被写成 dev，也不能关闭本地登录。
	_, prodSvc := newTestAdminWithMode(t, &mockOidcOps{configured: true}, "prod")
	if err := prodSvc.cfg.Set(ctx, oidcKeyConfigured, "true"); err != nil {
		t.Fatalf("设置 OIDC 启用标记失败: %v", err)
	}
	if err := prodSvc.cfg.Set(ctx, oidcKeyProviderType, "mock"); err != nil {
		t.Fatalf("设置当前提供商失败: %v", err)
	}
	if err := prodSvc.cfg.Set(ctx, KeyAppMode, "dev"); err != nil {
		t.Fatalf("写入可被导入覆盖的 app_mode 失败: %v", err)
	}
	if err := prodSvc.SaveLocalAuth(ctx, LocalAuthSettings{AllowLocalLogin: false}); !errors.Is(err, ErrAuthDeadlock) {
		t.Fatalf("Production 启动 + mock 关闭本地登录应返回 ErrAuthDeadlock（不得信任 DB app_mode）: %v", err)
	}

	// Dev 启动：即使 DB app_mode 被写成 prod，Dev mock 仍可用。
	_, devSvc := newTestAdminWithMode(t, &mockOidcOps{configured: true}, "dev")
	if err := devSvc.cfg.Set(ctx, oidcKeyConfigured, "true"); err != nil {
		t.Fatalf("设置 OIDC 启用标记失败: %v", err)
	}
	if err := devSvc.cfg.Set(ctx, oidcKeyProviderType, "mock"); err != nil {
		t.Fatalf("设置当前提供商失败: %v", err)
	}
	if err := devSvc.cfg.Set(ctx, KeyAppMode, "prod"); err != nil {
		t.Fatalf("写入可被导入覆盖的 app_mode 失败: %v", err)
	}
	if err := devSvc.SaveLocalAuth(ctx, LocalAuthSettings{AllowLocalLogin: false}); err != nil {
		t.Fatalf("Dev 启动 + mock 应允许关闭本地登录: %v", err)
	}
}

// TestSaveLocalAuthMockMissingParamsRejected Dev + mock 但参数结构缺失时仍视为不可用，防止后续登录入口失效。
func TestSaveLocalAuthMockMissingParamsRejected(t *testing.T) {
	mock := &mockOidcOps{configured: true, params: map[string]mockOidcParams{}}
	_, svc := newTestAdmin(t, mock)
	ctx := context.Background()
	if err := svc.cfg.Set(ctx, oidcKeyProviderType, "mock"); err != nil {
		t.Fatalf("设置当前提供商失败: %v", err)
	}
	if err := svc.cfg.Set(ctx, KeyAppMode, "dev"); err != nil {
		t.Fatalf("设置运行模式失败: %v", err)
	}
	if err := svc.SaveLocalAuth(ctx, LocalAuthSettings{AllowLocalLogin: false}); !errors.Is(err, ErrAuthDeadlock) {
		t.Fatalf("Dev + mock 缺少参数时应返回 ErrAuthDeadlock: %v", err)
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

// TestCaptchaPlainStorage 验证码双密钥明文存储：回显明文 + 库内即明文（切换提供商/停用后可复用，R11 修复）
func TestCaptchaPlainStorage(t *testing.T) {
	st, svc := newTestAdmin(t, &mockOidcOps{})
	ctx := context.Background()
	if err := svc.SaveCaptcha(ctx, CaptchaSettings{Provider: "turnstile", SiteKey: "site-123", SecretKey: "secret-456", Pages: []string{"login"}}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	got := svc.GetCaptcha(ctx)
	if got.SiteKey != "site-123" || got.SecretKey != "secret-456" {
		t.Errorf("回显应为明文: site=%q secret=%q", got.SiteKey, got.SecretKey)
	}
	// 库内原始值即明文（不再是 AES 密文，停用后再开启无需重新配置）
	var raw string
	if err := st.DB().QueryRowContext(ctx, `SELECT value FROM system_config WHERE key = ?`, captchaKeySecretKey).Scan(&raw); err != nil {
		t.Fatalf("读取库内密钥失败: %v", err)
	}
	if raw != "secret-456" {
		t.Errorf("库内应为明文密钥: %q", raw)
	}
	// 停用后重新开启：密钥保留，留空即可复用（不再被 *** 覆盖损坏）
	if err := svc.SaveCaptcha(ctx, CaptchaSettings{Provider: "off", Pages: nil}); err != nil {
		t.Fatalf("停用保存失败: %v", err)
	}
	if err := svc.SaveCaptcha(ctx, CaptchaSettings{Provider: "turnstile", Pages: []string{"login"}}); err != nil {
		t.Fatalf("停用后重新开启失败: %v", err)
	}
	if got := svc.GetCaptcha(ctx); got.SiteKey != "site-123" || got.SecretKey != "secret-456" {
		t.Errorf("重新开启后密钥应保留明文: %+v", got)
	}
}

// TestLogLevelSwitch 日志级别：持久化 + 当前 Runtime LevelVar 立即生效，不影响其他 Runtime
func TestLogLevelSwitch(t *testing.T) {
	runtime := log.NewRuntime("error", "console")
	other := log.NewRuntime("error", "console")
	_, svc := newTestAdmin(t, &mockOidcOps{})
	svc.level = runtime.Level
	ctx := context.Background()
	if err := svc.SetLogLevel(ctx, "debug"); err != nil {
		t.Fatalf("设置日志级别失败: %v", err)
	}
	if got := svc.GetLogLevel(ctx); got != "debug" {
		t.Errorf("持久化异常: %s", got)
	}
	if !runtime.Logger.Enabled(ctx, slog.LevelDebug) {
		t.Error("当前 Runtime 的 debug 级别应已生效")
	}
	if other.Logger.Enabled(ctx, slog.LevelDebug) {
		t.Error("切换日志级别不应污染其他 Runtime")
	}
	if err := svc.SetLogLevel(ctx, "bogus"); !errors.Is(err, ErrBadRequest) {
		t.Errorf("非法级别应拒绝: %v", err)
	}
}

// TestIconValidation ICON：>2MB 拒绝、svg/gif 拒绝、png 通过且版本号递增
func TestIconValidation(t *testing.T) {
	_, svc := newTestAdmin(t, &mockOidcOps{})
	ctx := context.Background()
	if got := svc.GetSiteInfo(ctx).Name; got != DefaultSiteName {
		t.Fatalf("未设置站点名时应返回默认名称: %q", got)
	}
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
	if info.Name != "站点" {
		t.Errorf("已保存站点名未生效: %q", info.Name)
	}
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

// TestFrontendURLAndCallbackURLImmediate 前端地址/独立回调地址保存即时生效；独立值优先，显式清除后回退推导。
func TestFrontendURLAndCallbackURLImmediate(t *testing.T) {
	_, svc := newTestAdmin(t, &mockOidcOps{})
	ctx := context.Background()
	if err := svc.SaveOidc(ctx, OidcSettings{ProviderType: "generic", BaseURL: "https://idp.example.com",
		Realm: "realm", ClientID: "client-x", ClientSecret: "sec123",
		FrontendURL: "https://app.example.com", CallbackURL: "https://callback.example.com/api/auth/oidc/callback"}); err != nil {
		t.Fatalf("保存 OIDC 失败: %v", err)
	}
	got, err := svc.GetOidc(ctx)
	if err != nil {
		t.Fatalf("回显失败: %v", err)
	}
	if got.FrontendURL != "https://app.example.com" || got.CallbackURL != "https://callback.example.com/api/auth/oidc/callback" {
		t.Errorf("地址应即时保存并回显: %+v", got)
	}
	// 仅改前端地址时独立回调继续优先。
	if err := svc.SaveOidc(ctx, OidcSettings{ProviderType: "generic", BaseURL: "https://idp.example.com",
		Realm: "realm", ClientID: "client-x", FrontendURL: "https://new.example.com"}); err != nil {
		t.Fatalf("更新前端地址失败: %v", err)
	}
	got, _ = svc.GetOidc(ctx)
	if got.FrontendURL != "https://new.example.com" || got.CallbackURL != "https://callback.example.com/api/auth/oidc/callback" {
		t.Errorf("独立回调应优先且前端地址即时生效: %+v", got)
	}
	// 显式清除独立回调后回退推导。
	if err := svc.SaveOidc(ctx, OidcSettings{ProviderType: "generic", BaseURL: "https://idp.example.com",
		Realm: "realm", ClientID: "client-x", ClearCallbackURL: true}); err != nil {
		t.Fatalf("清除独立回调失败: %v", err)
	}
	got, _ = svc.GetOidc(ctx)
	if got.CallbackURL != "" {
		t.Fatalf("清除后 callback_url 应为空: %+v", got)
	}
	resolved, err := ResolveOidcCallbackURL(got.CallbackURL, got.FrontendURL)
	if err != nil || resolved != "https://new.example.com"+OidcCallbackPath {
		t.Fatalf("清除后应回退 frontend_url 推导: resolved=%q err=%v", resolved, err)
	}
	if got.ClientSecret != "" || !got.ClientSecretConfigured {
		t.Errorf("Secret 应空回显且状态为已配置: %+v", got)
	}
}

// TestOidcSecretKeepAndReplace Secret 空值保持原值、显式新值替换、回显始终为空。
func TestOidcSecretKeepAndReplace(t *testing.T) {
	mock := &mockOidcOps{}
	_, svc := newTestAdmin(t, mock)
	ctx := context.Background()
	in := OidcSettings{ProviderType: "generic", BaseURL: "https://idp.example.com", Realm: "realm", ClientID: "client-x"}
	in.ClientSecret = "sec123"
	if err := svc.SaveOidc(ctx, in); err != nil {
		t.Fatalf("保存 OIDC 失败: %v", err)
	}
	if got, _ := svc.GetOidc(ctx); got.ClientSecret != "" || !got.ClientSecretConfigured {
		t.Fatalf("GET 应空 Secret + 已配置状态: %+v", got)
	}
	// 空值保存：请求透传空串，底层保留原密文
	in.ClientSecret = ""
	if err := svc.SaveOidc(ctx, in); err != nil {
		t.Fatalf("空值保存失败: %v", err)
	}
	if mock.secret != "sec123" {
		t.Errorf("空值保存不应覆盖已存 Secret: %q", mock.secret)
	}
	if len(mock.saveCalls) == 0 || mock.saveCalls[len(mock.saveCalls)-1].clientSecret != "" {
		t.Errorf("PUT 空值应原样传给 SaveParams: %+v", mock.saveCalls)
	}
	// 显式新值替换
	in.ClientSecret = "new-secret"
	if err := svc.SaveOidc(ctx, in); err != nil {
		t.Fatalf("显式替换失败: %v", err)
	}
	if mock.secret != "new-secret" {
		t.Errorf("显式新值应替换旧 Secret: %q", mock.secret)
	}
}

// TestOidcPlaceholderRejected 服务端拒绝把回显占位符当新 Secret 保存，且不调用底层写入。
func TestOidcPlaceholderRejected(t *testing.T) {
	mock := &mockOidcOps{secret: "sec123"}
	_, svc := newTestAdmin(t, mock)
	ctx := context.Background()
	in := OidcSettings{ProviderType: "generic", BaseURL: "https://idp.example.com", ClientID: "c", ClientSecret: MaskedSecret}
	if err := svc.SaveOidc(ctx, in); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("占位符应返回 ErrBadRequest: %v", err)
	}
	if mock.secret != "sec123" || len(mock.saveCalls) != 0 {
		t.Errorf("占位符不应改写库内 Secret 或触发写入: secret=%q calls=%+v", mock.secret, mock.saveCalls)
	}
}

// TestOidcDamagedPlaceholderHandling 历史占位符：GET 显示未配置；空值保存拒绝；重新输入后恢复。
func TestOidcDamagedPlaceholderHandling(t *testing.T) {
	mock := &mockOidcOps{secret: MaskedSecret}
	_, svc := newTestAdmin(t, mock)
	ctx := context.Background()
	got, err := svc.GetOidc(ctx)
	if err != nil {
		t.Fatalf("GET 失败: %v", err)
	}
	if got.ClientSecret != "" || got.ClientSecretConfigured {
		t.Fatalf("历史占位符应按未配置回显: %+v", got)
	}
	in := OidcSettings{ProviderType: "generic", BaseURL: "https://idp.example.com", ClientID: "c"}
	if err := svc.SaveOidc(ctx, in); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("空值沿用历史占位符应被拒绝: %v", err)
	}
	in.ClientSecret = "fresh-secret"
	if err := svc.SaveOidc(ctx, in); err != nil {
		t.Fatalf("重新填写后应保存成功: %v", err)
	}
	if got, _ := svc.GetOidc(ctx); !got.ClientSecretConfigured || got.ClientSecret != "" {
		t.Errorf("重新填写后应恢复已配置状态: %+v", got)
	}
}

// TestOidcProviderSwitchKeepsOtherProviderSecret 切换提供商只写目标 provider；
// 目标无旧 Secret 时禁止空 Secret，显式新 Secret 后切换，切回源提供商时空 Secret 保留源原值。
func TestOidcProviderSwitchKeepsOtherProviderSecret(t *testing.T) {
	mock := &mockOidcOps{params: map[string]mockOidcParams{
		"keycloak": {baseURL: "https://kc.example.com", realm: "master", clientID: "kc", secret: "kc-secret"},
	}}
	_, svc := newTestAdmin(t, mock)
	ctx := context.Background()
	// 切到 generic：目标无已存 Secret，空 Secret 必须拒绝，且不写 generic、不碰 keycloak
	err := svc.SaveOidc(ctx, OidcSettings{ProviderType: "generic", BaseURL: "https://generic.example.com", ClientID: "g"})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("目标无旧 Secret 时空 Secret 应返回 ErrBadRequest: %v", err)
	}
	if mock.params["keycloak"].secret != "kc-secret" {
		t.Errorf("拒绝时不应覆盖其他提供商 Secret: %+v", mock.params)
	}
	if _, ok := mock.params["generic"]; ok {
		t.Errorf("拒绝时空 Secret 不应写入 generic 参数: %+v", mock.params["generic"])
	}
	// 显式新 Secret 才能切换到 generic
	if err := svc.SaveOidc(ctx, OidcSettings{ProviderType: "generic", BaseURL: "https://generic.example.com", ClientID: "g", ClientSecret: "g-secret"}); err != nil {
		t.Fatalf("显式新 Secret 切换到 generic 失败: %v", err)
	}
	if mock.params["keycloak"].secret != "kc-secret" {
		t.Errorf("切换不应覆盖其他提供商 Secret: %+v", mock.params)
	}
	if mock.params["generic"].secret != "g-secret" {
		t.Errorf("generic 应写入显式新 Secret: %+v", mock.params["generic"])
	}
	// 切回 keycloak：空 Secret 且字段一致，保留该提供商原值
	if err := svc.SaveOidc(ctx, OidcSettings{ProviderType: "keycloak", BaseURL: "https://kc.example.com", Realm: "master", ClientID: "kc"}); err != nil {
		t.Fatalf("切回 keycloak 失败: %v", err)
	}
	if mock.params["keycloak"].secret != "kc-secret" {
		t.Errorf("切回后空 Secret 应保留原值: %+v", mock.params["keycloak"])
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
	if err := svc.cfg.Set(ctx, oidcKeyConfigured, "true"); err != nil {
		t.Fatalf("设置 OIDC 启用标记失败: %v", err)
	}
	if err := svc.cfg.Set(ctx, oidcKeyProviderType, "generic"); err != nil {
		t.Fatalf("设置当前提供商失败: %v", err)
	}
	if err := svc.cfg.Set(ctx, KeyFrontendURL, "https://app.example.com"); err != nil {
		t.Fatalf("设置前端地址失败: %v", err)
	}
	// 先保存本地登录关（OIDC 可用时允许）
	if err := svc.SaveLocalAuth(ctx, LocalAuthSettings{AllowLocalLogin: false}); err != nil {
		t.Fatalf("保存本地登录关失败: %v", err)
	}
	mock.configured = false // 模拟 OIDC 配置状态不可知（以入参+库内密文判定）
	// 新 secret 为空但库内已有密文 + base_url/realm/client_id 与已存一致 → 视为可用，允许保存
	if err := svc.SaveOidc(ctx, OidcSettings{ProviderType: "generic", BaseURL: "https://idp.example.com", Realm: "realm", ClientID: "client-x"}); err != nil {
		t.Errorf("库内已有密文时应可保存: %v", err)
	}
	_ = io.Discard // 占位避免未使用（io 供后续扩展）
}

// TestR3102SaveOidcRejectsHTTPBaseURL 写入口必须在任何配置写入前拒绝 HTTP Base URL。
func TestR3102SaveOidcRejectsHTTPBaseURL(t *testing.T) {
	mock := &mockOidcOps{configured: true, secret: "cipher"}
	st, svc := newTestAdmin(t, mock)
	ctx := context.Background()
	err := svc.SaveOidc(ctx, OidcSettings{
		ProviderType: "generic", BaseURL: "http://idp.example.com", ClientID: "c", ClientSecret: "s",
	})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("HTTP Base URL 应被 ErrBadRequest 拒绝，实际: %v", err)
	}
	if len(mock.saveCalls) != 0 {
		t.Fatalf("拒绝后不应调用 SaveParams，实际 %d 次", len(mock.saveCalls))
	}
	var count int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM system_config WHERE key LIKE 'oidc_%'`).Scan(&count); err != nil {
		t.Fatalf("查询 OIDC 配置失败: %v", err)
	}
	if count != 0 {
		t.Fatalf("拒绝后不应写入 OIDC 配置，实际 %d 个键", count)
	}
}

// TestR3102SaveLocalAuthRejectsHTTPOidcBaseURL 已有 HTTP OIDC 配置时，不得关闭本地登录形成死锁。
func TestR3102SaveLocalAuthRejectsHTTPOidcBaseURL(t *testing.T) {
	mock := &mockOidcOps{
		configured: true,
		params: map[string]mockOidcParams{
			"generic": {baseURL: "http://idp.example.com", clientID: "c", secret: "cipher"},
		},
	}
	_, svc := newTestAdmin(t, mock)
	ctx := context.Background()
	if err := svc.cfg.Set(ctx, oidcKeyProviderType, "generic"); err != nil {
		t.Fatalf("设置当前提供商失败: %v", err)
	}
	if err := svc.SaveLocalAuth(ctx, LocalAuthSettings{AllowLocalLogin: false}); !errors.Is(err, ErrAuthDeadlock) {
		t.Fatalf("HTTP OIDC Base URL 不得作为关闭本地登录的依据，实际: %v", err)
	}
	if !svc.cfg.GetBool(ctx, KeyAllowLocalLogin, true) {
		t.Fatal("拒绝时不应关闭本地登录")
	}
}

// TestGetOidcForProviderReadsOnlyTarget R31-03：目标读取只返回指定提供商自己的字段与 Secret 状态。
func TestGetOidcForProviderReadsOnlyTarget(t *testing.T) {
	mock := &mockOidcOps{params: map[string]mockOidcParams{
		"keycloak": {baseURL: "https://kc.example.com", realm: "master", clientID: "kc", secret: "kc-secret"},
		"generic":  {baseURL: "https://g.example.com", clientID: "g", secret: "g-secret"},
	}}
	_, svc := newTestAdmin(t, mock)
	ctx := context.Background()

	got, err := svc.GetOidcForProvider(ctx, "keycloak")
	if err != nil {
		t.Fatalf("读取 keycloak 失败: %v", err)
	}
	if got.ProviderType != "keycloak" || got.BaseURL != "https://kc.example.com" || got.Realm != "master" || got.ClientID != "kc" {
		t.Fatalf("keycloak 字段回显异常: %+v", got)
	}
	if got.ClientSecret != "" || !got.ClientSecretConfigured || got.ParamsDamaged {
		t.Fatalf("keycloak Secret 状态异常: %+v", got)
	}

	got, err = svc.GetOidcForProvider(ctx, "generic")
	if err != nil {
		t.Fatalf("读取 generic 失败: %v", err)
	}
	if got.BaseURL != "https://g.example.com" || got.ClientID != "g" || !got.ClientSecretConfigured {
		t.Fatalf("generic 不应混入 keycloak 字段: %+v", got)
	}

	if _, err := svc.GetOidcForProvider(ctx, "unknown"); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("非法目标提供商应返回 ErrBadRequest: %v", err)
	}
}

// TestGetOidcForProviderDamagedState R31-03：目标损坏时保留可解析字段并返回最小损坏提示。
func TestGetOidcForProviderDamagedState(t *testing.T) {
	mock := &mockOidcOps{params: map[string]mockOidcParams{
		"generic": {baseURL: "https://g.example.com", realm: "r", clientID: "g", secret: MaskedSecret},
		"auth0":   {baseURL: "https://a.example.com", clientID: "a", jsonDamaged: true},
	}}
	_, svc := newTestAdmin(t, mock)
	ctx := context.Background()

	got, err := svc.GetOidcForProvider(ctx, "generic")
	if err != nil {
		t.Fatalf("读取损坏 Secret 目标失败: %v", err)
	}
	if got.BaseURL != "https://g.example.com" || got.ClientID != "g" || got.ClientSecretConfigured || !got.ParamsDamaged || got.ParamsWarning == "" {
		t.Fatalf("Secret 损坏目标应保留字段并要求重填: %+v", got)
	}

	got, err = svc.GetOidcForProvider(ctx, "auth0")
	if err != nil {
		t.Fatalf("读取损坏 JSON 目标失败: %v", err)
	}
	if got.BaseURL != "" || got.ClientID != "" || !got.ParamsDamaged || got.ParamsWarning == "" {
		t.Fatalf("JSON 损坏目标不应猜测字段: %+v", got)
	}

	missing, err := svc.GetOidcForProvider(ctx, "keycloak")
	if err != nil {
		t.Fatalf("读取未配置目标失败: %v", err)
	}
	if missing.BaseURL != "" || missing.ClientSecretConfigured || missing.ParamsDamaged {
		t.Fatalf("未配置目标应清空字段且不标损坏: %+v", missing)
	}
}

// TestSaveOidcEmptySecretRequiresTargetFieldsUnchanged R31-03：空 Secret 复用必须 Base URL/Realm/Client ID 三项均一致。
func TestSaveOidcEmptySecretRequiresTargetFieldsUnchanged(t *testing.T) {
	cases := []struct {
		name string
		in   OidcSettings
	}{
		{"BaseURL 变化", OidcSettings{ProviderType: "keycloak", BaseURL: "https://evil.example.com", Realm: "master", ClientID: "kc"}},
		{"Realm 变化", OidcSettings{ProviderType: "keycloak", BaseURL: "https://kc.example.com", Realm: "other", ClientID: "kc"}},
		{"ClientID 变化", OidcSettings{ProviderType: "keycloak", BaseURL: "https://kc.example.com", Realm: "master", ClientID: "other"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockOidcOps{params: map[string]mockOidcParams{
				"keycloak": {baseURL: "https://kc.example.com", realm: "master", clientID: "kc", secret: "kc-secret"},
			}}
			_, svc := newTestAdmin(t, mock)
			ctx := context.Background()
			if err := svc.SaveOidc(ctx, tc.in); !errors.Is(err, ErrBadRequest) {
				t.Fatalf("字段变化时空 Secret 应返回 ErrBadRequest: %v", err)
			}
			if len(mock.saveCalls) != 0 {
				t.Fatalf("拒绝时不应调用 SaveParams: %+v", mock.saveCalls)
			}
			p := mock.params["keycloak"]
			if p.baseURL != "https://kc.example.com" || p.realm != "master" || p.clientID != "kc" || p.secret != "kc-secret" {
				t.Fatalf("拒绝时不应改动目标参数: %+v", p)
			}
		})
	}
}
