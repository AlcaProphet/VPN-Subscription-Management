package config

import (
	"errors"
	"fmt"
	"testing"
)

// TestValidateImportedAuthUsableRejectsDamagedOidcSecret 导入校验：本地登录关闭时，
// OIDC Client Secret 必须能用导入包签名密钥解密且不是脱敏占位符，防止导入后认证死锁。
func TestValidateImportedAuthUsableRejectsDamagedOidcSecret(t *testing.T) {
	signingKey := "import-test-signing-key"
	validCipher, err := Encrypt([]byte("oidc-secret"), []byte(signingKey))
	if err != nil {
		t.Fatalf("加密测试 Secret 失败: %v", err)
	}
	placeholderCipher, err := Encrypt([]byte(MaskedSecret), []byte(signingKey))
	if err != nil {
		t.Fatalf("加密占位符失败: %v", err)
	}
	build := func(secretCipher string) map[string]string {
		return map[string]string{
			KeyConfigured:        "true",
			KeyAllowLocalLogin:   "false",
			KeySigningKey:        signingKey,
			"frontend_url":       "https://app.example.com",
			"oidc_configured":    "true",
			"oidc_provider_type": "generic",
			"oidc_params_generic": fmt.Sprintf(
				`{"base_url":"https://idp.example.com","client_id":"client","client_secret":%q}`, secretCipher),
		}
	}

	if err := ValidateImportedAuthUsable(build(validCipher)); err != nil {
		t.Fatalf("有效 OIDC Secret 不应被拒绝: %v", err)
	}
	if err := ValidateImportedAuthUsable(build(placeholderCipher)); !errors.Is(err, ErrAuthDeadlock) {
		t.Fatalf("占位符应被 ErrAuthDeadlock 拒绝: %v", err)
	}
	if err := ValidateImportedAuthUsable(build("not-a-valid-cipher")); !errors.Is(err, ErrAuthDeadlock) {
		t.Fatalf("无法解密的密文应被 ErrAuthDeadlock 拒绝: %v", err)
	}
	// 本地登录开启时不依赖 OIDC，不应因坏 Secret 阻断导入
	cfg := build(placeholderCipher)
	cfg[KeyAllowLocalLogin] = "true"
	if err := ValidateImportedAuthUsable(cfg); err != nil {
		t.Fatalf("本地登录开启时不应拒绝: %v", err)
	}
}

// TestR3102ValidateImportedAuthUsableHTTPBaseURL 导入路径：本地登录关闭时必须拒绝 HTTP Base URL；
// 本地登录开启时按既定策略放行，由运行时 OIDC 守卫阻止真实 HTTP 请求。
func TestR3102ValidateImportedAuthUsableHTTPBaseURL(t *testing.T) {
	signingKey := "import-http-test-signing-key"
	validCipher, err := Encrypt([]byte("oidc-secret"), []byte(signingKey))
	if err != nil {
		t.Fatalf("加密测试 Secret 失败: %v", err)
	}
	build := func(allowLocal, baseURL string) map[string]string {
		return map[string]string{
			KeyConfigured:        "true",
			KeyAllowLocalLogin:   allowLocal,
			KeySigningKey:        signingKey,
			"frontend_url":       "https://app.example.com",
			"oidc_configured":    "true",
			"oidc_provider_type": "generic",
			"oidc_params_generic": fmt.Sprintf(
				`{"base_url":%q,"client_id":"client","client_secret":%q}`, baseURL, validCipher),
		}
	}

	if err := ValidateImportedAuthUsable(build("false", "http://idp.example.com")); !errors.Is(err, ErrAuthDeadlock) {
		t.Fatalf("本地登录关闭且 HTTP Base URL 应被 ErrAuthDeadlock 拒绝，实际: %v", err)
	}
	if err := ValidateImportedAuthUsable(build("false", "https://idp.example.com")); err != nil {
		t.Fatalf("本地登录关闭且 HTTPS Base URL 不应被拒绝: %v", err)
	}
	if err := ValidateImportedAuthUsable(build("true", "http://idp.example.com")); err != nil {
		t.Fatalf("本地登录开启时按既定策略不应拒绝: %v", err)
	}
}

