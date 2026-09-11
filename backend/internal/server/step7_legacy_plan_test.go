package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"vpn-sub/internal/log"
	"vpn-sub/internal/tasks"
	"vpn-sub/internal/xray"
)

func TestLegacyStringArrayClashPlanDownloadFallback(t *testing.T) {
	ctx := context.Background()
	_, st, cfg := newAssemblyTestEnv(t)

	// 历史 Build6 修复前的 render_plan_json：manual_proxies/proxy_groups/rules 均为字符串数组。
	legacyPlan, err := json.Marshal(map[string]any{
		"manual_proxies": []string{},
		"proxy_groups":   []string{},
		"rules":          []string{"DOMAIN-SUFFIX,example.com,PROXY"},
	})
	if err != nil {
		t.Fatalf("构造历史 plan 失败: %v", err)
	}

	res, err := st.DB().ExecContext(ctx,
		`INSERT INTO platforms (slug, name, product_type) VALUES ('legacy-plan', '历史 plan 平台', 'yaml')`)
	if err != nil {
		t.Fatalf("插入平台失败: %v", err)
	}
	platformID, _ := res.LastInsertId()
	res, err = st.DB().ExecContext(ctx,
		`INSERT INTO subscriptions (slug, name, platform_id, product_type, current_version) VALUES ('legacy-plan-sub', '历史 plan 订阅', ?, 'yaml', 1)`, platformID)
	if err != nil {
		t.Fatalf("插入订阅失败: %v", err)
	}
	subID, _ := res.LastInsertId()
	res, err = st.DB().ExecContext(ctx,
		`INSERT INTO versions (owner_type, owner_id, version_no, file_path) VALUES ('subscription', ?, 1, 'legacy-plan.yaml')`, subID)
	if err != nil {
		t.Fatalf("插入版本失败: %v", err)
	}
	versionID, _ := res.LastInsertId()
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO assembly_blueprints (version_id, target_syntax, fixed_params_json, selection_json, custom_rules_json, render_plan_json)
		 VALUES (?, 'clash-yaml', '{}', '{"xray_candidates":[]}', '[]', ?)`, versionID, string(legacyPlan)); err != nil {
		t.Fatalf("插入蓝图失败: %v", err)
	}

	lg := log.New("error", "console")
	instSvc := xray.NewInstanceService(st, lg, tasks.NewRegistry())
	creds := xray.NewCredentialService(st, cfg)
	syncSvc := xray.NewSyncService(st, cfg, creds, instSvc, tasks.NewRegistry(), lg)
	content := []byte("proxies: []\nrules:\n  - # {{xray_nodes}}\n")
	out, err := renderUserSubscription(ctx, st, cfg, syncSvc, creds, subID, 0, content, "legacy-plan.yaml")
	if err != nil {
		t.Fatalf("旧字符串数组 plan 下载重渲染应走回退路径，实际失败: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "# Xray 高级模式未启用") {
		t.Fatalf("回退路径未替换占位注释:\n%s", text)
	}
	if strings.Contains(text, "DOMAIN-SUFFIX,example.com,PROXY") {
		t.Fatalf("旧 plan 不应被当作新结构误渲染:\n%s", text)
	}
}
