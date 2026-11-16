// hysteria2_protocol_test.go：Build32 Step 9 Hysteria2 协议合同测试。
// 覆盖 endpoint_mode 端口替代、obfs_mode 三态与 gecko 包大小、Realm 子树与凭据、
// TLS／mTLS、QUIC 窗口关系、hop-interval 语法与敏感路径。
package node

import (
	"context"
	"strings"
	"testing"
)

func hysteria2CreateInput(name string, params map[string]any, state *CurrentState) CreateManualInput {
	return CreateManualInput{
		Name: name, Protocol: "hysteria2", Host: "example.com", Port: 443,
		ProtocolJSON: params, CurrentState: state,
	}
}

func hysteria2State(endpointMode, obfsMode string) *CurrentState {
	return &CurrentState{Selectors: map[string]string{"endpoint_mode": endpointMode, "obfs_mode": obfsMode}}
}

func TestHysteria2SelectorAndEndpointPolicyDeclaration(t *testing.T) {
	proto, err := GetProtocol("hysteria2")
	if err != nil {
		t.Fatal(err)
	}
	endpoint, ok := selectorSchemaByName(proto, "endpoint_mode")
	if !ok || endpoint.SourceField != "" || endpoint.Default != "single" {
		t.Fatalf("Hysteria2 必须声明 state_only endpoint_mode selector: %+v", endpoint)
	}
	obfs, ok := selectorSchemaByName(proto, "obfs_mode")
	if !ok || obfs.SourceField != "" || obfs.Default != "none" {
		t.Fatalf("Hysteria2 必须声明 state_only obfs_mode selector: %+v", obfs)
	}
	wantObfs := []string{"none", "salamander", "gecko"}
	for i := range wantObfs {
		if obfs.Values[i] != wantObfs[i] {
			t.Fatalf("obfs_mode 允许值顺序异常: %+v", obfs.Values)
		}
	}
	single, err := MatchEndpointPolicy(proto, CurrentState{Selectors: map[string]string{"endpoint_mode": "single"}})
	if err != nil {
		t.Fatal(err)
	}
	if single.HostMode != "required" || single.PortMode != "required" || !single.EmitHost || !single.EmitPort {
		t.Fatalf("single 模式必须输出 server＋port: %+v", single)
	}
	ports, err := MatchEndpointPolicy(proto, CurrentState{Selectors: map[string]string{"endpoint_mode": "ports"}})
	if err != nil {
		t.Fatal(err)
	}
	if ports.HostMode != "required" || ports.PortMode != "hidden" || !ports.EmitHost || ports.EmitPort {
		t.Fatalf("ports 模式必须隐藏顶层 port: %+v", ports)
	}
}

func TestHysteria2PasswordRequired(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.CreateManual(context.Background(), hysteria2CreateInput("hy2-no-password",
		map[string]any{}, hysteria2State("single", "none")))
	if err == nil || !strings.Contains(err.Error(), "password") {
		t.Fatalf("Hysteria2 密码必填，实际: %v", err)
	}
}

