package assembly

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	gyaml "github.com/goccy/go-yaml"

	"vpn-sub/internal/node"
	"vpn-sub/internal/rulespec"
)

// renderClash 渲染 Clash YAML 产物。
func (s *Service) renderClash(in GenerateInput, ld *loadedData) (*RenderResult, error) {
	root := orderedMapToMapSlice(in.FixedParams)
	// proxies：manual 节点按勾选顺序输出
	proxies := make([]any, 0, len(in.NodeNames))
	for _, name := range in.NodeNames {
		nd := ld.nodes[name]
		if nd.Source != "manual" {
			continue
		}
		proxies = append(proxies, orderedMapToMapSlice(s.clashProxy(nd)))
	}
	root = append(root, gyaml.MapItem{Key: "proxies", Value: proxies})
	comments := gyaml.CommentMap(nil)
	if hasXrayNode(ld) {
		comments = proxyCommentMap("# {{xray_nodes}}", len(proxies) > 0)
	}
	// proxy-groups
	groups := make([]any, 0, len(in.GroupNames)+3)
	// 三个强制组（固定键序 name/type/proxies，R14-02）
	forceGroups := []*OrderedMap{
		orderedGroupFields(&ClashPlanGroup{Name: node.ForceDirect, Type: "select", Proxies: []string{"DIRECT"}}),
		orderedGroupFields(&ClashPlanGroup{Name: node.ForceOverseas, Type: "select", Proxies: s.forceMemberRenderNames(in.OverseasMembers, ld)}),
		orderedGroupFields(&ClashPlanGroup{Name: node.ForceFallback, Type: "select", Proxies: s.forceMemberRenderNames(in.FallbackGroupMembers, ld)}),
	}
	for _, g := range forceGroups {
		groups = append(groups, orderedMapToMapSlice(g))
	}
	// 勾选代理组（固定键序 name/type/proxies）
	for _, name := range in.GroupNames {
		g := ld.groups[name]
		rawMembers := clashGroupMemberOrder(in, g)
		proxies := make([]string, 0, len(rawMembers))
		for _, ref := range rawMembers {
			if nd, ok := ld.nodes[ref]; ok {
				proxies = append(proxies, nd.RenderName)
			} else {
				proxies = append(proxies, ref)
			}
		}
		planGroup := clashPlanGroupFromData(g, proxies)
		groups = append(groups, orderedMapToMapSlice(orderedGroupFields(&planGroup)))
	}
	root = append(root, gyaml.MapItem{Key: "proxy-groups", Value: groups})
	// rules
	rules := make([]any, 0)
	skipped := []SkipItem{}
	appendRule := func(ruleType, value, target string) {
		typ, normalized, err := rulespec.ValidateValue(ruleType, value)
		if err != nil {
			return
		}
		mapped := rulespec.SupportsAndMapLegacy(typ, rulespec.TargetClash)
		if !mapped.Supported {
			skipped = append(skipped, SkipItem{Kind: "rule", Name: ruleType + "," + value, Reason: "Clash 不支持该规则类型"})
			return
		}
		line := mapped.RenderType + ","
		if mapped.RenderType != "MATCH" {
			line += normalized + ","
		}
		line += target
		if mapped.SupportsNoResolve {
			line += ",no-resolve"
		}
		rules = append(rules, line)
	}
	for _, psel := range in.Pools {
		entries := ld.pools[psel.PoolID]
		for _, e := range entries {
			appendRule(e.RuleType, e.MatchValue, psel.Target)
		}
	}
	for _, r := range in.CustomRules {
		appendRule(r.RuleType, r.MatchValue, r.Target)
	}
	appendRule("GEOIP", "CN", "DIRECT")
	appendRule("MATCH", "", node.ForceFallback)
	root = append(root, gyaml.MapItem{Key: "rules", Value: rules})
	if hasXrayNode(ld) {
		xrayNames := make([]any, 0)
		for _, name := range in.NodeNames {
			if nd := ld.nodes[name]; nd != nil && nd.Source == "xray" {
				xrayNames = append(xrayNames, nd.RenderName)
			}
		}
		root = append(root, gyaml.MapItem{Key: overlayXrayNamesKey, Value: xrayNames})
	}
	if err := applyClashOverlay(&root, in.Overlay); err != nil {
		return nil, fmt.Errorf("应用覆盖层失败: %w", err)
	}
	if issues := checkForceGroupInvariants(root); HasError(issues) {
		for _, issue := range issues {
			if issue.Severity == "error" {
				return nil, fmt.Errorf("%w: %s", ErrBadRequest, issue.Message)
			}
		}
	}
	mapDelete(&root, overlayXrayNamesKey)
	content, err := marshalClashYAML(root, comments)
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
				ClashPlanGroup{Name: node.ForceFallback, Type: "select", Proxies: append([]string{}, in.FallbackGroupMembers...), Force: true},
			)
			for _, name := range in.GroupNames {
				g := ld.groups[name]
				proxies := clashGroupMemberOrder(in, g)
				out = append(out, clashPlanGroupFromData(g, proxies))
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
					typ, value, err := rulespec.ValidateValue(e.RuleType, e.MatchValue)
					if err == nil {
						out = append(out, ClashPlanRule{Type: typ, Value: value, Target: psel.Target})
					}
				}
			}
			for _, r := range in.CustomRules {
				if r.RuleType == "USER-AGENT" {
					continue
				}
				typ, value, err := rulespec.ValidateValue(r.RuleType, r.MatchValue)
				if err == nil {
					out = append(out, ClashPlanRule{Type: typ, Value: value, Target: r.Target})
				}
			}
			return out
		}(),
		Fallback: []string{"GEOIP,CN,DIRECT", "MATCH," + node.ForceFallback},
		Overlay:  in.Overlay,
	}
	planRaw, err := json.Marshal(plan)
	if err != nil {
		return nil, fmt.Errorf("序列化 Clash 渲染计划失败: %w", err)
	}
	return &RenderResult{Content: content, Skipped: skipped, RenderPlan: planRaw, Issues: CheckClashContent(content)}, nil
}

