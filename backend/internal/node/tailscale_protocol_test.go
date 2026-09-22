// tailscale_protocol_test.go：Build32 Step 14 Tailscale 协议合同测试（schema／归一化／校验层）。
// 覆盖无 endpoint、可编辑字段边界、三态 bool、exit-node 与 LAN access 依赖、
// hostname／control-url／exit-node 校验，以及 state-dir 不进入 protocol_json。
package node

import (
	"context"
	"strings"
	"testing"
)

func tailscaleProto(t *testing.T) Protocol {
	t.Helper()
	proto, err := GetProtocol("tailscale")
	if err != nil {
		t.Fatal(err)
	}
	return proto
}

func tailscaleField(t *testing.T, name string) FieldSchema {
	t.Helper()
	for _, field := range tailscaleProto(t).FormSchema {
		if field.Name == name {
			return field
		}
	}
	t.Fatalf("Tailscale schema 缺少字段 %s", name)
	return FieldSchema{}
}

func createTailscale(t *testing.T, svc *Service, name string, params map[string]any) (*Node, error) {
	t.Helper()
	return svc.CreateManual(context.Background(), CreateManualInput{
		Name: name, Protocol: "tailscale", ProtocolJSON: params,
	})
}

// TestTailscaleEndpointPolicyHidesEndpoint 覆盖固定的 hidden／hidden endpoint policy。
func TestTailscaleEndpointPolicyHidesEndpoint(t *testing.T) {
	proto := tailscaleProto(t)
	if len(proto.EndpointPolicies) != 1 {
		t.Fatalf("Tailscale 必须声明唯一一条 hidden endpoint policy: %+v", proto.EndpointPolicies)
	}
	policy, err := MatchEndpointPolicy(proto, CurrentState{})
	if err != nil {
		t.Fatal(err)
	}
	if policy.HostMode != "hidden" || policy.PortMode != "hidden" || policy.EmitHost || policy.EmitPort {
		t.Fatalf("Tailscale 必须固定隐藏 host／port: %+v", policy)
	}
	host, port, _, err := NormalizeEndpoint(proto, CurrentState{}, "example.com", 41641)
	if err != nil || host != "" || port != 0 {
		t.Fatalf("Tailscale endpoint 必须规范化为 ''/0: host=%q port=%d err=%v", host, port, err)
	}

	svc, _, _ := newTestService(t)
	created, err := createTailscale(t, svc, "ts-no-endpoint", map[string]any{})
	if err != nil {
		t.Fatalf("无 endpoint 创建失败: %v", err)
	}
	if created.Host != "" || created.Port != 0 {
		t.Fatalf("Tailscale 节点必须落库为空 endpoint，实际 %q/%d", created.Host, created.Port)
	}
}

// TestTailscaleEditableFieldBoundary 覆盖可编辑字段白名单与 state-dir 排除。
func TestTailscaleEditableFieldBoundary(t *testing.T) {
	proto := tailscaleProto(t)
	allowed := map[string]bool{
		"hostname": true, "auth-key": true, "control-url": true, "ephemeral": true, "udp": true,
		"accept-routes": true, "exit-node": true, "exit-node-allow-lan-access": true,
		"tfo": true, "mptcp": true, "interface-name": true, "routing-mark": true, "ip-version": true, "dialer-proxy": true,
	}
	for _, field := range proto.FormSchema {
		if !allowed[field.Name] {
			t.Fatalf("Tailscale 出现未允许的可编辑字段 %s", field.Name)
		}
		delete(allowed, field.Name)
	}
	// state-dir 是 adapter 派生字段，不得出现在 schema。
	if _, ok := findSchemaField(proto.FormSchema, "state-dir"); ok {
		t.Fatal("state-dir 不得出现在 Tailscale schema 中")
	}
	if len(proto.Selectors) != 0 {
		t.Fatalf("Tailscale 不应为单一合法维度制造 selector: %+v", proto.Selectors)
	}
	for _, name := range []string{"hostname", "auth-key", "control-url", "ephemeral", "udp", "accept-routes", "exit-node", "exit-node-allow-lan-access"} {
		if _, ok := findSchemaField(proto.FormSchema, name); !ok {
			t.Fatalf("Tailscale 缺少可编辑字段 %s", name)
		}
	}
}

