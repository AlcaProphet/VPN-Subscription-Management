package assembly

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"vpn-sub/internal/rulespec"
	"vpn-sub/internal/store"
)

type noResolvePoolEntry struct {
	Type      string
	Value     string
	NoResolve bool
}

func insertNoResolvePool(t *testing.T, st *store.Store, entries ...noResolvePoolEntry) int64 {
	t.Helper()
	ctx := context.Background()
	res, err := st.DB().ExecContext(ctx, `INSERT INTO rule_pools (name) VALUES ('no-resolve-测试池')`)
	if err != nil {
		t.Fatalf("插入素材池失败: %v", err)
	}
	poolID, _ := res.LastInsertId()
	srcRes, err := st.DB().ExecContext(ctx,
		`INSERT INTO rule_pool_sources (pool_id, kind, source_mode, sort_order) VALUES (?,'manual','auto',-1)`, poolID)
	if err != nil {
		t.Fatalf("插入 manual 来源失败: %v", err)
	}
	sourceID, _ := srcRes.LastInsertId()
	for i, e := range entries {
		family, matcher, ok := rulespec.CanonicalizeLegacyType(e.Type)
		if !ok {
			t.Fatalf("未知规则类型: %s", e.Type)
		}
		rule := rulespec.CanonicalRule{Family: family, Matcher: matcher, Value: e.Value, Options: rulespec.RuleOptions{NoResolve: e.NoResolve}}
		optsRaw := `{}`
		if e.NoResolve {
			optsRaw = `{"no_resolve":true}`
		}
		crRes, err := st.DB().ExecContext(ctx,
			`INSERT INTO pool_canonical_rules (pool_id, semantic_key, family, matcher, value, options_json) VALUES (?,?,?,?,?,?)`,
			poolID, rule.SemanticKey(), string(rule.Family), string(rule.Matcher), rule.Value, optsRaw)
		if err != nil {
			t.Fatalf("插入 canonical 失败: %v", err)
		}
		crID, _ := crRes.LastInsertId()
		if _, err := st.DB().ExecContext(ctx,
			`INSERT INTO pool_rule_origins (pool_id, canonical_rule_id, source_id, snapshot_id, sort_order, raw_line, line_no) VALUES (?,?,?,NULL,?,?,0)`,
			poolID, crID, sourceID, i, e.Type+","+e.Value); err != nil {
			t.Fatalf("插入 origin 失败: %v", err)
		}
	}
	return poolID
}

func TestClashRenderUsesInstanceNoResolve(t *testing.T) {
	svc, st, _ := newTestService(t)
	pid := insertPlatform(t, st, "yaml")
	insertManualNode(t, st, "节点A", "vless", map[string]any{"uuid": "11111111-2222-3333-4444-555555555555"})
	insertGroup(t, st, "组A", "select", []string{"节点A"}, nil, true, false)
	poolID := insertNoResolvePool(t, st,
		noResolvePoolEntry{Type: "IP-CIDR", Value: "1.2.3.0/24", NoResolve: false},
		noResolvePoolEntry{Type: "IP-CIDR", Value: "10.0.0.0/8", NoResolve: true},
	)
	res, err := svc.Render(context.Background(), GenerateInput{
		TargetSyntax: ClashYAML, PlatformID: pid,
		NodeNames: []string{"节点A"}, GroupNames: []string{"组A"}, OverseasMembers: []string{"节点A"}, FallbackGroupMembers: []string{"🚀直接连接", "🌎国外流量"},
		Pools: []PoolSelection{{PoolID: poolID, Target: "组A"}},
	})
	if err != nil {
		t.Fatalf("Clash Render 失败: %v", err)
	}
	content := string(res.Content)
	if strings.Contains(content, "IP-CIDR,1.2.3.0/24,组A,no-resolve") {
		t.Errorf("未设置 no-resolve 的规则不应输出后缀:\n%s", content)
	}
	if !strings.Contains(content, "IP-CIDR,10.0.0.0/8,组A,no-resolve") {
		t.Errorf("设置 no-resolve 的规则应输出后缀:\n%s", content)
	}
	// 新 Clash plan 的每条规则必须显式包含 boolean no_resolve（false 也要出现）。
	var plan struct {
		Rules []map[string]any `json:"rules"`
	}
	if err := json.Unmarshal(res.RenderPlan, &plan); err != nil {
		t.Fatalf("解析 render plan 失败: %v", err)
	}
	if len(plan.Rules) != 2 {
		t.Fatalf("plan rules 数量应为 2: %d", len(plan.Rules))
	}
	for i, r := range plan.Rules {
		v, ok := r["no_resolve"]
		if !ok {
			t.Fatalf("第 %d 条新 plan 规则缺少显式 no_resolve: %#v", i, r)
		}
		if _, ok := v.(bool); !ok {
			t.Fatalf("第 %d 条 no_resolve 应为 boolean: %#v", i, r)
		}
	}
}

