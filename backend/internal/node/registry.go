// Package node 提供统一节点表（manual/xray）的业务层：协议注册表、manual CRUD、
// xray 显示名与启停/公共标记切换。本 Build 仅实现 manual 全流程与 xray 数据层占位。
package node

import (
	"errors"
	"fmt"
)

// FieldSchema 描述协议表单字段（供前端动态渲染）。
type FieldSchema struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"` // text / password / number / bool / select
	Required bool     `json:"required"`
	Default  any      `json:"default,omitempty"`
	Label    string   `json:"label"`
	Help     string   `json:"help,omitempty"`
	Options  []string `json:"options,omitempty"`
}

// LinkMapping 描述协议在 SR / 通用节点订阅中的链接映射能力。
type LinkMapping struct {
	SR      bool     `json:"sr"`
	Generic bool     `json:"generic"`
	Params  []string `json:"params,omitempty"` // 主要参与链接生成的参数名（供前端提示）
}

// Protocol 协议注册表条目。
type Protocol struct {
	Protocol        string        `json:"protocol"`
	Label           string        `json:"label"`
	FormSchema      []FieldSchema `json:"form_schema"`
	SensitiveFields []string      `json:"sensitive_fields"`
	LinkMappings    LinkMapping   `json:"link_mappings"`
}

