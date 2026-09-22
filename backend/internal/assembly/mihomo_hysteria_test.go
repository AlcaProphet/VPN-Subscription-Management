package assembly

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	gyaml "github.com/goccy/go-yaml"

	"vpn-sub/internal/node"
)

// TestMihomo11931HysteriaProtocolStructures 覆盖 Hysteria adapter 在固定 Mihomo v1.19.31
// 上的代表性正反例；普通 go test 未设置外部二进制时跳过，严格入口由 .mihomo-test.sh 负责。
func TestMihomo11931HysteriaProtocolStructures(t *testing.T) {
	bin := os.Getenv("MIHOMO_11931_BIN")
	if bin == "" {
		t.Skip("未设置 MIHOMO_11931_BIN，跳过固定 Mihomo 1.19.31 Hysteria 验收")
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
				name: "base64 auth with hopping", render: "fixed-hysteria-base64",
				proxy: new(Service).clashProxy(&nodeData{
					Protocol: "hysteria", RenderName: "fixed-hysteria-base64", Host: "192.0.2.1", Port: 443,
					ProtocolJSON: map[string]any{"up": "100 Mbps", "down": "100 Mbps", "auth": "dGVzdC1hdXRo",
						"ports": "1000-2000", "protocol": "udp", "sni": "example.com", "alpn": []any{"hysteria"}},
					CurrentState: node.CurrentState{Selectors: map[string]string{"auth_mode": "base64"}},
				}),
			},
			{
				name: "string auth no obfs", render: "fixed-hysteria-string",
				proxy: new(Service).clashProxy(&nodeData{
					Protocol: "hysteria", RenderName: "fixed-hysteria-string", Host: "192.0.2.1", Port: 443,
					ProtocolJSON: map[string]any{"up": "100 Mbps", "down": "100 Mbps", "auth-str": "plain-auth"},
					CurrentState: node.CurrentState{Selectors: map[string]string{"auth_mode": "string"}},
				}),
			},
		}
		for _, tc := range generated {
			t.Run(tc.name, func(t *testing.T) {
				content := mihomoSSPluginConfig(t, orderedMapToMapSlice(tc.proxy), tc.render)
				output, err := runMihomoConfigTest(t, bin, content)
				if err != nil {
					t.Fatalf("Mihomo 1.19.31 拒绝生成的 Hysteria 结构 %s: %v\n%s\nYAML:\n%s", tc.name, err, output, content)
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
				name: "hysteria invalid base64 auth",
				proxy: gyaml.MapSlice{
					{Key: "name", Value: "bad-hysteria-auth"}, {Key: "type", Value: "hysteria"},
					{Key: "server", Value: "192.0.2.1"}, {Key: "port", Value: 443},
					{Key: "up", Value: "100 Mbps"}, {Key: "down", Value: "100 Mbps"},
					{Key: "auth", Value: "not base64!!"},
				},
				wantOutput: "illegal base64",
			},
			{
				name: "hysteria invalid bandwidth",
				proxy: gyaml.MapSlice{
					{Key: "name", Value: "bad-hysteria-bandwidth"}, {Key: "type", Value: "hysteria"},
					{Key: "server", Value: "192.0.2.1"}, {Key: "port", Value: 443},
					{Key: "up", Value: "abc"}, {Key: "down", Value: "100 Mbps"},
				},
				wantOutput: "invaild upload speed",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				content := mihomoSSPluginConfig(t, tc.proxy, "bad-"+strings.ReplaceAll(tc.name, " ", "-"))
				output, err := runMihomoConfigTest(t, bin, content)
				if err == nil {
					t.Fatalf("Mihomo 1.19.31 应拒绝无效 Hysteria 结构 %s\n%s", tc.name, content)
				}
				if tc.wantOutput != "" && !strings.Contains(string(output), tc.wantOutput) {
					t.Fatalf("Mihomo 1.19.31 拒绝原因异常，期望包含 %q，实际:\n%s", tc.wantOutput, output)
				}
			})
		}
	})
}
