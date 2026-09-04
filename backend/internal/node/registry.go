// Package node 提供统一节点表的协议注册表与业务能力。
package node

import (
	"fmt"

	"vpn-sub/internal/ssplugin"
)

// FieldSchema 描述协议表单字段（供前端动态渲染）。
type FieldSchema struct {
	Name           string           `json:"name"`
	Type           string           `json:"type"` // text/password/number/bool/select/object/text-list/int-list
	Required       bool             `json:"required"`
	Default        any              `json:"default,omitempty"`
	Label          string           `json:"label"`
	Help           string           `json:"help,omitempty"`
	Options        []string         `json:"options,omitempty"`
	Section        string           `json:"section,omitempty"`        // auth/transport/security/switches/advanced
	ObjectKind     string           `json:"object_kind,omitempty"`    // fields/map/list
	MapValueType   string           `json:"map_value_type,omitempty"` // map 叶子值类型；当前支持 string
	ItemIDField    string           `json:"item_id_field,omitempty"`  // 含敏感子字段的 list 条目稳定身份
	Properties     []FieldSchema    `json:"properties,omitempty"`     // fields 属性或 list 元素字段
	AllowUnknown   bool             `json:"allow_unknown,omitempty"`  // 保留客户端扩展键
	Group          string           `json:"group,omitempty"`          // basic/auth/connection/switches/advanced
	Advanced       bool             `json:"advanced,omitempty"`       // 开关/高级区内的“更多/高级”分层标记
	When           *ConditionRule   `json:"when,omitempty"`
	RequiredWhen   *ConditionRule   `json:"required_when,omitempty"`
	ResetOn        []string         `json:"reset_on,omitempty"`
	Feature        *FeatureSchema   `json:"feature,omitempty"`
	OptionItems    []OptionItem     `json:"option_items,omitempty"`
	AllowCustom    *bool            `json:"allow_custom,omitempty"`
	CanonicalPath  string           `json:"canonical_path,omitempty"`
	Aliases        []string         `json:"aliases,omitempty"`
	TargetEvidence []TargetEvidence `json:"target_evidence,omitempty"`
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

func f(name, typ, label string) FieldSchema {
	return FieldSchema{Name: name, Type: typ, Label: label, Section: fieldSection(name, typ)}
}

func boolPtr(value bool) *bool { return &value }

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

func obj(name, label, kind string, properties ...FieldSchema) FieldSchema {
	v := f(name, "object", label)
	v.ObjectKind = kind
	v.Properties = properties
	v.AllowUnknown = true
	return v
}

func customPluginOpts() FieldSchema {
	field := obj("plugin-opts", "自定义插件参数", "map")
	field.MapValueType = "string"
	field.Help = "仅用于未知自定义插件；所有键值均为普通字符串参数。"
	return field
}

func fieldSection(name, typ string) string {
	if typ == "bool" {
		return "switches"
	}
	switch name {
	case "uuid", "password", "username", "token", "auth", "auth-str", "obfs-password", "private-key", "public-key", "pre-shared-key", "psk", "auth-key", "private-key-passphrase":
		return "auth"
	case "sni", "servername", "alpn", "fingerprint", "client-fingerprint", "certificate", "ca", "ca-str", "host-key", "host-key-algorithms", "reality-opts", "ech-opts":
		return "security"
	case "cipher", "flow", "network", "transport", "plugin", "plugin-opts", "obfs-opts", "v2ray-plugin-opts", "shadow-tls-opts", "restls-opts", "http-opts", "h2-opts", "grpc-opts", "ws-opts", "xhttp-opts", "ws-path", "ws-headers", "ss-opts", "smux", "multiplexing", "packet-encoding":
		return "transport"
	default:
		return "advanced"
	}
}

func realityOpts() FieldSchema {
	return obj("reality-opts", "REALITY", "fields",
		f("public-key", "text", "公钥"), f("short-id", "text", "Short ID"))
}

func grpcOpts() FieldSchema {
	return obj("grpc-opts", "gRPC 参数", "fields", f("grpc-service-name", "text", "服务名称"))
}

func wsOpts() FieldSchema {
	return obj("ws-opts", "WebSocket 参数", "fields",
		f("path", "text", "路径"), obj("headers", "请求头", "map"), f("max-early-data", "number", "Early Data 上限"),
		f("early-data-header-name", "text", "Early Data Header"), def("v2ray-http-upgrade", "bool", "HTTP Upgrade", false),
		def("v2ray-http-upgrade-fast-open", "bool", "HTTP Upgrade Fast Open", false))
}

func httpOpts() FieldSchema {
	return obj("http-opts", "HTTP 参数", "fields",
		f("method", "text", "方法"), f("path", "text-list", "路径"), obj("headers", "请求头", "map"))
}

func h2Opts() FieldSchema {
	return obj("h2-opts", "H2 参数", "fields", f("path", "text", "路径"), f("host", "text", "Host"))
}

func xhttpOpts() FieldSchema {
	mode := f("mode", "select", "模式")
	mode.Options = []string{"auto", "stream-one", "stream-up", "packet-up"}
	return obj("xhttp-opts", "XHTTP 参数", "fields",
		f("path", "text", "路径"), f("host", "text", "Host"), mode)
}

func smuxOpts() FieldSchema {
	brutal := featureObject("smux.brutal", obj("brutal-opts", "Brutal 参数", "fields",
		def("enabled", "bool", "启用", false), f("up", "text", "上行"), f("down", "text", "下行")))
	return featureObject("smux", obj("smux", "多路复用", "fields",
		def("enabled", "bool", "启用", false), sel("protocol", "协议", "smux", "smux", "yamux", "h2mux"),
		f("max-connections", "number", "最大连接数"), f("min-streams", "number", "最小流数"), f("max-streams", "number", "最大流数"),
		def("padding", "bool", "填充", false), def("statistic", "bool", "统计", false), def("only-tcp", "bool", "仅 TCP", false),
		brutal))
}

func obfsOpts() FieldSchema {
	mode := f("mode", "select", "模式")
	mode.Default = "http"
	mode.AllowCustom = boolPtr(true)
	setOptionItems(&mode, option("http", "HTTP", "common", "mihomo-1.19.29"), option("tls", "TLS", "common", "mihomo-1.19.29"))
	return obj("obfs-opts", "obfs 参数", "fields", mode, f("host", "text", "Host"))
}

func v2rayPluginOpts() FieldSchema {
	mode := f("mode", "select", "模式")
	mode.Default = "websocket"
	mode.AllowCustom = boolPtr(true)
	setOptionItems(&mode, option("websocket", "WebSocket", "common", "mihomo-1.19.29"))
	version := f("version", "select", "版本")
	version.AllowCustom = boolPtr(true)
	setOptionItems(&version, option("1", "v1", "common", "mihomo-1.19.29"), option("2", "v2", "common", "mihomo-1.19.29"))
	return obj("v2ray-plugin-opts", "v2ray-plugin 参数", "fields",
		mode, f("host", "text", "Host"), def("tls", "bool", "TLS", false), f("path", "text", "路径"), obj("headers", "请求头", "map"),
		version, def("mux", "bool", "Mux", false), def("v2ray-http-upgrade", "bool", "HTTP Upgrade", false),
		def("v2ray-http-upgrade-fast-open", "bool", "HTTP Upgrade Fast Open", false))
}

func shadowTlsOpts() FieldSchema {
	return obj("shadow-tls-opts", "shadow-tls 参数", "fields",
		f("password", "password", "密码"), f("fingerprint", "text", "指纹"), def("skip-cert-verify", "bool", "跳过证书校验", false))
}

func restlsOpts() FieldSchema {
	return obj("restls-opts", "restls 参数", "fields",
		f("password", "password", "密码"), f("path", "text", "路径"), f("fingerprint", "text", "指纹"),
		def("skip-cert-verify", "bool", "跳过证书校验", false), f("version-hint", "text", "版本提示"), f("restls-script", "text", "Restls Script"))
}

func ssOpts() FieldSchema {
	return obj("ss-opts", "内层 SS", "fields",
		def("enabled", "bool", "启用内层 SS", false), f("method", "text", "内层加密方式"),
		f("password", "password", "密码"))
}

func wireGuardPeers() FieldSchema {
	peers := obj("peers", "Peer 列表", "list",
		f("server", "text", "服务器"), f("port", "number", "端口"), f("public-key", "text", "公钥"),
		f("pre-shared-key", "password", "预共享密钥"), f("reserved", "int-list", "保留字节"), f("allowed-ips", "text-list", "Allowed IPs"))
	peers.ItemIDField = sensitiveItemIDField
	return peers
}
func commonFieldSchema() []FieldSchema {
	tfo := def("tfo", "bool", "TCP Fast Open", false)
	mptcp := def("mptcp", "bool", "MPTCP", false)
	mptcp.Advanced = true
	return []FieldSchema{
		tfo, mptcp, f("interface-name", "text", "出站网卡"),
		f("routing-mark", "number", "路由标记"), sel("ip-version", "IP 版本", "dual", "dual", "ipv4", "ipv6", "ipv4-prefer", "ipv6-prefer"), f("dialer-proxy", "text", "拨号代理"),
	}
}
func common(v ...FieldSchema) []FieldSchema { return append(v, commonFieldSchema()...) }
func links(params ...string) LinkMapping    { return LinkMapping{SR: true, Generic: true, Params: params} }

// ManualProtocols 返回 manual 节点可用的协议注册表（19 项封闭清单，ssr 除外）。
func ManualProtocols() []Protocol {
	protocols := []Protocol{
		{Protocol: "ss", Label: "Shadowsocks", FormSchema: common(
			req("cipher", "text", "加密方式"), req("password", "password", "密码"), def("udp", "bool", "UDP", true), sel("plugin", "插件", "", "", "obfs", "v2ray-plugin", "shadow-tls", "restls"),
			customPluginOpts(), obfsOpts(), v2rayPluginOpts(), shadowTlsOpts(), restlsOpts(),
			def("udp-over-tcp", "bool", "UDP over TCP", false), f("udp-over-tcp-version", "number", "UDP over TCP 版本"), f("client-fingerprint", "text", "客户端指纹"), smuxOpts()),
			SensitiveFields: []string{"password", "shadow-tls-opts.password", "restls-opts.password"}, LinkMappings: links("cipher", "password", "plugin", "obfs-opts", "v2ray-plugin-opts", "shadow-tls-opts", "restls-opts")},
		{Protocol: "vmess", Label: "VMess", FormSchema: common(
			req("uuid", "password", "UUID"), def("alterId", "number", "AlterId", 0), def("cipher", "text", "加密方式", "auto"), def("udp", "bool", "UDP", true), sel("network", "传输", "tcp", "tcp", "ws", "grpc", "h2", "http"),
			def("tls", "bool", "TLS", false), f("servername", "text", "SNI"), f("alpn", "text-list", "ALPN"), def("packet-addr", "bool", "Packet Address", false), def("xudp", "bool", "XUDP", false), f("packet-encoding", "text", "包编码"),
			def("skip-cert-verify", "bool", "跳过证书校验", false), f("fingerprint", "text", "TLS 指纹"), f("client-fingerprint", "text", "客户端指纹"), realityOpts(),
			httpOpts(), h2Opts(), grpcOpts(), wsOpts(), def("global-padding", "bool", "全局填充", false),
			def("authenticated-length", "bool", "认证长度", false), smuxOpts()), SensitiveFields: []string{"uuid"}, LinkMappings: links("uuid", "alterId", "cipher", "network", "tls", "servername", "alpn", "reality-opts", "ws-opts", "grpc-opts")},
		{Protocol: "vless", Label: "VLESS", FormSchema: common(
			req("uuid", "password", "UUID"), f("flow", "text", "Flow"), def("tls", "bool", "TLS", false), f("alpn", "text-list", "ALPN"), def("udp", "bool", "UDP", true), def("packet-addr", "bool", "Packet Address", false),
			def("xudp", "bool", "XUDP", false), f("packet-encoding", "text", "包编码"), sel("network", "传输", "tcp", "tcp", "ws", "grpc", "h2", "http", "xhttp"), realityOpts(),
			httpOpts(), h2Opts(), grpcOpts(), wsOpts(), xhttpOpts(),
			f("ws-path", "text", "WebSocket Path"), obj("ws-headers", "WebSocket Headers", "map"), def("skip-cert-verify", "bool", "跳过证书校验", false), f("fingerprint", "text", "TLS 指纹"),
			f("servername", "text", "SNI"), f("client-fingerprint", "text", "客户端指纹"), smuxOpts(), def("encryption", "text", "加密", "none")),
			SensitiveFields: []string{"uuid"}, LinkMappings: links("uuid", "flow", "network", "tls", "servername", "alpn", "reality-opts", "ws-opts", "grpc-opts", "h2-opts", "http-opts", "xhttp-opts")},
		{Protocol: "trojan", Label: "Trojan", FormSchema: common(
			req("password", "password", "密码"), f("alpn", "text-list", "ALPN"), f("sni", "text", "SNI"), def("skip-cert-verify", "bool", "跳过证书校验", false), f("fingerprint", "text", "TLS 指纹"), def("udp", "bool", "UDP", true),
			sel("network", "传输", "tcp", "tcp", "ws", "http", "h2", "grpc", "xhttp"), realityOpts(), grpcOpts(), wsOpts(), ssOpts(), f("client-fingerprint", "text", "客户端指纹")),
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
			wireGuardPeers(), def("remote-dns-resolve", "bool", "远端 DNS 解析", false), f("dns", "text-list", "DNS"), f("refresh-server-ip-interval", "number", "刷新服务器 IP 间隔")),
			SensitiveFields: []string{"private-key", "pre-shared-key", "peers[].pre-shared-key"}, LinkMappings: links("private-key", "public-key", "ip", "ipv6", "allowed-ips", "pre-shared-key", "mtu", "dns")},
		{Protocol: "http", Label: "HTTP", FormSchema: common(f("username", "text", "用户名"), f("password", "password", "密码"), def("tls", "bool", "TLS", false), f("sni", "text", "SNI"), def("skip-cert-verify", "bool", "跳过证书校验", false), f("fingerprint", "text", "TLS 指纹"), obj("headers", "请求头", "map")), SensitiveFields: []string{"password"}, LinkMappings: links("username", "password", "tls", "sni")},
		{Protocol: "socks5", Label: "SOCKS5", FormSchema: common(f("username", "text", "用户名"), f("password", "password", "密码"), def("tls", "bool", "TLS", false), def("udp", "bool", "UDP", true), def("skip-cert-verify", "bool", "跳过证书校验", false), f("fingerprint", "text", "TLS 指纹")), SensitiveFields: []string{"password"}, LinkMappings: links("username", "password", "tls", "udp")},
		{Protocol: "snell", Label: "Snell", FormSchema: common(req("psk", "password", "PSK"), def("udp", "bool", "UDP", true), def("version", "number", "版本", 2)), SensitiveFields: []string{"psk"}},
		{Protocol: "anytls", Label: "AnyTLS", FormSchema: common(req("password", "password", "密码"), f("alpn", "text-list", "ALPN"), f("sni", "text", "SNI"), f("client-fingerprint", "text", "客户端指纹"), def("skip-cert-verify", "bool", "跳过证书校验", false), f("fingerprint", "text", "TLS 指纹"), f("certificate", "text", "证书"), f("private-key", "password", "私钥"), obj("ech-opts", "ECH 参数", "fields", def("enable", "bool", "启用", false), f("config", "text", "配置")), def("udp", "bool", "UDP", true), f("idle-session-check-interval", "number", "空闲检查间隔"), f("idle-session-timeout", "number", "空闲超时"), f("min-idle-session", "number", "最小空闲会话")), SensitiveFields: []string{"password", "private-key"}, LinkMappings: links("password", "sni", "alpn", "client-fingerprint")},
		{Protocol: "mieru", Label: "Mieru", FormSchema: common(req("username", "text", "用户名"), req("password", "password", "密码"), f("port-range", "text", "端口范围"), sel("transport", "传输", "TCP", "TCP", "UDP"), def("udp", "bool", "UDP", true), sel("multiplexing", "多路复用", "MULTIPLEXING_OFF", "MULTIPLEXING_OFF", "LOW", "MIDDLE", "HIGH"), f("handshake-mode", "text", "握手模式")), SensitiveFields: []string{"password"}},
		{Protocol: "masque", Label: "MASQUE", FormSchema: common(req("private-key", "password", "私钥"), req("public-key", "text", "公钥"), f("ip", "text", "IP"), f("ipv6", "text", "IPv6"), f("mtu", "number", "MTU"), def("udp", "bool", "UDP", true), def("remote-dns-resolve", "bool", "远端 DNS 解析", false), f("dns", "text-list", "DNS")), SensitiveFields: []string{"private-key"}},
		{Protocol: "openvpn", Label: "OpenVPN", FormSchema: common(req("client-config", "text", "客户端配置"))},
		{Protocol: "ssh", Label: "SSH", FormSchema: common(req("username", "text", "用户名"), f("password", "password", "密码"), f("private-key", "password", "私钥"), f("private-key-passphrase", "password", "私钥口令"), f("host-key", "text", "Host Key"), f("host-key-algorithms", "text", "Host Key 算法")), SensitiveFields: []string{"password", "private-key", "private-key-passphrase"}},
		{Protocol: "shadowquic", Label: "ShadowQUIC", FormSchema: common(req("password", "password", "密码"), f("sni", "text", "SNI")), SensitiveFields: []string{"password"}},
		{Protocol: "trusttunnel", Label: "TrustTunnel", FormSchema: common(f("password", "password", "密码")), SensitiveFields: []string{"password"}},
		{Protocol: "tailscale", Label: "Tailscale", FormSchema: common(f("auth-key", "password", "认证密钥")), SensitiveFields: []string{"auth-key"}},
	}
	enrichFirstBatchProtocols(protocols)
	return protocols
}

