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
	versionFields := strings.Fields(string(version))
	if len(versionFields) < 3 || versionFields[0] != "Mihomo" || versionFields[1] != "Meta" || versionFields[2] != "v1.19.29" {
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

	t.Run("accepts generated structures", func(t *testing.T) {
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
				content := mihomoSSPluginConfig(t, orderedMapToMapSlice(proxy), "ss-"+tc.name)
				output, err := runMihomoConfigTest(t, bin, content)
				if err != nil {
					t.Fatalf("Mihomo 1.19.29 拒绝生成的 %s 结构: %v\n%s\nYAML:\n%s", tc.plugin, err, output, content)
				}
			})
		}
	})

	invalidCases := []struct {
		name       string
		plugin     string
		opts       map[string]any
		wantOutput string
	}{
		{name: "shadow-tls missing host", plugin: "shadow-tls", opts: map[string]any{"password": "plugin-secret", "version": 3}, wantOutput: "unset fields: host"},
		{name: "restls missing host", plugin: "restls", opts: map[string]any{"password": "plugin-secret", "version-hint": "tls13"}, wantOutput: "unset fields: host"},
		{name: "obfs invalid mode", plugin: "obfs", opts: map[string]any{"mode": "websocket"}, wantOutput: "obfs mode error"},
		{name: "v2ray-plugin invalid mode", plugin: "v2ray-plugin", opts: map[string]any{"mode": "http"}, wantOutput: "obfs mode error"},
	}
	t.Run("rejects invalid structures", func(t *testing.T) {
		for _, tc := range invalidCases {
			t.Run(tc.name, func(t *testing.T) {
				proxyName := "invalid-" + tc.plugin
				proxy := mihomoSSPluginProxy(proxyName, tc.plugin, tc.opts)
				content := mihomoSSPluginConfig(t, proxy, proxyName)
				output, err := runMihomoConfigTest(t, bin, content)
				if err == nil {
					t.Fatalf("Mihomo 1.19.29 应拒绝无效 %s 结构\n%s\nYAML:\n%s", tc.plugin, output, content)
				}
				if !strings.Contains(string(output), tc.wantOutput) {
					t.Fatalf("Mihomo 1.19.29 拒绝原因异常，期望包含 %q，实际:\n%s", tc.wantOutput, output)
				}
			})
		}
	})

	t.Run("legacy string still requires project self-check", func(t *testing.T) {
		proxyName := "legacy-obfs"
		proxy := mihomoSSPluginProxy(proxyName, "obfs-local;obfs=http", nil)
		content := mihomoSSPluginConfig(t, proxy, proxyName)
		if output, err := runMihomoConfigTest(t, bin, content); err != nil {
			t.Fatalf("固定回归要求 Mihomo 1.19.29 接受旧插件字符串，以证明内核检查不足: %v\n%s", err, output)
		}
		issues := CheckClashContent(content)
		for _, issue := range issues {
			if issue.Severity == "error" && issue.Path == "$.proxies[0].plugin" {
				return
			}
		}
		t.Fatalf("项目自检必须拒绝 Mihomo 可接受的旧插件字符串: %+v", issues)
	})
}

func mihomoSSPluginProxy(name, plugin string, opts map[string]any) gyaml.MapSlice {
	proxy := gyaml.MapSlice{
		{Key: "name", Value: name},
		{Key: "type", Value: "ss"},
		{Key: "server", Value: "192.0.2.1"},
		{Key: "port", Value: 443},
		{Key: "cipher", Value: "aes-128-gcm"},
		{Key: "password", Value: "secret"},
		{Key: "plugin", Value: plugin},
	}
	if opts != nil {
		proxy = append(proxy, gyaml.MapItem{Key: "plugin-opts", Value: opts})
	}
	return proxy
}

func mihomoSSPluginConfig(t *testing.T, proxy gyaml.MapSlice, proxyName string) []byte {
	t.Helper()
	root := gyaml.MapSlice{
		{Key: "mixed-port", Value: 7890},
		{Key: "proxies", Value: []any{proxy}},
		{Key: "proxy-groups", Value: []any{gyaml.MapSlice{
			{Key: "name", Value: "PROXY"}, {Key: "type", Value: "select"}, {Key: "proxies", Value: []string{proxyName}},
		}}},
		{Key: "rules", Value: []string{"MATCH,PROXY"}},
	}
	content, err := gyaml.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func runMihomoConfigTest(t *testing.T, bin string, content []byte) ([]byte, error) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return exec.Command(bin, "-t", "-f", configPath).CombinedOutput()
}
