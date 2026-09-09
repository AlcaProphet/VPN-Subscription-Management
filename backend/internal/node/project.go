package node

import (
	"fmt"
	"strings"
)

// ProjectActive 按当前状态投影实际活动的协议参数。
// security 是表单层的统一状态字段，VLESS/VMess 输出时转换回既有 tls 语义；
// 未激活的已知分支不会进入结果，未知对象键在其所属活动对象内保留。
func ProjectActive(proto Protocol, state CurrentState, params map[string]any) map[string]any {
	params = normalizeProtocolParameters(proto, params)
	params = cleanDisabledFeatures(proto.FormSchema, params)
	// 旧状态没有嵌套功能标识时，从实际控制值派生；关闭父功能不会复活子功能。
	state.Features = activeFeatures(proto.FormSchema, params)
	out := make(map[string]any, len(params))
	for _, field := range proto.FormSchema {
		if !field.Matches(state, "") {
			continue
		}
		value, ok := params[field.Name]
		if !ok {
			continue
		}
		projected, ok := projectFieldValue(field, value, state)
		if ok {
			out[field.Name] = projected
		}
	}
	if proto.Protocol == "vless" || proto.Protocol == "vmess" {
		delete(out, "security")
		if state.Security == "tls" || state.Security == "reality" {
			out["tls"] = true
		}
	}
	return out
}

func projectFieldValue(field FieldSchema, value any, state CurrentState) (any, bool) {
	if !hasEffectiveValue(value) {
		return nil, false
	}
	if field.Type != "object" {
		return cloneProjectValue(value), true
	}

	switch field.ObjectKind {
	case "fields":
		object, ok := value.(map[string]any)
		if !ok {
			return cloneProjectValue(value), true
		}
		return projectObjectFields(field, object, state)
	case "map":
		object, ok := value.(map[string]any)
		if !ok {
			return cloneProjectValue(value), true
		}
		out := make(map[string]any, len(object))
		for key, item := range object {
			if field.MapValueType == "string" {
				if _, ok := item.(string); ok {
					out[key] = cloneProjectValue(item)
				}
				continue
			}
			if hasEffectiveValue(item) {
				out[key] = cloneProjectValue(item)
			}
		}
		if len(out) == 0 {
			return nil, false
		}
		return out, true
	case "list":
		items, ok := value.([]any)
		if !ok {
			return cloneProjectValue(value), true
		}
		out := make([]any, 0, len(items))
		for _, item := range items {
			object, ok := item.(map[string]any)
			if !ok {
				if hasEffectiveValue(item) {
					out = append(out, cloneProjectValue(item))
				}
				continue
			}
			projected, ok := projectObjectFields(field, object, state)
			if ok {
				out = append(out, projected)
			}
		}
		if len(out) == 0 {
			return nil, false
		}
		return out, true
	default:
		return cloneProjectValue(value), true
	}
}

