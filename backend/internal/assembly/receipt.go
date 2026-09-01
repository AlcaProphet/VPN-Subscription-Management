// receipt.go：转换回执与零输出门槛（不含内置兜底）。
package assembly

import (
	"vpn-sub/internal/rulespec"
)

// buildReceipt 计算素材池+自定义规则的目标转换回执。
func buildReceipt(in GenerateInput, ld *loadedData) *ConversionReceipt {
	target := targetFromSyntax(in.TargetSyntax)
	if target == "" {
		return nil
	}
	r := &ConversionReceipt{}
	count := func(ruleType, value string) {
		r.Input++
		family, matcher, ok := rulespec.CanonicalizeLegacyType(ruleType)
		if !ok {
			r.TargetValidationFailed++
			return
		}
		mapped := rulespec.SupportsAndMap(rulespec.CanonicalRule{Family: family, Matcher: matcher, Value: value}, target)
		if !mapped.Supported {
			r.SkippedUnsupported++
			return
		}
		if mapped.ConversionKind == "" || mapped.ConversionKind == "direct" {
			r.DirectOutput++
		} else {
			r.EquivalentConversions++
		}
	}
	for _, p := range in.Pools {
		for _, e := range ld.pools[p.PoolID] {
			count(e.RuleType, e.MatchValue)
		}
	}
	for _, rule := range in.CustomRules {
		count(rule.RuleType, rule.MatchValue)
	}
	r.FinalOutput = r.DirectOutput + r.EquivalentConversions
	return r
}

func targetFromSyntax(s TargetSyntax) rulespec.Target {
	switch s {
	case ClashYAML:
		return rulespec.TargetClash
	case SrConf:
		return rulespec.TargetSR
	default:
		return ""
	}
}
