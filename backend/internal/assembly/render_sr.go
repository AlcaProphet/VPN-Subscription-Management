package assembly

import (
	"encoding/json"
	"fmt"
	"strings"

	"vpn-sub/internal/rulespec"
)

// renderSrSubs 渲染 SR 节点订阅（带 STATUS/REMARKS 头）或通用节点订阅（无头）。
func (s *Service) renderSrSubs(in GenerateInput, ld *loadedData, sr bool) (*RenderResult, error) {
	var b strings.Builder
	if sr {
		status := ""
		if v, ok := in.FixedParams.Get("status"); ok {
			status, _ = v.(string)
		}
		remarks := ""
		if v, ok := in.FixedParams.Get("remarks"); ok {
			remarks, _ = v.(string)
		}
		if status == "" {
			status = "2026/01/01 Version"
		}
		if remarks == "" {
			remarks = "VPN Subscription"
		}
		b.WriteString("STATUS=")
		b.WriteString(status)
		b.WriteString("\n")
		b.WriteString("REMARKS=")
		b.WriteString(remarks)
		b.WriteString("\n")
	}
	skipped := []SkipItem{}
	for _, name := range in.NodeNames {
		nd := ld.nodes[name]
		var link string
		var err error
		if sr {
			link, err = srLink(nd)
		} else {
			link, err = genericLink(nd)
		}
		if err != nil {
			skipped = append(skipped, SkipItem{Kind: "node", Name: nd.Name, Reason: err.Error()})
			continue
		}
		b.WriteString(link)
		b.WriteString("\n")
	}
	if hasXrayNode(ld) {
		b.WriteString("# {{xray_nodes}}\n")
	}
	content := []byte(b.String())
	plan, err := json.Marshal(map[string]any{"node_names": in.NodeNames})
	if err != nil {
		return nil, fmt.Errorf("序列化 SR 订阅渲染计划失败: %w", err)
	}
	return &RenderResult{Content: content, Skipped: skipped, RenderPlan: plan}, nil
}

// renderSrConf 渲染 SR 分流规则。
func (s *Service) renderSrConf(in GenerateInput, ld *loadedData) (*RenderResult, error) {
	var b strings.Builder
	b.WriteString("[General]\n")
	for _, k := range in.FixedParams.Keys() {
		v, _ := in.FixedParams.Get(k)
		b.WriteString(k)
		b.WriteString(" = ")
		b.WriteString(fmt.Sprint(v))
		b.WriteString("\n")
	}
	b.WriteString("\n[Rule]\n")
	skipped := []SkipItem{}
	appendRule := func(ruleType, value, target string) {
		typ, normalized, err := rulespec.ValidateValue(ruleType, value)
		if err != nil {
			skipped = append(skipped, SkipItem{Kind: "rule", Name: ruleType + "," + value, Reason: err.Error()})
			return
		}
		if !rulespec.Definitions[typ].SR {
			skipped = append(skipped, SkipItem{Kind: "rule", Name: typ + "," + normalized, Reason: "Shadowrocket 不支持该规则类型"})
			return
		}
		b.WriteString(formatRuleLine(typ, normalized, target))
		b.WriteString("\n")
	}
	for _, psel := range in.Pools {
		for _, e := range ld.pools[psel.PoolID] {
			appendRule(e.RuleType, e.MatchValue, psel.Target)
		}
	}
	for _, r := range in.CustomRules {
		appendRule(r.RuleType, r.MatchValue, r.Target)
	}
	b.WriteString("GEOIP,CN,DIRECT\n")
	final := in.FinalDirection
	if final == "" {
		final = "PROXY"
	}
	if final != "PROXY" && final != "DIRECT" {
		final = "PROXY"
	}
	b.WriteString("FINAL,")
	b.WriteString(final)
	b.WriteString("\n")
	content := []byte(b.String())
	plan, err := json.Marshal(map[string]any{"final_direction": final})
	if err != nil {
		return nil, fmt.Errorf("序列化 SR 分流渲染计划失败: %w", err)
	}
	return &RenderResult{Content: content, Skipped: skipped, RenderPlan: plan}, nil
}

// formatRuleLine 生成规则行；IP-CIDR/IP-CIDR6 追加 no-resolve。
func formatRuleLine(ruleType, value, target string) string {
	line := ruleType + "," + value + "," + target
	if ruleType == "IP-CIDR" || ruleType == "IP-CIDR6" {
		line += ",no-resolve"
	}
	return line
}
