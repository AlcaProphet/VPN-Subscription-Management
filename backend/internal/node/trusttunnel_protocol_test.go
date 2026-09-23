// trusttunnel_protocol_test.go：Build32 Step 17 TrustTunnel 协议合同测试。
// 覆盖用户名／密码成对（允许两者同时为空）、TLS／ECH／mTLS 字段与清空、
// reuse_mode selector 的互斥分支与正整数约束、A→B→A 不恢复，以及 quic 关闭清空 QUIC 调优字段。
package node

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func trusttunnelProto(t *testing.T) Protocol {
	t.Helper()
	proto, err := GetProtocol("trusttunnel")
	if err != nil {
		t.Fatal(err)
	}
	return proto
}

func trusttunnelFieldMust(t *testing.T, name string) FieldSchema {
	t.Helper()
	field, ok := findSchemaField(trusttunnelProto(t).FormSchema, name)
	if !ok {
		t.Fatalf("TrustTunnel schema 缺少字段 %s", name)
	}
	return field
}

func createTrustTunnel(t *testing.T, svc *Service, name string, params map[string]any, state *CurrentState) (*Node, error) {
	t.Helper()
	return svc.CreateManual(context.Background(), CreateManualInput{
		Name: name, Protocol: "trusttunnel", Host: "example.com", Port: 443,
		ProtocolJSON: params, CurrentState: state,
	})
}

// reuseSelectors 构造 TrustTunnel 的 selector 提交值。
func reuseSelectors(mode string) *CurrentState {
	return &CurrentState{Selectors: map[string]string{"reuse_mode": mode}}
}

// TestTrustTunnelCredentialPair 覆盖 username／password 必须同时为空或同时非空，且 password 为 secret。
func TestTrustTunnelCredentialPair(t *testing.T) {
	proto := trusttunnelProto(t)
	for _, name := range []string{"username", "password"} {
		field := trusttunnelFieldMust(t, name)
		if field.Required {
			t.Fatalf("TrustTunnel %s 允许与另一字段同时为空，不得单独必填: %+v", name, field)
		}
	}
	if !contains(proto.SensitiveFields, "password") {
		t.Fatalf("TrustTunnel password 必须是敏感字段: %v", proto.SensitiveFields)
	}

	svc, _, _ := newTestService(t)
	// 两者同时为空是合法的。
	if _, err := createTrustTunnel(t, svc, "tt-no-auth", map[string]any{}, nil); err != nil {
		t.Fatalf("用户名与密码同时为空必须合法: %v", err)
	}
	// 两者同时非空也是合法的。
	if _, err := createTrustTunnel(t, svc, "tt-auth", map[string]any{"username": "u", "password": "p"}, nil); err != nil {
		t.Fatalf("用户名与密码同时提供必须合法: %v", err)
	}
	for _, tc := range []struct {
		name   string
		params map[string]any
		want   string
	}{
		{name: "only-username", params: map[string]any{"username": "u"}, want: "password"},
		{name: "only-password", params: map[string]any{"password": "p"}, want: "username"},
		{name: "blank-username", params: map[string]any{"username": "   ", "password": "p"}, want: "username"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := createTrustTunnel(t, svc, "tt-"+tc.name, tc.params, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%s 必须按字段 %s 拒绝，实际: %v", tc.name, tc.want, err)
			}
		})
	}
}

