package assembly

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	gyaml "github.com/goccy/go-yaml"
)

// TestMihomo11931TrustTunnelProtocolStructures 覆盖 TrustTunnel adapter 在固定 Mihomo v1.19.31
// 上的代表性正反例；普通 go test 未设置外部二进制时跳过，严格入口由 .mihomo-test.sh 负责。
// ECH 启用路径不在内核证据内：固定 tag 在 enable=true 且未提供有效 ECHConfigList 时会走
// resolver 查询，因此与既有 SS 插件门禁一致，只验证 enable=false 时对象不写入 wire。
func TestMihomo11931TrustTunnelProtocolStructures(t *testing.T) {
	bin := os.Getenv("MIHOMO_11931_BIN")
	if bin == "" {
		t.Skip("未设置 MIHOMO_11931_BIN，跳过固定 Mihomo 1.19.31 TrustTunnel 验收")
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
				name: "quic connections with tuning",
				params: map[string]any{
					"username": "tt-user", "password": "tt-fixture-secret",
					"alpn": []any{"h3"}, "sni": "example.com", "client-fingerprint": "chrome",
					"skip-cert-verify": true, "udp": true, "health-check": true,
					"quic": true, "congestion-controller": "bbr_meta_v2", "cwnd": 64, "bbr-profile": "standard",
					"max-connections": 8, "min-streams": 5,
				},
			},
			{
				name: "h2 streams with ech disabled",
				params: map[string]any{
					"username": "tt-user", "password": "tt-fixture-secret",
					"alpn": []any{"h2"}, "max-streams": 4,
					"ech-opts": map[string]any{"enable": false, "query-server-name": "ech.example.com"},
				},
			},
			{
				name: "defaults without reuse and without quic",
				params: map[string]any{
					"username": "tt-user", "password": "tt-fixture-secret",
				},
			},
		}
		for _, tc := range generated {
			t.Run(tc.name, func(t *testing.T) {
				render := "fixed-tt-" + strings.ReplaceAll(tc.name, " ", "-")
				proxy := clashProxyFixture(t, &nodeData{
					Protocol: "trusttunnel", RenderName: render,
					Host: "192.0.2.1", Port: 443, ProtocolJSON: tc.params,
				})
				content := mihomoSSPluginConfig(t, orderedMapToMapSlice(proxy), render)
				output, err := runMihomoConfigTest(t, bin, content)
				if err != nil {
					t.Fatalf("Mihomo 1.19.31 拒绝生成的 TrustTunnel 结构 %s: %v\n%s\nYAML:\n%s", tc.name, err, output, content)
				}
			})
		}
	})

	t.Run("rejects invalid structures", func(t *testing.T) {
		cases := []struct {
			name       string
			proxy      gyaml.MapSlice
			wantOutput string
		}{
			{
				name: "quic requires h3 alpn",
				proxy: gyaml.MapSlice{
					{Key: "name", Value: "bad-tt-quic-alpn"},
					{Key: "type", Value: "trusttunnel"},
					{Key: "server", Value: "192.0.2.1"}, {Key: "port", Value: 443},
					{Key: "quic", Value: true},
					{Key: "alpn", Value: []any{"h2"}},
				},
				wantOutput: "require alpn h3",
			},
			{
				name: "h2 requires h2 alpn",
				proxy: gyaml.MapSlice{
					{Key: "name", Value: "bad-tt-h2-alpn"},
					{Key: "type", Value: "trusttunnel"},
					{Key: "server", Value: "192.0.2.1"}, {Key: "port", Value: 443},
					{Key: "alpn", Value: []any{"h3"}},
				},
				wantOutput: "require alpn h2",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				content := mihomoSSPluginConfig(t, tc.proxy, "bad-tt")
				output, err := runMihomoConfigTest(t, bin, content)
				if err == nil {
					t.Fatalf("Mihomo 1.19.31 应拒绝无效 TrustTunnel 结构 %s\n%s", tc.name, content)
				}
				if tc.wantOutput != "" && !strings.Contains(string(output), tc.wantOutput) {
					t.Fatalf("Mihomo 1.19.31 拒绝原因异常，期望包含 %q，实际:\n%s", tc.wantOutput, output)
				}
			})
		}
	})
}
