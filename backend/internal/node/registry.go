// Package node 提供统一节点表的协议注册表与业务能力。
package node

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

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
	AllowUnknown   bool             `json:"allow_unknown"`            // 显式白名单；固定对象必须下发 false，开放 Map 才为 true
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
	SelectorName   string           `json:"selector_name,omitempty"`
	StateOnly      bool             `json:"state_only,omitempty"`
	// ClearWhenInactive 声明「仅当该字段在新 selector 状态下不再活动时才清空」（Build32 Step 18 引入）。
	// 与无方向的 ResetOn 不同：多分支共享的字段（例如 OpenVPN 的 cert／key 同时属于 cert 与
	// cert_userpass）在自己仍然活动的分支切换中必须保留，只在离开全部分支时清空。
	// 声明该属性的字段必须同时声明 When.Selectors，否则注册期直接阻断。
	ClearWhenInactive bool `json:"clear_when_inactive,omitempty"`
}

type LinkMapping struct {
	SR      bool     `json:"sr"`
	Generic bool     `json:"generic"`
	Params  []string `json:"params,omitempty"`
}
type Protocol struct {
	Protocol         string           `json:"protocol"`
	Label            string           `json:"label"`
	FormSchema       []FieldSchema    `json:"form_schema"`
	Selectors        []SelectorSchema `json:"selectors,omitempty"`
	EndpointPolicies []EndpointPolicy `json:"endpoint_policies,omitempty"`
	SensitiveFields  []string         `json:"sensitive_fields"`
	LinkMappings     LinkMapping      `json:"link_mappings"`
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
	return v
}

// openMap 声明显式允许未知普通键的开放 Map；其余 object 默认拒绝未知键。
func openMap(name, label string) FieldSchema {
	v := obj(name, label, "map")
	v.AllowUnknown = true
	return v
}

func customPluginOpts() FieldSchema {
	field := openMap("plugin-opts", "自定义插件参数")
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
		f("path", "text", "路径"), openMap("headers", "请求头"), f("max-early-data", "number", "Early Data 上限"),
		f("early-data-header-name", "text", "Early Data Header"), def("v2ray-http-upgrade", "bool", "HTTP Upgrade", false),
		def("v2ray-http-upgrade-fast-open", "bool", "HTTP Upgrade Fast Open", false))
}

func httpOpts() FieldSchema {
	return obj("http-opts", "HTTP 参数", "fields",
		f("method", "text", "方法"), f("path", "text-list", "路径"), openMap("headers", "请求头"))
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
	setOptionItems(&mode, option("http", "HTTP", "common", "mihomo-1.19.31"), option("tls", "TLS", "common", "mihomo-1.19.31"))
	return obj("obfs-opts", "obfs 参数", "fields", mode, f("host", "text", "Host"))
}

func v2rayPluginOpts() FieldSchema {
	mode := f("mode", "select", "模式")
	mode.Default = "websocket"
	mode.AllowCustom = boolPtr(true)
	setOptionItems(&mode, option("websocket", "WebSocket", "common", "mihomo-1.19.31"))
	return obj("v2ray-plugin-opts", "v2ray-plugin 参数", "fields",
		mode, f("host", "text", "Host"), def("tls", "bool", "TLS", false), f("path", "text", "路径"), openMap("headers", "请求头"),
		obj("ech-opts", "ECH 参数", "fields", def("enable", "bool", "启用", false), f("config", "text", "配置"), f("query-server-name", "text", "查询服务器名称")),
		def("mux", "bool", "Mux", false), def("v2ray-http-upgrade", "bool", "HTTP Upgrade", false),
		def("v2ray-http-upgrade-fast-open", "bool", "HTTP Upgrade Fast Open", false), f("fingerprint", "text", "证书指纹"),
		f("certificate", "multiline", "客户端证书"), f("private-key", "secret-multiline", "客户端私钥"),
		def("skip-cert-verify", "bool", "跳过证书校验", false), f("name-cert-verify", "text", "证书名称校验"))
}

func shadowTlsOpts() FieldSchema {
	return obj("shadow-tls-opts", "shadow-tls 参数", "fields",
		f("password", "password", "密码"), f("host", "text", "Host"), f("version", "number", "版本"), f("alpn", "text-list", "ALPN"),
		f("fingerprint", "text", "证书指纹"), f("certificate", "multiline", "客户端证书"), f("private-key", "secret-multiline", "客户端私钥"),
		def("skip-cert-verify", "bool", "跳过证书校验", false), f("name-cert-verify", "text", "证书名称校验"))
}

func restlsOpts() FieldSchema {
	return obj("restls-opts", "restls 参数", "fields",
		f("password", "password", "密码"), f("host", "text", "Host"), f("version-hint", "text", "版本提示"),
		f("restls-script", "multiline", "Restls Script"), f("fingerprint", "text", "证书指纹"),
		def("skip-cert-verify", "bool", "跳过证书校验", false), f("name-cert-verify", "text", "证书名称校验"))
}

func ssOpts() FieldSchema {
	return obj("ss-opts", "内层 SS", "fields",
		def("enabled", "bool", "启用内层 SS", false), f("method", "text", "内层加密方式"),
		f("password", "password", "密码"))
}

// wireGuardPeerModeField 声明标准 WireGuard 的单 Peer／多 Peer selector；
// state_only 只保存在 current_state.selectors，不进入 protocol_json 与 wire。
func wireGuardPeerModeField() FieldSchema {
	field := sel("peer-mode", "Peer 模式", "single", "single", "peers")
	field.StateOnly = true
	field.SelectorName = "peer_mode"
	field.Group = "connection"
	field.Help = "single 使用顶层服务器与公钥；peers 使用结构化 Peer 列表并隐藏顶层 endpoint。"
	return field
}

// wireGuardModeField 声明只在指定 Peer 模式活动、切换 selector 即清空的字段。
// required 表示该字段在其活动分支内条件必填。
func wireGuardModeField(field FieldSchema, modes []string, required bool) FieldSchema {
	condition := &ConditionRule{Selectors: map[string][]string{"peer_mode": modes}}
	field.When = condition
	if required {
		field.RequiredWhen = condition
	}
	field.ResetOn = []string{"selector.peer_mode"}
	return field
}

// wireGuardPeersField 声明多 Peer 结构化列表；每项以 _credential_id 作为稳定凭据身份，
// 使 Peer 重排／删除／替换只影响对应 PSK（R27-07 数组凭据合同）。
func wireGuardPeersField() FieldSchema {
	peers := obj("peers", "Peer 列表", "list",
		req("server", "text", "服务器"), req("port", "number", "端口"), req("public-key", "text", "公钥"),
		f("pre-shared-key", "password", "预共享密钥"), f("reserved", "byte-sequence", "保留字节"),
		req("allowed-ips", "text-list", "Allowed IPs"))
	peers.ItemIDField = sensitiveItemIDField
	condition := &ConditionRule{Selectors: map[string][]string{"peer_mode": {"peers"}}}
	peers.When = condition
	peers.RequiredWhen = condition
	peers.ResetOn = []string{"selector.peer_mode"}
	peers.Group = "connection"
	return peers
}

// ipStackModeField 声明固定 tag IPStackOption.mode；空值表示内核默认 auto（WireGuard／MASQUE 共用）。
func ipStackModeField() FieldSchema {
	field := sel("mode", "IP 栈", "", "", "auto", "gvisor", "mips")
	field.Group = "advanced"
	return field
}

// ipStackControllerField 声明固定 tag IPStackOption.congestion-controller；空值为内核默认（共用）。
func ipStackControllerField() FieldSchema {
	field := sel("congestion-controller", "拥塞控制器", "", "", "cubic", "reno", "bbr", "bbr3")
	field.Group = "advanced"
	return field
}

// ipStackField 声明固定 tag 的 ip-stack 对象（WireGuard／MASQUE 共用枚举合同）。
func ipStackField() FieldSchema {
	field := obj("ip-stack", "IP 栈参数", "fields", ipStackModeField(), ipStackControllerField())
	field.Group = "advanced"
	return field
}

// dnsListField 声明仅在 remote-dns-resolve 开启时活动、条件必填的 DNS 列表（WireGuard／MASQUE 共用）。
// 关闭开关时的清空与 When 由 setScalarFeatures 统一挂接 feature.remote-dns-resolve。
func dnsListField() FieldSchema {
	field := f("dns", "text-list", "DNS")
	field.RequiredWhen = &ConditionRule{Features: []string{"remote-dns-resolve"}}
	field.Group = "advanced"
	return field
}

// mieruEndpointModeField 声明单端口／端口段 selector；range 模式下顶层 port 由 endpoint policy 隐藏。
func mieruEndpointModeField() FieldSchema {
	field := sel("endpoint-mode", "端口模式", "single", "single", "range")
	field.StateOnly = true
	field.SelectorName = "endpoint_mode"
	field.Group = "connection"
	field.Help = "single 使用顶层端口；range 使用单个 begin-end 端口段并隐藏顶层端口。"
	return field
}

// mieruPortRangeField 声明仅在 range 模式活动且必填的端口段；single 模式清空。
func mieruPortRangeField() FieldSchema {
	field := f("port-range", "text", "端口范围")
	condition := &ConditionRule{Selectors: map[string][]string{"endpoint_mode": {"range"}}}
	field.When = condition
	field.RequiredWhen = condition
	field.ResetOn = []string{"selector.endpoint_mode"}
	field.Group = "connection"
	field.Help = "只接受单个 begin-end，两端为 1-65535 且 begin ≤ end。"
	return field
}

// mieruTransportField 声明固定 tag 精确匹配的传输枚举；不做大小写或别名转换。
func mieruTransportField() FieldSchema {
	field := sel("transport", "传输", "TCP", "TCP", "UDP")
	field.Group = "connection"
	return field
}

// mieruMultiplexingField 声明固定 tag 的完整多路复用常量名；空值表示内核默认，不强写默认值。
func mieruMultiplexingField() FieldSchema {
	field := sel("multiplexing", "多路复用", "",
		"", "MULTIPLEXING_OFF", "MULTIPLEXING_LOW", "MULTIPLEXING_MIDDLE", "MULTIPLEXING_HIGH")
	field.Group = "connection"
	return field
}

