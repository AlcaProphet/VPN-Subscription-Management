package assembly

import (
	"fmt"
	"strings"

	gyaml "github.com/goccy/go-yaml"

	"vpn-sub/internal/node"
	"vpn-sub/internal/proxygroup"
	"vpn-sub/internal/rulespec"
)

// OutputIssue 是 Clash 产物静态自检问题。
type OutputIssue struct {
	Severity string `json:"severity"`
	Path     string `json:"path"`
	Message  string `json:"message"`
}

// CheckClashContent 检查 CVR 导入要求及常见的节点、组和规则引用错误。
func CheckClashContent(content []byte) []OutputIssue {
	var decoded any
	if err := gyaml.UnmarshalWithOptions(content, &decoded, gyaml.UseOrderedMap()); err != nil {
		return []OutputIssue{{Severity: "error", Path: "$", Message: "YAML 解析失败: " + err.Error()}}
	}
	root, ok := yamlMap(decoded)
	if !ok {
		return []OutputIssue{{Severity: "error", Path: "$", Message: "YAML 顶层必须是映射"}}
	}

	var issues []OutputIssue
	proxiesRaw, hasProxies := mapGet(root, "proxies")
	providersRaw, hasProviders := mapGet(root, "proxy-providers")
	if !hasProxies && !hasProviders {
		issues = append(issues, outputError("$", "必须包含顶层 proxies 或 proxy-providers"))
	}

	proxyNames := map[string]bool{}
	if hasProxies {
		proxies, ok := seqOf(proxiesRaw)
		if !ok {
			issues = append(issues, outputError("$.proxies", "proxies 必须是列表"))
		} else {
			for i, raw := range proxies {
				path := fmt.Sprintf("$.proxies[%d]", i)
				proxy, ok := yamlMap(raw)
				if !ok {
					issues = append(issues, outputError(path, "节点必须是映射"))
					continue
				}
				name := mapString(proxy, "name")
				typ := mapString(proxy, "type")
				if name == "" {
					issues = append(issues, outputError(path, "节点缺少 name"))
				} else if proxyNames[name] {
					issues = append(issues, outputError(path, "节点名重复: "+name))
				}
				if name != "" {
					proxyNames[name] = true
				}
				if typ == "" {
					issues = append(issues, outputError(path, "节点缺少 type"))
					continue
				}
				if typ != "direct" && typ != "dns" {
					for _, key := range []string{"server", "port"} {
						if _, exists := mapGet(proxy, key); !exists {
							issues = append(issues, outputError(path, "节点缺少 "+key))
						}
					}
				}
				proto, err := node.GetProtocol(typ)
				if err != nil {
					if typ != "direct" && typ != "dns" {
						issues = append(issues, outputError(path, "不支持的节点类型: "+typ))
					}
					continue
				}
				for _, field := range proto.FormSchema {
					if !field.Required {
						continue
					}
					value, exists := mapGet(proxy, field.Name)
					if !exists || isEmptyYAMLValue(value) {
						issues = append(issues, outputError(path, typ+" 缺少必填字段 "+field.Name))
					}
				}
			}
		}
	}

	providerNames := map[string]bool{}
	if hasProviders {
		providers, ok := yamlMap(providersRaw)
		if !ok {
			issues = append(issues, outputError("$.proxy-providers", "proxy-providers 必须是映射"))
		} else {
			for _, item := range providers {
				if name, ok := item.Key.(string); ok && name != "" {
					providerNames[name] = true
				}
			}
		}
	}

	groupsRaw, _ := mapGet(root, "proxy-groups")
	groups, groupsOK := seqOf(groupsRaw)
	groupNames := map[string]bool{}
	if groupsRaw != nil && !groupsOK {
		issues = append(issues, outputError("$.proxy-groups", "proxy-groups 必须是列表"))
	}
	for i, raw := range groups {
		group, ok := yamlMap(raw)
		if !ok {
			continue
		}
		name := mapString(group, "name")
		if name == "" {
			issues = append(issues, outputError(fmt.Sprintf("$.proxy-groups[%d]", i), "代理组缺少 name"))
			continue
		}
		if groupNames[name] {
			issues = append(issues, outputError(fmt.Sprintf("$.proxy-groups[%d]", i), "代理组名重复: "+name))
		}
		groupNames[name] = true
	}

	allowed := map[string]bool{
		node.ReservedDirect: true, node.ReservedReject: true, node.ReservedRejectDrop: true,
		node.ReservedPass: true, node.ReservedCompatible: true,
	}
	for name := range proxyNames {
		allowed[name] = true
	}
	for name := range groupNames {
		allowed[name] = true
	}
	for name := range providerNames {
		allowed[name] = true
	}

	for i, raw := range groups {
		path := fmt.Sprintf("$.proxy-groups[%d]", i)
		group, ok := yamlMap(raw)
		if !ok {
			issues = append(issues, outputError(path, "代理组必须是映射"))
			continue
		}
		typ := mapString(group, "type")
		if !proxygroup.ValidGroupTypes[typ] {
			issues = append(issues, outputError(path, "不支持的代理组类型: "+typ))
		}
		members, membersOK := seqOfValue(group, "proxies")
		uses, usesOK := seqOfValue(group, "use")
		includeAll := mapBool(group, "include-all") || mapBool(group, "include-all-proxies") || mapBool(group, "include-all-providers")
		if typ == "select" && len(members) == 0 && len(uses) == 0 && !includeAll {
			issues = append(issues, outputError(path, "select 组不能同时缺少 proxies/use/include-all"))
		}
		if _, exists := mapGet(group, "proxies"); exists && !membersOK {
			issues = append(issues, outputError(path, "proxies 必须是列表"))
		}
		for _, member := range members {
			name, ok := scalarString(member)
			if ok && !allowed[name] {
				issues = append(issues, outputError(path, "代理组引用不存在: "+name))
			}
		}
		if _, exists := mapGet(group, "use"); exists && !usesOK {
			issues = append(issues, outputError(path, "use 必须是列表"))
		}
		for _, provider := range uses {
			name, ok := scalarString(provider)
			if ok && !providerNames[name] {
				issues = append(issues, outputError(path, "代理组引用不存在的 provider: "+name))
			}
		}
	}

	rulesRaw, _ := mapGet(root, "rules")
	rules, rulesOK := seqOf(rulesRaw)
	if rulesRaw != nil && !rulesOK {
		issues = append(issues, outputError("$.rules", "rules 必须是列表"))
	}
	hasGeoFallback := false
	hasMatchFallback := false
	for i, raw := range rules {
		path := fmt.Sprintf("$.rules[%d]", i)
		line, ok := scalarString(raw)
		if !ok {
			issues = append(issues, outputError(path, "规则必须是字符串"))
			continue
		}
		typ, value, target, noResolve, err := rulespec.ParseRendered(line)
		if err != nil {
			issues = append(issues, outputError(path, "非法规则行: "+line+"（"+err.Error()+"）"))
			continue
		}
		mapped := rulespec.SupportsAndMapLegacy(typ, rulespec.TargetClash)
		if !mapped.Supported || typ == "USER-AGENT" {
			issues = append(issues, outputError(path, "不支持的 Clash 规则类型: "+typ))
			continue
		}
		if _, _, err := rulespec.ValidateValue(typ, value); err != nil {
			issues = append(issues, outputError(path, err.Error()))
			continue
		}
		if noResolve && !mapped.SupportsNoResolve {
			issues = append(issues, outputError(path, typ+" 不允许 no-resolve"))
		}
		if typ == "MATCH" {
			hasMatchFallback = true
		} else if typ == "GEOIP" && value == "CN" && target == node.ReservedDirect {
			hasGeoFallback = true
		}
		if !allowed[target] {
			issues = append(issues, outputError(path, "规则目标不存在: "+target))
		}
	}
	if !hasGeoFallback {
		issues = append(issues, OutputIssue{Severity: "warning", Path: "$.rules", Message: "缺少 GEOIP,CN,DIRECT 兜底规则"})
	}
	if !hasMatchFallback {
		issues = append(issues, OutputIssue{Severity: "warning", Path: "$.rules", Message: "缺少 MATCH 兜底规则"})
	}
	return issues
}

