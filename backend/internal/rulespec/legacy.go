// legacy.go：旧规则类型到中央能力注册表的兼容映射（供装配/自定义规则使用）。
package rulespec

// LegacyCapability 描述旧规则类型在 Clash/SR 目标上的渲染能力。
type LegacyCapability struct {
	RuleType          string
	Scope             TargetScope
	ClashRenderType   string
	SRRenderType      string
	SupportsNoResolve bool
	MaterialPool      bool
	Advanced          bool
}

var legacyCapabilityMap = map[string]LegacyCapability{
	"DOMAIN":             {RuleType: "DOMAIN", Scope: ScopeCommon, ClashRenderType: "DOMAIN", SRRenderType: "DOMAIN", MaterialPool: true, Advanced: true},
	"DOMAIN-SUFFIX":      {RuleType: "DOMAIN-SUFFIX", Scope: ScopeCommon, ClashRenderType: "DOMAIN-SUFFIX", SRRenderType: "DOMAIN-SUFFIX", MaterialPool: true, Advanced: true},
	"DOMAIN-KEYWORD":     {RuleType: "DOMAIN-KEYWORD", Scope: ScopeCommon, ClashRenderType: "DOMAIN-KEYWORD", SRRenderType: "DOMAIN-KEYWORD", MaterialPool: true, Advanced: true},
	"DOMAIN-REGEX":       {RuleType: "DOMAIN-REGEX", Scope: ScopeClashOnly, ClashRenderType: "DOMAIN-REGEX", MaterialPool: true, Advanced: true},
	"GEOSITE":            {RuleType: "GEOSITE", Scope: ScopeClashOnly, ClashRenderType: "GEOSITE", MaterialPool: false, Advanced: true},
	"GEOIP":              {RuleType: "GEOIP", Scope: ScopeCommon, ClashRenderType: "GEOIP", SRRenderType: "GEOIP", SupportsNoResolve: true, MaterialPool: true, Advanced: true},
	"SRC-GEOIP":          {RuleType: "SRC-GEOIP", Scope: ScopeClashOnly, ClashRenderType: "SRC-GEOIP", MaterialPool: false, Advanced: true},
	"IP-ASN":             {RuleType: "IP-ASN", Scope: ScopeCommon, ClashRenderType: "IP-ASN", SRRenderType: "IP-ASN", SupportsNoResolve: true, MaterialPool: true, Advanced: true},
	"SRC-IP-ASN":         {RuleType: "SRC-IP-ASN", Scope: ScopeClashOnly, ClashRenderType: "SRC-IP-ASN", MaterialPool: false, Advanced: true},
	"IP-CIDR":            {RuleType: "IP-CIDR", Scope: ScopeCommon, ClashRenderType: "IP-CIDR", SRRenderType: "IP-CIDR", SupportsNoResolve: true, MaterialPool: true, Advanced: true},
	"IP-CIDR6":           {RuleType: "IP-CIDR6", Scope: ScopeCommon, ClashRenderType: "IP-CIDR6", SRRenderType: "IP-CIDR6", SupportsNoResolve: true, MaterialPool: true, Advanced: true},
	"SRC-IP-CIDR":        {RuleType: "SRC-IP-CIDR", Scope: ScopeClashOnly, ClashRenderType: "SRC-IP-CIDR", MaterialPool: false, Advanced: true},
	"IP-SUFFIX":          {RuleType: "IP-SUFFIX", Scope: ScopeClashOnly, ClashRenderType: "IP-SUFFIX", SupportsNoResolve: true, MaterialPool: false, Advanced: true},
	"SRC-IP-SUFFIX":      {RuleType: "SRC-IP-SUFFIX", Scope: ScopeClashOnly, ClashRenderType: "SRC-IP-SUFFIX", MaterialPool: false, Advanced: true},
	"SRC-PORT":           {RuleType: "SRC-PORT", Scope: ScopeClashOnly, ClashRenderType: "SRC-PORT", MaterialPool: false, Advanced: true},
	"DST-PORT":           {RuleType: "DST-PORT", Scope: ScopeClashOnly, ClashRenderType: "DST-PORT", MaterialPool: false, Advanced: true},
	"IN-PORT":            {RuleType: "IN-PORT", Scope: ScopeClashOnly, ClashRenderType: "IN-PORT", MaterialPool: false, Advanced: true},
	"DSCP":               {RuleType: "DSCP", Scope: ScopeClashOnly, ClashRenderType: "DSCP", MaterialPool: false, Advanced: true},
	"PROCESS-NAME":       {RuleType: "PROCESS-NAME", Scope: ScopeCommon, ClashRenderType: "PROCESS-NAME", SRRenderType: "PROCESS-NAME", MaterialPool: true, Advanced: true},
	"PROCESS-PATH":       {RuleType: "PROCESS-PATH", Scope: ScopeClashOnly, ClashRenderType: "PROCESS-PATH", MaterialPool: false, Advanced: true},
	"PROCESS-NAME-REGEX": {RuleType: "PROCESS-NAME-REGEX", Scope: ScopeCommon, ClashRenderType: "PROCESS-NAME-REGEX", SRRenderType: "PROCESS-NAME-REGEX", MaterialPool: true, Advanced: true},
	"PROCESS-PATH-REGEX": {RuleType: "PROCESS-PATH-REGEX", Scope: ScopeClashOnly, ClashRenderType: "PROCESS-PATH-REGEX", MaterialPool: false, Advanced: true},
	"NETWORK":            {RuleType: "NETWORK", Scope: ScopeClashOnly, ClashRenderType: "NETWORK", MaterialPool: false, Advanced: true},
	"UID":                {RuleType: "UID", Scope: ScopeClashOnly, ClashRenderType: "UID", MaterialPool: false, Advanced: true},
	"IN-TYPE":            {RuleType: "IN-TYPE", Scope: ScopeClashOnly, ClashRenderType: "IN-TYPE", MaterialPool: false, Advanced: true},
	"IN-USER":            {RuleType: "IN-USER", Scope: ScopeClashOnly, ClashRenderType: "IN-USER", MaterialPool: false, Advanced: true},
	"IN-NAME":            {RuleType: "IN-NAME", Scope: ScopeClashOnly, ClashRenderType: "IN-NAME", MaterialPool: false, Advanced: true},
	"SUB-RULE":           {RuleType: "SUB-RULE", Scope: ScopeClashOnly, ClashRenderType: "SUB-RULE", MaterialPool: false, Advanced: true},
	"RULE-SET":           {RuleType: "RULE-SET", Scope: ScopeClashOnly, ClashRenderType: "RULE-SET", SupportsNoResolve: true, MaterialPool: false, Advanced: true},
	"AND":                {RuleType: "AND", Scope: ScopeClashOnly, ClashRenderType: "AND", MaterialPool: false, Advanced: true},
	"OR":                 {RuleType: "OR", Scope: ScopeClashOnly, ClashRenderType: "OR", MaterialPool: false, Advanced: true},
	"NOT":                {RuleType: "NOT", Scope: ScopeClashOnly, ClashRenderType: "NOT", MaterialPool: false, Advanced: true},
	"MATCH":              {RuleType: "MATCH", Scope: ScopeClashOnly, ClashRenderType: "MATCH", MaterialPool: false, Advanced: true},
	"USER-AGENT":         {RuleType: "USER-AGENT", Scope: ScopeSrOnly, ClashRenderType: "", SRRenderType: "USER-AGENT", MaterialPool: true, Advanced: true},
}

