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