// TestTrustTunnelTLSAndECH 覆盖 TLS 字段集合、证书／私钥成对与 ECH 关闭清空子字段。
func TestTrustTunnelTLSAndECH(t *testing.T) {
	for _, name := range []string{"alpn", "sni", "ech-opts", "client-fingerprint", "skip-cert-verify",
		"name-cert-verify", "fingerprint", "certificate", "private-key"} {
		trusttunnelFieldMust(t, name)
	}
	if got := trusttunnelFieldMust(t, "alpn").Type; got != "text-list" {
		t.Fatalf("alpn 必须是 text-list，实际 %s", got)
	}
	for _, name := range []string{"certificate", "private-key"} {
		field := trusttunnelFieldMust(t, name)
		if field.Type != "multiline" && field.Type != "secret-multiline" {
			t.Fatalf("%s 必须是多行类型: %+v", name, field)
		}
	}
	if !contains(trusttunnelProto(t).SensitiveFields, "private-key") {
		t.Fatalf("private-key 必须是敏感字段: %v", trusttunnelProto(t).SensitiveFields)
	}

	ech, ok := findSchemaField(trusttunnelProto(t).FormSchema, "ech-opts")
	if !ok || ech.Feature == nil || ech.Feature.Name != "ech" {
		t.Fatalf("ech-opts 必须声明 ech 功能开关: %+v", ech)
	}
	for _, child := range ech.Properties {
		if child.Name == "enable" {
			continue
		}
		if child.When == nil || len(child.When.Features) == 0 {
			t.Fatalf("ech-opts.%s 必须在启用时才活动: %+v", child.Name, child)
		}
		if !child.ShouldReset("feature.ech") {
			t.Fatalf("ech-opts.%s 必须随功能关闭清空: %+v", child.Name, child)
		}
	}

	svc, _, _ := newTestService(t)
	// 证书／私钥成对。
	if _, err := createTrustTunnel(t, svc, "tt-cert-only",
		map[string]any{"certificate": "-----BEGIN CERTIFICATE-----"}, nil); err == nil {
		t.Fatal("只提供证书必须被拒绝")
	}
	if _, err := createTrustTunnel(t, svc, "tt-key-only",
		map[string]any{"private-key": "-----BEGIN PRIVATE KEY-----"}, nil); err == nil {
		t.Fatal("只提供私钥必须被拒绝")
	}
	if _, err := createTrustTunnel(t, svc, "tt-cert-pair",
		map[string]any{"certificate": "-----BEGIN CERTIFICATE-----", "private-key": "-----BEGIN PRIVATE KEY-----"}, nil); err != nil {
		t.Fatalf("证书与私钥成对必须合法: %v", err)
	}

	// ECH 关闭清空子字段。
	created, err := createTrustTunnel(t, svc, "tt-ech-off", map[string]any{
		"ech-opts": map[string]any{"enable": false, "config": "ech-config", "query-server-name": "ech.example.com"},
	}, nil)
	if err != nil {
		t.Fatalf("ECH 关闭的节点应可创建: %v", err)
	}
	if opts, ok := created.ProtocolJSON["ech-opts"].(map[string]any); ok {
		if _, exists := opts["config"]; exists {
			t.Fatalf("ECH 关闭必须清空 config: %+v", opts)
		}
		if _, exists := opts["query-server-name"]; exists {
			t.Fatalf("ECH 关闭必须清空 query-server-name: %+v", opts)
		}
	}
}

