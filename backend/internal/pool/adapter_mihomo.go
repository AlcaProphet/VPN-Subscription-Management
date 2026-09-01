// adapter_mihomo.go：Mihomo domain/ipcidr/classical YAML provider 适配器。
package pool

import (
	"fmt"
	"strings"

	gyaml "github.com/goccy/go-yaml"

	"vpn-sub/internal/rulespec"
)

type mihomoPayload struct {
	Payload []any `yaml:"payload"`
}

// decodeMihomoPayload 严格读取顶层 payload，并拒绝空列表和非字符串条目。
func decodeMihomoPayload(body []byte) ([]string, error) {
	var doc mihomoPayload
	if err := gyaml.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("Mihomo YAML 解析失败: %w", err)
	}
	if len(doc.Payload) == 0 {
		return nil, fmt.Errorf("%w: payload 不能为空", ErrUnrecognizedSource)
	}
	items := make([]string, 0, len(doc.Payload))
	for _, raw := range doc.Payload {
		item, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("%w: payload 只能包含字符串条目", ErrConflictingDocumentFormat)
		}
		item = strings.TrimSpace(item)
		if item == "" {
			return nil, fmt.Errorf("%w: payload 不能包含空条目", ErrConflictingDocumentFormat)
		}
		items = append(items, item)
	}
	return items, nil
}

// parseMihomoDomainYAML 解析 Mihomo domain provider。
func parseMihomoDomainYAML(body []byte) ([]rulespec.CanonicalRule, []ParseDiagnostic, error) {
	items, err := decodeMihomoPayload(body)
	if err != nil {
		return nil, nil, err
	}
	var rules []rulespec.CanonicalRule
	var diagnostics []ParseDiagnostic
	for i, item := range items {
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

// parseMihomoIPCIDRYAML 解析 Mihomo ipcidr provider；payload 只允许 IPv4/IPv6 CIDR。
func parseMihomoIPCIDRYAML(body []byte) ([]rulespec.CanonicalRule, []ParseDiagnostic, error) {
	items, err := decodeMihomoPayload(body)
	if err != nil {
		return nil, nil, err
	}
	rules := make([]rulespec.CanonicalRule, 0, len(items))
	for _, item := range items {
		value, err := NormalizeCIDRValue(item)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: ipcidr payload 包含非法 CIDR %q", ErrConflictingDocumentFormat, item)
		}
		rules = append(rules, rulespec.CanonicalRule{
			Family:  rulespec.FamilyIP,
			Matcher: rulespec.MatcherCIDR,
			Value:   value,
		})
	}
	return rules, nil, nil
}

// parseMihomoClassicalYAML 解析 Mihomo classical rules YAML。
func parseMihomoClassicalYAML(body []byte) ([]rulespec.CanonicalRule, []ParseDiagnostic, error) {
	items, err := decodeMihomoPayload(body)
	if err != nil {
		return nil, nil, err
	}
	var rules []rulespec.CanonicalRule
	var diagnostics []ParseDiagnostic
	for i, item := range items {
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
