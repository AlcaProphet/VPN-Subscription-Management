// masque_protocol_test.go：Build32 Step 13 MASQUE 协议合同测试。
// 覆盖 network_mode selector、h3-l4proxy 强制关闭 UDP、QUIC 分支调优字段、
// EC 密钥结构、本地地址、URI／MTU／握手超时校验与 name-cert-verify 排除边界。
package node

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"strings"
	"testing"
)

// masqueRSAPublicKeyBase64 生成合法 PKIX 但非 ECDSA 的公钥，用于验证结构校验不能只判断 Base64。
func masqueRSAPublicKeyBase64(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("生成测试 RSA 密钥失败: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("序列化 RSA 公钥失败: %v", err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

// 固定测试用 P-256 密钥对（由 x509 在同一步骤生成，一次性测试夹具，不是真实凭据）。
const (
	masqueFixturePrivateKey = "MHcCAQEEIMufpAZbwGL1tVijQ1W75eD7XO5WPksPV4jDdBOcaewXoAoGCCqGSM49AwEHoUQDQgAEnX9WTsWhQKf1dpuENdH0NzmFspTB2M65dUgddx9WRkon8WkdycFYUetBTFGyLg+qs8VtaACuGIcnvfXkIG7L5g=="
	masqueFixturePublicKey  = "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEnX9WTsWhQKf1dpuENdH0NzmFspTB2M65dUgddx9WRkon8WkdycFYUetBTFGyLg+qs8VtaACuGIcnvfXkIG7L5g=="
)

func masqueProto(t *testing.T) Protocol {
	t.Helper()
	proto, err := GetProtocol("masque")
	if err != nil {
		t.Fatal(err)
	}
	return proto
}

func masqueField(t *testing.T, name string) FieldSchema {
	t.Helper()
	for _, field := range masqueProto(t).FormSchema {
		if field.Name == name {
			return field
		}
	}
	t.Fatalf("MASQUE schema 缺少字段 %s", name)
	return FieldSchema{}
}

// masqueTestKeys 生成一次性 P-256 密钥对，分别按固定 tag 的 SEC1 私钥与 PKIX 公钥形态编码。
func masqueTestKeys(t *testing.T) (privateKey, publicKey string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("生成测试 EC 密钥失败: %v", err)
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("序列化 EC 私钥失败: %v", err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("序列化 EC 公钥失败: %v", err)
	}
	return base64.StdEncoding.EncodeToString(der), base64.StdEncoding.EncodeToString(publicDER)
}

func masqueBaseParams(t *testing.T) map[string]any {
	t.Helper()
	privateKey, publicKey := masqueTestKeys(t)
	return map[string]any{"private-key": privateKey, "public-key": publicKey, "ip": "192.0.2.2"}
}

func masqueState(mode string) *CurrentState {
	return &CurrentState{Selectors: map[string]string{"network_mode": mode}}
}

func createMASQUE(t *testing.T, svc *Service, name string, params map[string]any, state *CurrentState) (*Node, error) {
	t.Helper()
	return svc.CreateManual(context.Background(), CreateManualInput{
		Name: name, Protocol: "masque", Host: "example.com", Port: 443,
		ProtocolJSON: params, CurrentState: state,
	})
}

// TestMASQUENetworkModeSelectorDeclaration 覆盖 network_mode selector 与空值默认 quic。
func TestMASQUENetworkModeSelectorDeclaration(t *testing.T) {
	proto := masqueProto(t)
	selector, ok := selectorSchemaByName(proto, "network_mode")
	if !ok || selector.SourceField != "" || selector.Default != "quic" {
		t.Fatalf("MASQUE 必须声明 state_only network_mode selector: %+v", selector)
	}
	if strings.Join(selector.Values, ",") != "quic,h2,h3_l4proxy" {
		t.Fatalf("network_mode 允许值异常: %+v", selector.Values)
	}
	if got := DeriveCurrentState(proto, map[string]any{}).Selectors["network_mode"]; got != "quic" {
		t.Fatalf("空值必须派生 quic，实际 %q", got)
	}
	// MASQUE 始终需要顶层 endpoint，不声明 selector 相关 endpoint policy。
	if len(proto.EndpointPolicies) != 0 {
		t.Fatalf("MASQUE 使用默认 required/required endpoint policy: %+v", proto.EndpointPolicies)
	}
	policy, err := MatchEndpointPolicy(proto, CurrentState{Selectors: map[string]string{"network_mode": "h3_l4proxy"}})
	if err != nil {
		t.Fatal(err)
	}
	if policy.HostMode != "required" || policy.PortMode != "required" || !policy.EmitHost || !policy.EmitPort {
		t.Fatalf("MASQUE endpoint policy 必须始终输出 server/port: %+v", policy)
	}
}

// TestMASQUEH3L4ProxyForcesUDPDisabled 覆盖 h3-l4proxy 强制关闭 UDP 且切回不恢复。
func TestMASQUEH3L4ProxyForcesUDPDisabled(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	base := masqueBaseParams(t)
	base["udp"] = true
	created, err := createMASQUE(t, svc, "masque-udp-quic", base, masqueState("quic"))
	if err != nil {
		t.Fatalf("quic 模式创建失败: %v", err)
	}
	if created.ProtocolJSON["udp"] != true {
		t.Fatalf("quic 模式应保留 UDP 开关: %+v", created.ProtocolJSON)
	}

	// 切到 h3_l4proxy：udp 必须被清空（内核默认 false），不得保留 true。
	l4 := masqueBaseParams(t)
	if _, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "masque", Host: "example.com", Port: 443, BaseRevision: created.EditRevision,
		ProtocolJSON: l4, CurrentState: masqueState("h3_l4proxy"),
	}); err != nil {
		t.Fatalf("h3_l4proxy 更新失败: %v", err)
	}
	l4Raw, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := l4Raw.ProtocolJSON["udp"]; exists {
		t.Fatalf("h3_l4proxy 必须清空 udp: %+v", l4Raw.ProtocolJSON)
	}

	// 切回 quic：旧 udp=true 不得复活。
	updated, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "masque", Host: "example.com", Port: 443, BaseRevision: l4Raw.EditRevision,
		ProtocolJSON: masqueBaseParams(t), CurrentState: masqueState("quic"),
	})
	if err != nil {
		t.Fatalf("切回 quic 失败: %v", err)
	}
	if updated.ProtocolJSON["udp"] == true {
		t.Fatalf("切回 quic 不得恢复旧 udp=true: %+v", updated.ProtocolJSON)
	}
}