// TestTrustTunnelReuseModeSelector 覆盖 reuse_mode selector 声明、分支必填与正整数约束。
func TestTrustTunnelReuseModeSelector(t *testing.T) {
	proto := trusttunnelProto(t)
	if len(proto.Selectors) != 1 {
		t.Fatalf("TrustTunnel 必须声明唯一 reuse_mode selector: %+v", proto.Selectors)
	}
	selector := proto.Selectors[0]
	if selector.Name != "reuse_mode" || strings.Join(selector.Values, ",") != "none,connections,streams" || selector.Default != "none" {
		t.Fatalf("reuse_mode selector 合同不符: %+v", selector)
	}
	modeField := trusttunnelFieldMust(t, "reuse-mode")
	if !modeField.StateOnly || modeField.SelectorName != "reuse_mode" {
		t.Fatalf("reuse-mode 必须是 state_only selector 字段: %+v", modeField)
	}

	connections := []string{"connections"}
	streams := []string{"streams"}
	for _, name := range []string{"max-connections", "min-streams"} {
		field := trusttunnelFieldMust(t, name)
		if field.When == nil || strings.Join(field.When.Selectors["reuse_mode"], ",") != strings.Join(connections, ",") {
			t.Fatalf("%s 必须只在 connections 分支活动: %+v", name, field)
		}
		if !field.ShouldReset("selector.reuse_mode") {
			t.Fatalf("%s 必须随 selector 切换清空: %+v", name, field)
		}
	}
	// 按用户确认：connections 分支要求 max-connections 必填，min-streams 可选正整数。
	if condition := trusttunnelFieldMust(t, "max-connections").RequiredWhen; condition == nil {
		t.Fatal("max-connections 在 connections 分支必须必填")
	}
	if condition := trusttunnelFieldMust(t, "min-streams").RequiredWhen; condition != nil {
		t.Fatal("min-streams 在 connections 分支必须可选")
	}
	maxStreams := trusttunnelFieldMust(t, "max-streams")
	if maxStreams.When == nil || strings.Join(maxStreams.When.Selectors["reuse_mode"], ",") != strings.Join(streams, ",") {
		t.Fatalf("max-streams 必须只在 streams 分支活动: %+v", maxStreams)
	}
	if maxStreams.RequiredWhen == nil {
		t.Fatal("max-streams 在 streams 分支必须必填")
	}

	svc, _, _ := newTestService(t)
	// none 分支清空三个复用数字。
	created, err := createTrustTunnel(t, svc, "tt-reuse-none",
		map[string]any{"max-connections": 8, "min-streams": 5, "max-streams": 4}, reuseSelectors("none"))
	if err != nil {
		t.Fatalf("none 分支应可创建: %v", err)
	}
	for _, key := range []string{"max-connections", "min-streams", "max-streams"} {
		if _, exists := created.ProtocolJSON[key]; exists {
			t.Fatalf("none 分支必须清空 %s: %+v", key, created.ProtocolJSON)
		}
	}

	// connections 分支缺 max-connections 必须阻断。
	if _, err := createTrustTunnel(t, svc, "tt-conn-missing",
		map[string]any{"min-streams": 5}, reuseSelectors("connections")); err == nil || !strings.Contains(err.Error(), "max-connections") {
		t.Fatalf("connections 分支缺 max-connections 必须按字段拒绝，实际: %v", err)
	}
	// connections 分支只填 max-connections 合法。
	if _, err := createTrustTunnel(t, svc, "tt-conn-ok",
		map[string]any{"max-connections": 8}, reuseSelectors("connections")); err != nil {
		t.Fatalf("connections 分支只提供 max-connections 必须合法: %v", err)
	}
	// connections 分支两个字段都提供也合法。
	if _, err := createTrustTunnel(t, svc, "tt-conn-both",
		map[string]any{"max-connections": 8, "min-streams": 5}, reuseSelectors("connections")); err != nil {
		t.Fatalf("connections 分支提供两个正整数必须合法: %v", err)
	}
	// streams 分支缺 max-streams 必须阻断。
	if _, err := createTrustTunnel(t, svc, "tt-streams-missing", map[string]any{}, reuseSelectors("streams")); err == nil || !strings.Contains(err.Error(), "max-streams") {
		t.Fatalf("streams 分支缺 max-streams 必须按字段拒绝，实际: %v", err)
	}
	// 正整数约束。
	for _, tc := range []struct {
		mode  string
		key   string
		value any
	}{
		{mode: "connections", key: "max-connections", value: 0},
		{mode: "connections", key: "max-connections", value: -1},
		{mode: "connections", key: "min-streams", value: 0},
		{mode: "connections", key: "min-streams", value: 2.5},
		{mode: "streams", key: "max-streams", value: 0},
		{mode: "streams", key: "max-streams", value: -3},
	} {
		t.Run(tc.mode+"-"+tc.key+"-"+fmt.Sprint(tc.value), func(t *testing.T) {
			params := map[string]any{tc.key: tc.value}
			if tc.mode == "connections" && tc.key == "min-streams" {
				params["max-connections"] = 8
			}
			if _, err := createTrustTunnel(t, svc, "tt-positive-"+tc.key, params, reuseSelectors(tc.mode)); err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("%s=%v 必须是正整数，实际: %v", tc.key, tc.value, err)
			}
		})
	}
	// 两组不得混合：显式 connections 时 max-streams 不得落库。
	created, err = createTrustTunnel(t, svc, "tt-no-mixing",
		map[string]any{"max-connections": 8, "max-streams": 4}, reuseSelectors("connections"))
	if err != nil {
		t.Fatalf("混合提交应被规范化而不是报错: %v", err)
	}
	if _, exists := created.ProtocolJSON["max-streams"]; exists {
		t.Fatalf("connections 分支不得同时保存 max-streams: %+v", created.ProtocolJSON)
	}
}

