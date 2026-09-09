// types.go：Step 2 解析管线的结果与诊断类型。
package pool

import (
	"errors"

	"vpn-sub/internal/rulespec"
)

// SourceMode 是用户为每个 URL 选择的来源模式。
type SourceMode string

const (
	SourceModeClash        SourceMode = "clash"
	SourceModeShadowrocket SourceMode = "shadowrocket"
	SourceModeAuto         SourceMode = "auto"
)

// DetectedFormat 是整份文档的详细格式。
type DetectedFormat string

const (
	FormatLegacyDomainText    DetectedFormat = "legacy-domain-text"
	FormatPlainDomainText     DetectedFormat = "plain-domain-text"
	FormatMihomoDomainYAML    DetectedFormat = "mihomo-domain-yaml"
	FormatMihomoIPCIDRYAML    DetectedFormat = "mihomo-ipcidr-yaml"
	FormatMihomoClassicalYAML DetectedFormat = "mihomo-classical-yaml"
	FormatTypedRuleText       DetectedFormat = "typed-rule-text"
	FormatPlainIPCIDRText     DetectedFormat = "plain-ipcidr-text"
	FormatSingBoxSourceJSON   DetectedFormat = "sing-box-source-json"
)

// ParseDiagnostic 表示单条诊断。
type ParseDiagnostic struct {
	Line    int    `json:"line"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Raw     string `json:"raw"`
}

// RuleOriginMeta 保存单个解析位置的原始证据。
type RuleOriginMeta struct {
	Line  int    `json:"line"`
	Raw   string `json:"raw"`
	Order int    `json:"order"`
}

// ParsedRule 保存已解析规则与其来源位置证据。
type ParsedRule struct {
	Rule   rulespec.CanonicalRule
	Origin RuleOriginMeta
}

// RuleCountStat 是 v1 stats 的 family/matcher/scope 分项统计。
type RuleCountStat struct {
	Family     string `json:"family"`
	Matcher    string `json:"matcher"`
	Scope      string `json:"scope"`
	Accepted   int    `json:"accepted"`
	Excluded   int    `json:"excluded"`
	Rejected   int    `json:"rejected"`
	Duplicates int    `json:"duplicates"`
}

// ParseResult 是单份来源文档的解析与来源准入结果。
type ParseResult struct {
	Format               DetectedFormat           `json:"format"`
	Profile              string                   `json:"profile"` // common / clash / shadowrocket / unknown
	Rules                []rulespec.CanonicalRule `json:"-"`       // 兼容投影：唯一 Canonical，由 Items 派生
	Items                []ParsedRule             `json:"-"`       // 唯一与重复的全部有效 origin，按原始顺序
	Diagnostics          []ParseDiagnostic        `json:"diagnostics"`
	Input                int                      `json:"input"`
	Recognized           int                      `json:"recognized"`
	Accepted             int                      `json:"accepted"`
	Excluded             int                      `json:"excluded"`
	Rejected             int                      `json:"rejected"`
	Duplicates           int                      `json:"duplicates"`
	RuleCounts           []RuleCountStat          `json:"-"`
	UnclassifiedRejected int                      `json:"-"`
	EvidenceCodes        []string                 `json:"-"`
}

// 解析/探测硬错误。
var (
	ErrUnrecognizedSource        = errors.New("unrecognized source")
	ErrAmbiguousSourceFormat     = errors.New("ambiguous source format")
	ErrConflictingDocumentFormat = errors.New("conflicting document format")
	ErrMixedPlatformSource       = errors.New("mixed platform source")
	ErrHTMLSource                = errors.New("html source")
	ErrNoAcceptedRules           = errors.New("no accepted rules")
	ErrThresholdNotMet           = errors.New("recognition threshold not met")
)