// TestMASQUEQUICTuningOnlyActiveInQUICMode 覆盖 QUIC 调优字段只在 quic 分支活动。
func TestMASQUEQUICTuningOnlyActiveInQUICMode(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	params := masqueBaseParams(t)
	params["congestion-controller"] = "bbr_meta_v2"
	params["cwnd"] = 64
	params["bbr-profile"] = "standard"
	created, err := createMASQUE(t, svc, "masque-tuning-quic", params, masqueState("quic"))
	if err != nil {
		t.Fatalf("quic 模式创建失败: %v", err)
	}
	for _, key := range []string{"congestion-controller", "cwnd", "bbr-profile"} {
		if _, exists := created.ProtocolJSON[key]; !exists {
			t.Fatalf("quic 分支必须保留 %s: %+v", key, created.ProtocolJSON)
		}
	}

	// h2 分支必须清空 QUIC 调优字段。
	h2 := masqueBaseParams(t)
	h2["congestion-controller"] = "bbr_meta_v2"
	h2["cwnd"] = 64
	updated, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "masque", Host: "example.com", Port: 443, BaseRevision: created.EditRevision,
		ProtocolJSON: h2, CurrentState: masqueState("h2"),
	})
	if err != nil {
		t.Fatalf("h2 更新失败: %v", err)
	}
	for _, key := range []string{"congestion-controller", "cwnd", "bbr-profile"} {
		if _, exists := updated.ProtocolJSON[key]; exists {
			t.Fatalf("h2 分支必须清空 %s: %+v", key, updated.ProtocolJSON)
		}
	}
}

// TestMASQUECongestionControllerEnum 覆盖拥塞控制器按固定 tag 枚举校验。
func TestMASQUECongestionControllerEnum(t *testing.T) {
	field := masqueField(t, "congestion-controller")
	if strings.Join(field.Options, ",") != ",cubic,new_reno,bbr_meta_v1,bbr_meta_v2,bbr" {
		t.Fatalf("congestion-controller 枚举必须与固定 tag 一致: %+v", field.Options)
	}
	svc, _, _ := newTestService(t)
	state := masqueState("quic")
	params := masqueBaseParams(t)
	params["congestion-controller"] = "vegas"
	if _, err := createMASQUE(t, svc, "masque-cc-bad", params, state); err == nil || !strings.Contains(err.Error(), "congestion-controller") {
		t.Fatalf("非法拥塞控制器必须按字段拒绝，实际: %v", err)
	}
}

