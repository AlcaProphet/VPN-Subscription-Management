// adapter_typed.go：显式类型文本适配器。
package pool

import (
	"strings"

	"vpn-sub/internal/rulespec"
)

// parseTypedText 解析 DOMAIN,DOMAIN-SUFFIX,IP-CIDR,USER-AGENT 等显式类型行。
func parseTypedText(body []byte) ([]rulespec.CanonicalRule, []ParseDiagnostic, error) {
	var rules []rulespec.CanonicalRule
	var diagnostics []ParseDiagnostic
	lines := strings.Split(string(body), "\n")
	for i, raw := range lines {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		typ, value, _, ok := ParseLine(line)
		if !ok {
			diagnostics = append(diagnostics, ParseDiagnostic{Line: i + 1, Kind: "reject", Message: "无法解析的显式规则行", Raw: line})
			continue
		}
		family, matcher, ok := rulespec.CanonicalizeLegacyType(typ)
		if !ok {
			diagnostics = append(diagnostics, ParseDiagnostic{Line: i + 1, Kind: "reject", Message: "不支持的规则类型: " + typ, Raw: line})
			continue
		}
		rule := rulespec.CanonicalRule{Family: family, Matcher: matcher, Value: value}
		if strings.Contains(strings.ToLower(line), "no-resolve") {
			rule.Options.NoResolve = true
		}
		normalized, err := NormalizeCanonical(rule)
		if err != nil {
			diagnostics = append(diagnostics, ParseDiagnostic{Line: i + 1, Kind: "reject", Message: err.Error(), Raw: line})
			continue
		}
		rules = append(rules, normalized)
	}
	return rules, diagnostics, nil
}
