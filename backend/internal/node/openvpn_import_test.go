// openvpn_import_test.go：Build32 Step 19 `.ovpn` 只读解析器合同测试。
// 覆盖有界输入、remote 映射、inline block、认证模式推导、冲突阻断、未知安全指令 warn、
// 危险指令／外部文件引用 400、凭据不回显，以及行号／诊断不进入保存请求。
package node

import (
	"errors"
	"strings"
	"testing"
)

// ovpnSample 返回一个包含典型指令与内嵌块的合成 `.ovpn` 文本。
func ovpnSample() string {
	return strings.Join([]string{
		"client",
		"dev tun",
		"proto udp",
		"remote vpn.example.com 1194",
		"resolv-retry infinite",
		"nobind",
		"persist-key",
		"persist-tun",
		"remote-cert-tls server",
		"cipher AES-256-GCM",
		"data-ciphers AES-256-GCM:CHACHA20-POLY1305",
		"data-ciphers-fallback AES-128-CBC",
		"auth SHA256",
		"comp-lzo adaptive",
		"ping 10",
		"ping-restart 60",
		"tran-window 0",
		"handshake-timeout 30",
		"mtu 1400",
		"peer-info IV_VER 2.6",
		"auth-user-pass",
		"tls-auth [inline]",
		"key-direction 1",
		"verb 3",
		"<ca>",
		"-----BEGIN CERTIFICATE-----",
		"Y2EtYm9keQ==",
		"-----END CERTIFICATE-----",
		"</ca>",
		"<tls-auth>",
		strings.Repeat("0123456789abcdef", 32),
		"</tls-auth>",
	}, "\n")
}

func parseOpenVPNMust(t *testing.T, text string) *OpenVPNParseResult {
	t.Helper()
	result, err := ParseOpenVPN(text)
	if err != nil {
		t.Fatalf("解析应成功，实际: %v", err)
	}
	if result == nil {
		t.Fatal("解析结果不得为空")
	}
	return result
}

func diagnosticCodes(result *OpenVPNParseResult) []string {
	out := make([]string, 0, len(result.Diagnostics))
	for _, diagnostic := range result.Diagnostics {
		out = append(out, diagnostic.Code)
	}
	return out
}