func TestHysteria2EndpointModeSwitching(t *testing.T) {
	svc, _, _ := newTestService(t)
	// ports 模式：ports 必填、顶层 port 规范化为 0、hop-interval 可编辑。
	created, err := svc.CreateManual(context.Background(), hysteria2CreateInput("hy2-ports",
		map[string]any{"password": "hy2-secret", "ports": "1000-2000", "hop-interval": "10-20"},
		hysteria2State("ports", "none")))
	if err != nil {
		t.Fatalf("ports 模式创建失败: %v", err)
	}
	if created.Port != 0 {
		t.Fatalf("ports 模式必须把顶层 port 规范化为 0，实际 %d", created.Port)
	}
	if created.ProtocolJSON["ports"] != "1000-2000" || created.ProtocolJSON["hop-interval"] != "10-20" {
		t.Fatalf("ports 模式必须保留 ports／hop-interval: %+v", created.ProtocolJSON)
	}
	// single 模式：清空 ports／hop-interval，port 保持必填。
	single, err := svc.UpdateManual(context.Background(), created.ID, UpdateManualInput{
		Protocol: "hysteria2", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"password": "hy2-secret", "ports": "1000-2000", "hop-interval": "10-20"},
		CurrentState: hysteria2State("single", "none"), BaseRevision: created.EditRevision,
		ResetScopes: []string{"selector.endpoint_mode"},
	})
	if err != nil {
		t.Fatalf("切回 single 模式失败: %v", err)
	}
	for _, key := range []string{"ports", "hop-interval"} {
		if _, exists := single.ProtocolJSON[key]; exists {
			t.Fatalf("single 模式必须清空 %s: %+v", key, single.ProtocolJSON)
		}
	}
	if single.Port != 443 {
		t.Fatalf("single 模式必须保留顶层 port，实际 %d", single.Port)
	}
}

func TestHysteria2PortsAndHopIntervalSyntax(t *testing.T) {
	svc, _, _ := newTestService(t)
	for _, valid := range []string{"1000", "1000-2000"} {
		params := map[string]any{"password": "hy2-secret", "ports": valid}
		if _, err := svc.CreateManual(context.Background(), hysteria2CreateInput("hy2-ports-"+strings.ReplaceAll(valid, "-", "_"), params, hysteria2State("ports", "none"))); err != nil {
			t.Fatalf("合法 ports %q 应通过: %v", valid, err)
		}
	}
	for _, invalid := range []string{"abc", "0-100", "2000-1000"} {
		params := map[string]any{"password": "hy2-secret", "ports": invalid}
		_, err := svc.CreateManual(context.Background(), hysteria2CreateInput("hy2-ports-bad-"+strings.NewReplacer("-", "_").Replace(invalid), params, hysteria2State("ports", "none")))
		if err == nil || !strings.Contains(err.Error(), "ports") {
			t.Fatalf("非法 ports %q 必须返回字段级错误，实际: %v", invalid, err)
		}
	}
	// hop-interval 支持单值或单范围，且最终最小值不低于 5 秒。
	for _, valid := range []string{"10", "10-20"} {
		params := map[string]any{"password": "hy2-secret", "ports": "1000-2000", "hop-interval": valid}
		if _, err := svc.CreateManual(context.Background(), hysteria2CreateInput("hy2-hop-"+strings.ReplaceAll(valid, "-", "_"), params, hysteria2State("ports", "none"))); err != nil {
			t.Fatalf("合法 hop-interval %q 应通过: %v", valid, err)
		}
	}
	for _, invalid := range []string{"abc", "1", "1-3", "20-10"} {
		params := map[string]any{"password": "hy2-secret", "ports": "1000-2000", "hop-interval": invalid}
		_, err := svc.CreateManual(context.Background(), hysteria2CreateInput("hy2-hop-bad-"+strings.NewReplacer("-", "_").Replace(invalid), params, hysteria2State("ports", "none")))
		if err == nil || !strings.Contains(err.Error(), "hop-interval") {
			t.Fatalf("非法 hop-interval %q 必须返回字段级错误，实际: %v", invalid, err)
		}
	}
}

