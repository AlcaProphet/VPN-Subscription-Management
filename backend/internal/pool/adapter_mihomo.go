// adapter_mihomo.go：Mihomo domain/classical YAML provider 适配器。
package pool

import (
	"fmt"
	"strings"

	gyaml "github.com/goccy/go-yaml"

	"vpn-sub/internal/rulespec"
)

type mihomoPayload struct {
	Payload []string `yaml:"payload"`
}

// parseMihomoDomainYAML 解析 Mihomo domain provider。
func parseMihomoDomainYAML(body []byte) ([]rulespec.CanonicalRule, []ParseDiagnostic, error) {
	var doc mihomoPayload
	if err := gyaml.Unmarshal(body, &doc); err != nil {
		return nil, nil, fmt.Errorf("Mihomo YAML 解析失败: %w", err)
	}
	var rules []rulespec.CanonicalRule
	var diagnostics []ParseDiagnostic
	for i, raw := range doc.Payload {
		item := strings.TrimSpace(strings.Trim(raw, `"'`))
		if item == "" {
			continue
		}
		matcher := rulespec.MatcherExact
		value := item
		switch {
		case strings.HasPrefix(item, "+."):
			matcher = rulespec.MatcherSuffix
			value = strings.TrimPrefix(item, "+.")
		case strings.HasPrefix(item, "."):
			matcher = rulespec.MatcherSubdomainOnly
			value = strings.TrimPrefix(item, ".")
		case strings.Contains(item, "*"):
			matcher = rulespec.MatcherProviderLabelWildcard
		}
		rule, err := NormalizeExplicitDomain(value, matcher)
		if err != nil {
			diagnostics = append(diagnostics, ParseDiagnostic{Line: i + 1, Kind: "reject", Message: err.Error(), Raw: item})
			continue
		}
		rules = append(rules, rule)
	}
	return rules, diagnostics, nil
}

// parseMihomoClassicalYAML 解析 Mihomo classical rules YAML。
func parseMihomoClassicalYAML(body []byte) ([]rulespec.CanonicalRule, []ParseDiagnostic, error) {
	var doc mihomoPayload
	if err := gyaml.Unmarshal(body, &doc); err != nil {
		return nil, nil, fmt.Errorf("Mihomo classical YAML 解析失败: %w", err)
	}
	var rules []rulespec.CanonicalRule
	var diagnostics []ParseDiagnostic
	for i, raw := range doc.Payload {
		item := strings.TrimSpace(strings.Trim(raw, `"'`))
		if item == "" {
			continue
		}
		typ, value, _, ok := ParseLine(item)
		if !ok {
			diagnostics = append(diagnostics, ParseDiagnostic{Line: i + 1, Kind: "reject", Message: "classical 条目无法解析", Raw: item})
			continue
		}
		family, matcher, ok := rulespec.CanonicalizeLegacyType(typ)
		if !ok {
			diagnostics = append(diagnostics, ParseDiagnostic{Line: i + 1, Kind: "reject", Message: "不支持的规则类型: " + typ, Raw: item})
			continue
		}
		rule := rulespec.CanonicalRule{Family: family, Matcher: matcher, Value: value}
		if strings.Contains(strings.ToLower(item), "no-resolve") {
			rule.Options.NoResolve = true
		}
		normalized, err := NormalizeCanonical(rule)
		if err != nil {
			diagnostics = append(diagnostics, ParseDiagnostic{Line: i + 1, Kind: "reject", Message: err.Error(), Raw: item})
			continue
		}
		rules = append(rules, normalized)
	}
	return rules, diagnostics, nil
}
