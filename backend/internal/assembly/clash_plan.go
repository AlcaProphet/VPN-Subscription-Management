package assembly

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	gyaml "github.com/goccy/go-yaml"

	"vpn-sub/internal/node"
	"vpn-sub/internal/rulespec"
)

// ClashPlan 是 Clash 装配渲染计划的自包含结构。
// 节点引用统一使用 nodes.name 稳定键；manual_proxies 冻结完整条目，proxy_groups 冻结生成时点结构。
type ClashPlan struct {
	Head          *OrderedMap      `json:"head"`
	ManualProxies []*OrderedMap    `json:"manual_proxies"`
	ProxyGroups   []ClashPlanGroup `json:"proxy_groups"`
	Rules         []ClashPlanRule  `json:"rules"`
	Fallback      []string         `json:"fallback"`
	Overlay       OverlayInput     `json:"overlay,omitempty"`
}

// ClashPlanGroup 是渲染计划中的代理组结构。
type ClashPlanGroup struct {
	Name                string   `json:"name"`
	Type                string   `json:"type"`
	Proxies             []string `json:"proxies,omitempty"`
	Use                 []string `json:"use,omitempty"`
	URL                 string   `json:"url,omitempty"`
	ExpectedStatus      string   `json:"expected-status,omitempty"`
	Interval            int      `json:"interval,omitempty"`
	Timeout             int      `json:"timeout,omitempty"`
	MaxFailedTimes      int      `json:"max-failed-times,omitempty"`
	Lazy                bool     `json:"lazy,omitempty"`
	DisableUDP          bool     `json:"disable-udp,omitempty"`
	InterfaceName       string   `json:"interface-name,omitempty"`
	RoutingMark         int      `json:"routing-mark,omitempty"`
	Filter              string   `json:"filter,omitempty"`
	ExcludeFilter       string   `json:"exclude-filter,omitempty"`
	ExcludeType         string   `json:"exclude-type,omitempty"`
	IncludeAll          bool     `json:"include-all,omitempty"`
	IncludeAllProxies   bool     `json:"include-all-proxies,omitempty"`
	IncludeAllProviders bool     `json:"include-all-providers,omitempty"`
	Hidden              bool     `json:"hidden,omitempty"`
	Icon                string   `json:"icon,omitempty"`
	Force               bool     `json:"force,omitempty"`
}

// ClashPlanRule 是冻结后的规则行（不含 no-resolve 后缀，渲染时按类型补）。
type ClashPlanRule struct {
	Type   string `json:"type"`
	Value  string `json:"value"`
	Target string `json:"target"`
}

// DynamicNode 是下载重渲染时注入的动态 Xray 节点。
type DynamicNode struct {
	Name         string         `json:"name"`
	RenderName   string         `json:"render_name"`
	Protocol     string         `json:"protocol"`
	Host         string         `json:"host"`
	Port         int            `json:"port"`
	ProtocolJSON map[string]any `json:"protocol_json"`
}

