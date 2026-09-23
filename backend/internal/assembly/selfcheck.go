package assembly

import (
	"fmt"
	"strings"

	gyaml "github.com/goccy/go-yaml"

	"vpn-sub/internal/node"
	"vpn-sub/internal/proxygroup"
	"vpn-sub/internal/rulespec"
	"vpn-sub/internal/ssplugin"
)

// endpointPolicyRequirements 从协议声明的 endpoint policy 推导 self-check 是否强制 server／port。
// 只要存在一种合法状态隐藏该字段，就不把它写成 YAML 自检的硬性必填。
func endpointPolicyRequirements(proto node.Protocol) (requireHost, requirePort bool) {
	requireHost, requirePort = true, true
	for _, policy := range proto.EndpointPolicies {
		if !policy.EmitHost {
			requireHost = false
		}
		if !policy.EmitPort {
			requirePort = false
		}
	}
	return requireHost, requirePort
}

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
					proto, err := node.GetProtocol(typ)
					if err != nil {
						issues = append(issues, outputError(path, "不支持的节点类型: "+typ))
						for _, key := range []string{"server", "port"} {
							if _, exists := mapGet(proxy, key); !exists {
								issues = append(issues, outputError(path+"."+key, "节点缺少 "+key))
							}
						}
						continue
					}
					// endpoint 必填性由协议声明的 policy 推导：任一合法状态隐藏该字段即不硬性要求，
					// 具体组合由协议 adapter 与 endpoint policy 保证。
					requireHost, requirePort := endpointPolicyRequirements(proto)
					for _, key := range []string{"server", "port"} {
						if (key == "server" && !requireHost) || (key == "port" && !requirePort) {
							continue
						}
						if _, exists := mapGet(proxy, key); !exists {
							issues = append(issues, outputError(path+"."+key, "节点缺少 "+key))
						}
					}
					for _, field := range proto.FormSchema {
						if !field.Required {
							continue
						}
						value, exists := mapGet(proxy, field.Name)
						if !exists || isEmptyYAMLValue(value) {
							issues = append(issues, outputError(path+"."+field.Name, typ+" 缺少必填字段 "+field.Name))
						}
					}
					if typ == "ss" {
						issues = append(issues, checkSSPluginStructure(proxy, path)...)
					}
					issues = append(issues, checkProtocolWireShape(typ, proxy, path)...)
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

func checkSSPluginStructure(proxy gyaml.MapSlice, path string) []OutputIssue {
	var issues []OutputIssue
	for _, name := range ssplugin.KnownNames() {
		definition, _ := ssplugin.Lookup(name)
		if _, exists := mapGet(proxy, definition.StorageKey); exists {
			issues = append(issues, outputError(path+"."+definition.StorageKey, "SS 插件内部参数对象不得进入 Clash 产物"))
		}
	}

	pluginRaw, hasPlugin := mapGet(proxy, "plugin")
	optsRaw, hasOpts := mapGet(proxy, "plugin-opts")
	if !hasPlugin {
		if hasOpts {
			issues = append(issues, outputError(path+".plugin-opts", "缺少 plugin 时不得输出 plugin-opts"))
		}
		return issues
	}
	plugin, ok := pluginRaw.(string)
	if !ok || strings.TrimSpace(plugin) == "" {
		issues = append(issues, outputError(path+".plugin", "plugin 必须是非空字符串"))
		return issues
	}
	if containsUnescapedPluginParameter(plugin) {
		issues = append(issues, outputError(path+".plugin", "plugin 必须是纯插件名，不得包含 URI 参数串"))
	}

	var opts gyaml.MapSlice
	if hasOpts {
		var mapOK bool
		opts, mapOK = yamlMap(optsRaw)
		if !mapOK {
			issues = append(issues, outputError(path+".plugin-opts", "plugin-opts 必须是映射"))
			return issues
		}
	}

	definition, known := ssplugin.Lookup(plugin)
	if !known {
		for _, item := range opts {
			key, keyOK := item.Key.(string)
			if !keyOK {
				continue
			}
			if _, valueOK := item.Value.(string); !valueOK {
				issues = append(issues, outputError(path+".plugin-opts."+key, "未知 SS 插件参数必须是字符串"))
			}
		}
		return issues
	}
	clash, exists := definition.Target(ssplugin.TargetClash)
	if !exists {
		return issues
	}
	for _, field := range clash.RequiredFields {
		value, exists := mapGet(opts, field)
		if !exists || isEmptyYAMLValue(value) {
			issues = append(issues, outputError(path+".plugin-opts."+field, "SS 插件 "+plugin+" 缺少必需参数 "+field))
		}
	}
	for field, allowed := range clash.AllowedValues {
		value, exists := mapGet(opts, field)
		if !exists {
			continue
		}
		text, valueOK := value.(string)
		if !valueOK || !containsAllowedValue(allowed, text) {
			issues = append(issues, outputError(path+".plugin-opts."+field, "SS 插件 "+plugin+" 参数 "+field+" 不受 Clash 目标支持"))
		}
	}
	return issues
}

