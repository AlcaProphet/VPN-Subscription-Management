// pipeline.go：Step 2 解析管线编排与来源准入。
package pool

import (
	"fmt"
	"strings"

	"vpn-sub/internal/rulespec"
)

// ParseSource 执行完整解析管线：探测 → 唯一适配器 → 规范化 → 来源准入 → 阈值。
func ParseSource(body []byte, mode SourceMode) (*ParseResult, error) {
	format, err := DetectOne(body, mode)
	if err != nil {
		return nil, err
	}
	var rules []rulespec.CanonicalRule
	var diagnostics []ParseDiagnostic
	switch format {
	case FormatPlainDomainText, FormatLegacyDomainText:
		rules, diagnostics, err = parseDomainText(body)
	case FormatMihomoDomainYAML:
		rules, diagnostics, err = parseMihomoDomainYAML(body)
	case FormatMihomoClassicalYAML:
		rules, diagnostics, err = parseMihomoClassicalYAML(body)
	case FormatTypedRuleText:
		rules, diagnostics, err = parseTypedText(body)
	case FormatPlainIPCIDRText:
		rules, diagnostics, err = parseIPList(body)
	case FormatSingBoxSourceJSON:
		rules, diagnostics, err = parseSingBoxSourceJSON(body, mode)
	default:
		return nil, fmt.Errorf("%w: 未知格式 %s", ErrUnrecognizedSource, format)
	}
	if err != nil {
		return nil, err
	}
	return finalizeParseResult(format, rules, diagnostics, mode, body)
}

// finalizeParseResult 统计、去重、来源准入与阈值判断。
func finalizeParseResult(format DetectedFormat, rules []rulespec.CanonicalRule, diagnostics []ParseDiagnostic, mode SourceMode, body []byte) (*ParseResult, error) {
	res := &ParseResult{Format: format, Diagnostics: diagnostics}
	res.Input = len(rules)
	for _, d := range diagnostics {
		if d.Kind == "reject" {
			res.Input++
		}
	}
	res.Recognized = len(rules)
	res.Profile = "unknown"

	accepted := make([]rulespec.CanonicalRule, 0, len(rules))
	seen := map[string]bool{}
	hasClashPrivate := false
	hasSRPrivate := false

	for _, rule := range rules {
		cap := capabilityForRule(rule)
		if !cap.MaterialPool {
			res.Rejected++
			diagnostics = append(diagnostics, ParseDiagnostic{Kind: "reject", Message: "不是素材池可选能力", Raw: rule.Value})
			continue
		}
		switch mode {
		case SourceModeClash:
			if !supportsTarget(rule, rulespec.TargetClash) {
				res.Excluded++
				continue
			}
		case SourceModeShadowrocket:
			if !supportsTarget(rule, rulespec.TargetSR) {
				res.Excluded++
				continue
			}
		}
		if !supportsTarget(rule, rulespec.TargetClash) {
			hasSRPrivate = true
		}
		if !supportsTarget(rule, rulespec.TargetSR) {
			hasClashPrivate = true
		}
		key := rule.SemanticKey()
		if seen[key] {
			res.Duplicates++
			continue
		}
		seen[key] = true
		accepted = append(accepted, rule)
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
	res.Accepted = len(accepted)
	if mode == SourceModeClash || mode == SourceModeShadowrocket {
		res.Excluded += len(rules) - res.Accepted - res.Rejected
	}
	if res.Accepted == 0 {
		return nil, ErrNoAcceptedRules
	}
	if !meetsRecognitionThreshold(res.Input, res.Recognized) {
		return nil, ErrThresholdNotMet
	}
	return res, nil
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
