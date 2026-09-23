package assembly

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"vpn-sub/internal/node"
)

// ClashNodeDraft 是显式 Clash adapter 的服务端输入。
// NodeID/Persisted 由服务端根据节点生命周期注入，客户端不能提交。
type ClashNodeDraft struct {
	NodeID    int64
	Persisted bool
	Protocol  string
	Name      string
	Host      string
	Port      int
	State     node.CurrentState
	Params    map[string]any
}

// ClashProtocolAdapter 是单一协议从内部活动模型到 Mihomo wire map 的纯函数。
type ClashProtocolAdapter func(ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error)

var clashProtocolAdapters = map[string]ClashProtocolAdapter{}

// registerClashProtocolAdapter 注册协议 adapter；同一协议重复注册直接阻断。
func registerClashProtocolAdapter(protocol string, adapter ClashProtocolAdapter) {
	if _, exists := clashProtocolAdapters[protocol]; exists {
		panic(fmt.Sprintf("Clash adapter 重复注册: %s", protocol))
	}
	clashProtocolAdapters[protocol] = adapter
}

func clashAdapter(protocol string) (ClashProtocolAdapter, bool) {
	adapter, ok := clashProtocolAdapters[protocol]
	return adapter, ok
}

// legacyAdapterPendingCount 返回尚未迁移到显式 adapter 的 manual 协议数量。
// Step 20 起 19 个 manual 协议全部迁移完毕，该值必须恒为 0；保留函数用于清单门禁。
func legacyAdapterPendingCount() int {
	count := 0
	for _, protocol := range node.ManualProtocols() {
		if _, ok := clashAdapter(protocol.Protocol); !ok {
			count++
		}
	}
	return count
}

// buildClashProxy 是正式装配与节点检查共用的唯一 Clash 入口。
// 未注册 adapter 直接返回显式错误，不存在第二套 legacy 拼装路径。
func (s *Service) buildClashProxy(nd *nodeData, persisted bool) (*OrderedMap, []node.TargetDiagnostic, error) {
	adapter, ok := clashAdapter(nd.Protocol)
	if !ok {
		return nil, nil, fmt.Errorf("协议 %s 没有显式 Clash adapter", nd.Protocol)
	}
	proto, err := node.GetProtocol(nd.Protocol)
	if err != nil {
		return nil, nil, err
	}
	state := nd.CurrentState
	if state.Network == "" && state.Security == "" && state.Plugin == nil && len(state.Features) == 0 && len(state.Selectors) == 0 {
		state = node.DeriveCurrentState(proto, nd.ProtocolJSON)
	} else {
		state = node.HydrateCurrentStateForRead(proto, state, nd.ProtocolJSON, nd.StateFormat)
	}
	params := activeProtocolJSON(nd)
	fields, diagnostics, err := adapter(ClashNodeDraft{
		NodeID: nd.NodeID, Persisted: persisted, Protocol: nd.Protocol, Name: nd.RenderName,
		Host: nd.Host, Port: nd.Port, State: state, Params: params,
	})
	if err != nil {
		return nil, diagnostics, err
	}
	policy, err := node.MatchEndpointPolicy(proto, state)
	if err != nil {
		return nil, diagnostics, err
	}
	return orderedMapFromClashFields(nd.RenderName, nd.Protocol, nd.Host, nd.Port, fields, policy), diagnostics, nil
}

// cloneClashParams 复制活动参数，保证 adapter 不修改调用方持有的 map。
func cloneClashParams(params map[string]any) map[string]any {
	out := make(map[string]any, len(params))
	for key, value := range params {
		out[key] = value
	}
	return out
}

