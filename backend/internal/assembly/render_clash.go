package assembly

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"gopkg.in/yaml.v3"

	"vpn-sub/internal/node"
)

// renderClash 渲染 Clash YAML 产物。
func (s *Service) renderClash(in GenerateInput, ld *loadedData) (*RenderResult, error) {
	root := &yaml.Node{Kind: yaml.MappingNode}
	// 头部表单值（按管理员填写顺序输出）
	for _, k := range in.FixedParams.Keys() {
		v, _ := in.FixedParams.Get(k)
		valNode, err := toYAMLNode(v)
		if err != nil {
			return nil, err
		}
		root.Content = append(root.Content, scalarNode(k), valNode)
	}
	// proxies：manual 节点按勾选顺序输出
	proxiesNode := &yaml.Node{Kind: yaml.SequenceNode}
	first := true
	for _, name := range in.NodeNames {
		nd := ld.nodes[name]
		if nd.Source != "manual" {
			continue
		}
		p := s.clashProxy(nd)
		pNode, err := toYAMLNode(p)
		if err != nil {
			return nil, err
		}
		if hasXrayNode(ld) && first {
			pNode.HeadComment = "# {{xray_nodes}}"
			first = false
		}
		proxiesNode.Content = append(proxiesNode.Content, pNode)
	}
	if hasXrayNode(ld) && len(proxiesNode.Content) == 0 {
		// 无 manual 节点时仍保留占位注释（作为 proxies 区的引导行）
		proxiesNode.HeadComment = "# {{xray_nodes}}"
	}
	root.Content = append(root.Content, scalarNode("proxies"), proxiesNode)
	// proxy-groups
	groupsNode := &yaml.Node{Kind: yaml.SequenceNode}
	// 三个强制组（固定键序 name/type/proxies，R14-02）
	forceGroups := []*OrderedMap{
		NewOrderedMap().Set("name", node.ForceDirect).Set("type", "select").Set("proxies", []string{"DIRECT"}),
		NewOrderedMap().Set("name", node.ForceOverseas).Set("type", "select").Set("proxies", s.overseasRenderNames(in, ld)),
		NewOrderedMap().Set("name", node.ForceFallback).Set("type", "select").Set("proxies", []string{node.ForceDirect, node.ForceOverseas}),
	}
	for _, g := range forceGroups {
		gn, err := toYAMLNode(g)
		if err != nil {
			return nil, err
		}
		groupsNode.Content = append(groupsNode.Content, gn)
	}
	// 勾选代理组（固定键序 name/type/proxies）
	for _, name := range in.GroupNames {
		g := ld.groups[name]
		proxies := make([]string, 0, len(g.Nodes)+len(g.Groups))
		for _, ref := range g.Nodes {
			if nd, ok := ld.nodes[ref]; ok {
				proxies = append(proxies, nd.RenderName)
			}
		}
		proxies = append(proxies, g.Groups...)
		gn, err := toYAMLNode(NewOrderedMap().Set("name", g.Name).Set("type", g.GroupType).Set("proxies", proxies))
		if err != nil {
			return nil, err
		}
		groupsNode.Content = append(groupsNode.Content, gn)
	}
	root.Content = append(root.Content, scalarNode("proxy-groups"), groupsNode)
	// rules
	rulesNode := &yaml.Node{Kind: yaml.SequenceNode}
	skipped := []SkipItem{}
	appendRule := func(ruleType, value, target string) {
		line := ruleType + "," + value
		if target != "" {
			line += "," + target
		}
		if ruleType == "IP-CIDR" || ruleType == "IP-CIDR6" {
			line += ",no-resolve"
		}
		rulesNode.Content = append(rulesNode.Content, scalarNode(line))
	}
	for _, psel := range in.Pools {
		entries := ld.pools[psel.PoolID]
		for _, e := range entries {
			if e.RuleType == "USER-AGENT" {
				skipped = append(skipped, SkipItem{Kind: "rule", Name: e.MatchValue, Reason: "Clash 不支持 USER-AGENT 规则"})
				continue
			}
			appendRule(e.RuleType, e.MatchValue, psel.Target)
		}
	}
	for _, r := range in.CustomRules {
		if r.RuleType == "USER-AGENT" {
			skipped = append(skipped, SkipItem{Kind: "rule", Name: r.MatchValue, Reason: "Clash 不支持 USER-AGENT 规则"})
			continue
		}
		appendRule(r.RuleType, r.MatchValue, r.Target)
	}
	appendRule("GEOIP", "CN", "DIRECT")
	appendRule("MATCH", node.ForceFallback, "")
	root.Content = append(root.Content, scalarNode("rules"), rulesNode)
	content, err := yaml.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("序列化 Clash YAML 失败: %w", err)
	}
	plan := ClashPlan{
		Head: in.FixedParams,
		ManualProxies: func() []*OrderedMap {
			out := make([]*OrderedMap, 0, len(in.NodeNames))
			for _, name := range in.NodeNames {
				nd := ld.nodes[name]
				if nd.Source != "manual" {
					continue
				}
				p := s.clashProxy(nd)
				// 计划内节点引用统一存 nodes.name 稳定键，渲染时再映射当前 renderName。
				p.Set("name", nd.Name)
				out = append(out, p)
			}
			return out
		}(),
		ProxyGroups: func() []ClashPlanGroup {
			out := make([]ClashPlanGroup, 0, len(in.GroupNames)+3)
			out = append(out,
				ClashPlanGroup{Name: node.ForceDirect, Type: "select", Proxies: []string{"DIRECT"}, Force: true},
				ClashPlanGroup{Name: node.ForceOverseas, Type: "select", Proxies: append([]string{}, in.OverseasMembers...), Force: true},
				ClashPlanGroup{Name: node.ForceFallback, Type: "select", Proxies: []string{node.ForceDirect, node.ForceOverseas}, Force: true},
			)
			for _, name := range in.GroupNames {
				g := ld.groups[name]
				proxies := make([]string, 0, len(g.Nodes)+len(g.Groups))
				proxies = append(proxies, g.Nodes...)
				proxies = append(proxies, g.Groups...)
				out = append(out, ClashPlanGroup{Name: g.Name, Type: g.GroupType, Proxies: proxies, Force: false})
			}
			return out
		}(),
		Rules: func() []ClashPlanRule {
			out := make([]ClashPlanRule, 0)
			for _, psel := range in.Pools {
				for _, e := range ld.pools[psel.PoolID] {
					if e.RuleType == "USER-AGENT" {
						continue
					}
					out = append(out, ClashPlanRule{Type: e.RuleType, Value: e.MatchValue, Target: psel.Target})
				}
			}
			for _, r := range in.CustomRules {
				if r.RuleType == "USER-AGENT" {
					continue
				}
				out = append(out, ClashPlanRule{Type: r.RuleType, Value: r.MatchValue, Target: r.Target})
			}
			return out
		}(),
		Fallback: []string{"GEOIP,CN,DIRECT", "MATCH," + node.ForceFallback},
	}
	planRaw, err := json.Marshal(plan)
	if err != nil {
		return nil, fmt.Errorf("序列化 Clash 渲染计划失败: %w", err)
	}
	return &RenderResult{Content: content, Skipped: skipped, RenderPlan: planRaw}, nil
}

