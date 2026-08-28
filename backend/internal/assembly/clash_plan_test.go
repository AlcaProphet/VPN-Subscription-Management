package assembly

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderClashPlanFullReRender(t *testing.T) {
	plan := ClashPlan{
		Head: NewOrderedMap().Set("mode", "rule").Set("port", 7890),
		ManualProxies: []*OrderedMap{
			NewOrderedMap().Set("name", "manual-a").Set("type", "vless").Set("server", "a.example.com").Set("port", 443).Set("uuid", "u"),
		},
		ProxyGroups: []ClashPlanGroup{
			{Name: "🚀直接连接", Type: "select", Proxies: []string{"DIRECT"}, Force: true},
			{Name: "🌎国外流量", Type: "select", Proxies: []string{"manual-a"}, Force: true},
			{Name: "🛟无法归属的流量", Type: "select", Proxies: []string{"🚀直接连接", "🌎国外流量"}, Force: true},
			{Name: "组A", Type: "select", Proxies: []string{"manual-a"}, Force: false},
		},
		Rules:    []ClashPlanRule{{Type: "DOMAIN-SUFFIX", Value: "example.com", Target: "组A"}},
		Fallback: []string{"GEOIP,CN,DIRECT", "MATCH,🛟无法归属的流量"},
	}
	raw, _ := json.Marshal(plan)
	content, err := RenderClashPlan(raw, nil, map[string]string{"manual-a": "手动A"}, "")
	if err != nil {
		t.Fatalf("RenderClashPlan 失败: %v", err)
	}
	text := string(content)
	for _, want := range []string{"mode: rule", "port: 7890", "name: 手动A", "组A", "DOMAIN-SUFFIX,example.com,组A", "MATCH,", "无法归属的流量"} {
		if !strings.Contains(text, want) {
			t.Errorf("重渲染缺少 %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "# {{xray_nodes}}") {
		t.Errorf("重渲染不应残留占位符:\n%s", text)
	}
}

func TestRenderClashPlanRemovesUnreachableGroupAndDowngradesRule(t *testing.T) {
	plan := ClashPlan{
		Head: NewOrderedMap(),
		ManualProxies: []*OrderedMap{
			NewOrderedMap().Set("name", "manual-a").Set("type", "vless").Set("server", "a.example.com").Set("port", 443).Set("uuid", "u"),
		},
		ProxyGroups: []ClashPlanGroup{
			{Name: "🚀直接连接", Type: "select", Proxies: []string{"DIRECT"}, Force: true},
			{Name: "🌎国外流量", Type: "select", Proxies: []string{"manual-a"}, Force: true},
			{Name: "🛟无法归属的流量", Type: "select", Proxies: []string{"🚀直接连接", "🌎国外流量"}, Force: true},
			{Name: "动态组", Type: "select", Proxies: []string{"dynamic-x"}, Force: false},
		},
		Rules:    []ClashPlanRule{{Type: "DOMAIN-SUFFIX", Value: "example.com", Target: "动态组"}},
		Fallback: []string{"GEOIP,CN,DIRECT", "MATCH,🛟无法归属的流量"},
	}
	raw, _ := json.Marshal(plan)
	// 动态节点为空：动态组不可达应被删除，规则目标降级 DIRECT
	content, err := RenderClashPlan(raw, nil, map[string]string{"manual-a": "手动A"}, "")
	if err != nil {
		t.Fatalf("RenderClashPlan 失败: %v", err)
	}
	text := string(content)
	if strings.Contains(text, "动态组") {
		t.Errorf("不可达普通组应被删除:\n%s", text)
	}
	if !strings.Contains(text, "DOMAIN-SUFFIX,example.com,DIRECT") {
		t.Errorf("被删除组规则应降级 DIRECT:\n%s", text)
	}
	if !strings.Contains(text, "国外流量") || !strings.Contains(text, "无法归属的流量") {
		t.Errorf("强制组应保留:\n%s", text)
	}
}

func TestRenderClashPlanComment(t *testing.T) {
	plan := ClashPlan{
		Head:          NewOrderedMap(),
		ManualProxies: []*OrderedMap{},
		ProxyGroups: []ClashPlanGroup{
			{Name: "🚀直接连接", Type: "select", Proxies: []string{"DIRECT"}, Force: true},
			{Name: "🌎国外流量", Type: "select", Proxies: []string{}, Force: true},
			{Name: "🛟无法归属的流量", Type: "select", Proxies: []string{"🚀直接连接", "🌎国外流量"}, Force: true},
		},
		Rules:    []ClashPlanRule{},
		Fallback: []string{"GEOIP,CN,DIRECT", "MATCH,🛟无法归属的流量"},
	}
	raw, _ := json.Marshal(plan)
	content, err := RenderClashPlan(raw, nil, nil, "# 节点未开通，请联系管理员")
	if err != nil {
		t.Fatalf("RenderClashPlan 失败: %v", err)
	}
	if !strings.Contains(string(content), "# 节点未开通，请联系管理员") {
		t.Errorf("应输出无凭据注释:\n%s", content)
	}
	if strings.Contains(string(content), "# {{xray_nodes}}") {
		t.Errorf("不应残留占位符:\n%s", content)
	}
	if got, want := clashGroupMembers(t, content, "🚀直接连接"), []string{"DIRECT"}; !equalStrings(got, want) {
		t.Errorf("直接连接空节点兜底异常: got=%v want=%v", got, want)
	}
	if got, want := clashGroupMembers(t, content, "🌎国外流量"), []string{"🚀直接连接"}; !equalStrings(got, want) {
		t.Errorf("国外流量空节点兜底异常: got=%v want=%v", got, want)
	}
	if got, want := clashGroupMembers(t, content, "🛟无法归属的流量"), []string{"🚀直接连接", "🌎国外流量"}; !equalStrings(got, want) {
		t.Errorf("无法归属组仍有可达组时不应替换成员: got=%v want=%v", got, want)
	}
}

func TestRenderClashPlanKeepsProviderGroupAndFields(t *testing.T) {
	plan := ClashPlan{
		Head: NewOrderedMap().Set("proxy-providers", map[string]any{"provider-a": map[string]any{"type": "http", "url": "https://example.com/sub"}}),
		ProxyGroups: []ClashPlanGroup{{
			Name: "Provider组", Type: "load-balance", Use: []string{"provider-a"},
			URL: "https://www.gstatic.com/generate_204", ExpectedStatus: "204", Interval: 300,
			Timeout: 5000, MaxFailedTimes: 5, Lazy: true, DisableUDP: true,
			Filter: "HK|JP", ExcludeType: "Direct|Reject", IncludeAllProviders: true, Hidden: true, Icon: "https://example.com/icon.png",
		}},
		Rules: []ClashPlanRule{{Type: "RULE-SET", Value: "provider-a", Target: "Provider组"}},
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	content, err := RenderClashPlan(raw, nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, want := range []string{"name: Provider组", "type: load-balance", "use:", "provider-a", "expected-status: \"204\"", "interval: 300", "timeout: 5000", "max-failed-times: 5", "disable-udp: true", "include-all-providers: true", "RULE-SET,provider-a,Provider组,no-resolve"} {
		if !strings.Contains(text, want) {
			t.Errorf("Provider 组输出缺少 %q:\n%s", want, text)
		}
	}
}