func TestSrConfUsesInstanceNoResolve(t *testing.T) {
	svc, st, _ := newTestService(t)
	rid := insertRule(t, st)
	poolID := insertNoResolvePool(t, st,
		noResolvePoolEntry{Type: "IP-CIDR", Value: "1.2.3.0/24", NoResolve: false},
		noResolvePoolEntry{Type: "IP-CIDR", Value: "10.0.0.0/8", NoResolve: true},
	)
	res, err := svc.Render(context.Background(), GenerateInput{
		TargetSyntax: SrConf, RuleID: rid,
		Pools: []PoolSelection{{PoolID: poolID, Target: "PROXY"}}, FinalDirection: "DIRECT",
	})
	if err != nil {
		t.Fatalf("SR conf Render 失败: %v", err)
	}
	content := string(res.Content)
	if strings.Contains(content, "IP-CIDR,1.2.3.0/24,PROXY,no-resolve") {
		t.Errorf("未设置 no-resolve 的 SR 规则不应输出后缀:\n%s", content)
	}
	if !strings.Contains(content, "IP-CIDR,10.0.0.0/8,PROXY,no-resolve") {
		t.Errorf("设置 no-resolve 的 SR 规则应输出后缀:\n%s", content)
	}
}

func TestRenderClashPlanHistoricalAndExplicitCompatibility(t *testing.T) {
	// 历史字段缺失/null 时按类型补 no-resolve；显式 false 时不补，显式 true 且类型支持时补。
	base := `{"head":{},"proxy_groups":[{"name":"组A","type":"select","proxies":["DIRECT"]}],"rules":%s}`
	cases := []struct {
		name     string
		ruleJSON string
		want     string
	}{
		{name: "missing", ruleJSON: `[{"type":"IP-CIDR","value":"1.2.3.0/24","target":"组A"}]`, want: "IP-CIDR,1.2.3.0/24,组A,no-resolve"},
		{name: "null", ruleJSON: `[{"type":"IP-CIDR","value":"1.2.3.0/24","target":"组A","no_resolve":null}]`, want: "IP-CIDR,1.2.3.0/24,组A,no-resolve"},
		{name: "false", ruleJSON: `[{"type":"IP-CIDR","value":"1.2.3.0/24","target":"组A","no_resolve":false}]`, want: "IP-CIDR,1.2.3.0/24,组A"},
		{name: "true", ruleJSON: `[{"type":"IP-CIDR","value":"10.0.0.0/8","target":"组A","no_resolve":true}]`, want: "IP-CIDR,10.0.0.0/8,组A,no-resolve"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte(strings.ReplaceAll(base, "%s", tc.ruleJSON))
			content, err := RenderClashPlan(raw, nil, nil, "")
			if err != nil {
				t.Fatalf("RenderClashPlan 失败: %v", err)
			}
			if !strings.Contains(string(content), tc.want) {
				t.Errorf("期望包含 %q:\n%s", tc.want, content)
			}
		})
	}
}

func TestDowngradeRuleLinesPreservesExistingNoResolveOnly(t *testing.T) {
	lines := []string{
		"IP-CIDR,1.2.3.0/24,已删组",
		"IP-CIDR,10.0.0.0/8,已删组,no-resolve",
		"DOMAIN,example.com,已删组",
	}
	got := downgradeRuleLines(lines, map[string]bool{})
	if len(got) != 3 {
		t.Fatalf("降级行数异常: %d", len(got))
	}
	if got[0] != "IP-CIDR,1.2.3.0/24,DIRECT" {
		t.Errorf("无后缀行不应被类型补后缀: %q", got[0])
	}
	if got[1] != "IP-CIDR,10.0.0.0/8,DIRECT,no-resolve" {
		t.Errorf("已有后缀行应保留后缀: %q", got[1])
	}
	if got[2] != "DOMAIN,example.com,DIRECT" {
		t.Errorf("DOMAIN 行降级异常: %q", got[2])
	}
}