// checkForceGroupInvariants 检查装配产物的三类系统强制组与 DIRECT 层级约束。
func checkForceGroupInvariants(root gyaml.MapSlice) []OutputIssue {
	groupsRaw, ok := mapGet(root, "proxy-groups")
	if !ok {
		return []OutputIssue{outputError("$.proxy-groups", "缺少系统强制组")}
	}
	groups, ok := seqOf(groupsRaw)
	if !ok {
		return nil // 列表形态由通用自检报告。
	}
	groupMaps := map[string]gyaml.MapSlice{}
	groupIndexes := map[string]int{}
	for i, raw := range groups {
		group, ok := yamlMap(raw)
		if !ok {
			continue
		}
		name := mapString(group, "name")
		if name == "" {
			continue
		}
		groupMaps[name] = group
		groupIndexes[name] = i
	}

	var issues []OutputIssue
	forceNames := []string{node.ForceDirect, node.ForceOverseas, node.ForceFallback}
	for _, name := range forceNames {
		group, exists := groupMaps[name]
		if !exists {
			issues = append(issues, outputError("$.proxy-groups", "缺少系统强制组: "+name))
			continue
		}
		path := fmt.Sprintf("$.proxy-groups[%d]", groupIndexes[name])
		if mapString(group, "type") != "select" {
			issues = append(issues, outputError(path, "系统强制组必须使用 select 类型: "+name))
		}
	}

	for name, group := range groupMaps {
		members, membersOK := seqOfValue(group, "proxies")
		if !membersOK {
			continue
		}
		if name != node.ForceDirect {
			for _, raw := range members {
				member, ok := scalarString(raw)
				if ok && member == node.ReservedDirect {
					path := fmt.Sprintf("$.proxy-groups[%d]", groupIndexes[name])
					issues = append(issues, outputError(path, "DIRECT 只能作为『"+node.ForceDirect+"』组的固定成员"))
				}
			}
		}
	}

	if group, exists := groupMaps[node.ForceDirect]; exists {
		members, ok := seqOfValue(group, "proxies")
		if !ok || len(members) != 1 {
			path := fmt.Sprintf("$.proxy-groups[%d]", groupIndexes[node.ForceDirect])
			issues = append(issues, outputError(path, "『"+node.ForceDirect+"』组成员必须严格为 [DIRECT]"))
		} else if member, ok := scalarString(members[0]); !ok || member != node.ReservedDirect {
			path := fmt.Sprintf("$.proxy-groups[%d]", groupIndexes[node.ForceDirect])
			issues = append(issues, outputError(path, "『"+node.ForceDirect+"』组成员必须严格为 [DIRECT]"))
		}
	}

	validateConfigurable := func(name string, allowedGroups map[string]bool) {
		group, exists := groupMaps[name]
		if !exists {
			return
		}
		path := fmt.Sprintf("$.proxy-groups[%d]", groupIndexes[name])
		members, ok := seqOfValue(group, "proxies")
		if !ok || len(members) == 0 {
			issues = append(issues, outputError(path, "『"+name+"』组未包含任何成员"))
			return
		}
		seen := map[string]bool{}
		for _, raw := range members {
			member, ok := scalarString(raw)
			if !ok {
				continue
			}
			if seen[member] {
				issues = append(issues, outputError(path, "『"+name+"』组成员重复: "+member))
				continue
			}
			seen[member] = true
			if member == node.ReservedDirect || allowedGroups[member] {
				continue
			}
			if _, isGroup := groupMaps[member]; isGroup {
				issues = append(issues, outputError(path, "『"+name+"』组不允许引用代理组: "+member))
			}
		}
	}
	validateConfigurable(node.ForceOverseas, map[string]bool{node.ForceDirect: true})
	validateConfigurable(node.ForceFallback, map[string]bool{node.ForceDirect: true, node.ForceOverseas: true})
	return issues
}

