// mieru_protocol_test.go：Build32 Step 12 Mieru 协议合同测试。
// 覆盖 endpoint_mode selector 与 port／port-range 真互斥、固定 tag 完整枚举、
// transport 精确值、traffic-pattern Base64／语义校验与不回显原值。
package node

import (
	"context"
	"strings"
	"testing"

	mierutp "github.com/enfein/mieru/v3/apis/trafficpattern"
	"github.com/enfein/mieru/v3/pkg/appctl/appctlpb"
	"google.golang.org/protobuf/proto"
)

func mieruProto(t *testing.T) Protocol {
	t.Helper()
	proto, err := GetProtocol("mieru")
	if err != nil {
		t.Fatal(err)
	}
	return proto
}

func mieruField(t *testing.T, name string) FieldSchema {
	t.Helper()
	for _, field := range mieruProto(t).FormSchema {
		if field.Name == name {
			return field
		}
	}
	t.Fatalf("Mieru schema 缺少字段 %s", name)
	return FieldSchema{}
}

// mieruBaseParams 是 Mieru single 模式的最小合法参数。
func mieruBaseParams() map[string]any {
	return map[string]any{"username": "user", "password": "mieru-secret", "transport": "TCP"}
}

func mieruState(mode string) *CurrentState {
	return &CurrentState{Selectors: map[string]string{"endpoint_mode": mode}}
}

func createMieru(t *testing.T, svc *Service, name string, params map[string]any, state *CurrentState) (*Node, error) {
	t.Helper()
	return svc.CreateManual(context.Background(), CreateManualInput{
		Name: name, Protocol: "mieru", Host: "example.com", Port: 8964,
		ProtocolJSON: params, CurrentState: state,
	})
}

// mieruValidTrafficPattern 用固定 tag 的同一个 Encode 生成合法 Base64 流量特征。
func mieruValidTrafficPattern() string {
	return mierutp.Encode(&appctlpb.TrafficPattern{
		TcpFragment: &appctlpb.TCPFragment{Enable: proto.Bool(true), MaxSleepMs: proto.Int32(50)},
	})
}

// mieruSemanticallyInvalidTrafficPattern 生成可解析但语义越界的 Base64 流量特征。
func mieruSemanticallyInvalidTrafficPattern() string {
	return mierutp.Encode(&appctlpb.TrafficPattern{
		Nonce: &appctlpb.NoncePattern{MinLen: proto.Int32(13)},
	})
}

// TestMieruEndpointModeSelectorDeclaration 覆盖 endpoint_mode selector 与缺省派生。
func TestMieruEndpointModeSelectorDeclaration(t *testing.T) {
	proto := mieruProto(t)
	selector, ok := selectorSchemaByName(proto, "endpoint_mode")
	if !ok || selector.SourceField != "" || selector.Default != "single" {
		t.Fatalf("Mieru 必须声明 state_only endpoint_mode selector: %+v", selector)
	}
	if got := DeriveCurrentState(proto, map[string]any{"port-range": "1000-2000"}).Selectors["endpoint_mode"]; got != "range" {
		t.Fatalf("存在 port-range 应派生 range，实际 %q", got)
	}
	if got := DeriveCurrentState(proto, mieruBaseParams()).Selectors["endpoint_mode"]; got != "single" {
		t.Fatalf("无 port-range 应派生 single，实际 %q", got)
	}
}

// TestMieruEndpointPolicies 覆盖 single／range 两条 endpoint policy。
func TestMieruEndpointPolicies(t *testing.T) {
	proto := mieruProto(t)
	if len(proto.EndpointPolicies) != 2 {
		t.Fatalf("Mieru 必须声明 single／range 两条 endpoint policy: %+v", proto.EndpointPolicies)
	}
	single, err := MatchEndpointPolicy(proto, CurrentState{Selectors: map[string]string{"endpoint_mode": "single"}})
	if err != nil {
		t.Fatal(err)
	}
	if single.HostMode != "required" || single.PortMode != "required" || !single.EmitHost || !single.EmitPort {
		t.Fatalf("single 模式必须要求并输出顶层 endpoint: %+v", single)
	}
	rng, err := MatchEndpointPolicy(proto, CurrentState{Selectors: map[string]string{"endpoint_mode": "range"}})
	if err != nil {
		t.Fatal(err)
	}
	if rng.HostMode != "required" || rng.PortMode != "hidden" || !rng.EmitHost || rng.EmitPort {
		t.Fatalf("range 模式必须保留 host 并隐藏 port: %+v", rng)
	}
	host, port, _, err := NormalizeEndpoint(proto, CurrentState{Selectors: map[string]string{"endpoint_mode": "range"}}, "example.com", 8964)
	if err != nil || host != "example.com" || port != 0 {
		t.Fatalf("range 模式必须把顶层 port 规范化为 0: host=%q port=%d err=%v", host, port, err)
	}
}

