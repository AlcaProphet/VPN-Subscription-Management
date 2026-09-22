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

// TestMihomo11931FirstBatchProtocolStructures 覆盖首批四协议在固定
// Mihomo v1.19.31 上的代表性正反例。普通 go test 未设置外部二进制时仍跳过，
// 严格执行由根目录 .mihomo-test.sh 负责。
func TestMihomo11931FirstBatchProtocolStructures(t *testing.T) {
	bin := os.Getenv("MIHOMO_11931_BIN")
	if bin == "" {
		t.Skip("未设置 MIHOMO_11931_BIN，跳过固定 Mihomo 1.19.31 首批协议验收")
	}
	version, err := exec.Command(bin, "-v").CombinedOutput()
	if err != nil {
		t.Fatalf("读取 Mihomo 版本失败: %v: %s", err, version)
	}
	fields := strings.Fields(string(version))
	if len(fields) < 3 || fields[0] != "Mihomo" || fields[1] != "Meta" || fields[2] != "v1.19.31" {
		t.Fatalf("固定验收要求 Mihomo v1.19.31，实际: %s", version)
	}

	positives := []string{
		"vless-ws-tls.json",
		"vmess-ws-tls.json",
		"trojan-ws-tls.json",
		"ss-aes-gcm.json",
	}
	t.Run("accepts fixtures", func(t *testing.T) {
		for _, name := range positives {
			t.Run(name, func(t *testing.T) {
				raw, err := os.ReadFile(filepath.Join("testdata", "node_check", name))
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
				renderName := "fixed-" + strings.TrimSuffix(name, ".json")
				proxy := new(Service).clashProxy(&nodeData{
					Protocol: fixture.Protocol, RenderName: renderName,
					Host: fixture.Host, Port: fixture.Port, ProtocolJSON: fixture.ProtocolJSON,
				})
				content := mihomoSSPluginConfig(t, orderedMapToMapSlice(proxy), renderName)
				output, err := runMihomoConfigTest(t, bin, content)
				if err != nil {
					t.Fatalf("Mihomo 1.19.31 拒绝生成的 %s 结构: %v\n%s\nYAML:\n%s", fixture.Protocol, err, output, content)
				}
			})
		}
	})

	invalid := []struct {
		name       string
		proxy      gyaml.MapSlice
		wantOutput string
	}{
		{
			name: "vless missing uuid",
			proxy: gyaml.MapSlice{
				{Key: "name", Value: "bad-vless"}, {Key: "type", Value: "vless"},
				{Key: "server", Value: "example.com"}, {Key: "port", Value: 443},
			},
			wantOutput: "unset fields: uuid",
		},
		{
			name: "vmess missing uuid",
			proxy: gyaml.MapSlice{
				{Key: "name", Value: "bad-vmess"}, {Key: "type", Value: "vmess"},
				{Key: "server", Value: "example.com"}, {Key: "port", Value: 443},
			},
			wantOutput: "uuid",
		},
		{
			name: "trojan missing password",
			proxy: gyaml.MapSlice{
				{Key: "name", Value: "bad-trojan"}, {Key: "type", Value: "trojan"},
				{Key: "server", Value: "example.com"}, {Key: "port", Value: 443},
			},
			wantOutput: "unset fields: password",
		},
		{
			name: "ss invalid cipher",
			proxy: gyaml.MapSlice{
				{Key: "name", Value: "bad-ss"}, {Key: "type", Value: "ss"},
				{Key: "server", Value: "example.com"}, {Key: "port", Value: 8388},
				{Key: "cipher", Value: "not-a-cipher"}, {Key: "password", Value: "secret"},
			},
			wantOutput: "unknown method: not-a-cipher",
		},
	}
	t.Run("rejects invalid structures", func(t *testing.T) {
		for _, tc := range invalid {
			t.Run(tc.name, func(t *testing.T) {
				content := mihomoSSPluginConfig(t, tc.proxy, "bad-"+strings.ReplaceAll(tc.name, " ", "-"))
				output, err := runMihomoConfigTest(t, bin, content)
				if err == nil {
					t.Fatalf("Mihomo 1.19.31 应拒绝无效结构 %s\n%s", tc.name, content)
				}
				if !strings.Contains(string(output), tc.wantOutput) {
					t.Fatalf("Mihomo 1.19.31 拒绝原因异常，期望包含 %q，实际:\n%s", tc.wantOutput, output)
				}
			})
		}
	})
}
