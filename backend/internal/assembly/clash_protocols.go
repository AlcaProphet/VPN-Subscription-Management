package assembly

import (
	"fmt"
	"sort"
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