// editorFormSchema 排除新连接模型已取代的顶层入口，不影响内部字段校验与嵌套插件参数。
func editorFormSchema(proto Protocol) []FieldSchema {
	if proto.Protocol != "vless" && proto.Protocol != "vmess" {
		return proto.FormSchema
	}
	fields := make([]FieldSchema, 0, len(proto.FormSchema))
	for _, field := range proto.FormSchema {
		if field.Name == "tls" || proto.Protocol == "vless" && (field.Name == "ws-path" || field.Name == "ws-headers") {
			continue
		}
		fields = append(fields, field)
	}
	return fields
}

func enrichFirstBatchProtocols(protocols []Protocol) {
	for i := range protocols {
		for j := range protocols[i].FormSchema {
			setDefaultFieldGroup(&protocols[i].FormSchema[j])
		}
		switch protocols[i].Protocol {
		case "vless":
			enrichVLESS(&protocols[i])
		case "vmess":
			enrichVMess(&protocols[i])
		case "trojan":
			enrichTrojan(&protocols[i])
		case "ss":
			enrichSS(&protocols[i])
		}
		organizeFirstBatchForm(&protocols[i])
		setCanonicalPaths(protocols[i].FormSchema, "")
		setScalarFeatures(protocols[i].FormSchema)
	}
}

