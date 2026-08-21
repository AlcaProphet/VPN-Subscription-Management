// rebind.go：装配快照 Xray 引用重绑辅助（Build7 Step2 导入后处理）。
// 快照按 nodes.name 稳定键保存；导入后同名节点恢复即视为已重绑，失配按悬空容错并返回提示。
package assembly

import (
	"context"
	"encoding/json"
	"fmt"
)

// CheckXrayReferences 扫描全部装配快照，核对 selection_json / render_plan_json 中的 Xray 节点稳定名是否已存在。
// 不修改历史快照；仅返回失配提示供导入任务完成信息展示。
func (s *Service) CheckXrayReferences(ctx context.Context) ([]string, error) {
	type blueprintRow struct {
		id          int64
		versionID   int64
		selection   string
		plan        string
		targetSyntax string
	}
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT id, version_id, selection_json, render_plan_json, target_syntax FROM assembly_blueprints ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("读取装配快照失败: %w", err)
	}
	var blueprints []blueprintRow
	for rows.Next() {
		var b blueprintRow
		if err := rows.Scan(&b.id, &b.versionID, &b.selection, &b.plan, &b.targetSyntax); err != nil {
			_ = rows.Close()
			return nil, err
		}
		blueprints = append(blueprints, b)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var hints []string
	for _, b := range blueprints {
		var sel struct {
			XrayCandidates []string `json:"xray_candidates"`
			NodeNames      []string `json:"node_names"`
		}
		if err := json.Unmarshal([]byte(b.selection), &sel); err != nil {
			hints = append(hints, fmt.Sprintf("蓝图 %d（版本 %d）selection_json 解析失败，跳过引用核对", b.id, b.versionID))
			continue
		}
		xraySet := map[string]bool{}
		for _, name := range sel.XrayCandidates {
			xraySet[name] = true
		}
		seen := map[string]bool{}
		check := func(name string) {
			if !xraySet[name] || seen[name] {
				return
			}
			seen[name] = true
			var n int
			if err := s.store.DB().QueryRowContext(ctx,
				`SELECT COUNT(*) FROM nodes WHERE name = ? AND source = 'xray'`, name).Scan(&n); err != nil {
				hints = append(hints, fmt.Sprintf("蓝图 %d（版本 %d）核对 Xray 节点 %q 失败: %v", b.id, b.versionID, name, err))
				return
			}
			if n == 0 {
				hints = append(hints, fmt.Sprintf("蓝图 %d（版本 %d）的 Xray 节点 %q 未匹配，已按悬空容错保留", b.id, b.versionID, name))
			}
		}
		for _, name := range sel.XrayCandidates {
			check(name)
		}
		var plan struct {
			ManualProxies []struct {
				Name string `json:"name"`
			} `json:"manual_proxies"`
			ProxyGroups []struct {
				Name    string   `json:"name"`
				Proxies []string `json:"proxies"`
			} `json:"proxy_groups"`
			NodeNames []string `json:"node_names"`
		}
		_ = json.Unmarshal([]byte(b.plan), &plan)
		manualNames := map[string]bool{}
		for _, mp := range plan.ManualProxies {
			if mp.Name != "" {
				manualNames[mp.Name] = true
			}
		}
		groupNames := map[string]bool{}
		for _, g := range plan.ProxyGroups {
			if g.Name != "" {
				groupNames[g.Name] = true
			}
		}
		for _, g := range plan.ProxyGroups {
			for _, p := range g.Proxies {
				if p == "" || p == "DIRECT" || groupNames[p] || manualNames[p] {
					continue
				}
				check(p)
			}
		}
		for _, name := range plan.NodeNames {
			check(name)
		}
	}
	return hints, nil
}
