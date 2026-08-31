// adapter_ip.go：纯 IP/CIDR/ASN 列适配器。
package pool

import (
	"net"
	"strings"

	"vpn-sub/internal/rulespec"
)

// parseIPList 解析纯 IP/CIDR/ASN 列表。
func parseIPList(body []byte) ([]rulespec.CanonicalRule, []ParseDiagnostic, error) {
	var rules []rulespec.CanonicalRule
	var diagnostics []ParseDiagnostic
	lines := strings.Split(string(body), "\n")
	for i, raw := range lines {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		upper := strings.ToUpper(line)
		var rule rulespec.CanonicalRule
		var err error
		if strings.HasPrefix(upper, "AS") || isDigits(line) {
			var v string
			v, err = NormalizeASNValue(line)
			if err == nil {
				rule = rulespec.CanonicalRule{Family: rulespec.FamilyIP, Matcher: rulespec.MatcherASN, Value: v}
			}
		} else if strings.Contains(line, "/") {
			var v string
			v, err = NormalizeCIDRValue(line)
			if err == nil {
				rule = rulespec.CanonicalRule{Family: rulespec.FamilyIP, Matcher: rulespec.MatcherCIDR, Value: v}
			}
		} else if ip := net.ParseIP(line); ip != nil {
			if ip.To4() != nil {
				rule = rulespec.CanonicalRule{Family: rulespec.FamilyIP, Matcher: rulespec.MatcherCIDR, Value: ip.String() + "/32"}
			} else {
				rule = rulespec.CanonicalRule{Family: rulespec.FamilyIP, Matcher: rulespec.MatcherCIDR, Value: ip.String() + "/128"}
			}
		} else {
			err = net.UnknownNetworkError("not ip")
		}
		if err != nil {
			diagnostics = append(diagnostics, ParseDiagnostic{Line: i + 1, Kind: "reject", Message: err.Error(), Raw: line})
			continue
		}
		rules = append(rules, rule)
	}
	return rules, diagnostics, nil
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