// organizeFirstBatchForm 只调整首批协议的展示顺序与层次，不改变规范路径和存储结构。
func organizeFirstBatchForm(p *Protocol) {
	var order []string
	switch p.Protocol {
	case "vless":
		order = []string{"uuid", "network", "security", "flow", "ws-opts", "grpc-opts", "h2-opts", "http-opts", "xhttp-opts", "servername", "client-fingerprint", "reality-opts", "alpn", "fingerprint"}
	case "vmess":
		order = []string{"uuid", "network", "security", "cipher", "ws-opts", "grpc-opts", "h2-opts", "http-opts", "servername", "client-fingerprint", "alpn", "fingerprint"}
	case "trojan":
		order = []string{"password", "network", "ws-opts", "grpc-opts", "sni", "client-fingerprint", "alpn", "fingerprint"}
	case "ss":
		order = []string{"password", "cipher", "plugin", "plugin-opts", "obfs-opts", "v2ray-plugin-opts", "shadow-tls-opts", "restls-opts", "client-fingerprint"}
		updateField(p.FormSchema, "client-fingerprint", func(field *FieldSchema) {
			setPluginCondition(field, "shadow-tls", "restls")
		})
	default:
		return
	}
	for _, parent := range []string{"ws-opts", "v2ray-plugin-opts"} {
		for _, name := range []string{"max-early-data", "early-data-header-name", "v2ray-http-upgrade", "v2ray-http-upgrade-fast-open", "mux", "version"} {
			updateNestedField(p.FormSchema, parent, name, func(field *FieldSchema) { field.Advanced = true })
		}
	}
	updateNestedField(p.FormSchema, "restls-opts", "restls-script", func(field *FieldSchema) { field.Advanced = true })
	// 兼容/性能开关集中进“更多开关”，其余参数仍留在原结构化区域。
	var classify func([]FieldSchema, bool)
	classify = func(fields []FieldSchema, inheritedAdvanced bool) {
		for i := range fields {
			field := &fields[i]
			advanced := inheritedAdvanced || field.Advanced || field.Group == "advanced"
			if field.Type == "bool" {
				field.Group = "switches"
				field.Advanced = advanced
			}
			classify(field.Properties, advanced)
		}
	}
	classify(p.FormSchema, false)
	ordered := make([]FieldSchema, 0, len(p.FormSchema))
	seen := make(map[string]bool, len(order))
	for _, name := range order {
		for _, field := range p.FormSchema {
			if field.Name == name {
				ordered = append(ordered, field)
				seen[name] = true
				break
			}
		}
	}
	for _, field := range p.FormSchema {
		if !seen[field.Name] {
			ordered = append(ordered, field)
		}
	}
	p.FormSchema = ordered
}