// TestTailscaleAuthKeyOptional 覆盖 auth-key 可空、为 secret，且不进入 protocol_json 之外。
func TestTailscaleAuthKeyOptional(t *testing.T) {
	field := tailscaleField(t, "auth-key")
	if field.Type != "password" || field.Required || field.RequiredWhen != nil {
		t.Fatalf("auth-key 必须是可空 secret 字段: %+v", field)
	}
	if !contains(protoSensitiveFields(t), "auth-key") {
		t.Fatal("auth-key 必须在 SensitiveFields 中")
	}
	svc, _, _ := newTestService(t)
	created, err := createTailscale(t, svc, "ts-empty-auth", map[string]any{"hostname": "node-a"})
	if err != nil {
		t.Fatalf("空 auth-key 必须允许保存: %v", err)
	}
	if _, exists := created.ProtocolJSON["auth-key"]; exists {
		t.Fatalf("未填写的 auth-key 不应进入 protocol_json: %+v", created.ProtocolJSON)
	}
	if contains(created.SavedSensitivePaths, "auth-key") {
		t.Fatalf("未配置的 auth-key 不应标记为已保存密文: %v", created.SavedSensitivePaths)
	}
}

// TestTailscaleTriStateBooleans 覆盖 accept-routes／exit-node-allow-lan-access 的 unset/false/true 三态。
func TestTailscaleTriStateBooleans(t *testing.T) {
	for _, name := range []string{"accept-routes", "exit-node-allow-lan-access"} {
		field := tailscaleField(t, name)
		if field.Type != "bool" {
			t.Fatalf("%s 必须是 bool: %+v", name, field)
		}
		if field.Default != nil {
			t.Fatalf("%s 必须保持三态（不声明 default），实际 %#v", name, field.Default)
		}
	}

	svc, _, _ := newTestService(t)
	ctx := context.Background()

	// unset：不落库，保持未设置。
	base, err := createTailscale(t, svc, "ts-tristate-unset", map[string]any{"hostname": "node-a"})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	for _, name := range []string{"accept-routes", "exit-node-allow-lan-access"} {
		if _, exists := base.ProtocolJSON[name]; exists {
			t.Fatalf("未设置的三态 bool %s 不应落库: %+v", name, base.ProtocolJSON)
		}
	}

	// 显式 false 必须保留，不能被“未设置”吞掉。
	falsy, err := svc.UpdateManual(ctx, base.ID, UpdateManualInput{
		Protocol: "tailscale", BaseRevision: base.EditRevision,
		ProtocolJSON: map[string]any{"hostname": "node-a", "accept-routes": false},
	})
	if err != nil {
		t.Fatalf("显式 false 更新失败: %v", err)
	}
	if value, exists := falsy.ProtocolJSON["accept-routes"]; !exists || value != false {
		t.Fatalf("显式 false 必须保留: %+v", falsy.ProtocolJSON)
	}

	// 显式 true 必须保留。
	truthy, err := svc.UpdateManual(ctx, base.ID, UpdateManualInput{
		Protocol: "tailscale", BaseRevision: falsy.EditRevision,
		ProtocolJSON: map[string]any{"hostname": "node-a", "accept-routes": true},
	})
	if err != nil {
		t.Fatalf("显式 true 更新失败: %v", err)
	}
	if value, exists := truthy.ProtocolJSON["accept-routes"]; !exists || value != true {
		t.Fatalf("显式 true 必须保留: %+v", truthy.ProtocolJSON)
	}
}

func protoSensitiveFields(t *testing.T) []string {
	t.Helper()
	return tailscaleProto(t).SensitiveFields
}