// TestMieruPortRangeMutualExclusion 覆盖 port 与 port-range 严格二选一与切换清空。
func TestMieruPortRangeMutualExclusion(t *testing.T) {
	svc, _, _ := newTestService(t)

	// range 模式：port-range 必填，顶层 port 隐藏为 0。
	rangeState := mieruState("range")
	params := mieruBaseParams()
	if _, err := createMieru(t, svc, "mieru-range-missing", params, rangeState); err == nil || !strings.Contains(err.Error(), "port-range") {
		t.Fatalf("range 模式缺少 port-range 必须被拒绝，实际: %v", err)
	}
	params["port-range"] = "1000-2000"
	created, err := createMieru(t, svc, "mieru-range-ok", params, rangeState)
	if err != nil {
		t.Fatalf("range 模式创建失败: %v", err)
	}
	if created.Port != 0 {
		t.Fatalf("range 模式顶层 port 必须为 0，实际 %d", created.Port)
	}
	if created.ProtocolJSON["port-range"] != "1000-2000" {
		t.Fatalf("range 模式必须保留 port-range: %+v", created.ProtocolJSON)
	}

	// single 模式：清空 port-range，保留顶层 port。
	singleState := mieruState("single")
	created, err = createMieru(t, svc, "mieru-single-clears", params, singleState)
	if err != nil {
		t.Fatalf("single 模式创建失败: %v", err)
	}
	if _, exists := created.ProtocolJSON["port-range"]; exists {
		t.Fatalf("single 模式必须清空 port-range: %+v", created.ProtocolJSON)
	}
	if created.Port != 8964 {
		t.Fatalf("single 模式必须保留顶层 port，实际 %d", created.Port)
	}
}

// TestMieruPortRangeValidation 覆盖单个 begin-end、两端 1-65535 且 begin≤end。
func TestMieruPortRangeValidation(t *testing.T) {
	svc, _, _ := newTestService(t)
	state := mieruState("range")
	for _, tc := range []struct {
		name  string
		value string
		ok    bool
	}{
		{name: "single begin-end", value: "1000-2000", ok: true},
		{name: "same begin and end", value: "1000-1000", ok: true},
		{name: "below range", value: "0-100", ok: false},
		{name: "above range", value: "1000-70000", ok: false},
		{name: "reversed", value: "2000-1000", ok: false},
		{name: "multiple ranges", value: "1-2-3", ok: false},
		{name: "comma list", value: "1000,2000", ok: false},
		{name: "not numeric", value: "abc-def", ok: false},
		{name: "single port only", value: "1000", ok: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := mieruBaseParams()
			params["port-range"] = tc.value
			_, err := createMieru(t, svc, "mieru-range-"+strings.ReplaceAll(tc.name, " ", "-"), params, state)
			if tc.ok && err != nil {
				t.Fatalf("合法 port-range %q 应通过: %v", tc.value, err)
			}
			if !tc.ok && (err == nil || !strings.Contains(err.Error(), "port-range")) {
				t.Fatalf("非法 port-range %q 必须按字段拒绝，实际: %v", tc.value, err)
			}
		})
	}
}

