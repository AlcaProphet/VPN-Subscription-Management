package assembly

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	gyaml "github.com/goccy/go-yaml"

	"vpn-sub/internal/node"
)

// 固定内核 WireGuard 夹具使用 32 字节 Base64 密钥；内容不参与校验，仅满足曲线密钥形状。
const (
	wgFixedPrivateKey = "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE="
	wgFixedPeerKey    = "AgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgI="
	wgFixedPeerKey2   = "AwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwM="
	wgFixedPSK        = "BwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwc="
)

// TestMihomo11931WireGuardProtocolStructures 覆盖 WireGuard adapter 在固定 Mihomo v1.19.31
// 上的代表性正反例；普通 go test 未设置外部二进制时跳过，严格入口由 .mihomo-test.sh 负责。
func TestMihomo11931WireGuardProtocolStructures(t *testing.T) {
	bin := os.Getenv("MIHOMO_11931_BIN")
	if bin == "" {
		t.Skip("未设置 MIHOMO_11931_BIN，跳过固定 Mihomo 1.19.31 WireGuard 验收")
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
			host   string
			port   int
			state  node.CurrentState
			params map[string]any
		}{
			{
				name: "single peer with reserved and ip stack", render: "fixed-wg-single",
				host: "192.0.2.1", port: 51820,
				state: node.CurrentState{Selectors: map[string]string{"peer_mode": "single"}},
				params: map[string]any{
					"private-key": wgFixedPrivateKey, "public-key": wgFixedPeerKey,
					"pre-shared-key": wgFixedPSK, "reserved": []int{1, 2, 3},
					"allowed-ips": []any{"0.0.0.0/0", "::/0"},
					"ip":          "192.0.2.2/32", "mtu": 1420, "workers": 4, "udp": true,
					"persistent-keepalive": 25,
					"ip-stack":             map[string]any{"mode": "gvisor", "congestion-controller": "bbr3"},
					"remote-dns-resolve":   true, "dns": []any{"1.1.1.1"},
				},
			},
			{
				name: "multi peer without top level endpoint", render: "fixed-wg-peers",
				host: "192.0.2.1", port: 51820,
				state: node.CurrentState{Selectors: map[string]string{"peer_mode": "peers"}},
				params: map[string]any{
					"private-key": wgFixedPrivateKey, "ip": "192.0.2.2/32", "ipv6": "2001:db8::2/128",
					"peers": []any{
						map[string]any{"server": "192.0.2.10", "port": 51820, "public-key": wgFixedPeerKey,
							"allowed-ips": []any{"10.0.0.0/24"}, "pre-shared-key": wgFixedPSK, "reserved": []int{7, 8, 9}},
						map[string]any{"server": "192.0.2.11", "port": 51821, "public-key": wgFixedPeerKey2,
							"allowed-ips": []any{"10.0.1.0/24"}},
					},
				},
			},
		}
		for _, tc := range generated {
			t.Run(tc.name, func(t *testing.T) {
				proxy := new(Service).clashProxy(&nodeData{
					Protocol: "wireguard", RenderName: tc.render, Host: tc.host, Port: tc.port,
					ProtocolJSON: tc.params, CurrentState: tc.state,
				})
				content := mihomoSSPluginConfig(t, orderedMapToMapSlice(proxy), tc.render)
				output, err := runMihomoConfigTest(t, bin, content)
				if err != nil {
					t.Fatalf("Mihomo 1.19.31 拒绝生成的 WireGuard 结构 %s: %v\n%s\nYAML:\n%s", tc.name, err, output, content)
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
				name: "wireguard reserved with two bytes",
				proxy: gyaml.MapSlice{
					{Key: "name", Value: "bad-wg-reserved"}, {Key: "type", Value: "wireguard"},
					{Key: "server", Value: "192.0.2.1"}, {Key: "port", Value: 51820},
					{Key: "private-key", Value: wgFixedPrivateKey}, {Key: "public-key", Value: wgFixedPeerKey},
					{Key: "ip", Value: "192.0.2.2/32"}, {Key: "reserved", Value: []any{1, 2}},
				},
				wantOutput: "invalid reserved value",
			},
			{
				name: "wireguard invalid private key base64",
				proxy: gyaml.MapSlice{
					{Key: "name", Value: "bad-wg-key"}, {Key: "type", Value: "wireguard"},
					{Key: "server", Value: "192.0.2.1"}, {Key: "port", Value: 51820},
					{Key: "private-key", Value: "not-base64!!"}, {Key: "public-key", Value: wgFixedPeerKey},
					{Key: "ip", Value: "192.0.2.2/32"},
				},
				wantOutput: "decode private key",
			},
			{
				name: "wireguard peer missing allowed ips",
				proxy: gyaml.MapSlice{
					{Key: "name", Value: "bad-wg-peer"}, {Key: "type", Value: "wireguard"},
					{Key: "private-key", Value: wgFixedPrivateKey}, {Key: "ip", Value: "192.0.2.2/32"},
					{Key: "peers", Value: []any{gyaml.MapSlice{
						{Key: "server", Value: "192.0.2.10"}, {Key: "port", Value: 51820},
						{Key: "public-key", Value: wgFixedPeerKey},
					}}},
				},
				wantOutput: "missing allowed_ips",
			},
			{
				name: "wireguard invalid ip stack mode",
				proxy: gyaml.MapSlice{
					{Key: "name", Value: "bad-wg-stack"}, {Key: "type", Value: "wireguard"},
					{Key: "server", Value: "192.0.2.1"}, {Key: "port", Value: 51820},
					{Key: "private-key", Value: wgFixedPrivateKey}, {Key: "public-key", Value: wgFixedPeerKey},
					{Key: "ip", Value: "192.0.2.2/32"},
					{Key: "ip-stack", Value: gyaml.MapSlice{{Key: "mode", Value: "system"}}},
				},
				wantOutput: "invalid IP stack mode",
			},
			{
				name: "wireguard missing local address",
				proxy: gyaml.MapSlice{
					{Key: "name", Value: "bad-wg-address"}, {Key: "type", Value: "wireguard"},
					{Key: "server", Value: "192.0.2.1"}, {Key: "port", Value: 51820},
					{Key: "private-key", Value: wgFixedPrivateKey}, {Key: "public-key", Value: wgFixedPeerKey},
				},
				wantOutput: "missing local address",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				content := mihomoSSPluginConfig(t, tc.proxy, "bad-"+strings.ReplaceAll(tc.name, " ", "-"))
				output, err := runMihomoConfigTest(t, bin, content)
				if err == nil {
					t.Fatalf("Mihomo 1.19.31 应拒绝无效 WireGuard 结构 %s\n%s", tc.name, content)
				}
				if tc.wantOutput != "" && !strings.Contains(string(output), tc.wantOutput) {
					t.Fatalf("Mihomo 1.19.31 拒绝原因异常，期望包含 %q，实际:\n%s", tc.wantOutput, output)
				}
			})
		}
	})
}
