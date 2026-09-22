// tuic_protocol_test.go：Build32 Step 10 TUIC 协议合同测试。
// 覆盖 auth_mode=v4/v5 凭据强互斥、UUID 与 IP 校验、UOT 版本、数据报／中继包联动、
// disable-sni 清空冲突 SNI 与风险提示、TLS／mTLS 及敏感路径。
package node

import (
	"context"
	"strings"
	"testing"
)

func tuicCreateInput(name string, params map[string]any, state *CurrentState) CreateManualInput {
	return CreateManualInput{
		Name: name, Protocol: "tuic", Host: "example.com", Port: 443,
		ProtocolJSON: params, CurrentState: state,
	}
}

func tuicState(mode string) *CurrentState {
	return &CurrentState{Selectors: map[string]string{"auth_mode": mode}}
}

func tuicUOTState(mode string) *CurrentState {
	return &CurrentState{Selectors: map[string]string{"auth_mode": mode}, Features: []string{"udp-over-stream"}}
}

const tuicTestUUID = "11111111-2222-3333-4444-555555555555"

func TestTUICAuthModeSelectorDeclaration(t *testing.T) {
	proto, err := GetProtocol("tuic")
	if err != nil {
		t.Fatal(err)
	}
	selector, ok := selectorSchemaByName(proto, "auth_mode")
	if !ok || selector.SourceField != "" || selector.Default != "v5" {
		t.Fatalf("TUIC 必须声明 state_only auth_mode selector: %+v", selector)
	}
	if len(selector.Values) != 2 || selector.Values[0] != "v4" || selector.Values[1] != "v5" {
		t.Fatalf("auth_mode 允许值必须为 v4/v5: %+v", selector.Values)
	}
	for _, name := range []string{"token", "uuid", "password"} {
		field, ok := findSchemaField(proto.FormSchema, name)
		if !ok || field.When == nil || field.RequiredWhen == nil || !field.ShouldReset("selector.auth_mode") {
			t.Fatalf("%s 必须声明分支条件、条件必填与 selector 清空: %+v", name, field)
		}
	}
}

func TestTUICCongestionControllerEnum(t *testing.T) {
	proto, err := GetProtocol("tuic")
	if err != nil {
		t.Fatal(err)
	}
	field, ok := findSchemaField(proto.FormSchema, "congestion-controller")
	want := []string{"", "cubic", "new_reno", "bbr_meta_v1", "bbr_meta_v2", "bbr"}
	if !ok || field.Type != "select" || len(field.Options) != len(want) {
		t.Fatalf("TUIC congestion-controller 必须声明固定 tag 枚举: %+v", field)
	}
	for i := range want {
		if field.Options[i] != want[i] {
			t.Fatalf("congestion-controller 允许值顺序异常: %+v", field.Options)
		}
	}

	svc, _, _ := newTestService(t)
	_, err = svc.CreateManual(context.Background(), tuicCreateInput("tuic-bad-congestion",
		map[string]any{"uuid": tuicTestUUID, "password": "pw", "congestion-controller": "unknown"}, tuicState("v5")))
	if err == nil || !strings.Contains(err.Error(), "congestion-controller") {
		t.Fatalf("未知拥塞控制器必须返回字段级错误，实际: %v", err)
	}
}

func TestTUICAuthModeDerivation(t *testing.T) {
	proto, _ := GetProtocol("tuic")
	if got := DeriveCurrentState(proto, map[string]any{"token": "t"}).Selectors["auth_mode"]; got != "v4" {
		t.Fatalf("存在 token 应派生 v4，实际 %q", got)
	}
	if got := DeriveCurrentState(proto, map[string]any{"uuid": tuicTestUUID}).Selectors["auth_mode"]; got != "v5" {
		t.Fatalf("存在 uuid 应派生 v5，实际 %q", got)
	}
	if got := DeriveCurrentState(proto, map[string]any{}).Selectors["auth_mode"]; got != "v5" {
		t.Fatalf("均无值应按 tag 默认版本派生 v5，实际 %q", got)
	}
}

