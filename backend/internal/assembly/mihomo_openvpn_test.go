package assembly

import (
	"encoding/pem"
	"os"
	"os/exec"
	"strings"
	"testing"

	gyaml "github.com/goccy/go-yaml"
)

// openvpnTestPEM 生成固定内核可解析的合成 PEM 块（只在配置构造期做 pem.Decode，不校验签名）。
func openvpnTestPEM(pemType string, size int) string {
	body := make([]byte, size)
	for i := range body {
		body[i] = byte(i % 251)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: pemType, Bytes: body}))
}

// openvpnStaticKey 生成固定 tag DecodeStaticKey 需要的 256 字节（512 个十六进制字符）静态密钥。
func openvpnStaticKey() string {
	return strings.Repeat("0123456789abcdef", 32)
}

// TestMihomo11931OpenVPNProtocolStructures 覆盖 OpenVPN adapter 在固定 Mihomo v1.19.31
// 上的代表性正反例；普通 go test 未设置外部二进制时跳过，严格入口由 .mihomo-test.sh 负责。
func TestMihomo11931OpenVPNProtocolStructures(t *testing.T) {
	bin := os.Getenv("MIHOMO_11931_BIN")
	if bin == "" {
		t.Skip("未设置 MIHOMO_11931_BIN，跳过固定 Mihomo 1.19.31 OpenVPN 验收")
	}
	version, err := exec.Command(bin, "-v").CombinedOutput()
	if err != nil {
		t.Fatalf("读取 Mihomo 版本失败: %v: %s", err, version)
	}
	fields := strings.Fields(string(version))
	if len(fields) < 3 || fields[0] != "Mihomo" || fields[1] != "Meta" || fields[2] != "v1.19.31" {
		t.Fatalf("固定验收要求 Mihomo v1.19.31，实际: %s", version)
	}

	ca := openvpnTestPEM("CERTIFICATE", 320)
	cert := openvpnTestPEM("CERTIFICATE", 288)
	key := openvpnTestPEM("PRIVATE KEY", 256)
	tlsCryptV2 := openvpnTestPEM("OpenVPN tls-crypt-v2 client key", 400)

	t.Run("accepts generated structures", func(t *testing.T) {
		generated := []struct {
			name   string
			params map[string]any
		}{
			{
				name: "userpass with tls-auth and tuning",
				params: map[string]any{
					"ca": ca, "username": "ovpn-user", "password": "ovpn-fixture-secret",
					"proto": "tcp", "dev": "tun", "cipher": "CHACHA20-POLY1305",
					"data-ciphers":          []any{"AES-256-GCM", "CHACHA20-POLY1305"},
					"data-ciphers-fallback": "AES-128-CBC", "auth": "SHA256", "comp-lzo": "adaptive",
					"tls-auth": openvpnStaticKey(), "key-direction": "1",
					"ping": 10, "ping-restart": 60, "tran-window": 0, "handshake-timeout": 30, "mtu": 1400,
					"peer-info": map[string]any{"IV_VER": "2.6", "IV_PLAT": "mac"},
					"ip-stack":  map[string]any{"mode": "auto"},
				},
			},
			{
				name: "cert_userpass with tls-crypt and dns",
				params: map[string]any{
					"ca": ca, "cert": cert, "key": key,
					"username": "ovpn-user", "password": "ovpn-fixture-secret",
					"tls-crypt": openvpnStaticKey(), "proto": "udp",
					"remote-dns-resolve": true, "dns": []any{"1.1.1.1", "8.8.8.8"},
				},
			},
			{
				name: "cert only with tls-crypt-v2",
				params: map[string]any{
					"ca": ca, "cert": cert, "key": key, "tls-crypt-v2": tlsCryptV2,
				},
			},
		}
		for _, tc := range generated {
			t.Run(tc.name, func(t *testing.T) {
				render := "fixed-ovpn-" + strings.ReplaceAll(tc.name, " ", "-")
				proxy := clashProxyFixture(t, &nodeData{
					Protocol: "openvpn", RenderName: render,
					Host: "192.0.2.1", Port: 1194, ProtocolJSON: tc.params,
				})
				content := mihomoSSPluginConfig(t, orderedMapToMapSlice(proxy), render)
				output, err := runMihomoConfigTest(t, bin, content)
				if err != nil {
					t.Fatalf("Mihomo 1.19.31 拒绝生成的 OpenVPN 结构 %s: %v\n%s\nYAML:\n%s", tc.name, err, output, content)
				}
			})
		}
	})

	t.Run("rejects invalid structures", func(t *testing.T) {
		base := func() gyaml.MapSlice {
			return gyaml.MapSlice{
				{Key: "name", Value: "bad-ovpn"},
				{Key: "type", Value: "openvpn"},
				{Key: "server", Value: "192.0.2.1"}, {Key: "port", Value: 1194},
			}
		}
		withCA := func(extra ...gyaml.MapItem) gyaml.MapSlice {
			proxy := append(base(), gyaml.MapItem{Key: "ca", Value: ca})
			return append(proxy, extra...)
		}
		cases := []struct {
			name       string
			proxy      gyaml.MapSlice
			wantOutput string
		}{
			{
				// ca 没有 omitempty，解码期就以 has unset fields 拒绝，早于构造器的内联块校验。
				name:       "missing ca",
				proxy:      append(base(), gyaml.MapItem{Key: "username", Value: "u"}),
				wantOutput: "has unset fields: ca",
			},
			{
				name:       "non PEM ca",
				proxy:      append(base(), gyaml.MapItem{Key: "ca", Value: "not-a-pem"}, gyaml.MapItem{Key: "username", Value: "u"}),
				wantOutput: "not PEM",
			},
			{
				name:       "cert without key",
				proxy:      withCA(gyaml.MapItem{Key: "cert", Value: cert}),
				wantOutput: "cert and key must both be set",
			},
			{
				name:       "no cert and no username",
				proxy:      withCA(),
				wantOutput: "requires either cert+key or username",
			},
			{
				name: "tls-auth and tls-crypt mutually exclusive",
				proxy: withCA(
					gyaml.MapItem{Key: "username", Value: "u"},
					gyaml.MapItem{Key: "tls-auth", Value: openvpnStaticKey()},
					gyaml.MapItem{Key: "tls-crypt", Value: openvpnStaticKey()},
				),
				wantOutput: "mutually exclusive",
			},
			{
				name:       "unsupported cipher",
				proxy:      withCA(gyaml.MapItem{Key: "username", Value: "u"}, gyaml.MapItem{Key: "cipher", Value: "AES-256-CFB"}),
				wantOutput: "unsupported openvpn cipher",
			},
			{
				name:       "unsupported dev",
				proxy:      withCA(gyaml.MapItem{Key: "username", Value: "u"}, gyaml.MapItem{Key: "dev", Value: "tap"}),
				wantOutput: "only dev tun",
			},
			{
				name:       "unsupported proto",
				proxy:      withCA(gyaml.MapItem{Key: "username", Value: "u"}, gyaml.MapItem{Key: "proto", Value: "sctp"}),
				wantOutput: "unsupported openvpn proto",
			},
			{
				name:       "short tls-auth static key",
				proxy:      withCA(gyaml.MapItem{Key: "username", Value: "u"}, gyaml.MapItem{Key: "tls-auth", Value: "aabbcc"}),
				wantOutput: "invalid static key length",
			},
			{
				name:       "invalid key-direction",
				proxy:      withCA(gyaml.MapItem{Key: "username", Value: "u"}, gyaml.MapItem{Key: "tls-auth", Value: openvpnStaticKey()}, gyaml.MapItem{Key: "key-direction", Value: "2"}),
				wantOutput: "key-direction",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				content := mihomoSSPluginConfig(t, tc.proxy, "bad-ovpn")
				output, err := runMihomoConfigTest(t, bin, content)
				if err == nil {
					t.Fatalf("Mihomo 1.19.31 应拒绝无效 OpenVPN 结构 %s\n%s", tc.name, content)
				}
				if tc.wantOutput != "" && !strings.Contains(string(output), tc.wantOutput) {
					t.Fatalf("Mihomo 1.19.31 拒绝原因异常，期望包含 %q，实际:\n%s", tc.wantOutput, output)
				}
			})
		}
	})
}