func projectObjectFields(field FieldSchema, object map[string]any, state CurrentState) (map[string]any, bool) {
	out := make(map[string]any, len(object))
	known := make(map[string]bool, len(field.Properties))
	for _, property := range field.Properties {
		known[property.Name] = true
		if !property.Matches(state, "") {
			continue
		}
		value, ok := object[property.Name]
		if !ok {
			continue
		}
		projected, ok := projectFieldValue(property, value, state)
		if ok {
			out[property.Name] = projected
		}
	}
	if field.AllowUnknown {
		for key, value := range object {
			if known[key] || !hasEffectiveValue(value) {
				continue
			}
			out[key] = cloneProjectValue(value)
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// ValidateCurrentState 校验当前状态、活动参数和首批明确非法组合。
// 目标限定的 required_when 不在保存节点时执行，具体目标门槛由检查/输出阶段处理。
func ValidateCurrentState(proto Protocol, state CurrentState, params map[string]any) error {
	return ValidateCurrentStateForTarget(proto, state, params, "")
}

// ValidateCurrentStateForTarget 在节点检查阶段额外执行目标限定的条件必填。
// target 为空表示保存节点本身，不要求用户预先选择某个输出目标。
func ValidateCurrentStateForTarget(proto Protocol, state CurrentState, params map[string]any, target string) error {
	params = normalizeProtocolParameters(proto, params)
	derived := DeriveCurrentState(proto, params)
	if stateEmpty(state) {
		state = derived
	}
	hasNetwork := hasSchemaField(proto.FormSchema, "network") || hasSchemaField(proto.FormSchema, "transport")
	if hasNetwork && state.Network != derived.Network {
		return fmt.Errorf("current_state.network 与 protocol_json.network 不一致")
	}
	hasSecurity := hasSchemaField(proto.FormSchema, "security") || hasSchemaField(proto.FormSchema, "tls") || proto.Protocol == "trojan"
	if hasSecurity && state.Security != derived.Security {
		return fmt.Errorf("current_state.security 与 protocol_json 安全参数不一致")
	}
	if !samePlugin(state.Plugin, derived.Plugin) {
		return fmt.Errorf("current_state.plugin 与 protocol_json.plugin 不一致")
	}
	if !sameStringSet(state.Features, derived.Features) {
		return fmt.Errorf("current_state.features 与 protocol_json 功能开关不一致")
	}
	if err := validateStateOptions(proto, state); err != nil {
		return err
	}
	if err := validateActiveFields(proto.FormSchema, state, params, "", target); err != nil {
		return err
	}
	if err := validateProtocolCombination(proto, state, params); err != nil {
		return err
	}
	return nil
}

func stateEmpty(state CurrentState) bool {
	return state.Network == "" && state.Security == "" && state.Plugin == nil && len(state.Features) == 0
}

func samePlugin(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func validateStateOptions(proto Protocol, state CurrentState) error {
	if state.Network != "" {
		if field, ok := findSchemaField(proto.FormSchema, "network"); ok {
			if err := validateOption(field, state.Network, "network"); err != nil {
				return err
			}
		}
	}
	security, ok := findSchemaField(proto.FormSchema, "security")
	if ok && state.Security != "" {
		if err := validateOption(security, state.Security, "security"); err != nil {
			return err
		}
	}
	return nil
}

func findSchemaField(fields []FieldSchema, name string) (FieldSchema, bool) {
	for _, field := range fields {
		if field.Name == name {
			return field, true
		}
	}
	return FieldSchema{}, false
}

func validateActiveFields(fields []FieldSchema, state CurrentState, params map[string]any, prefix, target string) error {
	for _, field := range fields {
		if !field.Matches(state, target) {
			continue
		}
		path := field.Name
		if prefix != "" {
			path = prefix + "." + path
		}
		value, exists := params[field.Name]
		if field.RequiredFor(state, target) && (!exists || !hasEffectiveValue(value)) {
			return fmt.Errorf("字段 %s 必填", path)
		}
		if !exists || !hasEffectiveValue(value) {
			continue
		}
		if err := validateActiveFieldValue(field, value, state, path, target); err != nil {
			return err
		}
	}
	return nil
}

func validateActiveFieldValue(field FieldSchema, value any, state CurrentState, path, target string) error {
	if field.Type != "object" {
		if err := validateFieldValue(field, value, path); err != nil {
			return err
		}
		return validateOption(field, value, path)
	}
	switch field.ObjectKind {
	case "fields":
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("字段 %s 类型应为 object", path)
		}
		return validateActiveObjectFields(field, object, state, path, target)
	case "map":
		return validateFieldValue(field, value, path)
	case "list":
		items, ok := value.([]any)
		if !ok {
			return fmt.Errorf("字段 %s 类型应为 object list", path)
		}
		for i, item := range items {
			object, ok := item.(map[string]any)
			if !ok {
				return fmt.Errorf("字段 %s[%d] 类型应为 object", path, i)
			}
			if err := validateActiveObjectFields(field, object, state, fmt.Sprintf("%s[%d]", path, i), target); err != nil {
				return err
			}
		}
		return nil
	default:
		return nil
	}
}

func validateActiveObjectFields(field FieldSchema, object map[string]any, state CurrentState, path, target string) error {
	known := make(map[string]bool, len(field.Properties))
	for _, property := range field.Properties {
		known[property.Name] = true
		if !property.Matches(state, target) {
			continue
		}
		value, exists := object[property.Name]
		propertyPath := path + "." + property.Name
		if property.RequiredFor(state, target) && (!exists || !hasEffectiveValue(value)) {
			return fmt.Errorf("字段 %s 必填", propertyPath)
		}
		if !exists || !hasEffectiveValue(value) {
			continue
		}
		if err := validateActiveFieldValue(property, value, state, propertyPath, target); err != nil {
			return err
		}
	}
	if !field.AllowUnknown {
		for key := range object {
			if !known[key] {
				return fmt.Errorf("字段 %s.%s 未在协议注册表中声明", path, key)
			}
		}
	}
	return nil
}

func validateOption(field FieldSchema, value any, path string) error {
	if len(field.OptionItems) == 0 && len(field.Options) == 0 {
		return nil
	}
	text, ok := value.(string)
	if !ok {
		return nil
	}
	allowed := false
	if len(field.OptionItems) > 0 {
		for _, item := range field.OptionItems {
			if item.Value == text {
				allowed = true
				break
			}
		}
	} else {
		for _, item := range field.Options {
			if item == text {
				allowed = true
				break
			}
		}
	}
	if !allowed && !allowsCustom(field) {
		return fmt.Errorf("字段 %s 不允许值 %q", path, text)
	}
	return nil
}

func allowsCustom(field FieldSchema) bool {
	return field.AllowCustom != nil && *field.AllowCustom
}

func validateProtocolCombination(proto Protocol, state CurrentState, params map[string]any) error {
	switch proto.Protocol {
	case "vless":
		if state.Network == "xhttp" {
			xhttp := objectValue(params, "xhttp-opts")
			if mode, ok := xhttp["mode"].(string); ok && mode == "none" {
				return errorsForField("xhttp-opts.mode", "XHTTP mode 不能使用 none 作为未指定")
			}
		}
		if state.Security == "reality" && !configuredObject(params["reality-opts"]) {
			return errorsForField("reality-opts", "REALITY 安全模式缺少 reality-opts")
		}
		if security, ok := params["security"].(string); ok {
			if security == "none" && (boolValue(params["tls"]) || configuredObject(params["reality-opts"])) {
				return errorsForField("security", "security=none 不能同时启用 TLS 或 REALITY")
			}
			if security == "tls" && configuredObject(params["reality-opts"]) {
				return errorsForField("security", "security=tls 不能同时配置 REALITY 参数")
			}
		}
	case "vmess":
		if state.Security == "reality" || configuredObject(params["reality-opts"]) {
			return errorsForField("reality-opts", "VMess 首批不开放 REALITY 表单")
		}
	case "ss":
		if cipher, _ := params["cipher"].(string); cipher == "auto" {
			return errorsForField("cipher", "Shadowsocks 不支持 auto 算法")
		}
	case "trojan":
		// h2/http/xhttp 等自定义传输由目标检查/装配阶段诊断，不在此阻止保存。
		if opts := objectValue(params, "ss-opts"); boolValue(opts["enabled"]) {
			if strings.TrimSpace(stringValue(opts["method"])) == "" {
				return errorsForField("ss-opts.method", "启用内层 SS 时必须填写 method")
			}
			if strings.TrimSpace(stringValue(opts["password"])) == "" {
				return errorsForField("ss-opts.password", "启用内层 SS 时必须填写密码")
			}
		}
	}
	return nil
}

func errorsForField(path, message string) error {
	return fmt.Errorf("字段 %s: %s", path, message)
}

func objectValue(params map[string]any, key string) map[string]any {
	value, _ := params[key].(map[string]any)
	return value
}

func boolValue(value any) bool {
	result, _ := value.(bool)
	return result
}

func stringValue(value any) string {
	result, _ := value.(string)
	return result
}

func normalizeProtocolParameters(proto Protocol, params map[string]any) map[string]any {
	out := cloneJSONMap(params)
	if proto.Protocol == "trojan" {
		if opts, ok := out["ss-opts"].(map[string]any); ok {
			if _, hasMethod := opts["method"]; !hasMethod {
				if cipher, hasCipher := opts["cipher"]; hasCipher {
					opts["method"] = cloneJSONValue(cipher)
				}
			}
			delete(opts, "cipher")
		}
	}
	canonicalizeTransportAliases(proto, out)
	// 保存、读取、检查和输出共用 WS 别名归一化，规范值优先，旧值只补缺失项。
	switch proto.Protocol {
	case "vless", "vmess", "trojan":
		canonicalizeWsAliases(out)
	}
	if proto.Protocol == "ss" {
		canonicalizeSSPluginOpts(out)
	}
	return out
}

// canonicalizeTransportAliases 将 URI 导入或历史请求中的顶层传输别名
// 收敛到协议注册表的规范对象路径，避免同一参数以两套表达落库。
func canonicalizeTransportAliases(proto Protocol, params map[string]any) {
	network, _ := params["network"].(string)
	if network == "" {
		if transport, ok := params["transport"].(string); ok {
			network = transport
		}
	}
	switch proto.Protocol {
	case "vless", "vmess":
		switch network {
		case "ws":
			if hasTransportAlias(params, "path", "host") {
				opts := ensureObjectParam(params, "ws-opts")
				if _, ok := opts["path"]; !ok {
					if path, exists := params["path"]; exists {
						opts["path"] = cloneJSONValue(path)
					}
				}
				if _, ok := opts["headers"]; !ok {
					if host, exists := params["host"]; exists {
						opts["headers"] = map[string]any{"Host": cloneJSONValue(host)}
					}
				}
			}
			delete(params, "path")
			delete(params, "host")
		case "grpc":
			if hasTransportAlias(params, "path", "serviceName") {
				opts := ensureObjectParam(params, "grpc-opts")
				if _, ok := opts["grpc-service-name"]; !ok {
					if path, exists := params["path"]; exists {
						opts["grpc-service-name"] = cloneJSONValue(path)
					} else if serviceName, exists := params["serviceName"]; exists {
						opts["grpc-service-name"] = cloneJSONValue(serviceName)
					}
				}
			}
			delete(params, "path")
			delete(params, "serviceName")
		case "h2":
			if hasTransportAlias(params, "path", "host") {
				opts := ensureObjectParam(params, "h2-opts")
				if _, ok := opts["path"]; !ok {
					if path, exists := params["path"]; exists {
						opts["path"] = cloneJSONValue(path)
					}
				}
				if _, ok := opts["host"]; !ok {
					if host, exists := params["host"]; exists {
						opts["host"] = cloneJSONValue(host)
					}
				}
			}
			delete(params, "path")
			delete(params, "host")
		case "http":
			if hasTransportAlias(params, "path", "host") {
				opts := ensureObjectParam(params, "http-opts")
				if _, ok := opts["path"]; !ok {
					if path, exists := params["path"]; exists {
						opts["path"] = []any{cloneJSONValue(path)}
					}
				}
				if _, ok := opts["headers"]; !ok {
					if host, exists := params["host"]; exists {
						opts["headers"] = map[string]any{"Host": []any{cloneJSONValue(host)}}
					}
				}
			}
			delete(params, "path")
			delete(params, "host")
		case "xhttp":
			if hasTransportAlias(params, "path", "host", "mode") {
				opts := ensureObjectParam(params, "xhttp-opts")
				if _, ok := opts["path"]; !ok {
					if path, exists := params["path"]; exists {
						opts["path"] = cloneJSONValue(path)
					}
				}
				if _, ok := opts["host"]; !ok {
					if host, exists := params["host"]; exists {
						opts["host"] = cloneJSONValue(host)
					}
				}
				if _, ok := opts["mode"]; !ok {
					if mode, exists := params["mode"]; exists {
						opts["mode"] = cloneJSONValue(mode)
					}
				}
			}
			delete(params, "path")
			delete(params, "host")
			delete(params, "mode")
		}
	case "trojan":
		switch network {
		case "ws":
			if hasTransportAlias(params, "path", "host") {
				opts := ensureObjectParam(params, "ws-opts")
				if _, ok := opts["path"]; !ok {
					if path, exists := params["path"]; exists {
						opts["path"] = cloneJSONValue(path)
					}
				}
				if _, ok := opts["headers"]; !ok {
					if host, exists := params["host"]; exists {
						opts["headers"] = map[string]any{"Host": cloneJSONValue(host)}
					}
				}
			}
			delete(params, "path")
			delete(params, "host")
		case "grpc":
			if hasTransportAlias(params, "path", "serviceName") {
				opts := ensureObjectParam(params, "grpc-opts")
				if _, ok := opts["grpc-service-name"]; !ok {
					if path, exists := params["path"]; exists {
						opts["grpc-service-name"] = cloneJSONValue(path)
					}
				}
			}
			delete(params, "path")
			delete(params, "serviceName")
		}
	}
}

func hasTransportAlias(params map[string]any, keys ...string) bool {
	for _, key := range keys {
		if _, ok := params[key]; ok {
			return true
		}
	}
	return false
}

func ensureObjectParam(m map[string]any, key string) map[string]any {
	if existing, ok := m[key].(map[string]any); ok {
		return existing
	}
	created := map[string]any{}
	m[key] = created
	return created
}

func protocolParamsForStorage(proto Protocol, params map[string]any) map[string]any {
	out := normalizeProtocolParameters(proto, params)
	out = cleanDisabledFeatures(proto.FormSchema, out)
	if proto.Protocol != "vless" && proto.Protocol != "vmess" {
		return out
	}
	security, ok := out["security"].(string)
	if !ok || security == "" {
		return out
	}
	switch security {
	case "tls", "reality":
		out["tls"] = true
	case "none":
		if _, exists := out["tls"]; exists {
			out["tls"] = false
		}
	}
	delete(out, "security")
	return out
}

// validateKnownTopLevel 拒绝协议注册表未声明的顶层字段，避免更新/导入时静默丢弃。
// 未知内容应作为扩展显式声明 scope/targets 后保存。
func validateKnownTopLevel(proto Protocol, params map[string]any) error {
	allowed := make(map[string]bool, len(proto.FormSchema))
	for _, field := range proto.FormSchema {
		allowed[field.Name] = true
	}
	for key := range params {
		if !allowed[key] {
			return fmt.Errorf("字段 %s 未在协议注册表中声明，请将其归入 extensions", key)
		}
	}
	return nil
}

func cloneProjectValue(value any) any {
	switch v := value.(type) {
	case []string:
		return append([]string(nil), v...)
	case []int:
		return append([]int(nil), v...)
	case []int64:
		return append([]int64(nil), v...)
	default:
		return cloneJSONValue(value)
	}
}

func hasEffectiveValue(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(v) != ""
	case map[string]any:
		return len(v) > 0
	case []any:
		return len(v) > 0
	case []string:
		return len(v) > 0
	case []int:
		return len(v) > 0
	case []int64:
		return len(v) > 0
	default:
		return true
	}
}