func orderedMapFromClashFields(name, protocol, host string, port int, fields map[string]any, policy node.EndpointPolicy) *OrderedMap {
	p := NewOrderedMap()
	p.Set("name", name)
	p.Set("type", protocol)
	if policy.EmitHost {
		p.Set("server", host)
	}
	if policy.EmitPort {
		p.Set("port", port)
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		switch key {
		case "name", "type", "server", "port":
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		p.Set(key, fields[key])
	}
	return p
}

func init() {
	registerClashProtocolAdapter("ss", ssClashAdapter)
	registerClashProtocolAdapter("vmess", vmessClashAdapter)
	registerClashProtocolAdapter("vless", vlessClashAdapter)
	registerClashProtocolAdapter("trojan", trojanClashAdapter)
	registerClashProtocolAdapter("http", httpClashAdapter)
	registerClashProtocolAdapter("socks5", socks5ClashAdapter)
	registerClashProtocolAdapter("ssh", sshClashAdapter)
	registerClashProtocolAdapter("snell", snellClashAdapter)
	registerClashProtocolAdapter("hysteria", hysteriaClashAdapter)
	registerClashProtocolAdapter("hysteria2", hysteria2ClashAdapter)
	registerClashProtocolAdapter("tuic", tuicClashAdapter)
	registerClashProtocolAdapter("wireguard", wireguardClashAdapter)
	registerClashProtocolAdapter("mieru", mieruClashAdapter)
	registerClashProtocolAdapter("masque", masqueClashAdapter)
	registerClashProtocolAdapter("tailscale", tailscaleClashAdapter)
	registerClashProtocolAdapter("anytls", anytlsClashAdapter)
	registerClashProtocolAdapter("shadowquic", shadowquicClashAdapter)
	registerClashProtocolAdapter("trusttunnel", trusttunnelClashAdapter)
	registerClashProtocolAdapter("openvpn", openvpnClashAdapter)
}

// ssClashAdapter 把 Shadowsocks 内部活动模型映射为 Mihomo v1.19.31 ShadowSocksOption。
// 插件沿用项目既有规范化：存储对象 → plugin + plugin-opts（含 Clash 目标默认值）；
// 插件存储键、state_only 与未启用的 feature 对象永不进入 wire。
func ssClashAdapter(draft ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
	projected := cloneClashParams(draft.Params)
	projectSSPluginForClash(projected)
	fields := make(map[string]any, 14)
	copyClashActiveFields(fields, projected,
		"password", "cipher", "udp", "plugin", "udp-over-tcp", "udp-over-tcp-version", "client-fingerprint")
	copyClashActiveObject(fields, projected, "plugin-opts")
	copyClashFeatureObject(fields, projected, "smux")
	copyClashActiveFields(fields, projected, basicOptionClashFields...)
	return fields, nil, nil
}

// vmessClashAdapter 把 VMess 内部活动模型映射为 Mihomo v1.19.31 VmessOption。
// alterId 与 cipher 在固定 tag 中没有 omitempty，必须始终输出（缺省 0／auto）；
// 固定 tag 存在但项目 schema 未开放的字段（tlsmirror-opts／mekya-opts／mkcp-opts 等）不会输出。
func vmessClashAdapter(draft ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
	fields := make(map[string]any, 24)
	copyClashActiveFields(fields, draft.Params,
		"uuid", "udp", "network", "tls", "skip-cert-verify", "name-cert-verify",
		"fingerprint", "certificate", "private-key", "servername", "packet-addr", "xudp",
		"packet-encoding", "global-padding", "authenticated-length", "client-fingerprint")
	copyClashALPN(fields, draft.Params, "alpn")
	copyClashEnabledObject(fields, draft.Params, "ech-opts")
	copyClashActiveObject(fields, draft.Params, "reality-opts")
	copyClashTransportObject(fields, draft.Params, "http-opts", "path")
	copyClashTransportObject(fields, draft.Params, "h2-opts", "host")
	copyClashActiveObject(fields, draft.Params, "grpc-opts")
	copyClashActiveObject(fields, draft.Params, "ws-opts")
	copyClashFeatureObject(fields, draft.Params, "smux")
	fields["alterId"] = clashAlterID(draft.Params["alterId"])
	fields["cipher"] = clashVMessCipher(draft.Params["cipher"])
	copyClashActiveFields(fields, draft.Params, basicOptionClashFields...)
	return fields, nil, nil
}

// vlessClashAdapter 把 VLESS 内部活动模型映射为 Mihomo v1.19.31 VlessOption。
// 固定 tag 没有的 ECH／伪装／mTLS 字段不在项目 schema 内，也不会因共享表单被输出；
// ws-path／ws-headers 旧别名已在归一化阶段收敛进 ws-opts，不单独进入 wire。
func vlessClashAdapter(draft ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
	fields := make(map[string]any, 24)
	copyClashActiveFields(fields, draft.Params,
		"uuid", "flow", "tls", "udp", "packet-addr", "xudp", "packet-encoding", "encryption",
		"network", "skip-cert-verify", "name-cert-verify", "fingerprint", "certificate", "private-key",
		"servername", "client-fingerprint")
	copyClashALPN(fields, draft.Params, "alpn")
	copyClashEnabledObject(fields, draft.Params, "ech-opts")
	copyClashActiveObject(fields, draft.Params, "reality-opts")
	copyClashTransportObject(fields, draft.Params, "http-opts", "path")
	copyClashTransportObject(fields, draft.Params, "h2-opts", "host")
	copyClashActiveObject(fields, draft.Params, "grpc-opts")
	copyClashActiveObject(fields, draft.Params, "ws-opts")
	copyClashActiveObject(fields, draft.Params, "xhttp-opts")
	copyClashFeatureObject(fields, draft.Params, "smux")
	copyClashActiveFields(fields, draft.Params, basicOptionClashFields...)
	return fields, nil, nil
}

// trojanClashAdapter 把 Trojan 内部活动模型映射为 Mihomo v1.19.31 TrojanOption。
// 内层 SS 只在 enabled 时输出；固定 tag 没有的 ECH／伪装／mTLS 字段不会输出。
func trojanClashAdapter(draft ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
	fields := make(map[string]any, 14)
	copyClashActiveFields(fields, draft.Params,
		"password", "sni", "skip-cert-verify", "name-cert-verify", "fingerprint", "certificate",
		"private-key", "udp", "network", "client-fingerprint")
	copyClashALPN(fields, draft.Params, "alpn")
	copyClashEnabledObject(fields, draft.Params, "ech-opts")
	copyClashActiveObject(fields, draft.Params, "reality-opts")
	copyClashActiveObject(fields, draft.Params, "grpc-opts")
	copyClashActiveObject(fields, draft.Params, "ws-opts")
	if ssOpts, ok := draft.Params["ss-opts"].(map[string]any); ok && boolValue(ssOpts["enabled"]) {
		fields["ss-opts"] = ssOpts
	}
	copyClashActiveFields(fields, draft.Params, basicOptionClashFields...)
	return fields, nil, nil
}

// copyClashALPN 把 alpn 规范为内核数组：既接受数组，也接受首批协议历史沿用的
// 逗号分隔字符串（旧 legacy 投影按逗号拆分，Step 20 必须保持同一语义）。
func copyClashALPN(fields map[string]any, params map[string]any, key string) {
	value, ok := params[key]
	if !ok {
		return
	}
	if text, isText := value.(string); isText {
		if items := clashStringList(strings.ReplaceAll(text, ",", "\n")); len(items) > 0 {
			fields[key] = items
		}
		return
	}
	if items := clashStringList(value); len(items) > 0 {
		fields[key] = items
	}
}

// copyClashTransportObject 复制协议传输对象，并把声明的列表子字段规范为内核数组形状。
func copyClashTransportObject(fields map[string]any, params map[string]any, key string, listFields ...string) {
	object, ok := params[key].(map[string]any)
	if !ok || len(object) == 0 {
		return
	}
	out := make(map[string]any, len(object))
	for name, value := range object {
		out[name] = value
	}
	for _, name := range listFields {
		if items := clashStringList(out[name]); len(items) > 0 {
			out[name] = items
		}
	}
	fields[key] = out
}

// copyClashFeatureObject 只在对象的 enabled 开关为 true 时复制（例如 smux）。
func copyClashFeatureObject(fields map[string]any, params map[string]any, key string) {
	object, ok := params[key].(map[string]any)
	if !ok || !boolValue(object["enabled"]) {
		return
	}
	fields[key] = object
}

// clashAlterID 读取 VMess 的 alterId，缺省与固定 tag 的 schema 默认一致为 0。
func clashAlterID(value any) int {
	parsed, ok := clashIntValue(value)
	if !ok {
		return 0
	}
	return parsed
}

// clashVMessCipher 读取 VMess 加密方式，缺省与 schema 默认一致为 auto。
func clashVMessCipher(value any) string {
	text := clashTextValue(value)
	if text == "" {
		return "auto"
	}
	return text
}

// shadowquicClashAdapter 把 ShadowQUIC 内部活动模型映射为 Mihomo v1.19.31 ShadowQuicOption。
// TLS 只输出 sni／alpn；quic-versions 以有序去重数组输出；UOT 没有附加版本字段；
// 固定 tag 不消费 TFO／MPTCP，因此使用受限 BasicOption 子集。
func shadowquicClashAdapter(draft ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
	fields := make(map[string]any, 18)
	copyClashActiveFields(fields, draft.Params,
		"username", "password", "sni", "udp-over-stream", "zero-rtt", "keep-alive-interval",
		"congestion-controller", "up", "down", "cwnd", "bbr-profile",
		"recv-window-conn", "recv-window", "disable-mtu-discovery",
		"max-datagram-frame-size", "max-open-streams")
	copyClashListFields(fields, draft.Params, "alpn", "quic-versions")
	copyClashActiveFields(fields, draft.Params, basicOptionWithoutTFOMPTCP...)
	var diagnostics []node.TargetDiagnostic
	if zeroRTT, _ := draft.Params["zero-rtt"].(bool); zeroRTT {
		diagnostics = append(diagnostics, node.TargetDiagnostic{
			Severity: "warn", Code: "shadowquic_zero_rtt_replay_risk", Target: "clash-yaml", FieldPath: "zero-rtt",
			Message:  "开启 0-RTT 会复用会话票据，早期数据存在重放风险；风险提示不替代服务端验证",
			Evidence: "mihomo-1.19.31-yaml",
		})
	}
	return fields, diagnostics, nil
}

// trusttunnelClashAdapter 把 TrustTunnel 内部活动模型映射为 Mihomo v1.19.31 TrustTunnelOption。
// reuse_mode selector 与 state_only 不进入 wire；非当前复用分支的三个数字已由 schema 清空域移除；
// quic 关闭时 congestion-controller／cwnd／bbr-profile 同样已清空，因此这里只做点名键复制。
// 固定 tag 的 NewTrustTunnel 消费 TFO／MPTCP，因此使用完整 BasicOption 白名单。
func trusttunnelClashAdapter(draft ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
	fields := make(map[string]any, 24)
	copyClashActiveFields(fields, draft.Params,
		"username", "password", "sni", "client-fingerprint", "skip-cert-verify",
		"name-cert-verify", "fingerprint", "certificate", "private-key",
		"udp", "health-check", "quic", "congestion-controller", "cwnd", "bbr-profile",
		"max-connections", "min-streams", "max-streams")
	copyClashListFields(fields, draft.Params, "alpn")
	copyClashEnabledObject(fields, draft.Params, "ech-opts")
	copyClashActiveFields(fields, draft.Params, basicOptionClashFields...)
	return fields, nil, nil
}

// openvpnClashAdapter 把 OpenVPN 结构化活动模型映射为 Mihomo v1.19.31 OpenVPNOption 的点名 wire key。
// auth_mode／tls_key_mode selector 与 state_only 不进入 wire；非当前认证／TLS key 分支的凭据
// 已由 schema 的 clear_when_inactive／reset_on 与后端 clearSelectorScopedFields 移除，
// 原始 .ovpn、导入行号、未知指令与 client-config 永不输出。
// 固定 tag 的构造器消费 TFO／MPTCP，因此使用完整 BasicOption 白名单。
func openvpnClashAdapter(draft ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
	fields := make(map[string]any, 28)
	copyClashActiveFields(fields, draft.Params,
		"proto", "dev", "cipher", "data-ciphers-fallback", "auth", "comp-lzo",
		"ca", "cert", "key", "tls-auth", "key-direction", "tls-crypt", "tls-crypt-v2",
		"username", "password", "ping", "ping-restart", "handshake-timeout", "mtu",
		"udp", "remote-dns-resolve")
	copyClashListFields(fields, draft.Params, "data-ciphers", "dns")
	copyClashActiveObject(fields, draft.Params, "peer-info")
	copyClashActiveObject(fields, draft.Params, "ip-stack")
	copyClashOptionalZeroInt(fields, draft.Params, "tran-window")
	copyClashActiveFields(fields, draft.Params, basicOptionClashFields...)
	return fields, nil, nil
}

// copyClashOptionalZeroInt 复制需要区分「未设置」与「显式 0」的可选整数（OpenVPN tran-window）。
// 固定 tag 用 *int 表达该区别，因此显式 0 必须写入 wire，不能被通用活动值过滤跳过。
func copyClashOptionalZeroInt(fields map[string]any, params map[string]any, key string) {
	value, ok := clashIntValue(params[key])
	if !ok {
		return
	}
	fields[key] = value
}

// anytlsCamouflageWireKeys 是三种附加伪装的固定 tag wire key；只有当前安全对象会进入 YAML。
var anytlsCamouflageWireKeys = []string{"shadow-tls-opts", "restls-opts", "jls-opts"}

// anytlsClashAdapter 把 AnyTLS 内部活动模型映射为 Mihomo v1.19.31 AnyTLSOption。
// selector 不进入 wire；三种伪装对象互斥已由 schema 清空域保证，adapter 只输出实际存在的那一个。
func anytlsClashAdapter(draft ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
	fields := make(map[string]any, 22)
	copyClashActiveFields(fields, draft.Params,
		"password", "sni", "client-fingerprint", "skip-cert-verify", "name-cert-verify", "fingerprint",
		"certificate", "private-key", "udp", "client-metadata",
		"idle-session-check-interval", "idle-session-timeout", "min-idle-session", "disable-reuse")
	copyClashListFields(fields, draft.Params, "alpn")
	copyClashEnabledObject(fields, draft.Params, "ech-opts")
	for _, key := range anytlsCamouflageWireKeys {
		copyClashActiveObject(fields, draft.Params, key)
	}
	copyClashActiveFields(fields, draft.Params, basicOptionClashFields...)
	return fields, nil, nil
}

// tailscaleStateDirPrefix 是服务端派生的稳定状态目录前缀；不接受用户路径，也不进入 protocol_json。
const tailscaleStateDirPrefix = "tailscale/node-"

// tailscaleClashAdapter 把 Tailscale 内部活动模型映射为 Mihomo v1.19.31 TailscaleOption。
// wire 永不输出 server／port（endpoint policy 固定 hidden）；state-dir 由服务端注入的稳定 NodeID 派生：
// 已保存节点必须 NodeID>0，新建草稿必须 NodeID=0 且不输出任何占位目录。
func tailscaleClashAdapter(draft ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
	fields := make(map[string]any, 10)
	copyClashActiveFields(fields, draft.Params, "hostname", "auth-key", "control-url", "ephemeral", "udp", "exit-node")
	copyClashTriStateBool(fields, draft.Params, "accept-routes", "exit-node-allow-lan-access")
	copyClashActiveFields(fields, draft.Params, basicOptionWithoutTFOMPTCP...)
	switch {
	case draft.Persisted && draft.NodeID > 0:
		fields["state-dir"] = fmt.Sprintf("%s%d", tailscaleStateDirPrefix, draft.NodeID)
	case draft.Persisted:
		return nil, nil, errors.New("Tailscale 已保存节点缺少稳定 NodeID，无法派生 state-dir")
	case draft.NodeID != 0:
		return nil, nil, errors.New("Tailscale 未保存草稿不得携带节点 ID")
	}
	var diagnostics []node.TargetDiagnostic
	if clashTextValue(draft.Params["auth-key"]) == "" {
		// 只提示首次真实连接需要交互登录：不启动 tsnet、不联网、不生成或伪造登录 URL。
		diagnostics = append(diagnostics, node.TargetDiagnostic{
			Severity: "warn", Code: "tailscale_auth_key_interactive_login", Target: "clash-yaml", FieldPath: "auth-key",
			Message:  "未配置认证密钥：首次真实连接需要交互式登录，静态检查不会生成或回显登录地址",
			Evidence: "mihomo-1.19.31-yaml",
		})
	}
	if strings.HasPrefix(clashTextValue(draft.Params["control-url"]), "http://") {
		diagnostics = append(diagnostics, node.TargetDiagnostic{
			Severity: "warn", Code: "tailscale_control_url_insecure", Target: "clash-yaml", FieldPath: "control-url",
			Message:  "控制面使用非 HTTPS 地址，凭据传输不受 TLS 保护；本地 Headscale 场景可自行评估",
			Evidence: "mihomo-1.19.31-yaml",
		})
	}
	return fields, diagnostics, nil
}

// clashTextValue 读取字符串字段并去除首尾空白。
func clashTextValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

// copyClashTriStateBool 复制显式设置的三态 bool：false 必须保留，未设置必须省略。
func copyClashTriStateBool(fields map[string]any, params map[string]any, keys ...string) {
	for _, key := range keys {
		value, ok := params[key]
		if !ok {
			continue
		}
		if typed, ok := value.(bool); ok {
			fields[key] = typed
		}
	}
}

// masqueNetworkToWire 把 state_only network_mode 映射为固定 tag 的 network 值。
// quic 是内核默认分支，不写 network；h2 与 h3_l4proxy 分别映射固定 tag 的规范值。
var masqueNetworkToWire = map[string]string{"h2": "h2", "h3_l4proxy": "h3-l4proxy"}

// masqueClashAdapter 把 MASQUE 内部活动模型映射为 Mihomo v1.19.31 MasqueOption。
// name-cert-verify 在固定 tag 中只是 placeholder，schema 与 wire 都不包含；
// h3_l4proxy 的 UDP 关闭与 h2／h3_l4proxy 的 QUIC 调优清空由 selector 清空域保证。
func masqueClashAdapter(draft ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
	fields := make(map[string]any, 18)
	copyClashActiveFields(fields, draft.Params,
		"private-key", "public-key", "ip", "ipv6", "uri", "sni", "mtu", "udp", "handshake-timeout",
		"skip-cert-verify", "congestion-controller", "cwnd", "bbr-profile", "remote-dns-resolve")
	if network, ok := masqueNetworkToWire[draft.State.Selectors["network_mode"]]; ok {
		fields["network"] = network
	}
	copyClashActiveObject(fields, draft.Params, "ip-stack")
	copyClashListFields(fields, draft.Params, "dns")
	copyClashActiveFields(fields, draft.Params, basicOptionWithoutTFOMPTCP...)
	return fields, nil, nil
}

// mieruClashAdapter 把 Mieru 内部活动模型映射为 Mihomo v1.19.31 MieruOption。
// port 与 port-range 严格二选一：range 模式下顶层 port 由 endpoint policy 隐藏（规范化为 0），
// single 模式下 port-range 已由 selector 清空域移除，adapter 不再兜底拼接。
func mieruClashAdapter(draft ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
	fields := make(map[string]any, 8)
	copyClashActiveFields(fields, draft.Params,
		"username", "password", "transport", "udp", "multiplexing", "handshake-mode", "traffic-pattern")
	if draft.State.Selectors["endpoint_mode"] == "range" {
		copyClashActiveFields(fields, draft.Params, "port-range")
	}
	copyClashActiveFields(fields, draft.Params, basicOptionClashFields...)
	return fields, nil, nil
}

// basicOptionClashFields 是所有协议共享的 Mihomo BasicOption 白名单。
var basicOptionClashFields = []string{"tfo", "mptcp", "interface-name", "routing-mark", "ip-version", "dialer-proxy"}

// basicOptionWithoutTFOMPTCP 用于固定 tag 构造器不消费 TFO／MPTCP 的协议
// （WireGuard／MASQUE／Tailscale／ShadowQUIC）：共享 schema 保留字段，但不得盲目输出。
var basicOptionWithoutTFOMPTCP = []string{"interface-name", "routing-mark", "ip-version", "dialer-proxy"}

// wireguardClashAdapter 把标准 WireGuard 内部活动模型映射为 Mihomo v1.19.31 WireGuardOption。
// single 模式输出顶层 peer 字段；peers 模式只输出结构化 peers[]；两者都不输出 selector 与 _credential_id。
// AmneziaWG 不在本 adapter 内（Build32 第三章明确排除）。
func wireguardClashAdapter(draft ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
	fields := make(map[string]any, 16)
	copyClashActiveFields(fields, draft.Params,
		"private-key", "ip", "ipv6", "workers", "mtu", "udp", "persistent-keepalive",
		"remote-dns-resolve", "refresh-server-ip-interval")
	copyClashActiveObject(fields, draft.Params, "ip-stack")
	copyClashListFields(fields, draft.Params, "dns")
	if draft.State.Selectors["peer_mode"] == "peers" {
		if peers := wireguardPeerWireList(draft.Params["peers"]); len(peers) > 0 {
			fields["peers"] = peers
		}
	} else {
		copyClashActiveFields(fields, draft.Params, "public-key", "pre-shared-key")
		copyClashByteSequence(fields, draft.Params, "reserved")
		copyClashListFields(fields, draft.Params, "allowed-ips")
	}
	copyClashActiveFields(fields, draft.Params, basicOptionWithoutTFOMPTCP...)
	return fields, nil, nil
}

// wireguardPeerWireList 只复制固定 tag WireGuardPeerOption 的点名 wire key，
// 剥离 _credential_id 等内部编辑元数据，避免污染订阅产物。
func wireguardPeerWireList(value any) []any {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]any, 0, len(items))
	for _, item := range items {
		peer, ok := item.(map[string]any)
		if !ok {
			continue
		}
		wire := make(map[string]any, 6)
		copyClashActiveFields(wire, peer, "server", "port", "public-key", "pre-shared-key")
		copyClashByteSequence(wire, peer, "reserved")
		copyClashListFields(wire, peer, "allowed-ips")
		out = append(out, wire)
	}
	return out
}

