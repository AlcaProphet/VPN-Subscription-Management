// adapter_typed.go：显式类型文本适配器。
package pool

import (
	"strings"

	"vpn-sub/internal/rulespec"
)

// parseTypedText 解析 DOMAIN,DOMAIN-SUFFIX,IP-CIDR,USER-AGENT 等显式类型行。
func parseTypedText(body []byte) ([]ParsedRule, []ParseDiagnostic, error) {
	var rules []ParsedRule
	var diagnostics []ParseDiagnostic
	lines := strings.Split(string(body), "\n")
	for i, raw := range lines {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		typ, value, noResolve, _, ok := parseRuleLine(line)
		if !ok {
			diagnostics = append(diagnostics, ParseDiagnostic{Line: i + 1, Kind: "reject", Message: "无法解析的显式规则行", Raw: line})
			continue
		}
		if !rulespec.IsMaterialPoolType(typ) {
			diagnostics = append(diagnostics, ParseDiagnostic{Line: i + 1, Kind: "reject", Message: "不是素材池可选能力: " + typ, Raw: line})
			continue
		}
		family, matcher, ok := rulespec.CanonicalizeLegacyType(typ)
		if !ok {
			diagnostics = append(diagnostics, ParseDiagnostic{Line: i + 1, Kind: "reject", Message: "不支持的规则类型: " + typ, Raw: line})
			continue
		}
		rule := rulespec.CanonicalRule{Family: family, Matcher: matcher, Value: value, Options: rulespec.RuleOptions{NoResolve: noResolve}}
		normalized, err := NormalizeCanonical(rule)
		if err != nil {
			diagnostics = append(diagnostics, ParseDiagnostic{Line: i + 1, Kind: "reject", Message: err.Error(), Raw: line})
			continue
		}
		rules = append(rules, ParsedRule{Rule: normalized, Origin: RuleOriginMeta{Line: i + 1, Raw: line, Order: i}})
	}
	return rules, diagnostics, nil
}
