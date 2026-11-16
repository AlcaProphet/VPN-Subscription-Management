// openvpn_protocol_test.go：Build32 Step 18 OpenVPN 结构化协议合同测试。
// 覆盖 auth_mode 三态与 cert_userpass 组合认证、无方向 reset_on 之外的「仅清除非活动凭据」、
// tls_key_mode 三选一互斥、字段枚举／非负整数／tran-window 未设置与显式 0、peer-info 与 DNS 合同，
// 以及 client-config 编辑入口被移除。
package node

import (
	"context"
	"strings"
	"testing"
)

func openvpnProto(t *testing.T) Protocol {
	t.Helper()
	proto, err := GetProtocol("openvpn")
	if err != nil {
		t.Fatal(err)
	}
	return proto
}

func openvpnFieldMust(t *testing.T, name string) FieldSchema {
	t.Helper()
	field, ok := findSchemaField(openvpnProto(t).FormSchema, name)
	if !ok {
		t.Fatalf("OpenVPN schema 缺少字段 %s", name)
	}
	return field
}

// openvpnBaseParams 返回满足 CA 必填的最小参数集（auth_mode=userpass）。
func openvpnBaseParams() map[string]any {
	return map[string]any{
		"ca":       "-----BEGIN CERTIFICATE-----\nTUlJQmZUA==\n-----END CERTIFICATE-----",
		"username": "ovpn-user",
		"password": "ovpn-password",
	}
}

func createOpenVPN(t *testing.T, svc *Service, name string, params map[string]any, state *CurrentState) (*Node, error) {
	t.Helper()
	return svc.CreateManual(context.Background(), CreateManualInput{
		Name: name, Protocol: "openvpn", Host: "vpn.example.com", Port: 1194,
		ProtocolJSON: params, CurrentState: state,
	})
}

func openvpnState(authMode, tlsKeyMode string) *CurrentState {
	return &CurrentState{Selectors: map[string]string{"auth_mode": authMode, "tls_key_mode": tlsKeyMode}}
}

// TestOpenVPNRemovedClientConfig 覆盖原始 client-config 入口已从编辑 schema 移除且不可再写入。
func TestOpenVPNRemovedClientConfig(t *testing.T) {
	if _, ok := findSchemaField(openvpnProto(t).FormSchema, "client-config"); ok {
		t.Fatal("OpenVPN 结构化 schema 不得保留 client-config 入口")
	}
	svc, _, _ := newTestService(t)
	params := openvpnBaseParams()
	params["client-config"] = "remote vpn.example.com 1194\nclient\n"
	if _, err := createOpenVPN(t, svc, "ovpn-legacy-client-config", params, nil); err == nil {
		t.Fatal("client-config 必须作为未知顶层字段被拒绝，不得静默丢弃既有数据")
	}
}

