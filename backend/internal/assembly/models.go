// Package assembly 提供基础模式装配器渲染内核：四语法渲染、快照模型、链接编码与校验。
// 本包只读数据库，不直接写库；蓝图落库由上层在版本事务内调用 SaveBlueprintTx。
package assembly

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// TargetSyntax 装配目标语法。
type TargetSyntax string

const (
	ClashYAML   TargetSyntax = "clash-yaml"
	SrSubs      TargetSyntax = "sr-subs"
	GenericSubs TargetSyntax = "generic-subs"
	SrConf      TargetSyntax = "sr-conf"
)

// OrderedMap 保留 JSON 对象键序的 map，用于头部表单值。
type OrderedMap struct {
	keys   []string
	values map[string]any
}

// NewOrderedMap 创建空的有序 map。
func NewOrderedMap() *OrderedMap {
	return &OrderedMap{values: map[string]any{}}
}

// Set 写入/更新键值；新键追加到末尾。
func (m *OrderedMap) Set(key string, value any) *OrderedMap {
	if m.values == nil {
		m.values = map[string]any{}
	}
	if _, ok := m.values[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
	return m
}

// Get 读取键值。
func (m *OrderedMap) Get(key string) (any, bool) {
	if m == nil {
		return nil, false
	}
	v, ok := m.values[key]
	return v, ok
}

// Keys 返回按插入顺序排列的键。
func (m *OrderedMap) Keys() []string {
	if m == nil {
		return nil
	}
	return m.keys
}

// Len 返回键数量。
func (m *OrderedMap) Len() int {
	if m == nil {
		return 0
	}
	return len(m.keys)
}

// UnmarshalJSON 按 JSON 对象原始顺序解析。
func (m *OrderedMap) UnmarshalJSON(data []byte) error {
	if m.values == nil {
		m.values = map[string]any{}
	}
	m.keys = nil
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return errors.New("fixed_params 必须是 JSON 对象")
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return err
		}
		key, ok := keyTok.(string)
		if !ok {
			return errors.New("fixed_params 键必须是字符串")
		}
		var val any
		if err := dec.Decode(&val); err != nil {
			return err
		}
		m.Set(key, val)
	}
	if _, err := dec.Token(); err != nil {
		return err
	}
	return nil
}

// MarshalJSON 按插入顺序输出 JSON 对象。
func (m *OrderedMap) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte(`{}`), nil
	}
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range m.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, err := json.Marshal(k)
		if err != nil {
			return nil, fmt.Errorf("序列化 OrderedMap 键失败: %w", err)
		}
		buf.Write(kb)
		buf.WriteByte(':')
		vb, err := json.Marshal(m.values[k])
		if err != nil {
			return nil, err
		}
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

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
	TargetSyntax    TargetSyntax    `json:"target_syntax"`
	PlatformID      int64           `json:"platform_id"`
	RuleID          int64           `json:"rule_id"`
	FixedParams     *OrderedMap     `json:"fixed_params"`
	NodeNames       []string        `json:"node_names"`
	GroupNames      []string        `json:"group_names"`
	OverseasMembers []string        `json:"overseas_members"`
	Pools           []PoolSelection `json:"pools"`
	CustomRules     []RuleLine      `json:"custom_rules"`
	FinalDirection  string          `json:"final_direction"`
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
