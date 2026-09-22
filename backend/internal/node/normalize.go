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
	}
	if proto.Protocol == "vless" {
		canonicalizeRealityAliases(params)
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
