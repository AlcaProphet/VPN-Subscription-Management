package assembly

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gyaml "github.com/goccy/go-yaml"
)

// TestMihomo11931HTTPProtocolStructures 覆盖 HTTP 显式 adapter 在固定 Mihomo v1.19.31
// 上的代表性正反例；普通 go test 未设置外部二进制时跳过，严格入口由 .mihomo-test.sh 负责。
func TestMihomo11931HTTPProtocolStructures(t *testing.T) {
	bin := os.Getenv("MIHOMO_11931_BIN")
	if bin == "" {
		t.Skip("未设置 MIHOMO_11931_BIN，跳过固定 Mihomo 1.19.31 HTTP 验收")
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
		raw, err := os.ReadFile(filepath.Join("testdata", "node_check", "http-basic-tls.json"))
		if err != nil {
			t.Fatalf("读取固定夹具失败: %v", err)
		}
		var fixture struct {
			Protocol     string         `json:"protocol"`
			Host         string         `json:"host"`
			Port         int            `json:"port"`
			ProtocolJSON map[string]any `json:"protocol_json"`
		}
		if err := json.Unmarshal(raw, &fixture); err != nil {
			t.Fatalf("解析固定夹具失败: %v", err)
		}
		proxy := new(Service).clashProxy(&nodeData{
			Protocol: fixture.Protocol, RenderName: "fixed-http-basic-tls",
			Host: fixture.Host, Port: fixture.Port, ProtocolJSON: fixture.ProtocolJSON,
		})
		content := mihomoSSPluginConfig(t, orderedMapToMapSlice(proxy), "fixed-http-basic-tls")
		output, err := runMihomoConfigTest(t, bin, content)
		if err != nil {
			t.Fatalf("Mihomo 1.19.31 拒绝生成的 HTTP 结构: %v\n%s\nYAML:\n%s", err, output, content)
		}
	})

	t.Run("rejects invalid structures", func(t *testing.T) {
		cases := []struct {
			name       string
			proxy      gyaml.MapSlice
			wantOutput string
		}{
			{
				name: "http invalid client certificate",
				proxy: gyaml.MapSlice{
					{Key: "name", Value: "bad-http-cert"}, {Key: "type", Value: "http"},
					{Key: "server", Value: "192.0.2.1"}, {Key: "port", Value: 8080},
					{Key: "tls", Value: true}, {Key: "certificate", Value: "not-a-pem"},
				},
				wantOutput: "parse certificate failed",
			},
			{
				name: "http headers non string value",
				proxy: gyaml.MapSlice{
					{Key: "name", Value: "bad-http-headers"}, {Key: "type", Value: "http"},
					{Key: "server", Value: "192.0.2.1"}, {Key: "port", Value: 8080},
					{Key: "headers", Value: map[string]any{"X-Test": 7}},
				},
				wantOutput: "headers",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				content := mihomoSSPluginConfig(t, tc.proxy, "bad-"+strings.ReplaceAll(tc.name, " ", "-"))
				output, err := runMihomoConfigTest(t, bin, content)
				if err == nil {
					t.Fatalf("Mihomo 1.19.31 应拒绝无效 HTTP 结构 %s\n%s", tc.name, content)
				}
				if !strings.Contains(string(output), tc.wantOutput) {
					t.Fatalf("Mihomo 1.19.31 拒绝原因异常，期望包含 %q，实际:\n%s", tc.wantOutput, output)
				}
			})
		}
	})
}