// copyClashActiveObject 复制已声明且非空的对象字段（例如 ip-stack），空对象不写入 wire。
func copyClashActiveObject(fields map[string]any, params map[string]any, key string) {
	object, ok := params[key].(map[string]any)
	if !ok || len(object) == 0 {
		return
	}
	fields[key] = object
}

// copyClashByteSequence 把 byte-sequence 规范化为 3 个 0-255 整数后复制，供 reserved 输出。
func copyClashByteSequence(fields map[string]any, params map[string]any, key string) {
	value, ok := params[key]
	if !ok {
		return
	}
	values, err := node.ParseByteSequence(value)
	if err != nil || len(values) == 0 {
		return
	}
	fields[key] = values
}

// httpClashAdapter 把 HTTP 内部活动模型映射为 Mihomo v1.19.31 HttpOption wire 字段。
// 输入已由 schema 校验并经 ProjectActive 投影，因此非活动分支与 state_only 不会出现。
func httpClashAdapter(draft ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
	fields := make(map[string]any, 12)
	copyClashActiveFields(fields, draft.Params, "username", "password")
	if tls, _ := draft.Params["tls"].(bool); tls {
		fields["tls"] = true
		copyClashActiveFields(fields, draft.Params,
			"sni", "skip-cert-verify", "name-cert-verify", "fingerprint", "certificate", "private-key")
	}
	copyClashActiveFields(fields, draft.Params, "headers")
	copyClashActiveFields(fields, draft.Params, basicOptionClashFields...)
	return fields, nil, nil
}

