// normalize.go：IDNA、PSL、CIDR、ASN 与 Canonical Rule 规范化。
package pool

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"vpn-sub/internal/rulespec"
)

// NormalizeDomainName 返回 IDNA ASCII 小写域名。
func NormalizeDomainName(value string) (string, error) {
	v := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(value), "."))
	if v == "" {
		return "", fmt.Errorf("域名不能为空")
	}
	if strings.ContainsAny(v, "/: \t") {
		return "", fmt.Errorf("域名含非法字符: %s", v)
	}
	ascii, err := idna.Lookup.ToASCII(v)
	if err != nil {
		return "", fmt.Errorf("IDNA 转换失败: %w", err)
	}
	return strings.ToLower(ascii), nil
}

// InferDomainRule 对裸域名执行 PSL 推断：eTLD+1 → suffix，否则 exact。
func InferDomainRule(value string) (rulespec.CanonicalRule, error) {
	normalized, err := NormalizeDomainName(value)
	if err != nil {
		return rulespec.CanonicalRule{}, err
	}
	labels := strings.Split(normalized, ".")
	if len(labels) < 2 {
		return rulespec.CanonicalRule{}, fmt.Errorf("单标签域名拒绝: %s", normalized)
	}
	etld1, err := publicsuffix.EffectiveTLDPlusOne(normalized)
	if err != nil {
		return rulespec.CanonicalRule{}, fmt.Errorf("无法计算 eTLD+1: %w", err)
	}
	matcher := rulespec.MatcherExact
	if normalized == etld1 {
		matcher = rulespec.MatcherSuffix
	}
	return rulespec.CanonicalRule{Family: rulespec.FamilyDomain, Matcher: matcher, Value: normalized}, nil
}

// NormalizeExplicitDomain 将显式完整/后缀域名规范化为 Canonical Rule。
func NormalizeExplicitDomain(value string, matcher rulespec.Matcher) (rulespec.CanonicalRule, error) {
	normalized, err := NormalizeDomainName(value)
	if err != nil {
		return rulespec.CanonicalRule{}, err
	}
	return rulespec.CanonicalRule{Family: rulespec.FamilyDomain, Matcher: matcher, Value: normalized}, nil
}

// NormalizeCIDRValue 将 IP/CIDR 归一为网络地址。
func NormalizeCIDRValue(value string) (string, error) {
	_, network, err := net.ParseCIDR(strings.TrimSpace(value))
	if err != nil {
		return "", fmt.Errorf("非法 CIDR: %w", err)
	}
	return network.String(), nil
}

// NormalizeASNValue 接受正整数或 AS 前缀，返回纯数字字符串。
func NormalizeASNValue(value string) (string, error) {
	v := strings.TrimSpace(value)
	if len(v) > 2 && strings.EqualFold(v[:2], "AS") {
		v = strings.TrimSpace(v[2:])
	}
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil || n == 0 {
		return "", fmt.Errorf("ASN 必须是正整数")
	}
	return strconv.FormatUint(n, 10), nil
}

// NormalizeCanonical 对 Canonical Rule 做值与格式规范化。
func NormalizeCanonical(rule rulespec.CanonicalRule) (rulespec.CanonicalRule, error) {
	switch rule.Family {
	case rulespec.FamilyDomain:
		switch rule.Matcher {
		case rulespec.MatcherExact, rulespec.MatcherSuffix, rulespec.MatcherKeyword:
			v, err := NormalizeDomainName(rule.Value)
			if err != nil {
				return rulespec.CanonicalRule{}, err
			}
			rule.Value = v
			return rule, nil
		default:
			return rule, nil
		}
	case rulespec.FamilyIP:
		switch rule.Matcher {
		case rulespec.MatcherCIDR:
			v, err := NormalizeCIDRValue(rule.Value)
			if err != nil {
				return rulespec.CanonicalRule{}, err
			}
			rule.Value = v
			return rule, nil
		case rulespec.MatcherASN:
			v, err := NormalizeASNValue(rule.Value)
			if err != nil {
				return rulespec.CanonicalRule{}, err
			}
			rule.Value = v
			return rule, nil
		}
	}
	return rule, nil
}
