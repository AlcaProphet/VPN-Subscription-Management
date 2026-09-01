package server

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"vpn-sub/internal/assembly"
	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/internal/tasks"
	"vpn-sub/internal/xray"
	"vpn-sub/migrations"
)

// TestRenderUserSubscription10kRules 验证用户订阅动态渲染在 1 万规则规模下仍满足 <500ms 性能基准。
func TestRenderUserSubscription10kRules(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(ctx, migrations.FS); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	if err := cfg.Set(ctx, config.KeySigningKey, "0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Set(ctx, config.KeyAdvancedMode, "true"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO platforms (slug, name, product_type) VALUES ('plat','平台','yaml')`); err != nil {
		t.Fatal(err)
	}
	var platformID int64
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM platforms WHERE slug='plat'`).Scan(&platformID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO subscriptions (slug, name, platform_id, current_version, product_type) VALUES ('sub','订阅',?,1,'yaml')`, platformID); err != nil {
		t.Fatal(err)
	}
	var subID int64
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM subscriptions WHERE slug='sub'`).Scan(&subID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO versions (owner_type, owner_id, version_no, file_path) VALUES ('subscription',?,1,'x.yaml')`, subID); err != nil {
		t.Fatal(err)
	}
	var versionID int64
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM versions WHERE owner_type='subscription' AND owner_id=? AND version_no=1`, subID).Scan(&versionID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO users (username, email, role, user_source, status, group_id) VALUES ('u','u@x.com','user','local','active',NULL)`); err != nil {
		t.Fatal(err)
	}
	var userID int64
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM users WHERE email='u@x.com'`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	rules := make([]assembly.ClashPlanRule, 10000)
	for i := range rules {
		rules[i] = assembly.ClashPlanRule{Type: "DOMAIN-SUFFIX", Value: fmt.Sprintf("example%d.com", i), Target: "PROXY"}
	}
	plan := assembly.ClashPlan{
		Head:          assembly.NewOrderedMap(),
		ManualProxies: []*assembly.OrderedMap{},
		ProxyGroups:   []assembly.ClashPlanGroup{},
		Rules:         rules,
		Fallback:      []string{"GEOIP,CN,DIRECT", "MATCH,🛟无法归属的流量"},
	}
	planRaw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO assembly_blueprints (version_id, target_syntax, fixed_params_json, selection_json, custom_rules_json, render_plan_json)
		 VALUES (?, 'clash-yaml', '{}', '{"xray_candidates":[]}', '[]', ?)`, versionID, string(planRaw)); err != nil {
		t.Fatal(err)
	}
	instSvc := xray.NewInstanceService(st, log.New("error", "console"), tasks.NewRegistry())
	creds := xray.NewCredentialService(st, cfg)
	syncSvc := xray.NewSyncService(st, cfg, creds, instSvc, tasks.NewRegistry(), log.New("error", "console"))
	start := time.Now()
	if _, err := renderUserSubscription(ctx, st, cfg, syncSvc, creds, subID, userID, []byte("# {{xray_nodes}}\n"), "x.yaml"); err != nil {
		t.Fatalf("renderUserSubscription 失败: %v", err)
	}
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Fatalf("用户订阅 1 万规则渲染超过 500ms: %v", d)
	}
}