// socks5ClashAdapter 把 SOCKS5 内部活动模型映射为 Mihomo v1.19.31 Socks5Option。
// 固定 tag 的 Socks5Option 没有独立 sni 字段，因此 adapter 不下发也不输出 sni。
func socks5ClashAdapter(draft ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
	fields := make(map[string]any, 10)
	copyClashActiveFields(fields, draft.Params, "username", "password", "udp")
	if tls, _ := draft.Params["tls"].(bool); tls {
		fields["tls"] = true
		copyClashActiveFields(fields, draft.Params,
			"skip-cert-verify", "name-cert-verify", "fingerprint", "certificate", "private-key")
	}
	copyClashActiveFields(fields, draft.Params, basicOptionClashFields...)
	return fields, nil, nil
}

// tuicClashAdapter 把 TUIC 内部活动模型映射为 Mihomo v1.19.31 TuicOption。
// v4/v5 凭据互斥已由 schema 保证；UOT 版本只在开启时以整数输出；disable-sni 给出风险 warn。
func tuicClashAdapter(draft ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
	fields := make(map[string]any, 30)
	copyClashActiveFields(fields, draft.Params,
		"token", "uuid", "password", "ip", "heartbeat-interval", "reduce-rtt", "request-timeout",
		"udp-relay-mode", "congestion-controller", "disable-sni", "max-udp-relay-packet-size",
		"fast-open", "max-open-streams", "cwnd", "bbr-profile", "skip-cert-verify", "name-cert-verify",
		"fingerprint", "certificate", "private-key", "recv-window-conn", "recv-window",
		"disable-mtu-discovery", "max-datagram-frame-size", "udp-over-stream")
	copyClashListFields(fields, draft.Params, "alpn")
	if version, ok := clashIntValue(draft.Params["udp-over-stream-version"]); ok {
		fields["udp-over-stream-version"] = version
	}
	copyClashEnabledObject(fields, draft.Params, "ech-opts")
	copyClashActiveFields(fields, draft.Params, basicOptionClashFields...)
	var diagnostics []node.TargetDiagnostic
	if disabled, _ := draft.Params["disable-sni"].(bool); disabled {
		// disable-sni 语义上覆盖 SNI：即使上游未清空也绝不下发冲突的 server-name。
		diagnostics = append(diagnostics, node.TargetDiagnostic{
			Severity: "warn", Code: "tuic_disable_sni_risk", Target: "clash-yaml", FieldPath: "disable-sni",
			Message:  "disable-sni 会同时跳过证书主机名验证，存在中间人风险",
			Evidence: "mihomo-1.19.31-yaml",
		})
	} else {
		copyClashActiveFields(fields, draft.Params, "sni")
	}
	return fields, diagnostics, nil
}

