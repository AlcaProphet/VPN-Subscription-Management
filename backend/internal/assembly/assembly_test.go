package assembly

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	gyaml "github.com/goccy/go-yaml"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/node"
	"vpn-sub/internal/rulespec"
	"vpn-sub/internal/store"
	"vpn-sub/migrations"
)

func newTestService(t testing.TB) (*Service, *store.Store, *config.Service) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), migrations.FS); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	if err := cfg.Set(context.Background(), config.KeySigningKey, "test-signing-key-0123456789abcdef"); err != nil {
		t.Fatalf("写入签名密钥失败: %v", err)
	}
	svc := NewService(st, cfg, log.New("error", "console"))
	return svc, st, cfg
}

func insertPlatform(t *testing.T, st *store.Store, productType string) int64 {
	t.Helper()
	res, err := st.DB().ExecContext(context.Background(),
		`INSERT INTO platforms (slug, name, product_type) VALUES (?, ?, ?)`,
		"platform-"+strings.ToLower(productType), "测试平台", productType)
	if err != nil {
		t.Fatalf("插入平台失败: %v", err)
	}
	id, _ := res.LastInsertId()
	if _, err := st.DB().ExecContext(context.Background(),
		`INSERT INTO subscriptions (slug, name, platform_id, product_type) VALUES (?,?,?,?)`,
		"sub-"+strings.ToLower(productType), "测试订阅", id, productType); err != nil {
		t.Fatalf("插入订阅失败: %v", err)
	}
	return id
}

