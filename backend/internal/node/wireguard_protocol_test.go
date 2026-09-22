// wireguard_protocol_test.go：Build32 Step 11 标准 WireGuard 协议合同测试。
// 覆盖 peer_mode selector、single／peers endpoint policy、字段互斥与切换清空、
// Peer 稳定身份与 PSK 生命周期、密钥／地址／reserved／allowed-ips／ip-stack／DNS 校验，
// 以及 AmneziaWG 明确排除边界。
package node

import (
	"context"
	"strings"
	"testing"
)

// WireGuard 测试用固定 Base64 凭据：均为 32 字节合法 curve25519 形状（内容不参与校验）。
const (
	wgPrivateKey = "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE="
	wgPublicKey  = "AgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgI="
	wgPublicKey2 = "AwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwM="
	wgPSK        = "BwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwc="
	wgShortKey   = "AQEBAQEBAQEBAQEBAQEBAQ=="
)

func wireGuardProto(t *testing.T) Protocol {
	t.Helper()
	proto, err := GetProtocol("wireguard")
	if err != nil {
		t.Fatal(err)
	}
	return proto
}

func wireGuardField(t *testing.T, name string) FieldSchema {
	t.Helper()
	for _, field := range wireGuardProto(t).FormSchema {
		if field.Name == name {
			return field
		}
	}
	t.Fatalf("WireGuard schema 缺少字段 %s", name)
	return FieldSchema{}
}

// wireGuardSingleParams 是 single 模式的最小合法参数。
func wireGuardSingleParams() map[string]any {
	return map[string]any{
		"private-key": wgPrivateKey,
		"public-key":  wgPublicKey,
		"ip":          "192.0.2.2",
	}
}

// wireGuardPeerItem 构造一个最小合法 Peer 条目。
func wireGuardPeerItem(server, publicKey, allowedIP string) map[string]any {
	return map[string]any{
		"server": server, "port": 51820, "public-key": publicKey,
		"allowed-ips": []any{allowedIP},
	}
}

func wireGuardPeersParams() map[string]any {
	return map[string]any{
		"private-key": wgPrivateKey,
		"ip":          "192.0.2.2",
		"peers": []any{
			wireGuardPeerItem("peer-a.example.com", wgPublicKey, "10.0.0.0/24"),
			wireGuardPeerItem("peer-b.example.com", wgPublicKey2, "10.0.1.0/24"),
		},
	}
}

// wireGuardTextList 兼容保存后回读的 []string 与 JSON 解码后的 []any。
func wireGuardTextList(t *testing.T, value any) []string {
	t.Helper()
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				t.Fatalf("列表项不是字符串: %#v", item)
			}
			out = append(out, text)
		}
		return out
	}
	t.Fatalf("列表类型异常: %#v", value)
	return nil
}

// wireGuardIntList 兼容保存后回读的 []int 与 JSON 解码后的 []any。
func wireGuardIntList(t *testing.T, value any) []int {
	t.Helper()
	if typed, ok := value.([]int); ok {
		return typed
	}
	typed, ok := value.([]any)
	if !ok {
		t.Fatalf("整数列表类型异常: %#v", value)
	}
	out := make([]int, 0, len(typed))
	for _, item := range typed {
		number, ok := numberParam(item)
		if !ok {
			t.Fatalf("整数列表项不是数值: %#v", item)
		}
		out = append(out, int(number))
	}
	return out
}

// createWireGuard 以 single／peers 共用入口创建 WireGuard 节点；endpoint 由 policy 规范化。
func createWireGuard(t *testing.T, svc *Service, name string, params map[string]any, state *CurrentState) (*Node, error) {
	t.Helper()
	return svc.CreateManual(context.Background(), CreateManualInput{
		Name: name, Protocol: "wireguard", Host: "example.com", Port: 51820,
		ProtocolJSON: params, CurrentState: state,
	})
}