// RenderClashPlan 根据自包含渲染计划 + 动态节点 + 名称映射，全量重渲染 Clash YAML。
// manualNames 提供 nodes.name 稳定键 → 当前 renderName 映射；缺失时回退计划内 name。
func RenderClashPlan(planRaw []byte, dynamic []DynamicNode, manualNames map[string]string, comment string) ([]byte, error) {
	var plan ClashPlan
	if err := json.Unmarshal(planRaw, &plan); err != nil {
		return nil, fmt.Errorf("解析 Clash 渲染计划失败: %w", err)
	}
	if plan.Head == nil {
		plan.Head = NewOrderedMap()
	}

	// 稳定键 → 当前渲染名
	renderNames := map[string]string{}
	for k, v := range manualNames {
		if v != "" {
			renderNames[k] = v
		}
	}
	for _, d := range dynamic {
		renderNames[d.Name] = d.RenderName
	}

	// 先组装完整基础文档：头部 + manual 节点 + 动态 Xray 节点 + 全量代理组 + 全量规则。
	root := orderedMapToMapSlice(plan.Head)

	proxyValues := make([]any, 0, len(plan.ManualProxies)+len(dynamic))
	for _, mp := range plan.ManualProxies {
		if mp == nil {
			continue
		}
		if nameVal, ok := mp.Get("name"); ok {
			if nameStr, ok := nameVal.(string); ok {
				if r, ok := renderNames[nameStr]; ok && r != "" {
					mp.Set("name", r)
				}
			}
		}
		proxyValues = append(proxyValues, orderedMapToMapSlice(mp))
	}
	for _, d := range dynamic {
		proxyValues = append(proxyValues, orderedMapToMapSlice(dynamicClashProxy(d)))
	}
	root = append(root, gyaml.MapItem{Key: "proxies", Value: proxyValues})

	groupValues := make([]any, 0, len(plan.ProxyGroups))
	for i := range plan.ProxyGroups {
		g := plan.ProxyGroups[i]
		g.Proxies = translateGroupMembers(g.Proxies, renderNames)
		groupValues = append(groupValues, orderedMapToMapSlice(orderedGroupFields(&g)))
	}
	root = append(root, gyaml.MapItem{Key: "proxy-groups", Value: groupValues})

	ruleValues := make([]any, 0, len(plan.Rules)+len(plan.Fallback))
	for _, r := range plan.Rules {
		line := r.Type + ","
		if r.Type != "MATCH" {
			line += r.Value + ","
		}
		line += r.Target
		if rulespec.Definitions[r.Type].NoResolve {
			line += ",no-resolve"
		}
		ruleValues = append(ruleValues, line)
	}
	for _, fb := range plan.Fallback {
		ruleValues = append(ruleValues, fb)
	}
	root = append(root, gyaml.MapItem{Key: "rules", Value: ruleValues})

	// 应用覆盖层：seq → merge → 控制面恢复 → 清理 → 排序，发生在可达性收敛之前。
	if err := applyClashOverlay(&root, plan.Overlay); err != nil {
		return nil, fmt.Errorf("应用覆盖层失败: %w", err)
	}

	// 基于覆盖层之后的最终文档做可达性收敛。
	allGroups := parseClashPlanGroups(&root)
	groupNames := map[string]bool{}
	for _, g := range allGroups {
		groupNames[g.Name] = true
	}
	forceNames := map[string]bool{}
	for _, g := range plan.ProxyGroups {
		if g.Force {
			forceNames[g.Name] = true
		}
	}
	reachable := map[string]bool{"DIRECT": true}
	if proxiesRaw, ok := mapGet(root, "proxies"); ok {
		if items, ok := seqOf(proxiesRaw); ok {
			for _, item := range items {
				if name := goccyNameOf(item); name != "" {
					reachable[name] = true
				}
			}
		}
	}
	providerNames := collectProviderNames(&root)

	kept := map[string]bool{}
	for _, g := range allGroups {
		if forceNames[g.Name] {
			kept[g.Name] = true
		}
	}
	changed := true
	for changed {
		changed = false
		for _, g := range allGroups {
			if kept[g.Name] {
				continue
			}
			if clashGroupReachable(g, nil, reachable, groupNames, kept, providerNames) {
				kept[g.Name] = true
				changed = true
			}
		}
	}

	finalGroups := make([]ClashPlanGroup, 0, len(allGroups))
	finalGroupSet := map[string]bool{}
	for _, g := range allGroups {
		if !kept[g.Name] {
			continue
		}
		members := make([]string, 0, len(g.Proxies))
		for _, m := range g.Proxies {
			if m == "DIRECT" {
				members = append(members, m)
				continue
			}
			if groupNames[m] {
				if kept[m] {
					members = append(members, m)
				}
				continue
			}
			if reachable[m] {
				members = append(members, m)
			}
		}
		if forceNames[g.Name] && len(members) == 0 {
			if g.Name == node.ForceDirect {
				members = []string{node.ReservedDirect}
			} else {
				members = []string{node.ForceDirect}
			}
		}
		if !forceNames[g.Name] && len(members) == 0 && !groupHasProviderOrInclude(g, providerNames) {
			continue
		}
		g.Proxies = members
		finalGroups = append(finalGroups, g)
		finalGroupSet[g.Name] = true
	}

	// 基于最终保留组重写规则目标；覆盖层引入的规则保持原样，仅对被删除组降级 DIRECT。
	ruleLines := downgradeRuleLines(rootRuleStrings(&root), finalGroupSet)
	finalGroupValues := make([]any, 0, len(finalGroups))
	for i := range finalGroups {
		finalGroupValues = append(finalGroupValues, orderedMapToMapSlice(orderedGroupFields(&finalGroups[i])))
	}
	mapSet(&root, "proxy-groups", finalGroupValues)
	mapSet(&root, "rules", ruleLines)

	comments := proxyCommentMap(comment, len(proxyValues) > 0)
	content, err := marshalClashYAML(root, comments)
	if err != nil {
		return nil, fmt.Errorf("序列化 Clash YAML 失败: %w", err)
	}
	return content, nil
}