// SupportsAndMapLegacy 按旧规则类型返回目标映射结果。
func SupportsAndMapLegacy(ruleType string, target Target) MappingResult {
	typ := normalizeRuleType(ruleType)
	cap, ok := legacyCapabilityMap[typ]
	if !ok {
		return MappingResult{Supported: false, TargetScope: ScopeUnsupported}
	}
	switch target {
	case TargetClash:
		if cap.ClashRenderType == "" {
			return MappingResult{Supported: false, RenderType: "", SupportsNoResolve: cap.SupportsNoResolve, TargetScope: cap.Scope}
		}
		return MappingResult{Supported: true, RenderType: cap.ClashRenderType, ConversionKind: "direct", SupportsNoResolve: cap.SupportsNoResolve, TargetScope: cap.Scope}
	case TargetSR:
		if cap.SRRenderType == "" {
			return MappingResult{Supported: false, RenderType: "", SupportsNoResolve: cap.SupportsNoResolve, TargetScope: cap.Scope}
		}
		return MappingResult{Supported: true, RenderType: cap.SRRenderType, ConversionKind: "direct", SupportsNoResolve: cap.SupportsNoResolve, TargetScope: cap.Scope}
	default:
		return MappingResult{Supported: false, TargetScope: ScopeUnsupported}
	}
}

// LegacyMetadata returns sorted metadata for frontend type dropdowns.
func LegacyMetadata() []LegacyCapability {
	types := make([]string, 0, len(legacyCapabilityMap))
	for k := range legacyCapabilityMap {
		types = append(types, k)
	}
	sortStrings(types)
	out := make([]LegacyCapability, 0, len(types))
	for _, t := range types {
		out = append(out, legacyCapabilityMap[t])
	}
	return out
}

func normalizeRuleType(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 'a' + 'A'
		}
	}
	return string(b)
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
