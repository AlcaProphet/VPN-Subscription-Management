package assembly

import (
	"context"
	"strings"
	"testing"
)

// TestCheckXrayReferences 装配快照中同名 Xray 节点视为已重绑，失配节点返回悬空提示。
func TestCheckXrayReferences(t *testing.T) {
	svc, st, _ := newTestService(t)
	ctx := context.Background()
	pid := insertPlatform(t, st, "yaml")
	var subID int64
	if err := st.DB().QueryRowContext(ctx,
		`SELECT id FROM subscriptions WHERE platform_id=?`, pid).Scan(&subID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO versions (owner_type, owner_id, version_no, file_path) VALUES ('subscription',?,1,'x.yaml')`, subID); err != nil {
		t.Fatal(err)
	}
	var versionID int64
	if err := st.DB().QueryRowContext(ctx,
		`SELECT id FROM versions WHERE owner_type='subscription' AND owner_id=? AND version_no=1`, subID).Scan(&versionID); err != nil {
		t.Fatal(err)
	}
	insertXrayNode(t, st, "instance-x-in-a", "", "in-a")
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO assembly_blueprints (version_id, target_syntax, fixed_params_json, selection_json, custom_rules_json, render_plan_json)
		 VALUES (?, 'clash-yaml', '{}', ?, '[]', ?)`,
		versionID,
		`{"node_names":["instance-x-in-a","instance-x-missing"],"xray_candidates":["instance-x-in-a","instance-x-missing"]}`,
		`{"manual_proxies":[],"proxy_groups":[{"name":"组A","type":"select","proxies":["instance-x-in-a","instance-x-missing"]}],"rules":[]}`,
	); err != nil {
		t.Fatal(err)
	}
	hints, err := svc.CheckXrayReferences(ctx)
	if err != nil {
		t.Fatalf("CheckXrayReferences 失败: %v", err)
	}
	foundMissing := false
	for _, h := range hints {
		if strings.Contains(h, "instance-x-missing") && strings.Contains(h, "未匹配") {
			foundMissing = true
		}
		if strings.Contains(h, "instance-x-in-a") && strings.Contains(h, "未匹配") {
			t.Fatalf("同名已恢复节点不应提示未匹配: %v", hints)
		}
	}
	if !foundMissing {
		t.Fatalf("应提示失配节点 instance-x-missing，实际 %v", hints)
	}
}
