package assembly

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"vpn-sub/internal/config"
	"vpn-sub/internal/store"
)

const encPrefix = "enc:v1:"

// 业务错误。
var (
	ErrBadRequest = errors.New("参数错误")
)

// Service 装配渲染服务（只读加载 + 纯函数渲染 + 蓝图落库辅助）。
type Service struct {
	store *store.Store
	cfg   *config.Service
	log   *slog.Logger
}

// NewService 构造装配渲染服务。
func NewService(st *store.Store, cfg *config.Service, lg *slog.Logger) *Service {
	return &Service{store: st, cfg: cfg, log: lg}
}

// Preview 渲染预览（不落库）。
func (s *Service) Preview(ctx context.Context, in GenerateInput) (*PreviewResult, error) {
	ld, err := s.loadData(ctx, in)
	if err != nil {
		return nil, err
	}
	if err := s.validate(in, ld); err != nil {
		return nil, err
	}
	res, err := s.render(in, ld)
	if err != nil {
		return nil, err
	}
	return &PreviewResult{Content: res.Content, Skipped: res.Skipped, Warnings: s.Warnings(in, res), Receipt: res.Receipt}, nil
}

// Render 渲染并返回结构化计划（供 generate 使用）。
func (s *Service) Render(ctx context.Context, in GenerateInput) (*RenderResult, error) {
	ld, err := s.loadData(ctx, in)
	if err != nil {
		return nil, err
	}
	if err := s.validate(in, ld); err != nil {
		return nil, err
	}
	return s.render(in, ld)
}

// SaveBlueprintTx 在版本事务内写入 assembly_blueprints（version_id 1:1）。
func (s *Service) SaveBlueprintTx(ctx context.Context, tx *sql.Tx, versionID int64, in GenerateInput, renderPlan json.RawMessage) error {
	if in.FixedParams == nil {
		in.FixedParams = NewOrderedMap()
	}
	// SR conf 的 FINAL 方向按 Build5 要求并入 fixed_params_json（仅存储，不影响渲染输出）。
	fixedForStorage := NewOrderedMap()
	for _, k := range in.FixedParams.Keys() {
		v, _ := in.FixedParams.Get(k)
		fixedForStorage.Set(k, v)
	}
	if in.TargetSyntax == SrConf {
		fixedForStorage.Set("final_direction", in.FinalDirection)
	}
	fixed, err := json.Marshal(fixedForStorage)
	if err != nil {
		return err
	}
	var platformID, ruleID any
	if in.TargetSyntax == SrConf {
		ruleID = in.RuleID
	} else {
		platformID = in.PlatformID
	}
	xrayCandidates, err := s.xrayCandidateNamesTx(ctx, tx, in.NodeNames)
	if err != nil {
		return err
	}
	selection := map[string]any{
		"node_names":             in.NodeNames,
		"group_names":            in.GroupNames,
		"group_node_orders":      in.GroupNodeOrders,
		"group_member_orders":    in.GroupMemberOrders,
		"overseas_members":       in.OverseasMembers,
		"fallback_group_members": in.FallbackGroupMembers,
		"pools":                  in.Pools,
		"final_direction":        in.FinalDirection,
		"xray_candidates":        xrayCandidates,
		"overlay":                in.Overlay,
	}
	sel, err := json.Marshal(selection)
	if err != nil {
		return err
	}
	custom, err := json.Marshal(in.CustomRules)
	if err != nil {
		return err
	}
	if renderPlan == nil {
		renderPlan = json.RawMessage(`{}`)
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO assembly_blueprints (version_id, target_syntax, fixed_params_json, selection_json, custom_rules_json, render_plan_json, platform_id, rule_id)
		 VALUES (?,?,?,?,?,?,?,?)`,
		versionID, string(in.TargetSyntax), string(fixed), string(sel), string(custom), string(renderPlan), platformID, ruleID)
	return err
}

// xrayCandidateNamesTx 返回本次勾选节点中 source='xray' 的稳定名列表。
func (s *Service) xrayCandidateNamesTx(ctx context.Context, tx *sql.Tx, names []string) ([]string, error) {
	out := make([]string, 0, len(names))
	for _, name := range names {
		var n int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM nodes WHERE name = ? AND source = 'xray'`, name).Scan(&n); err != nil {
			return nil, fmt.Errorf("查询 xray 候选节点失败: %w", err)
		}
		if n > 0 {
			out = append(out, name)
		}
	}
	return out, nil
}

// Warnings 从渲染跳过项等生成用户提示。
func (s *Service) Warnings(in GenerateInput, res *RenderResult) []string {
	var warnings []string
	// 规则类警告仅对会产出规则正文的 Clash YAML 与 Shadowrocket 分流规则输出；
	// sr-subs / generic-subs 是纯节点订阅，不渲染规则内容，避免无关警告。
	if (in.TargetSyntax == ClashYAML || in.TargetSyntax == SrConf) &&
		len(in.Pools) == 0 && len(in.CustomRules) == 0 {
		warnings = append(warnings, "未选择任何规则素材池或手动规则，将生成空规则")
	}
	for _, sk := range res.Skipped {
		switch sk.Kind {
		case "node":
			warnings = append(warnings, "节点已跳过："+sk.Name+"（"+sk.Reason+"）")
		case "rule":
			warnings = append(warnings, "规则已跳过："+sk.Name+"（"+sk.Reason+"）")
		}
	}
	for _, issue := range res.Issues {
		warnings = append(warnings, fmt.Sprintf("产物自检[%s] %s: %s", issue.Severity, issue.Path, issue.Message))
	}
	for _, diag := range res.Diagnostics {
		if diag.Severity == "warn" || diag.Severity == "info" || diag.Severity == "unverified" {
			warnings = append(warnings, fmt.Sprintf("节点诊断[%s] %s: %s", diag.Target, diag.Code, diag.Message))
		}
	}
	return warnings
}

// hasXrayNode 判断本次装配是否勾选 xray 来源节点。
func hasXrayNode(ld *loadedData) bool {
	for _, n := range ld.nodes {
		if n.Source == "xray" {
			return true
		}
	}
	return false
}

// containsString 简单包含判断。
func containsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