// TestMASQUEKeysRequireECStructure 覆盖两类密钥按固定 tag 的 EC 结构校验。
func TestMASQUEKeysRequireECStructure(t *testing.T) {
	svc, _, _ := newTestService(t)
	state := masqueState("quic")

	// 合法 EC 密钥对通过。
	if _, err := createMASQUE(t, svc, "masque-keys-ok", masqueBaseParams(t), state); err != nil {
		t.Fatalf("合法 EC 密钥对应通过: %v", err)
	}

	// 仅 Base64 可解码但结构非法：随机 32 字节不构成 SEC1 EC 私钥，也不构成 PKIX 公钥。
	random := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	for _, tc := range []struct {
		name string
		key  string
		want string
	}{
		{name: "private-key not SEC1 EC", key: "private-key", want: "private-key"},
		{name: "public-key not PKIX ECDSA", key: "public-key", want: "public-key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := masqueBaseParams(t)
			params[tc.key] = random
			if _, err := createMASQUE(t, svc, "masque-key-"+tc.key, params, state); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%s 结构非法必须被拒绝，实际: %v", tc.key, err)
			}
		})
	}

	// 非 Base64 也必须被拒绝。
	params := masqueBaseParams(t)
	params["private-key"] = "not-base64!!"
	if _, err := createMASQUE(t, svc, "masque-key-b64", params, state); err == nil || !strings.Contains(err.Error(), "private-key") {
		t.Fatalf("非 Base64 私钥必须被拒绝，实际: %v", err)
	}

	// RSA 公钥是合法 PKIX 但不是 ECDSA，必须被拒绝。
	rsaPublic := masqueRSAPublicKeyBase64(t)
	params = masqueBaseParams(t)
	params["public-key"] = rsaPublic
	if _, err := createMASQUE(t, svc, "masque-key-rsa", params, state); err == nil || !strings.Contains(err.Error(), "public-key") {
		t.Fatalf("非 ECDSA 公钥必须被拒绝，实际: %v", err)
	}
}

// TestMASQUERequiresLocalAddress 覆盖 ip／ipv6 至少一项与缺省前缀规范化。
func TestMASQUERequiresLocalAddress(t *testing.T) {
	svc, _, _ := newTestService(t)
	state := masqueState("quic")

	params := masqueBaseParams(t)
	delete(params, "ip")
	if _, err := createMASQUE(t, svc, "masque-no-address", params, state); err == nil {
		t.Fatal("ip／ipv6 均缺失必须被拒绝")
	}

	params["ipv6"] = "2001:db8::2"
	created, err := createMASQUE(t, svc, "masque-prefix-default", params, state)
	if err != nil {
		t.Fatalf("仅 ipv6 应合法: %v", err)
	}
	if created.ProtocolJSON["ipv6"] != "2001:db8::2/128" {
		t.Fatalf("缺省 IPv6 前缀应规范为 /128，实际 %v", created.ProtocolJSON["ipv6"])
	}

	params = masqueBaseParams(t)
	params["ip"] = "192.0.2.2"
	created, err = createMASQUE(t, svc, "masque-prefix-ipv4", params, state)
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if created.ProtocolJSON["ip"] != "192.0.2.2/32" {
		t.Fatalf("缺省 IPv4 前缀应规范为 /32，实际 %v", created.ProtocolJSON["ip"])
	}

	params["ip"] = "not-an-ip"
	if _, err := createMASQUE(t, svc, "masque-bad-address", params, state); err == nil {
		t.Fatal("非法 ip 必须被拒绝")
	}
}

// TestMASQUEURIValidationAndNoEcho 覆盖 uri 校验且错误不回显原值。
func TestMASQUEURIValidationAndNoEcho(t *testing.T) {
	svc, _, _ := newTestService(t)
	state := masqueState("quic")

	params := masqueBaseParams(t)
	params["uri"] = "https://user:secret@example.com/connect?token=abc#frag"
	if _, err := createMASQUE(t, svc, "masque-uri-ok", params, state); err != nil {
		t.Fatalf("合法绝对 URI 应通过: %v", err)
	}

	for _, bad := range []string{"not a url", "/relative/path", "example.com/connect"} {
		t.Run(bad, func(t *testing.T) {
			params := masqueBaseParams(t)
			params["uri"] = bad
			_, err := createMASQUE(t, svc, "masque-uri-bad", params, state)
			if err == nil {
				t.Fatalf("非法 uri %q 必须被拒绝", bad)
			}
			if !strings.Contains(err.Error(), "uri") {
				t.Fatalf("错误必须定位到 uri，实际: %v", err)
			}
			if strings.Contains(err.Error(), bad) {
				t.Fatalf("错误文本不得回显 uri 原值: %v", err)
			}
		})
	}
}

