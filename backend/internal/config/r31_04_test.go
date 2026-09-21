package config

import (
	"context"
	"errors"
	"testing"
)

// TestR3104AdminKeyFaultStateAndWriteBlock S1/K1：管理端 GET 返回 signing_key_fault 而非吞成未配置；
// SaveOidc 在任何参数/提供商/站点写入前拒绝，SaveLocalAuth 不允许在密钥故障时关闭本地登录。
func TestR3104AdminKeyFaultStateAndWriteBlock(t *testing.T) {
	mock := &mockOidcOps{
		configured:  true,
		describeErr: ErrSigningKeyUnavailable,
		describeState: OidcParamsState{
			State:    OidcParamsSigningKeyFault,
			Present:  true,
			BaseURL:  "https://idp.example.com",
			Realm:    "r",
			ClientID: "c",
		},
	}
	st, svc := newTestAdmin(t, mock)
	ctx := context.Background()
	if err := svc.cfg.Set(ctx, oidcKeyProviderType, "generic"); err != nil {
		t.Fatalf("设置 provider 失败: %v", err)
	}

	got, err := svc.GetOidc(ctx)
	if err != nil {
		t.Fatalf("GetOidc 密钥故障不应返回错误: %v", err)
	}
	if got.ParamsState != OidcParamsSigningKeyFault || got.ParamsWarning == "" || got.ClientSecretConfigured {
		t.Fatalf("当前提供商 GET 密钥故障状态异常: %+v", got)
	}
	if got.BaseURL != "https://idp.example.com" || got.ClientID != "c" || got.ClientSecret != "" {
		t.Fatalf("密钥故障应保留可解析非 Secret 字段且 Secret 空: %+v", got)
	}

	target, err := svc.GetOidcForProvider(ctx, "generic")
	if err != nil {
		t.Fatalf("GetOidcForProvider 密钥故障不应返回错误: %v", err)
	}
	if target.ParamsState != OidcParamsSigningKeyFault || target.BaseURL != "https://idp.example.com" || target.ClientID != "c" {
		t.Fatalf("目标 GET 密钥故障状态异常: %+v", target)
	}

	// 显式新 Secret 与空 Secret 都必须在写入前返回类型化密钥故障。
	for _, clientSecret := range []string{"new-secret", ""} {
		in := OidcSettings{ProviderType: "generic", BaseURL: "https://idp.example.com", Realm: "r", ClientID: "c", ClientSecret: clientSecret}
		if err := svc.SaveOidc(ctx, in); !errors.Is(err, ErrSigningKeyUnavailable) {
			t.Fatalf("SaveOidc secret=%q 应返回 ErrSigningKeyUnavailable: %v", clientSecret, err)
		}
	}
	if len(mock.saveCalls) != 0 {
		t.Fatalf("签名密钥故障不得触发参数写调用: %+v", mock.saveCalls)
	}
	var configured string
	if err := st.DB().QueryRow(`SELECT COALESCE((SELECT value FROM system_config WHERE key = ?), '')`, oidcKeyConfigured).Scan(&configured); err != nil {
		t.Fatalf("查询 oidc_configured 失败: %v", err)
	}
	if configured == "true" {
		t.Fatalf("签名密钥故障保存不得写入 oidc_configured=true")
	}

	if err := svc.SaveLocalAuth(ctx, LocalAuthSettings{AllowLocalLogin: false}); !errors.Is(err, ErrAuthDeadlock) {
		t.Fatalf("签名密钥故障时关闭本地登录应返回 ErrAuthDeadlock: %v", err)
	}
}