func setDefaultFieldGroup(field *FieldSchema) {
	if field.Group != "" {
		return
	}
	switch field.Section {
	case "auth":
		field.Group = "auth"
	case "transport", "security":
		field.Group = "connection"
	case "switches":
		field.Group = "switches"
	default:
		field.Group = "advanced"
	}
	for i := range field.Properties {
		setDefaultFieldGroup(&field.Properties[i])
	}
}

func setCanonicalPaths(fields []FieldSchema, prefix string) {
	for i := range fields {
		path := fields[i].Name
		if prefix != "" {
			path = prefix + "." + path
		}
		if fields[i].CanonicalPath == "" {
			fields[i].CanonicalPath = path
		}
		setCanonicalPaths(fields[i].Properties, path)
	}
}

func updateField(fields []FieldSchema, name string, fn func(*FieldSchema)) bool {
	for i := range fields {
		if fields[i].Name == name {
			fn(&fields[i])
			return true
		}
	}
	return false
}

func updateNestedField(fields []FieldSchema, parent, child string, fn func(*FieldSchema)) bool {
	for i := range fields {
		if fields[i].Name == parent {
			return updateField(fields[i].Properties, child, fn)
		}
	}
	return false
}

func appendField(fields []FieldSchema, field FieldSchema) []FieldSchema {
	if updateField(fields, field.Name, func(existing *FieldSchema) { *existing = field }) {
		return fields
	}
	return append(fields, field)
}

