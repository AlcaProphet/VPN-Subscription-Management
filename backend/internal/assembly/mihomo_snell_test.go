package assembly

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	gyaml "github.com/goccy/go-yaml"

	"vpn-sub/internal/node"
)

// TestMihomo11931SnellProtocolStructures 覆盖 Snell 显式 adapter 在固定 Mihomo v1.19.31
// 上的代表性正反例；普通 go test 未设置外部二进制时跳过，严格入口由 .mihomo-test.sh 负责。
func TestMihomo11931SnellProtocolStructures(t *testing.T) {
	bin := os.Getenv("MIHOMO_11931_BIN")
	if bin == "" {
		t.Skip("未设置 MIHOMO_11931_BIN，跳过固定 Mihomo 1.19.31 Snell 验收")
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
			render string
			proxy  *OrderedMap
		}{
			{
				name: "http obfs v4", render: "fixed-snell-http",
				proxy: new(Service).clashProxy(&nodeData{
					Protocol: "snell", RenderName: "fixed-snell-http", Host: "192.0.2.1", Port: 443,
					ProtocolJSON: map[string]any{"psk": "psk-secret", "version": "4", "udp": true,
						"obfs-opts": map[string]any{"host": "bing.com"}},
					CurrentState: node.CurrentState{Selectors: map[string]string{"version": "4", "obfs_mode": "http"}},
				}),
			},
			{
				name: "v2 fixed reuse", render: "fixed-snell-v2",
				proxy: new(Service).clashProxy(&nodeData{
					Protocol: "snell", RenderName: "fixed-snell-v2", Host: "192.0.2.1", Port: 443,
					ProtocolJSON: map[string]any{"psk": "psk-secret", "version": "2"},
					CurrentState: node.CurrentState{Selectors: map[string]string{"version": "2", "obfs_mode": "none"}},
				}),
			},
			{
				name: "shadow-tls v4", render: "fixed-snell-shadow-tls",
				proxy: new(Service).clashProxy(&nodeData{
					Protocol: "snell", RenderName: "fixed-snell-shadow-tls", Host: "192.0.2.1", Port: 443,
					ProtocolJSON: map[string]any{"psk": "psk-secret", "version": "4", "client-fingerprint": "chrome",
						"obfs-opts": map[string]any{"host": "bing.com", "password": "obfs-secret", "version": 2}},
					CurrentState: node.CurrentState{Selectors: map[string]string{"version": "4", "obfs_mode": "shadow_tls"}},
				}),
			},
		}
		for _, tc := range generated {
			t.Run(tc.name, func(t *testing.T) {
				content := mihomoSSPluginConfig(t, orderedMapToMapSlice(tc.proxy), tc.render)
				output, err := runMihomoConfigTest(t, bin, content)
				if err != nil {
					t.Fatalf("Mihomo 1.19.31 拒绝生成的 Snell 结构 %s: %v\n%s\nYAML:\n%s", tc.name, err, output, content)
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
				name: "snell invalid obfs mode",
				proxy: gyaml.MapSlice{
					{Key: "name", Value: "bad-snell-obfs"}, {Key: "type", Value: "snell"},
					{Key: "server", Value: "192.0.2.1"}, {Key: "port", Value: 443},
					{Key: "psk", Value: "psk-secret"}, {Key: "version", Value: 4},
					{Key: "obfs-opts", Value: map[string]any{"mode": "bogus", "host": "bing.com"}},
				},
				wantOutput: "obfs mode error",
			},
			{
				name: "snell v1 udp",
				proxy: gyaml.MapSlice{
					{Key: "name", Value: "bad-snell-udp"}, {Key: "type", Value: "snell"},
					{Key: "server", Value: "192.0.2.1"}, {Key: "port", Value: 443},
					{Key: "psk", Value: "psk-secret"}, {Key: "version", Value: 1}, {Key: "udp", Value: true},
				},
				wantOutput: "not support UDP",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				content := mihomoSSPluginConfig(t, tc.proxy, "bad-"+strings.ReplaceAll(tc.name, " ", "-"))
				output, err := runMihomoConfigTest(t, bin, content)
				if err == nil {
					t.Fatalf("Mihomo 1.19.31 应拒绝无效 Snell 结构 %s\n%s", tc.name, content)
				}
				if tc.wantOutput != "" && !strings.Contains(string(output), tc.wantOutput) {
					t.Fatalf("Mihomo 1.19.31 拒绝原因异常，期望包含 %q，实际:\n%s", tc.wantOutput, output)
				}
			})
		}
	})
}