// TestOpenVPNAuthModeSelector 覆盖 auth_mode／tls_key_mode selector 声明与认证字段清空语义。
func TestOpenVPNAuthModeSelector(t *testing.T) {
	proto := openvpnProto(t)
	declared := map[string]SelectorSchema{}
	for _, selector := range proto.Selectors {
		declared[selector.Name] = selector
	}
	auth, ok := declared["auth_mode"]
	if !ok || strings.Join(auth.Values, ",") != "userpass,cert,cert_userpass" {
		t.Fatalf("OpenVPN auth_mode selector 合同不符: %+v", auth)
	}
	if auth.Default != "userpass" {
		t.Fatalf("OpenVPN auth_mode 默认值应为 userpass: %+v", auth)
	}
	tlsKey, ok := declared["tls_key_mode"]
	if !ok || strings.Join(tlsKey.Values, ",") != "none,tls_auth,tls_crypt,tls_crypt_v2" || tlsKey.Default != "none" {
		t.Fatalf("OpenVPN tls_key_mode selector 合同不符: %+v", tlsKey)
	}
	for _, name := range []string{"auth-mode", "tls-key-mode"} {
		field := openvpnFieldMust(t, name)
		if !field.StateOnly {
			t.Fatalf("OpenVPN %s 必须是 state_only: %+v", name, field)
		}
	}
	// 多分支共享字段只声明 clear_when_inactive，不声明无方向 reset_on，避免误清仍活动的凭据。
	for _, name := range []string{"username", "password", "cert", "key"} {
		field := openvpnFieldMust(t, name)
		if field.When == nil || len(field.When.Selectors["auth_mode"]) == 0 {
			t.Fatalf("OpenVPN %s 必须按 auth_mode 分支活动: %+v", name, field)
		}
		if !field.ClearWhenInactive {
			t.Fatalf("OpenVPN %s 必须声明 clear_when_inactive: %+v", name, field)
		}
		if field.ShouldReset("selector.auth_mode") {
			t.Fatalf("OpenVPN %s 不得声明无方向 reset_on，否则会清掉仍活动的凭据: %+v", name, field)
		}
	}
	// userpass 与 cert_userpass 需要用户名密码；cert 与 cert_userpass 需要证书私钥。
	for _, name := range []string{"username", "password"} {
		field := openvpnFieldMust(t, name)
		if field.RequiredWhen == nil || len(field.RequiredWhen.Selectors["auth_mode"]) != 2 {
			t.Fatalf("OpenVPN %s 必须在 userpass／cert_userpass 分支条件必填: %+v", name, field)
		}
	}
	for _, name := range []string{"cert", "key"} {
		field := openvpnFieldMust(t, name)
		if field.RequiredWhen == nil || len(field.RequiredWhen.Selectors["auth_mode"]) != 2 {
			t.Fatalf("OpenVPN %s 必须在 cert／cert_userpass 分支条件必填: %+v", name, field)
		}
	}
	if openvpnFieldMust(t, "cert").Type != "multiline" || openvpnFieldMust(t, "ca").Type != "multiline" {
		t.Fatal("OpenVPN cert／ca 必须是 multiline")
	}
	if openvpnFieldMust(t, "key").Type != "secret-multiline" {
		t.Fatal("OpenVPN key 必须是 secret-multiline")
	}
	for _, path := range []string{"password", "key", "tls-auth", "tls-crypt", "tls-crypt-v2"} {
		if !contains(proto.SensitiveFields, path) {
			t.Fatalf("OpenVPN 缺少敏感路径 %s: %v", path, proto.SensitiveFields)
		}
	}
	for _, path := range []string{"ca", "cert"} {
		if contains(proto.SensitiveFields, path) {
			t.Fatalf("OpenVPN %s 不应是敏感字段（Build32 要求 multiline 非 secret）: %v", path, proto.SensitiveFields)
		}
	}
}

// TestOpenVPNAuthCombinations 覆盖三种合法认证模式与缺半／全空反例。
func TestOpenVPNAuthCombinations(t *testing.T) {
	ca := openvpnBaseParams()["ca"]
	cert := "-----BEGIN CERTIFICATE-----\nQ0VSVA==\n-----END CERTIFICATE-----"
	key := "-----BEGIN PRIVATE KEY-----\nS0VZ\n-----END PRIVATE KEY-----"
	svc, _, _ := newTestService(t)

	legal := []struct {
		name   string
		mode   string
		params map[string]any
	}{
		{name: "userpass", mode: "userpass", params: map[string]any{"ca": ca, "username": "u", "password": "p"}},
		{name: "cert", mode: "cert", params: map[string]any{"ca": ca, "cert": cert, "key": key}},
		{name: "cert_userpass", mode: "cert_userpass", params: map[string]any{"ca": ca, "cert": cert, "key": key, "username": "u", "password": "p"}},
	}
	for _, tc := range legal {
		t.Run("legal-"+tc.name, func(t *testing.T) {
			if _, err := createOpenVPN(t, svc, "ovpn-legal-"+tc.name, tc.params, openvpnState(tc.mode, "none")); err != nil {
				t.Fatalf("合法认证模式 %s 必须通过: %v", tc.name, err)
			}
		})
	}

	invalid := []struct {
		name   string
		mode   string
		params map[string]any
		want   string
	}{
		{name: "userpass-missing-password", mode: "userpass", params: map[string]any{"ca": ca, "username": "u"}, want: "password"},
		{name: "userpass-missing-username", mode: "userpass", params: map[string]any{"ca": ca, "password": "p"}, want: "username"},
		{name: "cert-missing-key", mode: "cert", params: map[string]any{"ca": ca, "cert": cert}, want: "key"},
		{name: "cert-missing-cert", mode: "cert", params: map[string]any{"ca": ca, "key": key}, want: "cert"},
		{name: "cert_userpass-missing-password", mode: "cert_userpass", params: map[string]any{"ca": ca, "cert": cert, "key": key, "username": "u"}, want: "password"},
		{name: "cert_userpass-missing-key", mode: "cert_userpass", params: map[string]any{"ca": ca, "cert": cert, "username": "u", "password": "p"}, want: "key"},
		{name: "both-empty", mode: "userpass", params: map[string]any{"ca": ca}, want: "username"},
	}
	for _, tc := range invalid {
		t.Run("invalid-"+tc.name, func(t *testing.T) {
			_, err := createOpenVPN(t, svc, "ovpn-invalid-"+tc.name, tc.params, openvpnState(tc.mode, "none"))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%s 必须按字段 %s 拒绝，实际: %v", tc.name, tc.want, err)
			}
		})
	}
	// CA 必填。
	if _, err := createOpenVPN(t, svc, "ovpn-missing-ca",
		map[string]any{"username": "u", "password": "p"}, openvpnState("userpass", "none")); err == nil || !strings.Contains(err.Error(), "ca") {
		t.Fatalf("缺少 CA 必须按字段拒绝，实际: %v", err)
	}
}

