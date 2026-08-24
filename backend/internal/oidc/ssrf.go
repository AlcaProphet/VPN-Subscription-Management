package oidc

import (
	"errors"
	"fmt"
	"net"
	"net/url"
)

// validateOIDCURL 校验 OIDC 发现文档/Token 端点仅允许公网 HTTPS 地址，
// 拒绝回环、私网、链路本地、组播、未指定等非公网 IP，防止 SSRF。
func validateOIDCURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("URL 解析失败: %w", err)
	}
	if u.Scheme != "https" {
		return errors.New("仅支持 HTTPS 地址")
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("URL 缺少主机名")
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("解析主机失败: %w", err)
	}
	if len(ips) == 0 {
		return errors.New("URL 主机无解析结果")
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("禁止访问非公网地址: %s", ip)
		}
	}
	return nil
}

// isBlockedIP 判断是否为禁止访问的非公网 IP。
func isBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || ip.IsMulticast()
}
