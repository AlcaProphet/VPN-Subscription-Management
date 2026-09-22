package assembly

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	gyaml "github.com/goccy/go-yaml"

	"vpn-sub/internal/node"
)

// TestMihomo11931TUICProtocolStructures 覆盖 TUIC adapter 在固定 Mihomo v1.19.31
// 上的代表性正反例；普通 go test 未设置外部二进制时跳过，严格入口由 .mihomo-test.sh 负责。
func TestMihomo11931TUICProtocolStructures(t *testing.T) {
	bin := os.Getenv("MIHOMO_11931_BIN")
	if bin == "" {
		t.Skip("未设置 MIHOMO_11931_BIN，跳过固定 Mihomo 1.19.31 TUIC 验收")
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
			state  node.CurrentState
			params map[string]any
		}{
			{
				name: "v5 uuid password", render: "fixed-tuic-v5",
				state: node.CurrentState{Selectors: map[string]string{"auth_mode": "v5"}},
				params: map[string]any{"uuid": "11111111-2222-3333-4444-555555555555", "password": "pw-secret",
					"sni": "example.com", "alpn": []any{"h3"}},
			},
			{
				name: "v4 token with uot", render: "fixed-tuic-v4",
				state: node.CurrentState{Selectors: map[string]string{"auth_mode": "v4"}, Features: []string{"udp-over-stream"}},
				params: map[string]any{"token": "token-secret", "sni": "example.com",
					"udp-over-stream": true, "udp-over-stream-version": "2"},
			},
		}
		for _, tc := range generated {
			t.Run(tc.name, func(t *testing.T) {
				proxy := new(Service).clashProxy(&nodeData{
					Protocol: "tuic", RenderName: tc.render, Host: "192.0.2.1", Port: 443,
					ProtocolJSON: tc.params, CurrentState: tc.state,
				})
				content := mihomoSSPluginConfig(t, orderedMapToMapSlice(proxy), tc.render)
				output, err := runMihomoConfigTest(t, bin, content)
				if err != nil {
					t.Fatalf("Mihomo 1.19.31 拒绝生成的 TUIC 结构 %s: %v\n%s\nYAML:\n%s", tc.name, err, output, content)
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
				name: "tuic unknown uot version",
				proxy: gyaml.MapSlice{
					{Key: "name", Value: "bad-tuic-uot"}, {Key: "type", Value: "tuic"},
					{Key: "server", Value: "192.0.2.1"}, {Key: "port", Value: 443},
					{Key: "uuid", Value: "11111111-2222-3333-4444-555555555555"}, {Key: "password", Value: "pw-secret"},
					{Key: "udp-over-stream", Value: true}, {Key: "udp-over-stream-version", Value: 9},
				},
				wantOutput: "unknown udp over stream protocol version",
			},
			{
				name: "tuic invalid client certificate",
				proxy: gyaml.MapSlice{
					{Key: "name", Value: "bad-tuic-cert"}, {Key: "type", Value: "tuic"},
					{Key: "server", Value: "192.0.2.1"}, {Key: "port", Value: 443},
					{Key: "uuid", Value: "11111111-2222-3333-4444-555555555555"}, {Key: "password", Value: "pw-secret"},
					{Key: "certificate", Value: "not-a-pem"},
				},
				wantOutput: "parse certificate failed",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				content := mihomoSSPluginConfig(t, tc.proxy, "bad-"+strings.ReplaceAll(tc.name, " ", "-"))
				output, err := runMihomoConfigTest(t, bin, content)
				if err == nil {
					t.Fatalf("Mihomo 1.19.31 应拒绝无效 TUIC 结构 %s\n%s", tc.name, content)
				}
				if tc.wantOutput != "" && !strings.Contains(string(output), tc.wantOutput) {
					t.Fatalf("Mihomo 1.19.31 拒绝原因异常，期望包含 %q，实际:\n%s", tc.wantOutput, output)
				}
			})
		}
	})
}
