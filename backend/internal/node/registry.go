// Package node 提供统一节点表的协议注册表与业务能力。
package node

import "fmt"

// FieldSchema 描述协议表单字段（供前端动态渲染）。
type FieldSchema struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"` // text/password/number/bool/select/object/text-list/int-list
	Required bool     `json:"required"`
	Default  any      `json:"default,omitempty"`
	Label    string   `json:"label"`
	Help     string   `json:"help,omitempty"`
	Options  []string `json:"options,omitempty"`
}

type LinkMapping struct {
	SR      bool     `json:"sr"`
	Generic bool     `json:"generic"`
	Params  []string `json:"params,omitempty"`
}
type Protocol struct {
	Protocol        string        `json:"protocol"`
	Label           string        `json:"label"`
	FormSchema      []FieldSchema `json:"form_schema"`
	SensitiveFields []string      `json:"sensitive_fields"`
	LinkMappings    LinkMapping   `json:"link_mappings"`
}

func f(name, typ, label string) FieldSchema   { return FieldSchema{Name: name, Type: typ, Label: label} }
func req(name, typ, label string) FieldSchema { v := f(name, typ, label); v.Required = true; return v }
func def(name, typ, label string, value any) FieldSchema {
	v := f(name, typ, label)
	v.Default = value
	return v
}
func sel(name, label string, value any, options ...string) FieldSchema {
	v := def(name, "select", label, value)
	v.Options = options
	return v
}
func commonFieldSchema() []FieldSchema {
	return []FieldSchema{
		def("tfo", "bool", "TCP Fast Open", false), def("mptcp", "bool", "MPTCP", false), f("interface-name", "text", "出站网卡"),
		f("routing-mark", "number", "路由标记"), sel("ip-version", "IP 版本", "dual", "dual", "ipv4", "ipv6", "ipv4-prefer", "ipv6-prefer"), f("dialer-proxy", "text", "拨号代理"),
	}
}
func common(v ...FieldSchema) []FieldSchema { return append(v, commonFieldSchema()...) }
func links(params ...string) LinkMapping    { return LinkMapping{SR: true, Generic: true, Params: params} }