// mieruHandshakeModeField 声明固定 tag 的完整握手模式常量名；空值表示内核默认。
func mieruHandshakeModeField() FieldSchema {
	field := sel("handshake-mode", "握手模式", "", "", "HANDSHAKE_STANDARD", "HANDSHAKE_NO_WAIT")
	field.Group = "connection"
	return field
}

// mieruTrafficPatternField 声明 Base64 流量特征；语义由固定 tag 校验，错误不回显原值。
func mieruTrafficPatternField() FieldSchema {
	field := f("traffic-pattern", "text", "流量特征")
	field.Group = "advanced"
	field.Help = "固定 tag 的 Base64 编码 TrafficPattern；错误只定位到本字段，不回显原值。"
	return field
}

// shadowQUICVersionsField 声明 QUIC 版本有序去重列表；空列表表示内核默认，不强写默认值。
func shadowQUICVersionsField() FieldSchema {
	field := f("quic-versions", "text-list", "QUIC 版本")
	field.Help = "只接受固定 tag parser 支持的 v1／v2；空列表使用内核默认顺序。"
	return field
}

// anytlsSecurityModeField 声明附加伪装 selector；state_only 只保存在 current_state.selectors。
func anytlsSecurityModeField() FieldSchema {
	field := sel("security-mode", "附加安全", "plain", "plain", "shadow_tls", "restls", "jls")
	field.StateOnly = true
	field.SelectorName = "security_mode"
	field.Group = "connection"
	field.Help = "plain 只使用 TLS；其余三种伪装对象严格互斥，切换即清空旧分支凭据。"
	return field
}

// anytlsCamouflageField 声明只在指定 security_mode 活动、切换即清空的伪装字段。
func anytlsCamouflageField(field FieldSchema, modes []string, required bool) FieldSchema {
	condition := &ConditionRule{Selectors: map[string][]string{"security_mode": modes}}
	field.When = condition
	if required {
		field.RequiredWhen = condition
	}
	field.ResetOn = []string{"selector.security_mode"}
	return field
}

// anytlsCamouflageObject 组装固定伪装对象：对象本身也按 selector 活动，非当前分支整块清空。
func anytlsCamouflageObject(name, label string, mode string, properties ...FieldSchema) FieldSchema {
	field := obj(name, label, "fields", properties...)
	condition := &ConditionRule{Selectors: map[string][]string{"security_mode": {mode}}}
	field.When = condition
	field.RequiredWhen = condition
	field.ResetOn = []string{"selector.security_mode"}
	field.Group = "connection"
	return field
}

// anytlsShadowTLSOptsField 声明 ShadowTLS 参数：固定 tag 的 ShadowTLSOptions 只有 password 与 version。
func anytlsShadowTLSOptsField() FieldSchema {
	version := sel("version", "版本", "", "", "1", "2", "3")
	return anytlsCamouflageObject("shadow-tls-opts", "ShadowTLS 参数", "shadow_tls",
		anytlsCamouflageField(f("password", "password", "密码"), []string{"shadow_tls"}, true),
		anytlsCamouflageField(version, []string{"shadow_tls"}, false))
}

// anytlsRestlsOptsField 声明 Restls 参数：固定 tag 的 version-hint 只接受 tls12／tls13。
func anytlsRestlsOptsField() FieldSchema {
	versionHint := sel("version-hint", "版本提示", "", "", "tls12", "tls13")
	return anytlsCamouflageObject("restls-opts", "Restls 参数", "restls",
		anytlsCamouflageField(f("password", "password", "密码"), []string{"restls"}, true),
		anytlsCamouflageField(versionHint, []string{"restls"}, true),
		anytlsCamouflageField(f("restls-script", "secret-multiline", "Restls Script"), []string{"restls"}, false))
}

// anytlsJLSOptsField 声明 JLS 参数：固定 tag 的 jls.NewConfig 要求用户名与密码同时非空。
func anytlsJLSOptsField() FieldSchema {
	return anytlsCamouflageObject("jls-opts", "JLS 参数", "jls",
		anytlsCamouflageField(f("username", "text", "用户名"), []string{"jls"}, true),
		anytlsCamouflageField(f("password", "password", "密码"), []string{"jls"}, true))
}

// trustTunnelReuseModeField 声明 TrustTunnel 连接复用 selector；state_only 只保存在 current_state.selectors。
// 固定 tag 只用优先级处理三个复用数字（max-connections 优先、max-streams 其次），不做互斥校验；
// 项目按 Build32 用 selector 表达互斥模式，并保证两组参数不得同时落库。
func trustTunnelReuseModeField() FieldSchema {
	field := sel("reuse-mode", "连接复用", "none", "none", "connections", "streams")
	field.StateOnly = true
	field.SelectorName = "reuse_mode"
	field.Group = "connection"
	field.Help = "none 不启用复用；connections 复用连接并要求最大连接数；streams 按最大流数复用。两组参数互斥。"
	return field
}

// trustTunnelReuseNumber 声明只在指定复用分支活动、且切换 selector 即清空的复用数字。
func trustTunnelReuseNumber(name, label, mode string, required bool) FieldSchema {
	field := f(name, "number", label)
	condition := &ConditionRule{Selectors: map[string][]string{"reuse_mode": {mode}}}
	field.When = condition
	if required {
		field.RequiredWhen = condition
	}
	field.ResetOn = []string{"selector.reuse_mode"}
	field.Group = "connection"
	return field
}

// trustTunnelQUICField 声明独立 QUIC 开关：关闭时固定 tag 走 HTTP/2 隧道（ALPN 需含 h2），
// 开启时走 HTTP/3（ALPN 需含 h3）。开关本身由 setScalarFeatures 注册为 quic 功能。
func trustTunnelQUICField() FieldSchema {
	field := def("quic", "bool", "QUIC (HTTP/3)", false)
	field.Group = "connection"
	field.Help = "关闭使用 HTTP/2 隧道（ALPN 需含 h2）；开启使用 HTTP/3（ALPN 需含 h3）。"
	return field
}

// trustTunnelQUICTuningField 声明只在 quic 开启时活动、关闭即清空的 QUIC 调优字段。
// 固定 tag 在非 QUIC 分支不读取这三个字段，因此关闭时必须清空且禁止输出。
func trustTunnelQUICTuningField(field FieldSchema) FieldSchema {
	field.When = &ConditionRule{Features: []string{"quic"}}
	field.ResetOn = []string{"feature.quic"}
	return field
}

// openvpnAuthModeField 声明 OpenVPN 三种认证模式 selector；state_only 只保存在 current_state.selectors。
func openvpnAuthModeField() FieldSchema {
	field := sel("auth-mode", "认证方式", "userpass", "userpass", "cert", "cert_userpass")
	field.StateOnly = true
	field.SelectorName = "auth_mode"
	field.Group = "auth"
	field.Help = "userpass 使用用户名与密码；cert 使用客户端证书与私钥；cert_userpass 两者同时使用。"
	return field
}

// openvpnTLSKeyModeField 声明 TLS key 三选一 selector；state_only 只保存在 current_state.selectors。
func openvpnTLSKeyModeField() FieldSchema {
	field := sel("tls-key-mode", "TLS Key 模式", "none", "none", "tls_auth", "tls_crypt", "tls_crypt_v2")
	field.StateOnly = true
	field.SelectorName = "tls_key_mode"
	field.Group = "auth"
	field.Help = "固定 tag 的 tls-auth、tls-crypt 与 tls-crypt-v2 严格互斥。"
	return field
}

// openvpnAuthField 声明认证凭据字段。cert／key 同时属于 cert 与 cert_userpass 两个分支，
// 因此只声明 clear_when_inactive 而**不**声明无方向 reset_on：
// 离开自己仍然活动的分支时凭据必须保留，只有离开全部分支才清空（Build32 Step 18）。
func openvpnAuthField(name, typ, label string, modes []string, required bool) FieldSchema {
	field := f(name, typ, label)
	condition := &ConditionRule{Selectors: map[string][]string{"auth_mode": modes}}
	field.When = condition
	if required {
		field.RequiredWhen = condition
	}
	field.ClearWhenInactive = true
	field.Group = "auth"
	return field
}

// openvpnTLSKeyField 声明只在指定 tls_key_mode 分支活动且条件必填的 TLS key。
// 三个 key 各自只属于一个分支，因此可以使用无方向 reset_on。
func openvpnTLSKeyField(name, label, mode string) FieldSchema {
	field := f(name, "secret-multiline", label)
	condition := &ConditionRule{Selectors: map[string][]string{"tls_key_mode": {mode}}}
	field.When = condition
	field.RequiredWhen = condition
	field.ResetOn = []string{"selector.tls_key_mode"}
	field.Group = "auth"
	return field
}

// openvpnKeyDirectionField 声明只在 tls_auth 分支活动的方向选择；固定 tag 只接受 0／1／空。
func openvpnKeyDirectionField() FieldSchema {
	field := sel("key-direction", "Key Direction", "", "", "0", "1")
	field.When = &ConditionRule{Selectors: map[string][]string{"tls_key_mode": {"tls_auth"}}}
	field.ClearWhenInactive = true
	field.Group = "auth"
	return field
}

// openvpnCipherValues 是固定 tag 的 `normalizeCipher` 与 `ValidateInstallScriptSubset` 共同认可的 cipher 集合。
// 项目按 Build32 与用户确认把 `data-ciphers`／`data-ciphers-fallback` 收紧到同一集合（固定 tag 本身不校验）。
var openvpnCipherValues = []string{
	"AES-128-GCM", "AES-192-GCM", "AES-256-GCM",
	"AES-128-CBC", "AES-192-CBC", "AES-256-CBC", "CHACHA20-POLY1305",
}

// openvpnCipherField 声明 cipher 枚举；空值由固定 tag 归一化为 AES-128-GCM，这里直接给出规范默认值。
func openvpnCipherField() FieldSchema {
	field := sel("cipher", "加密方式", "AES-128-GCM", openvpnCipherValues...)
	field.Group = "connection"
	return field
}

