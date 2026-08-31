// adapter_singbox.go：sing-box source JSON 简单子集适配器。
package pool

import (
	"encoding/json"
	"fmt"
	"strings"

	"vpn-sub/internal/rulespec"
)

type singBoxRule struct {
	Domain        []string `json:"domain"`
	DomainSuffix  []string `json:"domain_suffix"`
	DomainKeyword []string `json:"domain_keyword"`
	IPCIDR        []string `json:"ip_cidr"`
	IPCIDR6       []string `json:"ip_cidr6"`
	Invert        bool     `json:"invert"`
	Type          string   `json:"type"`
}

type singBoxSource struct {
	Version int           `json:"version"`
	Rules   []singBoxRule `json:"rules"`
}

// parseSingBoxSourceJSON 解析 sing-box source 仅支持简单单 family 子集。
func parseSingBoxSourceJSON(body []byte, mode SourceMode) ([]rulespec.CanonicalRule, []ParseDiagnostic, error) {
	if mode != SourceModeAuto {
		return nil, nil, fmt.Errorf("sing-box 来源仅在 auto 模式识别")
	}
	var doc singBoxSource
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, nil, fmt.Errorf("sing-box JSON 解析失败: %w", err)
	}
	if doc.Version <= 0 {
		return nil, nil, fmt.Errorf("sing-box source 缺少有效 version")
	}
	var rules []rulespec.CanonicalRule
	var diagnostics []ParseDiagnostic
	for i, r := range doc.Rules {
		if r.Invert || r.Type != "" {
			diagnostics = append(diagnostics, ParseDiagnostic{Line: i + 1, Kind: "reject", Message: "sing-box invert/logical 整项拒绝", Raw: fmt.Sprintf("%+v", r)})
			continue
		}
		fieldCount := 0
		for _, group := range [][]string{r.Domain, r.DomainSuffix, r.DomainKeyword, r.IPCIDR, r.IPCIDR6} {
			if len(group) > 0 {
				fieldCount++
			}
		}
		if fieldCount == 0 {
			diagnostics = append(diagnostics, ParseDiagnostic{Line: i + 1, Kind: "reject", Message: "sing-box 规则无支持字段", Raw: fmt.Sprintf("%+v", r)})
			continue
		}
		if fieldCount > 1 {
			diagnostics = append(diagnostics, ParseDiagnostic{Line: i + 1, Kind: "reject", Message: "sing-box 多条件 AND 整项拒绝", Raw: fmt.Sprintf("%+v", r)})
			continue
		}
		for _, v := range r.Domain {
			rule, err := NormalizeExplicitDomain(strings.TrimSpace(v), rulespec.MatcherExact)
			if err == nil {
				rules = append(rules, rule)
			}
		}
		for _, v := range r.DomainSuffix {
			rule, err := NormalizeExplicitDomain(strings.TrimSpace(v), rulespec.MatcherSuffix)
			if err == nil {
				rules = append(rules, rule)
			}
		}
		for _, v := range r.DomainKeyword {
			normalized := strings.ToLower(strings.TrimSpace(v))
			if normalized != "" {
				rules = append(rules, rulespec.CanonicalRule{Family: rulespec.FamilyDomain, Matcher: rulespec.MatcherKeyword, Value: normalized})
			}
		}
		for _, v := range append(append([]string{}, r.IPCIDR...), r.IPCIDR6...) {
			cidr, err := NormalizeCIDRValue(strings.TrimSpace(v))
			if err == nil {
				rules = append(rules, rulespec.CanonicalRule{Family: rulespec.FamilyIP, Matcher: rulespec.MatcherCIDR, Value: cidr})
			}
		}
	}
	return rules, diagnostics, nil
}