// parseClashPlanGroups 从覆盖层后的 MapSlice 解析代理组，保留高级字段。
func parseClashPlanGroups(root *gyaml.MapSlice) []ClashPlanGroup {
	raw, ok := mapGet(*root, "proxy-groups")
	if !ok {
		return nil
	}
	items, ok := seqOf(raw)
	if !ok {
		return nil
	}
	out := make([]ClashPlanGroup, 0, len(items))
	for _, item := range items {
		m, ok := yamlMap(item)
		if !ok {
			continue
		}
		out = append(out, clashGroupFromYAML(m))
	}
	return out
}

func clashGroupFromYAML(m gyaml.MapSlice) ClashPlanGroup {
	g := ClashPlanGroup{
		Name:                mapString(m, "name"),
		Type:                mapString(m, "type"),
		URL:                 mapString(m, "url"),
		ExpectedStatus:      mapString(m, "expected-status"),
		InterfaceName:       mapString(m, "interface-name"),
		Filter:              mapString(m, "filter"),
		ExcludeFilter:       mapString(m, "exclude-filter"),
		ExcludeType:         mapString(m, "exclude-type"),
		Icon:                mapString(m, "icon"),
		Interval:            mapIntValue(m, "interval"),
		Timeout:             mapIntValue(m, "timeout"),
		MaxFailedTimes:      mapIntValue(m, "max-failed-times"),
		RoutingMark:         mapIntValue(m, "routing-mark"),
		Lazy:                mapBoolValue(m, "lazy"),
		DisableUDP:          mapBoolValue(m, "disable-udp"),
		IncludeAll:          mapBoolValue(m, "include-all"),
		IncludeAllProxies:   mapBoolValue(m, "include-all-proxies"),
		IncludeAllProviders: mapBoolValue(m, "include-all-providers"),
		Hidden:              mapBoolValue(m, "hidden"),
	}
	if value, ok := mapGet(m, "proxies"); ok {
		if items, ok := seqOf(value); ok {
			for _, item := range items {
				if s, ok := item.(string); ok {
					g.Proxies = append(g.Proxies, s)
				}
			}
		}
	}
	if value, ok := mapGet(m, "use"); ok {
		if items, ok := seqOf(value); ok {
			for _, item := range items {
				if s, ok := item.(string); ok {
					g.Use = append(g.Use, s)
				}
			}
		}
	}
	return g
}

func mapIntValue(m gyaml.MapSlice, key string) int {
	value, ok := mapGet(m, key)
	if !ok {
		return 0
	}
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case uint64:
		return int(v)
	case float64:
		return int(v)
	case string:
		var n int
		_, _ = fmt.Sscanf(v, "%d", &n)
		return n
	default:
		return 0
	}
}

func mapBoolValue(m gyaml.MapSlice, key string) bool {
	value, ok := mapGet(m, key)
	if !ok {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false
	}
}

// rootRuleStrings 返回覆盖层后规则列表中的字符串。
func rootRuleStrings(root *gyaml.MapSlice) []string {
	raw, ok := mapGet(*root, "rules")
	if !ok {
		return nil
	}
	items, ok := seqOf(raw)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// downgradeRuleLines 把目标组已被删除的规则降级为 DIRECT。
func downgradeRuleLines(lines []string, finalGroupSet map[string]bool) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		typ, value, target, noResolve, err := rulespec.ParseRendered(line)
		if err != nil {
			out = append(out, line)
			continue
		}
		if target == "" || target == "DIRECT" || finalGroupSet[target] {
			out = append(out, line)
			continue
		}
		rebuilt := typ + ","
		if typ != "MATCH" {
			rebuilt += value + ","
		}
		rebuilt += "DIRECT"
		if noResolve || rulespec.Definitions[typ].NoResolve {
			rebuilt += ",no-resolve"
		}
		out = append(out, rebuilt)
	}
	return out
}