func insertRule(t *testing.T, st *store.Store) int64 {
	t.Helper()
	res, err := st.DB().ExecContext(context.Background(),
		`INSERT INTO rules (slug, name) VALUES ('rule-test', '测试规则')`)
	if err != nil {
		t.Fatalf("插入规则失败: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func insertManualNode(t *testing.T, st *store.Store, name, protocol string, params map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(params)
	if _, err := st.DB().ExecContext(context.Background(),
		`INSERT INTO nodes (source, name, protocol, host, port, protocol_json) VALUES ('manual', ?, ?, ?, ?, ?)`,
		name, protocol, "example.com", 443, string(raw)); err != nil {
		t.Fatalf("插入 manual 节点失败: %v", err)
	}
}

func insertXrayNode(t *testing.T, st *store.Store, name, displayName, tag string) {
	t.Helper()
	res, err := st.DB().ExecContext(context.Background(),
		`INSERT INTO xray_instances (name, slug, api_addr) VALUES ('实例', 'instance-x', 'https://example.com')`)
	if err != nil {
		t.Fatalf("插入实例失败: %v", err)
	}
	instID, _ := res.LastInsertId()
	var display any
	if displayName != "" {
		display = displayName
	}
	if _, err := st.DB().ExecContext(context.Background(),
		`INSERT INTO nodes (source, name, display_name, instance_id, tag, protocol, host, port, protocol_json, allocatable, missing)
		 VALUES ('xray', ?, ?, ?, ?, 'vless', 'xray.example.com', 443, '{"uuid":"xray-uuid","network":"tcp"}', 1, 0)`,
		name, display, instID, tag); err != nil {
		t.Fatalf("插入 xray 节点失败: %v", err)
	}
}

func insertGroup(t *testing.T, st *store.Store, name, groupType string, nodes, groups []string, enabled bool, preset bool) {
	t.Helper()
	def := map[string]any{"type": groupType, "nodes": nodes, "groups": groups}
	raw, _ := json.Marshal(def)
	gtype := "custom"
	presetKey := ""
	if preset {
		gtype = "preset"
		presetKey = name
	}
	en := 0
	if enabled {
		en = 1
	}
	if _, err := st.DB().ExecContext(context.Background(),
		`INSERT INTO proxy_groups (name, type, preset_key, enabled, definition_json) VALUES (?,?,?,?,?)`,
		name, gtype, presetKey, en, string(raw)); err != nil {
		t.Fatalf("插入代理组失败: %v", err)
	}
}

func insertPool(t *testing.T, st *store.Store, entries ...struct{ Type, Value string }) int64 {
	t.Helper()
	res, err := st.DB().ExecContext(context.Background(), `INSERT INTO rule_pools (name) VALUES ('测试池')`)
	if err != nil {
		t.Fatalf("插入素材池失败: %v", err)
	}
	poolID, _ := res.LastInsertId()
	srcRes, err := st.DB().ExecContext(context.Background(),
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
		rule := rulespec.CanonicalRule{Family: family, Matcher: matcher, Value: e.Value}
		crRes, err := st.DB().ExecContext(context.Background(),
			`INSERT INTO pool_canonical_rules (pool_id, semantic_key, family, matcher, value, options_json) VALUES (?,?,?,?,?,?)`,
			poolID, rule.SemanticKey(), string(rule.Family), string(rule.Matcher), rule.Value, `{}`)
		if err != nil {
			t.Fatalf("插入 canonical 失败: %v", err)
		}
		crID, _ := crRes.LastInsertId()
		if _, err := st.DB().ExecContext(context.Background(),
			`INSERT INTO pool_rule_origins (pool_id, canonical_rule_id, source_id, snapshot_id, sort_order, raw_line, line_no) VALUES (?,?,?,NULL,?,?,0)`,
			poolID, crID, sourceID, i, e.Type+","+e.Value); err != nil {
			t.Fatalf("插入 origin 失败: %v", err)
		}
	}
	return poolID
}

func TestPreviewClash(t *testing.T) {
	svc, st, _ := newTestService(t)
	pid := insertPlatform(t, st, "yaml")
	insertManualNode(t, st, "节点A", "vless", map[string]any{"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp", "tls": true, "servername": "sni.example.com"})
	insertGroup(t, st, "组A", "select", []string{"节点A"}, nil, true, false)
	poolID := insertPool(t, st,
		struct{ Type, Value string }{"DOMAIN-SUFFIX", "example.com"},
		struct{ Type, Value string }{"IP-CIDR", "1.2.3.0/24"},
	)
	res, err := svc.Preview(context.Background(), GenerateInput{
		TargetSyntax: ClashYAML, PlatformID: pid,
		FixedParams:          NewOrderedMap().Set("port", 7890).Set("mode", "rule"),
		NodeNames:            []string{"节点A"},
		GroupNames:           []string{"组A"},
		OverseasMembers:      []string{"节点A"},
		FallbackGroupMembers: []string{"🚀直接连接", "🌎国外流量"},
		Pools:                []PoolSelection{{PoolID: poolID, Target: "组A"}},
	})
	if err != nil {
		t.Fatalf("Clash 预览失败: %v", err)
	}
	content := string(res.Content)
	for _, want := range []string{"port: 7890", "name: 节点A", "proxy-groups:", "DOMAIN-SUFFIX,example.com,组A", "IP-CIDR,1.2.3.0/24,组A,no-resolve", "GEOIP,CN,DIRECT", "无法归属的流量"} {
		if !strings.Contains(content, want) {
			t.Errorf("Clash 内容缺少 %q\n%s", want, content)
		}
	}
	if strings.Contains(content, "# {{xray_nodes}}") {
		t.Error("未勾选 xray 节点时不应输出占位标记")
	}
}

func TestPreviewClashGroupNodeOrder(t *testing.T) {
	svc, st, _ := newTestService(t)
	pid := insertPlatform(t, st, "yaml")
	insertManualNode(t, st, "节点A", "vless", map[string]any{"uuid": "11111111-2222-3333-4444-555555555555"})
	insertManualNode(t, st, "节点B", "vless", map[string]any{"uuid": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"})
	insertGroup(t, st, "组A", "select", []string{"节点A", "节点B"}, nil, true, false)
	res, err := svc.Preview(context.Background(), GenerateInput{
		TargetSyntax: ClashYAML, PlatformID: pid,
		NodeNames:            []string{"节点A", "节点B"},
		GroupNames:           []string{"组A"},
		GroupNodeOrders:      map[string][]string{"组A": {"节点B", "节点A"}},
		OverseasMembers:      []string{"节点A", "节点B"},
		FallbackGroupMembers: []string{"🚀直接连接", "🌎国外流量"},
	})
	if err != nil {
		t.Fatalf("Clash 预览失败: %v", err)
	}
	content := string(res.Content)
	groupSeg := content[strings.Index(content, "name: 组A"):]
	idxB := strings.Index(groupSeg, "- 节点B")
	idxA := strings.Index(groupSeg, "- 节点A")
	if idxB < 0 || idxA < 0 || idxB > idxA {
		t.Errorf("group_node_orders 应让节点B排在节点A之前:\n%s", content)
	}
}

func TestPreviewClashForcedGroupMembers(t *testing.T) {
	svc, st, _ := newTestService(t)
	pid := insertPlatform(t, st, "yaml")
	insertManualNode(t, st, "节点A", "vless", map[string]any{"uuid": "11111111-2222-3333-4444-555555555555"})
	insertManualNode(t, st, "节点B", "vless", map[string]any{"uuid": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"})

	res, err := svc.Preview(context.Background(), GenerateInput{
		TargetSyntax:         ClashYAML,
		PlatformID:           pid,
		NodeNames:            []string{"节点A", "节点B"},
		OverseasMembers:      []string{"节点B", node.ForceDirect, "节点A"},
		FallbackGroupMembers: []string{node.ForceOverseas, "节点A", node.ForceDirect},
	})
	if err != nil {
		t.Fatalf("Clash 强制组预览失败: %v", err)
	}
	if got, want := clashGroupMembers(t, res.Content, node.ForceDirect), []string{node.ReservedDirect}; !equalStrings(got, want) {
		t.Errorf("直接连接成员异常: got=%v want=%v", got, want)
	}
	if got, want := clashGroupMembers(t, res.Content, node.ForceOverseas), []string{"节点B", node.ForceDirect, "节点A"}; !equalStrings(got, want) {
		t.Errorf("国外流量成员或顺序异常: got=%v want=%v", got, want)
	}
	if got, want := clashGroupMembers(t, res.Content, node.ForceFallback), []string{node.ForceOverseas, "节点A", node.ForceDirect}; !equalStrings(got, want) {
		t.Errorf("无法归属成员或顺序异常: got=%v want=%v", got, want)
	}
}

func TestForcedGroupMemberValidation(t *testing.T) {
	svc, st, _ := newTestService(t)
	pid := insertPlatform(t, st, "yaml")
	insertManualNode(t, st, "节点A", "vless", map[string]any{"uuid": "11111111-2222-3333-4444-555555555555"})
	insertGroup(t, st, "普通组", "select", []string{"节点A"}, nil, true, false)

	tests := []struct {
		name     string
		overseas []string
		fallback []string
		want     string
	}{
		{name: "国外流量拒绝底层 DIRECT", overseas: []string{node.ReservedDirect}, fallback: []string{node.ForceDirect}, want: "不能直接引用 DIRECT"},
		{name: "无法归属拒绝底层 DIRECT", overseas: []string{node.ForceDirect}, fallback: []string{node.ReservedDirect}, want: "不能直接引用 DIRECT"},
		{name: "国外流量允许直接连接组", overseas: []string{node.ForceDirect}, fallback: []string{node.ForceDirect}},
		{name: "无法归属允许国外流量组", overseas: []string{node.ForceDirect}, fallback: []string{node.ForceOverseas}},
		{name: "无法归属拒绝普通组", overseas: []string{node.ForceDirect}, fallback: []string{"普通组"}, want: "成员必须是已勾选节点"},
		{name: "无法归属拒绝自身", overseas: []string{node.ForceDirect}, fallback: []string{node.ForceFallback}, want: "不允许引用系统组"},
		{name: "无法归属不能为空", overseas: []string{node.ForceDirect}, fallback: nil, want: "未包含任何成员"},
		{name: "国外流量成员不能重复", overseas: []string{node.ForceDirect, node.ForceDirect}, fallback: []string{node.ForceDirect}, want: "成员重复"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Preview(context.Background(), GenerateInput{
				TargetSyntax:         ClashYAML,
				PlatformID:           pid,
				NodeNames:            []string{"节点A"},
				OverseasMembers:      tc.overseas,
				FallbackGroupMembers: tc.fallback,
			})
			if tc.want == "" {
				if err != nil {
					t.Fatalf("合法强制组成员被拒绝: %v", err)
				}
				return
			}
			if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("应拒绝并包含 %q，实际 %v", tc.want, err)
			}
		})
	}
}

func TestPreviewSubsAndGeneric(t *testing.T) {
	svc, st, _ := newTestService(t)
	sp := insertPlatform(t, st, "subs")
	gp := insertPlatform(t, st, "generic-subs")
	insertManualNode(t, st, "中文节点", "vless", map[string]any{"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp"})
	insertManualNode(t, st, "Snell节点", "snell", map[string]any{"psk": "secret"})
	// SR subs
	res, err := svc.Preview(context.Background(), GenerateInput{
		TargetSyntax: SrSubs, PlatformID: sp,
		FixedParams: NewOrderedMap().Set("status", "2026/01/01 Version").Set("remarks", "My VPN"),
		NodeNames:   []string{"中文节点", "Snell节点"},
	})
	if err != nil {
		t.Fatalf("SR subs 预览失败: %v", err)
	}
	content := string(res.Content)
	if !strings.HasPrefix(content, "STATUS=2026/01/01 Version\nREMARKS=My VPN\n") {
		t.Errorf("SR subs 头部异常:\n%s", content)
	}
	if !strings.Contains(content, "vless://") || !strings.Contains(content, "remarks=") {
		t.Errorf("SR subs 应包含 vless 链接:\n%s", content)
	}
	foundSkip := false
	for _, sk := range res.Skipped {
		if sk.Name == "Snell节点" {
			foundSkip = true
		}
	}
	if !foundSkip {
		t.Error("SR subs 应跳过 Snell 节点")
	}
	// generic subs
	res, err = svc.Preview(context.Background(), GenerateInput{
		TargetSyntax: GenericSubs, PlatformID: gp,
		NodeNames: []string{"中文节点"},
	})
	if err != nil {
		t.Fatalf("generic subs 预览失败: %v", err)
	}
	if strings.HasPrefix(string(res.Content), "STATUS=") {
		t.Error("generic subs 不应输出 STATUS 头")
	}
	if !strings.Contains(string(res.Content), "vless://") {
		t.Errorf("generic subs 应包含 vless 链接:\n%s", string(res.Content))
	}
}

func TestPreviewSrConf(t *testing.T) {
	svc, st, _ := newTestService(t)
	rid := insertRule(t, st)
	poolID := insertPool(t, st,
		struct{ Type, Value string }{"DOMAIN-SUFFIX", "example.com"},
		struct{ Type, Value string }{"USER-AGENT", "Telegram"},
	)
	res, err := svc.Preview(context.Background(), GenerateInput{
		TargetSyntax: SrConf, RuleID: rid,
		FixedParams:    NewOrderedMap().Set("loglevel", "warning"),
		Pools:          []PoolSelection{{PoolID: poolID, Target: "PROXY"}},
		FinalDirection: "DIRECT",
	})
	if err != nil {
		t.Fatalf("SR conf 预览失败: %v", err)
	}
	content := string(res.Content)
	for _, want := range []string{"[General]", "loglevel = warning", "[Rule]", "DOMAIN-SUFFIX,example.com,PROXY", "USER-AGENT,Telegram,PROXY", "GEOIP,CN,DIRECT", "FINAL,DIRECT"} {
		if !strings.Contains(content, want) {
			t.Errorf("SR conf 缺少 %q\n%s", want, content)
		}
	}
}

func TestClashSkipUserAgent(t *testing.T) {
	svc, st, _ := newTestService(t)
	pid := insertPlatform(t, st, "yaml")
	insertManualNode(t, st, "节点A", "vless", map[string]any{"uuid": "11111111-2222-3333-4444-555555555555"})
	insertGroup(t, st, "组A", "select", []string{"节点A"}, nil, true, false)
	poolID := insertPool(t, st, struct{ Type, Value string }{"USER-AGENT", "Telegram"})
	res, err := svc.Preview(context.Background(), GenerateInput{
		TargetSyntax: ClashYAML, PlatformID: pid,
		NodeNames: []string{"节点A"}, GroupNames: []string{"组A"}, OverseasMembers: []string{"节点A"}, FallbackGroupMembers: []string{"🚀直接连接", "🌎国外流量"},
		Pools: []PoolSelection{{PoolID: poolID, Target: "组A"}},
	})
	if err != nil {
		t.Fatalf("Clash 预览失败: %v", err)
	}
	if strings.Contains(string(res.Content), "USER-AGENT") {
		t.Error("Clash 不应输出 USER-AGENT")
	}
	if len(res.Skipped) == 0 || res.Skipped[0].Kind != "rule" {
		t.Errorf("应记录 USER-AGENT 跳过项: %+v", res.Skipped)
	}
}

func TestClashExtendedRulesAndSrSkip(t *testing.T) {
	svc, st, _ := newTestService(t)
	pid := insertPlatform(t, st, "yaml")
	rid := insertRule(t, st)
	insertManualNode(t, st, "节点A", "vless", map[string]any{"uuid": "11111111-2222-3333-4444-555555555555"})
	insertGroup(t, st, "组A", "select", []string{"节点A"}, nil, true, false)
	rules := []RuleLine{
		{RuleType: "GEOSITE", MatchValue: "cn", Target: "组A"},
		{RuleType: "IP-ASN", MatchValue: "45102", Target: "组A"},
		{RuleType: "AND", MatchValue: "((DOMAIN,a.com),(NETWORK,tcp))", Target: "组A"},
	}
	res, err := svc.Preview(context.Background(), GenerateInput{TargetSyntax: ClashYAML, PlatformID: pid, NodeNames: []string{"节点A"}, GroupNames: []string{"组A"}, GroupNodeOrders: map[string][]string{"组A": {"节点A"}}, OverseasMembers: []string{"节点A"}, FallbackGroupMembers: []string{"🚀直接连接", "🌎国外流量"}, CustomRules: rules})
	if err != nil {
		t.Fatalf("Clash 扩展规则失败: %v", err)
	}
	content := string(res.Content)
	for _, want := range []string{"GEOSITE,cn,组A", "IP-ASN,45102,组A,no-resolve", "AND,((DOMAIN,a.com),(NETWORK,tcp)),组A", "MATCH,🛟无法归属的流量"} {
		if !strings.Contains(content, want) {
			t.Errorf("Clash 缺少 %q:\n%s", want, content)
		}
	}
	srRules := append([]RuleLine(nil), rules...)
	for i := range srRules {
		srRules[i].Target = "PROXY"
	}
	res, err = svc.Preview(context.Background(), GenerateInput{TargetSyntax: SrConf, RuleID: rid, CustomRules: srRules, FinalDirection: "PROXY"})
	if err != nil {
		t.Fatalf("SR 扩展规则预览失败: %v", err)
	}
	if strings.Contains(string(res.Content), "GEOSITE") || strings.Contains(string(res.Content), "AND") || len(res.Skipped) != 2 {
		t.Fatalf("SR 应跳过 GEOSITE/AND 并保留 IP-ASN: content=%s skipped=%+v", res.Content, res.Skipped)
	}
	if !strings.Contains(string(res.Content), "IP-ASN,45102,PROXY") {
		t.Fatalf("SR 应输出 IP-ASN: content=%s", res.Content)
	}
}

func TestPreviewExtendedProxyGroupFields(t *testing.T) {
	svc, st, _ := newTestService(t)
	pid := insertPlatform(t, st, "yaml")
	insertManualNode(t, st, "节点A", "vless", map[string]any{"uuid": "11111111-2222-3333-4444-555555555555"})
	definition := `{"type":"load-balance","groups":[],"use":["provider-a"],"url":"https://www.gstatic.com/generate_204","expected-status":"204","interval":300,"timeout":5000,"max-failed-times":5,"lazy":true,"disable-udp":true,"include-all-providers":true,"icon":"https://example.com/icon.png"}`
	if _, err := st.DB().ExecContext(context.Background(), `INSERT INTO proxy_groups (name,type,preset_key,enabled,definition_json) VALUES ('Provider组','custom','',1,?)`, definition); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Preview(context.Background(), GenerateInput{
		TargetSyntax: ClashYAML, PlatformID: pid, NodeNames: []string{"节点A"}, GroupNames: []string{"Provider组"}, OverseasMembers: []string{"节点A"}, FallbackGroupMembers: []string{"🚀直接连接", "🌎国外流量"},
		FixedParams: NewOrderedMap().Set("proxy-providers", map[string]any{"provider-a": map[string]any{"type": "http", "url": "https://example.com/sub"}}),
	})
	if err != nil {
		t.Fatalf("扩展代理组预览失败: %v", err)
	}
	content := string(res.Content)
	for _, want := range []string{"name: Provider组", "type: load-balance", "use:", "provider-a", "interval: 300", "timeout: 5000", "include-all-providers: true"} {
		if !strings.Contains(content, want) {
			t.Errorf("扩展代理组缺少 %q:\n%s", want, content)
		}
	}
}

func TestXrayPlaceholderAndDisplayName(t *testing.T) {
	svc, st, _ := newTestService(t)
	pid := insertPlatform(t, st, "yaml")
	insertXrayNode(t, st, "instance-x-vless", "日本节点", "vless")
	insertGroup(t, st, "组A", "select", []string{"instance-x-vless"}, nil, true, false)
	res, err := svc.Preview(context.Background(), GenerateInput{
		TargetSyntax: ClashYAML, PlatformID: pid,
		NodeNames: []string{"instance-x-vless"}, GroupNames: []string{"组A"}, OverseasMembers: []string{"instance-x-vless"}, FallbackGroupMembers: []string{"🚀直接连接", "🌎国外流量"},
	})
	if err != nil {
		t.Fatalf("Clash 含 xray 预览失败: %v", err)
	}
	content := string(res.Content)
	if !strings.Contains(content, "# {{xray_nodes}}") {
		t.Errorf("含 xray 节点应输出占位标记:\n%s", content)
	}
	if !strings.Contains(content, "日本节点") {
		t.Errorf("xray display_name 应作为渲染名:\n%s", content)
	}
}

func TestValidationErrors(t *testing.T) {
	svc, st, _ := newTestService(t)
	pid := insertPlatform(t, st, "yaml")
	insertManualNode(t, st, "节点A", "vless", map[string]any{"uuid": "11111111-2222-3333-4444-555555555555"})
	// 空 🌎国外流量
	_, err := svc.Preview(context.Background(), GenerateInput{
		TargetSyntax: ClashYAML, PlatformID: pid,
		NodeNames: []string{"节点A"}, OverseasMembers: nil, FallbackGroupMembers: []string{"🚀直接连接", "🌎国外流量"},
	})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("空国外流量应 ErrBadRequest，实际 %v", err)
	}
	// 停用预设组
	insertGroup(t, st, "停用组", "select", []string{"节点A"}, nil, false, true)
	_, err = svc.Preview(context.Background(), GenerateInput{
		TargetSyntax: ClashYAML, PlatformID: pid,
		NodeNames: []string{"节点A"}, GroupNames: []string{"停用组"}, OverseasMembers: []string{"节点A"}, FallbackGroupMembers: []string{"🚀直接连接", "🌎国外流量"},
	})
	if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "预设组已停用") {
		t.Fatalf("停用预设组应拒绝，实际 %v", err)
	}
	// 未勾选子组
	insertGroup(t, st, "子组", "select", []string{"节点A"}, nil, true, false)
	insertGroup(t, st, "父组", "select", []string{"节点A"}, []string{"子组"}, true, false)
	_, err = svc.Preview(context.Background(), GenerateInput{
		TargetSyntax: ClashYAML, PlatformID: pid,
		NodeNames: []string{"节点A"}, GroupNames: []string{"父组"}, OverseasMembers: []string{"节点A"}, FallbackGroupMembers: []string{"🚀直接连接", "🌎国外流量"},
	})
	if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "未勾选的组") {
		t.Fatalf("未勾选子组应拒绝，实际 %v", err)
	}
	// 平台无订阅
	pid2 := insertPlatform(t, st, "generic-subs")
	_ = pid2
	// 单独插入无订阅平台
	res, err := st.DB().ExecContext(context.Background(),
		`INSERT INTO platforms (slug, name, product_type) VALUES ('platform-empty','空平台','subs')`)
	if err != nil {
		t.Fatalf("插入空平台失败: %v", err)
	}
	emptyID, _ := res.LastInsertId()
	_, err = svc.Preview(context.Background(), GenerateInput{TargetSyntax: SrSubs, PlatformID: emptyID, NodeNames: []string{"节点A"}})
	if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "创建订阅条目") {
		t.Fatalf("无订阅平台应拒绝，实际 %v", err)
	}
}