// clashIntValue 把内部数值或数字字符串读取为 int。
func clashIntValue(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		return parsed, err == nil
	default:
		return 0, false
	}
}

// hysteria2ClashAdapter 把 Hysteria2 内部活动模型映射为 Mihomo v1.19.31 Hysteria2Option。
// wire 的 obfs 由 state_only selector 注入；ports 模式不输出顶层 port（endpoint policy 已隐藏）。
func hysteria2ClashAdapter(draft ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
	fields := make(map[string]any, 26)
	copyClashActiveFields(fields, draft.Params,
		"password", "ports", "hop-interval", "up", "down", "obfs-password",
		"obfs-min-packet-size", "obfs-max-packet-size", "sni",
		"skip-cert-verify", "name-cert-verify", "fingerprint", "certificate", "private-key",
		"cwnd", "bbr-profile", "udp-mtu", "handshake-timeout",
		"initial-stream-receive-window", "max-stream-receive-window",
		"initial-connection-receive-window", "max-connection-receive-window")
	if mode := draft.State.Selectors["obfs_mode"]; mode != "" && mode != "none" {
		fields["obfs"] = mode
	}
	copyClashListFields(fields, draft.Params, "alpn")
	copyClashEnabledObject(fields, draft.Params, "ech-opts")
	copyClashEnabledObject(fields, draft.Params, "realm-opts")
	copyClashActiveFields(fields, draft.Params, basicOptionClashFields...)
	return fields, nil, nil
}