func insertField(fields []FieldSchema, field FieldSchema, before string) []FieldSchema {
	for i := range fields {
		if fields[i].Name == before {
			next := make([]FieldSchema, 0, len(fields)+1)
			next = append(next, fields[:i]...)
			next = append(next, field)
			next = append(next, fields[i:]...)
			return next
		}
	}
	return append(fields, field)
}

func option(value, label, group, verified string) OptionItem {
	return OptionItem{Value: value, Label: label, Group: group, Verified: verified}
}

func setOptionItems(field *FieldSchema, items ...OptionItem) {
	field.OptionItems = append([]OptionItem(nil), items...)
	field.Options = make([]string, 0, len(items))
	for _, item := range items {
		field.Options = append(field.Options, item.Value)
	}
}

func setNetworkCondition(field *FieldSchema, network ...string) {
	field.When = &ConditionRule{Network: append([]string(nil), network...)}
	field.ResetOn = []string{"network"}
	field.Group = "connection"
}

func setSecurityCondition(field *FieldSchema, security ...string) {
	field.When = &ConditionRule{Security: append([]string(nil), security...)}
	field.ResetOn = []string{"security"}
	field.Group = "connection"
}

func setPluginCondition(field *FieldSchema, plugins ...string) {
	field.When = &ConditionRule{Plugin: append([]string(nil), plugins...)}
	field.ResetOn = []string{"plugin"}
	field.Group = "connection"
}

func setTargetEvidence(field *FieldSchema, evidence ...TargetEvidence) {
	field.TargetEvidence = append([]TargetEvidence(nil), evidence...)
}