func TestLinksEncoding(t *testing.T) {
	svc, st, _ := newTestService(t)
	gp := insertPlatform(t, st, "generic-subs")
	// 中文/emoji 名称 + 非 ASCII 域名
	if _, err := st.DB().ExecContext(context.Background(),
		`INSERT INTO nodes (source, name, protocol, host, port, protocol_json) VALUES ('manual','😀节点','vless','例子.中国',443,'{"uuid":"11111111-2222-3333-4444-555555555555","network":"tcp"}')`); err != nil {
		t.Fatalf("插入节点失败: %v", err)
	}
	res, err := svc.Preview(context.Background(), GenerateInput{
		TargetSyntax: GenericSubs, PlatformID: gp,
		NodeNames: []string{"😀节点"},
	})
	if err != nil {
		t.Fatalf("链接编码预览失败: %v", err)
	}
	content := string(res.Content)
	if !strings.Contains(content, "xn--") {
		t.Errorf("非 ASCII 域名应转 punycode: %s", content)
	}
	if !strings.Contains(content, "%F0%9F%98%80") && !strings.Contains(content, "%E2%9C%") {
		// 只验证包含百分号编码即可
		if !strings.Contains(content, "%") {
			t.Errorf("节点名应 URL 编码: %s", content)
		}
	}
}