func clashPlanGroupFromData(g *groupData, proxies []string) ClashPlanGroup {
	return ClashPlanGroup{
		Name: g.Name, Type: g.GroupType, Proxies: proxies, Use: append([]string(nil), g.Use...),
		URL: g.URL, ExpectedStatus: g.ExpectedStatus, Interval: g.Interval, Timeout: g.Timeout,
		MaxFailedTimes: g.MaxFailedTimes, Lazy: g.Lazy, DisableUDP: g.DisableUDP,
		InterfaceName: g.InterfaceName, RoutingMark: g.RoutingMark, Filter: g.Filter,
		ExcludeFilter: g.ExcludeFilter, ExcludeType: g.ExcludeType, IncludeAll: g.IncludeAll,
		IncludeAllProxies: g.IncludeAllProxies, IncludeAllProviders: g.IncludeAllProviders,
		Hidden: g.Hidden, Icon: g.Icon,
	}
}

// clashProxy 构造 Clash proxies 条目（固定 name/type/server/port 在前，其余协议字段按键名排序，保证产物键序稳定）。
func (s *Service) clashProxy(nd *nodeData) *OrderedMap {
	p := NewOrderedMap()
	p.Set("name", nd.RenderName)
	p.Set("type", nd.Protocol)
	p.Set("server", nd.Host)
	p.Set("port", nd.Port)
	params := normalizeClashFields(nd.Protocol, nd.ProtocolJSON)
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

// normalizeClashFields 把表单中的逗号列表转换为 mihomo 原生数组。
func normalizeClashFields(protocol string, params map[string]any) map[string]any {
	out := make(map[string]any, len(params))
	for key, value := range params {
		out[key] = value
	}
	proto, err := node.GetProtocol(clashProtocolName(protocol))
	if err != nil {
		return out
	}
	for _, schema := range proto.FormSchema {
		text, ok := out[schema.Name].(string)
		if !ok || strings.TrimSpace(text) == "" {
			continue
		}
		parts := splitList(text)
		switch schema.Type {
		case "text-list":
			out[schema.Name] = parts
		case "int-list":
			values := make([]int, 0, len(parts))
			valid := true
			for _, part := range parts {
				value, err := strconv.Atoi(part)
				if err != nil {
					valid = false
					break
				}
				values = append(values, value)
			}
			if valid {
				out[schema.Name] = values
			}
		}
	}
	return out
}

func splitList(value string) []string {
	raw := strings.Split(value, ",")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// forceMemberRenderNames 将强制组成员中的节点稳定键转为渲染名，系统组名原样保留。
func (s *Service) forceMemberRenderNames(members []string, ld *loadedData) []string {
	out := make([]string, 0, len(members))
	for _, name := range members {
		if name == node.ForceDirect || name == node.ForceOverseas {
			out = append(out, name)
			continue
		}
		if nd, ok := ld.nodes[name]; ok {
			out = append(out, nd.RenderName)
		}
	}
	return out
}

// clashGroupMemberOrder 返回普通代理组的原始成员顺序（节点稳定键 + 默认携带子组名）。
// 优先使用显式 group_member_orders，否则兼容旧数据按节点顺序 + 默认子组。
func clashGroupMemberOrder(in GenerateInput, g *groupData) []string {
	if orders, ok := in.GroupMemberOrders[g.Name]; ok {
		return orders
	}
	nodes := in.GroupNodeOrders[g.Name]
	out := make([]string, 0, len(nodes)+len(g.Groups))
	out = append(out, nodes...)
	out = append(out, g.Groups...)
	return out
}
