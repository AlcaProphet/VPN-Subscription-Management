package assembly

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	gyaml "github.com/goccy/go-yaml"
)

func TestOverlaySeqSemantics(t *testing.T) {
	root := gyaml.MapSlice{
		{Key: "proxies", Value: []any{
			gyaml.MapSlice{{Key: "name", Value: "p1"}, {Key: "type", Value: "ss"}},
			gyaml.MapSlice{{Key: "name", Value: "p2"}, {Key: "type", Value: "ss"}},
		}},
		{Key: "proxy-groups", Value: []any{
			gyaml.MapSlice{{Key: "name", Value: "sel"}, {Key: "type", Value: "select"}, {Key: "proxies", Value: []any{"p2"}}},
			gyaml.MapSlice{{Key: "name", Value: "other"}, {Key: "type", Value: "url-test"}, {Key: "proxies", Value: []any{"p1"}}},
		}},
		{Key: "rules", Value: []any{"DOMAIN,old,DIRECT"}},
	}
	ov := OverlayInput{
		RulesYAML:   "prepend:\n  - DOMAIN,new,sel\nappend:\n  - MATCH,sel\ndelete:\n  - DOMAIN,old,DIRECT\n",
		ProxiesYAML: "prepend:\n  - name: p3\n    type: ss\n    server: x\n    port: 1\nappend:\n  - p4\ndelete:\n  - p1\n",
		GroupsYAML:  "prepend:\n  - name: gNew\n    type: select\n    proxies:\n      - p3\n",
	}
	if err := applyClashOverlay(&root, ov); err != nil {
		t.Fatalf("applyClashOverlay 失败: %v", err)
	}
	text := yamlText(t, root)
	for _, want := range []string{"DOMAIN,new,sel", "MATCH,sel", "name: p3", "name: gNew", "name: sel"} {
		if !strings.Contains(text, want) {
			t.Errorf("覆盖层结果缺少 %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "DOMAIN,old,DIRECT") || strings.Contains(text, "p1") {
		t.Errorf("delete 未生效:\n%s", text)
	}
	// p3 应插入第一个 selector 组最前且 p2 保留
	selStart := strings.Index(text, "name: sel")
	selSeg := text[selStart:]
	if !strings.Contains(selSeg, "p3") {
		t.Errorf("新增节点未插入 selector 组:\n%s", text)
	}
}

func TestOverlayMergeAndControlPlane(t *testing.T) {
	root := gyaml.MapSlice{
		{Key: "mode", Value: "rule"},
		{Key: "port", Value: 7890},
		{Key: "dns", Value: gyaml.MapSlice{{Key: "ipv6", Value: true}}},
	}
	ov := OverlayInput{
		MergeYAML: "Mode: direct\nport: 9999\nmixed-port: 1234\ndns:\n  ipv6: false\n  enable: true\ncustom: hi\n",
	}
	if err := applyClashOverlay(&root, ov); err != nil {
		t.Fatalf("applyClashOverlay 失败: %v", err)
	}
	text := yamlText(t, root)
	for _, want := range []string{"mode: rule", "port: 7890", "dns:", "ipv6: true", "enable: true", "custom: hi"} {
		if !strings.Contains(text, want) {
			t.Errorf("Merge/控制面结果缺少 %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "mode: direct") || strings.Contains(text, "port: 9999") || strings.Contains(text, "mixed-port") || strings.Contains(text, "ipv6: false") {
		t.Errorf("控制面或 dns.ipv6 被覆盖:\n%s", text)
	}
}

func TestOverlayCleanupAndSort(t *testing.T) {
	root := gyaml.MapSlice{
		{Key: "mode", Value: "rule"},
		{Key: "port", Value: 7890},
		{Key: "proxy-providers", Value: gyaml.MapSlice{{Key: "prov1", Value: gyaml.MapSlice{{Key: "type", Value: "http"}}}}},
		{Key: "proxies", Value: []any{gyaml.MapSlice{{Key: "name", Value: "p1"}}}},
		{Key: "proxy-groups", Value: []any{
			gyaml.MapSlice{
				{Key: "name", Value: "g1"}, {Key: "type", Value: "select"},
				{Key: "use", Value: []any{"prov1", "missing"}},
				{Key: "proxies", Value: []any{"p1", "missing", "provider-supplied"}},
			},
			gyaml.MapSlice{
				{Key: "name", Value: "g2"}, {Key: "type", Value: "select"},
				{Key: "proxies", Value: []any{"missing2"}},
			},
		}},
		{Key: "rules", Value: []any{"MATCH,DIRECT"}},
	}
	if err := applyClashOverlay(&root, OverlayInput{}); err != nil {
		t.Fatalf("applyClashOverlay 失败: %v", err)
	}
	text := yamlText(t, root)
	// 有效 provider 存在时保留未知字符串成员
	if !strings.Contains(text, "provider-supplied") {
		t.Errorf("has_valid_provider 语义未保留未知成员:\n%s", text)
	}
	if strings.Contains(text, "name: g2") && strings.Contains(text, "missing2") {
		t.Errorf("无 provider 的悬空引用未清理:\n%s", text)
	}
	// 顶层排序：mode/port 在 proxies 之前，proxies/proxy-groups/rules 收尾
	modeIdx := strings.Index(text, "mode:")
	portIdx := strings.Index(text, "port:")
	proxiesIdx := strings.Index(text, "proxies:")
	groupsIdx := strings.Index(text, "proxy-groups:")
	rulesIdx := strings.Index(text, "rules:")
	if modeIdx < 0 || portIdx < 0 || proxiesIdx < 0 || groupsIdx < 0 || rulesIdx < 0 || !(modeIdx < portIdx && portIdx < proxiesIdx && proxiesIdx < groupsIdx && groupsIdx < rulesIdx) {
		t.Errorf("顶层排序不符合 CVR use_sort:\n%s", text)
	}
}

func TestRenderClashPlanOverlay(t *testing.T) {
	plan := ClashPlan{
		Head: NewOrderedMap().Set("mode", "rule"),
		ManualProxies: []*OrderedMap{
			NewOrderedMap().Set("name", "manual-a").Set("type", "vless").Set("server", "a.example.com").Set("port", 443).Set("uuid", "u"),
		},
		ProxyGroups: []ClashPlanGroup{
			{Name: "🚀直接连接", Type: "select", Proxies: []string{"DIRECT"}, Force: true},
			{Name: "🌎国外流量", Type: "select", Proxies: []string{"manual-a"}, Force: true},
			{Name: "🛟无法归属的流量", Type: "select", Proxies: []string{"🚀直接连接", "🌎国外流量"}, Force: true},
		},
		Rules:    []ClashPlanRule{{Type: "DOMAIN-SUFFIX", Value: "example.com", Target: "🌎国外流量"}},
		Fallback: []string{"GEOIP,CN,DIRECT", "MATCH,🛟无法归属的流量"},
		Overlay: OverlayInput{
			ProxiesYAML: "prepend:\n  - name: overlay-node\n    type: ss\n    server: o.example.com\n    port: 8388\n",
			RulesYAML:   "prepend:\n  - DOMAIN,overlay.test,🌎国外流量\n",
			MergeYAML:   "custom-field: hello\n",
		},
	}
	raw, err := jsonMarshalPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	content, err := RenderClashPlan(raw, nil, map[string]string{"manual-a": "手动A"}, "")
	if err != nil {
		t.Fatalf("RenderClashPlan 覆盖层失败: %v", err)
	}
	text := string(content)
	for _, want := range []string{"name: overlay-node", "DOMAIN,overlay.test,🌎国外流量", "custom-field: hello", "name: 手动A"} {
		if !strings.Contains(text, want) {
			t.Errorf("下载重渲染覆盖层缺少 %q:\n%s", want, text)
		}
	}
}

func TestBlueprintOverlayRoundtrip(t *testing.T) {
	svc, st, _ := newTestService(t)
	ctx := context.Background()
	res, err := st.DB().ExecContext(ctx,
		`INSERT INTO versions (owner_type, owner_id, version_no, file_path, file_name) VALUES ('subscription', 1, 1, 'subscription/1/v1', 'subscription.yaml')`)
	if err != nil {
		t.Fatalf("插入测试版本失败: %v", err)
	}
	versionID, _ := res.LastInsertId()
	ov := OverlayInput{
		MergeYAML:   "custom: 1\n",
		RulesYAML:   "prepend: [DOMAIN,x,组]\n",
		ProxiesYAML: "prepend:\n  - name: p\n    type: ss\n",
		GroupsYAML:  "append:\n  - name: g\n    type: select\n",
	}
	err = st.TxImmediate(ctx, func(tx *sql.Tx) error {
		return svc.SaveBlueprintTx(ctx, tx, versionID, GenerateInput{
			TargetSyntax: ClashYAML, PlatformID: 1, Overlay: ov,
		}, json.RawMessage(`{}`))
	})
	if err != nil {
		t.Fatalf("保存蓝图失败: %v", err)
	}
	data, err := svc.GetBlueprint(ctx, versionID)
	if err != nil {
		t.Fatalf("读取蓝图失败: %v", err)
	}
	if data.Overlay.MergeYAML != ov.MergeYAML || data.Overlay.RulesYAML != ov.RulesYAML ||
		data.Overlay.ProxiesYAML != ov.ProxiesYAML || data.Overlay.GroupsYAML != ov.GroupsYAML {
		t.Fatalf("蓝图 overlay 回读不一致: %+v", data.Overlay)
	}
	var sel map[string]any
	if err := json.Unmarshal(data.Selection, &sel); err != nil {
		t.Fatalf("解析 selection 失败: %v", err)
	}
	if _, ok := sel["overlay"]; !ok {
		t.Fatalf("selection_json 缺少 overlay 键: %s", data.Selection)
	}
}

func yamlText(t *testing.T, root gyaml.MapSlice) string {
	t.Helper()
	out, err := marshalClashYAML(root, nil)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	return string(out)
}

func jsonMarshalPlan(v any) ([]byte, error) {
	return json.Marshal(v)
}