// TestWireGuardPeerModeSelectorDeclaration 覆盖 peer_mode selector 声明与 v1／缺省派生。
func TestWireGuardPeerModeSelectorDeclaration(t *testing.T) {
	proto := wireGuardProto(t)
	selector, ok := selectorSchemaByName(proto, "peer_mode")
	if !ok || selector.SourceField != "" || selector.Default != "single" {
		t.Fatalf("WireGuard 必须声明 state_only peer_mode selector: %+v", selector)
	}
	peers := wireGuardPeersParams()["peers"]
	if got := DeriveCurrentState(proto, map[string]any{"peers": peers}).Selectors["peer_mode"]; got != "peers" {
		t.Fatalf("peers 非空应派生 peers，实际 %q", got)
	}
	if got := DeriveCurrentState(proto, wireGuardSingleParams()).Selectors["peer_mode"]; got != "single" {
		t.Fatalf("无 peers 应派生 single，实际 %q", got)
	}
}

// TestWireGuardEndpointPolicies 覆盖 single／peers 两条 endpoint policy 的必填与 wire 输出。
func TestWireGuardEndpointPolicies(t *testing.T) {
	proto := wireGuardProto(t)
	if len(proto.EndpointPolicies) != 2 {
		t.Fatalf("WireGuard 必须声明 single／peers 两条 endpoint policy: %+v", proto.EndpointPolicies)
	}
	single, err := MatchEndpointPolicy(proto, CurrentState{Selectors: map[string]string{"peer_mode": "single"}})
	if err != nil {
		t.Fatal(err)
	}
	if single.HostMode != "required" || single.PortMode != "required" || !single.EmitHost || !single.EmitPort {
		t.Fatalf("single 模式必须要求并输出顶层 endpoint: %+v", single)
	}
	peers, err := MatchEndpointPolicy(proto, CurrentState{Selectors: map[string]string{"peer_mode": "peers"}})
	if err != nil {
		t.Fatal(err)
	}
	if peers.HostMode != "hidden" || peers.PortMode != "hidden" || peers.EmitHost || peers.EmitPort {
		t.Fatalf("peers 模式必须隐藏顶层 endpoint: %+v", peers)
	}

	// peers 模式下即使请求携带残值也必须规范化为 ''／0。
	host, port, _, err := NormalizeEndpoint(proto, CurrentState{Selectors: map[string]string{"peer_mode": "peers"}}, "example.com", 51820)
	if err != nil || host != "" || port != 0 {
		t.Fatalf("peers 模式 endpoint 残值未清空: host=%q port=%d err=%v", host, port, err)
	}
}

// TestWireGuardPeersModeClearsTopLevelPeerFields 覆盖 peers 模式清空顶层 Peer 字段。
func TestWireGuardPeersModeClearsTopLevelPeerFields(t *testing.T) {
	svc, _, _ := newTestService(t)
	params := wireGuardPeersParams()
	params["public-key"] = wgPublicKey
	params["pre-shared-key"] = wgPSK
	params["reserved"] = []any{1, 2, 3}
	params["allowed-ips"] = []any{"10.0.0.0/8"}
	state := CurrentState{Selectors: map[string]string{"peer_mode": "peers"}}
	created, err := createWireGuard(t, svc, "wg-peers-clears", params, &state)
	if err != nil {
		t.Fatalf("peers 模式创建失败: %v", err)
	}
	for _, key := range []string{"public-key", "pre-shared-key", "reserved", "allowed-ips"} {
		if _, exists := created.ProtocolJSON[key]; exists {
			t.Fatalf("peers 模式必须清空顶层 %s: %+v", key, created.ProtocolJSON)
		}
	}
	if created.Host != "" || created.Port != 0 {
		t.Fatalf("peers 模式必须把 endpoint 规范化为 ''/0，实际 %q/%d", created.Host, created.Port)
	}
	if _, exists := created.ProtocolJSON["peer-mode"]; exists {
		t.Fatalf("state_only 字段不得进入 protocol_json: %+v", created.ProtocolJSON)
	}
}

