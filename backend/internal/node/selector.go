package node

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// selectorSchemaByName 返回协议声明的 selector。
func selectorSchemaByName(proto Protocol, name string) (SelectorSchema, bool) {
	for _, selector := range proto.Selectors {
		if selector.Name == name {
			return selector, true
		}
	}
	return SelectorSchema{}, false
}

// selectorValueAllowed 判断 selector 值是否在注册表允许集合内。
func selectorValueAllowed(selector SelectorSchema, value string) bool {
	for _, allowed := range selector.Values {
		if allowed == value {
			return true
		}
	}
	return false
}

// selectorSourceValue 从 protocol_json 的来源字段读取 selector 文本值。
func selectorSourceValue(params map[string]any, path string) (string, bool) {
	value, ok := GetPath(params, path)
	if !ok {
		return "", false
	}
	return selectorStringValue(value)
}

func selectorStringValue(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case bool:
		return strconv.FormatBool(v), true
	case int:
		return strconv.Itoa(v), true
	case int32:
		return strconv.FormatInt(int64(v), 10), true
	case int64:
		return strconv.FormatInt(v, 10), true
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32), true
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case json.Number:
		return v.String(), true
	default:
		return "", false
	}
}

// deriveStateOnlySelector 为 state_only selector 提供协议级派生规则（v1 读取与缺省提交共用）。
// 无规则的协议回退注册表默认值；派生只读取 protocol_json，不写库。
func deriveStateOnlySelector(proto Protocol, name string, params map[string]any) (string, bool) {
	switch proto.Protocol {
	case "http", "socks5":
		if name == "auth_mode" {
			if hasTextParam(params, "username") || hasTextParam(params, "password") {
				return "basic", true
			}
			return "none", true
		}
	}
	return "", false
}

// hasTextParam 判断字段是否存在非空文本值。
func hasTextParam(params map[string]any, name string) bool {
	value, ok := params[name].(string)
	return ok && strings.TrimSpace(value) != ""
}

// deriveSelectors 从当前协议参数派生 selector 值。
// 普通 selector 读取 source_field 或默认值；state_only 使用协议派生规则或注册表默认值。
func deriveSelectors(proto Protocol, params map[string]any) map[string]string {
	if len(proto.Selectors) == 0 {
		return nil
	}
	out := make(map[string]string, len(proto.Selectors))
	for _, selector := range proto.Selectors {
		value := selector.Default
		if selector.SourceField != "" {
			if sourceValue, ok := selectorSourceValue(params, selector.SourceField); ok && sourceValue != "" {
				value = sourceValue
			}
		} else if derived, ok := deriveStateOnlySelector(proto, selector.Name, params); ok {
			value = derived
		}
		out[selector.Name] = value
	}
	return out
}

// HydrateCurrentStateForRead 供装配层在读取节点内存副本时补全 selector，不写数据库。
func HydrateCurrentStateForRead(proto Protocol, state CurrentState, params map[string]any, version int) CurrentState {
	return hydrateCurrentStateForRead(proto, state, params, version)
}

// hydrateCurrentStateForRead 在内存中补全 selector；v1 只派生不回写，v2 保留已存值。
func hydrateCurrentStateForRead(proto Protocol, state CurrentState, params map[string]any, version int) CurrentState {
	out := state
	out.Features = append([]string(nil), state.Features...)
	out.Selectors = map[string]string{}
	for name, value := range state.Selectors {
		out.Selectors[name] = value
	}
	derived := deriveSelectors(proto, params)
	for _, selector := range proto.Selectors {
		if version < currentStateFormatVersion || out.Selectors[selector.Name] == "" {
			out.Selectors[selector.Name] = derived[selector.Name]
		}
	}
	if len(out.Selectors) == 0 {
		out.Selectors = nil
	}
	return out
}