func enrichVLESS(p *Protocol) {
	setField := func(name string, fn func(*FieldSchema)) { updateField(p.FormSchema, name, fn) }
	setField("network", func(field *FieldSchema) {
		setOptionItems(field,
			option("tcp", "TCP", "common", "mihomo-1.19.29"),
			option("ws", "WebSocket", "common", "mihomo-1.19.29"),
			option("grpc", "gRPC", "common", "mihomo-1.19.29"),
			option("h2", "HTTP/2", "extended", "mihomo-1.19.29"),
			option("http", "HTTP", "extended", "mihomo-1.19.29"),
			option("xhttp", "XHTTP", "extended", "mihomo-1.19.29"))
		field.AllowCustom = boolPtr(true)
		field.CanonicalPath = "network"
		field.Group = "connection"
	})
	setField("tls", func(field *FieldSchema) {
		field.Group = "advanced"
		field.Help = "兼容输入；新表单优先使用 security。"
		field.When = &ConditionRule{Security: []string{"tls", "reality"}}
		field.ResetOn = []string{"security"}
		field.CanonicalPath = "security"
		field.Aliases = []string{"tls"}
	})
	security := sel("security", "安全", "none", "none", "tls", "reality")
	security.AllowCustom = boolPtr(false)
	security.Group = "connection"
	security.CanonicalPath = "security"
	security.Aliases = []string{"tls"}
	setOptionItems(&security,
		option("none", "无", "common", "mihomo-1.19.29"),
		option("tls", "TLS", "common", "mihomo-1.19.29"),
		option("reality", "REALITY", "extended", "mihomo-1.19.29"))
	p.FormSchema = insertField(p.FormSchema, security, "servername")

	for _, item := range []struct {
		name    string
		network string
	}{
		{"ws-opts", "ws"}, {"grpc-opts", "grpc"}, {"h2-opts", "h2"},
		{"http-opts", "http"}, {"xhttp-opts", "xhttp"},
	} {
		setField(item.name, func(field *FieldSchema) { setNetworkCondition(field, item.network) })
	}
	setField("reality-opts", func(field *FieldSchema) {
		setSecurityCondition(field, "reality")
		updateNestedField(p.FormSchema, "reality-opts", "public-key", func(child *FieldSchema) {
			child.RequiredWhen = &ConditionRule{Security: []string{"reality"}, Targets: []string{"clash-yaml"}}
		})
		updateNestedField(p.FormSchema, "reality-opts", "short-id", func(child *FieldSchema) {
			child.RequiredWhen = &ConditionRule{Security: []string{"reality"}, Targets: []string{"clash-yaml"}}
		})
	})
	setField("flow", func(field *FieldSchema) {
		field.When = &ConditionRule{Network: []string{"tcp"}, Security: []string{"tls", "reality"}}
		field.ResetOn = []string{"network", "security"}
		field.Group = "connection"
		field.AllowCustom = boolPtr(true)
		setOptionItems(field, option("xtls-rprx-vision", "Vision", "common", "mihomo-1.19.29"))
	})
	for _, name := range []string{"servername", "alpn", "client-fingerprint"} {
		setField(name, func(field *FieldSchema) { setSecurityCondition(field, "tls", "reality") })
	}
	for _, name := range []string{"skip-cert-verify", "fingerprint"} {
		setField(name, func(field *FieldSchema) { setSecurityCondition(field, "tls") })
	}
	setField("alpn", func(field *FieldSchema) {
		field.AllowCustom = boolPtr(true)
		setOptionItems(field,
			option("h2", "h2", "common", "mihomo-1.19.29"),
			option("http/1.1", "http/1.1", "common", "mihomo-1.19.29"))
	})
	setField("client-fingerprint", func(field *FieldSchema) {
		field.AllowCustom = boolPtr(true)
		setOptionItems(field,
			option("chrome", "Chrome", "common", "mihomo-1.19.29"),
			option("firefox", "Firefox", "common", "mihomo-1.19.29"),
			option("safari", "Safari", "common", "mihomo-1.19.29"),
			option("iOS", "iOS", "common", "mihomo-1.19.29"),
			option("android", "Android", "common", "mihomo-1.19.29"),
			option("edge", "Edge", "extended", "mihomo-1.19.29"),
			option("random", "Random", "extended", "mihomo-1.19.29"))
	})
	setField("xhttp-opts", func(field *FieldSchema) {
		updateField(field.Properties, "mode", func(mode *FieldSchema) {
			mode.Default = nil
			mode.AllowCustom = boolPtr(true)
			setOptionItems(mode,
				option("auto", "Auto", "common", "mihomo-1.19.29"),
				option("stream-one", "Stream One", "common", "mihomo-1.19.29"),
				option("stream-up", "Stream Up", "common", "mihomo-1.19.29"),
				option("packet-up", "Packet Up", "extended", "mihomo-1.19.29"))
		})
	})
	setField("encryption", func(field *FieldSchema) {
		field.AllowCustom = boolPtr(true)
		setOptionItems(field, option("none", "none", "common", "mihomo-1.19.29"))
		setTargetEvidence(field,
			TargetEvidence{Target: "generic-subs", Client: "project adapter", Version: "current", Entry: "vless-uri", Status: "complete"},
			TargetEvidence{Target: "sr-subs", Client: "Shadowrocket", Version: "unverified", Entry: "vless-uri", Status: "partial"})
	})
	for _, name := range []string{"smux", "packet-encoding", "ws-path", "ws-headers"} {
		setField(name, func(field *FieldSchema) {
			field.Group = "advanced"
			field.Advanced = true
		})
	}
}