// TestWireGuardSingleModeClearsPeers 覆盖 single 模式清空 peers 列表。
func TestWireGuardSingleModeClearsPeers(t *testing.T) {
	svc, _, _ := newTestService(t)
	params := wireGuardSingleParams()
	params["peers"] = wireGuardPeersParams()["peers"]
	state := CurrentState{Selectors: map[string]string{"peer_mode": "single"}}
	created, err := createWireGuard(t, svc, "wg-single-clears", params, &state)
	if err != nil {
		t.Fatalf("single 模式创建失败: %v", err)
	}
	if _, exists := created.ProtocolJSON["peers"]; exists {
		t.Fatalf("single 模式必须清空 peers: %+v", created.ProtocolJSON)
	}
	if created.Host != "example.com" || created.Port != 51820 {
		t.Fatalf("single 模式必须保留顶层 endpoint，实际 %q/%d", created.Host, created.Port)
	}
}

// TestWireGuardSingleRequiresTopLevelIdentity 覆盖 single 模式顶层必填。
func TestWireGuardSingleRequiresTopLevelIdentity(t *testing.T) {
	svc, _, _ := newTestService(t)
	state := CurrentState{Selectors: map[string]string{"peer_mode": "single"}}
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
		want   string
	}{
		{name: "missing public-key", mutate: func(p map[string]any) { delete(p, "public-key") }, want: "public-key"},
		{name: "missing private-key", mutate: func(p map[string]any) { delete(p, "private-key") }, want: "private-key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := wireGuardSingleParams()
			tc.mutate(params)
			_, err := createWireGuard(t, svc, "wg-single-"+strings.ReplaceAll(tc.name, " ", "-"), params, &state)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("single 模式缺少 %s 必须被拒绝，实际: %v", tc.want, err)
			}
		})
	}
}

// TestWireGuardPeersRequiresAtLeastTwoEntries 覆盖多 Peer 至少两条。
func TestWireGuardPeersRequiresAtLeastTwoEntries(t *testing.T) {
	svc, _, _ := newTestService(t)
	state := CurrentState{Selectors: map[string]string{"peer_mode": "peers"}}

	params := wireGuardPeersParams()
	params["peers"] = []any{wireGuardPeerItem("peer-a.example.com", wgPublicKey, "10.0.0.0/24")}
	if _, err := createWireGuard(t, svc, "wg-peers-one", params, &state); err == nil || !strings.Contains(err.Error(), "peers") {
		t.Fatalf("peers 模式只有 1 条必须按 peers 字段拒绝，实际: %v", err)
	}

	empty := wireGuardPeersParams()
	delete(empty, "peers")
	if _, err := createWireGuard(t, svc, "wg-peers-empty", empty, &state); err == nil || !strings.Contains(err.Error(), "peers") {
		t.Fatalf("peers 模式缺少 peers 列表必须按 peers 字段拒绝，实际: %v", err)
	}
}

// TestWireGuardPeersRequiresPerEntryFields 覆盖每个 Peer 的必填字段与稳定身份。
func TestWireGuardPeersRequiresPerEntryFields(t *testing.T) {
	svc, _, _ := newTestService(t)
	state := CurrentState{Selectors: map[string]string{"peer_mode": "peers"}}
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
		want   string
	}{
		{name: "missing server", mutate: func(p map[string]any) { delete(p, "server") }, want: "server"},
		{name: "missing port", mutate: func(p map[string]any) { delete(p, "port") }, want: "port"},
		{name: "missing public-key", mutate: func(p map[string]any) { delete(p, "public-key") }, want: "public-key"},
		{name: "missing allowed-ips", mutate: func(p map[string]any) { delete(p, "allowed-ips") }, want: "allowed-ips"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := wireGuardPeersParams()
			peers := params["peers"].([]any)
			tc.mutate(peers[0].(map[string]any))
			_, err := createWireGuard(t, svc, "wg-peer-"+strings.ReplaceAll(tc.name, " ", "-"), params, &state)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Peer 缺少 %s 必须被拒绝，实际: %v", tc.want, err)
			}
		})
	}
}