// resolveSelectors 校验并归一化本次请求的 selector，同时把普通 selector 写入 protocol_json 来源字段。
func resolveSelectors(proto Protocol, requested map[string]string, derived map[string]string, params map[string]any) (map[string]string, error) {
	if len(proto.Selectors) == 0 {
		if len(requested) > 0 {
			return nil, fmt.Errorf("当前协议不支持 selector: %s", firstMapKey(requested))
		}
		return nil, nil
	}
	for name := range requested {
		if _, ok := selectorSchemaByName(proto, name); !ok {
			return nil, fmt.Errorf("当前协议未声明 selector: %s", name)
		}
	}
	out := make(map[string]string, len(proto.Selectors))
	for _, selector := range proto.Selectors {
		value, hasValue := requested[selector.Name]
		if hasValue {
			value = strings.TrimSpace(value)
			hasValue = value != ""
		}
		if !hasValue {
			// 未显式提交时使用 wire 派生值；state_only 使用协议派生规则或注册表默认值。
			if selector.SourceField != "" {
				value = derived[selector.Name]
			} else if stateOnly, ok := deriveStateOnlySelector(proto, selector.Name, params); ok {
				value = stateOnly
			}
			if value == "" {
				value = selector.Default
			}
		}
		if !selectorValueAllowed(selector, value) {
			return nil, fmt.Errorf("字段 selector.%s 不允许值 %q", selector.Name, value)
		}
		if selector.SourceField != "" {
			sourceValue, exists := selectorSourceValue(params, selector.SourceField)
			if exists && sourceValue != "" && sourceValue != value {
				return nil, fmt.Errorf("字段 selector.%s 与 protocol_json 不一致", selector.Name)
			}
			if !exists || sourceValue == "" {
				SetPath(params, selector.SourceField, value)
			}
		}
		out[selector.Name] = value
	}
	return out, nil
}

// selectorResetScopes 比较旧、新 selector，返回后端强制复核后的清空作用域。
func selectorResetScopes(proto Protocol, oldState, newState CurrentState) []string {
	if len(proto.Selectors) == 0 {
		return nil
	}
	var scopes []string
	for _, selector := range proto.Selectors {
		oldValue := oldState.Selectors[selector.Name]
		newValue := newState.Selectors[selector.Name]
		if oldValue == "" {
			oldValue = selector.Default
		}
		if newValue == "" {
			newValue = selector.Default
		}
		if oldValue != newValue {
			scopes = append(scopes, "selector."+selector.Name)
		}
	}
	return scopes
}

// appendResetScopes 去重追加服务端复核出的作用域。
func appendResetScopes(existing []string, extra []string) []string {
	out := append([]string(nil), existing...)
	seen := map[string]bool{}
	for _, scope := range out {
		seen[scope] = true
	}
	for _, scope := range extra {
		if scope == "" || seen[scope] {
			continue
		}
		seen[scope] = true
		out = append(out, scope)
	}
	return out
}

// validateStateOnlyParams 阻止 state_only 字段进入 protocol_json。
func validateStateOnlyParams(proto Protocol, params map[string]any) error {
	stateOnlyNames := map[string]bool{}
	for _, field := range proto.FormSchema {
		if field.StateOnly {
			stateOnlyNames[field.Name] = true
		}
	}
	for _, selector := range proto.Selectors {
		if selector.SourceField == "" {
			stateOnlyNames[selector.Name] = true
		}
	}
	for name := range params {
		if stateOnlyNames[name] {
			return fmt.Errorf("字段 %s 只能保存在 current_state.selectors，不能写入 protocol_json", name)
		}
	}
	return nil
}

// validateSelectors 检查请求 selector 集合是否与协议声明、来源字段一致。
func validateSelectors(proto Protocol, state CurrentState, params map[string]any) error {
	if len(proto.Selectors) == 0 {
		if len(state.Selectors) > 0 {
			return fmt.Errorf("当前协议不支持 selector: %s", firstMapKey(state.Selectors))
		}
		return nil
	}
	for name, value := range state.Selectors {
		selector, ok := selectorSchemaByName(proto, name)
		if !ok {
			return fmt.Errorf("当前协议未声明 selector: %s", name)
		}
		if !selectorValueAllowed(selector, value) {
			return fmt.Errorf("字段 selector.%s 不允许值 %q", name, value)
		}
		if selector.SourceField == "" {
			continue
		}
		sourceValue, exists := selectorSourceValue(params, selector.SourceField)
		if exists && sourceValue != "" && sourceValue != value {
			return fmt.Errorf("字段 selector.%s 与 protocol_json 不一致", name)
		}
	}
	// 未显式保存 selector 时按来源字段或默认值投影，禁止用缺失值绕过验证。
	for _, selector := range proto.Selectors {
		if _, exists := state.Selectors[selector.Name]; exists {
			continue
		}
		value := selector.Default
		if selector.SourceField != "" {
			if sourceValue, ok := selectorSourceValue(params, selector.SourceField); ok && sourceValue != "" {
				value = sourceValue
			}
		}
		if !selectorValueAllowed(selector, value) {
			return fmt.Errorf("字段 selector.%s 默认值非法: %q", selector.Name, value)
		}
	}
	return nil
}

func firstMapKey(values map[string]string) string {
	for key := range values {
		return key
	}
	return ""
}