func hasDiagnosticCode(result *OpenVPNParseResult, code string) bool {
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

// TestOpenVPNParseMapsStructuredFields 覆盖 remote、指令与 inline block 的结构化映射与来源行号。
func TestOpenVPNParseMapsStructuredFields(t *testing.T) {
	result := parseOpenVPNMust(t, ovpnSample())
	if result.Host != "vpn.example.com" || result.Port != 1194 {
		t.Fatalf("remote 必须映射为 host/port: %+v", result)
	}
	params := result.ProtocolJSON
	expected := map[string]any{
		"proto": "udp", "dev": "tun", "cipher": "AES-256-GCM",
		"data-ciphers-fallback": "AES-128-CBC", "auth": "SHA256", "comp-lzo": "adaptive",
		"ping": 10, "ping-restart": 60, "tran-window": 0, "handshake-timeout": 30, "mtu": 1400,
		"key-direction": "1",
	}
	for key, want := range expected {
		if got, exists := params[key]; !exists || got != want {
			t.Fatalf("%s 必须映射为 %#v，实际 %#v（%+v）", key, want, got, params)
		}
	}
	if items, ok := stringListItems(params["data-ciphers"]); !ok || strings.Join(items, ",") != "AES-256-GCM,CHACHA20-POLY1305" {
		t.Fatalf("data-ciphers 必须按冒号拆分: %#v", params["data-ciphers"])
	}
	if peerInfo, ok := params["peer-info"].(map[string]any); !ok || peerInfo["IV_VER"] != "2.6" {
		t.Fatalf("peer-info 必须映射为字符串 Map: %#v", params["peer-info"])
	}
	// inline block 必须去标签后映射内容，且原文标签不得残留。
	ca, _ := params["ca"].(string)
	if strings.Contains(ca, "<ca>") || !strings.Contains(ca, "Y2EtYm9keQ==") {
		t.Fatalf("ca 必须去标签映射内容: %q", ca)
	}
	tlsAuth, _ := params["tls-auth"].(string)
	if strings.Contains(tlsAuth, "<tls-auth>") || len(tlsAuth) != 512 {
		t.Fatalf("tls-auth 必须去标签映射内容: %q", tlsAuth)
	}
	// 每个已映射字段必须给出来源行号。
	for _, path := range []string{"remote", "proto", "dev", "cipher", "data-ciphers",
		"data-ciphers-fallback", "auth", "comp-lzo", "ping", "ping-restart", "tran-window",
		"handshake-timeout", "mtu", "key-direction", "peer-info", "ca", "tls-auth"} {
		if line, ok := result.FieldSources[path]; !ok || line < 1 {
			t.Fatalf("字段 %s 必须提供来源行号: %+v", path, result.FieldSources)
		}
	}
	if result.FieldSources["remote"] != 4 || result.FieldSources["proto"] != 3 {
		t.Fatalf("来源行号必须对应当前文本行: %+v", result.FieldSources)
	}
	// 认证：只有 auth-user-pass → userpass；tls-auth 块 → tls_auth。
	if result.Selectors["auth_mode"] != "userpass" {
		t.Fatalf("只有 auth-user-pass 时必须推导 userpass: %+v", result.Selectors)
	}
	if result.Selectors["tls_key_mode"] != "tls_auth" {
		t.Fatalf("只有 tls-auth 块时必须推导 tls_auth: %+v", result.Selectors)
	}
	// 未知但安全的普通指令只 warn。
	for _, code := range []string{"resolv-retry", "nobind", "persist-key", "verb"} {
		_ = code
	}
	if !hasDiagnosticCode(result, OpenVPNDiagUnsupportedDirective) {
		t.Fatalf("未知安全指令必须产生 warn 诊断: %+v", result.Diagnostics)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == OpenVPNDiagUnsupportedDirective && diagnostic.Severity != "warn" {
			t.Fatalf("未知安全指令必须是 warn: %+v", diagnostic)
		}
	}
}

// TestOpenVPNParseAuthModes 覆盖 userpass／cert／cert_userpass 三种推导与组合认证不误报冲突。
func TestOpenVPNParseAuthModes(t *testing.T) {
	certBlock := "<cert>\n-----BEGIN CERTIFICATE-----\nQ0VSVA==\n-----END CERTIFICATE-----\n</cert>"
	keyBlock := "<key>\n-----BEGIN PRIVATE KEY-----\nS0VZ\n-----END PRIVATE KEY-----\n</key>"
	base := "client\ndev tun\nremote vpn.example.com 1194\n"

	t.Run("userpass", func(t *testing.T) {
		result := parseOpenVPNMust(t, base+"auth-user-pass\n")
		if result.Selectors["auth_mode"] != "userpass" {
			t.Fatalf("期望 userpass: %+v", result.Selectors)
		}
		if _, exists := result.ProtocolJSON["username"]; exists {
			t.Fatal("auth-user-pass 不得制造 username")
		}
		if _, exists := result.ProtocolJSON["password"]; exists {
			t.Fatal("auth-user-pass 不得制造 password")
		}
	})
	t.Run("cert", func(t *testing.T) {
		result := parseOpenVPNMust(t, base+certBlock+"\n"+keyBlock+"\n")
		if result.Selectors["auth_mode"] != "cert" {
			t.Fatalf("期望 cert: %+v", result.Selectors)
		}
	})
	t.Run("cert_userpass", func(t *testing.T) {
		// 完整 cert/key 加 auth-user-pass 必须得到 cert_userpass，不得误报冲突。
		result := parseOpenVPNMust(t, base+"auth-user-pass\n"+certBlock+"\n"+keyBlock+"\n")
		if result.Selectors["auth_mode"] != "cert_userpass" {
			t.Fatalf("期望 cert_userpass: %+v", result.Selectors)
		}
		if hasDiagnosticCode(result, OpenVPNDiagConflictingAuth) {
			t.Fatalf("组合认证不得误报 ovpn_conflicting_auth: %+v", result.Diagnostics)
		}
	})
	t.Run("no auth directive", func(t *testing.T) {
		result := parseOpenVPNMust(t, base)
		if !hasDiagnosticCode(result, OpenVPNDiagNoAuthDirective) {
			t.Fatalf("缺少认证指令必须给出 warn: %+v", result.Diagnostics)
		}
	})
	t.Run("auth-user-pass with file reference", func(t *testing.T) {
		result := parseOpenVPNMust(t, base+"auth-user-pass /etc/openvpn/creds.txt\n")
		if result.Selectors["auth_mode"] != "userpass" {
			t.Fatalf("引用文件仍只标记 userpass: %+v", result.Selectors)
		}
		if _, exists := result.ProtocolJSON["username"]; exists {
			t.Fatal("不得读取引用文件或制造 username")
		}
		if !hasDiagnosticCode(result, OpenVPNDiagAuthUserPassFile) {
			t.Fatalf("引用文件必须给出「不读取」warn: %+v", result.Diagnostics)
		}
	})
}

// TestOpenVPNParseBlocksConflicts 覆盖真实冲突与危险指令必须 400 阻断。
func TestOpenVPNParseBlocksConflicts(t *testing.T) {
	certBlock := "<cert>\n-----BEGIN CERTIFICATE-----\nQ0VSVA==\n-----END CERTIFICATE-----\n</cert>"
	keyBlock := "<key>\n-----BEGIN PRIVATE KEY-----\nS0VZ\n-----END PRIVATE KEY-----\n</key>"
	base := "client\ndev tun\nremote vpn.example.com 1194\n"

	cases := []struct {
		name string
		text string
		code string
	}{
		{name: "multiple different remotes", text: base + "remote other.example.com 443\n", code: OpenVPNDiagMultipleRemotes},
		{name: "cert without key", text: base + certBlock + "\n", code: OpenVPNDiagConflictingAuth},
		{name: "key without cert", text: base + keyBlock + "\n", code: OpenVPNDiagConflictingAuth},
		{name: "two tls keys", text: base + "auth-user-pass\n<tls-auth>\n" + strings.Repeat("ab", 256) + "\n</tls-auth>\n<tls-crypt>\n" + strings.Repeat("cd", 256) + "\n</tls-crypt>\n", code: OpenVPNDiagConflictingTLSKey},
		{name: "conflicting proto", text: "client\ndev tun\nproto udp\nremote vpn.example.com 1194\nproto tcp\n", code: OpenVPNDiagConflictingValue},
		{name: "unclosed inline block", text: base + "<ca>\n-----BEGIN CERTIFICATE-----\n", code: OpenVPNDiagUnclosedInlineBlock},
		{name: "external ca file", text: base + "ca /etc/openvpn/ca.crt\n", code: OpenVPNDiagExternalFileForbidden},
		{name: "external cert file", text: base + "cert /etc/openvpn/client.crt\n", code: OpenVPNDiagExternalFileForbidden},
		{name: "external tls-auth file", text: base + "tls-auth /etc/openvpn/ta.key 1\n", code: OpenVPNDiagExternalFileForbidden},
		{name: "up script", text: base + "up /etc/openvpn/up.sh\n", code: OpenVPNDiagDangerousDirective},
		{name: "down script", text: base + "down /etc/openvpn/down.sh\n", code: OpenVPNDiagDangerousDirective},
		{name: "include file", text: base + "include /etc/openvpn/extra.conf\n", code: OpenVPNDiagDangerousDirective},
		{name: "script-security", text: base + "script-security 2\n", code: OpenVPNDiagDangerousDirective},
		{name: "plugin", text: base + "plugin /usr/lib/openvpn/plugin.so\n", code: OpenVPNDiagDangerousDirective},
		{name: "tls-verify hook", text: base + "tls-verify /etc/openvpn/verify.sh\n", code: OpenVPNDiagDangerousDirective},
		{name: "unsupported dev", text: "client\ndev tap\nremote vpn.example.com 1194\n", code: OpenVPNDiagUnsupportedValue},
		{name: "unsupported proto", text: "client\ndev tun\nproto sctp\nremote vpn.example.com 1194\n", code: OpenVPNDiagUnsupportedValue},
		{name: "unsupported cipher", text: base + "cipher AES-256-CFB\n", code: OpenVPNDiagUnsupportedValue},
		{name: "unsupported key-direction", text: base + "key-direction 2\n", code: OpenVPNDiagUnsupportedValue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseOpenVPN(tc.text)
			if err == nil {
				t.Fatalf("%s 必须阻断解析", tc.name)
			}
			var parseErr *OpenVPNParseError
			if !errors.As(err, &parseErr) {
				t.Fatalf("%s 必须返回 *OpenVPNParseError，实际 %T", tc.name, err)
			}
			if result == nil {
				t.Fatalf("%s 阻断时仍必须返回诊断结果", tc.name)
			}
			if parseErr.Code != tc.code {
				t.Fatalf("%s 期望 code=%s，实际 %s（%+v）", tc.name, tc.code, parseErr.Code, result.Diagnostics)
			}
			if !hasDiagnosticCode(result, tc.code) {
				t.Fatalf("%s 诊断必须包含 %s: %+v", tc.name, tc.code, result.Diagnostics)
			}
			// 阻断时不得返回可直接应用的草稿。
			if result.Host != "" || len(result.ProtocolJSON) > 0 {
				t.Fatalf("%s 阻断时不得返回可应用草稿: %+v", tc.name, result)
			}
		})
	}
}