// TestWireGuardKeysRequireBase64AndLength 覆盖 private-key／public-key／PSK 的 Base64 与 32 字节长度。
func TestWireGuardKeysRequireBase64AndLength(t *testing.T) {
	svc, _, _ := newTestService(t)
	single := CurrentState{Selectors: map[string]string{"peer_mode": "single"}}
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
		want   string
	}{
		{name: "private-key not base64", mutate: func(p map[string]any) { p["private-key"] = "not-base64!!" }, want: "private-key"},
		{name: "private-key wrong length", mutate: func(p map[string]any) { p["private-key"] = wgShortKey }, want: "private-key"},
		{name: "public-key not base64", mutate: func(p map[string]any) { p["public-key"] = "plain-text-key" }, want: "public-key"},
		{name: "public-key wrong length", mutate: func(p map[string]any) { p["public-key"] = wgShortKey }, want: "public-key"},
		{name: "pre-shared-key wrong length", mutate: func(p map[string]any) { p["pre-shared-key"] = wgShortKey }, want: "pre-shared-key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := wireGuardSingleParams()
			tc.mutate(params)
			_, err := createWireGuard(t, svc, "wg-key-"+strings.ReplaceAll(tc.name, " ", "-"), params, &single)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("非法 %s 必须被拒绝，实际: %v", tc.want, err)
			}
		})
	}

	// Peer 内 public-key／pre-shared-key 使用同一合同。
	peerState := CurrentState{Selectors: map[string]string{"peer_mode": "peers"}}
	params := wireGuardPeersParams()
	params["peers"].([]any)[0].(map[string]any)["public-key"] = wgShortKey
	if _, err := createWireGuard(t, svc, "wg-peer-key-short", params, &peerState); err == nil || !strings.Contains(err.Error(), "public-key") {
		t.Fatalf("Peer 公钥长度非法必须被拒绝，实际: %v", err)
	}
}

// TestWireGuardRequiresLocalAddress 覆盖 ip／ipv6 至少一项。
func TestWireGuardRequiresLocalAddress(t *testing.T) {
	svc, _, _ := newTestService(t)
	single := CurrentState{Selectors: map[string]string{"peer_mode": "single"}}
	params := wireGuardSingleParams()
	delete(params, "ip")
	if _, err := createWireGuard(t, svc, "wg-no-address", params, &single); err == nil {
		t.Fatal("ip／ipv6 均缺失必须被拒绝")
	}
	params["ipv6"] = "2001:db8::2"
	if _, err := createWireGuard(t, svc, "wg-only-ipv6", params, &single); err != nil {
		t.Fatalf("仅 ipv6 应合法: %v", err)
	}
	params["ip"] = "not-an-ip"
	if _, err := createWireGuard(t, svc, "wg-bad-address", params, &single); err == nil {
		t.Fatal("非法 ip 必须被拒绝")
	}
}

// TestWireGuardLocalPrefixDefaults 覆盖缺省前缀规范化为 /32、/128。
func TestWireGuardLocalPrefixDefaults(t *testing.T) {
	svc, _, _ := newTestService(t)
	single := CurrentState{Selectors: map[string]string{"peer_mode": "single"}}
	params := wireGuardSingleParams()
	params["ip"] = "192.0.2.2"
	params["ipv6"] = "2001:db8::2"
	created, err := createWireGuard(t, svc, "wg-prefix-default", params, &single)
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if created.ProtocolJSON["ip"] != "192.0.2.2/32" {
		t.Fatalf("缺省 IPv4 前缀应规范为 /32，实际 %v", created.ProtocolJSON["ip"])
	}
	if created.ProtocolJSON["ipv6"] != "2001:db8::2/128" {
		t.Fatalf("缺省 IPv6 前缀应规范为 /128，实际 %v", created.ProtocolJSON["ipv6"])
	}
	// 显式前缀不得被改写。
	params["ip"] = "192.0.2.0/24"
	params["ipv6"] = "2001:db8::/64"
	kept, err := createWireGuard(t, svc, "wg-prefix-explicit", params, &single)
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if kept.ProtocolJSON["ip"] != "192.0.2.0/24" || kept.ProtocolJSON["ipv6"] != "2001:db8::/64" {
		t.Fatalf("显式前缀不得被改写: %+v", kept.ProtocolJSON)
	}
}