// TestMieruTransportRequiresExactValues 覆盖 transport 只接受精确 TCP／UDP，不做大小写转换。
func TestMieruTransportRequiresExactValues(t *testing.T) {
	transport := mieruField(t, "transport")
	if strings.Join(transport.Options, ",") != "TCP,UDP" {
		t.Fatalf("transport 枚举必须为精确 TCP／UDP: %+v", transport.Options)
	}
	svc, _, _ := newTestService(t)
	state := mieruState("single")
	for _, tc := range []struct {
		value string
		ok    bool
	}{{"TCP", true}, {"UDP", true}, {"tcp", false}, {"udp", false}, {"Tcp", false}, {"QUIC", false}} {
		t.Run(tc.value, func(t *testing.T) {
			params := mieruBaseParams()
			params["transport"] = tc.value
			_, err := createMieru(t, svc, "mieru-transport-"+strings.ToLower(tc.value), params, state)
			if tc.ok && err != nil {
				t.Fatalf("transport=%q 应通过: %v", tc.value, err)
			}
			if !tc.ok && (err == nil || !strings.Contains(err.Error(), "transport")) {
				t.Fatalf("transport=%q 必须按字段拒绝，实际: %v", tc.value, err)
			}
		})
	}
	// 保存后不得被大小写转换。
	params := mieruBaseParams()
	params["transport"] = "UDP"
	created, err := createMieru(t, svc, "mieru-transport-kept", params, state)
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if created.ProtocolJSON["transport"] != "UDP" {
		t.Fatalf("transport 不得被改写: %+v", created.ProtocolJSON["transport"])
	}
}

// TestMieruEnumCompleteConstants 覆盖 multiplexing／handshake-mode 的完整常量名与空值默认语义。
func TestMieruEnumCompleteConstants(t *testing.T) {
	multiplexing := mieruField(t, "multiplexing")
	wantMultiplexing := []string{"", "MULTIPLEXING_OFF", "MULTIPLEXING_LOW", "MULTIPLEXING_MIDDLE", "MULTIPLEXING_HIGH"}
	if strings.Join(multiplexing.Options, ",") != strings.Join(wantMultiplexing, ",") {
		t.Fatalf("multiplexing 枚举必须是完整上游常量: %+v", multiplexing.Options)
	}
	if multiplexing.Default != "" {
		t.Fatalf("multiplexing 空值表示内核默认，不得强写默认值: %+v", multiplexing.Default)
	}
	handshake := mieruField(t, "handshake-mode")
	wantHandshake := []string{"", "HANDSHAKE_STANDARD", "HANDSHAKE_NO_WAIT"}
	if strings.Join(handshake.Options, ",") != strings.Join(wantHandshake, ",") {
		t.Fatalf("handshake-mode 枚举必须是完整上游常量: %+v", handshake.Options)
	}
	if handshake.Default != "" {
		t.Fatalf("handshake-mode 空值表示内核默认，不得强写默认值: %+v", handshake.Default)
	}

	svc, _, _ := newTestService(t)
	proto := mieruProto(t)
	// multiplexing 仍是既有标量功能域：state.features 必须与参数一致，否则先报状态不一致。
	// 因此按协议自身的派生结果构造 state，把用例聚焦在枚举值校验上。
	mieruStateFor := func(params map[string]any) *CurrentState {
		derived := DeriveCurrentState(proto, params)
		return &CurrentState{Selectors: derived.Selectors, Features: derived.Features}
	}
	for _, tc := range []struct {
		name  string
		value string
		ok    bool
	}{
		{name: "MULTIPLEXING_OFF", value: "MULTIPLEXING_OFF", ok: true},
		{name: "MULTIPLEXING_HIGH", value: "MULTIPLEXING_HIGH", ok: true},
		{name: "legacy LOW", value: "LOW", ok: false},
		{name: "legacy MIDDLE", value: "MIDDLE", ok: false},
		{name: "legacy HIGH", value: "HIGH", ok: false},
	} {
		t.Run("multiplexing "+tc.name, func(t *testing.T) {
			params := mieruBaseParams()
			params["multiplexing"] = tc.value
			_, err := createMieru(t, svc, "mieru-mux-"+strings.ToLower(tc.value), params, mieruStateFor(params))
			if tc.ok && err != nil {
				t.Fatalf("multiplexing=%q 应通过: %v", tc.value, err)
			}
			if !tc.ok && (err == nil || !strings.Contains(err.Error(), "multiplexing")) {
				t.Fatalf("multiplexing=%q 必须按字段拒绝，实际: %v", tc.value, err)
			}
		})
	}
	for _, tc := range []struct {
		name  string
		value string
		ok    bool
	}{
		{name: "HANDSHAKE_STANDARD", value: "HANDSHAKE_STANDARD", ok: true},
		{name: "HANDSHAKE_NO_WAIT", value: "HANDSHAKE_NO_WAIT", ok: true},
		{name: "default constant", value: "HANDSHAKE_DEFAULT", ok: false},
		{name: "lowercase", value: "handshake_standard", ok: false},
	} {
		t.Run("handshake "+tc.name, func(t *testing.T) {
			params := mieruBaseParams()
			params["handshake-mode"] = tc.value
			_, err := createMieru(t, svc, "mieru-hs-"+strings.ToLower(tc.value), params, mieruStateFor(params))
			if tc.ok && err != nil {
				t.Fatalf("handshake-mode=%q 应通过: %v", tc.value, err)
			}
			if !tc.ok && (err == nil || !strings.Contains(err.Error(), "handshake-mode")) {
				t.Fatalf("handshake-mode=%q 必须按字段拒绝，实际: %v", tc.value, err)
			}
		})
	}

	// 未设置时不得把默认值强写入数据库。
	created, err := createMieru(t, svc, "mieru-enum-unset", mieruBaseParams(), mieruStateFor(mieruBaseParams()))
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	for _, key := range []string{"multiplexing", "handshake-mode", "traffic-pattern"} {
		if _, exists := created.ProtocolJSON[key]; exists {
			t.Fatalf("未设置的 %s 不得写入数据库: %+v", key, created.ProtocolJSON)
		}
	}
}

