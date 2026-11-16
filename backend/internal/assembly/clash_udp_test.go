package assembly

import (
	"bytes"
	"context"
	"testing"

	gyaml "github.com/goccy/go-yaml"

	"vpn-sub/internal/node"
)

func TestClashManualVLESSAndVMessUDPDefault(t *testing.T) {
	ctx := context.Background()
	svc, st, _ := newTestService(t)
	platformID := insertPlatform(t, st, "yaml")
	cases := []struct {
		name    string
		params  map[string]any
		wantUDP bool
	}{
		{name: "default", wantUDP: true},
		{name: "enabled", params: map[string]any{"udp": true}, wantUDP: true},
		{name: "disabled", params: map[string]any{"udp": false}},
	}
	for _, protocol := range []string{"vless", "vmess"} {
		for _, tc := range cases {
			name := protocol + "-" + tc.name
			t.Run(name, func(t *testing.T) {
				params := map[string]any{"uuid": "11111111-2222-3333-4444-555555555555"}
				for key, value := range tc.params {
					params[key] = value
				}
				insertManualNode(t, st, name, protocol, params)
				check, err := svc.CheckNodeTargetDraft(ctx, node.CheckTargetDraft{
					Target: "clash-yaml", Protocol: protocol, RenderName: name,
					Host: "example.com", Port: 443, Params: params,
				})
				if err != nil {
					t.Fatal(err)
				}
				assertClashProxyUDP(t, []byte(check.Preview), name, tc.wantUDP)

				in := GenerateInput{
					TargetSyntax: ClashYAML, PlatformID: platformID, NodeNames: []string{name},
					OverseasMembers: []string{name}, FallbackGroupMembers: []string{name},
					CustomRules: []RuleLine{{RuleType: "DOMAIN-SUFFIX", MatchValue: "example.net", Target: node.ForceDirect}},
				}
				preview, err := svc.Preview(ctx, in)
				if err != nil {
					t.Fatal(err)
				}
				rendered, err := svc.Render(ctx, in)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(preview.Content, rendered.Content) {
					t.Fatal("装配预览与生成正文不一致")
				}
				assertClashProxyUDP(t, rendered.Content, name, tc.wantUDP)
				download, err := RenderClashPlan(rendered.RenderPlan, nil, nil, "")
				if err != nil {
					t.Fatal(err)
				}
				assertClashProxyUDP(t, download, name, tc.wantUDP)
			})
		}
	}
}

func TestClashUDPDefaultDoesNotApplyToHistoricalPlanOrXray(t *testing.T) {
	const uuid = "11111111-2222-3333-4444-555555555555"
	// 历史计划冻结的是生成时的完整节点条目。
	plan := []byte(`{"manual_proxies":[{"name":"historical","type":"vless","server":"example.com","port":443,"uuid":"` + uuid + `"}]}`)
	dynamic := []DynamicNode{
		{Name: "dynamic-vless", RenderName: "dynamic-vless", Protocol: "vless", Host: "example.com", Port: 443,
			ProtocolJSON: map[string]any{"uuid": uuid}},
		{Name: "dynamic-vmess", RenderName: "dynamic-vmess", Protocol: "vmess", Host: "example.com", Port: 443,
			ProtocolJSON: map[string]any{"uuid": uuid}},
	}
	content, err := RenderClashPlan(plan, dynamic, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"historical", "dynamic-vless", "dynamic-vmess"} {
		assertClashProxyUDP(t, content, name, false)
	}
}

func assertClashProxyUDP(t *testing.T, content []byte, name string, wantUDP bool) {
	t.Helper()
	var config struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := gyaml.Unmarshal(content, &config); err != nil {
		t.Fatalf("解析 Clash YAML 失败: %v", err)
	}
	for _, proxy := range config.Proxies {
		if proxy["name"] != name {
			continue
		}
		udp, exists := proxy["udp"]
		if exists != wantUDP || exists && udp != true {
			t.Errorf("节点 %s 的 udp 产物值为 %v（存在=%t），预期输出 udp:true=%t", name, udp, exists, wantUDP)
		}
		return
	}
	t.Fatalf("Clash YAML 缺少节点 %s", name)
}
