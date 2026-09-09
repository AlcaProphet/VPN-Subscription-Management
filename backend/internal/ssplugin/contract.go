// Package ssplugin 提供 Shadowsocks 插件的固定合同，供节点 schema、输出与诊断共用。
package ssplugin

import (
	"fmt"
	"sort"
	"strings"
)

// 固定输出目标名称与平台 target_syntax 保持一致。
const (
	TargetClash        = "clash-yaml"
	TargetShadowrocket = "sr-subs"
	TargetGeneric      = "generic-subs"
)

// TargetIssue 是由固定插件合同派生的目标诊断，不依赖上层节点或装配类型。
type TargetIssue struct {
	Severity  string
	Code      string
	FieldPath string
	Message   string
}

// SupportLevel 描述插件参数在目标上的固定证据等级。
type SupportLevel string

const (
	SupportComplete    SupportLevel = "complete"
	SupportPartial     SupportLevel = "partial"
	SupportUnverified  SupportLevel = "unverified"
	SupportUnsupported SupportLevel = "unsupported"
)

// TargetContract 描述一个插件在单一输出目标上的参数合同。
type TargetContract struct {
	Support           SupportLevel
	Defaults          map[string]string
	RequiredFields    []string
	ExpressibleFields []string
	AllowedValues     map[string][]string
}

// Definition 描述一个已知插件的内部存储键与目标合同。
type Definition struct {
	Name       string
	StorageKey string
	Targets    map[string]TargetContract
}

// Target 返回目标合同的副本，避免调用方修改全局定义。
func (d Definition) Target(target string) (TargetContract, bool) {
	contract, ok := d.Targets[target]
	if !ok {
		return TargetContract{}, false
	}
	return cloneTarget(contract), true
}

var definitions = []Definition{
	{
		Name: "obfs", StorageKey: "obfs-opts",
		Targets: map[string]TargetContract{
			TargetClash:        targetWithAllowedValues(SupportComplete, []string{"mode"}, map[string]string{"mode": "http"}, map[string][]string{"mode": {"http", "tls"}}, "mode", "host"),
			TargetShadowrocket: target(SupportPartial, nil, nil, "mode", "host"),
			TargetGeneric:      target(SupportPartial, nil, nil, "mode", "host"),
		},
	},
	{
		Name: "v2ray-plugin", StorageKey: "v2ray-plugin-opts",
		Targets: map[string]TargetContract{
			TargetClash: targetWithAllowedValues(SupportComplete, []string{"mode"}, map[string]string{"mode": "websocket"}, map[string][]string{"mode": {"websocket"}},
				"mode", "host", "path", "headers", "tls", "ech-opts", "mux", "v2ray-http-upgrade", "v2ray-http-upgrade-fast-open", "fingerprint", "certificate", "private-key", "skip-cert-verify", "name-cert-verify"),
			TargetShadowrocket: target(SupportPartial, nil, nil, "mode", "host", "path", "tls"),
			TargetGeneric:      target(SupportPartial, nil, nil, "mode", "host", "path", "tls"),
		},
	},
	{
		Name: "shadow-tls", StorageKey: "shadow-tls-opts",
		Targets: map[string]TargetContract{
			TargetClash: target(SupportComplete, []string{"host"}, nil,
				"host", "password", "version", "alpn", "fingerprint", "certificate", "private-key", "skip-cert-verify", "name-cert-verify"),
			TargetShadowrocket: target(SupportUnverified, nil, nil,
				"host", "password", "version", "alpn", "fingerprint", "certificate", "private-key", "skip-cert-verify", "name-cert-verify"),
			TargetGeneric: target(SupportUnsupported, nil, nil),
		},
	},
	{
		Name: "restls", StorageKey: "restls-opts",
		Targets: map[string]TargetContract{
			TargetClash: target(SupportComplete, []string{"password", "host", "version-hint"}, nil,
				"password", "host", "version-hint", "restls-script", "fingerprint", "skip-cert-verify", "name-cert-verify"),
			TargetShadowrocket: target(SupportUnverified, nil, nil,
				"password", "host", "version-hint", "restls-script", "fingerprint", "skip-cert-verify", "name-cert-verify"),
			TargetGeneric: target(SupportUnsupported, nil, nil),
		},
	},
}

