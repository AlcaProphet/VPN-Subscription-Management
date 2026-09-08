package assembly

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gyaml "github.com/goccy/go-yaml"

	"vpn-sub/internal/ssplugin"
)

func TestMihomo11929AcceptsGeneratedSSPluginStructures(t *testing.T) {
	bin := os.Getenv("MIHOMO_11929_BIN")
	if bin == "" {
		t.Skip("未设置 MIHOMO_11929_BIN，跳过固定 Mihomo 1.19.29 二进制验收")
	}
	version, err := exec.Command(bin, "-v").CombinedOutput()
	if err != nil {
		t.Fatalf("读取 Mihomo 版本失败: %v: %s", err, version)
	}
	if !strings.Contains(string(version), "v1.19.29") {
		t.Fatalf("固定验收要求 Mihomo v1.19.29，实际: %s", version)
	}

	cases := []struct {
		name   string
		plugin string
		opts   map[string]any
	}{
		{name: "obfs", plugin: "obfs", opts: map[string]any{"mode": "http", "host": "cdn.example.com"}},
		{name: "v2ray-plugin", plugin: "v2ray-plugin", opts: map[string]any{
			"mode": "websocket", "host": "cdn.example.com", "path": "/ws", "tls": true, "skip-cert-verify": true,
			"ech-opts": map[string]any{"enable": false, "query-server-name": "ech.example.com"},
		}},
		{name: "shadow-tls", plugin: "shadow-tls", opts: map[string]any{"host": "tls.example.com", "password": "plugin-secret", "version": 3}},
		{name: "restls", plugin: "restls", opts: map[string]any{"password": "plugin-secret", "host": "tls.example.com", "version-hint": "tls13"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			definition, ok := ssplugin.Lookup(tc.plugin)
			if !ok {
				t.Fatalf("缺少已知插件合同: %s", tc.plugin)
			}
			params := map[string]any{
				"cipher": "aes-128-gcm", "password": "secret", "plugin": tc.plugin,
				definition.StorageKey: tc.opts,
			}
			proxy := new(Service).clashProxy(&nodeData{RenderName: "ss-" + tc.name, Protocol: "ss", Host: "example.com", Port: 443, ProtocolJSON: params})
			root := gyaml.MapSlice{
				{Key: "mixed-port", Value: 7890},
				{Key: "proxies", Value: []any{orderedMapToMapSlice(proxy)}},
				{Key: "proxy-groups", Value: []any{gyaml.MapSlice{
					{Key: "name", Value: "PROXY"}, {Key: "type", Value: "select"}, {Key: "proxies", Value: []string{"ss-" + tc.name}},
				}}},
				{Key: "rules", Value: []string{"MATCH,PROXY"}},
			}
			content, err := gyaml.Marshal(root)
			if err != nil {
				t.Fatal(err)
			}
			configPath := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(configPath, content, 0o600); err != nil {
				t.Fatal(err)
			}
			output, err := exec.Command(bin, "-t", "-f", configPath).CombinedOutput()
			if err != nil {
				t.Fatalf("Mihomo 1.19.29 拒绝生成的 %s 结构: %v\n%s\nYAML:\n%s", tc.plugin, err, output, content)
			}
		})
	}
}
