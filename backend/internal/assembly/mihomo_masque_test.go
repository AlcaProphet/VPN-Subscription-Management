package assembly

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	gyaml "github.com/goccy/go-yaml"

	"vpn-sub/internal/node"
)

// 固定内核 MASQUE 夹具使用一次性 P-256 密钥对（由 x509 生成，不是真实凭据）。
const (
	masqueFixedPrivateKey = "MHcCAQEEIMufpAZbwGL1tVijQ1W75eD7XO5WPksPV4jDdBOcaewXoAoGCCqGSM49AwEHoUQDQgAEnX9WTsWhQKf1dpuENdH0NzmFspTB2M65dUgddx9WRkon8WkdycFYUetBTFGyLg+qs8VtaACuGIcnvfXkIG7L5g=="
	masqueFixedPublicKey  = "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEnX9WTsWhQKf1dpuENdH0NzmFspTB2M65dUgddx9WRkon8WkdycFYUetBTFGyLg+qs8VtaACuGIcnvfXkIG7L5g=="
)

// TestMihomo11931MASQUEProtocolStructures 覆盖 MASQUE adapter 在固定 Mihomo v1.19.31
// 上的代表性正反例；普通 go test 未设置外部二进制时跳过，严格入口由 .mihomo-test.sh 负责。
func TestMihomo11931MASQUEProtocolStructures(t *testing.T) {
	bin := os.Getenv("MIHOMO_11931_BIN")
	if bin == "" {
		t.Skip("未设置 MIHOMO_11931_BIN，跳过固定 Mihomo 1.19.31 MASQUE 验收")
	}
	version, err := exec.Command(bin, "-v").CombinedOutput()
	if err != nil {
		t.Fatalf("读取 Mihomo 版本失败: %v: %s", err, version)
	}
	fields := strings.Fields(string(version))
	if len(fields) < 3 || fields[0] != "Mihomo" || fields[1] != "Meta" || fields[2] != "v1.19.31" {
		t.Fatalf("固定验收要求 Mihomo v1.19.31，实际: %s", version)
	}

	t.Run("accepts generated structures", func(t *testing.T) {
		generated := []struct {
			name   string
			state  node.CurrentState
			params map[string]any
		}{
			{
				name:  "quic default network",
				state: node.CurrentState{Selectors: map[string]string{"network_mode": "quic"}},
				params: map[string]any{
					"private-key": masqueFixedPrivateKey, "public-key": masqueFixedPublicKey,
					"ip": "192.0.2.2/32", "sni": "example.com", "mtu": 1280, "udp": true,
					"congestion-controller": "bbr_meta_v2", "cwnd": 64, "bbr-profile": "standard",
					"ip-stack": map[string]any{"mode": "gvisor", "congestion-controller": "bbr"},
				},
			},
			{
				name:  "h2 without quic tuning",
				state: node.CurrentState{Selectors: map[string]string{"network_mode": "h2"}},
				params: map[string]any{
					"private-key": masqueFixedPrivateKey, "public-key": masqueFixedPublicKey,
					"ip": "192.0.2.2/32", "ipv6": "2001:db8::2/128", "udp": true,
				},
			},
			{
				name:  "h3 l4proxy without udp",
				state: node.CurrentState{Selectors: map[string]string{"network_mode": "h3_l4proxy"}},
				params: map[string]any{
					"private-key": masqueFixedPrivateKey, "public-key": masqueFixedPublicKey,
					"ip": "192.0.2.2/32",
				},
			},
		}
		for _, tc := range generated {
			t.Run(tc.name, func(t *testing.T) {
				proxy := new(Service).clashProxy(&nodeData{
					Protocol: "masque", RenderName: "fixed-masque-" + strings.ReplaceAll(tc.name, " ", "-"),
					Host: "192.0.2.1", Port: 443, ProtocolJSON: tc.params, CurrentState: tc.state,
				})
				render := "fixed-masque-" + strings.ReplaceAll(tc.name, " ", "-")
				content := mihomoSSPluginConfig(t, orderedMapToMapSlice(proxy), render)
				output, err := runMihomoConfigTest(t, bin, content)
				if err != nil {
					t.Fatalf("Mihomo 1.19.31 拒绝生成的 MASQUE 结构 %s: %v\n%s\nYAML:\n%s", tc.name, err, output, content)
				}
			})
		}
	})

	t.Run("rejects invalid structures", func(t *testing.T) {
		cases := []struct {
			name       string
			extra      gyaml.MapSlice
			wantOutput string
		}{
			{
				name: "masque private key not EC",
				extra: gyaml.MapSlice{
					{Key: "private-key", Value: "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="},
					{Key: "public-key", Value: masqueFixedPublicKey}, {Key: "ip", Value: "192.0.2.2/32"},
				},
				wantOutput: "failed to parse private key",
			},
			{
				name: "masque public key not ECDSA",
				extra: gyaml.MapSlice{
					{Key: "private-key", Value: masqueFixedPrivateKey},
					{Key: "public-key", Value: "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="},
					{Key: "ip", Value: "192.0.2.2/32"},
				},
				wantOutput: "failed to parse public key",
			},
			{
				name: "masque negative handshake timeout",
				extra: gyaml.MapSlice{
					{Key: "private-key", Value: masqueFixedPrivateKey}, {Key: "public-key", Value: masqueFixedPublicKey},
					{Key: "ip", Value: "192.0.2.2/32"}, {Key: "handshake-timeout", Value: -1},
				},
				wantOutput: "handshake timeout must be non-negative",
			},
			{
				name: "masque invalid ip stack mode",
				extra: gyaml.MapSlice{
					{Key: "private-key", Value: masqueFixedPrivateKey}, {Key: "public-key", Value: masqueFixedPublicKey},
					{Key: "ip", Value: "192.0.2.2/32"},
					{Key: "ip-stack", Value: gyaml.MapSlice{{Key: "mode", Value: "system"}}},
				},
				wantOutput: "invalid IP stack mode",
			},
			{
				name: "masque missing local address",
				extra: gyaml.MapSlice{
					{Key: "private-key", Value: masqueFixedPrivateKey}, {Key: "public-key", Value: masqueFixedPublicKey},
				},
				wantOutput: "missing local address",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				proxy := gyaml.MapSlice{
					{Key: "name", Value: "bad-" + strings.ReplaceAll(tc.name, " ", "-")},
					{Key: "type", Value: "masque"},
					{Key: "server", Value: "192.0.2.1"}, {Key: "port", Value: 443},
				}
				proxy = append(proxy, tc.extra...)
				content := mihomoSSPluginConfig(t, proxy, "bad-masque")
				output, err := runMihomoConfigTest(t, bin, content)
				if err == nil {
					t.Fatalf("Mihomo 1.19.31 应拒绝无效 MASQUE 结构 %s\n%s", tc.name, content)
				}
				if tc.wantOutput != "" && !strings.Contains(string(output), tc.wantOutput) {
					t.Fatalf("Mihomo 1.19.31 拒绝原因异常，期望包含 %q，实际:\n%s", tc.wantOutput, output)
				}
			})
		}
	})
}