// TestOpenVPNAuthBranchDirectionalClearing 覆盖「只清除新分支不再活动的凭据」与 A→B→A 不恢复。
func TestOpenVPNAuthBranchDirectionalClearing(t *testing.T) {
	ca := openvpnBaseParams()["ca"]
	cert := "-----BEGIN CERTIFICATE-----\nQ0VSVA==\n-----END CERTIFICATE-----"
	key := "-----BEGIN PRIVATE KEY-----\nS0VZ\n-----END PRIVATE KEY-----"
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	created, err := createOpenVPN(t, svc, "ovpn-direction", map[string]any{
		"ca": ca, "cert": cert, "key": key, "username": "u", "password": "p",
	}, openvpnState("cert_userpass", "none"))
	if err != nil {
		t.Fatalf("创建组合认证节点失败: %v", err)
	}
	if _, err := svc.Get(ctx, created.ID); err != nil {
		t.Fatalf("读取节点失败: %v", err)
	}

	// cert_userpass → cert：只清除 username／password，证书与私钥必须保留。
	updated, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Name: created.Name, Protocol: "openvpn", Host: created.Host, Port: created.Port,
		ProtocolJSON: map[string]any{"ca": ca, "cert": cert},
		CurrentState: openvpnState("cert", "none"), BaseRevision: created.EditRevision,
	})
	if err != nil {
		t.Fatalf("切换到 cert 失败: %v", err)
	}
	for _, gone := range []string{"username", "password"} {
		if _, exists := updated.ProtocolJSON[gone]; exists {
			t.Fatalf("cert 分支必须清除 %s: %+v", gone, updated.ProtocolJSON)
		}
	}
	if updated.ProtocolJSON["cert"] != cert {
		t.Fatalf("cert 分支必须保留证书: %+v", updated.ProtocolJSON)
	}
	// 私钥以密文保留：读取时按敏感字段脱敏，但派生状态仍必须是 cert。
	stored, err := svc.Get(ctx, updated.ID)
	if err != nil {
		t.Fatalf("重新读取失败: %v", err)
	}
	if len(stored.SavedSensitivePaths) == 0 {
		t.Fatalf("私钥必须在切换后仍以密文保留: %+v", stored.SavedSensitivePaths)
	}

	// cert → userpass：只清除 cert／key，username／password 重新填写。
	userpass, err := svc.UpdateManual(ctx, updated.ID, UpdateManualInput{
		Name: updated.Name, Protocol: "openvpn", Host: updated.Host, Port: updated.Port,
		ProtocolJSON: map[string]any{"ca": ca, "username": "u2", "password": "p2"},
		CurrentState: openvpnState("userpass", "none"), BaseRevision: updated.EditRevision,
	})
	if err != nil {
		t.Fatalf("切换到 userpass 失败: %v", err)
	}
	for _, gone := range []string{"cert", "key"} {
		if _, exists := userpass.ProtocolJSON[gone]; exists {
			t.Fatalf("userpass 分支必须清除 %s: %+v", gone, userpass.ProtocolJSON)
		}
	}
	if userpass.ProtocolJSON["username"] != "u2" {
		t.Fatalf("userpass 分支必须保留本次填写的用户名: %+v", userpass.ProtocolJSON)
	}

	// A→B→A：切回 cert 且不重填私钥必须阻断，不得恢复已清空的凭据。
	if _, err := svc.UpdateManual(ctx, userpass.ID, UpdateManualInput{
		Name: userpass.Name, Protocol: "openvpn", Host: userpass.Host, Port: userpass.Port,
		ProtocolJSON: map[string]any{"ca": ca, "cert": cert},
		CurrentState: openvpnState("cert", "none"), BaseRevision: userpass.EditRevision,
	}); err == nil || !strings.Contains(err.Error(), "key") {
		t.Fatalf("A→B→A 不得恢复已清空的私钥，实际: %v", err)
	}

	// userpass → cert_userpass：仍活动的 username／password 必须保留，不得被清空。
	combined, err := svc.UpdateManual(ctx, userpass.ID, UpdateManualInput{
		Name: userpass.Name, Protocol: "openvpn", Host: userpass.Host, Port: userpass.Port,
		ProtocolJSON: map[string]any{"ca": ca, "cert": cert, "key": key},
		CurrentState: openvpnState("cert_userpass", "none"), BaseRevision: userpass.EditRevision,
	})
	if err != nil {
		t.Fatalf("切换到 cert_userpass 失败: %v", err)
	}
	if combined.ProtocolJSON["username"] != "u2" {
		t.Fatalf("userpass→cert_userpass 不得清除仍活动的用户名: %+v", combined.ProtocolJSON)
	}
	if combined.ProtocolJSON["cert"] != cert {
		t.Fatalf("userpass→cert_userpass 必须保留新填写的证书: %+v", combined.ProtocolJSON)
	}
}