// copyClashEnabledObject 只在对象自身的 enable 开关为 true 时复制，避免把禁用配置与凭据写入 wire。
func copyClashEnabledObject(fields map[string]any, params map[string]any, key string) {
	object, ok := params[key].(map[string]any)
	if !ok || !boolValue(object["enable"]) {
		return
	}
	fields[key] = object
}

// hysteriaClashAdapter 把 Hysteria v1 内部活动模型映射为 Mihomo v1.19.31 HysteriaOption。
// 只输出当前认证分支的 auth／auth-str；兼容别名 obfs-protocol／up-speed／down-speed 永不进入 wire。
func hysteriaClashAdapter(draft ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
	fields := make(map[string]any, 22)
	copyClashActiveFields(fields, draft.Params,
		"up", "down", "ports", "protocol", "auth", "auth-str", "obfs", "sni",
		"skip-cert-verify", "name-cert-verify", "fingerprint", "certificate", "private-key",
		"recv-window-conn", "recv-window", "disable-mtu-discovery", "fast-open", "hop-interval")
	copyClashListFields(fields, draft.Params, "alpn")
	copyClashEnabledObject(fields, draft.Params, "ech-opts")
	copyClashActiveFields(fields, draft.Params, basicOptionClashFields...)
	return fields, nil, nil
}