// ManualProtocols 返回 manual 节点可用的协议注册表（19 项封闭清单，ssr 除外）。
func ManualProtocols() []Protocol {
	return []Protocol{
		{
			Protocol: "ss", Label: "Shadowsocks",
			FormSchema: []FieldSchema{
				{Name: "cipher", Type: "text", Required: true, Default: "aes-256-gcm", Label: "加密方式"},
				{Name: "password", Type: "password", Required: true, Label: "密码"},
				{Name: "udp", Type: "bool", Default: true, Label: "UDP"},
			},
			SensitiveFields: []string{"password"},
			LinkMappings:    LinkMapping{SR: true, Generic: true, Params: []string{"cipher", "password"}},
		},
		{
			Protocol: "vmess", Label: "VMess",
			FormSchema: []FieldSchema{
				{Name: "uuid", Type: "password", Required: true, Label: "UUID"},
				{Name: "alterId", Type: "number", Default: 0, Label: "AlterId"},
				{Name: "cipher", Type: "text", Default: "auto", Label: "加密方式"},
				{Name: "udp", Type: "bool", Default: true, Label: "UDP"},
				{Name: "network", Type: "select", Default: "tcp", Label: "传输", Options: []string{"tcp", "ws", "grpc", "h2", "http"}},
				{Name: "tls", Type: "bool", Default: false, Label: "TLS"},
				{Name: "servername", Type: "text", Label: "SNI"},
				{Name: "path", Type: "text", Label: "Path"},
				{Name: "host", Type: "text", Label: "Host"},
			},
			SensitiveFields: []string{"uuid"},
			LinkMappings:    LinkMapping{SR: true, Generic: true, Params: []string{"uuid", "alterId", "cipher", "network", "tls", "servername"}},
		},
		{
			Protocol: "vless", Label: "VLESS",
			FormSchema: []FieldSchema{
				{Name: "uuid", Type: "password", Required: true, Label: "UUID"},
				{Name: "flow", Type: "text", Label: "Flow"},
				{Name: "network", Type: "select", Default: "tcp", Label: "传输", Options: []string{"tcp", "ws", "grpc", "h2", "http"}},
				{Name: "tls", Type: "bool", Default: false, Label: "TLS"},
				{Name: "servername", Type: "text", Label: "SNI"},
				{Name: "client-fingerprint", Type: "text", Label: "指纹"},
				{Name: "reality-opts", Type: "text", Label: "REALITY 公钥/ShortId（JSON）"},
				{Name: "udp", Type: "bool", Default: true, Label: "UDP"},
			},
			SensitiveFields: []string{"uuid"},
			LinkMappings:    LinkMapping{SR: true, Generic: true, Params: []string{"uuid", "flow", "network", "tls", "servername", "reality-opts"}},
		},
		{
			Protocol: "trojan", Label: "Trojan",
			FormSchema: []FieldSchema{
				{Name: "password", Type: "password", Required: true, Label: "密码"},
				{Name: "sni", Type: "text", Label: "SNI"},
				{Name: "alpn", Type: "text", Label: "ALPN"},
				{Name: "skip-cert-verify", Type: "bool", Default: false, Label: "跳过证书校验"},
				{Name: "udp", Type: "bool", Default: true, Label: "UDP"},
			},
			SensitiveFields: []string{"password"},
			LinkMappings:    LinkMapping{SR: true, Generic: true, Params: []string{"password", "sni", "alpn"}},
		},
		{
			Protocol: "hysteria", Label: "Hysteria",
			FormSchema: []FieldSchema{
				{Name: "auth", Type: "password", Required: true, Label: "认证"},
				{Name: "protocol", Type: "text", Default: "udp", Label: "协议"},
				{Name: "up", Type: "text", Label: "上行"},
				{Name: "down", Type: "text", Label: "下行"},
				{Name: "sni", Type: "text", Label: "SNI"},
				{Name: "obfs", Type: "text", Label: "混淆"},
			},
			SensitiveFields: []string{"auth"},
			LinkMappings:    LinkMapping{SR: true, Generic: true, Params: []string{"auth", "protocol", "up", "down", "sni", "obfs"}},
		},
		{
			Protocol: "hysteria2", Label: "Hysteria2",
			FormSchema: []FieldSchema{
				{Name: "password", Type: "password", Required: true, Label: "密码"},
				{Name: "sni", Type: "text", Label: "SNI"},
				{Name: "obfs", Type: "text", Label: "混淆"},
				{Name: "obfs-password", Type: "password", Label: "混淆密码"},
				{Name: "insecure", Type: "bool", Default: false, Label: "允许不安全"},
			},
			SensitiveFields: []string{"password", "obfs-password"},
			LinkMappings:    LinkMapping{SR: true, Generic: true, Params: []string{"password", "sni", "obfs", "obfs-password"}},
		},
		{
			Protocol: "tuic", Label: "TUIC",
			FormSchema: []FieldSchema{
				{Name: "uuid", Type: "password", Required: true, Label: "UUID"},
				{Name: "password", Type: "password", Label: "密码"},
				{Name: "sni", Type: "text", Label: "SNI"},
				{Name: "alpn", Type: "text", Label: "ALPN"},
				{Name: "allow_insecure", Type: "bool", Default: false, Label: "允许不安全"},
			},
			SensitiveFields: []string{"uuid", "password"},
			LinkMappings:    LinkMapping{SR: true, Generic: true, Params: []string{"uuid", "password", "sni", "alpn"}},
		},
		{
			Protocol: "wireguard", Label: "WireGuard",
			FormSchema: []FieldSchema{
				{Name: "private-key", Type: "password", Required: true, Label: "私钥"},
				{Name: "public-key", Type: "text", Required: true, Label: "公钥"},
				{Name: "address", Type: "text", Label: "地址"},
				{Name: "allowed-ips", Type: "text", Label: "AllowedIPs"},
				{Name: "pre-shared-key", Type: "password", Label: "预共享密钥"},
				{Name: "mtu", Type: "number", Label: "MTU"},
				{Name: "dns", Type: "text", Label: "DNS"},
			},
			SensitiveFields: []string{"private-key", "pre-shared-key"},
			LinkMappings:    LinkMapping{SR: true, Generic: true, Params: []string{"private-key", "public-key", "address", "allowed-ips"}},
		},
		{
			Protocol: "http", Label: "HTTP",
			FormSchema: []FieldSchema{
				{Name: "username", Type: "text", Label: "用户名"},
				{Name: "password", Type: "password", Label: "密码"},
				{Name: "tls", Type: "bool", Default: false, Label: "TLS"},
				{Name: "skip-cert-verify", Type: "bool", Default: false, Label: "跳过证书校验"},
			},
			SensitiveFields: []string{"password"},
			LinkMappings:    LinkMapping{SR: true, Generic: true, Params: []string{"username", "password", "tls"}},
		},
		{
			Protocol: "socks5", Label: "SOCKS5",
			FormSchema: []FieldSchema{
				{Name: "username", Type: "text", Label: "用户名"},
				{Name: "password", Type: "password", Label: "密码"},
				{Name: "tls", Type: "bool", Default: false, Label: "TLS"},
				{Name: "udp", Type: "bool", Default: true, Label: "UDP"},
				{Name: "skip-cert-verify", Type: "bool", Default: false, Label: "跳过证书校验"},
			},
			SensitiveFields: []string{"password"},
			LinkMappings:    LinkMapping{SR: true, Generic: true, Params: []string{"username", "password", "tls", "udp"}},
		},
		{
			Protocol: "snell", Label: "Snell",
			FormSchema: []FieldSchema{
				{Name: "psk", Type: "password", Required: true, Label: "PSK"},
				{Name: "version", Type: "number", Default: 2, Label: "版本"},
				{Name: "obfs", Type: "text", Label: "混淆"},
			},
			SensitiveFields: []string{"psk"},
			LinkMappings:    LinkMapping{SR: false, Generic: false},
		},
		{
			Protocol: "anytls", Label: "AnyTLS",
			FormSchema: []FieldSchema{
				{Name: "password", Type: "password", Required: true, Label: "密码"},
				{Name: "sni", Type: "text", Label: "SNI"},
				{Name: "alpn", Type: "text", Label: "ALPN"},
				{Name: "client-fingerprint", Type: "text", Label: "指纹"},
				{Name: "allowInsecure", Type: "bool", Default: false, Label: "允许不安全"},
				{Name: "udp", Type: "bool", Default: true, Label: "UDP"},
			},
			SensitiveFields: []string{"password"},
			LinkMappings:    LinkMapping{SR: true, Generic: true, Params: []string{"password", "sni", "alpn"}},
		},
		{
			Protocol: "mieru", Label: "Mieru",
			FormSchema: []FieldSchema{
				{Name: "username", Type: "text", Required: true, Label: "用户名"},
				{Name: "password", Type: "password", Required: true, Label: "密码"},
			},
			SensitiveFields: []string{"password"},
			LinkMappings:    LinkMapping{SR: false, Generic: false},
		},
		{
			Protocol: "masque", Label: "MASQUE",
			FormSchema: []FieldSchema{
				{Name: "password", Type: "password", Label: "密码"},
				{Name: "sni", Type: "text", Label: "SNI"},
			},
			SensitiveFields: []string{"password"},
			LinkMappings:    LinkMapping{SR: false, Generic: false},
		},
		{
			Protocol: "openvpn", Label: "OpenVPN",
			FormSchema: []FieldSchema{
				{Name: "client-config", Type: "text", Required: true, Label: "客户端配置"},
			},
			SensitiveFields: []string{},
			LinkMappings:    LinkMapping{SR: false, Generic: false},
		},
		{
			Protocol: "ssh", Label: "SSH",
			FormSchema: []FieldSchema{
				{Name: "username", Type: "text", Required: true, Label: "用户名"},
				{Name: "private-key", Type: "password", Label: "私钥"},
			},
			SensitiveFields: []string{"private-key"},
			LinkMappings:    LinkMapping{SR: false, Generic: false},
		},
		{
			Protocol: "shadowquic", Label: "ShadowQUIC",
			FormSchema: []FieldSchema{
				{Name: "password", Type: "password", Required: true, Label: "密码"},
				{Name: "sni", Type: "text", Label: "SNI"},
			},
			SensitiveFields: []string{"password"},
			LinkMappings:    LinkMapping{SR: false, Generic: false},
		},
		{
			Protocol: "trusttunnel", Label: "TrustTunnel",
			FormSchema: []FieldSchema{
				{Name: "password", Type: "password", Label: "密码"},
			},
			SensitiveFields: []string{"password"},
			LinkMappings:    LinkMapping{SR: false, Generic: false},
		},
		{
			Protocol: "tailscale", Label: "Tailscale",
			FormSchema: []FieldSchema{
				{Name: "auth-key", Type: "password", Label: "认证密钥"},
			},
			SensitiveFields: []string{"auth-key"},
			LinkMappings:    LinkMapping{SR: false, Generic: false},
		},
	}
}