// TestOpenVPNTLSKeyModes 覆盖 tls_key_mode 三选一互斥与 key-direction 边界。
func TestOpenVPNTLSKeyModes(t *testing.T) {
	ca := openvpnBaseParams()["ca"]
	tlsAuth := strings.Repeat("ab", 256)
	tlsCrypt := strings.Repeat("cd", 256)
	svc, _, _ := newTestService(t)

	legal := []struct {
		name   string
		mode   string
		params map[string]any
	}{
		{name: "none", mode: "none", params: map[string]any{"ca": ca, "username": "u", "password": "p"}},
		{name: "tls_auth", mode: "tls_auth", params: map[string]any{"ca": ca, "username": "u", "password": "p", "tls-auth": tlsAuth, "key-direction": "1"}},
		{name: "tls_crypt", mode: "tls_crypt", params: map[string]any{"ca": ca, "username": "u", "password": "p", "tls-crypt": tlsCrypt}},
		{name: "tls_crypt_v2", mode: "tls_crypt_v2", params: map[string]any{"ca": ca, "username": "u", "password": "p", "tls-crypt-v2": tlsCrypt}},
	}
	for _, tc := range legal {
		t.Run("legal-"+tc.name, func(t *testing.T) {
			if _, err := createOpenVPN(t, svc, "ovpn-key-"+tc.name, tc.params, openvpnState("userpass", tc.mode)); err != nil {
				t.Fatalf("合法 tls_key_mode %s 必须通过: %v", tc.name, err)
			}
		})
	}

	// tls_key_mode 声明的 key 缺失必须条件必填。
	if _, err := createOpenVPN(t, svc, "ovpn-tls-auth-missing",
		map[string]any{"ca": ca, "username": "u", "password": "p"}, openvpnState("userpass", "tls_auth")); err == nil || !strings.Contains(err.Error(), "tls-auth") {
		t.Fatalf("tls_auth 分支缺 tls-auth 必须按字段拒绝，实际: %v", err)
	}
	// 三种 key 必须互斥：显式声明 tls_auth 时 tls-crypt 不得落库。
	created, err := createOpenVPN(t, svc, "ovpn-key-exclusive", map[string]any{
		"ca": ca, "username": "u", "password": "p", "tls-auth": tlsAuth, "tls-crypt": tlsCrypt,
	}, openvpnState("userpass", "tls_auth"))
	if err != nil {
		t.Fatalf("互斥提交应被规范化而不是报错: %v", err)
	}
	if _, exists := created.ProtocolJSON["tls-crypt"]; exists {
		t.Fatalf("tls_auth 分支不得同时保存 tls-crypt: %+v", created.ProtocolJSON)
	}
	// key-direction 只在 tls_auth 分支活动，且只允许 0／1／空。
	field := openvpnFieldMust(t, "key-direction")
	if field.When == nil || len(field.When.Selectors["tls_key_mode"]) != 1 || field.When.Selectors["tls_key_mode"][0] != "tls_auth" {
		t.Fatalf("key-direction 必须只在 tls_auth 分支活动: %+v", field)
	}
	if !field.ClearWhenInactive {
		t.Fatalf("key-direction 必须声明 clear_when_inactive: %+v", field)
	}
	if got := strings.Join(field.Options, ","); got != ",0,1" {
		t.Fatalf("key-direction 允许值必须是 0／1／空: %s", got)
	}
	if _, err := createOpenVPN(t, svc, "ovpn-key-direction-bad", map[string]any{
		"ca": ca, "username": "u", "password": "p", "tls-auth": tlsAuth, "key-direction": "2",
	}, openvpnState("userpass", "tls_auth")); err == nil {
		t.Fatal("key-direction=2 必须被拒绝")
	}
}