func TestOnlySnellSubsRejected(t *testing.T) {
	svc, st, _ := newTestService(t)
	pid := insertPlatform(t, st, "subs")
	insertManualNode(t, st, "Snell节点", "snell", map[string]any{"psk": "secret"})
	_, err := svc.Preview(context.Background(), GenerateInput{
		TargetSyntax: SrSubs, PlatformID: pid, NodeNames: []string{"Snell节点"},
	})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("仅不可转节点应拒绝，实际 %v", err)
	}
}

// TestClashHeaderOrder 保留头部表单键序
func TestClashHeaderOrder(t *testing.T) {
	svc, st, _ := newTestService(t)
	pid := insertPlatform(t, st, "yaml")
	insertManualNode(t, st, "节点A", "vless", map[string]any{"uuid": "11111111-2222-3333-4444-555555555555"})
	insertGroup(t, st, "组A", "select", []string{"节点A"}, nil, true, false)
	res, err := svc.Preview(context.Background(), GenerateInput{
		TargetSyntax: ClashYAML, PlatformID: pid,
		FixedParams:          NewOrderedMap().Set("mode", "rule").Set("port", 7890),
		NodeNames:            []string{"节点A"},
		GroupNames:           []string{"组A"},
		OverseasMembers:      []string{"节点A"},
		FallbackGroupMembers: []string{"🚀直接连接", "🌎国外流量"},
	})
	if err != nil {
		t.Fatalf("Clash 预览失败: %v", err)
	}
	content := string(res.Content)
	modeIdx := strings.Index(content, "mode:")
	portIdx := strings.Index(content, "port:")
	if modeIdx < 0 || portIdx < 0 || modeIdx > portIdx {
		t.Fatalf("头部键序未保留：mode=%d port=%d\n%s", modeIdx, portIdx, content)
	}
}