// ManualProtocols 返回 manual 节点可用的协议注册表（19 项封闭清单，ssr 除外）。
func ManualProtocols() []Protocol {
	return []Protocol{
		{Protocol: "ss", Label: "Shadowsocks", FormSchema: common(
			req("cipher", "text", "加密方式"), req("password", "password", "密码"), def("udp", "bool", "UDP", true), sel("plugin", "插件", "", "", "obfs", "v2ray-plugin", "shadow-tls", "restls"),
			f("plugin-opts", "object", "插件参数"), def("udp-over-tcp", "bool", "UDP over TCP", false), f("udp-over-tcp-version", "number", "UDP over TCP 版本"), f("client-fingerprint", "text", "客户端指纹"), f("smux", "object", "多路复用")),
			SensitiveFields: []string{"password", "plugin-opts.password"}, LinkMappings: links("cipher", "password", "plugin", "plugin-opts")},
		{Protocol: "vmess", Label: "VMess", FormSchema: common(
			req("uuid", "password", "UUID"), def("alterId", "number", "AlterId", 0), def("cipher", "text", "加密方式", "auto"), def("udp", "bool", "UDP", true), sel("network", "传输", "tcp", "tcp", "ws", "grpc", "h2", "http"),
			def("tls", "bool", "TLS", false), f("servername", "text", "SNI"), f("alpn", "text-list", "ALPN"), def("packet-addr", "bool", "Packet Address", false), def("xudp", "bool", "XUDP", false), f("packet-encoding", "text", "包编码"),
			def("skip-cert-verify", "bool", "跳过证书校验", false), f("fingerprint", "text", "TLS 指纹"), f("client-fingerprint", "text", "客户端指纹"), f("reality-opts", "object", "REALITY"),
			f("http-opts", "object", "HTTP 参数"), f("h2-opts", "object", "H2 参数"), f("grpc-opts", "object", "gRPC 参数"), f("ws-opts", "object", "WebSocket 参数"), def("global-padding", "bool", "全局填充", false),
			def("authenticated-length", "bool", "认证长度", false), f("smux", "object", "多路复用")), SensitiveFields: []string{"uuid"}, LinkMappings: links("uuid", "alterId", "cipher", "network", "tls", "servername", "alpn", "reality-opts", "ws-opts", "grpc-opts")},
		{Protocol: "vless", Label: "VLESS", FormSchema: common(
			req("uuid", "password", "UUID"), f("flow", "text", "Flow"), def("tls", "bool", "TLS", false), f("alpn", "text-list", "ALPN"), def("udp", "bool", "UDP", true), def("packet-addr", "bool", "Packet Address", false),
			def("xudp", "bool", "XUDP", false), f("packet-encoding", "text", "包编码"), sel("network", "传输", "tcp", "tcp", "ws", "grpc", "h2", "http", "xhttp"), f("reality-opts", "object", "REALITY"),
			f("http-opts", "object", "HTTP 参数"), f("h2-opts", "object", "H2 参数"), f("grpc-opts", "object", "gRPC 参数"), f("ws-opts", "object", "WebSocket 参数"), f("xhttp-opts", "object", "XHTTP 参数"),
			f("ws-path", "text", "WebSocket Path"), f("ws-headers", "object", "WebSocket Headers"), def("skip-cert-verify", "bool", "跳过证书校验", false), f("fingerprint", "text", "TLS 指纹"),
			f("servername", "text", "SNI"), f("client-fingerprint", "text", "客户端指纹"), f("smux", "object", "多路复用"), def("encryption", "text", "加密", "none")),
			SensitiveFields: []string{"uuid"}, LinkMappings: links("uuid", "flow", "network", "tls", "servername", "alpn", "reality-opts", "ws-opts", "grpc-opts", "h2-opts", "http-opts", "xhttp-opts")},
		{Protocol: "trojan", Label: "Trojan", FormSchema: common(
			req("password", "password", "密码"), f("alpn", "text-list", "ALPN"), f("sni", "text", "SNI"), def("skip-cert-verify", "bool", "跳过证书校验", false), f("fingerprint", "text", "TLS 指纹"), def("udp", "bool", "UDP", true),
			sel("network", "传输", "tcp", "tcp", "ws", "http", "h2", "grpc", "xhttp"), f("reality-opts", "object", "REALITY"), f("grpc-opts", "object", "gRPC 参数"), f("ws-opts", "object", "WebSocket 参数"), f("ss-opts", "object", "内层 SS"), f("client-fingerprint", "text", "客户端指纹")),
			SensitiveFields: []string{"password", "ss-opts.password"}, LinkMappings: links("password", "sni", "alpn", "network", "reality-opts", "ws-opts", "grpc-opts")},
		{Protocol: "hysteria", Label: "Hysteria", FormSchema: common(
			req("auth", "password", "认证"), f("auth-str", "password", "认证字符串"), f("ports", "text", "端口组"), f("protocol", "text", "协议"), f("obfs-protocol", "text", "混淆协议"), f("up", "text", "上行"), f("up-speed", "number", "上行速率"),
			f("down", "text", "下行"), f("down-speed", "number", "下行速率"), f("obfs", "text", "混淆"), f("sni", "text", "SNI"), def("skip-cert-verify", "bool", "跳过证书校验", false), f("fingerprint", "text", "TLS 指纹"),
			f("alpn", "text-list", "ALPN"), f("ca", "text", "CA 文件"), f("ca-str", "text", "CA 内容"), f("recv-window-conn", "number", "连接接收窗口"), f("recv-window", "number", "接收窗口"),
			def("disable-mtu-discovery", "bool", "禁用 MTU 发现", false), def("fast-open", "bool", "Fast Open", false), f("hop-interval", "number", "Hop 间隔")), SensitiveFields: []string{"auth", "auth-str"}, LinkMappings: links("auth", "protocol", "up", "down", "sni", "alpn", "ports", "obfs")},
		{Protocol: "hysteria2", Label: "Hysteria2", FormSchema: common(
			req("password", "password", "密码"), f("ports", "text", "端口组"), f("hop-interval", "number", "Hop 间隔"), f("protocol", "text", "协议"), f("obfs-protocol", "text", "混淆协议"), f("up", "text", "上行"), f("down", "text", "下行"),
			f("obfs", "text", "混淆"), f("obfs-password", "password", "混淆密码"), f("sni", "text", "SNI"), def("skip-cert-verify", "bool", "跳过证书校验", false), f("fingerprint", "text", "TLS 指纹"), f("alpn", "text-list", "ALPN"),
			f("ca", "text", "CA 文件"), f("ca-str", "text", "CA 内容"), f("cwnd", "number", "拥塞窗口"), f("udp-mtu", "number", "UDP MTU")), SensitiveFields: []string{"password", "obfs-password"}, LinkMappings: links("password", "sni", "alpn", "obfs", "obfs-password", "ports")},
		{Protocol: "tuic", Label: "TUIC", FormSchema: common(
			f("token", "password", "Token"), f("uuid", "password", "UUID"), f("password", "password", "密码"), f("ip", "text", "IP"), f("heartbeat-interval", "number", "心跳间隔"), f("alpn", "text-list", "ALPN"), def("reduce-rtt", "bool", "减少 RTT", false),
			f("request-timeout", "number", "请求超时"), f("udp-relay-mode", "text", "UDP 中继模式"), f("congestion-controller", "text", "拥塞控制器"), def("disable-sni", "bool", "禁用 SNI", false), f("max-udp-relay-packet-size", "number", "最大 UDP 中继包"),
			def("fast-open", "bool", "Fast Open", false), f("max-open-streams", "number", "最大并发流"), f("cwnd", "number", "拥塞窗口"), def("skip-cert-verify", "bool", "跳过证书校验", false), f("fingerprint", "text", "TLS 指纹"),
			f("ca", "text", "CA 文件"), f("ca-str", "text", "CA 内容"), f("recv-window-conn", "number", "连接接收窗口"), f("recv-window", "number", "接收窗口"), def("disable-mtu-discovery", "bool", "禁用 MTU 发现", false),
			f("max-datagram-frame-size", "number", "最大数据报帧"), f("sni", "text", "SNI"), def("udp-over-stream", "bool", "UDP over Stream", false), f("udp-over-stream-version", "number", "UDP over Stream 版本")),
			SensitiveFields: []string{"token", "uuid", "password"}, LinkMappings: links("token", "uuid", "password", "sni", "alpn")},
		{Protocol: "wireguard", Label: "WireGuard", FormSchema: common(
			req("private-key", "password", "私钥"), req("public-key", "text", "公钥"), f("pre-shared-key", "password", "预共享密钥"), f("reserved", "int-list", "保留字节"), f("allowed-ips", "text-list", "Allowed IPs"),
			f("ip", "text", "IP"), f("ipv6", "text", "IPv6"), f("workers", "number", "Worker 数"), f("mtu", "number", "MTU"), def("udp", "bool", "UDP", true), f("persistent-keepalive", "number", "持久 Keepalive"),
			f("peers", "object", "Peer 列表"), def("remote-dns-resolve", "bool", "远端 DNS 解析", false), f("dns", "text-list", "DNS"), f("refresh-server-ip-interval", "number", "刷新服务器 IP 间隔")),
			SensitiveFields: []string{"private-key", "pre-shared-key"}, LinkMappings: links("private-key", "public-key", "ip", "ipv6", "allowed-ips", "pre-shared-key", "mtu", "dns")},
		{Protocol: "http", Label: "HTTP", FormSchema: common(f("username", "text", "用户名"), f("password", "password", "密码"), def("tls", "bool", "TLS", false), f("sni", "text", "SNI"), def("skip-cert-verify", "bool", "跳过证书校验", false), f("fingerprint", "text", "TLS 指纹"), f("headers", "object", "请求头")), SensitiveFields: []string{"password"}, LinkMappings: links("username", "password", "tls", "sni")},
		{Protocol: "socks5", Label: "SOCKS5", FormSchema: common(f("username", "text", "用户名"), f("password", "password", "密码"), def("tls", "bool", "TLS", false), def("udp", "bool", "UDP", true), def("skip-cert-verify", "bool", "跳过证书校验", false), f("fingerprint", "text", "TLS 指纹")), SensitiveFields: []string{"password"}, LinkMappings: links("username", "password", "tls", "udp")},
		{Protocol: "snell", Label: "Snell", FormSchema: common(req("psk", "password", "PSK"), def("udp", "bool", "UDP", true), def("version", "number", "版本", 2)), SensitiveFields: []string{"psk"}},
		{Protocol: "anytls", Label: "AnyTLS", FormSchema: common(req("password", "password", "密码"), f("alpn", "text-list", "ALPN"), f("sni", "text", "SNI"), f("client-fingerprint", "text", "客户端指纹"), def("skip-cert-verify", "bool", "跳过证书校验", false), f("fingerprint", "text", "TLS 指纹"), f("certificate", "text", "证书"), f("private-key", "password", "私钥"), f("ech-opts", "object", "ECH 参数"), def("udp", "bool", "UDP", true), f("idle-session-check-interval", "number", "空闲检查间隔"), f("idle-session-timeout", "number", "空闲超时"), f("min-idle-session", "number", "最小空闲会话")), SensitiveFields: []string{"password", "private-key"}, LinkMappings: links("password", "sni", "alpn", "client-fingerprint")},
		{Protocol: "mieru", Label: "Mieru", FormSchema: common(req("username", "text", "用户名"), req("password", "password", "密码"), f("port-range", "text", "端口范围"), sel("transport", "传输", "TCP", "TCP", "UDP"), def("udp", "bool", "UDP", true), sel("multiplexing", "多路复用", "MULTIPLEXING_OFF", "MULTIPLEXING_OFF", "LOW", "MIDDLE", "HIGH"), f("handshake-mode", "text", "握手模式")), SensitiveFields: []string{"password"}},
		{Protocol: "masque", Label: "MASQUE", FormSchema: common(req("private-key", "password", "私钥"), req("public-key", "text", "公钥"), f("ip", "text", "IP"), f("ipv6", "text", "IPv6"), f("mtu", "number", "MTU"), def("udp", "bool", "UDP", true), def("remote-dns-resolve", "bool", "远端 DNS 解析", false), f("dns", "text-list", "DNS")), SensitiveFields: []string{"private-key"}},
		{Protocol: "openvpn", Label: "OpenVPN", FormSchema: common(req("client-config", "text", "客户端配置"))},
		{Protocol: "ssh", Label: "SSH", FormSchema: common(req("username", "text", "用户名"), f("password", "password", "密码"), f("private-key", "password", "私钥"), f("private-key-passphrase", "password", "私钥口令"), f("host-key", "text", "Host Key"), f("host-key-algorithms", "text", "Host Key 算法")), SensitiveFields: []string{"password", "private-key", "private-key-passphrase"}},
		{Protocol: "shadowquic", Label: "ShadowQUIC", FormSchema: common(req("password", "password", "密码"), f("sni", "text", "SNI")), SensitiveFields: []string{"password"}},
		{Protocol: "trusttunnel", Label: "TrustTunnel", FormSchema: common(f("password", "password", "密码")), SensitiveFields: []string{"password"}},
		{Protocol: "tailscale", Label: "Tailscale", FormSchema: common(f("auth-key", "password", "认证密钥")), SensitiveFields: []string{"auth-key"}},
	}
}

var protocolIndex = func() map[string]Protocol {
	m := make(map[string]Protocol)
	for _, p := range ManualProtocols() {
		m[p.Protocol] = p
	}
	return m
}()

func GetProtocol(name string) (Protocol, error) {
	p, ok := protocolIndex[name]
	if !ok {
		return Protocol{}, fmt.Errorf("不支持的协议: %s", name)
	}
	return p, nil
}
func HasProtocol(name string) bool { _, ok := protocolIndex[name]; return ok }
func SensitiveFieldsOf(protocol string) []string {
	if p, ok := protocolIndex[protocol]; ok {
		return p.SensitiveFields
	}
	return nil
}
