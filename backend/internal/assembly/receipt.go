package assembly

import (
	"strings"

	"vpn-sub/internal/rulespec"
)

const systemGeoRule = "GEOIP,CN,DIRECT"

// isGeoFallback 识别内置直连规则，覆盖层重新写入同一条规则时仍视为兜底。
func isGeoFallback(typ, value, target string, noResolve bool) bool {
	return typ == "GEOIP" && strings.EqualFold(value, "CN") && target == "DIRECT" && !noResolve
}

// assembleRules 用同一次目标映射生成规则、跳过项和转换统计。
func assembleRules(in GenerateInput, ld *loadedData, target rulespec.Target, targetLabel string) ([]ClashPlanRule, []SkipItem, *ConversionReceipt) {
	rules := make([]ClashPlanRule, 0, len(in.CustomRules))
	skipped := []SkipItem{}
	receipt := &ConversionReceipt{}
	appendRule := func(ruleType, value, policy string, noResolve bool) {
		receipt.Input++
		typ, normalized, err := rulespec.ValidateValue(ruleType, value)
		if err != nil {
			receipt.TargetValidationFailed++
			skipped = append(skipped, SkipItem{Kind: "rule", Name: ruleType + "," + value, Reason: err.Error()})
			return
		}
		mapped := rulespec.SupportsAndMapLegacy(typ, target)
		if !mapped.Supported {
			receipt.SkippedUnsupported++
			skipped = append(skipped, SkipItem{Kind: "rule", Name: typ + "," + normalized, Reason: targetLabel + " 不支持该规则类型"})
			return
		}
		if mapped.ConversionKind == "" || mapped.ConversionKind == "direct" {
			receipt.DirectOutput++
		} else {
			receipt.EquivalentConversions++
		}
		rules = append(rules, ClashPlanRule{Type: mapped.RenderType, Value: normalized, Target: policy, NoResolve: boolPtr(noResolve)})
	}
	for _, p := range in.Pools {
		for _, e := range ld.pools[p.PoolID] {
			appendRule(e.RuleType, e.MatchValue, p.Target, e.NoResolve)
		}
	}
	for _, r := range in.CustomRules {
		appendRule(r.RuleType, r.MatchValue, r.Target, false)
	}
	return rules, skipped, receipt
}

// formatRuleLine 仅在目标支持且规则设置了 no-resolve 时追加该选项。
func formatRuleLine(ruleType, value, target string, noResolve bool) string {
	line := ruleType + ","
	if ruleType != "MATCH" {
		line += value + ","
	}
	line += target
	if noResolve {
		line += ",no-resolve"
	}
	return line
}
