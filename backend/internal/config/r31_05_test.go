package config

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// TestR3105SaveOidcAddressValidationAtomic 地址校验失败必须发生在参数写入前，且不改变已存配置。
func TestR3105SaveOidcAddressValidationAtomic(t *testing.T) {
	mock := &mockOidcOps{}
	st, svc := newTestAdmin(t, mock)
	ctx := context.Background()
	before := map[string]string{}
	rows, err := st.DB().Query(`SELECT key, value FROM system_config`)
	if err != nil {
		t.Fatalf("读取初始配置失败: %v", err)
	}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			t.Fatalf("扫描初始配置失败: %v", err)
		}
		before[k] = v
	}
	_ = rows.Close()

	err = svc.SaveOidc(ctx, OidcSettings{
		ProviderType: "generic", BaseURL: "https://idp.example.com", ClientID: "client-x",
		ClientSecret: "secret", CallbackURL: "https://callback.example.com/not-the-callback",
	})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("错误回调路径应返回 ErrBadRequest: %v", err)
	}
	if len(mock.saveCalls) != 0 {
		t.Fatalf("地址校验失败不得先写 provider 参数: %+v", mock.saveCalls)
	}
	after := map[string]string{}
	rows, err = st.DB().Query(`SELECT key, value FROM system_config`)
	if err != nil {
		t.Fatalf("读取拒绝后配置失败: %v", err)
	}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			t.Fatalf("扫描拒绝后配置失败: %v", err)
		}
		after[k] = v
	}
	_ = rows.Close()
	if len(after) != len(before) {
		t.Fatalf("拒绝后配置键数应不变: before=%d after=%d", len(before), len(after))
	}
	for k, v := range before {
		if after[k] != v {
			t.Fatalf("拒绝后配置 %s 不应变化: before=%q after=%q", k, v, after[k])
		}
	}
}

// TestR3105ExplicitClearCallbackURL 显式清除语义：清除与保存同事务；独立值优先，清除后回退推导。
func TestR3105ExplicitClearCallbackURL(t *testing.T) {
	_, svc := newTestAdmin(t, &mockOidcOps{})
	ctx := context.Background()
	independent := "https://callback.example.com" + OidcCallbackPath
	if err := svc.SaveOidc(ctx, OidcSettings{
		ProviderType: "generic", BaseURL: "https://idp.example.com", Realm: "realm", ClientID: "client-x",
		ClientSecret: "secret", FrontendURL: "https://app.example.com", CallbackURL: independent,
	}); err != nil {
		t.Fatalf("保存独立回调失败: %v", err)
	}
	got, _ := svc.GetOidc(ctx)
	if got.CallbackURL != independent {
		t.Fatalf("独立回调应保存并优先: %+v", got)
	}

	// clear 与 callback 同提交语义冲突，拒绝且不改配置。
	if err := svc.SaveOidc(ctx, OidcSettings{
		ProviderType: "generic", BaseURL: "https://idp.example.com", Realm: "realm", ClientID: "client-x",
		ClearCallbackURL: true, CallbackURL: independent,
	}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("clear 与非空 callback 同提交应返回 ErrBadRequest: %v", err)
	}
	got, _ = svc.GetOidc(ctx)
	if got.CallbackURL != independent {
		t.Fatalf("冲突拒绝后独立回调不应变化: %+v", got)
	}

	// 清除需要有效 frontend_url 用于推导。
	if err := svc.SaveOidc(ctx, OidcSettings{
		ProviderType: "generic", BaseURL: "https://idp.example.com", Realm: "realm", ClientID: "client-x",
		ClearCallbackURL: true,
	}); err != nil {
		t.Fatalf("有 frontend_url 时应允许清除: %v", err)
	}
	got, _ = svc.GetOidc(ctx)
	if got.CallbackURL != "" {
		t.Fatalf("清除后 callback_url 应为空: %+v", got)
	}
	resolved, err := ResolveOidcCallbackURL(got.CallbackURL, got.FrontendURL)
	if err != nil || resolved != "https://app.example.com"+OidcCallbackPath {
		t.Fatalf("清除后应回退 frontend_url 推导: resolved=%q err=%v", resolved, err)
	}
}

// TestR3105ClearWithoutFrontendRejected 无有效 frontend_url 时不得清除独立回调，避免派生相对地址。
func TestR3105ClearWithoutFrontendRejected(t *testing.T) {
	_, svc := newTestAdmin(t, &mockOidcOps{})
	ctx := context.Background()
	independent := "https://callback.example.com" + OidcCallbackPath
	if err := svc.SaveOidc(ctx, OidcSettings{
		ProviderType: "generic", BaseURL: "https://idp.example.com", Realm: "realm", ClientID: "client-x",
		ClientSecret: "secret", CallbackURL: independent,
	}); err != nil {
		t.Fatalf("保存独立回调失败: %v", err)
	}
	err := svc.SaveOidc(ctx, OidcSettings{
		ProviderType: "generic", BaseURL: "https://idp.example.com", Realm: "realm", ClientID: "client-x",
		ClearCallbackURL: true,
	})
	if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "前端地址") {
		t.Fatalf("无 frontend_url 清除应返回 ErrBadRequest 并提示前端地址: %v", err)
	}
	got, _ := svc.GetOidc(ctx)
	if got.CallbackURL != independent {
		t.Fatalf("拒绝清除后独立回调不应变化: %+v", got)
	}
}

// TestR3105LocalLoginOffRequiresCallbackAddress 本地登录关闭时，OIDC 必须同时能解析出有效回调地址。
func TestR3105LocalLoginOffRequiresCallbackAddress(t *testing.T) {
	mock := &mockOidcOps{configured: true, secret: "cipher"}
	_, svc := newTestAdmin(t, mock)
	ctx := context.Background()
	if err := svc.cfg.Set(ctx, oidcKeyProviderType, "generic"); err != nil {
		t.Fatalf("设置 provider 失败: %v", err)
	}
	if err := svc.SaveLocalAuth(ctx, LocalAuthSettings{AllowLocalLogin: false}); !errors.Is(err, ErrAuthDeadlock) {
		t.Fatalf("无回调地址时关闭本地登录应返回 ErrAuthDeadlock: %v", err)
	}
	if err := svc.cfg.Set(ctx, KeyFrontendURL, "https://app.example.com"); err != nil {
		t.Fatalf("设置 frontend_url 失败: %v", err)
	}
	if err := svc.SaveLocalAuth(ctx, LocalAuthSettings{AllowLocalLogin: false}); err != nil {
		t.Fatalf("有可推导回调地址后应允许关闭本地登录: %v", err)
	}
}
