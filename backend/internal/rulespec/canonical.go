// canonical.go：Canonical Rule 语义与稳定去重键。
package rulespec

import (
	"strconv"
	"strings"
)

// Family 表示规则匹配对象。
type Family string

const (
	FamilyDomain    Family = "domain"
	FamilyIP        Family = "ip"
	FamilyUserAgent Family = "user_agent"
	FamilyProcess   Family = "process"
	FamilyNetwork   Family = "network"
	FamilyPort      Family = "port"
	FamilyGeo       Family = "geo"
)

// Matcher 表示规则匹配方式。
type Matcher string

const (
	MatcherExact                 Matcher = "exact"
	MatcherSuffix                Matcher = "suffix"
	MatcherKeyword               Matcher = "keyword"
	MatcherRouteWildcard         Matcher = "route_wildcard"
	MatcherSubdomainOnly         Matcher = "subdomain_only"
	MatcherProviderLabelWildcard Matcher = "provider_label_wildcard"
	MatcherRegex                 Matcher = "regex"
	MatcherCIDR                  Matcher = "cidr"
	MatcherASN                   Matcher = "asn"
	MatcherEquals                Matcher = "equals"
)

// RuleOptions 是 Canonical Rule 的真实匹配选项，不包含源 policy。
type RuleOptions struct {
	NoResolve bool `json:"no_resolve,omitempty"`
}

// CanonicalRule 是跨源/目标平台解耦的规则语义。
type CanonicalRule struct {
	Family  Family      `json:"family"`
	Matcher Matcher     `json:"matcher"`
	Value   string      `json:"value"`
	Options RuleOptions `json:"options,omitempty"`
}

// SemanticKey 返回稳定去重键；只包含影响实际匹配语义的字段。
func (r CanonicalRule) SemanticKey() string {
	return strings.Join([]string{
		string(r.Family),
		string(r.Matcher),
		r.Value,
		strconv.FormatBool(r.Options.NoResolve),
	}, "\x00")
}

// LegacyCanonical 保存旧规则类型名到 Canonical 语义的映射。
type LegacyCanonical struct {
	Family  Family
	Matcher Matcher
}

var legacyCanonicalMap = map[string]LegacyCanonical{
	"DOMAIN":             {FamilyDomain, MatcherExact},
	"DOMAIN-SUFFIX":      {FamilyDomain, MatcherSuffix},
	"DOMAIN-KEYWORD":     {FamilyDomain, MatcherKeyword},
	"DOMAIN-REGEX":       {FamilyDomain, MatcherRegex},
	"GEOSITE":            {FamilyGeo, MatcherExact},
	"GEOIP":              {FamilyGeo, MatcherEquals},
	"SRC-GEOIP":          {FamilyGeo, MatcherEquals},
	"IP-ASN":             {FamilyIP, MatcherASN},
	"SRC-IP-ASN":         {FamilyIP, MatcherASN},
	"IP-CIDR":            {FamilyIP, MatcherCIDR},
	"IP-CIDR6":           {FamilyIP, MatcherCIDR},
	"SRC-IP-CIDR":        {FamilyIP, MatcherCIDR},
	"IP-SUFFIX":          {FamilyIP, MatcherSuffix},
	"SRC-IP-SUFFIX":      {FamilyIP, MatcherSuffix},
	"SRC-PORT":           {FamilyPort, MatcherEquals},
	"DST-PORT":           {FamilyPort, MatcherEquals},
	"IN-PORT":            {FamilyPort, MatcherEquals},
	"DSCP":               {FamilyNetwork, MatcherEquals},
	"PROCESS-NAME":       {FamilyProcess, MatcherExact},
	"PROCESS-PATH":       {FamilyProcess, MatcherExact},
	"PROCESS-NAME-REGEX": {FamilyProcess, MatcherRegex},
	"PROCESS-PATH-REGEX": {FamilyProcess, MatcherRegex},
	"NETWORK":            {FamilyNetwork, MatcherEquals},
	"UID":                {FamilyProcess, MatcherEquals},
	"IN-TYPE":            {FamilyNetwork, MatcherEquals},
	"IN-USER":            {FamilyProcess, MatcherEquals},
	"IN-NAME":            {FamilyProcess, MatcherEquals},
	"SUB-RULE":           {FamilyProcess, MatcherEquals},
	"RULE-SET":           {FamilyProcess, MatcherEquals},
	"AND":                {FamilyNetwork, MatcherEquals},
	"OR":                 {FamilyNetwork, MatcherEquals},
	"NOT":                {FamilyNetwork, MatcherEquals},
	"MATCH":              {FamilyNetwork, MatcherEquals},
	"USER-AGENT":         {FamilyUserAgent, MatcherExact},
}

// CanonicalizeLegacyType 将旧规则类型名映射为 Canonical 的 family/matcher。
func CanonicalizeLegacyType(ruleType string) (Family, Matcher, bool) {
	lc, ok := legacyCanonicalMap[strings.ToUpper(strings.TrimSpace(ruleType))]
	if !ok {
		return "", "", false
	}
	return lc.Family, lc.Matcher, true
}
