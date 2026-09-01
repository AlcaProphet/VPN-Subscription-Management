// capability.go：中央能力注册表、目标映射与只读前端元数据。
package rulespec

import (
	"sort"
	"strings"
)

// Target 表示装配目标平台。
type Target string

const (
	TargetClash Target = "clash"
	TargetSR    Target = "sr"
)

// TargetScope 表示规则在 Clash/SR 双目标上的能力范围。
type TargetScope string

const (
	ScopeCommon      TargetScope = "common"
	ScopeClashOnly   TargetScope = "clash_only"
	ScopeSrOnly      TargetScope = "sr_only"
	ScopeUnsupported TargetScope = "unsupported"
)

// MappingResult 是 SupportsAndMap 的返回结果。
type MappingResult struct {
	Supported         bool        `json:"supported"`
	RenderType        string      `json:"render_type,omitempty"`
	ConversionKind    string      `json:"conversion_kind,omitempty"`
	SupportsNoResolve bool        `json:"supports_no_resolve"`
	TargetScope       TargetScope `json:"target_scope"`
}

// Capability 描述某类 Canonical Rule 的跨平台能力。
type Capability struct {
	Family            Family      `json:"family"`
	Matcher           Matcher     `json:"matcher"`
	Scope             TargetScope `json:"scope"`
	ClashRenderType   string      `json:"clash_render_type,omitempty"`
	SRRenderType      string      `json:"sr_render_type,omitempty"`
	SupportsNoResolve bool        `json:"supports_no_resolve"`
	MaterialPool      bool        `json:"material_pool"`
	Advanced          bool        `json:"advanced"`
}

// CapabilityMetadata 是供前端消费的只读能力元数据。
type CapabilityMetadata struct {
	Family            Family      `json:"family"`
	Matcher           Matcher     `json:"matcher"`
	Scope             TargetScope `json:"scope"`
	ClashRenderType   string      `json:"clash_render_type,omitempty"`
	SRRenderType      string      `json:"sr_render_type,omitempty"`
	SupportsNoResolve bool        `json:"supports_no_resolve"`
	MaterialPool      bool        `json:"material_pool"`
	Advanced          bool        `json:"advanced"`
}

// capabilityRegistry 是后端唯一能力事实来源；顺序即前端元数据顺序。
var capabilityRegistry = []Capability{
	{Family: FamilyDomain, Matcher: MatcherExact, Scope: ScopeCommon, ClashRenderType: "DOMAIN", SRRenderType: "DOMAIN", MaterialPool: true, Advanced: true},
	{Family: FamilyDomain, Matcher: MatcherSuffix, Scope: ScopeCommon, ClashRenderType: "DOMAIN-SUFFIX", SRRenderType: "DOMAIN-SUFFIX", MaterialPool: true, Advanced: true},
	{Family: FamilyDomain, Matcher: MatcherKeyword, Scope: ScopeCommon, ClashRenderType: "DOMAIN-KEYWORD", SRRenderType: "DOMAIN-KEYWORD", MaterialPool: true, Advanced: true},
	{Family: FamilyDomain, Matcher: MatcherRegex, Scope: ScopeClashOnly, ClashRenderType: "DOMAIN-REGEX", SRRenderType: "", MaterialPool: true, Advanced: true},
	{Family: FamilyDomain, Matcher: MatcherSubdomainOnly, Scope: ScopeUnsupported, ClashRenderType: "", SRRenderType: "", MaterialPool: false, Advanced: true},
	{Family: FamilyDomain, Matcher: MatcherProviderLabelWildcard, Scope: ScopeUnsupported, ClashRenderType: "", SRRenderType: "", MaterialPool: false, Advanced: true},
	{Family: FamilyDomain, Matcher: MatcherRouteWildcard, Scope: ScopeUnsupported, ClashRenderType: "", SRRenderType: "", MaterialPool: false, Advanced: true},
	{Family: FamilyIP, Matcher: MatcherCIDR, Scope: ScopeCommon, ClashRenderType: "IP-CIDR", SRRenderType: "IP-CIDR", SupportsNoResolve: true, MaterialPool: true, Advanced: true},
	{Family: FamilyIP, Matcher: MatcherASN, Scope: ScopeCommon, ClashRenderType: "IP-ASN", SRRenderType: "IP-ASN", SupportsNoResolve: true, MaterialPool: true, Advanced: true},
	{Family: FamilyUserAgent, Matcher: MatcherExact, Scope: ScopeSrOnly, ClashRenderType: "", SRRenderType: "USER-AGENT", MaterialPool: true, Advanced: true},
	{Family: FamilyProcess, Matcher: MatcherExact, Scope: ScopeCommon, ClashRenderType: "PROCESS-NAME", SRRenderType: "PROCESS-NAME", MaterialPool: true, Advanced: true},
	{Family: FamilyProcess, Matcher: MatcherRegex, Scope: ScopeCommon, ClashRenderType: "PROCESS-NAME-REGEX", SRRenderType: "PROCESS-NAME-REGEX", MaterialPool: true, Advanced: true},
	{Family: FamilyProcess, Matcher: MatcherEquals, Scope: ScopeClashOnly, ClashRenderType: "PROCESS-PATH", SRRenderType: "", MaterialPool: false, Advanced: true},
	{Family: FamilyNetwork, Matcher: MatcherEquals, Scope: ScopeClashOnly, ClashRenderType: "NETWORK", SRRenderType: "", MaterialPool: false, Advanced: true},
	{Family: FamilyPort, Matcher: MatcherEquals, Scope: ScopeClashOnly, ClashRenderType: "DST-PORT", SRRenderType: "", MaterialPool: false, Advanced: true},
	{Family: FamilyGeo, Matcher: MatcherEquals, Scope: ScopeCommon, ClashRenderType: "GEOIP", SRRenderType: "GEOIP", SupportsNoResolve: true, MaterialPool: true, Advanced: true},
	{Family: FamilyGeo, Matcher: MatcherExact, Scope: ScopeClashOnly, ClashRenderType: "GEOSITE", SRRenderType: "", MaterialPool: false, Advanced: true},
	{Family: FamilyProcess, Matcher: MatcherEquals, Scope: ScopeUnsupported, ClashRenderType: "", SRRenderType: "", MaterialPool: false, Advanced: true}, // RULE-SET / SUB-RULE / MATCH 等高级/终结规则占位
}