// clashProxy 构造 Clash proxies 条目（固定 name/type/server/port 在前，其余协议字段按键名排序，保证产物键序稳定）。
func (s *Service) clashProxy(nd *nodeData) *OrderedMap {
	p := NewOrderedMap()
	p.Set("name", nd.RenderName)
	p.Set("type", nd.Protocol)
	p.Set("server", nd.Host)
	p.Set("port", nd.Port)
	keys := make([]string, 0, len(nd.ProtocolJSON))
	for k := range nd.ProtocolJSON {
		if k == "name" || k == "type" || k == "server" || k == "port" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		p.Set(k, nd.ProtocolJSON[k])
	}
	return p
}

// overseasRenderNames 将 🌎国外流量 成员（nodes.name 稳定键）转为渲染名。
func (s *Service) overseasRenderNames(in GenerateInput, ld *loadedData) []string {
	out := make([]string, 0, len(in.OverseasMembers))
	for _, name := range in.OverseasMembers {
		if nd, ok := ld.nodes[name]; ok {
			out = append(out, nd.RenderName)
		}
	}
	return out
}

func scalarNode(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: v}
}

func toYAMLNode(v any) (*yaml.Node, error) {
	switch val := v.(type) {
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: val}, nil
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: strconv.FormatBool(val)}, nil
	case int:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: strconv.Itoa(val)}, nil
	case int64:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: strconv.FormatInt(val, 10)}, nil
	case float64:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: strconv.FormatFloat(val, 'g', -1, 64)}, nil
	case []string:
		n := &yaml.Node{Kind: yaml.SequenceNode}
		for _, s := range val {
			n.Content = append(n.Content, scalarNode(s))
		}
		return n, nil
	case []any:
		n := &yaml.Node{Kind: yaml.SequenceNode}
		for _, item := range val {
			child, err := toYAMLNode(item)
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, child)
		}
		return n, nil
	case *OrderedMap:
		n := &yaml.Node{Kind: yaml.MappingNode}
		for _, k := range val.Keys() {
			item, _ := val.Get(k)
			child, err := toYAMLNode(item)
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, scalarNode(k), child)
		}
		return n, nil
	case map[string]any:
		n := &yaml.Node{Kind: yaml.MappingNode}
		for k, item := range val {
			child, err := toYAMLNode(item)
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, scalarNode(k), child)
		}
		return n, nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Value: string(b)}, nil
	}
}
