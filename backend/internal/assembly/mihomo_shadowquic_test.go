package assembly

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	gyaml "github.com/goccy/go-yaml"
)

// TestMihomo11931ShadowQUICProtocolStructures 覆盖 ShadowQUIC adapter 在固定 Mihomo v1.19.31
// 上的代表性正反例；普通 go test 未设置外部二进制时跳过，严格入口由 .mihomo-test.sh 负责。
func TestMihomo11931ShadowQUICProtocolStructures(t *testing.T) {
	bin := os.Getenv("MIHOMO_11931_BIN")
	if bin == "" {
		t.Skip("未设置 MIHOMO_11931_BIN，跳过固定 Mihomo 1.19.31 ShadowQUIC 验收")
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
			params map[string]any
		}{
			{
				name: "default quic versions with uot",
				params: map[string]any{
					"username": "squic-user", "password": "squic-fixture-secret",
					"sni": "example.com", "alpn": []any{"h3"},
					"udp-over-stream": true, "zero-rtt": true, "keep-alive-interval": 30,
					"congestion-controller": "bbr_meta_v2", "up": "100 Mbps", "down": "100 Mbps",
					"cwnd": 64, "bbr-profile": "standard", "recv-window-conn": 1024, "recv-window": 2048,
					"disable-mtu-discovery": true, "max-datagram-frame-size": 1400, "max-open-streams": 1024,
				},
			},
			{
				name: "explicit ordered quic versions",
				params: map[string]any{
					"username": "squic-user", "password": "squic-fixture-secret",
					"quic-versions": []any{"v2", "v1"},
				},
			},
		}
		for _, tc := range generated {
			t.Run(tc.name, func(t *testing.T) {
				render := "fixed-squic-" + strings.ReplaceAll(tc.name, " ", "-")
				proxy := new(Service).clashProxy(&nodeData{
					Protocol: "shadowquic", RenderName: render,
					Host: "192.0.2.1", Port: 443, ProtocolJSON: tc.params,
				})
				content := mihomoSSPluginConfig(t, orderedMapToMapSlice(proxy), render)
				output, err := runMihomoConfigTest(t, bin, content)
				if err != nil {
					t.Fatalf("Mihomo 1.19.31 拒绝生成的 ShadowQUIC 结构 %s: %v\n%s\nYAML:\n%s", tc.name, err, output, content)
				}
			})
		}
	})

	t.Run("rejects invalid structures", func(t *testing.T) {
		cases := []struct {
			name       string
			versions   []any
			wantOutput string
		}{
			{name: "unsupported version v3", versions: []any{"v3"}, wantOutput: "unsupported QUIC version"},
			{name: "mixed supported and unsupported", versions: []any{"v1", "bogus"}, wantOutput: "unsupported QUIC version"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				proxy := gyaml.MapSlice{
					{Key: "name", Value: "bad-" + strings.ReplaceAll(tc.name, " ", "-")},
					{Key: "type", Value: "shadowquic"},
					{Key: "server", Value: "192.0.2.1"}, {Key: "port", Value: 443},
					{Key: "username", Value: "u"}, {Key: "password", Value: "p"},
					{Key: "quic-versions", Value: tc.versions},
				}
				content := mihomoSSPluginConfig(t, proxy, "bad-squic")
				output, err := runMihomoConfigTest(t, bin, content)
				if err == nil {
					t.Fatalf("Mihomo 1.19.31 应拒绝无效 ShadowQUIC 结构 %s\n%s", tc.name, content)
				}
				if tc.wantOutput != "" && !strings.Contains(string(output), tc.wantOutput) {
					t.Fatalf("Mihomo 1.19.31 拒绝原因异常，期望包含 %q，实际:\n%s", tc.wantOutput, output)
				}
			})
		}
	})
}