// TestTrustTunnelReuseBranchNotRestored 覆盖 selector 切换清空旧值且 A→B→A 不恢复。
func TestTrustTunnelReuseBranchNotRestored(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	created, err := createTrustTunnel(t, svc, "tt-switch",
		map[string]any{"max-connections": 8, "min-streams": 5}, reuseSelectors("connections"))
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	// A → B：切到 streams，旧复用数字必须清空。
	updated, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Name: created.Name, Protocol: "trusttunnel", Host: created.Host, Port: created.Port,
		ProtocolJSON: map[string]any{"max-streams": 4}, CurrentState: reuseSelectors("streams"),
		BaseRevision: created.EditRevision,
	})
	if err != nil {
		t.Fatalf("切换到 streams 失败: %v", err)
	}
	for _, key := range []string{"max-connections", "min-streams"} {
		if _, exists := updated.ProtocolJSON[key]; exists {
			t.Fatalf("切换到 streams 必须清空 %s: %+v", key, updated.ProtocolJSON)
		}
	}
	if value, ok := numberParam(updated.ProtocolJSON["max-streams"]); !ok || value != 4 {
		t.Fatalf("streams 分支必须保存 max-streams=4: %+v", updated.ProtocolJSON)
	}
	// B → A：切回 connections 且不重填 max-connections 必须阻断，不得恢复旧值 8。
	if _, err := svc.UpdateManual(ctx, updated.ID, UpdateManualInput{
		Name: updated.Name, Protocol: "trusttunnel", Host: updated.Host, Port: updated.Port,
		ProtocolJSON: map[string]any{}, CurrentState: reuseSelectors("connections"),
		BaseRevision: updated.EditRevision,
	}); err == nil || !strings.Contains(err.Error(), "max-connections") {
		t.Fatalf("A→B→A 不得恢复已清空的 max-connections，实际: %v", err)
	}
	// 重新显式填写新值后合法，且旧 max-streams 不复活。
	final, err := svc.UpdateManual(ctx, updated.ID, UpdateManualInput{
		Name: updated.Name, Protocol: "trusttunnel", Host: updated.Host, Port: updated.Port,
		ProtocolJSON: map[string]any{"max-connections": 16}, CurrentState: reuseSelectors("connections"),
		BaseRevision: updated.EditRevision,
	})
	if err != nil {
		t.Fatalf("重新填写 max-connections 失败: %v", err)
	}
	if value, ok := numberParam(final.ProtocolJSON["max-connections"]); !ok || value != 16 {
		t.Fatalf("必须保存新值 16: %+v", final.ProtocolJSON)
	}
	if _, exists := final.ProtocolJSON["max-streams"]; exists {
		t.Fatalf("max-streams 不得复活: %+v", final.ProtocolJSON)
	}
	if _, exists := final.ProtocolJSON["min-streams"]; exists {
		t.Fatalf("min-streams 不得复活: %+v", final.ProtocolJSON)
	}
}