func TestTUICCredentialBranchesAreMutuallyExclusive(t *testing.T) {
	svc, _, _ := newTestService(t)
	// v4：只活动 token，uuid/password 被清空。
	created, err := svc.CreateManual(context.Background(), tuicCreateInput("tuic-v4",
		map[string]any{"token": "token-secret", "uuid": tuicTestUUID, "password": "pw-secret"}, tuicState("v4")))
	if err != nil {
		t.Fatalf("v4 创建失败: %v", err)
	}
	for _, key := range []string{"uuid", "password"} {
		if _, exists := created.ProtocolJSON[key]; exists {
			t.Fatalf("v4 分支必须清空 %s: %+v", key, created.ProtocolJSON)
		}
	}
	// v5：只活动 uuid/password，token 被清空。
	createdV5, err := svc.CreateManual(context.Background(), tuicCreateInput("tuic-v5",
		map[string]any{"token": "token-secret", "uuid": tuicTestUUID, "password": "pw-secret"}, tuicState("v5")))
	if err != nil {
		t.Fatalf("v5 创建失败: %v", err)
	}
	if _, exists := createdV5.ProtocolJSON["token"]; exists {
		t.Fatalf("v5 分支必须清空 token: %+v", createdV5.ProtocolJSON)
	}
	// A→B→A：切回 v4 不恢复旧 token。
	back, err := svc.UpdateManual(context.Background(), createdV5.ID, UpdateManualInput{
		Protocol: "tuic", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"token": "second-token", "uuid": tuicTestUUID, "password": "pw-secret"},
		CurrentState: tuicState("v4"), BaseRevision: createdV5.EditRevision,
		ResetScopes: []string{"selector.auth_mode"},
	})
	if err != nil {
		t.Fatalf("切回 v4 失败: %v", err)
	}
	if _, exists := back.ProtocolJSON["uuid"]; exists {
		t.Fatalf("切回 v4 必须清空 uuid: %+v", back.ProtocolJSON)
	}
	if _, exists := back.ProtocolJSON["password"]; exists {
		t.Fatalf("切回 v4 必须清空 password: %+v", back.ProtocolJSON)
	}
}

func TestTUICV5RequiresValidUUIDAndPassword(t *testing.T) {
	svc, _, _ := newTestService(t)
	cases := []struct {
		name   string
		params map[string]any
		want   string
	}{
		{name: "invalid uuid", params: map[string]any{"uuid": "not-a-uuid", "password": "pw"}, want: "uuid"},
		{name: "empty uuid", params: map[string]any{"password": "pw"}, want: "uuid"},
		{name: "empty password", params: map[string]any{"uuid": tuicTestUUID}, want: "password"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateManual(context.Background(), tuicCreateInput("tuic-v5-"+strings.ReplaceAll(tc.name, " ", "-"), tc.params, tuicState("v5")))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("v5 必须返回 %s 字段级错误，实际: %v", tc.want, err)
			}
		})
	}
}

func TestTUICIPMustBeIP(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.CreateManual(context.Background(), tuicCreateInput("tuic-bad-ip",
		map[string]any{"uuid": tuicTestUUID, "password": "pw", "ip": "not-an-ip"}, tuicState("v5")))
	if err == nil || !strings.Contains(err.Error(), "ip") {
		t.Fatalf("非法 ip 必须返回字段级错误，实际: %v", err)
	}
	if _, err := svc.CreateManual(context.Background(), tuicCreateInput("tuic-good-ip",
		map[string]any{"uuid": tuicTestUUID, "password": "pw", "ip": "192.0.2.10"}, tuicState("v5"))); err != nil {
		t.Fatalf("合法 ip 应通过: %v", err)
	}
}

func TestTUICUDPOverStreamVersion(t *testing.T) {
	svc, _, _ := newTestService(t)
	// 关闭 UOT 时版本不活动并被清空。
	created, err := svc.CreateManual(context.Background(), tuicCreateInput("tuic-uot-off",
		map[string]any{"uuid": tuicTestUUID, "password": "pw", "udp-over-stream": false, "udp-over-stream-version": "2"}, tuicState("v5")))
	if err != nil {
		t.Fatalf("UOT 关闭创建失败: %v", err)
	}
	if _, exists := created.ProtocolJSON["udp-over-stream-version"]; exists {
		t.Fatalf("UOT 关闭必须清空 version: %+v", created.ProtocolJSON)
	}
	// 开启时只允许 1／2；输入 0 归一化为 1。
	for _, valid := range []string{"1", "2"} {
		if _, err := svc.CreateManual(context.Background(), tuicCreateInput("tuic-uot-"+valid,
			map[string]any{"uuid": tuicTestUUID, "password": "pw", "udp-over-stream": true, "udp-over-stream-version": valid}, tuicUOTState("v5"))); err != nil {
			t.Fatalf("合法 UOT 版本 %s 应通过: %v", valid, err)
		}
	}
	normalized, err := svc.CreateManual(context.Background(), tuicCreateInput("tuic-uot-zero",
		map[string]any{"uuid": tuicTestUUID, "password": "pw", "udp-over-stream": true, "udp-over-stream-version": 0}, tuicUOTState("v5")))
	if err != nil {
		t.Fatalf("UOT 版本 0 应归一化后通过: %v", err)
	}
	if got := normalized.ProtocolJSON["udp-over-stream-version"]; got != "1" {
		t.Fatalf("UOT 版本 0 必须归一化为 1，实际 %#v", got)
	}
	for _, invalid := range []string{"3", "9"} {
		_, err := svc.CreateManual(context.Background(), tuicCreateInput("tuic-uot-bad-"+invalid,
			map[string]any{"uuid": tuicTestUUID, "password": "pw", "udp-over-stream": true, "udp-over-stream-version": invalid}, tuicUOTState("v5")))
		if err == nil || !strings.Contains(err.Error(), "udp-over-stream-version") {
			t.Fatalf("非法 UOT 版本 %s 必须返回字段级错误，实际: %v", invalid, err)
		}
	}
}