// TestTailscaleLANAccessDependsOnExitNode 覆盖 non_empty 条件与清空语义。
func TestTailscaleLANAccessDependsOnExitNode(t *testing.T) {
	field := tailscaleField(t, "exit-node-allow-lan-access")
	if field.When == nil || len(field.When.NonEmpty) != 1 || field.When.NonEmpty[0] != "exit-node" {
		t.Fatalf("LAN access 必须声明 non_empty 依赖 exit-node: %+v", field.When)
	}
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	// 有 exit-node 时 LAN access 保留。
	withExit, err := createTailscale(t, svc, "ts-lan-with-exit", map[string]any{
		"hostname": "node-a", "exit-node": "100.64.0.1", "exit-node-allow-lan-access": true,
	})
	if err != nil {
		t.Fatalf("带出口节点创建失败: %v", err)
	}
	if withExit.ProtocolJSON["exit-node-allow-lan-access"] != true {
		t.Fatalf("exit-node 非空时 LAN access 必须保留: %+v", withExit.ProtocolJSON)
	}

	// 清空 exit-node 时必须同步清空 LAN access。
	cleared, err := svc.UpdateManual(ctx, withExit.ID, UpdateManualInput{
		Protocol: "tailscale", BaseRevision: withExit.EditRevision,
		ProtocolJSON: map[string]any{"hostname": "node-a", "exit-node": ""},
	})
	if err != nil {
		t.Fatalf("清空 exit-node 失败: %v", err)
	}
	if _, exists := cleared.ProtocolJSON["exit-node-allow-lan-access"]; exists {
		t.Fatalf("清空 exit-node 必须同步清空 LAN access: %+v", cleared.ProtocolJSON)
	}

	// 未提供 exit-node 时提交的 LAN access 不得落库。
	onlyLAN, err := createTailscale(t, svc, "ts-lan-without-exit", map[string]any{
		"hostname": "node-b", "exit-node-allow-lan-access": true,
	})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, exists := onlyLAN.ProtocolJSON["exit-node-allow-lan-access"]; exists {
		t.Fatalf("exit-node 为空时 LAN access 不得落库: %+v", onlyLAN.ProtocolJSON)
	}
}

// TestTailscaleHostnameValidation 覆盖设备名约束。
func TestTailscaleHostnameValidation(t *testing.T) {
	svc, _, _ := newTestService(t)
	for _, tc := range []struct {
		name     string
		hostname string
		ok       bool
	}{
		{name: "simple", hostname: "node-a", ok: true},
		{name: "digits", hostname: "node01", ok: true},
		{name: "empty allowed", hostname: "", ok: true},
		{name: "uppercase", hostname: "Node-A", ok: false},
		{name: "leading hyphen", hostname: "-node", ok: false},
		{name: "trailing hyphen", hostname: "node-", ok: false},
		{name: "underscore", hostname: "node_a", ok: false},
		{name: "space", hostname: "node a", ok: false},
		{name: "too long", hostname: strings.Repeat("a", 64), ok: false},
		{name: "max length", hostname: strings.Repeat("a", 63), ok: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := createTailscale(t, svc, "ts-host-"+strings.ReplaceAll(tc.name, " ", "-"), map[string]any{"hostname": tc.hostname})
			if tc.ok && err != nil {
				t.Fatalf("合法 hostname %q 应通过: %v", tc.hostname, err)
			}
			if !tc.ok && (err == nil || !strings.Contains(err.Error(), "hostname")) {
				t.Fatalf("非法 hostname %q 必须按字段拒绝，实际: %v", tc.hostname, err)
			}
		})
	}
}

// TestTailscaleControlURLValidation 覆盖 control-url 为空即官方控制面、非空必须是绝对 HTTP(S)。
func TestTailscaleControlURLValidation(t *testing.T) {
	svc, _, _ := newTestService(t)
	for _, tc := range []struct {
		name string
		url  string
		ok   bool
	}{
		{name: "empty official", url: "", ok: true},
		{name: "https headscale", url: "https://headscale.example.com", ok: true},
		{name: "http local headscale", url: "http://headscale.local:8080", ok: true},
		{name: "relative", url: "/control", ok: false},
		{name: "no host", url: "https://", ok: false},
		{name: "wrong scheme", url: "ftp://headscale.example.com", ok: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := createTailscale(t, svc, "ts-control-"+strings.ReplaceAll(tc.name, " ", "-"), map[string]any{"control-url": tc.url})
			if tc.ok && err != nil {
				t.Fatalf("合法 control-url %q 应通过: %v", tc.url, err)
			}
			if !tc.ok && (err == nil || !strings.Contains(err.Error(), "control-url")) {
				t.Fatalf("非法 control-url %q 必须按字段拒绝，实际: %v", tc.url, err)
			}
		})
	}
	// 非 HTTPS 只给安全提示，不阻止本地 Headscale。
	_, err := createTailscale(t, svc, "ts-control-insecure", map[string]any{"control-url": "http://headscale.local:8080"})
	if err != nil {
		t.Fatalf("非 HTTPS 控制面不得被禁止: %v", err)
	}
}

