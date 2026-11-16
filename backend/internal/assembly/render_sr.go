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
	diagnostics := []NodeDiagnostic{}
	outputCount := 0
	target := "generic-subs"
	if sr {
		target = "sr-subs"
	}
	for _, name := range in.NodeNames {
		nd := ld.nodes[name]
		nodeDiags := s.diagnoseNodeForTarget(target, nd)
		diagnostics = append(diagnostics, nodeDiags...)
		if hasBlockingNodeDiagnostic(nodeDiags) {
			skipped = append(skipped, SkipItem{Kind: "node", Name: nd.Name, Reason: firstNodeDiagnosticMessage(nodeDiags)})
			continue
		}
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
		outputCount++
	}
	if outputCount == 0 {
		return nil, fmt.Errorf("%w: 节点订阅至少需要 1 个可转换链接的节点", ErrBadRequest)
	}
	if hasXrayNode(ld) {
		b.WriteString("# {{xray_nodes}}\n")
	}
	content := []byte(b.String())
	plan, err := json.Marshal(map[string]any{"node_names": in.NodeNames})
	if err != nil {
		return nil, fmt.Errorf("序列化 SR 订阅渲染计划失败: %w", err)
	}
	return &RenderResult{Content: content, Skipped: skipped, Diagnostics: diagnostics, RenderPlan: plan}, nil
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
	rules, skipped, receipt := assembleRules(in, ld, rulespec.TargetSR, "Shadowrocket")
	for _, r := range rules {
		noResolve := *r.NoResolve && rulespec.SupportsAndMapLegacy(r.Type, rulespec.TargetSR).SupportsNoResolve
		b.WriteString(formatRuleLine(r.Type, r.Value, r.Target, noResolve))
		b.WriteString("\n")
		if !isGeoFallback(r.Type, r.Value, r.Target, noResolve) {
			receipt.FinalOutput++
		}
	}
	b.WriteString(systemGeoRule + "\n")
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
	return &RenderResult{Content: content, Skipped: skipped, RenderPlan: plan, Receipt: receipt}, nil
}
