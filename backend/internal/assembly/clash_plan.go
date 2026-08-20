package assembly

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
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
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Proxies []string `json:"proxies"`
	Force   bool     `json:"force,omitempty"`
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
			if clashGroupReachable(g, renderNames, reachable, groupNames, kept) {
				kept[g.Name] = true
				changed = true
			}
		}
	}

	// 生成最终组列表（保持计划顺序）
	type finalGroup struct {
		name    string
		typ     string
		members []string
		force   bool
	}
	finalGroups := make([]finalGroup, 0, len(plan.ProxyGroups))
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
		if !g.Force && len(members) == 0 {
			continue // 普通组完全不可达则删除
		}
		finalGroups = append(finalGroups, finalGroup{name: g.Name, typ: g.Type, members: members, force: g.Force})
		finalGroupSet[g.Name] = true
	}

	// rules：被删除组目标降级 DIRECT
	ruleLines := make([]string, 0, len(plan.Rules)+len(plan.Fallback))
	for _, r := range plan.Rules {
		target := r.Target
		if target != "" && target != "DIRECT" && !finalGroupSet[target] {
			target = "DIRECT"
		}
		line := r.Type + "," + r.Value
		if target != "" {
			line += "," + target
		}
		if r.Type == "IP-CIDR" || r.Type == "IP-CIDR6" {
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
	root := &yaml.Node{Kind: yaml.MappingNode}
	for _, k := range plan.Head.Keys() {
		v, _ := plan.Head.Get(k)
		valNode, err := toYAMLNode(v)
		if err != nil {
			return nil, err
		}
		root.Content = append(root.Content, scalarNode(k), valNode)
	}

	proxiesNode := &yaml.Node{Kind: yaml.SequenceNode}
	for i, p := range proxies {
		pNode, err := toYAMLNode(p)
		if err != nil {
			return nil, err
		}
		if comment != "" && i == 0 {
			pNode.HeadComment = comment
		}
		proxiesNode.Content = append(proxiesNode.Content, pNode)
	}
	if comment != "" && len(proxies) == 0 {
		proxiesNode.HeadComment = comment
	}
	root.Content = append(root.Content, scalarNode("proxies"), proxiesNode)

	groupsNode := &yaml.Node{Kind: yaml.SequenceNode}
	for _, g := range finalGroups {
		gn, err := toYAMLNode(NewOrderedMap().Set("name", g.name).Set("type", g.typ).Set("proxies", g.members))
		if err != nil {
			return nil, err
		}
		groupsNode.Content = append(groupsNode.Content, gn)
	}
	root.Content = append(root.Content, scalarNode("proxy-groups"), groupsNode)

	rulesNode := &yaml.Node{Kind: yaml.SequenceNode}
	for _, line := range ruleLines {
		rulesNode.Content = append(rulesNode.Content, scalarNode(line))
	}
	root.Content = append(root.Content, scalarNode("rules"), rulesNode)

	content, err := yaml.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("序列化 Clash YAML 失败: %w", err)
	}
	return content, nil
}

func clashGroupReachable(g ClashPlanGroup, renderNames map[string]string, reachable, groupNames, kept map[string]bool) bool {
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

func dynamicClashProxy(d DynamicNode) *OrderedMap {
	p := NewOrderedMap()
	p.Set("name", d.RenderName)
	p.Set("type", d.Protocol)
	p.Set("server", d.Host)
	p.Set("port", d.Port)
	keys := make([]string, 0, len(d.ProtocolJSON))
	for k := range d.ProtocolJSON {
		if k == "name" || k == "type" || k == "server" || k == "port" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		p.Set(k, d.ProtocolJSON[k])
	}
	return p
}