func enrichVMess(p *Protocol) {
	setField := func(name string, fn func(*FieldSchema)) { updateField(p.FormSchema, name, fn) }
	setField("cipher", func(field *FieldSchema) {
		field.AllowCustom = boolPtr(true)
		setOptionItems(field,
			option("auto", "Auto", "common", "mihomo-1.19.29"),
			option("aes-128-gcm", "AES-128-GCM", "common", "mihomo-1.19.29"),
			option("chacha20-poly1305", "ChaCha20-Poly1305", "common", "mihomo-1.19.29"),
			option("none", "None", "legacy", "mihomo-1.19.29"),
			option("zero", "Zero", "legacy", "mihomo-1.19.29"))
		setTargetEvidence(field,
			TargetEvidence{Target: "sr-subs", Client: "Clash Verge Rev", Version: "2.5.2", Entry: "vmess-uri", Status: "partial"},
			TargetEvidence{Target: "generic-subs", Client: "project adapter", Version: "current", Entry: "vmess-uri", Status: "complete"})
	})
	setField("network", func(field *FieldSchema) {
		setOptionItems(field,
			option("tcp", "TCP", "common", "mihomo-1.19.29"), option("ws", "WebSocket", "common", "mihomo-1.19.29"),
			option("grpc", "gRPC", "common", "mihomo-1.19.29"), option("h2", "HTTP/2", "extended", "mihomo-1.19.29"),
			option("http", "HTTP", "extended", "mihomo-1.19.29"))
		field.AllowCustom = boolPtr(true)
		field.Group = "connection"
	})
	security := sel("security", "安全", "none", "none", "tls")
	security.AllowCustom = boolPtr(false)
	security.Group = "connection"
	setOptionItems(&security, option("none", "无", "common", "mihomo-1.19.29"), option("tls", "TLS", "common", "mihomo-1.19.29"))
	p.FormSchema = insertField(p.FormSchema, security, "servername")
	setField("tls", func(field *FieldSchema) {
		field.Group = "advanced"
		field.Help = "兼容输入；新表单优先使用 security。"
		field.When = &ConditionRule{Security: []string{"tls"}}
		field.ResetOn = []string{"security"}
		field.CanonicalPath = "security"
		field.Aliases = []string{"tls"}
	})
	for _, item := range []struct {
		name    string
		network string
	}{
		{"ws-opts", "ws"}, {"grpc-opts", "grpc"}, {"h2-opts", "h2"}, {"http-opts", "http"},
	} {
		setField(item.name, func(field *FieldSchema) { setNetworkCondition(field, item.network) })
	}
	setField("reality-opts", func(field *FieldSchema) {
		field.Group = "advanced"
		field.Help = "VMess REALITY 仅作为后续候选，首批不开放表单入口。"
		field.When = &ConditionRule{Security: []string{"reality"}}
		field.ResetOn = []string{"security"}
		setTargetEvidence(field, TargetEvidence{Target: "clash-yaml", Client: "Mihomo", Version: "1.19.29", Entry: "vmess-reality", Status: "unverified"})
	})
	for _, name := range []string{"servername", "alpn", "skip-cert-verify", "fingerprint", "client-fingerprint"} {
		setField(name, func(field *FieldSchema) {
			field.When = &ConditionRule{Security: []string{"tls"}}
			field.ResetOn = []string{"security"}
			field.Group = "connection"
		})
	}
	setField("alpn", func(field *FieldSchema) {
		field.AllowCustom = boolPtr(true)
		setOptionItems(field,
			option("h2", "h2", "common", "mihomo-1.19.29"),
			option("http/1.1", "http/1.1", "common", "mihomo-1.19.29"))
	})
	setField("client-fingerprint", func(field *FieldSchema) {
		field.AllowCustom = boolPtr(true)
		setOptionItems(field,
			option("chrome", "Chrome", "common", "mihomo-1.19.29"),
			option("firefox", "Firefox", "common", "mihomo-1.19.29"),
			option("safari", "Safari", "common", "mihomo-1.19.29"),
			option("iOS", "iOS", "common", "mihomo-1.19.29"),
			option("android", "Android", "common", "mihomo-1.19.29"),
			option("edge", "Edge", "extended", "mihomo-1.19.29"),
			option("random", "Random", "extended", "mihomo-1.19.29"))
	})
	setField("alterId", func(field *FieldSchema) {
		field.Help = "旧版/兼容参数；默认缺省与显式 0 需区分。"
		setTargetEvidence(field, TargetEvidence{Target: "sr-subs", Client: "Clash Verge Rev", Version: "2.5.2", Entry: "vmess-uri", Status: "partial"})
	})
	for _, name := range []string{"smux", "packet-encoding", "global-padding", "authenticated-length"} {
		setField(name, func(field *FieldSchema) {
			field.Group = "advanced"
			field.Advanced = true
		})
	}
}

func enrichTrojan(p *Protocol) {
	setField := func(name string, fn func(*FieldSchema)) { updateField(p.FormSchema, name, fn) }
	setField("network", func(field *FieldSchema) {
		setOptionItems(field,
			option("tcp", "TCP", "common", "mihomo-1.19.29"), option("ws", "WebSocket", "common", "mihomo-1.19.29"),
			option("grpc", "gRPC", "common", "mihomo-1.19.29"))
		field.AllowCustom = boolPtr(true)
		field.Group = "connection"
		field.Help = "h2/http/xhttp 可手填，但首批不作为普通组合。"
		setTargetEvidence(field,
			TargetEvidence{Target: "sr-subs", Client: "Clash Verge Rev", Version: "2.5.2", Entry: "trojan-network", Status: "partial"},
			TargetEvidence{Target: "generic-subs", Client: "project adapter", Version: "current", Entry: "trojan-network", Status: "partial"},
		)
	})
	setField("reality-opts", func(field *FieldSchema) {
		field.Group = "advanced"
		field.Help = "Trojan REALITY 当前仅作为后续候选，首批不开放表单入口。"
		field.When = &ConditionRule{Security: []string{"reality"}}
	})
	for _, item := range []struct {
		name    string
		network string
	}{
		{"ws-opts", "ws"}, {"grpc-opts", "grpc"},
	} {
		setField(item.name, func(field *FieldSchema) { setNetworkCondition(field, item.network) })
	}
	for _, name := range []string{"sni", "alpn", "skip-cert-verify", "fingerprint", "client-fingerprint"} {
		setField(name, func(field *FieldSchema) { setSecurityCondition(field, "tls") })
	}
	setField("alpn", func(field *FieldSchema) {
		field.AllowCustom = boolPtr(true)
		setOptionItems(field,
			option("h2", "h2", "common", "mihomo-1.19.29"),
			option("http/1.1", "http/1.1", "common", "mihomo-1.19.29"))
	})
	setField("client-fingerprint", func(field *FieldSchema) {
		field.AllowCustom = boolPtr(true)
		setOptionItems(field,
			option("chrome", "Chrome", "common", "mihomo-1.19.29"),
			option("firefox", "Firefox", "common", "mihomo-1.19.29"),
			option("safari", "Safari", "common", "mihomo-1.19.29"),
			option("iOS", "iOS", "common", "mihomo-1.19.29"),
			option("android", "Android", "common", "mihomo-1.19.29"),
			option("edge", "Edge", "extended", "mihomo-1.19.29"),
			option("random", "Random", "extended", "mihomo-1.19.29"))
	})
	setField("ss-opts", func(field *FieldSchema) {
		field.Group = "advanced"
		field.ResetOn = []string{"protocol"}
		updateNestedField(p.FormSchema, "ss-opts", "method", func(method *FieldSchema) {
			method.AllowCustom = boolPtr(true)
			method.Aliases = []string{"cipher"}
			setOptionItems(method,
				option("aes-128-gcm", "AES-128-GCM", "common", "mihomo-1.19.29"),
				option("aes-256-gcm", "AES-256-GCM", "common", "mihomo-1.19.29"),
				option("chacha20-ietf-poly1305", "ChaCha20-Poly1305", "common", "mihomo-1.19.29"))
		})
		updateNestedField(p.FormSchema, "ss-opts", "password", func(password *FieldSchema) {
			password.RequiredWhen = &ConditionRule{Targets: []string{"clash-yaml"}}
		})
	})
}