// TestTailscaleExitNodeValidation 覆盖 exit-node 接受合法 IP 或 auto:* 形式。
func TestTailscaleExitNodeValidation(t *testing.T) {
	svc, _, _ := newTestService(t)
	for _, tc := range []struct {
		name     string
		exitNode string
		ok       bool
	}{
		{name: "empty", exitNode: "", ok: true},
		{name: "ipv4", exitNode: "100.64.0.1", ok: true},
		{name: "ipv6", exitNode: "fd7a:115c:a1e0::1", ok: true},
		{name: "auto any", exitNode: "auto:any", ok: true},
		{name: "auto preferred", exitNode: "auto:preferred", ok: true},
		{name: "auto empty suffix", exitNode: "auto:", ok: false},
		{name: "bare hostname", exitNode: "exit.example.com", ok: false},
		{name: "garbage", exitNode: "not a node", ok: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := createTailscale(t, svc, "ts-exit-"+strings.ReplaceAll(tc.name, " ", "-"), map[string]any{"exit-node": tc.exitNode})
			if tc.ok && err != nil {
				t.Fatalf("合法 exit-node %q 应通过: %v", tc.exitNode, err)
			}
			if !tc.ok && (err == nil || !strings.Contains(err.Error(), "exit-node")) {
				t.Fatalf("非法 exit-node %q 必须按字段拒绝，实际: %v", tc.exitNode, err)
			}
		})
	}
}

// TestTailscaleStateDirNeverInProtocolJSON 覆盖 state-dir 不出现在 schema／protocol_json／响应。
func TestTailscaleStateDirNeverInProtocolJSON(t *testing.T) {
	svc, _, _ := newTestService(t)
	created, err := createTailscale(t, svc, "ts-no-state-dir", map[string]any{"hostname": "node-a", "auth-key": "ts-auth-secret"})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, exists := created.ProtocolJSON["state-dir"]; exists {
		t.Fatalf("state-dir 不得进入 protocol_json: %+v", created.ProtocolJSON)
	}
	// 请求伪造 state-dir 必须被顶层白名单拒绝。
	if _, err := createTailscale(t, svc, "ts-forge-state-dir", map[string]any{"state-dir": "/tmp/evil"}); err == nil {
		t.Fatal("请求不得直接提交 state-dir")
	}
}

// TestTailscaleNonEmptyConditionMatching 覆盖 non_empty 条件在后端匹配中的语义。
func TestTailscaleNonEmptyConditionMatching(t *testing.T) {
	field := tailscaleField(t, "exit-node-allow-lan-access")
	state := CurrentState{}
	if field.Matches(state, map[string]any{}, "") {
		t.Fatal("exit-node 缺失时 non_empty 条件必须不匹配")
	}
	if field.Matches(state, map[string]any{"exit-node": "   "}, "") {
		t.Fatal("exit-node 仅有空白时 non_empty 条件必须不匹配")
	}
	if !field.Matches(state, map[string]any{"exit-node": "100.64.0.1"}, "") {
		t.Fatal("exit-node 非空时 non_empty 条件必须匹配")
	}
	// endpoint policy 不允许声明 non_empty 依赖（注册表初始化时校验）。
	proto := Protocol{Protocol: "x", EndpointPolicies: []EndpointPolicy{{
		When: &ConditionRule{NonEmpty: []string{"exit-node"}}, HostMode: "required", PortMode: "required", EmitHost: true, EmitPort: true,
	}}}
	if err := validateProtocolEndpointPolicies(proto); err == nil {
		t.Fatal("endpoint policy 不得声明 non_empty 条件")
	}
}