func TestHysteria2ObfsBranches(t *testing.T) {
	svc, _, _ := newTestService(t)
	// none：清空全部 obfs 子字段。
	created, err := svc.CreateManual(context.Background(), hysteria2CreateInput("hy2-obfs-none",
		map[string]any{"password": "hy2-secret", "obfs-password": "obfs-secret",
			"obfs-min-packet-size": 10, "obfs-max-packet-size": 20}, hysteria2State("single", "none")))
	if err != nil {
		t.Fatalf("obfs none 创建失败: %v", err)
	}
	for _, key := range []string{"obfs-password", "obfs-min-packet-size", "obfs-max-packet-size"} {
		if _, exists := created.ProtocolJSON[key]; exists {
			t.Fatalf("obfs_mode=none 必须清空 %s: %+v", key, created.ProtocolJSON)
		}
	}
	// salamander：要求密码；包大小不在该分支活动。
	_, err = svc.CreateManual(context.Background(), hysteria2CreateInput("hy2-obfs-salamander-no-pass",
		map[string]any{"password": "hy2-secret"}, hysteria2State("single", "salamander")))
	if err == nil || !strings.Contains(err.Error(), "obfs-password") {
		t.Fatalf("salamander 缺少密码必须定位 obfs-password，实际: %v", err)
	}
	// gecko：包大小活动且 min≤max。
	_, err = svc.CreateManual(context.Background(), hysteria2CreateInput("hy2-obfs-gecko-bad-range",
		map[string]any{"password": "hy2-secret", "obfs-password": "obfs-secret",
			"obfs-min-packet-size": 100, "obfs-max-packet-size": 20}, hysteria2State("single", "gecko")))
	if err == nil || !strings.Contains(err.Error(), "obfs-min-packet-size") {
		t.Fatalf("gecko 包大小倒置必须返回字段级错误，实际: %v", err)
	}
	if _, err := svc.CreateManual(context.Background(), hysteria2CreateInput("hy2-obfs-gecko-ok",
		map[string]any{"password": "hy2-secret", "obfs-password": "obfs-secret",
			"obfs-min-packet-size": 20, "obfs-max-packet-size": 100}, hysteria2State("single", "gecko"))); err != nil {
		t.Fatalf("gecko 合法包大小应通过: %v", err)
	}
}

func TestHysteria2TLSAndWindowRelations(t *testing.T) {
	svc, _, _ := newTestService(t)
	params := map[string]any{"password": "hy2-secret", "certificate": "CERT"}
	_, err := svc.CreateManual(context.Background(), hysteria2CreateInput("hy2-mtls-partial", params, hysteria2State("single", "none")))
	if err == nil || !strings.Contains(err.Error(), "private-key") {
		t.Fatalf("mTLS 缺半对必须定位 private-key，实际: %v", err)
	}
	params = map[string]any{"password": "hy2-secret",
		"initial-stream-receive-window": 4096, "max-stream-receive-window": 1024}
	_, err = svc.CreateManual(context.Background(), hysteria2CreateInput("hy2-window-bad", params, hysteria2State("single", "none")))
	if err == nil || !strings.Contains(err.Error(), "window") {
		t.Fatalf("流窗口 initial>max 必须返回字段级错误，实际: %v", err)
	}
	params = map[string]any{"password": "hy2-secret",
		"initial-stream-receive-window": 1024, "max-stream-receive-window": 4096,
		"initial-connection-receive-window": 2048, "max-connection-receive-window": 8192}
	if _, err := svc.CreateManual(context.Background(), hysteria2CreateInput("hy2-window-ok", params, hysteria2State("single", "none"))); err != nil {
		t.Fatalf("合法 QUIC 窗口关系应通过: %v", err)
	}
}

func TestHysteria2BandwidthAndIntegerBoundaries(t *testing.T) {
	svc, _, _ := newTestService(t)
	for _, tc := range []struct {
		field string
		value any
	}{
		{field: "up", value: "not-a-rate"},
		{field: "down", value: "0 Mbps"},
		{field: "cwnd", value: 0},
		{field: "udp-mtu", value: -1},
		{field: "handshake-timeout", value: 1.5},
		{field: "initial-stream-receive-window", value: 1.5},
		{field: "max-stream-receive-window", value: -1},
		{field: "initial-connection-receive-window", value: 1.5},
		{field: "max-connection-receive-window", value: -1},
	} {
		params := map[string]any{"password": "hy2-secret", tc.field: tc.value}
		_, err := svc.CreateManual(context.Background(), hysteria2CreateInput(
			"hy2-invalid-boundary-"+tc.field, params, hysteria2State("single", "none")))
		if err == nil || !strings.Contains(err.Error(), tc.field) {
			t.Fatalf("%s=%v 必须返回字段级错误，实际: %v", tc.field, tc.value, err)
		}
	}

	params := map[string]any{
		"password": "hy2-secret", "up": "100 Mbps", "down": "1 Gbps",
		"cwnd": 1, "udp-mtu": 1200, "handshake-timeout": 1,
		"initial-stream-receive-window": 0, "max-stream-receive-window": 4096,
		"initial-connection-receive-window": 0, "max-connection-receive-window": 8192,
	}
	if _, err := svc.CreateManual(context.Background(), hysteria2CreateInput(
		"hy2-valid-boundaries", params, hysteria2State("single", "none"))); err != nil {
		t.Fatalf("合法速率、正整数和非负整数应通过: %v", err)
	}
}