func containsUnescapedPluginParameter(value string) bool {
	escaped := false
	for _, char := range value {
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' {
			escaped = true
			continue
		}
		if char == ';' || char == '=' {
			return true
		}
	}
	return false
}

func containsAllowedValue(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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

// checkProtocolWireShape 检查后续协议在最终 YAML 中的关键 shape、互斥分支、endpoint 形状
// 与项目明确禁止的组合。它只识别结构性错误，不复制 node 侧的整套业务校验。
func checkProtocolWireShape(typ string, proxy gyaml.MapSlice, path string) []OutputIssue {
	switch typ {
	case "tailscale":
		return checkTailscaleWireShape(proxy, path)
	case "hysteria2":
		return checkHysteria2WireShape(proxy, path)
	case "mieru":
		return checkMieruWireShape(proxy, path)
	case "wireguard":
		return checkWireGuardWireShape(proxy, path)
	case "tuic":
		return checkTUICWireShape(proxy, path)
	case "trusttunnel":
		return checkTrustTunnelWireShape(proxy, path)
	case "openvpn":
		return checkOpenVPNWireShape(proxy, path)
	case "anytls":
		return checkAnyTLSWireShape(proxy, path)
	case "shadowquic":
		return checkShadowQUICWireShape(proxy, path)
	case "snell":
		return checkSnellWireShape(proxy, path)
	case "masque":
		return checkMASQUEWireShape(proxy, path)
	default:
		return nil
	}
}

// yamlHasKey 判断节点是否输出了指定键（区分「未输出」与「输出空值」）。
func yamlHasKey(proxy gyaml.MapSlice, key string) bool {
	_, exists := mapGet(proxy, key)
	return exists
}

// checkTailscaleWireShape：Tailscale 的 endpoint 由协议自身管理，不得出现顶层 server／port。
func checkTailscaleWireShape(proxy gyaml.MapSlice, path string) []OutputIssue {
	var issues []OutputIssue
	for _, key := range []string{"server", "port"} {
		if yamlHasKey(proxy, key) {
			issues = append(issues, outputError(path+"."+key, "Tailscale 由协议自身管理 endpoint，不得输出 "+key))
		}
	}
	return issues
}

// checkHysteria2WireShape：port 与 ports 严格二选一，混淆子字段必须与 obfs 同时出现。
func checkHysteria2WireShape(proxy gyaml.MapSlice, path string) []OutputIssue {
	var issues []OutputIssue
	hasPort := yamlHasKey(proxy, "port")
	hasPorts := yamlHasKey(proxy, "ports")
	switch {
	case hasPort && hasPorts:
		issues = append(issues, outputError(path+".ports", "Hysteria2 的 port 与 ports 严格二选一，不得同时输出"))
	case !hasPort && !hasPorts:
		issues = append(issues, outputError(path+".port", "Hysteria2 缺少 endpoint：必须输出 port 或 ports"))
	}
	if !yamlHasKey(proxy, "obfs") {
		for _, key := range []string{"obfs-password", "obfs-min-packet-size", "obfs-max-packet-size"} {
			if yamlHasKey(proxy, key) {
				issues = append(issues, outputError(path+"."+key, "Hysteria2 未输出 obfs 时不得输出混淆子字段 "+key))
			}
		}
	}
	return issues
}

// checkMieruWireShape：port 与 port-range 严格二选一。
func checkMieruWireShape(proxy gyaml.MapSlice, path string) []OutputIssue {
	var issues []OutputIssue
	hasPort := yamlHasKey(proxy, "port")
	hasRange := yamlHasKey(proxy, "port-range")
	switch {
	case hasPort && hasRange:
		issues = append(issues, outputError(path+".port-range", "Mieru 的 port 与 port-range 严格二选一，不得同时输出"))
	case !hasPort && !hasRange:
		issues = append(issues, outputError(path+".port", "Mieru 缺少 endpoint：必须输出 port 或 port-range"))
	}
	return issues
}

// checkWireGuardWireShape：peers 与顶层 peer 字段互斥，Peer 逐项必填，reserved 必须恰 3 字节。
func checkWireGuardWireShape(proxy gyaml.MapSlice, path string) []OutputIssue {
	var issues []OutputIssue
	peersRaw, hasPeers := mapGet(proxy, "peers")
	if hasPeers {
		for _, key := range []string{"server", "port", "public-key", "pre-shared-key", "allowed-ips"} {
			if yamlHasKey(proxy, key) {
				issues = append(issues, outputError(path+".peers", "WireGuard 输出 peers 时不得同时输出顶层 "+key))
				break
			}
		}
		peers, ok := seqOf(peersRaw)
		if !ok {
			issues = append(issues, outputError(path+".peers", "WireGuard peers 必须是列表"))
			return issues
		}
		for i, raw := range peers {
			peer, ok := yamlMap(raw)
			if !ok {
				issues = append(issues, outputError(fmt.Sprintf("%s.peers[%d]", path, i), "WireGuard Peer 必须是映射"))
				continue
			}
			for _, key := range []string{"server", "port", "public-key", "allowed-ips"} {
				value, exists := mapGet(peer, key)
				if !exists || isEmptyYAMLValue(value) {
					issues = append(issues, outputError(fmt.Sprintf("%s.peers[%d].%s", path, i, key),
						"WireGuard Peer 缺少必填字段 "+key))
				}
			}
		}
	}
	if !hasPeers {
		for _, key := range []string{"server", "port"} {
			if !yamlHasKey(proxy, key) {
				issues = append(issues, outputError(path+"."+key, "WireGuard 缺少顶层 endpoint 字段 "+key))
			}
		}
	}
	issues = append(issues, checkByteSequenceLength(proxy, path, "reserved")...)
	return issues
}

// checkByteSequenceLength 校验 reserved 必须是恰好 3 个 0-255 整数。
func checkByteSequenceLength(proxy gyaml.MapSlice, path, key string) []OutputIssue {
	raw, exists := mapGet(proxy, key)
	if !exists {
		return nil
	}
	items, ok := seqOf(raw)
	if !ok || len(items) != 3 {
		return []OutputIssue{outputError(path+"."+key, key+" 必须是恰好 3 个字节的整数数组")}
	}
	for _, item := range items {
		number, ok := yamlIntValue(item)
		if !ok || number < 0 || number > 255 {
			return []OutputIssue{outputError(path+"."+key, key+" 的每个字节必须在 0-255 之间")}
		}
	}
	return nil
}

// checkTUICWireShape：v4 Token 与 v5 UUID／密码互斥，UOT 版本依赖 UOT 开关。
func checkTUICWireShape(proxy gyaml.MapSlice, path string) []OutputIssue {
	var issues []OutputIssue
	if yamlHasKey(proxy, "token") && (yamlHasKey(proxy, "uuid") || yamlHasKey(proxy, "password")) {
		issues = append(issues, outputError(path+".token", "TUIC 的 v4 Token 与 v5 UUID／密码严格互斥，不得同时输出"))
	}
	if yamlHasKey(proxy, "udp-over-stream-version") && !mapBool(proxy, "udp-over-stream") {
		issues = append(issues, outputError(path+".udp-over-stream-version",
			"TUIC 未启用 udp-over-stream 时不得输出版本字段"))
	}
	return issues
}

// checkTrustTunnelWireShape：连接复用两组数字互斥。
func checkTrustTunnelWireShape(proxy gyaml.MapSlice, path string) []OutputIssue {
	var issues []OutputIssue
	connectionsUsed := yamlHasKey(proxy, "max-connections") || yamlHasKey(proxy, "min-streams")
	if connectionsUsed && yamlHasKey(proxy, "max-streams") {
		issues = append(issues, outputError(path+".max-streams",
			"TrustTunnel 的 max-connections／min-streams 与 max-streams 两组复用参数互斥"))
	}
	return issues
}

// checkOpenVPNWireShape：认证组成对、三种 TLS key 互斥、key-direction 依赖 tls-auth。
func checkOpenVPNWireShape(proxy gyaml.MapSlice, path string) []OutputIssue {
	var issues []OutputIssue
	for _, pair := range [][2]string{{"username", "password"}, {"cert", "key"}} {
		hasFirst := yamlHasKey(proxy, pair[0])
		hasSecond := yamlHasKey(proxy, pair[1])
		if hasFirst == hasSecond {
			continue
		}
		missing := pair[1]
		present := pair[0]
		if !hasFirst {
			missing, present = pair[0], pair[1]
		}
		issues = append(issues, outputError(path+"."+present,
			fmt.Sprintf("OpenVPN 认证组成对：输出 %s 时必须同时输出 %s", present, missing)))
	}
	firstKey := ""
	for _, key := range []string{"tls-auth", "tls-crypt", "tls-crypt-v2"} {
		if !yamlHasKey(proxy, key) {
			continue
		}
		if firstKey != "" {
			issues = append(issues, outputError(path+"."+key,
				"OpenVPN 的 tls-auth／tls-crypt／tls-crypt-v2 严格互斥，不得同时输出"))
			continue
		}
		firstKey = key
	}
	if yamlHasKey(proxy, "key-direction") && !yamlHasKey(proxy, "tls-auth") {
		issues = append(issues, outputError(path+".key-direction", "OpenVPN 的 key-direction 只能与 tls-auth 同时输出"))
	}
	return issues
}

// checkAnyTLSWireShape：三种附加伪装安全对象严格互斥。
func checkAnyTLSWireShape(proxy gyaml.MapSlice, path string) []OutputIssue {
	var issues []OutputIssue
	first := ""
	for _, key := range []string{"shadow-tls-opts", "restls-opts", "jls-opts"} {
		if !yamlHasKey(proxy, key) {
			continue
		}
		if first != "" {
			issues = append(issues, outputError(path+"."+key,
				"AnyTLS 的 shadow-tls-opts／restls-opts／jls-opts 严格互斥，不得同时输出"))
			continue
		}
		first = key
	}
	return issues
}

// checkShadowQUICWireShape：固定 tag 的 ShadowQuicOption 没有 TLS 校验、证书、ECH 与 UOT 版本字段。
func checkShadowQUICWireShape(proxy gyaml.MapSlice, path string) []OutputIssue {
	var issues []OutputIssue
	// udp-over-stream 是固定 tag 的真实字段；只拒绝 tag 中不存在的 TLS 校验、
	// 证书、ECH 与 UOT 版本字段。
	for _, key := range []string{"skip-cert-verify", "name-cert-verify", "certificate", "private-key",
		"ech-opts", "udp-over-stream-version", "udp-over-tcp", "udp-over-tcp-version"} {
		if yamlHasKey(proxy, key) {
			issues = append(issues, outputError(path+"."+key,
				"ShadowQUIC 的固定 tag option 不存在字段 "+key))
		}
	}
	return issues
}

// checkSnellWireShape：v1／v2 不得输出 udp，v1 不得输出 reuse。
func checkSnellWireShape(proxy gyaml.MapSlice, path string) []OutputIssue {
	versionRaw, exists := mapGet(proxy, "version")
	if !exists {
		return nil
	}
	version, ok := yamlIntValue(versionRaw)
	if !ok {
		return []OutputIssue{outputError(path+".version", "Snell version 必须是整数")}
	}
	var issues []OutputIssue
	if version <= 2 && yamlHasKey(proxy, "udp") {
		issues = append(issues, outputError(path+".udp", "Snell v1／v2 不支持 UDP，不得输出 udp"))
	}
	if version <= 1 && yamlHasKey(proxy, "reuse") {
		issues = append(issues, outputError(path+".reuse", "Snell v1 不支持 reuse，不得输出 reuse"))
	}
	return issues
}

// checkMASQUEWireShape：network 只接受 h2／h3-l4proxy，h3-l4proxy 不得启用 UDP。
func checkMASQUEWireShape(proxy gyaml.MapSlice, path string) []OutputIssue {
	raw, exists := mapGet(proxy, "network")
	if !exists {
		return nil
	}
	network, _ := raw.(string)
	if network != "h2" && network != "h3-l4proxy" {
		return []OutputIssue{outputError(path+".network", "MASQUE network 只接受 h2 或 h3-l4proxy")}
	}
	if network == "h3-l4proxy" && mapBool(proxy, "udp") {
		return []OutputIssue{outputError(path+".udp", "MASQUE h3-l4proxy 模式不支持 UDP，不得输出 udp: true")}
	}
	return nil
}

// yamlIntValue 把 YAML 解码得到的整数值统一读取为 int。
func yamlIntValue(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int8:
		return int(typed), true
	case int16:
		return int(typed), true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case uint:
		return int(typed), true
	case uint8:
		return int(typed), true
	case uint16:
		return int(typed), true
	case uint32:
		return int(typed), true
	case uint64:
		return int(typed), true
	case float32:
		return int(typed), true
	case float64:
		return int(typed), true
	default:
		return 0, false
	}
}
