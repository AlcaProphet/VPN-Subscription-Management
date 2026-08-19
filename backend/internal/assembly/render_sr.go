package assembly

import (
	"encoding/json"
	"fmt"
	"strings"

	"vpn-sub/internal/node"
)

// renderSrSubs 渲染 SR 节点订阅（带 STATUS/REMARKS 头）或通用节点订阅（无头）。
func (s *Service) renderSrSubs(in GenerateInput, ld *loadedData, sr bool) (*RenderResult, error) {
	var b strings.Builder
	if sr {
		status, _ := in.FixedParams["status"].(string)
		remarks, _ := in.FixedParams["remarks"].(string)
		if status == "" {
			status = "2026/01/01 Version"
		}
		if remarks == "" {
			remarks = "VPN Subscription"
		}
		b.WriteString("STATUS=" + status + "\n")
		b.WriteString("REMARKS=" + remarks + "\n")
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
		b.WriteString(link + "\n")
	}
	if hasXrayNode(ld) {
		b.WriteString("# {{xray_nodes}}\n")
	}
	content := []byte(b.String())
	plan, _ := json.Marshal(map[string]any{"node_names": in.NodeNames})
	return &RenderResult{Content: content, Skipped: skipped, RenderPlan: plan}, nil
}

// renderSrConf 渲染 SR 分流规则。
func (s *Service) renderSrConf(in GenerateInput, ld *loadedData) (*RenderResult, error) {
	var b strings.Builder
	b.WriteString("[General]\n")
	for k, v := range in.FixedParams {
		b.WriteString(k + " = " + fmt.Sprint(v) + "\n")
	}
	b.WriteString("\n[Rule]\n")
	for _, psel := range in.Pools {
		for _, e := range ld.pools[psel.PoolID] {
			b.WriteString(formatRuleLine(e.RuleType, e.MatchValue, psel.Target, true) + "\n")
		}
	}
	for _, r := range in.CustomRules {
		b.WriteString(formatRuleLine(r.RuleType, r.MatchValue, r.Target, true) + "\n")
	}
	b.WriteString("GEOIP,CN,DIRECT\n")
	final := in.FinalDirection
	if final == "" {
		final = "PROXY"
	}
	if final != "PROXY" && final != "DIRECT" {
		final = "PROXY"
	}
	b.WriteString("FINAL," + final + "\n")
	content := []byte(b.String())
	plan, _ := json.Marshal(map[string]any{"final_direction": final})
	return &RenderResult{Content: content, Skipped: nil, RenderPlan: plan}, nil
}

// formatRuleLine 生成规则行；sr=true 时保留 USER-AGENT 并给 IP 加 no-resolve。
func formatRuleLine(ruleType, value, target string, sr bool) string {
	line := ruleType + "," + value + "," + target
	if ruleType == "IP-CIDR" || ruleType == "IP-CIDR6" {
		line += ",no-resolve"
	}
	return line
}

// ensure validRuleTypes imported in this file? validate uses it; this file not.
var _ = node.ForceDirect