func enrichSS(p *Protocol) {
	setField := func(name string, fn func(*FieldSchema)) { updateField(p.FormSchema, name, fn) }
	setField("cipher", func(field *FieldSchema) {
		field.AllowCustom = boolPtr(true)
		setOptionItems(field,
			option("aes-128-gcm", "AES-128-GCM", "common", "mihomo-1.19.29"),
			option("aes-256-gcm", "AES-256-GCM", "common", "mihomo-1.19.29"),
			option("chacha20-ietf-poly1305", "ChaCha20-Poly1305", "common", "mihomo-1.19.29"),
			option("aes-192-gcm", "AES-192-GCM", "legacy", "mihomo-1.19.29"),
			option("xchacha20-ietf-poly1305", "XChaCha20-Poly1305", "legacy", "mihomo-1.19.29"),
			option("2022-blake3-aes-128-gcm", "SS 2022 AES-128", "pending", "project-unknown"),
			option("2022-blake3-aes-256-gcm", "SS 2022 AES-256", "pending", "project-unknown"),
			option("2022-blake3-chacha20-poly1305", "SS 2022 ChaCha20", "pending", "project-unknown"))
		setTargetEvidence(field,
			TargetEvidence{Target: "clash-yaml", Client: "Mihomo", Version: "1.19.29", Entry: "ss.cipher", Status: "complete"},
			TargetEvidence{Target: "sr-subs", Client: "Clash Verge Rev", Version: "2.5.2", Entry: "ss-uri", Status: "partial"})
	})
	setField("plugin", func(field *FieldSchema) {
		field.AllowCustom = boolPtr(true)
		field.ResetOn = []string{"plugin"}
		field.Group = "connection"
		setOptionItems(field,
			option("", "不使用插件", "common", "mihomo-1.19.29"), option("obfs", "obfs", "common", "mihomo-1.19.29"),
			option("v2ray-plugin", "v2ray-plugin", "common", "mihomo-1.19.29"), option("shadow-tls", "shadow-tls", "extended", "mihomo-1.19.29"),
			option("restls", "restls", "extended", "mihomo-1.19.29"))
	})
	setField("plugin-opts", func(field *FieldSchema) {
		excluded := append([]string{""}, ssplugin.KnownNames()...)
		field.When = &ConditionRule{PluginNot: excluded}
		field.ResetOn = []string{"plugin"}
		field.Group = "connection"
	})
	setField("obfs-opts", func(field *FieldSchema) {
		setPluginCondition(field, "obfs")
		setTargetEvidence(field, TargetEvidence{Target: "sr-subs", Client: "Clash Verge Rev", Version: "2.5.2", Entry: "ss-plugin-uri", Status: "partial"})
	})
	setField("v2ray-plugin-opts", func(field *FieldSchema) {
		setPluginCondition(field, "v2ray-plugin")
		setTargetEvidence(field, TargetEvidence{Target: "sr-subs", Client: "Clash Verge Rev", Version: "2.5.2", Entry: "ss-plugin-uri", Status: "partial"})
	})
	setField("shadow-tls-opts", func(field *FieldSchema) {
		setPluginCondition(field, "shadow-tls")
		setTargetEvidence(field, TargetEvidence{Target: "sr-subs", Client: "Clash Verge Rev", Version: "2.5.2", Entry: "ss-plugin-uri", Status: "partial"})
	})
	setField("restls-opts", func(field *FieldSchema) {
		setPluginCondition(field, "restls")
		setTargetEvidence(field, TargetEvidence{Target: "sr-subs", Client: "Clash Verge Rev", Version: "2.5.2", Entry: "ss-plugin-uri", Status: "partial"})
	})
	setField("client-fingerprint", func(field *FieldSchema) {
		field.Group = "connection"
		field.AllowCustom = boolPtr(true)
		setOptionItems(field,
			option("chrome", "Chrome", "common", "mihomo-1.19.29"),
			option("firefox", "Firefox", "common", "mihomo-1.19.29"),
			option("safari", "Safari", "common", "mihomo-1.19.29"),
			option("iOS", "iOS", "common", "mihomo-1.19.29"),
			option("android", "Android", "common", "mihomo-1.19.29"),
			option("edge", "Edge", "extended", "mihomo-1.19.29"),
			option("random", "Random", "extended", "mihomo-1.19.29"))
	})
	for _, name := range []string{"udp-over-tcp", "udp-over-tcp-version"} {
		setField(name, func(field *FieldSchema) {
			field.ResetOn = []string{"feature.udp-over-tcp"}
			if name == "udp-over-tcp-version" {
				field.When = &ConditionRule{Features: []string{"udp-over-tcp"}}
			}
		})
	}
	setField("smux", func(field *FieldSchema) {
		field.ResetOn = []string{"feature.smux"}
		field.Group = "advanced"
	})
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