// TestTrustTunnelQUICGate 覆盖 udp／health-check／quic 独立 bool 与 quic 关闭清空 QUIC 调优字段。
func TestTrustTunnelQUICGate(t *testing.T) {
	for _, name := range []string{"udp", "health-check", "quic"} {
		field := trusttunnelFieldMust(t, name)
		if field.Type != "bool" || field.StateOnly {
			t.Fatalf("%s 必须是独立 bool: %+v", name, field)
		}
	}
	quic := trusttunnelFieldMust(t, "quic")
	if quic.Feature == nil || quic.Feature.Name != "quic" {
		t.Fatalf("quic 必须作为功能开关驱动 QUIC 调优字段: %+v", quic)
	}
	for _, name := range []string{"congestion-controller", "cwnd", "bbr-profile"} {
		field := trusttunnelFieldMust(t, name)
		if field.When == nil || !contains(field.When.Features, "quic") {
			t.Fatalf("%s 必须只在 quic 开启时活动: %+v", name, field)
		}
		if !field.ShouldReset("feature.quic") {
			t.Fatalf("%s 必须随 quic 关闭清空: %+v", name, field)
		}
	}

	svc, _, _ := newTestService(t)
	// quic=false 时三个调优字段必须清空。
	created, err := createTrustTunnel(t, svc, "tt-quic-off", map[string]any{
		"quic": false, "congestion-controller": "bbr", "cwnd": 64, "bbr-profile": "standard",
	}, nil)
	if err != nil {
		t.Fatalf("quic=false 应可创建: %v", err)
	}
	for _, key := range []string{"congestion-controller", "cwnd", "bbr-profile"} {
		if _, exists := created.ProtocolJSON[key]; exists {
			t.Fatalf("quic=false 必须清空 %s: %+v", key, created.ProtocolJSON)
		}
	}
	// quic=true 时保留，且独立 bool 正常保存。
	created, err = createTrustTunnel(t, svc, "tt-quic-on", map[string]any{
		"quic": true, "udp": true, "health-check": true,
		"congestion-controller": "bbr_meta_v2", "cwnd": 64, "bbr-profile": "standard",
	}, nil)
	if err != nil {
		t.Fatalf("quic=true 应可创建: %v", err)
	}
	if created.ProtocolJSON["quic"] != true || created.ProtocolJSON["udp"] != true || created.ProtocolJSON["health-check"] != true {
		t.Fatalf("独立 bool 必须分别保存: %+v", created.ProtocolJSON)
	}
	if created.ProtocolJSON["congestion-controller"] != "bbr_meta_v2" {
		t.Fatalf("quic=true 必须保留拥塞控制器: %+v", created.ProtocolJSON)
	}
	if value, ok := numberParam(created.ProtocolJSON["cwnd"]); !ok || value != 64 {
		t.Fatalf("quic=true 必须保留 cwnd: %+v", created.ProtocolJSON)
	}
	// 非法枚举与非负整数约束。
	invalid := map[string]any{"quic": true, "congestion-controller": "vegas"}
	if _, err := createTrustTunnel(t, svc, "tt-quic-bad-cc", invalid, nil); err == nil || !strings.Contains(err.Error(), "congestion-controller") {
		t.Fatalf("非法拥塞控制器必须按字段拒绝，实际: %v", err)
	}
	invalid = map[string]any{"quic": true, "cwnd": -1}
	if _, err := createTrustTunnel(t, svc, "tt-quic-neg-cwnd", invalid, nil); err == nil || !strings.Contains(err.Error(), "cwnd") {
		t.Fatalf("cwnd 负数必须按字段拒绝，实际: %v", err)
	}
}

// TestTrustTunnelUnsetNotPersisted 覆盖未设置字段不强写默认值。
func TestTrustTunnelUnsetNotPersisted(t *testing.T) {
	svc, _, _ := newTestService(t)
	created, err := createTrustTunnel(t, svc, "tt-defaults", map[string]any{}, nil)
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	for _, key := range []string{"max-connections", "min-streams", "max-streams", "congestion-controller",
		"cwnd", "bbr-profile", "alpn", "sni", "certificate", "private-key", "health-check"} {
		if _, exists := created.ProtocolJSON[key]; exists {
			t.Fatalf("未设置的 %s 不得写入数据库: %+v", key, created.ProtocolJSON)
		}
	}
}

// TestTrustTunnelNoURIMapping 覆盖无 URI 映射协议不伪造 URI。
func TestTrustTunnelNoURIMapping(t *testing.T) {
	proto := trusttunnelProto(t)
	if proto.LinkMappings.SR || proto.LinkMappings.Generic {
		t.Fatalf("TrustTunnel 不得声明 URI 映射: %+v", proto.LinkMappings)
	}
}