// snellObfsModeToWire 把 state_only 混淆模式映射为固定 tag 的 obfs-opts.mode；none 不产生对象。
var snellObfsModeToWire = map[string]string{
	"http": "http", "tls": "tls", "shadow_tls": "shadow-tls", "restls": "restls", "jls": "jls",
}

// snellClashAdapter 把 Snell 内部活动模型映射为 Mihomo v1.19.31 SnellOption。
// obfs-opts.mode 由 selector 注入 wire；v2 固定 reuse；v1/v2 不输出 udp；v5 给出 v4 兼容诊断。
func snellClashAdapter(draft ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
	fields := make(map[string]any, 8)
	version := clashSnellVersion(draft.Params["version"])
	fields["psk"] = draft.Params["psk"]
	fields["version"] = version
	if udp, _ := draft.Params["udp"].(bool); udp && version >= 3 {
		fields["udp"] = true
	}
	if version == 2 {
		// v2 内核强制 reuse，wire 显式写 true 以反映实际语义。
		fields["reuse"] = true
	} else if version >= 4 {
		if reuse, _ := draft.Params["reuse"].(bool); reuse {
			fields["reuse"] = true
		}
	}
	if obfs := snellObfsWireFields(draft); len(obfs) > 0 {
		fields["obfs-opts"] = obfs
	}
	copyClashActiveFields(fields, draft.Params, "client-fingerprint")
	copyClashActiveFields(fields, draft.Params, basicOptionClashFields...)
	var diagnostics []node.TargetDiagnostic
	if version == 5 {
		diagnostics = append(diagnostics, node.TargetDiagnostic{
			Severity: "info", Code: "snell_v5_v4_compat", Target: "clash-yaml", FieldPath: "version",
			Message:  "Snell v5 由内核按 v4 客户端实现，wire 保留 version: 5 以兼容服务端",
			Evidence: "mihomo-1.19.31-yaml",
		})
	}
	return fields, diagnostics, nil
}

