// Package assembly 提供基础模式装配器渲染内核：四语法渲染、快照模型、链接编码与校验。
// 本包只读数据库，不直接写库；蓝图落库由上层在版本事务内调用 SaveBlueprintTx。
package assembly

import "encoding/json"

// TargetSyntax 装配目标语法。
type TargetSyntax string

const (
	ClashYAML   TargetSyntax = "clash-yaml"
	SrSubs      TargetSyntax = "sr-subs"
	GenericSubs TargetSyntax = "generic-subs"
	SrConf      TargetSyntax = "sr-conf"
)

// PoolSelection 素材池勾选（有序数组；Target 为规则目标组名或 PROXY/DIRECT）。
type PoolSelection struct {
	PoolID int64  `json:"pool_id"`
	Target string `json:"target"`
}

// RuleLine 手动补充规则行。
type RuleLine struct {
	RuleType   string `json:"rule_type"`
	MatchValue string `json:"match_value"`
	Target     string `json:"target"`
}

// GenerateInput 装配生成入参（对应 Design2 §5.9 selection/fixed/custom 映射）。
type GenerateInput struct {
	TargetSyntax     TargetSyntax     `json:"target_syntax"`
	PlatformID       int64            `json:"platform_id"`
	RuleID           int64            `json:"rule_id"`
	FixedParams      map[string]any   `json:"fixed_params"`
	NodeNames        []string         `json:"node_names"`
	GroupNames       []string         `json:"group_names"`
	OverseasMembers  []string         `json:"overseas_members"`
	Pools            []PoolSelection  `json:"pools"`
	CustomRules      []RuleLine       `json:"custom_rules"`
	FinalDirection   string           `json:"final_direction"`
}

// SkipItem 跳过项（不可转链接等）。
type SkipItem struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// RenderResult 渲染结果。
type RenderResult struct {
	Content    []byte          `json:"content"`
	Skipped    []SkipItem      `json:"skipped"`
	RenderPlan json.RawMessage `json:"render_plan,omitempty"`
}

// PreviewResult 预览/生成前置结果（含提示与名称变更对照）。
type PreviewResult struct {
	Content     []byte            `json:"content"`
	Skipped     []SkipItem        `json:"skipped"`
	Warnings    []string          `json:"warnings"`
	NameChanged map[string]string `json:"name_changed,omitempty"`
}