func TestHysteria2RealmSubtree(t *testing.T) {
	svc, _, _ := newTestService(t)
	// enable=false 清空整个对象。
	created, err := svc.CreateManual(context.Background(), hysteria2CreateInput("hy2-realm-off",
		map[string]any{"password": "hy2-secret", "realm-opts": map[string]any{
			"enable": false, "server-url": "https://realm.example.com", "token": "realm-token"}},
		hysteria2State("single", "none")))
	if err != nil {
		t.Fatalf("realm 关闭创建失败: %v", err)
	}
	if opts, ok := created.ProtocolJSON["realm-opts"].(map[string]any); ok {
		if _, exists := opts["token"]; exists {
			t.Fatalf("realm 关闭必须清空子凭据: %+v", opts)
		}
	}
	// 开启时 server-url 必填且必须是绝对 HTTP(S)。
	_, err = svc.CreateManual(context.Background(), hysteria2CreateInput("hy2-realm-bad-url",
		map[string]any{"password": "hy2-secret", "realm-opts": map[string]any{"enable": true, "server-url": "realm.example.com"}},
		&CurrentState{Selectors: map[string]string{"endpoint_mode": "single", "obfs_mode": "none"}, Features: []string{"realm"}}))
	if err == nil || !strings.Contains(err.Error(), "server-url") {
		t.Fatalf("realm server-url 必须是绝对 URL，实际: %v", err)
	}
	created, err = svc.CreateManual(context.Background(), hysteria2CreateInput("hy2-realm-on",
		map[string]any{"password": "hy2-secret", "realm-opts": map[string]any{
			"enable": true, "server-url": "https://realm.example.com", "realm-id": "realm-1",
			"token": "realm-token", "stun-servers": []any{"stun.example.com:3478"}}},
		&CurrentState{Selectors: map[string]string{"endpoint_mode": "single", "obfs_mode": "none"}, Features: []string{"realm"}}))
	if err != nil {
		t.Fatalf("realm 开启创建失败: %v", err)
	}
	foundToken := false
	for _, saved := range created.SavedSensitivePaths {
		if saved == "realm-opts.token" {
			foundToken = true
		}
	}
	if !foundToken {
		t.Fatalf("realm token 必须按敏感路径保存: %+v", created.SavedSensitivePaths)
	}
}

func TestHysteria2SensitivePathsDeclared(t *testing.T) {
	svc, _, _ := newTestService(t)
	created, err := svc.CreateManual(context.Background(), hysteria2CreateInput("hy2-sensitive",
		map[string]any{"password": "hy2-secret", "obfs-password": "obfs-secret",
			"certificate": "CERT", "private-key": "KEY"}, hysteria2State("single", "salamander")))
	if err != nil {
		t.Fatalf("创建 Hysteria2 节点失败: %v", err)
	}
	for _, want := range []string{"password", "obfs-password", "private-key"} {
		found := false
		for _, saved := range created.SavedSensitivePaths {
			if saved == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("敏感路径 %s 未登记为已保存密文: %+v", want, created.SavedSensitivePaths)
		}
	}
}