// snellObfsWireFields 组装当前混淆模式的 obfs-opts；none 或未知模式返回 nil。
func snellObfsWireFields(draft ClashNodeDraft) map[string]any {
	mode, ok := snellObfsModeToWire[draft.State.Selectors["obfs_mode"]]
	if !ok {
		return nil
	}
	source, _ := draft.Params["obfs-opts"].(map[string]any)
	out := make(map[string]any, len(source)+1)
	for key, value := range source {
		if clashValueActive(value) {
			out[key] = value
		}
	}
	if alpn := clashStringList(source["alpn"]); len(alpn) > 0 {
		out["alpn"] = alpn
	}
	out["mode"] = mode
	return out
}

// clashSnellVersion 把内部版本选择归一化为固定 tag 的整数版本，缺省按 v1。
func clashSnellVersion(value any) int {
	switch typed := value.(type) {
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
			return parsed
		}
	case int:
		return typed
	case float64:
		return int(typed)
	}
	return 1
}

// sshClashAdapter 把 SSH 内部活动模型映射为 Mihomo v1.19.31 SshOption wire 字段。
// host-key／host-key-algorithms 是内核数组字段；SshOption 固定 UDP=false，因此永不输出 udp。
func sshClashAdapter(draft ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
	fields := make(map[string]any, 8)
	copyClashActiveFields(fields, draft.Params, "username", "password", "private-key", "private-key-passphrase")
	copyClashListFields(fields, draft.Params, "host-key", "host-key-algorithms")
	copyClashActiveFields(fields, draft.Params, basicOptionClashFields...)
	var diagnostics []node.TargetDiagnostic
	if len(clashStringList(draft.Params["host-key"])) == 0 {
		diagnostics = append(diagnostics, node.TargetDiagnostic{
			Severity: "warn", Code: "ssh_host_key_unverified", Target: "clash-yaml", FieldPath: "host-key",
			Message:  "未配置 Host Key，客户端将接受任意服务器 Host Key，存在中间人风险",
			Evidence: "mihomo-1.19.31-yaml",
		})
	}
	return fields, diagnostics, nil
}

// copyClashListFields 把内部文本列表规范为去空白去重的 YAML 字符串数组后复制。
func copyClashListFields(fields map[string]any, params map[string]any, keys ...string) {
	for _, key := range keys {
		list := clashStringList(params[key])
		if len(list) == 0 {
			continue
		}
		fields[key] = list
	}
}

// clashStringList 把字符串或字符串列表规范为去空白去重的字符串数组。
func clashStringList(value any) []string {
	var items []string
	switch typed := value.(type) {
	case string:
		items = strings.Split(typed, "\n")
	case []string:
		items = typed
	case []any:
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				continue
			}
			items = append(items, text)
		}
	default:
		return nil
	}
	seen := make(map[string]bool, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	return out
}

// copyClashActiveFields 只复制已设置且非零值的字段，避免向 wire 输出空串、false 或 0。
func copyClashActiveFields(fields map[string]any, params map[string]any, keys ...string) {
	for _, key := range keys {
		value, ok := params[key]
		if !ok || !clashValueActive(value) {
			continue
		}
		fields[key] = value
	}
}

func clashValueActive(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case bool:
		return typed
	case int:
		return typed != 0
	case int32:
		return typed != 0
	case int64:
		return typed != 0
	case float32:
		return typed != 0
	case float64:
		return typed != 0
	case map[string]any:
		return len(typed) > 0
	case map[string]string:
		return len(typed) > 0
	case []any:
		return len(typed) > 0
	case []string:
		return len(typed) > 0
	default:
		return true
	}
}
