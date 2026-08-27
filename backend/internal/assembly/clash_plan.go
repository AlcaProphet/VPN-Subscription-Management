package assembly

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	gyaml "github.com/goccy/go-yaml"

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
	providerNames := clashPlanProviderNames(plan.Head)

	// proxies：manual 完整条目 + 动态节点
	proxies := make([]*OrderedMap, 0, len(plan.ManualProxies)+len(dynamic))
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
		proxies = append(proxies, mp)
	}
	for _, d := range dynamic {
		proxies = append(proxies, dynamicClashProxy(d))
	}

	// 可达集合：DIRECT + 所有最终 proxies 的渲染名
	reachable := map[string]bool{"DIRECT": true}
	for _, p := range proxies {
		if v, ok := p.Get("name"); ok {
			if s, ok := v.(string); ok && s != "" {
				reachable[s] = true
			}
		}
	}

	// 组名称集合
	groupNames := map[string]bool{}
	for _, g := range plan.ProxyGroups {
		groupNames[g.Name] = true
	}

	// 强制组始终保留；普通组按可达性迭代收敛
	kept := map[string]bool{}
	for _, g := range plan.ProxyGroups {
		if g.Force {
			kept[g.Name] = true
		}
	}
	changed := true
	for changed {
		changed = false
		for _, g := range plan.ProxyGroups {
			if kept[g.Name] {
				continue
			}
			if clashGroupReachable(g, renderNames, reachable, groupNames, kept, providerNames) {
				kept[g.Name] = true
				changed = true
			}
		}
	}

	// 生成最终组列表（保持计划顺序）
	finalGroups := make([]ClashPlanGroup, 0, len(plan.ProxyGroups))
	finalGroupSet := map[string]bool{}
	for _, g := range plan.ProxyGroups {
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
			if r, ok := renderNames[m]; ok {
				if reachable[r] {
					members = append(members, r)
				}
				continue
			}
			// 计划内 manual 节点即使当前 DB 已删除，仍按计划名称保留可达
			if reachable[m] {
				members = append(members, m)
			}
		}
		if g.Force && len(members) == 0 {
			members = []string{"DIRECT"}
		}
		if !g.Force && len(members) == 0 && !groupHasProviderOrInclude(g, providerNames) {
			continue // 普通组完全不可达则删除
		}
		g.Proxies = members
		finalGroups = append(finalGroups, g)
		finalGroupSet[g.Name] = true
	}

	// rules：被删除组目标降级 DIRECT
	ruleLines := make([]string, 0, len(plan.Rules)+len(plan.Fallback))
	for _, r := range plan.Rules {
		target := r.Target
		if target != "" && target != "DIRECT" && !finalGroupSet[target] {
			target = "DIRECT"
		}
		line := r.Type + ","
		if r.Type != "MATCH" {
			line += r.Value + ","
		}
		line += target
		if rulespec.Definitions[r.Type].NoResolve {
			line += ",no-resolve"
		}
		ruleLines = append(ruleLines, line)
	}
	for _, fb := range plan.Fallback {
		line := fb
		for gName := range groupNames {
			if !finalGroupSet[gName] && strings.Contains(line, gName) {
				line = strings.Replace(line, gName, "DIRECT", 1)
			}
		}
		ruleLines = append(ruleLines, line)
	}

	// 组装 YAML
	root := orderedMapToMapSlice(plan.Head)

	proxyValues := make([]any, 0, len(proxies))
	for _, p := range proxies {
		proxyValues = append(proxyValues, orderedMapToMapSlice(p))
	}
	root = append(root, gyaml.MapItem{Key: "proxies", Value: proxyValues})
	comments := proxyCommentMap(comment, len(proxyValues) > 0)

	groupValues := make([]any, 0, len(finalGroups))
	for _, g := range finalGroups {
		groupValues = append(groupValues, orderedMapToMapSlice(orderedGroupFields(&g)))
	}
	root = append(root, gyaml.MapItem{Key: "proxy-groups", Value: groupValues})

	ruleValues := make([]any, len(ruleLines))
	for i, line := range ruleLines {
		ruleValues[i] = line
	}
	root = append(root, gyaml.MapItem{Key: "rules", Value: ruleValues})

	content, err := marshalClashYAML(root, comments)
	if err != nil {
		return nil, fmt.Errorf("序列化 Clash YAML 失败: %w", err)
	}
	return content, nil
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