func TestTUICDisableSniClearsConflictingSNI(t *testing.T) {
	svc, _, _ := newTestService(t)
	created, err := svc.CreateManual(context.Background(), tuicCreateInput("tuic-disable-sni",
		map[string]any{"uuid": tuicTestUUID, "password": "pw", "sni": "s.example.com", "disable-sni": true}, tuicState("v5")))
	if err != nil {
		t.Fatalf("disable-sni 创建失败: %v", err)
	}
	if _, exists := created.ProtocolJSON["sni"]; exists {
		t.Fatalf("disable-sni 必须清空冲突 SNI: %+v", created.ProtocolJSON)
	}
}

func TestTUICDatagramAndRelayPacketLimits(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.CreateManual(context.Background(), tuicCreateInput("tuic-datagram-too-large",
		map[string]any{"uuid": tuicTestUUID, "password": "pw", "max-datagram-frame-size": 1500}, tuicState("v5")))
	if err == nil || !strings.Contains(err.Error(), "max-datagram-frame-size") {
		t.Fatalf("超过内核上限的数据报帧必须返回字段级错误，实际: %v", err)
	}
	_, err = svc.CreateManual(context.Background(), tuicCreateInput("tuic-relay-over-datagram",
		map[string]any{"uuid": tuicTestUUID, "password": "pw",
			"max-datagram-frame-size": 1200, "max-udp-relay-packet-size": 1300}, tuicState("v5")))
	if err == nil || !strings.Contains(err.Error(), "max-udp-relay-packet-size") {
		t.Fatalf("中继包超过数据报帧必须返回字段级错误，实际: %v", err)
	}
	if _, err := svc.CreateManual(context.Background(), tuicCreateInput("tuic-packets-ok",
		map[string]any{"uuid": tuicTestUUID, "password": "pw",
			"max-datagram-frame-size": 1400, "max-udp-relay-packet-size": 1252}, tuicState("v5"))); err != nil {
		t.Fatalf("合法包大小应通过: %v", err)
	}
}

func TestTUICNegativeNumbersAndTLSKeyPair(t *testing.T) {
	svc, _, _ := newTestService(t)
	for _, field := range []string{"heartbeat-interval", "request-timeout", "max-udp-relay-packet-size",
		"max-open-streams", "cwnd", "recv-window-conn", "recv-window", "max-datagram-frame-size"} {
		_, err := svc.CreateManual(context.Background(), tuicCreateInput("tuic-negative-"+field,
			map[string]any{"uuid": tuicTestUUID, "password": "pw", field: -1}, tuicState("v5")))
		if err == nil || !strings.Contains(err.Error(), field) {
			t.Fatalf("%s 为负数必须返回字段级错误，实际: %v", field, err)
		}
	}
	_, err := svc.CreateManual(context.Background(), tuicCreateInput("tuic-mtls-partial",
		map[string]any{"uuid": tuicTestUUID, "password": "pw", "certificate": "CERT"}, tuicState("v5")))
	if err == nil || !strings.Contains(err.Error(), "private-key") {
		t.Fatalf("mTLS 缺半对必须定位 private-key，实际: %v", err)
	}
}

func TestTUICSensitivePathsDeclared(t *testing.T) {
	svc, _, _ := newTestService(t)
	created, err := svc.CreateManual(context.Background(), tuicCreateInput("tuic-sensitive",
		map[string]any{"uuid": tuicTestUUID, "password": "pw", "certificate": "CERT", "private-key": "KEY"}, tuicState("v5")))
	if err != nil {
		t.Fatalf("创建 TUIC 节点失败: %v", err)
	}
	for _, want := range []string{"uuid", "password", "private-key"} {
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
