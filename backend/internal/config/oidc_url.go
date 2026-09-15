// config/oidc_url.go：OIDC 站点地址的校验、规范化与回调地址解析（R31-05）。
package config

import (
	"fmt"
	"strings"

	"vpn-sub/internal/urlguard"
)

// OidcCallbackPath 本站真实注册的 OIDC 回调路径；路由注册与地址校验必须复用同一常量。
const OidcCallbackPath = "/api/auth/oidc/callback"

// normalizeAndValidateFrontendURL 校验并规范化前端地址：绝对 http/https、host 非空，
// 禁止 userinfo/query/fragment；路径允许（兼容反代前缀），末尾连续斜杠会被移除。
func normalizeAndValidateFrontendURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("%w: 前端地址不能为空", ErrBadRequest)
	}
	u, err := urlguard.ParseAbsoluteHTTPURL(trimmed)
	if err != nil {
		return "", fmt.Errorf("%w: 前端地址无效: %v", ErrBadRequest, err)
	}
	u.Scheme = strings.ToLower(u.Scheme)
	return strings.TrimRight(u.String(), "/"), nil
}

// normalizeAndValidateCallbackURL 校验并规范化独立回调地址：绝对 http/https、
// 路径精确为 OidcCallbackPath；host 允许任意值，跨 host Cookie 约束由界面提示。
func normalizeAndValidateCallbackURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("%w: 回调地址不能为空", ErrBadRequest)
	}
	u, err := urlguard.ValidateOIDCCallbackURL(trimmed, OidcCallbackPath)
	if err != nil {
		return "", fmt.Errorf("%w: 回调地址无效: %v", ErrBadRequest, err)
	}
	u.Scheme = strings.ToLower(u.Scheme)
	return u.String(), nil
}

// ResolveOidcCallbackURL 解析真实 OIDC 最终使用的 redirect_uri：优先独立回调地址，
// 未设置时由 frontend_url 拼接 OidcCallbackPath。独立地址只做路径与 URL 安全校验，
// 允许任意 host；frontend_url 推导值允许带反代前缀，但本身必须是有效绝对地址。
func ResolveOidcCallbackURL(callbackURL, frontendURL string) (string, error) {
	if cb := strings.TrimSpace(callbackURL); cb != "" {
		return normalizeAndValidateCallbackURL(cb)
	}
	frontend := strings.TrimSpace(frontendURL)
	if frontend == "" {
		return "", fmt.Errorf("%w: 未配置前端地址，无法推导 OIDC 回调地址", ErrBadRequest)
	}
	normalized, err := normalizeAndValidateFrontendURL(frontend)
	if err != nil {
		return "", err
	}
	derived := normalized + OidcCallbackPath
	if _, err := urlguard.ParseAbsoluteHTTPURL(derived); err != nil {
		return "", fmt.Errorf("%w: 推导的 OIDC 回调地址无效: %v", ErrBadRequest, err)
	}
	return derived, nil
}