// protocolIndex 供校验与读取使用。
var protocolIndex = func() map[string]Protocol {
	m := make(map[string]Protocol, len(ManualProtocols()))
	for _, p := range ManualProtocols() {
		m[p.Protocol] = p
	}
	return m
}()

// GetProtocol 按协议名返回注册表条目。
func GetProtocol(name string) (Protocol, error) {
	p, ok := protocolIndex[name]
	if !ok {
		return Protocol{}, fmt.Errorf("不支持的协议: %s", name)
	}
	return p, nil
}

// HasProtocol 判断协议是否在 manual 注册表内。
func HasProtocol(name string) bool {
	_, ok := protocolIndex[name]
	return ok
}

// ProtocolNames 返回 manual 协议名列表（按注册表顺序）。
func ProtocolNames() []string {
	out := make([]string, 0, len(ManualProtocols()))
	for _, p := range ManualProtocols() {
		out = append(out, p.Protocol)
	}
	return out
}

// SensitiveFieldsOf 返回某协议敏感字段清单。
func SensitiveFieldsOf(protocol string) []string {
	p, ok := protocolIndex[protocol]
	if !ok {
		return nil
	}
	return p.SensitiveFields
}

// ErrProtocolNotFound 供调用方判断协议不存在。
var ErrProtocolNotFound = errors.New("协议不在注册表")