// HasError 判断自检结果是否包含阻断级问题。
func HasError(issues []OutputIssue) bool {
	for _, issue := range issues {
		if issue.Severity == "error" {
			return true
		}
	}
	return false
}

func outputError(path, message string) OutputIssue {
	return OutputIssue{Severity: "error", Path: path, Message: message}
}

func yamlMap(value any) (gyaml.MapSlice, bool) {
	switch v := value.(type) {
	case gyaml.MapSlice:
		return v, true
	case map[string]any:
		return toGoccyValue(v).(gyaml.MapSlice), true
	default:
		return nil, false
	}
}

func mapGet(m gyaml.MapSlice, key string) (any, bool) {
	for _, item := range m {
		if name, ok := item.Key.(string); ok && name == key {
			return item.Value, true
		}
	}
	return nil, false
}

func mapString(m gyaml.MapSlice, key string) string {
	value, _ := mapGet(m, key)
	text, _ := scalarString(value)
	return text
}

func mapBool(m gyaml.MapSlice, key string) bool {
	value, _ := mapGet(m, key)
	result, _ := value.(bool)
	return result
}

func seqOf(value any) ([]any, bool) {
	switch v := value.(type) {
	case []any:
		return v, true
	case []string:
		out := make([]any, len(v))
		for i := range v {
			out[i] = v[i]
		}
		return out, true
	case nil:
		return nil, false
	default:
		return nil, false
	}
}

func seqOfValue(m gyaml.MapSlice, key string) ([]any, bool) {
	value, exists := mapGet(m, key)
	if !exists {
		return nil, true
	}
	return seqOf(value)
}

func scalarString(value any) (string, bool) {
	text, ok := value.(string)
	return strings.TrimSpace(text), ok
}

func isEmptyYAMLValue(value any) bool {
	if value == nil {
		return true
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text) == ""
	}
	return false
}
