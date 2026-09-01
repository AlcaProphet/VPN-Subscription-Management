// adapter_legacy.go：legacy/plain 域名文本适配器。
package pool

import (
	"strings"

	"vpn-sub/internal/rulespec"
)

// parseDomainText 解析 full:/+. /裸域名文本。
func parseDomainText(body []byte) ([]rulespec.CanonicalRule, []ParseDiagnostic, error) {
	var rules []rulespec.CanonicalRule
	var diagnostics []ParseDiagnostic
	lines := strings.Split(string(body), "\n")
	for i, raw := range lines {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var rule rulespec.CanonicalRule
		var err error
		switch {
		case strings.HasPrefix(line, "full:"):
			rule, err = NormalizeExplicitDomain(strings.TrimSpace(strings.TrimPrefix(line, "full:")), rulespec.MatcherExact)
		case strings.HasPrefix(line, "+."):
			rule, err = NormalizeExplicitDomain(strings.TrimSpace(strings.TrimPrefix(line, "+.")), rulespec.MatcherSuffix)
		case strings.HasPrefix(line, "."):
			rule, err = NormalizeExplicitDomain(strings.TrimSpace(strings.TrimPrefix(line, ".")), rulespec.MatcherSubdomainOnly)
		default:
			rule, err = InferDomainRule(line)
		}
		if err != nil {
			diagnostics = append(diagnostics, ParseDiagnostic{Line: i + 1, Kind: "reject", Message: err.Error(), Raw: line})
			continue
		}
		rule, err = NormalizeCanonical(rule)
		if err != nil {
			diagnostics = append(diagnostics, ParseDiagnostic{Line: i + 1, Kind: "reject", Message: err.Error(), Raw: line})
			continue
		}
		rules = append(rules, rule)
	}
	return rules, diagnostics, nil
}