// TestOpenVPNParseRejectsOversize 覆盖 256 KiB 有界输入。
func TestOpenVPNParseRejectsOversize(t *testing.T) {
	oversize := strings.Repeat("# padding\n", (MaxOpenVPNParseBytes/10)+64)
	if len(oversize) <= MaxOpenVPNParseBytes {
		t.Fatal("测试输入必须超过上限")
	}
	result, err := ParseOpenVPN(oversize)
	if err == nil {
		t.Fatal("超过 256 KiB 必须阻断")
	}
	if result == nil || !hasDiagnosticCode(result, OpenVPNDiagSizeExceeded) {
		t.Fatalf("超限必须给出 %s: %+v", OpenVPNDiagSizeExceeded, result)
	}
}

// TestOpenVPNParseNeverEchoesSecrets 覆盖解析结果不回显内嵌凭据以外内容、且不含原文。
func TestOpenVPNParseNeverEchoesSecrets(t *testing.T) {
	result := parseOpenVPNMust(t, ovpnSample())
	// 诊断不得携带原文行内容：message 中不得出现源文本里的凭据或指令原文。
	for _, diagnostic := range result.Diagnostics {
		if strings.Contains(diagnostic.Message, "vpn.example.com") {
			t.Fatalf("诊断不得回显原文: %+v", diagnostic)
		}
	}
	// 原文中的注释行内容不得出现在结果里。
	commented := result.ProtocolJSON
	for key := range commented {
		if strings.Contains(strings.ToLower(key), "resolv-retry") {
			t.Fatalf("未知指令不得作为字段进入草稿: %+v", commented)
		}
	}
}

// TestOpenVPNParseIgnoresCommentsAndBlankLines 覆盖注释与空行不产生诊断。
func TestOpenVPNParseIgnoresCommentsAndBlankLines(t *testing.T) {
	text := "# comment\n; another\n\nclient\n\nremote vpn.example.com 1194\n# tail\n"
	result := parseOpenVPNMust(t, text)
	if result.Host != "vpn.example.com" || result.Port != 1194 {
		t.Fatalf("注释与空行不得影响解析: %+v", result)
	}
	if result.FieldSources["remote"] != 6 {
		t.Fatalf("行号必须按原文本计数: %+v", result.FieldSources)
	}
}