func target(support SupportLevel, required []string, defaults map[string]string, expressible ...string) TargetContract {
	return TargetContract{
		Support: support, RequiredFields: required, Defaults: defaults, ExpressibleFields: expressible,
	}
}

func targetWithAllowedValues(support SupportLevel, required []string, defaults map[string]string, allowed map[string][]string, expressible ...string) TargetContract {
	contract := target(support, required, defaults, expressible...)
	contract.AllowedValues = allowed
	return contract
}

// KnownNames 按稳定的表单顺序返回四个已知插件名。
func KnownNames() []string {
	names := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		names = append(names, definition.Name)
	}
	return names
}

// Lookup 返回已知插件合同的副本；未知插件由调用方使用普通字符串映射合同处理。
func Lookup(name string) (Definition, bool) {
	for _, definition := range definitions {
		if definition.Name == name {
			return cloneDefinition(definition), true
		}
	}
	return Definition{}, false
}

// AssessTarget 只检查当前活动插件及其参数对象，作为节点检查与正式装配的共同事实源。
func AssessTarget(plugin string, params map[string]any, target string) []TargetIssue {
	if plugin == "" {
		return nil
	}
	definition, known := Lookup(plugin)
	storageKey := "plugin-opts"
	if known {
		storageKey = definition.StorageKey
	}
	opts, exists, validShape := targetOptions(params[storageKey])
	if exists && !validShape {
		return []TargetIssue{targetIssue("error", "ss_plugin_shape_invalid", storageKey,
			fmt.Sprintf("SS 插件 %s 的参数对象必须是映射", plugin))}
	}

	if !known {
		if target == TargetGeneric {
			return []TargetIssue{targetIssue("error", "core_semantic_unexpressible", "plugin",
				fmt.Sprintf("%s 不支持未知 SS 插件 %s", target, plugin))}
		}
		issues := invalidUnknownOptionIssues(plugin, storageKey, opts)
		issues = append(issues, targetIssue("warn", "plugin_no_verified_mapping", "plugin",
			fmt.Sprintf("插件 %s 暂无已验证的目标映射，当前按原格式透传", plugin)))
		return issues
	}

	contract, ok := definition.Target(target)
	if !ok || contract.Support == SupportUnsupported {
		return []TargetIssue{targetIssue("error", "core_semantic_unexpressible", "plugin",
			fmt.Sprintf("%s 不支持 SS 插件 %s", target, plugin))}
	}

	issues := make([]TargetIssue, 0)
	if !exists && hasRequiredWithoutDefault(contract) {
		issues = append(issues, targetIssue("error", "ss_plugin_required_field_missing", storageKey,
			fmt.Sprintf("SS 插件 %s 缺少目标必需参数对象", plugin)))
	} else {
		for _, field := range contract.RequiredFields {
			value, hasValue := opts[field]
			_, hasDefault := contract.Defaults[field]
			if (!hasValue && !hasDefault) || hasValue && !hasEffectiveValue(value) {
				issues = append(issues, targetIssue("error", "ss_plugin_required_field_missing", storageKey+"."+field,
					fmt.Sprintf("SS 插件 %s 缺少目标必需参数 %s", plugin, field)))
			}
		}
	}
	for field, allowed := range contract.AllowedValues {
		value, hasValue := opts[field]
		if !hasValue {
			value, hasValue = contract.Defaults[field]
		}
		text, isString := value.(string)
		if hasValue && (!isString || !containsString(allowed, text)) {
			issues = append(issues, targetIssue("error", "plugin_option_unexpressible", storageKey+"."+field,
				fmt.Sprintf("SS 插件 %s 参数 %s 不受 %s 支持", plugin, field, target)))
		}
	}

	expressible := make(map[string]bool, len(contract.ExpressibleFields))
	for _, field := range contract.ExpressibleFields {
		expressible[field] = true
	}
	keys := sortedKeys(opts)
	for _, field := range keys {
		value := opts[field]
		if !expressible[field] {
			severity := "error"
			code := "plugin_option_unexpressible"
			if target == TargetClash {
				severity = "warn"
				code = "unverified_compatibility"
			}
			issues = append(issues, targetIssue(severity, code, storageKey+"."+field,
				fmt.Sprintf("SS 插件 %s 参数 %s 在 %s 上没有已验证的消费证据", plugin, field, target)))
			continue
		}
		if target != TargetClash && !uriValueExpressible(plugin, field, value) {
			issues = append(issues, targetIssue("error", "plugin_option_unexpressible", storageKey+"."+field,
				fmt.Sprintf("SS 插件 %s 参数 %s 无法在 URI 中无损表达", plugin, field)))
		}
	}

	switch contract.Support {
	case SupportPartial:
		issues = append(issues, targetIssue("warn", "plugin_partial_mapping", "plugin",
			fmt.Sprintf("SS 插件 %s 在 %s 上仅有部分映射证据", plugin, target)))
	case SupportUnverified:
		issues = append(issues, targetIssue("warn", "unverified_compatibility", "plugin",
			fmt.Sprintf("SS 插件 %s 在 %s 上仍需固定客户端验证", plugin, target)))
	}
	return issues
}