// TestR3105ValidateImportedAuthUsableOidcConfigured R31-05：本地登录关闭时必须启用 OIDC，
// 且能够解析出有效回调地址；本地登录开启时不因 OIDC 标记/地址阻断导入。
func TestR3105ValidateImportedAuthUsableOidcConfigured(t *testing.T) {
	signingKey := "r3105-import-signing-key"
	validCipher, err := Encrypt([]byte("oidc-secret"), []byte(signingKey))
	if err != nil {
		t.Fatalf("加密测试 Secret 失败: %v", err)
	}
	build := func(configured, allowLocal, frontend, callback string) map[string]string {
		return map[string]string{
			KeyConfigured:        "true",
			KeyAllowLocalLogin:   allowLocal,
			KeySigningKey:        signingKey,
			"frontend_url":       frontend,
			"callback_url":       callback,
			"oidc_configured":    configured,
			"oidc_provider_type": "generic",
			"oidc_params_generic": fmt.Sprintf(
				`{"base_url":"https://idp.example.com","client_id":"client","client_secret":%q}`, validCipher),
		}
	}

	if err := ValidateImportedAuthUsable(build("", "false", "https://app.example.com", "")); !errors.Is(err, ErrAuthDeadlock) {
		t.Fatalf("oidc_configured 缺失应被 ErrAuthDeadlock 拒绝: %v", err)
	}
	if err := ValidateImportedAuthUsable(build("false", "false", "https://app.example.com", "")); !errors.Is(err, ErrAuthDeadlock) {
		t.Fatalf("oidc_configured=false 应被 ErrAuthDeadlock 拒绝: %v", err)
	}
	if err := ValidateImportedAuthUsable(build("true", "false", "", "https://callback.example.com/api/auth/oidc/callback")); err != nil {
		t.Fatalf("独立回调有效时不应被拒绝: %v", err)
	}
	if err := ValidateImportedAuthUsable(build("true", "false", "https://app.example.com", "")); err != nil {
		t.Fatalf("frontend_url 推导回调有效时不应被拒绝: %v", err)
	}
	if err := ValidateImportedAuthUsable(build("true", "false", "", "")); !errors.Is(err, ErrAuthDeadlock) {
		t.Fatalf("无可解析回调地址应被 ErrAuthDeadlock 拒绝: %v", err)
	}
	if err := ValidateImportedAuthUsable(build("true", "false", "", "https://callback.example.com/wrong")); !errors.Is(err, ErrAuthDeadlock) {
		t.Fatalf("回调路径错误应被 ErrAuthDeadlock 拒绝: %v", err)
	}
	if err := ValidateImportedAuthUsable(build("false", "true", "", "")); err != nil {
		t.Fatalf("本地登录开启时不因 OIDC 未启用/无地址阻断导入: %v", err)
	}
}

// TestR3105ValidateImportedAuthUsableBooleanAndProviderSemantics 审计补齐：
// configured 的布尔语义必须与运行时一致；本地登录关闭时导入的 provider_type 必须属于已知白名单。
func TestR3105ValidateImportedAuthUsableBooleanAndProviderSemantics(t *testing.T) {
	const signingKey = "r3105-boolean-provider-signing-key"
	validCipher, err := Encrypt([]byte("oidc-secret"), []byte(signingKey))
	if err != nil {
		t.Fatalf("加密测试 Secret 失败: %v", err)
	}
	build := func(configured, oidcConfigured, providerType string) map[string]string {
		return map[string]string{
			KeyConfigured:        configured,
			KeyAllowLocalLogin:   "false",
			KeySigningKey:        signingKey,
			"frontend_url":       "https://app.example.com",
			"oidc_configured":    oidcConfigured,
			"oidc_provider_type": providerType,
			"oidc_params_" + providerType: fmt.Sprintf(
				`{"base_url":"https://idp.example.com","client_id":"client","client_secret":%q}`, validCipher),
		}
	}

	cases := []struct {
		name    string
		cfg     map[string]string
		wantErr error
	}{
		{name: "configured=TRUE 仍进入认证可用性校验", cfg: build("TRUE", "false", "generic"), wantErr: ErrAuthDeadlock},
		{name: "configured=1 仍进入认证可用性校验", cfg: build("1", "false", "generic"), wantErr: ErrAuthDeadlock},
		{name: "configured 非法值拒绝", cfg: build("yes", "true", "generic"), wantErr: ErrBadRequest},
		{name: "configured 带空白拒绝", cfg: build(" true ", "true", "generic"), wantErr: ErrBadRequest},
		{name: "oidc_configured 带空白拒绝", cfg: build("true", " true ", "generic"), wantErr: ErrAuthDeadlock},
		{name: "未知 provider_type 拒绝", cfg: build("true", "true", "bogus"), wantErr: ErrAuthDeadlock},
		{name: "provider_type 带空白拒绝", cfg: build("true", "true", " generic "), wantErr: ErrAuthDeadlock},
		{name: "合法 provider_type 通过", cfg: build("true", "true", "generic"), wantErr: nil},
		{name: "未配置导入跳过认证校验", cfg: build("false", "false", "bogus"), wantErr: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateImportedAuthUsable(tc.cfg)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("不应返回错误: %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("错误类型不符: got=%v want=%v", err, tc.wantErr)
			}
		})
	}
}