// TestWireGuardReservedByteSequence 覆盖 reserved 三整数／Base64 输入统一为三整数。
func TestWireGuardReservedByteSequence(t *testing.T) {
	if got := wireGuardField(t, "reserved").Type; got != "byte-sequence" {
		t.Fatalf("顶层 reserved 必须为 byte-sequence，实际 %s", got)
	}
	svc, _, _ := newTestService(t)
	single := CurrentState{Selectors: map[string]string{"peer_mode": "single"}}

	params := wireGuardSingleParams()
	params["reserved"] = []any{1, 2, 3}
	created, err := createWireGuard(t, svc, "wg-reserved-ints", params, &single)
	if err != nil {
		t.Fatalf("三整数 reserved 创建失败: %v", err)
	}
	values := wireGuardIntList(t, created.ProtocolJSON["reserved"])
	if len(values) != 3 || values[0] != 1 || values[2] != 3 {
		t.Fatalf("reserved 必须规范化为 3 整数数组，实际 %#v", created.ProtocolJSON["reserved"])
	}

	params["reserved"] = "AQID" // Base64(0x01,0x02,0x03)
	b64, err := createWireGuard(t, svc, "wg-reserved-base64", params, &single)
	if err != nil {
		t.Fatalf("Base64 reserved 创建失败: %v", err)
	}
	decoded := wireGuardIntList(t, b64.ProtocolJSON["reserved"])
	if len(decoded) != 3 || decoded[0] != 1 || decoded[1] != 2 || decoded[2] != 3 {
		t.Fatalf("Base64 reserved 必须解码为三整数，实际 %#v", b64.ProtocolJSON["reserved"])
	}

	for _, bad := range []any{[]any{1, 2}, []any{1, 2, 3, 4}, []any{0, 0, 256}} {
		params["reserved"] = bad
		if _, err := createWireGuard(t, svc, "wg-reserved-bad", params, &single); err == nil {
			t.Fatalf("非法 reserved %#v 必须被拒绝", bad)
		}
	}

	// Peer 内 reserved 使用同一类型合同。
	if got := wireGuardField(t, "peers").Properties[4].Type; got != "byte-sequence" {
		t.Fatalf("peers[].reserved 必须为 byte-sequence，实际 %s", got)
	}
}

// TestWireGuardAllowedIPsValidation 覆盖逐项 CIDR 校验、去重与跨 Peer 网段冲突。
func TestWireGuardAllowedIPsValidation(t *testing.T) {
	svc, _, _ := newTestService(t)
	peerState := CurrentState{Selectors: map[string]string{"peer_mode": "peers"}}

	params := wireGuardPeersParams()
	peers := params["peers"].([]any)
	peers[0].(map[string]any)["allowed-ips"] = []any{"0.0.0.0/0", "0.0.0.0/0", " ::/0 "}
	created, err := createWireGuard(t, svc, "wg-allowed-dedup", params, &peerState)
	if err != nil {
		t.Fatalf("allowed-ips 去重创建失败: %v", err)
	}
	got := wireGuardTextList(t, created.ProtocolJSON["peers"].([]any)[0].(map[string]any)["allowed-ips"])
	if len(got) != 2 {
		t.Fatalf("allowed-ips 必须去空白去重且保序，实际 %#v", got)
	}

	params = wireGuardPeersParams()
	peers = params["peers"].([]any)
	peers[0].(map[string]any)["allowed-ips"] = []any{"not-a-cidr"}
	if _, err := createWireGuard(t, svc, "wg-allowed-invalid", params, &peerState); err == nil {
		t.Fatal("非法 CIDR 必须被拒绝")
	}

	params = wireGuardPeersParams()
	peers = params["peers"].([]any)
	peers[0].(map[string]any)["allowed-ips"] = []any{"10.0.0.0/24"}
	peers[1].(map[string]any)["allowed-ips"] = []any{"10.0.0.0/24"}
	if _, err := createWireGuard(t, svc, "wg-allowed-conflict", params, &peerState); err == nil {
		t.Fatal("不同 Peer 的相同网段必须被拒绝")
	}
	peers[1].(map[string]any)["allowed-ips"] = []any{"10.0.1.0/24"}
	if _, err := createWireGuard(t, svc, "wg-allowed-distinct", params, &peerState); err != nil {
		t.Fatalf("不同网段应合法: %v", err)
	}
}

