package oidc

import (
	"net"

	"vpn-sub/internal/urlguard"
)

// validateOIDCURL 只做 URL 语法与协议校验。实际的 DNS 解析与公网 IP 校验统一在
// http.Transport.DialContext 拨号时完成，避免“校验解析”与“拨号解析”之间的 DNS rebinding 窗口。
func validateOIDCURL(rawURL string) error {
	return urlguard.ValidateHTTPS(rawURL)
}

// isBlockedIP 判断是否为禁止访问的非公网 IP。
func isBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || ip.IsMulticast()
}