// TestOverseasMemberMustBeSelected 国外流量成员必须是已勾选节点
func TestOverseasMemberMustBeSelected(t *testing.T) {
	svc, st, _ := newTestService(t)
	pid := insertPlatform(t, st, "yaml")
	insertManualNode(t, st, "节点A", "vless", map[string]any{"uuid": "11111111-2222-3333-4444-555555555555"})
	_, err := svc.Preview(context.Background(), GenerateInput{
		TargetSyntax: ClashYAML, PlatformID: pid,
		NodeNames:            []string{"节点A"},
		OverseasMembers:      []string{"未勾选节点"},
		FallbackGroupMembers: []string{"🚀直接连接", "🌎国外流量"},
	})
	if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "必须是已勾选节点") {
		t.Fatalf("未勾选国外流量成员应拒绝，实际 %v", err)
	}
}

// TestNonexistentPoolRejected 不存在的素材池应拒绝
func TestNonexistentPoolRejected(t *testing.T) {
	svc, st, _ := newTestService(t)
	pid := insertPlatform(t, st, "yaml")
	insertManualNode(t, st, "节点A", "vless", map[string]any{"uuid": "11111111-2222-3333-4444-555555555555"})
	insertGroup(t, st, "组A", "select", []string{"节点A"}, nil, true, false)
	_, err := svc.Preview(context.Background(), GenerateInput{
		TargetSyntax: ClashYAML, PlatformID: pid,
		NodeNames:            []string{"节点A"},
		GroupNames:           []string{"组A"},
		OverseasMembers:      []string{"节点A"},
		FallbackGroupMembers: []string{"🚀直接连接", "🌎国外流量"},
		Pools:                []PoolSelection{{PoolID: 99999, Target: "组A"}},
	})
	if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "素材池不存在") {
		t.Fatalf("不存在池应拒绝，实际 %v", err)
	}
}