// TestWireGuardIPStackEnums 覆盖 ip-stack.mode 与 congestion-controller 的固定 tag 枚举。
func TestWireGuardIPStackEnums(t *testing.T) {
	stack := wireGuardField(t, "ip-stack")
	if stack.Type != "object" || stack.ObjectKind != "fields" {
		t.Fatalf("ip-stack 必须是固定对象: %+v", stack)
	}
	props := map[string]FieldSchema{}
	for _, property := range stack.Properties {
		props[property.Name] = property
	}
	mode, ok := props["mode"]
	if !ok || strings.Join(mode.Options, ",") != ",auto,gvisor,mips" {
		t.Fatalf("ip-stack.mode 枚举必须是空值＋auto/gvisor/mips: %+v", mode)
	}
	controller, ok := props["congestion-controller"]
	if !ok || strings.Join(controller.Options, ",") != ",cubic,reno,bbr,bbr3" {
		t.Fatalf("ip-stack.congestion-controller 枚举必须是空值＋cubic/reno/bbr/bbr3: %+v", controller)
	}

	svc, _, _ := newTestService(t)
	single := CurrentState{Selectors: map[string]string{"peer_mode": "single"}}
	params := wireGuardSingleParams()
	params["ip-stack"] = map[string]any{"mode": "gvisor", "congestion-controller": "bbr3"}
	created, err := createWireGuard(t, svc, "wg-ipstack-ok", params, &single)
	if err != nil {
		t.Fatalf("合法 ip-stack 创建失败: %v", err)
	}
	if created.ProtocolJSON["ip-stack"].(map[string]any)["mode"] != "gvisor" {
		t.Fatalf("ip-stack 未按用户值保存: %+v", created.ProtocolJSON)
	}
	for _, bad := range []map[string]any{{"mode": "system"}, {"congestion-controller": "vegas"}, {"congestion-controller": "bbr_meta_v2"}} {
		params["ip-stack"] = bad
		if _, err := createWireGuard(t, svc, "wg-ipstack-bad", params, &single); err == nil {
			t.Fatalf("非法 ip-stack %+v 必须被拒绝", bad)
		}
	}
}

// TestWireGuardRemoteDNSResolveCondition 覆盖 remote-dns-resolve 与 dns 的条件关系。
func TestWireGuardRemoteDNSResolveCondition(t *testing.T) {
	if got := wireGuardField(t, "remote-dns-resolve").Type; got != "bool" {
		t.Fatalf("remote-dns-resolve 必须为 bool，实际 %s", got)
	}
	svc, _, _ := newTestService(t)
	single := CurrentState{Selectors: map[string]string{"peer_mode": "single"}}

	params := wireGuardSingleParams()
	params["remote-dns-resolve"] = false
	params["dns"] = []any{"1.1.1.1"}
	created, err := createWireGuard(t, svc, "wg-dns-off", params, &single)
	if err != nil {
		t.Fatalf("remote-dns-resolve=false 创建失败: %v", err)
	}
	if _, exists := created.ProtocolJSON["dns"]; exists {
		t.Fatalf("remote-dns-resolve=false 必须清空 dns: %+v", created.ProtocolJSON)
	}

	params["remote-dns-resolve"] = true
	delete(params, "dns")
	dnsOn := CurrentState{Selectors: map[string]string{"peer_mode": "single"}, Features: []string{"remote-dns-resolve"}}
	if _, err := createWireGuard(t, svc, "wg-dns-required", params, &dnsOn); err == nil {
		t.Fatal("remote-dns-resolve=true 时 dns 必填")
	}

	params["dns"] = []any{"1.1.1.1", "tls://dns.example.com", "system"}
	if _, err := createWireGuard(t, svc, "wg-dns-valid", params, &dnsOn); err != nil {
		t.Fatalf("合法 dns 应通过: %v", err)
	}
	for _, bad := range [][]any{{""}, {"   "}, {"ftp://dns.example.com"}, {"bad entry"}} {
		params["dns"] = bad
		if _, err := createWireGuard(t, svc, "wg-dns-bad", params, &dnsOn); err == nil {
			t.Fatalf("非法 dns %#v 必须被拒绝", bad)
		}
	}
}

