// Package urlguard 提供 OIDC 相关的统一 HTTPS 与绝对回调地址语法校验。
package urlguard

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ValidateHTTPS 只做 URL 语法与协议校验：必须为 https scheme 且 hostname 非空。
// DNS 解析与公网 IP 校验由调用方在拨号阶段完成，避免“校验解析”与“拨号解析”之间的 TOCTOU 窗口。
func ValidateHTTPS(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("URL 解析失败: %w", err)
	}
	if u.Scheme != "https" {
		return errors.New("仅支持 HTTPS 地址")
	}
	if u.Hostname() == "" {
		return errors.New("URL 缺少主机名")
	}
	return nil
}

// ParseAbsoluteHTTPURL 解析并校验绝对 http/https 地址：host 非空，禁止 userinfo、query 与 fragment。
// 返回值供调用方做路径等进一步校验与规范化。
func ParseAbsoluteHTTPURL(rawURL string) (*url.URL, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil, errors.New("地址不能为空")
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("URL 解析失败: %w", err)
	}
	if !strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https") {
		return nil, errors.New("仅支持 http/https 地址")
	}
	if u.Hostname() == "" {
		return nil, errors.New("URL 缺少主机名")
	}
	if u.User != nil {
		return nil, errors.New("URL 不能包含用户名或密码")
	}
	if u.RawQuery != "" || u.ForceQuery {
		return nil, errors.New("URL 不能包含查询参数")
	}
	if u.Fragment != "" {
		return nil, errors.New("URL 不能包含 fragment")
	}
	if u.Opaque != "" {
		return nil, errors.New("URL 格式无效")
	}
	return u, nil
}

// ValidateOIDCCallbackURL 在绝对 http/https 校验之外，要求路径精确等于 callbackPath。
// 允许任意 host，host 与发起登录站点的 Cookie 边界由调用方另行提示或约束。
func ValidateOIDCCallbackURL(rawURL, callbackPath string) (*url.URL, error) {
	u, err := ParseAbsoluteHTTPURL(rawURL)
	if err != nil {
		return nil, err
	}
	if u.EscapedPath() != callbackPath {
		return nil, fmt.Errorf("回调地址路径必须为 %s", callbackPath)
	}
	return u, nil
}