// TestMieruTrafficPatternValidation 覆盖 Base64 解码＋固定 tag 语义校验，且不回显原值。
func TestMieruTrafficPatternValidation(t *testing.T) {
	if got := mieruField(t, "traffic-pattern").Type; got != "text" {
		t.Fatalf("traffic-pattern 必须是普通文本字段: %s", got)
	}
	svc, _, _ := newTestService(t)
	state := mieruState("single")

	valid := mieruValidTrafficPattern()
	if valid == "" {
		t.Fatal("测试夹具生成失败：合法 traffic-pattern 为空")
	}
	params := mieruBaseParams()
	params["traffic-pattern"] = valid
	created, err := createMieru(t, svc, "mieru-pattern-valid", params, state)
	if err != nil {
		t.Fatalf("合法 traffic-pattern 应通过: %v", err)
	}
	if created.ProtocolJSON["traffic-pattern"] != valid {
		t.Fatalf("合法 traffic-pattern 必须原样保存: %+v", created.ProtocolJSON["traffic-pattern"])
	}

	invalid := mieruSemanticallyInvalidTrafficPattern()
	if invalid == "" {
		t.Fatal("测试夹具生成失败：语义非法 traffic-pattern 为空")
	}
	for _, tc := range []struct{ name, value string }{
		{name: "not base64", value: "!!!not-base64!!!"},
		{name: "semantically invalid", value: invalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := mieruBaseParams()
			params["traffic-pattern"] = tc.value
			_, err := createMieru(t, svc, "mieru-pattern-"+strings.ReplaceAll(tc.name, " ", "-"), params, state)
			if err == nil {
				t.Fatalf("非法 traffic-pattern %q 必须被拒绝", tc.name)
			}
			if !strings.Contains(err.Error(), "traffic-pattern") {
				t.Fatalf("错误必须定位到 traffic-pattern，实际: %v", err)
			}
			if strings.Contains(err.Error(), tc.value) {
				t.Fatalf("错误文本不得回显 traffic-pattern 原值: %v", err)
			}
		})
	}
}

// TestMieruUDPIsIndependentFromTransport 覆盖 udp 转发能力与 transport 分别保存。
func TestMieruUDPIsIndependentFromTransport(t *testing.T) {
	svc, _, _ := newTestService(t)
	state := mieruState("single")
	params := mieruBaseParams()
	params["transport"] = "UDP"
	params["udp"] = false
	created, err := createMieru(t, svc, "mieru-udp-independent", params, state)
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if created.ProtocolJSON["transport"] != "UDP" || created.ProtocolJSON["udp"] != false {
		t.Fatalf("transport 与 udp 必须分别保存: %+v", created.ProtocolJSON)
	}
}

// TestMieruStateOnlyFieldRejected 覆盖 endpoint-mode 不得写入 protocol_json。
func TestMieruStateOnlyFieldRejected(t *testing.T) {
	svc, _, _ := newTestService(t)
	params := mieruBaseParams()
	params["endpoint-mode"] = "single"
	if _, err := createMieru(t, svc, "mieru-state-only", params, nil); err == nil {
		t.Fatal("state_only 字段 endpoint-mode 不得写入 protocol_json")
	}
}