// TestWireGuardIntegerBounds 覆盖 workers／mtu／persistent-keepalive／refresh interval 的非负整数约束。
func TestWireGuardIntegerBounds(t *testing.T) {
	svc, _, _ := newTestService(t)
	single := CurrentState{Selectors: map[string]string{"peer_mode": "single"}}
	for _, name := range []string{"workers", "mtu", "persistent-keepalive", "refresh-server-ip-interval"} {
		t.Run(name, func(t *testing.T) {
			params := wireGuardSingleParams()
			params[name] = -1
			if _, err := createWireGuard(t, svc, "wg-neg-"+name, params, &single); err == nil {
				t.Fatalf("%s 负数必须被拒绝", name)
			}
			params[name] = 1.5
			if _, err := createWireGuard(t, svc, "wg-frac-"+name, params, &single); err == nil {
				t.Fatalf("%s 小数必须被拒绝", name)
			}
			params[name] = 0
			if _, err := createWireGuard(t, svc, "wg-zero-"+name, params, &single); err != nil {
				t.Fatalf("%s=0 应合法: %v", name, err)
			}
		})
	}
}

// TestWireGuardStateOnlyPeerModeRejected 覆盖 peer-mode 不得写入 protocol_json。
func TestWireGuardStateOnlyPeerModeRejected(t *testing.T) {
	svc, _, _ := newTestService(t)
	params := wireGuardSingleParams()
	params["peer-mode"] = "single"
	if _, err := createWireGuard(t, svc, "wg-state-only", params, nil); err == nil {
		t.Fatal("state_only 字段 peer-mode 不得写入 protocol_json")
	}
}

// TestWireGuardPeerIdentityRejectsDuplicateIDs 覆盖重复稳定身份拒绝保存。
func TestWireGuardPeerIdentityRejectsDuplicateIDs(t *testing.T) {
	svc, _, _ := newTestService(t)
	peerState := CurrentState{Selectors: map[string]string{"peer_mode": "peers"}}
	params := wireGuardPeersParams()
	peers := params["peers"].([]any)
	peers[0].(map[string]any)[sensitiveItemIDField] = "11111111-1111-1111-1111-111111111111"
	peers[1].(map[string]any)[sensitiveItemIDField] = "11111111-1111-1111-1111-111111111111"
	if _, err := createWireGuard(t, svc, "wg-peer-dup-id", params, &peerState); err == nil || !strings.Contains(err.Error(), "稳定身份") {
		t.Fatalf("重复 Peer 稳定身份必须拒绝保存，实际: %v", err)
	}
}

// TestWireGuardNoAmneziaWGActiveFields 锁定 AmneziaWG 在 schema／白名单中不存在。
func TestWireGuardNoAmneziaWGActiveFields(t *testing.T) {
	proto := wireGuardProto(t)
	names := make([]string, 0, len(proto.FormSchema))
	for _, field := range proto.FormSchema {
		names = append(names, field.Name)
	}
	for _, name := range names {
		if strings.Contains(strings.ToLower(name), "amnezia") {
			t.Fatalf("WireGuard schema 不得包含 AmneziaWG 活动字段: %s", name)
		}
	}
	svc, _, _ := newTestService(t)
	params := wireGuardSingleParams()
	params["amnezia-wg-option"] = map[string]any{"jc": 4}
	if _, err := createWireGuard(t, svc, "wg-amnezia", params, nil); err == nil {
		t.Fatal("amnezia-wg-option 必须被顶层白名单拒绝")
	}
}
