// Package urlguard 提供 OIDC 相关的统一 HTTPS 地址语法校验。
package urlguard

import (
	"errors"
	"fmt"
	"net/url"
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