// TestWarningsSkipRulesForNodeSubscriptions 节点订阅不输出无关的“空规则”警告。
func TestWarningsSkipRulesForNodeSubscriptions(t *testing.T) {
	svc, st, _ := newTestService(t)
	sp := insertPlatform(t, st, "subs")
	gp := insertPlatform(t, st, "generic-subs")
	insertManualNode(t, st, "节点A", "vless", map[string]any{"uuid": "11111111-2222-3333-4444-555555555555"})
	for _, tc := range []struct {
		target TargetSyntax
		pid    int64
	}{
		{SrSubs, sp},
		{GenericSubs, gp},
	} {
		res, err := svc.Preview(context.Background(), GenerateInput{
			TargetSyntax: tc.target,
			PlatformID:   tc.pid,
			NodeNames:    []string{"节点A"},
		})
		if err != nil {
			t.Fatalf("节点订阅预览失败: %v", err)
		}
		for _, w := range res.Warnings {
			if strings.Contains(w, "未选择任何规则素材池或手动规则") {
				t.Errorf("%s 不应输出规则空警告: %v", tc.target, res.Warnings)
			}
		}
	}
}

// TestWarningsKeepRulesForClashAndSrConf 规则类产物仍保留空规则警告。
func TestWarningsKeepRulesForClashAndSrConf(t *testing.T) {
	svc, st, _ := newTestService(t)
	pid := insertPlatform(t, st, "yaml")
	rid := insertRule(t, st)
	insertManualNode(t, st, "节点A", "vless", map[string]any{"uuid": "11111111-2222-3333-4444-555555555555"})
	insertGroup(t, st, "组A", "select", []string{"节点A"}, nil, true, false)
	clash, err := svc.Preview(context.Background(), GenerateInput{
		TargetSyntax: ClashYAML, PlatformID: pid,
		NodeNames: []string{"节点A"}, GroupNames: []string{"组A"}, OverseasMembers: []string{"节点A"}, FallbackGroupMembers: []string{"🚀直接连接", "🌎国外流量"},
	})
	if err != nil {
		t.Fatalf("Clash 预览失败: %v", err)
	}
	if !containsWarning(clash.Warnings, "未选择任何规则素材池或手动规则") {
		t.Errorf("Clash 无规则素材时应输出空规则警告: %v", clash.Warnings)
	}
	sr, err := svc.Preview(context.Background(), GenerateInput{
		TargetSyntax: SrConf, RuleID: rid,
		CustomRules: []RuleLine{},
	})
	if err != nil {
		t.Fatalf("SR 分流预览失败: %v", err)
	}
	if !containsWarning(sr.Warnings, "未选择任何规则素材池或手动规则") {
		t.Errorf("SR 分流无规则素材时应输出空规则警告: %v", sr.Warnings)
	}
}