// translateGroupMembers 将计划内稳定节点名转为当前渲染名；组名等无映射项原样保留。
func translateGroupMembers(members []string, renderNames map[string]string) []string {
	out := make([]string, 0, len(members))
	for _, m := range members {
		if r, ok := renderNames[m]; ok && r != "" {
			out = append(out, r)
			continue
		}
		out = append(out, m)
	}
	return out
}

func clashGroupReachable(g ClashPlanGroup, renderNames map[string]string, reachable, groupNames, kept, providers map[string]bool) bool {
	if groupHasProviderOrInclude(g, providers) {
		return true
	}
	for _, m := range g.Proxies {
		if m == "DIRECT" {
			return true
		}
		if groupNames[m] {
			if kept[m] {
				return true
			}
			continue
		}
		if r, ok := renderNames[m]; ok {
			if reachable[r] {
				return true
			}
			continue
		}
		if reachable[m] {
			return true
		}
	}
	return false
}

func groupHasProviderOrInclude(g ClashPlanGroup, providers map[string]bool) bool {
	if g.IncludeAll || g.IncludeAllProxies || g.IncludeAllProviders {
		return true
	}
	for _, name := range g.Use {
		if providers[name] {
			return true
		}
	}
	return false
}

func clashPlanProviderNames(head *OrderedMap) map[string]bool {
	out := map[string]bool{}
	if head == nil {
		return out
	}
	raw, ok := head.Get("proxy-providers")
	if !ok {
		return out
	}
	switch providers := raw.(type) {
	case map[string]any:
		for name := range providers {
			out[name] = true
		}
	case *OrderedMap:
		for _, name := range providers.Keys() {
			out[name] = true
		}
	}
	return out
}

// orderedGroupFields 按稳定键序输出非零代理组字段。
func orderedGroupFields(g *ClashPlanGroup) *OrderedMap {
	out := NewOrderedMap().Set("name", g.Name).Set("type", g.Type)
	if len(g.Proxies) > 0 {
		out.Set("proxies", g.Proxies)
	}
	if len(g.Use) > 0 {
		out.Set("use", g.Use)
	}
	if g.URL != "" {
		out.Set("url", g.URL)
	}
	if g.ExpectedStatus != "" {
		out.Set("expected-status", g.ExpectedStatus)
	}
	if g.Interval != 0 {
		out.Set("interval", g.Interval)
	}
	if g.Timeout != 0 {
		out.Set("timeout", g.Timeout)
	}
	if g.MaxFailedTimes != 0 {
		out.Set("max-failed-times", g.MaxFailedTimes)
	}
	if g.Lazy {
		out.Set("lazy", true)
	}
	if g.DisableUDP {
		out.Set("disable-udp", true)
	}
	if g.InterfaceName != "" {
		out.Set("interface-name", g.InterfaceName)
	}
	if g.RoutingMark != 0 {
		out.Set("routing-mark", g.RoutingMark)
	}
	if g.Filter != "" {
		out.Set("filter", g.Filter)
	}
	if g.ExcludeFilter != "" {
		out.Set("exclude-filter", g.ExcludeFilter)
	}
	if g.ExcludeType != "" {
		out.Set("exclude-type", g.ExcludeType)
	}
	if g.IncludeAll {
		out.Set("include-all", true)
	}
	if g.IncludeAllProxies {
		out.Set("include-all-proxies", true)
	}
	if g.IncludeAllProviders {
		out.Set("include-all-providers", true)
	}
	if g.Hidden {
		out.Set("hidden", true)
	}
	if g.Icon != "" {
		out.Set("icon", g.Icon)
	}
	return out
}

func dynamicClashProxy(d DynamicNode) *OrderedMap {
	p := NewOrderedMap()
	p.Set("name", d.RenderName)
	p.Set("type", clashProtocolName(d.Protocol))
	p.Set("server", d.Host)
	p.Set("port", d.Port)
	params := normalizeClashFields(clashProtocolName(d.Protocol), d.ProtocolJSON)
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "name" || k == "type" || k == "server" || k == "port" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		p.Set(k, params[k])
	}
	return p
}

// clashProtocolName 将检测/渲染使用的协议名映射为 Clash/Mihomo 配置中的类型名。
func clashProtocolName(protocol string) string {
	if protocol == "shadowsocks" {
		return "ss"
	}
	return protocol
}
