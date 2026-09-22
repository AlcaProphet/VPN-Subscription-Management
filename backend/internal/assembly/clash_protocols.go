package assembly

import (
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

// legacyAdapterPendingCount 返回尚未迁移到显式 adapter 的 manual 协议数量；Step 20 必须归零。
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
// 未迁移协议仍走临时 legacy 投影，但返回可观测的 legacy_adapter_pending。
func (s *Service) buildClashProxy(nd *nodeData, persisted bool) (*OrderedMap, []node.TargetDiagnostic, error) {
	adapter, ok := clashAdapter(nd.Protocol)
	if !ok {
		return s.legacyClashProxy(nd), []node.TargetDiagnostic{legacyAdapterPendingDiagnostic(nd.Protocol)}, nil
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

// clashProxy 保留原有调用形状；正式语义由 buildClashProxy 提供。
func (s *Service) clashProxy(nd *nodeData) *OrderedMap {
	p, _, err := s.buildClashProxy(nd, true)
	if err != nil {
		return s.legacyClashProxy(nd)
	}
	return p
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

func legacyAdapterPendingDiagnostic(protocol string) node.TargetDiagnostic {
	return node.TargetDiagnostic{
		Severity:  "info",
		Code:      "legacy_adapter_pending",
		Target:    "clash-yaml",
		FieldPath: "",
		Message:   "协议 " + protocol + " 尚未迁移到显式 Clash adapter，当前由 legacy 投影承载",
		Evidence:  "build32-step3",
	}
}

func init() {
	registerClashProtocolAdapter("http", httpClashAdapter)
	registerClashProtocolAdapter("socks5", socks5ClashAdapter)
	registerClashProtocolAdapter("ssh", sshClashAdapter)
	registerClashProtocolAdapter("snell", snellClashAdapter)
	registerClashProtocolAdapter("hysteria", hysteriaClashAdapter)
	registerClashProtocolAdapter("hysteria2", hysteria2ClashAdapter)
	registerClashProtocolAdapter("tuic", tuicClashAdapter)
}

// basicOptionClashFields 是所有协议共享的 Mihomo BasicOption 白名单。
var basicOptionClashFields = []string{"tfo", "mptcp", "interface-name", "routing-mark", "ip-version", "dialer-proxy"}

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
