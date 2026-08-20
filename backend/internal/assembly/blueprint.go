// blueprint.go：装配蓝图读取与引用校验（R14-16/N1：接入层不再直连 SQL）。
package assembly

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrSubscriptionNotFound 目标平台尚未创建订阅条目（接入层映射 400 文案）。
var ErrSubscriptionNotFound = errors.New("订阅不存在")

// BlueprintInvalidRef 蓝图失效引用项。
type BlueprintInvalidRef struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// BlueprintData 蓝图读取结果（含失效引用与显示名变化）。
type BlueprintData struct {
	TargetSyntax string                `json:"target_syntax"`
	VersionNo    int64                 `json:"version_no"`
	FixedParams  json.RawMessage       `json:"fixed_params"`
	Selection    json.RawMessage       `json:"selection"`
	CustomRules  json.RawMessage       `json:"custom_rules"`
	RenderPlan   json.RawMessage       `json:"render_plan"`
	PlatformID   *int64                `json:"platform_id,omitempty"`
	RuleID       *int64                `json:"rule_id,omitempty"`
	InvalidRefs  []BlueprintInvalidRef `json:"invalid_refs"`
	NameChanged  map[string]string     `json:"name_changed,omitempty"`
}

// FindSubscriptionByPlatform 按平台唯一订阅条目解析订阅 ID（R14-16/N1 下沉）。
func (s *Service) FindSubscriptionByPlatform(ctx context.Context, platformID int64) (int64, error) {
	var subID int64
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT id FROM subscriptions WHERE platform_id = ?`, platformID).Scan(&subID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrSubscriptionNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("读取平台订阅失败: %w", err)
	}
	return subID, nil
}

// GetBlueprint 读取装配蓝图并校验悬空引用、计算显示名变化（R14-16/N1 下沉）。
func (s *Service) GetBlueprint(ctx context.Context, versionID int64) (*BlueprintData, error) {
	var row struct {
		TargetSyntax    string
		VersionNo       int64
		FixedParamsJSON string
		SelectionJSON   string
		CustomRulesJSON string
		RenderPlanJSON  string
		PlatformID      *int64
		RuleID          *int64
	}
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT b.target_syntax, v.version_no, b.fixed_params_json, b.selection_json, b.custom_rules_json, b.render_plan_json, b.platform_id, b.rule_id
		 FROM assembly_blueprints b
		 JOIN versions v ON v.id = b.version_id
		 WHERE b.version_id = ?`, versionID).Scan(
		&row.TargetSyntax, &row.VersionNo, &row.FixedParamsJSON, &row.SelectionJSON, &row.CustomRulesJSON, &row.RenderPlanJSON,
		&row.PlatformID, &row.RuleID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("蓝图不存在")
	}
	if err != nil {
		return nil, fmt.Errorf("读取装配蓝图失败: %w", err)
	}
	var sel struct {
		NodeNames       []string        `json:"node_names"`
		GroupNames      []string        `json:"group_names"`
		Pools           []PoolSelection `json:"pools"`
		OverseasMembers []string        `json:"overseas_members"`
	}
	if err := json.Unmarshal([]byte(row.SelectionJSON), &sel); err != nil {
		return nil, fmt.Errorf("蓝图 selection_json 解析失败: %w", err)
	}
	out := &BlueprintData{
		TargetSyntax: row.TargetSyntax,
		VersionNo:    row.VersionNo,
		FixedParams:  json.RawMessage(row.FixedParamsJSON),
		Selection:    json.RawMessage(row.SelectionJSON),
		CustomRules:  json.RawMessage(row.CustomRulesJSON),
		RenderPlan:   json.RawMessage(row.RenderPlanJSON),
		PlatformID:   row.PlatformID,
		RuleID:       row.RuleID,
		InvalidRefs:  []BlueprintInvalidRef{},
		NameChanged:  map[string]string{},
	}
	checkExists := func(table, column string, value any, kind, name string) error {
		var n int
		if err := s.store.DB().QueryRowContext(ctx,
			`SELECT COUNT(*) FROM `+table+` WHERE `+column+` = ?`, value).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			out.InvalidRefs = append(out.InvalidRefs, BlueprintInvalidRef{Kind: kind, Name: name})
		}
		return nil
	}
	for _, name := range sel.NodeNames {
		if err := checkExists("nodes", "name", name, "node", name); err != nil {
			return nil, err
		}
	}
	for _, name := range sel.GroupNames {
		if err := checkExists("proxy_groups", "name", name, "group", name); err != nil {
			return nil, err
		}
	}
	for _, p := range sel.Pools {
		if err := checkExists("rule_pools", "id", p.PoolID, "pool", fmt.Sprintf("%d", p.PoolID)); err != nil {
			return nil, err
		}
	}
	for _, name := range sel.NodeNames {
		var dbName string
		var display sql.NullString
		err := s.store.DB().QueryRowContext(ctx,
			`SELECT name, display_name FROM nodes WHERE name = ?`, name).Scan(&dbName, &display)
		if err != nil {
			continue
		}
		render := dbName
		if display.Valid && display.String != "" {
			render = display.String
		}
		if render != name {
			out.NameChanged[name] = render
		}
	}
	return out, nil
}
