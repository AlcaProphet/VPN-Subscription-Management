package assembly

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	gyaml "github.com/goccy/go-yaml"

	"vpn-sub/internal/node"
)

// TestMihomo11931AnyTLSProtocolStructures 覆盖 AnyTLS adapter 在固定 Mihomo v1.19.31
// 上的代表性正反例；普通 go test 未设置外部二进制时跳过，严格入口由 .mihomo-test.sh 负责。
func TestMihomo11931AnyTLSProtocolStructures(t *testing.T) {
	bin := os.Getenv("MIHOMO_11931_BIN")
	if bin == "" {
		t.Skip("未设置 MIHOMO_11931_BIN，跳过固定 Mihomo 1.19.31 AnyTLS 验收")
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
			state  node.CurrentState
			params map[string]any
		}{
			{
				name:  "plain with session tuning",
				state: node.CurrentState{Selectors: map[string]string{"security_mode": "plain"}},
				params: map[string]any{
					"password": "anytls-fixture-secret", "sni": "example.com", "alpn": []any{"h2"},
					"client-fingerprint": "chrome", "skip-cert-verify": true, "udp": true,
					"idle-session-check-interval": 30, "idle-session-timeout": 60, "min-idle-session": 2,
				},
			},
			{
				name:  "shadow tls camouflage",
				state: node.CurrentState{Selectors: map[string]string{"security_mode": "shadow_tls"}},
				params: map[string]any{
					"password": "anytls-fixture-secret", "sni": "example.com",
					"shadow-tls-opts": map[string]any{"password": "shadow-fixture-secret", "version": "3"},
				},
			},
			{
				name:  "jls camouflage",
				state: node.CurrentState{Selectors: map[string]string{"security_mode": "jls"}},
				params: map[string]any{
					"password": "anytls-fixture-secret", "sni": "example.com",
					"jls-opts": map[string]any{"username": "jls-user", "password": "jls-fixture-secret"},
				},
			},
		}
		for _, tc := range generated {
			t.Run(tc.name, func(t *testing.T) {
				proxy := clashProxyFixture(t, &nodeData{
					Protocol: "anytls", RenderName: "fixed-anytls-" + strings.ReplaceAll(tc.name, " ", "-"),
					Host: "192.0.2.1", Port: 443, ProtocolJSON: tc.params, CurrentState: tc.state,
				})
				render := "fixed-anytls-" + strings.ReplaceAll(tc.name, " ", "-")
				content := mihomoSSPluginConfig(t, orderedMapToMapSlice(proxy), render)
				output, err := runMihomoConfigTest(t, bin, content)
				if err != nil {
					t.Fatalf("Mihomo 1.19.31 拒绝生成的 AnyTLS 结构 %s: %v\n%s\nYAML:\n%s", tc.name, err, output, content)
				}
			})
		}
	})

	t.Run("rejects invalid structures", func(t *testing.T) {
		cases := []struct {
			name       string
			extra      gyaml.MapSlice
			wantOutput string
		}{
			{
				name: "shadow tls unknown version",
				extra: gyaml.MapSlice{
					{Key: "password", Value: "pw"}, {Key: "sni", Value: "example.com"},
					{Key: "shadow-tls-opts", Value: gyaml.MapSlice{
						{Key: "password", Value: "shadow"}, {Key: "version", Value: 9}}},
				},
				wantOutput: "unknown protocol version",
			},
			{
				name: "restls invalid version hint",
				extra: gyaml.MapSlice{
					{Key: "password", Value: "pw"}, {Key: "sni", Value: "example.com"},
					{Key: "restls-opts", Value: gyaml.MapSlice{
						{Key: "password", Value: "restls"}, {Key: "version-hint", Value: "tls11"}}},
				},
				wantOutput: "invalid version hint",
			},
			{
				name: "jls missing username",
				extra: gyaml.MapSlice{
					{Key: "password", Value: "pw"}, {Key: "sni", Value: "example.com"},
					{Key: "jls-opts", Value: gyaml.MapSlice{{Key: "password", Value: "jls"}}},
				},
				wantOutput: "has unset fields: username",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				proxy := gyaml.MapSlice{
					{Key: "name", Value: "bad-" + strings.ReplaceAll(tc.name, " ", "-")},
					{Key: "type", Value: "anytls"},
					{Key: "server", Value: "192.0.2.1"}, {Key: "port", Value: 443},
				}
				proxy = append(proxy, tc.extra...)
				content := mihomoSSPluginConfig(t, proxy, "bad-anytls")
				output, err := runMihomoConfigTest(t, bin, content)
				if err == nil {
					t.Fatalf("Mihomo 1.19.31 应拒绝无效 AnyTLS 结构 %s\n%s", tc.name, content)
				}
				if tc.wantOutput != "" && !strings.Contains(string(output), tc.wantOutput) {
					t.Fatalf("Mihomo 1.19.31 拒绝原因异常，期望包含 %q，实际 (%d 字节):\n%s", tc.wantOutput, len(output), string(output))
				}
			})
		}
	})
}
