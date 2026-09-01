// Package proxytrust 提供 TRUST_PROXY 与 TRUST_PROXY_CIDRS 的统一信任策略。
// server 与 setup 共用同一实现，避免“gin 信任某来源、frontend_url 推导却不信任”的口径漂移。
package proxytrust

import (
	"fmt"
	"net"
	"strings"
)

// Mode TRUST_PROXY 档位
type Mode string

const (
	ModeAuto Mode = "auto"
	ModeOn   Mode = "on"
	ModeOff  Mode = "off"
	ModeCIDR Mode = "cidr"
)

// Policy 不可变信任策略
type Policy struct {
	mode  Mode
	cidrs []*net.IPNet
	raw   string
}

// Parse 解析 TRUST_PROXY 与 TRUST_PROXY_CIDRS。
// 非法模式或非法 CIDR 返回错误，由启动层直接退出。
func Parse(mode, cidrs string) (*Policy, error) {
	m := Mode(strings.ToLower(strings.TrimSpace(mode)))
	switch m {
	case ModeAuto, ModeOn, ModeOff:
	case ModeCIDR:
	default:
		return nil, fmt.Errorf("TRUST_PROXY 仅支持 auto|on|off|cidr")
	}
	p := &Policy{mode: m, raw: strings.TrimSpace(cidrs)}
	if m == ModeCIDR {
		parts := strings.Split(p.raw, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			_, ipNet, err := net.ParseCIDR(part)
			if err != nil {
				return nil, fmt.Errorf("TRUST_PROXY_CIDRS 包含非法 CIDR %q: %w", part, err)
			}
			p.cidrs = append(p.cidrs, ipNet)
		}
	}
	return p, nil
}

// Mode 返回当前档位
func (p *Policy) Mode() Mode { return p.mode }

// RawCIDRs 返回原始 CIDR 配置
func (p *Policy) RawCIDRs() string { return p.raw }

// Trusted 判断远端地址是否为可信代理。
func (p *Policy) Trusted(remoteAddr string) bool {
	switch p.mode {
	case ModeOn:
		return true
	case ModeOff:
		return false
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	if p.mode == ModeAuto && (ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
		return true
	}
	if p.mode == ModeCIDR {
		for _, cidr := range p.cidrs {
			if cidr.Contains(ip) {
				return true
			}
		}
	}
	return false
}

// TrustedProxies 返回 gin SetTrustedProxies 可用的信任网段列表。
func (p *Policy) TrustedProxies() []string {
	switch p.mode {
	case ModeOn:
		return []string{"0.0.0.0/0", "::/0"}
	case ModeOff:
		return []string{}
	case ModeCIDR:
		out := []string{"127.0.0.1/8", "::1/128"}
		for _, cidr := range p.cidrs {
			out = append(out, cidr.String())
		}
		return out
	default: // auto
		return []string{"127.0.0.1/8", "::1/128", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}
	}
}