// findCapability 按 family+matcher 查找能力。
func findCapability(family Family, matcher Matcher) (Capability, bool) {
	for _, c := range capabilityRegistry {
		if c.Family == family && c.Matcher == matcher {
			return c, true
		}
	}
	return Capability{}, false
}

// SupportsAndMap 判断 Canonical Rule 是否可映射到目标，并返回渲染类型与转换信息。
func SupportsAndMap(rule CanonicalRule, target Target) MappingResult {
	cap, ok := findCapability(rule.Family, rule.Matcher)
	if !ok {
		return MappingResult{Supported: false, TargetScope: ScopeUnsupported}
	}
	switch target {
	case TargetClash:
		if cap.ClashRenderType == "" {
			return MappingResult{Supported: false, RenderType: "", TargetScope: cap.Scope, SupportsNoResolve: cap.SupportsNoResolve}
		}
		return MappingResult{Supported: true, RenderType: canonicalRenderType(rule, cap.ClashRenderType), ConversionKind: "direct", SupportsNoResolve: cap.SupportsNoResolve, TargetScope: cap.Scope}
	case TargetSR:
		if cap.SRRenderType == "" {
			return MappingResult{Supported: false, RenderType: "", TargetScope: cap.Scope, SupportsNoResolve: cap.SupportsNoResolve}
		}
		return MappingResult{Supported: true, RenderType: canonicalRenderType(rule, cap.SRRenderType), ConversionKind: "direct", SupportsNoResolve: cap.SupportsNoResolve, TargetScope: cap.Scope}
	default:
		return MappingResult{Supported: false, TargetScope: ScopeUnsupported}
	}
}

// canonicalRenderType 根据 Canonical CIDR 的地址族选择目标类型名。
func canonicalRenderType(rule CanonicalRule, defaultType string) string {
	if rule.Family == FamilyIP && rule.Matcher == MatcherCIDR && strings.Contains(rule.Value, ":") {
		return "IP-CIDR6"
	}
	return defaultType
}

// Capabilities 返回能力注册表副本。
func Capabilities() []Capability {
	out := make([]Capability, len(capabilityRegistry))
	copy(out, capabilityRegistry)
	return out
}

// Metadata 返回稳定的只读前端元数据。
func Metadata() []CapabilityMetadata {
	out := make([]CapabilityMetadata, 0, len(capabilityRegistry))
	for _, c := range capabilityRegistry {
		out = append(out, CapabilityMetadata{
			Family:            c.Family,
			Matcher:           c.Matcher,
			Scope:             c.Scope,
			ClashRenderType:   c.ClashRenderType,
			SRRenderType:      c.SRRenderType,
			SupportsNoResolve: c.SupportsNoResolve,
			MaterialPool:      c.MaterialPool,
			Advanced:          c.Advanced,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
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