func containsWarning(warnings []string, text string) bool {
	for _, w := range warnings {
		if strings.Contains(w, text) {
			return true
		}
	}
	return false
}

func clashGroupMembers(t *testing.T, content []byte, groupName string) []string {
	t.Helper()
	var decoded any
	if err := gyaml.UnmarshalWithOptions(content, &decoded, gyaml.UseOrderedMap()); err != nil {
		t.Fatalf("解析 Clash YAML 失败: %v", err)
	}
	root, ok := yamlMap(decoded)
	if !ok {
		t.Fatal("Clash YAML 顶层不是映射")
	}
	raw, ok := mapGet(root, "proxy-groups")
	if !ok {
		t.Fatal("Clash YAML 缺少 proxy-groups")
	}
	groups, ok := seqOf(raw)
	if !ok {
		t.Fatal("proxy-groups 不是列表")
	}
	for _, rawGroup := range groups {
		group, ok := yamlMap(rawGroup)
		if !ok || mapString(group, "name") != groupName {
			continue
		}
		members, ok := seqOfValue(group, "proxies")
		if !ok {
			t.Fatalf("组 %s 的 proxies 不是列表", groupName)
		}
		out := make([]string, 0, len(members))
		for _, rawMember := range members {
			member, ok := scalarString(rawMember)
			if !ok {
				t.Fatalf("组 %s 存在非字符串成员", groupName)
			}
			out = append(out, member)
		}
		return out
	}
	t.Fatalf("Clash YAML 缺少代理组 %s", groupName)
	return nil
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
