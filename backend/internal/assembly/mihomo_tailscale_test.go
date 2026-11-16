package assembly

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gyaml "github.com/goccy/go-yaml"

	"vpn-sub/internal/node"
)

// runMihomoConfigTestInHome 使用独立 -d 目录执行固定内核配置检查，避免污染工作区。
func runMihomoConfigTestInHome(t *testing.T, bin string, content []byte) ([]byte, string, error) {
	t.Helper()
	home := t.TempDir()
	configPath := filepath.Join(home, "config.yaml")
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(bin, "-t", "-d", home, "-f", configPath).CombinedOutput()
	return output, home, err
}

// TestMihomo11931TailscaleProtocolStructures 覆盖 Tailscale adapter 在固定 Mihomo v1.19.31
// 上的代表性正反例；普通 go test 未设置外部二进制时跳过，严格入口由 .mihomo-test.sh 负责。
func TestMihomo11931TailscaleProtocolStructures(t *testing.T) {
	bin := os.Getenv("MIHOMO_11931_BIN")
	if bin == "" {
		t.Skip("未设置 MIHOMO_11931_BIN，跳过固定 Mihomo 1.19.31 Tailscale 验收")
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
			nodeID int64
			state  node.CurrentState
			params map[string]any
		}{
			{
				name: "saved node with derived state dir", nodeID: 1,
				params: map[string]any{
					"hostname": "node-a", "auth-key": "ts-fixture-secret",
					"control-url": "https://headscale.example.com", "ephemeral": true, "udp": true,
					"accept-routes": true, "exit-node": "100.64.0.1", "exit-node-allow-lan-access": false,
				},
			},
			{
				name: "auto exit node without lan access",
				params: map[string]any{
					"hostname": "node-b", "auth-key": "ts-fixture-secret", "exit-node": "auto:any",
				},
			},
		}
		for _, tc := range generated {
			t.Run(tc.name, func(t *testing.T) {
				res, err := new(Service).CheckNodeTargetDraft(t.Context(), node.CheckTargetDraft{
					Target: "clash-yaml", Protocol: "tailscale", RenderName: "fixed-" + strings.ReplaceAll(tc.name, " ", "-"),
					Params: tc.params, NodeID: tc.nodeID, Persisted: tc.nodeID > 0, State: tc.state,
				})
				if err != nil {
					t.Fatalf("Tailscale 检查失败: %v", err)
				}
				content := []byte(res.Preview)
				output, home, err := runMihomoConfigTestInHome(t, bin, content)
				if err != nil {
					t.Fatalf("Mihomo 1.19.31 拒绝生成的 Tailscale 结构 %s: %v\n%s\nYAML:\n%s", tc.name, err, output, content)
				}
				// 配置检查阶段不得创建 state-dir 目录。
				if entries, readErr := os.ReadDir(home); readErr == nil {
					for _, entry := range entries {
						if entry.Name() == "tailscale" {
							t.Fatalf("配置检查不应急于创建 Tailscale 状态目录: %s", home)
						}
					}
				}
			})
		}
	})

	t.Run("rejects unsafe state dir", func(t *testing.T) {
		proxy := gyaml.MapSlice{
			{Key: "name", Value: "bad-tailscale-state"}, {Key: "type", Value: "tailscale"},
			{Key: "hostname", Value: "node-a"}, {Key: "state-dir", Value: "../escape"},
		}
		content := mihomoSSPluginConfig(t, proxy, "bad-tailscale-state")
		output, _, err := runMihomoConfigTestInHome(t, bin, content)
		if err == nil {
			t.Fatalf("Mihomo 1.19.31 应拒绝逃出受控目录的 state-dir\n%s", content)
		}
		if !strings.Contains(string(output), "safe") && !strings.Contains(string(output), "path") {
			t.Fatalf("Mihomo 1.19.31 拒绝原因异常:\n%s", output)
		}
	})
}