// openvpnFallbackCipherField 声明回退 cipher；空值表示不写 wire，由内核决定。
func openvpnFallbackCipherField() FieldSchema {
	field := sel("data-ciphers-fallback", "回退加密方式", "", append([]string{""}, openvpnCipherValues...)...)
	field.Group = "advanced"
	return field
}

// openvpnDataCiphersField 声明协商 cipher 列表；逐项必须落在固定 tag 的 cipher 集合内。
func openvpnDataCiphersField() FieldSchema {
	field := f("data-ciphers", "text-list", "协商加密列表")
	field.Group = "advanced"
	return field
}

// openvpnAuthDigestField 声明 auth 摘要枚举；空值由固定 tag 归一化为 SHA256。
func openvpnAuthDigestField() FieldSchema {
	field := sel("auth", "认证摘要", "SHA256", "MD5", "SHA1", "SHA256", "SHA384", "SHA512")
	field.Group = "connection"
	return field
}

// openvpnCompLZOField 声明 comp-lzo 枚举。固定 tag 只把 yes／adaptive 归一化为 yes、对其它值不报错，
// 项目按用户确认收紧为 yes／no／adaptive／空，避免写出无法验证的值。
func openvpnCompLZOField() FieldSchema {
	field := sel("comp-lzo", "Comp-LZO", "", "", "yes", "no", "adaptive")
	field.Group = "advanced"
	return field
}

// masqueNetworkModeField 声明 QUIC／h2／h3-l4proxy selector；wire 的 network 由 adapter 注入。
func masqueNetworkModeField() FieldSchema {
	field := sel("network-mode", "网络模式", "quic", "quic", "h2", "h3_l4proxy")
	field.StateOnly = true
	field.SelectorName = "network_mode"
	field.Group = "connection"
	field.Help = "quic 使用内核默认 QUIC；h2 使用 h2c；h3_l4proxy 使用 L3／L4 代理并强制关闭 UDP。"
	return field
}

// masqueModeField 声明只在指定 network_mode 活动、切换 selector 即清空的字段。
func masqueModeField(field FieldSchema, modes []string, required bool) FieldSchema {
	condition := &ConditionRule{Selectors: map[string][]string{"network_mode": modes}}
	field.When = condition
	if required {
		field.RequiredWhen = condition
	}
	field.ResetOn = []string{"selector.network_mode"}
	return field
}