func targetIssue(severity, code, path, message string) TargetIssue {
	return TargetIssue{Severity: severity, Code: code, FieldPath: path, Message: message}
}

func targetOptions(value any) (map[string]any, bool, bool) {
	if value == nil {
		return map[string]any{}, false, true
	}
	switch typed := value.(type) {
	case map[string]any:
		return typed, true, true
	case map[string]string:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = item
		}
		return out, true, true
	default:
		return nil, true, false
	}
}

func invalidUnknownOptionIssues(plugin, storageKey string, opts map[string]any) []TargetIssue {
	issues := make([]TargetIssue, 0)
	for _, field := range sortedKeys(opts) {
		if _, ok := opts[field].(string); ok {
			continue
		}
		issues = append(issues, targetIssue("error", "plugin_option_unexpressible", storageKey+"."+field,
			fmt.Sprintf("未知 SS 插件 %s 参数 %s 必须是字符串", plugin, field)))
	}
	return issues
}

func hasRequiredWithoutDefault(contract TargetContract) bool {
	for _, field := range contract.RequiredFields {
		if _, ok := contract.Defaults[field]; !ok {
			return true
		}
	}
	return false
}

func hasEffectiveValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case []string:
		return len(typed) > 0
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	default:
		return true
	}
}

func uriValueExpressible(plugin, field string, value any) bool {
	switch typed := value.(type) {
	case string:
		return true
	case bool:
		return plugin == "v2ray-plugin" && field == "tls" ||
			(plugin == "shadow-tls" || plugin == "restls") && field == "skip-cert-verify"
	case float64, int:
		return plugin == "shadow-tls" && field == "version"
	case []string:
		return plugin == "shadow-tls" && field == "alpn" && listHasNoComma(typed)
	case []any:
		if plugin != "shadow-tls" || field != "alpn" {
			return false
		}
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return false
			}
			values = append(values, text)
		}
		return listHasNoComma(values)
	default:
		return false
	}
}

func listHasNoComma(values []string) bool {
	for _, value := range values {
		if strings.Contains(value, ",") {
			return false
		}
	}
	return true
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func cloneDefinition(definition Definition) Definition {
	out := Definition{Name: definition.Name, StorageKey: definition.StorageKey, Targets: make(map[string]TargetContract, len(definition.Targets))}
	for name, contract := range definition.Targets {
		out.Targets[name] = cloneTarget(contract)
	}
	return out
}

func cloneTarget(contract TargetContract) TargetContract {
	out := TargetContract{
		Support:           contract.Support,
		RequiredFields:    append([]string(nil), contract.RequiredFields...),
		ExpressibleFields: append([]string(nil), contract.ExpressibleFields...),
	}
	if contract.Defaults != nil {
		out.Defaults = make(map[string]string, len(contract.Defaults))
		for key, value := range contract.Defaults {
			out.Defaults[key] = value
		}
	}
	if contract.AllowedValues != nil {
		out.AllowedValues = make(map[string][]string, len(contract.AllowedValues))
		for key, values := range contract.AllowedValues {
			out.AllowedValues[key] = append([]string(nil), values...)
		}
	}
	return out
}
