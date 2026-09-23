// normalize.go：manual 表单创建与 URI 导入共用的协议参数归一化和当前状态初始化（Build20 Step 1）。
package node

import (
	"fmt"
	"strings"

	"vpn-sub/internal/ssplugin"
)

// numberParam 把 JSON 数值读取为 float64；非数值返回 false。
func numberParam(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	default:
		return 0, false
	}
}

// NormalizeProtocolJSON 将兼容输入别名规约到规范路径，供手动创建与 URI 导入共用。
// 无法归属的未知顶层字段不会被自动删除，仍交由后续 validateKnownTopLevel 显式处理。
func NormalizeProtocolJSON(proto Protocol, params map[string]any) (map[string]any, error) {
	if params == nil {
		params = map[string]any{}
	}
	out := normalizeProtocolParameters(proto, params)
	canonicalizeLegacyAliases(proto, out)
	normalizeProtocolListFields(proto, out)
	if err := normalizeByteSequenceFields(proto.FormSchema, out, ""); err != nil {
		return nil, err
	}
	if err := ensureSensitiveItemIDs(out, proto.SensitiveFields); err != nil {
		return nil, err
	}
	return out, nil
}

// normalizeProtocolListFields 对协议声明的文本列表字段去空白去重并保持用户顺序。
// 类型错误不在此处报告，继续交由 validateFieldValue 给出字段级错误。
func normalizeProtocolListFields(proto Protocol, params map[string]any) {
	switch proto.Protocol {
	case "ssh":
		normalizeStringListField(params, "host-key")
		normalizeStringListField(params, "host-key-algorithms")
	case "mieru":
		normalizeTrimmedTextFields(params, "port-range", "traffic-pattern", "transport", "multiplexing", "handshake-mode")
	case "masque":
		normalizeLocalAddressPrefixes(params)
	case "tailscale":
		normalizeTrimmedTextFields(params, "hostname", "control-url", "exit-node")
	case "shadowquic":
		normalizeStringListField(params, "quic-versions")
	case "wireguard":
		normalizeLocalAddressPrefixes(params)
		normalizeStringListField(params, "allowed-ips")
		normalizeStringListField(params, "dns")
		normalizeWireGuardPeerFields(params)
	case "openvpn":
		normalizeStringListField(params, "data-ciphers")
		normalizeStringListField(params, "dns")
	}
}

// normalizeTrimmedTextFields 去除声明文本字段的首尾空白；只做去空白，不做大小写或别名转换，
// 保持固定 tag 的精确值语义。空字符串必须原样保留：它是「清空该字段」的显式信号，
// 若在此删除，更新合并会退回旧值，用户将无法清空文本字段（空值在上层统一按未设置处理）。
func normalizeTrimmedTextFields(params map[string]any, keys ...string) {
	for _, key := range keys {
		text, ok := params[key].(string)
		if !ok {
			continue
		}
		params[key] = strings.TrimSpace(text)
	}
}

// normalizeLocalAddressPrefixes 为省略前缀的本地地址补默认前缀：IPv4→/32、IPv6→/128
// （WireGuard／MASQUE 共用；固定 tag 的 Prefixes() 行为一致）。
// 空值删除，非法值保留给字段级校验报错，不在归一化阶段吞掉。
func normalizeLocalAddressPrefixes(params map[string]any) {
	for _, item := range []struct{ key, suffix string }{{"ip", "/32"}, {"ipv6", "/128"}} {
		text, ok := params[item.key].(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			delete(params, item.key)
			continue
		}
		if !strings.Contains(trimmed, "/") {
			trimmed += item.suffix
		}
		params[item.key] = trimmed
	}
}

// normalizeWireGuardPeerFields 对每个 Peer 的 allowed-ips 去空白去重并保持用户顺序。
func normalizeWireGuardPeerFields(params map[string]any) {
	peers, ok := params["peers"].([]any)
	if !ok {
		return
	}
	for _, value := range peers {
		peer, ok := value.(map[string]any)
		if !ok {
			continue
		}
		normalizeStringListField(peer, "allowed-ips")
	}
}

func normalizeStringListField(params map[string]any, key string) {
	items, ok := stringListItems(params[key])
	if !ok {
		return
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
	if len(out) == 0 {
		delete(params, key)
		return
	}
	params[key] = out
}

// stringListItems 把字符串、字符串列表或历史多行文本统一读取为字符串切片。
// 字符串按行拆分，兼容旧版 multiline 单文本字段迁移；非文本类型返回 false。
func stringListItems(value any) ([]string, bool) {
	switch typed := value.(type) {
	case []string:
		return typed, true
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, text)
		}
		return out, true
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil, true
		}
		return strings.Split(typed, "\n"), true
	default:
		return nil, false
	}
}

func canonicalizeLegacyAliases(proto Protocol, params map[string]any) {
	switch proto.Protocol {
	case "vless", "vmess":
		canonicalizeGrpcServiceName(params)
	case "trojan":
		canonicalizeGrpcServiceName(params)
		canonicalizeTrojanInnerSS(params)
	case "ss":
		canonicalizeSSPluginOpts(params)
	case "hysteria":
		canonicalizeHysteriaAliases(params)
	case "tuic":
		canonicalizeTUICGuards(params)
	case "tailscale":
		canonicalizeTailscaleGuards(params)
	}
	if proto.Protocol == "vless" {
		canonicalizeRealityAliases(params)
	}
}

// canonicalizeTailscaleGuards 在 exit-node 为空时清空依赖它的 LAN 访问三态开关。
func canonicalizeTailscaleGuards(params map[string]any) {
	if !hasTextParam(params, "exit-node") {
		delete(params, "exit-node-allow-lan-access")
	}
}

