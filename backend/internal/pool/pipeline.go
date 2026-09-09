// pipeline.go：Step 2 解析管线编排与来源准入。
package pool

import (
	"fmt"
	"sort"
	"strings"

	"vpn-sub/internal/rulespec"
)

// ParseSource 执行完整解析管线：探测 → 唯一适配器 → 规范化 → 来源准入 → 阈值。
func ParseSource(body []byte, mode SourceMode) (*ParseResult, error) {
	format, err := DetectOne(body, mode)
	if err != nil {
		return nil, err
	}
	var items []ParsedRule
	var diagnostics []ParseDiagnostic
	switch format {
	case FormatPlainDomainText, FormatLegacyDomainText:
		items, diagnostics, err = parseDomainText(body)
	case FormatMihomoDomainYAML:
		items, diagnostics, err = parseMihomoDomainYAML(body)
	case FormatMihomoIPCIDRYAML:
		items, diagnostics, err = parseMihomoIPCIDRYAML(body)
	case FormatMihomoClassicalYAML:
		items, diagnostics, err = parseMihomoClassicalYAML(body)
	case FormatTypedRuleText:
		items, diagnostics, err = parseTypedText(body)
	case FormatPlainIPCIDRText:
		items, diagnostics, err = parseIPList(body)
	case FormatSingBoxSourceJSON:
		items, diagnostics, err = parseSingBoxSourceJSON(body, mode)
	default:
		return nil, fmt.Errorf("%w: 未知格式 %s", ErrUnrecognizedSource, format)
	}
	if err != nil {
		return nil, err
	}
	res, err := finalizeParseResult(format, items, diagnostics, mode, body)
	if err != nil {
		return nil, err
	}
	res.EvidenceCodes = evidenceCodesForFormat(format)
	return res, nil
}

func evidenceCodesForFormat(format DetectedFormat) []string {
	switch format {
	case FormatSingBoxSourceJSON:
		return []string{"sing_box_version_and_rules"}
	case FormatMihomoDomainYAML:
		return []string{"top_level_payload", "payload_domain_only"}
	case FormatMihomoIPCIDRYAML:
		return []string{"top_level_payload", "payload_ipcidr_only"}
	case FormatMihomoClassicalYAML:
		return []string{"top_level_payload", "payload_classical_only"}
	case FormatTypedRuleText:
		return []string{"typed_rule_marker"}
	case FormatPlainIPCIDRText:
		return []string{"all_items_ip_cidr_or_asn"}
	case FormatLegacyDomainText:
		return []string{"legacy_domain_prefix"}
	case FormatPlainDomainText:
		return []string{"plain_domain_candidates"}
	default:
		return nil
	}
}

// finalizeParseResult 统计、去重、来源准入与阈值判断。
func finalizeParseResult(format DetectedFormat, items []ParsedRule, diagnostics []ParseDiagnostic, mode SourceMode, body []byte) (*ParseResult, error) {
	res := &ParseResult{Format: format, Diagnostics: diagnostics}
	res.Input = len(items)
	for _, d := range diagnostics {
		if d.Kind == "reject" {
			res.Input++
			res.Rejected++
			res.UnclassifiedRejected++
		}
	}
	res.Recognized = len(items)
	res.Profile = "unknown"

	accepted := make([]rulespec.CanonicalRule, 0, len(items))
	allItems := make([]ParsedRule, 0, len(items))
	seen := map[string]bool{}
	countMap := map[string]*RuleCountStat{}
	hasClashPrivate := false
	hasSRPrivate := false

	// detected_profile 必须基于来源模式排除前的全部已识别、规范化候选计算，
	// 否则显式 Clash/SR 模式会先把另一平台私有项剔除后误报为 common。
	for _, item := range items {
		rule := item.Rule
		if !supportsTarget(rule, rulespec.TargetClash) {
			hasSRPrivate = true
		}
		if !supportsTarget(rule, rulespec.TargetSR) {
			hasClashPrivate = true
		}
	}

	for _, item := range items {
		rule := item.Rule
		cap := capabilityForRule(rule)
		stat := ensureRuleCountStat(countMap, rule)
		if !cap.MaterialPool {
			res.Rejected++
			stat.Rejected++
			raw := item.Origin.Raw
			if raw == "" {
				raw = rule.Value
			}
			diagnostics = append(diagnostics, ParseDiagnostic{Kind: "reject", Message: "不是素材池可选能力", Raw: raw})
			continue
		}
		switch mode {
		case SourceModeClash:
			if !supportsTarget(rule, rulespec.TargetClash) {
				res.Excluded++
				stat.Excluded++
				continue
			}
		case SourceModeShadowrocket:
			if !supportsTarget(rule, rulespec.TargetSR) {
				res.Excluded++
				stat.Excluded++
				continue
			}
		}
		allItems = append(allItems, item)
		key := rule.SemanticKey()
		if seen[key] {
			res.Duplicates++
			stat.Duplicates++
			continue
		}
		seen[key] = true
		accepted = append(accepted, rule)
		stat.Accepted++
	}

	if mode == SourceModeAuto && hasClashPrivate && hasSRPrivate {
		return nil, ErrMixedPlatformSource
	}
	if hasClashPrivate {
		res.Profile = "clash"
	} else if hasSRPrivate {
		res.Profile = "shadowrocket"
	} else {
		res.Profile = "common"
	}

	res.Rules = accepted
	res.Items = allItems
	res.Accepted = len(accepted)
	res.Diagnostics = diagnostics
	res.RuleCounts = sortedRuleCounts(countMap)
	if res.Accepted == 0 {
		return nil, ErrNoAcceptedRules
	}
	if !meetsRecognitionThreshold(res.Input, res.Recognized) {
		return nil, ErrThresholdNotMet
	}
	return res, nil
}

func ensureRuleCountStat(counts map[string]*RuleCountStat, rule rulespec.CanonicalRule) *RuleCountStat {
	cap := capabilityForRule(rule)
	key := string(rule.Family) + "\x00" + string(rule.Matcher) + "\x00" + string(cap.Scope)
	if s, ok := counts[key]; ok {
		return s
	}
	s := &RuleCountStat{
		Family:  string(rule.Family),
		Matcher: string(rule.Matcher),
		Scope:   string(cap.Scope),
	}
	counts[key] = s
	return s
}

func sortedRuleCounts(counts map[string]*RuleCountStat) []RuleCountStat {
	out := make([]RuleCountStat, 0, len(counts))
	for _, s := range counts {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Family != out[j].Family {
			return out[i].Family < out[j].Family
		}
		if out[i].Matcher != out[j].Matcher {
			return out[i].Matcher < out[j].Matcher
		}
		return out[i].Scope < out[j].Scope
	})
	return out
}

func capabilityForRule(rule rulespec.CanonicalRule) rulespec.Capability {
	for _, c := range rulespec.Capabilities() {
		if c.Family == rule.Family && c.Matcher == rule.Matcher {
			return c
		}
	}
	return rulespec.Capability{}
}

func supportsTarget(rule rulespec.CanonicalRule, target rulespec.Target) bool {
	return rulespec.SupportsAndMap(rule, target).Supported
}

func countInputItems(body []byte) int {
	n := 0
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		n++
	}
	return n
}

func meetsRecognitionThreshold(input, recognized int) bool {
	if input < 10 {
		return recognized == input
	}
	return recognized*10 >= input*9
}