// TestMASQUENumericBounds 覆盖 mtu／handshake-timeout／cwnd 的类型与范围。
func TestMASQUENumericBounds(t *testing.T) {
	svc, _, _ := newTestService(t)
	state := masqueState("quic")
	for _, name := range []string{"mtu", "handshake-timeout", "cwnd"} {
		t.Run(name, func(t *testing.T) {
			params := masqueBaseParams(t)
			params[name] = -1
			if _, err := createMASQUE(t, svc, "masque-neg-"+name, params, state); err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("%s 负数必须按字段拒绝，实际: %v", name, err)
			}
			params[name] = 1.5
			if _, err := createMASQUE(t, svc, "masque-frac-"+name, params, state); err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("%s 小数必须按字段拒绝，实际: %v", name, err)
			}
			params[name] = 0
			if _, err := createMASQUE(t, svc, "masque-zero-"+name, params, state); err != nil {
				t.Fatalf("%s=0 应合法: %v", name, err)
			}
		})
	}
}

// TestMASQUENameCertVerifyExcluded 覆盖 name-cert-verify 不进入 schema 与 wire。
func TestMASQUENameCertVerifyExcluded(t *testing.T) {
	for _, field := range masqueProto(t).FormSchema {
		if field.Name == "name-cert-verify" {
			t.Fatal("MASQUE 的 name-cert-verify 在固定 tag 中只是 placeholder，不得进入 schema")
		}
	}
}

// TestMASQUEIPStackReusesSharedEnums 覆盖 ip-stack 复用 WireGuard 枚举合同。
func TestMASQUEIPStackReusesSharedEnums(t *testing.T) {
	stack := masqueField(t, "ip-stack")
	if stack.Type != "object" || stack.ObjectKind != "fields" {
		t.Fatalf("ip-stack 必须是固定对象: %+v", stack)
	}
	props := map[string]FieldSchema{}
	for _, property := range stack.Properties {
		props[property.Name] = property
	}
	if strings.Join(props["mode"].Options, ",") != ",auto,gvisor,mips" {
		t.Fatalf("ip-stack.mode 枚举必须与 WireGuard 一致: %+v", props["mode"].Options)
	}
	if strings.Join(props["congestion-controller"].Options, ",") != ",cubic,reno,bbr,bbr3" {
		t.Fatalf("ip-stack.congestion-controller 枚举必须与 WireGuard 一致: %+v", props["congestion-controller"].Options)
	}
	svc, _, _ := newTestService(t)
	state := masqueState("quic")
	params := masqueBaseParams(t)
	params["ip-stack"] = map[string]any{"mode": "gvisor", "congestion-controller": "bbr"}
	if _, err := createMASQUE(t, svc, "masque-ipstack-ok", params, state); err != nil {
		t.Fatalf("合法 ip-stack 应通过: %v", err)
	}
	params["ip-stack"] = map[string]any{"mode": "system"}
	if _, err := createMASQUE(t, svc, "masque-ipstack-bad", params, state); err == nil {
		t.Fatal("非法 ip-stack.mode 必须被拒绝")
	}
}

// TestMASQUERemoteDNSResolveCondition 覆盖 dns 只在远端解析开启时活动且条件必填。
func TestMASQUERemoteDNSResolveCondition(t *testing.T) {
	svc, _, _ := newTestService(t)

	params := masqueBaseParams(t)
	params["remote-dns-resolve"] = false
	params["dns"] = []any{"1.1.1.1"}
	created, err := createMASQUE(t, svc, "masque-dns-off", params, masqueState("quic"))
	if err != nil {
		t.Fatalf("remote-dns-resolve=false 创建失败: %v", err)
	}
	if _, exists := created.ProtocolJSON["dns"]; exists {
		t.Fatalf("remote-dns-resolve=false 必须清空 dns: %+v", created.ProtocolJSON)
	}

	params["remote-dns-resolve"] = true
	delete(params, "dns")
	if _, err := createMASQUE(t, svc, "masque-dns-required", params, masqueState("quic")); err == nil {
		t.Fatal("remote-dns-resolve=true 时 dns 必填")
	}
}

// TestMASQUEStateOnlyFieldRejected 覆盖 network-mode 不得写入 protocol_json。
func TestMASQUEStateOnlyFieldRejected(t *testing.T) {
	svc, _, _ := newTestService(t)
	params := masqueBaseParams(t)
	params["network-mode"] = "h2"
	if _, err := createMASQUE(t, svc, "masque-state-only", params, nil); err == nil {
		t.Fatal("state_only 字段 network-mode 不得写入 protocol_json")
	}
}