// canonicalizeTUICGuards 执行 TUIC 的分支清空与输入归一化：
// disable-sni 清空冲突 SNI；UOT 版本 0 归一化为 tag 缺省 legacy(1)，不作为第三个版本落库。
func canonicalizeTUICGuards(params map[string]any) {
	if disabled, _ := params["disable-sni"].(bool); disabled {
		delete(params, "sni")
	}
	if version, ok := numberParam(params["udp-over-stream-version"]); ok && version == 0 {
		params["udp-over-stream-version"] = "1"
	}
}

// canonicalizeHysteriaAliases 把 Stash 兼容入口收敛到规范字段：
// obfs-protocol → protocol，up-speed／down-speed（Mbps）→ up／down 规范字符串；旧键不落库。
func canonicalizeHysteriaAliases(params map[string]any) {
	if _, exists := params["protocol"]; !exists {
		if legacy, ok := params["obfs-protocol"]; ok {
			params["protocol"] = cloneJSONValue(legacy)
		}
	}
	delete(params, "obfs-protocol")
	for _, alias := range []struct{ legacy, canonical string }{{"up-speed", "up"}, {"down-speed", "down"}} {
		if _, exists := params[alias.canonical]; !exists {
			if speed, ok := numberParam(params[alias.legacy]); ok && speed > 0 {
				params[alias.canonical] = fmt.Sprintf("%d Mbps", int64(speed))
			}
		}
		delete(params, alias.legacy)
	}
}

// canonicalizeSSPluginOpts 把已知插件的旧版/URI plugin-opts 收敛到独立对象。
// 已存在的规范对象优先，旧对象只递归补齐缺失字段；未知插件继续使用字符串 map。
func canonicalizeSSPluginOpts(params map[string]any) {
	plugin, _ := params["plugin"].(string)
	opts, ok := params["plugin-opts"].(map[string]any)
	if !ok {
		return
	}
	definition, known := ssplugin.Lookup(plugin)
	if !known {
		return
	}
	if current, exists := params[definition.StorageKey]; exists {
		params[definition.StorageKey] = mergeJSONValues(opts, current)
	} else {
		params[definition.StorageKey] = cloneJSONValue(opts)
	}
	delete(params, "plugin-opts")
}

func canonicalizeWsAliases(params map[string]any) {
	_, hasPath := params["ws-path"]
	_, hasHost := params["ws-host"]
	_, hasHeaders := params["ws-headers"]
	if !hasPath && !hasHost && !hasHeaders {
		return
	}
	opts := ensureObjectParam(params, "ws-opts")
	if _, exists := opts["path"]; !exists {
		if value, ok := params["ws-path"]; ok {
			opts["path"] = cloneJSONValue(value)
		}
	}
	if headers, ok := opts["headers"].(map[string]any); ok {
		if value, ok := params["ws-host"]; ok {
			if _, exists := headers["Host"]; !exists {
				headers["Host"] = cloneJSONValue(value)
			}
		}
		if value, ok := params["ws-headers"]; ok {
			if source, ok := value.(map[string]any); ok {
				for key, item := range source {
					if _, exists := headers[key]; !exists {
						headers[key] = cloneJSONValue(item)
					}
				}
			}
		}
	} else {
		headers := map[string]any{}
		if value, ok := params["ws-host"]; ok {
			headers["Host"] = cloneJSONValue(value)
		}
		if value, ok := params["ws-headers"]; ok {
			if source, ok := value.(map[string]any); ok {
				for key, item := range source {
					if _, exists := headers[key]; !exists {
						headers[key] = cloneJSONValue(item)
					}
				}
			}
		}
		if len(headers) > 0 {
			opts["headers"] = headers
		}
	}
	delete(params, "ws-path")
	delete(params, "ws-host")
	delete(params, "ws-headers")
}

func canonicalizeGrpcServiceName(params map[string]any) {
	if value, ok := params["grpc-service-name"]; ok {
		opts := ensureObjectParam(params, "grpc-opts")
		if _, exists := opts["grpc-service-name"]; !exists {
			opts["grpc-service-name"] = cloneJSONValue(value)
		}
		delete(params, "grpc-service-name")
	}
}

func canonicalizeRealityAliases(params map[string]any) {
	opts := ensureObjectParam(params, "reality-opts")
	if value, ok := params["public-key"]; ok {
		if _, exists := opts["public-key"]; !exists {
			opts["public-key"] = cloneJSONValue(value)
		}
		delete(params, "public-key")
	}
	if value, ok := params["short-id"]; ok {
		if _, exists := opts["short-id"]; !exists {
			opts["short-id"] = cloneJSONValue(value)
		}
		delete(params, "short-id")
	}
}

// canonicalizeTrojanInnerSS 把 Trojan 旧表单顶层 cipher 收敛到 ss-opts.method。
func canonicalizeTrojanInnerSS(params map[string]any) {
	if value, ok := params["cipher"]; ok {
		opts := ensureObjectParam(params, "ss-opts")
		if _, exists := opts["method"]; !exists {
			opts["method"] = cloneJSONValue(value)
		}
		delete(params, "cipher")
	}
}

// InitCurrentState 对协议参数初始化最小当前状态。
// 非首批协议只保留其 schema 中真实存在的稳定标识，不生成不存在于该协议的字段。
func InitCurrentState(proto Protocol, params map[string]any) CurrentState {
	state := DeriveCurrentState(proto, params)
	if !hasSchemaField(proto.FormSchema, "network") && !hasSchemaField(proto.FormSchema, "transport") {
		state.Network = ""
	}
	if !hasSchemaField(proto.FormSchema, "security") && !hasSchemaField(proto.FormSchema, "tls") && proto.Protocol != "trojan" {
		state.Security = ""
	}
	return state
}