// TestOpenVPNFieldContracts 覆盖 ca／proto／dev、枚举、非负整数、tran-window 与 DNS 合同。
func TestOpenVPNFieldContracts(t *testing.T) {
	svc, _, _ := newTestService(t)

	if openvpnFieldMust(t, "dev").Default != "tun" || strings.Join(openvpnFieldMust(t, "dev").Options, ",") != "tun" {
		t.Fatalf("dev 必须固定为 tun: %+v", openvpnFieldMust(t, "dev"))
	}
	if got := strings.Join(openvpnFieldMust(t, "proto").Options, ","); got != "udp,tcp" {
		t.Fatalf("proto 只允许 udp／tcp: %s", got)
	}
	if got := strings.Join(openvpnFieldMust(t, "cipher").Options, ","); got != "AES-128-GCM,AES-192-GCM,AES-256-GCM,AES-128-CBC,AES-192-CBC,AES-256-CBC,CHACHA20-POLY1305" {
		t.Fatalf("cipher 枚举必须与固定 tag 一致: %s", got)
	}
	if got := strings.Join(openvpnFieldMust(t, "auth").Options, ","); got != "MD5,SHA1,SHA256,SHA384,SHA512" {
		t.Fatalf("auth 枚举必须与固定 tag 一致: %s", got)
	}
	if got := strings.Join(openvpnFieldMust(t, "comp-lzo").Options, ","); got != ",yes,no,adaptive" {
		t.Fatalf("comp-lzo 必须按项目侧收紧为 yes／no／adaptive／空: %s", got)
	}

	// 非法枚举与非负数必须按字段拒绝。
	for _, tc := range []struct{ key, value string }{
		{"proto", "sctp"},
		{"dev", "tap"},
		{"cipher", "AES-256-CFB"},
		{"auth", "SHA3-256"},
		{"comp-lzo", "maybe"},
	} {
		t.Run("invalid-"+tc.key, func(t *testing.T) {
			params := openvpnBaseParams()
			params[tc.key] = tc.value
			if _, err := createOpenVPN(t, svc, "ovpn-enum-"+tc.key, params, openvpnState("userpass", "none")); err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("%s=%s 必须按字段拒绝，实际: %v", tc.key, tc.value, err)
			}
		})
	}
	for _, name := range []string{"ping", "ping-restart", "handshake-timeout", "mtu", "tran-window"} {
		t.Run("negative-"+name, func(t *testing.T) {
			params := openvpnBaseParams()
			params[name] = -1
			if _, err := createOpenVPN(t, svc, "ovpn-neg-"+name, params, openvpnState("userpass", "none")); err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("%s 负数必须按字段拒绝，实际: %v", name, err)
			}
		})
	}

	// data-ciphers 列表项必须落在固定 tag 的 cipher 集合内。
	params := openvpnBaseParams()
	params["data-ciphers"] = []any{"AES-256-GCM", " bogus "}
	if _, err := createOpenVPN(t, svc, "ovpn-data-ciphers-bad", params, openvpnState("userpass", "none")); err == nil || !strings.Contains(err.Error(), "data-ciphers") {
		t.Fatalf("非法 data-ciphers 项必须按字段拒绝，实际: %v", err)
	}
	params = openvpnBaseParams()
	params["data-ciphers"] = []any{"AES-256-GCM", "CHACHA20-POLY1305", "AES-256-GCM"}
	params["data-ciphers-fallback"] = "AES-128-CBC"
	created, err := createOpenVPN(t, svc, "ovpn-data-ciphers-ok", params, openvpnState("userpass", "none"))
	if err != nil {
		t.Fatalf("合法 data-ciphers 必须通过: %v", err)
	}
	if got, ok := stringListItems(created.ProtocolJSON["data-ciphers"]); !ok || strings.Join(got, ",") != "AES-256-GCM,CHACHA20-POLY1305" {
		t.Fatalf("data-ciphers 必须去重保序: %+v", created.ProtocolJSON["data-ciphers"])
	}

	// tran-window：未设置与显式 0 必须可区分，且只有显式值才落库。
	noWindow, err := createOpenVPN(t, svc, "ovpn-window-unset", openvpnBaseParams(), openvpnState("userpass", "none"))
	if err != nil {
		t.Fatalf("未设置 tran-window 必须通过: %v", err)
	}
	if _, exists := noWindow.ProtocolJSON["tran-window"]; exists {
		t.Fatalf("未设置的 tran-window 不得落库: %+v", noWindow.ProtocolJSON)
	}
	zeroWindow, err := createOpenVPN(t, svc, "ovpn-window-zero", func() map[string]any {
		p := openvpnBaseParams()
		p["tran-window"] = 0
		return p
	}(), openvpnState("userpass", "none"))
	if err != nil {
		t.Fatalf("显式 0 必须通过: %v", err)
	}
	if value, exists := numberParam(zeroWindow.ProtocolJSON["tran-window"]); !exists || value != 0 {
		t.Fatalf("显式 tran-window=0 必须保留: %+v", zeroWindow.ProtocolJSON)
	}

	// remote-dns-resolve 关闭清空 DNS；开启时 DNS 必填。
	off, err := createOpenVPN(t, svc, "ovpn-dns-off", func() map[string]any {
		p := openvpnBaseParams()
		p["remote-dns-resolve"] = false
		p["dns"] = []any{"1.1.1.1"}
		return p
	}(), openvpnState("userpass", "none"))
	if err != nil {
		t.Fatalf("关闭远端 DNS 必须通过: %v", err)
	}
	if _, exists := off.ProtocolJSON["dns"]; exists {
		t.Fatalf("remote-dns-resolve=false 必须清空 DNS: %+v", off.ProtocolJSON)
	}
	// 显式提交状态时必须与派生功能一致，因此这里同步声明 remote-dns-resolve 功能。
	dnsOnState := &CurrentState{
		Selectors: map[string]string{"auth_mode": "userpass", "tls_key_mode": "none"},
		Features:  []string{"remote-dns-resolve"},
	}
	if _, err := createOpenVPN(t, svc, "ovpn-dns-missing", func() map[string]any {
		p := openvpnBaseParams()
		p["remote-dns-resolve"] = true
		return p
	}(), dnsOnState); err == nil || !strings.Contains(err.Error(), "dns") {
		t.Fatalf("开启远端 DNS 而缺少 dns 必须按字段拒绝，实际: %v", err)
	}
	if _, err := createOpenVPN(t, svc, "ovpn-dns-on", func() map[string]any {
		p := openvpnBaseParams()
		p["remote-dns-resolve"] = true
		p["dns"] = []any{"1.1.1.1", "8.8.8.8"}
		return p
	}(), dnsOnState); err != nil {
		t.Fatalf("开启远端 DNS 并提供 dns 必须通过: %v", err)
	}

	// peer-info 是普通字符串 Map，键不得为空。
	if openvpnFieldMust(t, "peer-info").ObjectKind != "map" {
		t.Fatalf("peer-info 必须是 map: %+v", openvpnFieldMust(t, "peer-info"))
	}
	if _, err := createOpenVPN(t, svc, "ovpn-peer-info-bad", func() map[string]any {
		p := openvpnBaseParams()
		p["peer-info"] = map[string]any{"": "v"}
		return p
	}(), openvpnState("userpass", "none")); err == nil || !strings.Contains(err.Error(), "peer-info") {
		t.Fatalf("空 peer-info 键必须按字段拒绝，实际: %v", err)
	}

	// 未设置的字段不得强写默认值。
	plain, err := createOpenVPN(t, svc, "ovpn-defaults", openvpnBaseParams(), openvpnState("userpass", "none"))
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	for _, key := range []string{"tls-auth", "tls-crypt", "tls-crypt-v2", "key-direction", "cert", "key",
		"data-ciphers", "data-ciphers-fallback", "comp-lzo", "ping", "ping-restart", "handshake-timeout",
		"mtu", "tran-window", "peer-info", "ip-stack", "dns"} {
		if _, exists := plain.ProtocolJSON[key]; exists {
			t.Fatalf("未设置的 %s 不得写入数据库: %+v", key, plain.ProtocolJSON)
		}
	}
}

// TestOpenVPNNoURIMapping 覆盖无 URI 映射协议不伪造 URI。
func TestOpenVPNNoURIMapping(t *testing.T) {
	proto := openvpnProto(t)
	if proto.LinkMappings.SR || proto.LinkMappings.Generic {
		t.Fatalf("OpenVPN 不得声明 URI 映射: %+v", proto.LinkMappings)
	}
}
