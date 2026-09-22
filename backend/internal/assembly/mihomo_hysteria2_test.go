package assembly

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	gyaml "github.com/goccy/go-yaml"

	"vpn-sub/internal/node"
)

// TestMihomo11931Hysteria2ProtocolStructures 覆盖 Hysteria2 adapter 在固定 Mihomo v1.19.31
// 上的代表性正反例；普通 go test 未设置外部二进制时跳过，严格入口由 .mihomo-test.sh 负责。
func TestMihomo11931Hysteria2ProtocolStructures(t *testing.T) {
	bin := os.Getenv("MIHOMO_11931_BIN")
	if bin == "" {
		t.Skip("未设置 MIHOMO_11931_BIN，跳过固定 Mihomo 1.19.31 Hysteria2 验收")
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
			port   int
			state  node.CurrentState
			params map[string]any
		}{
			{
				name: "single with gecko obfs", render: "fixed-hysteria2-single", port: 443,
				state: node.CurrentState{Selectors: map[string]string{"endpoint_mode": "single", "obfs_mode": "gecko"}},
				params: map[string]any{"password": "hy2-secret", "obfs-password": "obfs-secret",
					"obfs-min-packet-size": 20, "obfs-max-packet-size": 100, "sni": "example.com", "alpn": []any{"h3"}},
			},
			{
				name: "ports mode without port", render: "fixed-hysteria2-ports", port: 0,
				state:  node.CurrentState{Selectors: map[string]string{"endpoint_mode": "ports", "obfs_mode": "none"}},
				params: map[string]any{"password": "hy2-secret", "ports": "1000-2000", "hop-interval": "10-20"},
			},
		}
		for _, tc := range generated {
			t.Run(tc.name, func(t *testing.T) {
				proxy := new(Service).clashProxy(&nodeData{
					Protocol: "hysteria2", RenderName: tc.render, Host: "192.0.2.1", Port: tc.port,
					ProtocolJSON: tc.params, CurrentState: tc.state,
				})
				content := mihomoSSPluginConfig(t, orderedMapToMapSlice(proxy), tc.render)
				output, err := runMihomoConfigTest(t, bin, content)
				if err != nil {
					t.Fatalf("Mihomo 1.19.31 拒绝生成的 Hysteria2 结构 %s: %v\n%s\nYAML:\n%s", tc.name, err, output, content)
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
				name: "hysteria2 unknown obfs",
				proxy: gyaml.MapSlice{
					{Key: "name", Value: "bad-hysteria2-obfs"}, {Key: "type", Value: "hysteria2"},
					{Key: "server", Value: "192.0.2.1"}, {Key: "port", Value: 443},
					{Key: "password", Value: "hy2-secret"}, {Key: "obfs", Value: "bogus"},
					{Key: "obfs-password", Value: "obfs-secret"},
				},
				wantOutput: "unknown obfs type",
			},
			{
				name: "hysteria2 obfs without password",
				proxy: gyaml.MapSlice{
					{Key: "name", Value: "bad-hysteria2-obfs-password"}, {Key: "type", Value: "hysteria2"},
					{Key: "server", Value: "192.0.2.1"}, {Key: "port", Value: 443},
					{Key: "password", Value: "hy2-secret"}, {Key: "obfs", Value: "salamander"},
				},
				wantOutput: "missing obfs password",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				content := mihomoSSPluginConfig(t, tc.proxy, "bad-"+strings.ReplaceAll(tc.name, " ", "-"))
				output, err := runMihomoConfigTest(t, bin, content)
				if err == nil {
					t.Fatalf("Mihomo 1.19.31 应拒绝无效 Hysteria2 结构 %s\n%s", tc.name, content)
				}
				if tc.wantOutput != "" && !strings.Contains(string(output), tc.wantOutput) {
					t.Fatalf("Mihomo 1.19.31 拒绝原因异常，期望包含 %q，实际:\n%s", tc.wantOutput, output)
				}
			})
		}
	})
}