// tailscaleLANAccessField 声明只在 exit-node 非空时活动的三态 LAN 访问开关。
// non_empty 是 ConditionRule 的第八个维度，由 schema 驱动显示与清空，前端不另写依赖表。
func tailscaleLANAccessField() FieldSchema {
	field := f("exit-node-allow-lan-access", "bool", "出口节点允许 LAN 访问")
	field.When = &ConditionRule{NonEmpty: []string{"exit-node"}}
	field.Group = "switches"
	field.Help = "只有填写出口节点后才生效；未设置表示沿用 Tailscale 默认。"
	return field
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

// basicAuthModeField 声明 none/basic 认证模式 selector；state_only 只保存在 current_state.selectors。
func basicAuthModeField() FieldSchema {
	field := sel("auth-mode", "认证方式", "none", "none", "basic")
	field.StateOnly = true
	field.SelectorName = "auth_mode"
	field.Group = "auth"
	field.Help = "none 不使用代理认证；basic 使用用户名与密码。"
	return field
}

// basicAuthCredential 声明仅在 basic 认证下活动且成对必填的用户名／密码字段（HTTP／SOCKS5 共用）。
func basicAuthCredential(name, typ, label string) FieldSchema {
	field := f(name, typ, label)
	field.When = &ConditionRule{Selectors: map[string][]string{"auth_mode": {"basic"}}}
	field.RequiredWhen = &ConditionRule{Selectors: map[string][]string{"auth_mode": {"basic"}}}
	field.ResetOn = []string{"selector.auth_mode"}
	field.Group = "auth"
	return field
}

// sshAuthModeField 声明 SSH 的密码／私钥认证 selector；state_only 只保存在 current_state.selectors。
func sshAuthModeField() FieldSchema {
	field := sel("auth-mode", "认证方式", "password", "password", "private_key")
	field.StateOnly = true
	field.SelectorName = "auth_mode"
	field.Group = "auth"
	field.Help = "password 使用密码认证；private_key 使用 PEM 私钥（可带私钥口令）。"
	return field
}

// sshBranchCredential 声明只在指定 SSH 认证分支活动、切换 selector 即清空的凭据字段。
func sshBranchCredential(name, typ, label, branch string, required bool) FieldSchema {
	field := f(name, typ, label)
	condition := &ConditionRule{Selectors: map[string][]string{"auth_mode": {branch}}}
	field.When = condition
	if required {
		field.RequiredWhen = condition
	}
	field.ResetOn = []string{"selector.auth_mode"}
	field.Group = "auth"
	return field
}

// sshHostKeyField 声明结构化 Host Key 列表；为空时由 Clash 目标检查给出安全 warn。
func sshHostKeyField() FieldSchema {
	field := f("host-key", "text-list", "Host Key")
	field.Group = "security"
	field.Help = "每项一条 authorized-key；留空将接受任意服务器 Host Key（存在中间人风险）。"
	return field
}

// sshHostKeyAlgorithmsField 声明非空算法名列表；顺序保留、去空白去重由归一化保证。
func sshHostKeyAlgorithmsField() FieldSchema {
	field := f("host-key-algorithms", "text-list", "Host Key 算法")
	field.Group = "security"
	field.Help = "按用户顺序优先协商的 Host Key 算法名，例如 ssh-ed25519。"
	return field
}

// snellVersionField 声明 Snell 版本普通 selector 的来源字段；空值按 tag 默认 v1 归一化。
func snellVersionField() FieldSchema {
	field := sel("version", "版本", "1", "1", "2", "3", "4", "5")
	field.SelectorName = "version"
	field.Group = "connection"
	field.Help = "v1/v2 不支持 UDP；v2 固定启用 reuse；v5 由内核按 v4 客户端实现。"
	return field
}

// snellUDPField 声明仅在 v3 起可编辑的 UDP 开关；v1/v2 不活动并按清空域归零。
func snellUDPField() FieldSchema {
	field := def("udp", "bool", "UDP", false)
	field.When = &ConditionRule{Selectors: map[string][]string{"version": {"3", "4", "5"}}}
	field.ResetOn = []string{"selector.version"}
	field.Group = "switches"
	return field
}

// snellReuseField 声明仅在 v4/v5 可编辑的连接复用开关；v2 由 adapter 固定启用。
func snellReuseField() FieldSchema {
	field := def("reuse", "bool", "连接复用", false)
	field.When = &ConditionRule{Selectors: map[string][]string{"version": {"4", "5"}}}
	field.ResetOn = []string{"selector.version"}
	field.Group = "connection"
	return field
}

// snellObfsModeField 声明 state_only 混淆模式 selector；模式切换清空整个 obfs-opts 对象。
func snellObfsModeField() FieldSchema {
	field := sel("obfs-mode", "混淆模式", "none", "none", "http", "tls", "shadow_tls", "restls", "jls")
	field.StateOnly = true
	field.SelectorName = "obfs_mode"
	field.Group = "connection"
	field.Help = "none 不启用混淆；各模式只活动对应字段。"
	return field
}

// snellObfsField 声明只在指定混淆模式活动、切换即清空的嵌套字段。
// requiredModes 非空时，该字段在对应模式下条件必填。
func snellObfsField(field FieldSchema, activeModes, requiredModes []string) FieldSchema {
	field.When = &ConditionRule{Selectors: map[string][]string{"obfs_mode": activeModes}}
	if len(requiredModes) > 0 {
		field.RequiredWhen = &ConditionRule{Selectors: map[string][]string{"obfs_mode": requiredModes}}
	}
	field.ResetOn = []string{"selector.obfs_mode"}
	field.Group = "connection"
	return field
}

// snellObfsOptsField 声明结构化混淆参数；固定对象拒绝未知键，mode 由 selector 注入 wire。
func snellObfsOptsField() FieldSchema {
	hostModes := []string{"http", "tls", "shadow_tls", "restls", "jls"}
	credentialModes := []string{"shadow_tls", "restls", "jls"}
	field := obj("obfs-opts", "混淆参数", "fields",
		snellObfsField(f("host", "text", "Host"), hostModes, []string{"shadow_tls", "restls", "jls"}),
		snellObfsField(f("password", "password", "密码"), credentialModes, credentialModes),
		snellObfsField(f("version", "number", "ShadowTLS 版本"), []string{"shadow_tls"}, nil),
		snellObfsField(f("fingerprint", "text", "TLS 指纹"), []string{"shadow_tls", "restls"}, nil),
		snellObfsField(f("certificate", "multiline", "客户端证书"), []string{"shadow_tls"}, nil),
		snellObfsField(f("private-key", "secret-multiline", "客户端私钥"), []string{"shadow_tls"}, nil),
		snellObfsField(def("skip-cert-verify", "bool", "跳过证书校验", false), []string{"shadow_tls", "restls"}, nil),
		snellObfsField(f("name-cert-verify", "text", "证书名称校验"), []string{"shadow_tls", "restls"}, nil),
		snellObfsField(f("alpn", "text-list", "ALPN"), []string{"shadow_tls", "jls"}, nil),
		snellObfsField(f("version-hint", "text", "版本提示"), []string{"restls"}, []string{"restls"}),
		snellObfsField(f("restls-script", "secret-multiline", "Restls Script"), []string{"restls"}, nil),
		snellObfsField(def("force-tls12", "bool", "强制 TLS1.2", false), []string{"restls"}, nil),
		snellObfsField(f("username", "text", "用户名"), []string{"jls"}, []string{"jls"}),
	)
	field.When = &ConditionRule{Selectors: map[string][]string{"obfs_mode": hostModes}}
	field.ResetOn = []string{"selector.obfs_mode"}
	field.Group = "connection"
	return field
}

// snellClientFingerprintField 声明只在三类伪装分支活动的客户端指纹。
func snellClientFingerprintField() FieldSchema {
	field := f("client-fingerprint", "text", "客户端指纹")
	field.When = &ConditionRule{Selectors: map[string][]string{"obfs_mode": {"shadow_tls", "restls", "jls"}}}
	field.ResetOn = []string{"selector.obfs_mode"}
	field.Group = "connection"
	return field
}

// hysteriaAuthModeField 声明 Hysteria 三态认证 selector；state_only 只保存在 current_state.selectors。
func hysteriaAuthModeField() FieldSchema {
	field := sel("auth-mode", "认证方式", "none", "none", "base64", "string")
	field.StateOnly = true
	field.SelectorName = "auth_mode"
	field.Group = "auth"
	field.Help = "none 不使用认证；base64 使用 Base64 认证；string 使用认证字符串。"
	return field
}

// hysteriaAuthCredential 声明只在指定认证分支活动且必填的凭据字段。
func hysteriaAuthCredential(name, label, branch string) FieldSchema {
	field := f(name, "password", label)
	condition := &ConditionRule{Selectors: map[string][]string{"auth_mode": {branch}}}
	field.When = condition
	field.RequiredWhen = condition
	field.ResetOn = []string{"selector.auth_mode"}
	field.Group = "auth"
	return field
}

// hysteriaProtocolField 声明固定 tag 支持的伪装协议枚举；空值由 adapter 按 udp 处理。
func hysteriaProtocolField() FieldSchema {
	field := sel("protocol", "传输协议", "udp", "udp", "wechat-video", "faketcp")
	field.Group = "connection"
	return field
}

// echOptsField 声明 Mihomo 公共 ECH 参数；关闭 enable 时清空子字段（Hysteria／Hysteria2／TUIC／AnyTLS／TrustTunnel 共用）。
func echOptsField() FieldSchema {
	field := obj("ech-opts", "ECH 参数", "fields",
		def("enable", "bool", "启用", false),
		f("config", "text", "配置"),
		f("query-server-name", "text", "查询服务器名"))
	field.Feature = &FeatureSchema{Name: "ech", Toggle: "enable"}
	field.ResetOn = []string{"feature.ech"}
	for i := range field.Properties {
		if field.Properties[i].Name == "enable" {
			continue
		}
		field.Properties[i].When = &ConditionRule{Features: []string{"ech"}}
		field.Properties[i].ResetOn = []string{"feature.ech"}
	}
	field.Group = "connection"
	return field
}

// tuicAuthModeField 声明 TUIC v4／v5 认证 selector；state_only 只保存在 current_state.selectors。
func tuicAuthModeField() FieldSchema {
	field := sel("auth-mode", "认证方式", "v5", "v4", "v5")
	field.StateOnly = true
	field.SelectorName = "auth_mode"
	field.Group = "auth"
	field.Help = "v4 使用 Token；v5 使用 UUID 与密码。两组凭据互斥。"
	return field
}

// tuicAuthCredential 声明只在指定 TUIC 认证分支活动且必填的凭据字段。
func tuicAuthCredential(name, label, branch string) FieldSchema {
	field := f(name, "password", label)
	condition := &ConditionRule{Selectors: map[string][]string{"auth_mode": {branch}}}
	field.When = condition
	field.RequiredWhen = condition
	field.ResetOn = []string{"selector.auth_mode"}
	field.Group = "auth"
	return field
}

// tuicUDPRelayModeField 声明固定 tag 支持的 UDP 中继模式枚举。
func tuicUDPRelayModeField() FieldSchema {
	field := sel("udp-relay-mode", "UDP 中继模式", "quic", "quic", "native")
	field.Group = "connection"
	return field
}

// quicCongestionControllerField 声明固定 tag 实际处理的 QUIC 拥塞控制器；空值表示沿用内核默认行为
// （TUIC／MASQUE／ShadowQUIC 共用同一实现集合）。
func quicCongestionControllerField() FieldSchema {
	field := sel("congestion-controller", "拥塞控制器", "", "", "cubic", "new_reno", "bbr_meta_v1", "bbr_meta_v2", "bbr")
	field.Group = "advanced"
	return field
}

// tuicUDPOverStreamVersionField 声明仅在 UOT 开启时可选的版本；0 只作为兼容输入归一化。
func tuicUDPOverStreamVersionField() FieldSchema {
	field := sel("udp-over-stream-version", "UDP over Stream 版本", "1", "1", "2")
	field.Group = "connection"
	return field
}

// hysteria2EndpointModeField 声明端口替代 selector；ports 模式下顶层 port 由 endpoint policy 隐藏。
func hysteria2EndpointModeField() FieldSchema {
	field := sel("endpoint-mode", "端口模式", "single", "single", "ports")
	field.StateOnly = true
	field.SelectorName = "endpoint_mode"
	field.Group = "connection"
	field.Help = "single 使用顶层端口；ports 使用端口组并隐藏顶层端口。"
	return field
}

// hysteria2ObfsModeField 声明 state_only 混淆 selector；wire 的 obfs 由 adapter 注入。
func hysteria2ObfsModeField() FieldSchema {
	field := sel("obfs-mode", "混淆模式", "none", "none", "salamander", "gecko")
	field.StateOnly = true
	field.SelectorName = "obfs_mode"
	field.Group = "connection"
	field.Help = "none 不启用混淆；salamander／gecko 需要混淆密码。"
	return field
}

// hy2ModeField 声明只在指定端口／混淆模式下活动、切换即清空的字段。
func hy2ModeField(field FieldSchema, selector string, modes []string, required bool) FieldSchema {
	condition := &ConditionRule{Selectors: map[string][]string{selector: modes}}
	field.When = condition
	if required {
		field.RequiredWhen = condition
	}
	field.ResetOn = []string{"selector." + selector}
	field.Group = "connection"
	return field
}

// hy2PortsField 声明仅在 ports 模式活动的端口组入口。
func hy2PortsField() FieldSchema {
	return hy2ModeField(f("ports", "text", "端口组"), "endpoint_mode", []string{"ports"}, true)
}

// hy2HopIntervalField 声明仅在 ports 模式活动的 Hop 间隔（单值或单范围字符串）。
func hy2HopIntervalField() FieldSchema {
	return hy2ModeField(f("hop-interval", "text", "Hop 间隔"), "endpoint_mode", []string{"ports"}, false)
}

// hy2ObfsPasswordField 声明启用混淆即必填的混淆密码。
func hy2ObfsPasswordField() FieldSchema {
	return hy2ModeField(f("obfs-password", "password", "混淆密码"), "obfs_mode", []string{"salamander", "gecko"}, true)
}

// hy2ObfsPacketSizeField 声明仅在 gecko 分支活动的混淆包大小。
func hy2ObfsPacketSizeField(name, label string) FieldSchema {
	return hy2ModeField(f(name, "number", label), "obfs_mode", []string{"gecko"}, false)
}

// hysteria2RealmOptsField 声明 Realm 子树；关闭 enable 时清空全部子字段与子凭据。
func hysteria2RealmOptsField() FieldSchema {
	field := obj("realm-opts", "Realm 参数", "fields",
		def("enable", "bool", "启用", false),
		realmSubField(f("server-url", "text", "Realm 服务地址")),
		realmSubField(f("token", "password", "Token")),
		realmSubField(f("realm-id", "text", "Realm ID")),
		realmSubField(f("stun-servers", "text-list", "STUN 服务器")),
		realmSubField(f("sni", "text", "Realm SNI")),
		realmSubField(def("skip-cert-verify", "bool", "跳过证书校验", false)),
		realmSubField(f("name-cert-verify", "text", "证书名称校验")),
		realmSubField(f("fingerprint", "text", "TLS 指纹")),
		realmSubField(f("certificate", "multiline", "客户端证书")),
		realmSubField(f("private-key", "secret-multiline", "客户端私钥")),
		realmSubField(f("alpn", "text-list", "ALPN")))
	field.Feature = &FeatureSchema{Name: "realm", Toggle: "enable"}
	field.ResetOn = []string{"feature.realm"}
	field.Group = "connection"
	return field
}

// realmSubField 标记 Realm 对象内随 enable 活动、关闭即清空的子字段。
func realmSubField(field FieldSchema) FieldSchema {
	field.When = &ConditionRule{Features: []string{"realm"}}
	field.ResetOn = []string{"feature.realm"}
	return field
}

// tlsFeatureField 把 TLS 开关声明为标量功能域；关闭时清空全部 TLS 子字段（HTTP／SOCKS5 共用）。
func tlsFeatureField() FieldSchema {
	field := def("tls", "bool", "TLS", false)
	field.Feature = &FeatureSchema{Name: "tls"}
	field.ResetOn = []string{"feature.tls"}
	return field
}

// tlsSubField 标记仅在 TLS 开启时活动、关闭即清空的 TLS 身份字段（HTTP／SOCKS5 共用）。
func tlsSubField(field FieldSchema) FieldSchema {
	field.When = &ConditionRule{Features: []string{"tls"}}
	if !field.ShouldReset("feature.tls") {
		field.ResetOn = append(field.ResetOn, "feature.tls")
	}
	field.Group = "connection"
	return field
}

// httpHeadersField 是开放的字符串请求头映射；键去空白非空、大小写不敏感不重复由组合校验保证。
func httpHeadersField() FieldSchema {
	field := openMap("headers", "请求头")
	field.MapValueType = "string"
	field.Help = "值只允许字符串；键不能为空，且大小写不敏感地不能重复。"
	return field
}

// ManualProtocols 返回 manual 节点可用的协议注册表（19 项封闭清单，ssr 除外）。
func ManualProtocols() []Protocol {
	protocols := []Protocol{
		{Protocol: "ss", Label: "Shadowsocks", FormSchema: common(
			req("cipher", "text", "加密方式"), req("password", "password", "密码"), def("udp", "bool", "UDP", true), sel("plugin", "插件", "", "", "obfs", "v2ray-plugin", "shadow-tls", "restls"),
			customPluginOpts(), obfsOpts(), v2rayPluginOpts(), shadowTlsOpts(), restlsOpts(),
			def("udp-over-tcp", "bool", "UDP over TCP", false), f("udp-over-tcp-version", "number", "UDP over TCP 版本"), f("client-fingerprint", "text", "客户端指纹"), smuxOpts()),
			SensitiveFields: []string{"password", "v2ray-plugin-opts.private-key", "shadow-tls-opts.password", "shadow-tls-opts.private-key", "restls-opts.password"}, LinkMappings: links("cipher", "password", "plugin", "obfs-opts", "v2ray-plugin-opts", "shadow-tls-opts", "restls-opts")},
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
			f("ws-path", "text", "WebSocket Path"), openMap("ws-headers", "WebSocket Headers"), def("skip-cert-verify", "bool", "跳过证书校验", false), f("fingerprint", "text", "TLS 指纹"),
			f("servername", "text", "SNI"), f("client-fingerprint", "text", "客户端指纹"), smuxOpts(), def("encryption", "text", "加密", "none")),
			SensitiveFields: []string{"uuid"}, LinkMappings: links("uuid", "flow", "network", "tls", "servername", "alpn", "reality-opts", "ws-opts", "grpc-opts", "h2-opts", "http-opts", "xhttp-opts")},
		{Protocol: "trojan", Label: "Trojan", FormSchema: common(
			req("password", "password", "密码"), f("alpn", "text-list", "ALPN"), f("sni", "text", "SNI"), def("skip-cert-verify", "bool", "跳过证书校验", false), f("fingerprint", "text", "TLS 指纹"), def("udp", "bool", "UDP", true),
			sel("network", "传输", "tcp", "tcp", "ws", "http", "h2", "grpc", "xhttp"), realityOpts(), grpcOpts(), wsOpts(), ssOpts(), f("client-fingerprint", "text", "客户端指纹")),
			SensitiveFields: []string{"password", "ss-opts.password"}, LinkMappings: links("password", "sni", "alpn", "network", "reality-opts", "ws-opts", "grpc-opts")},
		{Protocol: "hysteria", Label: "Hysteria", FormSchema: common(
			hysteriaAuthModeField(),
			hysteriaAuthCredential("auth", "Base64 认证", "base64"),
			hysteriaAuthCredential("auth-str", "认证字符串", "string"),
			req("up", "text", "上行带宽"),
			req("down", "text", "下行带宽"),
			f("ports", "text", "端口跳跃"),
			hysteriaProtocolField(),
			f("obfs", "password", "混淆"),
			f("sni", "text", "SNI"),
			echOptsField(),
			def("skip-cert-verify", "bool", "跳过证书校验", false),
			f("name-cert-verify", "text", "证书名称校验"),
			f("fingerprint", "text", "TLS 指纹"),
			f("certificate", "multiline", "客户端证书"),
			f("private-key", "secret-multiline", "客户端私钥"),
			f("alpn", "text-list", "ALPN"),
			f("recv-window-conn", "number", "连接接收窗口"),
			f("recv-window", "number", "接收窗口"),
			def("disable-mtu-discovery", "bool", "禁用 MTU 发现", false),
			def("fast-open", "bool", "Fast Open", false),
			f("hop-interval", "number", "Hop 间隔")),
			Selectors:       []SelectorSchema{{Name: "auth_mode", Values: []string{"none", "base64", "string"}, Default: "none"}},
			SensitiveFields: []string{"auth", "auth-str", "obfs", "private-key"},
			LinkMappings:    links("auth", "protocol", "up", "down", "sni", "alpn", "ports", "obfs")},
		{Protocol: "hysteria2", Label: "Hysteria2", FormSchema: common(
			req("password", "password", "密码"),
			hysteria2EndpointModeField(),
			hysteria2ObfsModeField(),
			hy2PortsField(),
			hy2HopIntervalField(),
			f("up", "text", "上行带宽"),
			f("down", "text", "下行带宽"),
			hy2ObfsPasswordField(),
			hy2ObfsPacketSizeField("obfs-min-packet-size", "混淆最小包"),
			hy2ObfsPacketSizeField("obfs-max-packet-size", "混淆最大包"),
			f("sni", "text", "SNI"),
			echOptsField(),
			def("skip-cert-verify", "bool", "跳过证书校验", false),
			f("name-cert-verify", "text", "证书名称校验"),
			f("fingerprint", "text", "TLS 指纹"),
			f("certificate", "multiline", "客户端证书"),
			f("private-key", "secret-multiline", "客户端私钥"),
			f("alpn", "text-list", "ALPN"),
			f("cwnd", "number", "拥塞窗口"),
			f("bbr-profile", "text", "BBR Profile"),
			f("udp-mtu", "number", "UDP MTU"),
			f("handshake-timeout", "number", "握手超时"),
			f("initial-stream-receive-window", "number", "初始流接收窗口"),
			f("max-stream-receive-window", "number", "最大流接收窗口"),
			f("initial-connection-receive-window", "number", "初始连接接收窗口"),
			f("max-connection-receive-window", "number", "最大连接接收窗口"),
			hysteria2RealmOptsField()),
			Selectors: []SelectorSchema{
				{Name: "endpoint_mode", Values: []string{"single", "ports"}, Default: "single"},
				{Name: "obfs_mode", Values: []string{"none", "salamander", "gecko"}, Default: "none"},
			},
			EndpointPolicies: []EndpointPolicy{
				{When: &ConditionRule{Selectors: map[string][]string{"endpoint_mode": {"single"}}},
					HostMode: "required", PortMode: "required", EmitHost: true, EmitPort: true},
				{When: &ConditionRule{Selectors: map[string][]string{"endpoint_mode": {"ports"}}},
					HostMode: "required", PortMode: "hidden", EmitHost: true, EmitPort: false},
			},
			SensitiveFields: []string{"password", "obfs-password", "private-key", "realm-opts.token", "realm-opts.private-key"},
			LinkMappings:    links("password", "sni", "alpn", "obfs", "obfs-password", "ports")},
		{Protocol: "tuic", Label: "TUIC", FormSchema: common(
			tuicAuthModeField(),
			tuicAuthCredential("token", "Token", "v4"),
			tuicAuthCredential("uuid", "UUID", "v5"),
			tuicAuthCredential("password", "密码", "v5"),
			f("ip", "text", "IP"),
			f("sni", "text", "SNI"),
			echOptsField(),
			def("skip-cert-verify", "bool", "跳过证书校验", false),
			f("name-cert-verify", "text", "证书名称校验"),
			f("fingerprint", "text", "TLS 指纹"),
			f("certificate", "multiline", "客户端证书"),
			f("private-key", "secret-multiline", "客户端私钥"),
			f("alpn", "text-list", "ALPN"),
			def("reduce-rtt", "bool", "减少 RTT", false),
			f("request-timeout", "number", "请求超时"),
			f("heartbeat-interval", "number", "心跳间隔"),
			tuicUDPRelayModeField(),
			quicCongestionControllerField(),
			def("disable-sni", "bool", "禁用 SNI", false),
			f("max-udp-relay-packet-size", "number", "最大 UDP 中继包"),
			def("fast-open", "bool", "Fast Open", false),
			f("max-open-streams", "number", "最大并发流"),
			f("cwnd", "number", "拥塞窗口"),
			f("bbr-profile", "text", "BBR Profile"),
			f("recv-window-conn", "number", "连接接收窗口"),
			f("recv-window", "number", "接收窗口"),
			def("disable-mtu-discovery", "bool", "禁用 MTU 发现", false),
			f("max-datagram-frame-size", "number", "最大数据报帧"),
			def("udp-over-stream", "bool", "UDP over Stream", false),
			tuicUDPOverStreamVersionField()),
			Selectors:       []SelectorSchema{{Name: "auth_mode", Values: []string{"v4", "v5"}, Default: "v5"}},
			SensitiveFields: []string{"token", "uuid", "password", "private-key"},
			LinkMappings:    links("token", "uuid", "password", "sni", "alpn")},
		{Protocol: "wireguard", Label: "WireGuard", FormSchema: common(
			wireGuardPeerModeField(),
			req("private-key", "secret-multiline", "私钥"),
			wireGuardModeField(f("public-key", "text", "公钥"), []string{"single"}, true),
			wireGuardModeField(f("pre-shared-key", "password", "预共享密钥"), []string{"single"}, false),
			wireGuardModeField(f("reserved", "byte-sequence", "保留字节"), []string{"single"}, false),
			wireGuardModeField(f("allowed-ips", "text-list", "Allowed IPs"), []string{"single"}, false),
			wireGuardPeersField(),
			f("ip", "text", "IP"), f("ipv6", "text", "IPv6"), ipStackField(),
			f("workers", "number", "Worker 数"), f("mtu", "number", "MTU"), def("udp", "bool", "UDP", true),
			f("persistent-keepalive", "number", "持久 Keepalive"),
			def("remote-dns-resolve", "bool", "远端 DNS 解析", false), dnsListField(),
			f("refresh-server-ip-interval", "number", "刷新服务器 IP 间隔")),
			Selectors: []SelectorSchema{{Name: "peer_mode", Values: []string{"single", "peers"}, Default: "single"}},
			EndpointPolicies: []EndpointPolicy{
				{When: &ConditionRule{Selectors: map[string][]string{"peer_mode": {"single"}}},
					HostMode: "required", PortMode: "required", EmitHost: true, EmitPort: true},
				{When: &ConditionRule{Selectors: map[string][]string{"peer_mode": {"peers"}}},
					HostMode: "hidden", PortMode: "hidden", EmitHost: false, EmitPort: false},
			},
			SensitiveFields: []string{"private-key", "pre-shared-key", "peers[].pre-shared-key"}, LinkMappings: links("private-key", "public-key", "ip", "ipv6", "allowed-ips", "pre-shared-key", "mtu", "dns")},
		{Protocol: "http", Label: "HTTP", FormSchema: common(
			basicAuthModeField(),
			basicAuthCredential("username", "text", "用户名"),
			basicAuthCredential("password", "password", "密码"),
			tlsFeatureField(),
			tlsSubField(f("sni", "text", "SNI")),
			tlsSubField(def("skip-cert-verify", "bool", "跳过证书校验", false)),
			tlsSubField(f("name-cert-verify", "text", "证书名称校验")),
			tlsSubField(f("fingerprint", "text", "TLS 指纹")),
			tlsSubField(f("certificate", "multiline", "客户端证书")),
			tlsSubField(f("private-key", "secret-multiline", "客户端私钥")),
			httpHeadersField()),
			Selectors:       []SelectorSchema{{Name: "auth_mode", Values: []string{"none", "basic"}, Default: "none"}},
			SensitiveFields: []string{"password", "private-key"}, LinkMappings: links("username", "password", "tls", "sni")},
		{Protocol: "socks5", Label: "SOCKS5", FormSchema: common(
			basicAuthModeField(),
			basicAuthCredential("username", "text", "用户名"),
			basicAuthCredential("password", "password", "密码"),
			tlsFeatureField(),
			tlsSubField(def("skip-cert-verify", "bool", "跳过证书校验", false)),
			tlsSubField(f("name-cert-verify", "text", "证书名称校验")),
			tlsSubField(f("fingerprint", "text", "TLS 指纹")),
			tlsSubField(f("certificate", "multiline", "客户端证书")),
			tlsSubField(f("private-key", "secret-multiline", "客户端私钥")),
			def("udp", "bool", "UDP", true)),
			Selectors:       []SelectorSchema{{Name: "auth_mode", Values: []string{"none", "basic"}, Default: "none"}},
			SensitiveFields: []string{"password", "private-key"}, LinkMappings: links("username", "password", "tls", "udp")},
		{Protocol: "snell", Label: "Snell", FormSchema: common(
			req("psk", "password", "PSK"),
			snellVersionField(),
			snellUDPField(),
			snellReuseField(),
			snellObfsModeField(),
			snellObfsOptsField(),
			snellClientFingerprintField()),
			Selectors: []SelectorSchema{
				{Name: "version", Values: []string{"1", "2", "3", "4", "5"}, Default: "1", SourceField: "version"},
				{Name: "obfs_mode", Values: []string{"none", "http", "tls", "shadow_tls", "restls", "jls"}, Default: "none"},
			},
			SensitiveFields: []string{"psk", "obfs-opts.password", "obfs-opts.private-key", "obfs-opts.restls-script"}},
		{Protocol: "anytls", Label: "AnyTLS", FormSchema: common(
			anytlsSecurityModeField(),
			req("password", "password", "密码"),
			f("alpn", "text-list", "ALPN"),
			f("sni", "text", "SNI"),
			echOptsField(),
			f("client-fingerprint", "text", "客户端指纹"),
			def("skip-cert-verify", "bool", "跳过证书校验", false),
			f("name-cert-verify", "text", "证书名称校验"),
			f("fingerprint", "text", "TLS 指纹"),
			f("certificate", "multiline", "证书"),
			f("private-key", "secret-multiline", "私钥"),
			anytlsShadowTLSOptsField(),
			anytlsRestlsOptsField(),
			anytlsJLSOptsField(),
			def("udp", "bool", "UDP", true),
			f("client-metadata", "text", "客户端元数据"),
			f("idle-session-check-interval", "number", "空闲检查间隔"),
			f("idle-session-timeout", "number", "空闲超时"),
			f("min-idle-session", "number", "最小空闲会话"),
			def("disable-reuse", "bool", "禁用会话复用", false)),
			Selectors: []SelectorSchema{{Name: "security_mode", Values: []string{"plain", "shadow_tls", "restls", "jls"}, Default: "plain"}},
			SensitiveFields: []string{"password", "private-key", "shadow-tls-opts.password",
				"restls-opts.password", "restls-opts.restls-script", "jls-opts.password"},
			LinkMappings: links("password", "sni", "alpn", "client-fingerprint")},
		{Protocol: "mieru", Label: "Mieru", FormSchema: common(
			mieruEndpointModeField(),
			mieruPortRangeField(),
			req("username", "text", "用户名"),
			req("password", "password", "密码"),
			mieruTransportField(),
			def("udp", "bool", "UDP", true),
			mieruMultiplexingField(),
			mieruHandshakeModeField(),
			mieruTrafficPatternField()),
			Selectors: []SelectorSchema{{Name: "endpoint_mode", Values: []string{"single", "range"}, Default: "single"}},
			EndpointPolicies: []EndpointPolicy{
				{When: &ConditionRule{Selectors: map[string][]string{"endpoint_mode": {"single"}}},
					HostMode: "required", PortMode: "required", EmitHost: true, EmitPort: true},
				{When: &ConditionRule{Selectors: map[string][]string{"endpoint_mode": {"range"}}},
					HostMode: "required", PortMode: "hidden", EmitHost: true, EmitPort: false},
			},
			SensitiveFields: []string{"password"}},
		{Protocol: "masque", Label: "MASQUE", FormSchema: common(
			masqueNetworkModeField(),
			req("private-key", "secret-multiline", "私钥"),
			req("public-key", "text", "公钥"),
			f("ip", "text", "IP"), f("ipv6", "text", "IPv6"),
			f("uri", "text", "连接 URI"), f("sni", "text", "SNI"), f("mtu", "number", "MTU"),
			f("handshake-timeout", "number", "握手超时"),
			def("skip-cert-verify", "bool", "跳过证书校验", false),
			masqueModeField(def("udp", "bool", "UDP", true), []string{"quic", "h2"}, false),
			masqueModeField(quicCongestionControllerField(), []string{"quic"}, false),
			masqueModeField(f("cwnd", "number", "拥塞窗口"), []string{"quic"}, false),
			masqueModeField(f("bbr-profile", "text", "BBR Profile"), []string{"quic"}, false),
			ipStackField(),
			def("remote-dns-resolve", "bool", "远端 DNS 解析", false), dnsListField()),
			Selectors:       []SelectorSchema{{Name: "network_mode", Values: []string{"quic", "h2", "h3_l4proxy"}, Default: "quic"}},
			SensitiveFields: []string{"private-key"}},
		{Protocol: "openvpn", Label: "OpenVPN", FormSchema: common(
			openvpnAuthModeField(),
			openvpnTLSKeyModeField(),
			req("ca", "multiline", "CA 证书"),
			openvpnAuthField("username", "text", "用户名", []string{"userpass", "cert_userpass"}, true),
			openvpnAuthField("password", "password", "密码", []string{"userpass", "cert_userpass"}, true),
			openvpnAuthField("cert", "multiline", "客户端证书", []string{"cert", "cert_userpass"}, true),
			openvpnAuthField("key", "secret-multiline", "客户端私钥", []string{"cert", "cert_userpass"}, true),
			openvpnTLSKeyField("tls-auth", "TLS Auth Key", "tls_auth"),
			openvpnKeyDirectionField(),
			openvpnTLSKeyField("tls-crypt", "TLS Crypt Key", "tls_crypt"),
			openvpnTLSKeyField("tls-crypt-v2", "TLS Crypt v2 Client Key", "tls_crypt_v2"),
			sel("proto", "传输协议", "udp", "udp", "tcp"),
			sel("dev", "设备", "tun", "tun"),
			openvpnCipherField(),
			openvpnDataCiphersField(),
			openvpnFallbackCipherField(),
			openvpnAuthDigestField(),
			openvpnCompLZOField(),
			def("remote-dns-resolve", "bool", "远端 DNS 解析", false),
			dnsListField(),
			f("ping", "number", "Ping 间隔"),
			f("ping-restart", "number", "Ping 重启"),
			f("tran-window", "number", "过渡窗口"),
			f("handshake-timeout", "number", "握手超时"),
			f("mtu", "number", "MTU"),
			openMap("peer-info", "Peer Info"),
			def("udp", "bool", "UDP", true),
			ipStackField()),
			Selectors: []SelectorSchema{
				{Name: "auth_mode", Values: []string{"userpass", "cert", "cert_userpass"}, Default: "userpass"},
				{Name: "tls_key_mode", Values: []string{"none", "tls_auth", "tls_crypt", "tls_crypt_v2"}, Default: "none"},
			},
			SensitiveFields: []string{"password", "key", "tls-auth", "tls-crypt", "tls-crypt-v2"}},
		{Protocol: "ssh", Label: "SSH", FormSchema: common(
			sshAuthModeField(),
			req("username", "text", "用户名"),
			sshBranchCredential("password", "password", "密码", "password", true),
			sshBranchCredential("private-key", "secret-multiline", "私钥", "private_key", true),
			sshBranchCredential("private-key-passphrase", "password", "私钥口令", "private_key", false),
			sshHostKeyField(),
			sshHostKeyAlgorithmsField()),
			Selectors:       []SelectorSchema{{Name: "auth_mode", Values: []string{"password", "private_key"}, Default: "password"}},
			SensitiveFields: []string{"password", "private-key", "private-key-passphrase"}},
		{Protocol: "shadowquic", Label: "ShadowQUIC", FormSchema: common(
			req("username", "text", "用户名"),
			req("password", "password", "密码"),
			f("sni", "text", "SNI"),
			f("alpn", "text-list", "ALPN"),
			shadowQUICVersionsField(),
			def("udp-over-stream", "bool", "UDP over Stream", false),
			def("zero-rtt", "bool", "0-RTT", false),
			f("keep-alive-interval", "number", "保活间隔"),
			quicCongestionControllerField(),
			f("up", "text", "上行带宽"),
			f("down", "text", "下行带宽"),
			f("cwnd", "number", "拥塞窗口"),
			f("bbr-profile", "text", "BBR Profile"),
			f("recv-window-conn", "number", "连接接收窗口"),
			f("recv-window", "number", "接收窗口"),
			def("disable-mtu-discovery", "bool", "禁用 MTU 发现", false),
			f("max-datagram-frame-size", "number", "最大数据报帧"),
			f("max-open-streams", "number", "最大并发流")),
			SensitiveFields: []string{"password"}},
		{Protocol: "trusttunnel", Label: "TrustTunnel", FormSchema: common(
			trustTunnelReuseModeField(),
			f("username", "text", "用户名"),
			f("password", "password", "密码"),
			f("alpn", "text-list", "ALPN"),
			f("sni", "text", "SNI"),
			echOptsField(),
			f("client-fingerprint", "text", "客户端指纹"),
			def("skip-cert-verify", "bool", "跳过证书校验", false),
			f("name-cert-verify", "text", "证书名称校验"),
			f("fingerprint", "text", "TLS 指纹"),
			f("certificate", "multiline", "证书"),
			f("private-key", "secret-multiline", "私钥"),
			def("udp", "bool", "UDP", true),
			def("health-check", "bool", "健康检查", false),
			trustTunnelQUICField(),
			trustTunnelQUICTuningField(quicCongestionControllerField()),
			trustTunnelQUICTuningField(f("cwnd", "number", "拥塞窗口")),
			trustTunnelQUICTuningField(f("bbr-profile", "text", "BBR Profile")),
			trustTunnelReuseNumber("max-connections", "最大连接数", "connections", true),
			trustTunnelReuseNumber("min-streams", "最小流数", "connections", false),
			trustTunnelReuseNumber("max-streams", "最大流数", "streams", true)),
			Selectors:       []SelectorSchema{{Name: "reuse_mode", Values: []string{"none", "connections", "streams"}, Default: "none"}},
			SensitiveFields: []string{"password", "private-key"}},
		{Protocol: "tailscale", Label: "Tailscale", FormSchema: common(
			f("hostname", "text", "设备名"),
			f("auth-key", "password", "认证密钥"),
			f("control-url", "text", "控制面地址"),
			def("ephemeral", "bool", "临时节点", false),
			def("udp", "bool", "UDP", true),
			f("accept-routes", "bool", "接受路由"),
			f("exit-node", "text", "出口节点"),
			tailscaleLANAccessField()),
			EndpointPolicies: []EndpointPolicy{
				{HostMode: "hidden", PortMode: "hidden", EmitHost: false, EmitPort: false},
			},
			SensitiveFields: []string{"auth-key"}},
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
		for _, name := range []string{"max-early-data", "early-data-header-name", "v2ray-http-upgrade", "v2ray-http-upgrade-fast-open", "mux"} {
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

func applySSPluginContract(field *FieldSchema, plugin string) {
	definition, ok := ssplugin.Lookup(plugin)
	if !ok {
		return
	}
	targets := []struct {
		name    string
		client  string
		version string
		entry   string
	}{
		{ssplugin.TargetClash, "Mihomo", "1.19.31", "ss.plugin-opts"},
		{ssplugin.TargetShadowrocket, "Shadowrocket", "unverified", "ss-plugin-uri"},
		{ssplugin.TargetGeneric, "Clash Verge Rev", "2.5.2", "ss-plugin-uri"},
	}
	evidence := make([]TargetEvidence, 0, len(targets))
	for _, target := range targets {
		contract, exists := definition.Target(target.name)
		if !exists {
			continue
		}
		evidence = append(evidence, TargetEvidence{
			Target: target.name, Client: target.client, Version: target.version,
			Entry: target.entry, Status: string(contract.Support),
		})
	}
	setTargetEvidence(field, evidence...)
	clash, ok := definition.Target(ssplugin.TargetClash)
	if !ok {
		return
	}
	requiresObject := false
	for _, name := range clash.RequiredFields {
		if _, hasOutputDefault := clash.Defaults[name]; hasOutputDefault {
			continue
		}
		requiresObject = true
		updateField(field.Properties, name, func(property *FieldSchema) {
			property.RequiredWhen = &ConditionRule{Targets: []string{ssplugin.TargetClash}}
		})
	}
	if requiresObject {
		field.RequiredWhen = &ConditionRule{Plugin: []string{plugin}, Targets: []string{ssplugin.TargetClash}}
	}
}

func enrichVLESS(p *Protocol) {
	setField := func(name string, fn func(*FieldSchema)) { updateField(p.FormSchema, name, fn) }
	setField("network", func(field *FieldSchema) {
		setOptionItems(field,
			option("tcp", "TCP", "common", "mihomo-1.19.31"),
			option("ws", "WebSocket", "common", "mihomo-1.19.31"),
			option("grpc", "gRPC", "common", "mihomo-1.19.31"),
			option("h2", "HTTP/2", "extended", "mihomo-1.19.31"),
			option("http", "HTTP", "extended", "mihomo-1.19.31"),
			option("xhttp", "XHTTP", "extended", "mihomo-1.19.31"))
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
		option("none", "无", "common", "mihomo-1.19.31"),
		option("tls", "TLS", "common", "mihomo-1.19.31"),
		option("reality", "REALITY", "extended", "mihomo-1.19.31"))
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
		setOptionItems(field, option("xtls-rprx-vision", "Vision", "common", "mihomo-1.19.31"))
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
			option("h2", "h2", "common", "mihomo-1.19.31"),
			option("http/1.1", "http/1.1", "common", "mihomo-1.19.31"))
	})
	setField("client-fingerprint", func(field *FieldSchema) {
		field.AllowCustom = boolPtr(true)
		setOptionItems(field,
			option("chrome", "Chrome", "common", "mihomo-1.19.31"),
			option("firefox", "Firefox", "common", "mihomo-1.19.31"),
			option("safari", "Safari", "common", "mihomo-1.19.31"),
			option("iOS", "iOS", "common", "mihomo-1.19.31"),
			option("android", "Android", "common", "mihomo-1.19.31"),
			option("edge", "Edge", "extended", "mihomo-1.19.31"),
			option("random", "Random", "extended", "mihomo-1.19.31"))
	})
	setField("xhttp-opts", func(field *FieldSchema) {
		updateField(field.Properties, "mode", func(mode *FieldSchema) {
			mode.Default = nil
			mode.AllowCustom = boolPtr(true)
			setOptionItems(mode,
				option("auto", "Auto", "common", "mihomo-1.19.31"),
				option("stream-one", "Stream One", "common", "mihomo-1.19.31"),
				option("stream-up", "Stream Up", "common", "mihomo-1.19.31"),
				option("packet-up", "Packet Up", "extended", "mihomo-1.19.31"))
		})
	})
	setField("encryption", func(field *FieldSchema) {
		field.AllowCustom = boolPtr(true)
		setOptionItems(field, option("none", "none", "common", "mihomo-1.19.31"))
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
			option("auto", "Auto", "common", "mihomo-1.19.31"),
			option("aes-128-gcm", "AES-128-GCM", "common", "mihomo-1.19.31"),
			option("chacha20-poly1305", "ChaCha20-Poly1305", "common", "mihomo-1.19.31"),
			option("none", "None", "legacy", "mihomo-1.19.31"),
			option("zero", "Zero", "legacy", "mihomo-1.19.31"))
		setTargetEvidence(field,
			TargetEvidence{Target: "sr-subs", Client: "Clash Verge Rev", Version: "2.5.2", Entry: "vmess-uri", Status: "partial"},
			TargetEvidence{Target: "generic-subs", Client: "project adapter", Version: "current", Entry: "vmess-uri", Status: "complete"})
	})
	setField("network", func(field *FieldSchema) {
		setOptionItems(field,
			option("tcp", "TCP", "common", "mihomo-1.19.31"), option("ws", "WebSocket", "common", "mihomo-1.19.31"),
			option("grpc", "gRPC", "common", "mihomo-1.19.31"), option("h2", "HTTP/2", "extended", "mihomo-1.19.31"),
			option("http", "HTTP", "extended", "mihomo-1.19.31"))
		field.AllowCustom = boolPtr(true)
		field.Group = "connection"
	})
	security := sel("security", "安全", "none", "none", "tls")
	security.AllowCustom = boolPtr(false)
	security.Group = "connection"
	setOptionItems(&security, option("none", "无", "common", "mihomo-1.19.31"), option("tls", "TLS", "common", "mihomo-1.19.31"))
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
		setTargetEvidence(field, TargetEvidence{Target: "clash-yaml", Client: "Mihomo", Version: "1.19.31", Entry: "vmess-reality", Status: "unverified"})
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
			option("h2", "h2", "common", "mihomo-1.19.31"),
			option("http/1.1", "http/1.1", "common", "mihomo-1.19.31"))
	})
	setField("client-fingerprint", func(field *FieldSchema) {
		field.AllowCustom = boolPtr(true)
		setOptionItems(field,
			option("chrome", "Chrome", "common", "mihomo-1.19.31"),
			option("firefox", "Firefox", "common", "mihomo-1.19.31"),
			option("safari", "Safari", "common", "mihomo-1.19.31"),
			option("iOS", "iOS", "common", "mihomo-1.19.31"),
			option("android", "Android", "common", "mihomo-1.19.31"),
			option("edge", "Edge", "extended", "mihomo-1.19.31"),
			option("random", "Random", "extended", "mihomo-1.19.31"))
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
			option("tcp", "TCP", "common", "mihomo-1.19.31"), option("ws", "WebSocket", "common", "mihomo-1.19.31"),
			option("grpc", "gRPC", "common", "mihomo-1.19.31"))
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
			option("h2", "h2", "common", "mihomo-1.19.31"),
			option("http/1.1", "http/1.1", "common", "mihomo-1.19.31"))
	})
	setField("client-fingerprint", func(field *FieldSchema) {
		field.AllowCustom = boolPtr(true)
		setOptionItems(field,
			option("chrome", "Chrome", "common", "mihomo-1.19.31"),
			option("firefox", "Firefox", "common", "mihomo-1.19.31"),
			option("safari", "Safari", "common", "mihomo-1.19.31"),
			option("iOS", "iOS", "common", "mihomo-1.19.31"),
			option("android", "Android", "common", "mihomo-1.19.31"),
			option("edge", "Edge", "extended", "mihomo-1.19.31"),
			option("random", "Random", "extended", "mihomo-1.19.31"))
	})
	setField("ss-opts", func(field *FieldSchema) {
		field.Group = "advanced"
		field.ResetOn = []string{"protocol"}
		updateNestedField(p.FormSchema, "ss-opts", "method", func(method *FieldSchema) {
			method.AllowCustom = boolPtr(true)
			method.Aliases = []string{"cipher"}
			setOptionItems(method,
				option("aes-128-gcm", "AES-128-GCM", "common", "mihomo-1.19.31"),
				option("aes-256-gcm", "AES-256-GCM", "common", "mihomo-1.19.31"),
				option("chacha20-ietf-poly1305", "ChaCha20-Poly1305", "common", "mihomo-1.19.31"))
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
			option("aes-128-gcm", "AES-128-GCM", "common", "mihomo-1.19.31"),
			option("aes-256-gcm", "AES-256-GCM", "common", "mihomo-1.19.31"),
			option("chacha20-ietf-poly1305", "ChaCha20-Poly1305", "common", "mihomo-1.19.31"),
			option("aes-192-gcm", "AES-192-GCM", "legacy", "mihomo-1.19.31"),
			option("xchacha20-ietf-poly1305", "XChaCha20-Poly1305", "legacy", "mihomo-1.19.31"),
			option("2022-blake3-aes-128-gcm", "SS 2022 AES-128", "pending", "project-unknown"),
			option("2022-blake3-aes-256-gcm", "SS 2022 AES-256", "pending", "project-unknown"),
			option("2022-blake3-chacha20-poly1305", "SS 2022 ChaCha20", "pending", "project-unknown"))
		setTargetEvidence(field,
			TargetEvidence{Target: "clash-yaml", Client: "Mihomo", Version: "1.19.31", Entry: "ss.cipher", Status: "complete"},
			TargetEvidence{Target: "sr-subs", Client: "Clash Verge Rev", Version: "2.5.2", Entry: "ss-uri", Status: "partial"})
	})
	setField("plugin", func(field *FieldSchema) {
		field.AllowCustom = boolPtr(true)
		field.ResetOn = []string{"plugin"}
		field.Group = "connection"
		setOptionItems(field,
			option("", "不使用插件", "common", "mihomo-1.19.31"), option("obfs", "obfs", "common", "mihomo-1.19.31"),
			option("v2ray-plugin", "v2ray-plugin", "common", "mihomo-1.19.31"), option("shadow-tls", "shadow-tls", "extended", "mihomo-1.19.31"),
			option("restls", "restls", "extended", "mihomo-1.19.31"))
	})
	setField("plugin-opts", func(field *FieldSchema) {
		excluded := append([]string{""}, ssplugin.KnownNames()...)
		field.When = &ConditionRule{PluginNot: excluded}
		field.ResetOn = []string{"plugin"}
		field.Group = "connection"
	})
	setField("obfs-opts", func(field *FieldSchema) {
		setPluginCondition(field, "obfs")
		applySSPluginContract(field, "obfs")
	})
	setField("v2ray-plugin-opts", func(field *FieldSchema) {
		setPluginCondition(field, "v2ray-plugin")
		applySSPluginContract(field, "v2ray-plugin")
	})
	setField("shadow-tls-opts", func(field *FieldSchema) {
		setPluginCondition(field, "shadow-tls")
		applySSPluginContract(field, "shadow-tls")
	})
	setField("restls-opts", func(field *FieldSchema) {
		setPluginCondition(field, "restls")
		applySSPluginContract(field, "restls")
	})
	setField("client-fingerprint", func(field *FieldSchema) {
		field.Group = "connection"
		field.AllowCustom = boolPtr(true)
		setOptionItems(field,
			option("chrome", "Chrome", "common", "mihomo-1.19.31"),
			option("firefox", "Firefox", "common", "mihomo-1.19.31"),
			option("safari", "Safari", "common", "mihomo-1.19.31"),
			option("iOS", "iOS", "common", "mihomo-1.19.31"),
			option("android", "Android", "common", "mihomo-1.19.31"),
			option("edge", "Edge", "extended", "mihomo-1.19.31"),
			option("random", "Random", "extended", "mihomo-1.19.31"))
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
		if err := validateProtocolSelectors(p); err != nil {
			panic(fmt.Sprintf("协议 %s selector 注册错误: %v", p.Protocol, err))
		}
		if err := validateProtocolFieldTypes(p); err != nil {
			panic(fmt.Sprintf("协议 %s 字段类型注册错误: %v", p.Protocol, err))
		}
		if err := validateProtocolEndpointPolicies(p); err != nil {
			panic(fmt.Sprintf("协议 %s endpoint policy 注册错误: %v", p.Protocol, err))
		}
		m[p.Protocol] = p
	}
	return m
}()

var selectorNamePattern = regexp.MustCompile(`^[a-z0-9_]+$`)

// validateProtocolSelectors 检查 selector 声明与字段投影的一致性。
// 注册期失败必须阻断启动，避免客户端通过未声明 selector 影响保存或输出。
func validateProtocolSelectors(p Protocol) error {
	declared := make(map[string]SelectorSchema, len(p.Selectors))
	for _, selector := range p.Selectors {
		if !selectorNamePattern.MatchString(selector.Name) {
			return fmt.Errorf("selector 名称非法: %q", selector.Name)
		}
		if _, exists := declared[selector.Name]; exists {
			return fmt.Errorf("selector 名称重复: %s", selector.Name)
		}
		if len(selector.Values) == 0 {
			return fmt.Errorf("selector %s 缺少允许值", selector.Name)
		}
		values := make(map[string]bool, len(selector.Values))
		for _, value := range selector.Values {
			if value == "" {
				return fmt.Errorf("selector %s 含空允许值", selector.Name)
			}
			if values[value] {
				return fmt.Errorf("selector %s 允许值重复: %s", selector.Name, value)
			}
			values[value] = true
		}
		if _, ok := values[selector.Default]; !ok {
			return fmt.Errorf("selector %s 默认值不在允许集合: %s", selector.Name, selector.Default)
		}
		declared[selector.Name] = selector
	}
	fieldByPath := make(map[string]FieldSchema)
	var walk func([]FieldSchema, string)
	walk = func(fields []FieldSchema, prefix string) {
		for _, field := range fields {
			path := field.Name
			if prefix != "" {
				path = prefix + "." + field.Name
			}
			fieldByPath[path] = field
			if field.Type == "object" {
				walk(field.Properties, path)
			}
		}
	}
	walk(p.FormSchema, "")

	referenced := make(map[string]bool)
	for path, field := range fieldByPath {
		if field.StateOnly && field.SelectorName == "" {
			return fmt.Errorf("state_only 字段 %s 缺少 selector_name", path)
		}
		if field.SelectorName == "" {
			continue
		}
		selector, ok := declared[field.SelectorName]
		if !ok {
			return fmt.Errorf("字段 %s 引用未声明 selector: %s", path, field.SelectorName)
		}
		if referenced[field.SelectorName] {
			return fmt.Errorf("selector %s 被多个字段引用", field.SelectorName)
		}
		referenced[field.SelectorName] = true
		if field.StateOnly {
			if selector.SourceField != "" {
				return fmt.Errorf("state_only selector %s 不应声明 source_field", selector.Name)
			}
			if field.Type != "select" {
				return fmt.Errorf("state_only selector %s 对应字段必须为 select", selector.Name)
			}
			continue
		}
		if selector.SourceField == "" {
			return fmt.Errorf("普通 selector %s 缺少 source_field", selector.Name)
		}
		if selector.SourceField != path {
			return fmt.Errorf("selector %s source_field 与字段路径不一致", selector.Name)
		}
	}
	for name, selector := range declared {
		if !referenced[name] {
			return fmt.Errorf("selector %s 没有对应字段", name)
		}
		if selector.SourceField == "" {
			continue
		}
		field, ok := fieldByPath[selector.SourceField]
		if !ok {
			return fmt.Errorf("selector %s source_field 不存在: %s", name, selector.SourceField)
		}
		if field.SelectorName != name || field.StateOnly {
			return fmt.Errorf("selector %s source_field 字段投影不一致", name)
		}
	}
	return nil
}

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

// SchemaPathExists 报告点路径能否在协议 schema 中解析：支持嵌套对象属性、
// 固定对象列表元素与开放 Map 的任意键。用于保证 target evidence 的 field_path
// 可以回指 schema 的规范路径，而不是只给出笼统标签。
func SchemaPathExists(proto Protocol, path string) bool {
	if path == "" {
		return false
	}
	segments, ok := parseSchemaPath(path)
	return ok && schemaPathExists(proto.FormSchema, segments)
}

type schemaPathSegment struct {
	name    string
	indexed bool
}

func parseSchemaPath(path string) ([]schemaPathSegment, bool) {
	rawSegments := strings.Split(path, ".")
	segments := make([]schemaPathSegment, 0, len(rawSegments))
	for _, raw := range rawSegments {
		if raw == "" {
			return nil, false
		}
		segment := schemaPathSegment{name: raw}
		if open := strings.IndexByte(raw, '['); open >= 0 {
			if open == 0 || !strings.HasSuffix(raw, "]") || strings.Count(raw, "[") != 1 || strings.Count(raw, "]") != 1 {
				return nil, false
			}
			index, err := strconv.Atoi(raw[open+1 : len(raw)-1])
			if err != nil || index < 0 {
				return nil, false
			}
			segment.name = raw[:open]
			segment.indexed = true
		}
		segments = append(segments, segment)
	}
	return segments, true
}

func schemaPathExists(fields []FieldSchema, segments []schemaPathSegment) bool {
	if len(segments) == 0 {
		return true
	}
	for _, field := range fields {
		segment := segments[0]
		if field.Name != segment.name {
			continue
		}
		if segment.indexed && (field.Type != "object" || field.ObjectKind != "list") {
			return false
		}
		if len(segments) == 1 {
			return true
		}
		if field.Type != "object" {
			return false
		}
		if field.AllowUnknown {
			return true
		}
		return schemaPathExists(field.Properties, segments[1:])
	}
	return false
}
