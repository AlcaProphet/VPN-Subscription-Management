package assembly

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	gyaml "github.com/goccy/go-yaml"

	mierutp "github.com/enfein/mieru/v3/apis/trafficpattern"
	"github.com/enfein/mieru/v3/pkg/appctl/appctlpb"
	"google.golang.org/protobuf/proto"

	"vpn-sub/internal/node"
)

// mieruFixedTrafficPattern 用固定 tag 自身的编码器生成合法 Base64 TrafficPattern
// （TCP 分片启用 + 50ms 上限），项目测试不复制其 proto 定义。
func mieruFixedTrafficPattern() string {
	return mierutp.Encode(&appctlpb.TrafficPattern{
		TcpFragment: &appctlpb.TCPFragment{Enable: proto.Bool(true), MaxSleepMs: proto.Int32(50)},
	})
}

// TestMihomo11931MieruProtocolStructures 覆盖 Mieru adapter 在固定 Mihomo v1.19.31
// 上的代表性正反例；普通 go test 未设置外部二进制时跳过，严格入口由 .mihomo-test.sh 负责。
func TestMihomo11931MieruProtocolStructures(t *testing.T) {
	bin := os.Getenv("MIHOMO_11931_BIN")
	if bin == "" {
		t.Skip("未设置 MIHOMO_11931_BIN，跳过固定 Mihomo 1.19.31 Mieru 验收")
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
				name: "single port with complete enums", render: "fixed-mieru-single",
				host: "192.0.2.1", port: 8964,
				state: node.CurrentState{Selectors: map[string]string{"endpoint_mode": "single"}},
				params: map[string]any{
					"username": "mieru-user", "password": "pw-fixture", "transport": "TCP",
					"multiplexing": "MULTIPLEXING_HIGH", "handshake-mode": "HANDSHAKE_NO_WAIT",
					"traffic-pattern": mieruFixedTrafficPattern(), "udp": true,
				},
			},
			{
				name: "port range without top level port", render: "fixed-mieru-range",
				host: "192.0.2.1", port: 0,
				state: node.CurrentState{Selectors: map[string]string{"endpoint_mode": "range"}},
				params: map[string]any{
					"username": "mieru-user", "password": "pw-fixture", "transport": "UDP",
					"port-range": "1000-2000",
				},
			},
		}
		for _, tc := range generated {
			t.Run(tc.name, func(t *testing.T) {
				proxy := clashProxyFixture(t, &nodeData{
					Protocol: "mieru", RenderName: tc.render, Host: tc.host, Port: tc.port,
					ProtocolJSON: tc.params, CurrentState: tc.state,
				})
				content := mihomoSSPluginConfig(t, orderedMapToMapSlice(proxy), tc.render)
				output, err := runMihomoConfigTest(t, bin, content)
				if err != nil {
					t.Fatalf("Mihomo 1.19.31 拒绝生成的 Mieru 结构 %s: %v\n%s\nYAML:\n%s", tc.name, err, output, content)
				}
			})
		}
	})

	t.Run("rejects invalid structures", func(t *testing.T) {
		cases := []struct {
			name       string
			server     string
			port       int
			extra      gyaml.MapSlice
			wantOutput string
		}{
			{
				name: "port and port range together", server: "192.0.2.1", port: 8964,
				extra: gyaml.MapSlice{
					{Key: "port-range", Value: "1000-2000"}, {Key: "transport", Value: "TCP"},
					{Key: "username", Value: "u"}, {Key: "password", Value: "p"},
				},
				wantOutput: "port and port-range cannot be set at the same time",
			},
			{
				name: "unknown transport", server: "192.0.2.1", port: 8964,
				extra: gyaml.MapSlice{
					{Key: "transport", Value: "QUIC"}, {Key: "username", Value: "u"}, {Key: "password", Value: "p"},
				},
				wantOutput: "transport must be TCP or UDP",
			},
			{
				name: "legacy multiplexing value", server: "192.0.2.1", port: 8964,
				extra: gyaml.MapSlice{
					{Key: "transport", Value: "TCP"}, {Key: "username", Value: "u"}, {Key: "password", Value: "p"},
					{Key: "multiplexing", Value: "LOW"},
				},
				wantOutput: "invalid multiplexing level: LOW",
			},
			{
				name: "invalid traffic pattern", server: "192.0.2.1", port: 8964,
				extra: gyaml.MapSlice{
					{Key: "transport", Value: "TCP"}, {Key: "username", Value: "u"}, {Key: "password", Value: "p"},
					{Key: "traffic-pattern", Value: "!!!not-base64!!!"},
				},
				wantOutput: "failed to decode traffic pattern",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				proxy := gyaml.MapSlice{
					{Key: "name", Value: "bad-" + strings.ReplaceAll(tc.name, " ", "-")},
					{Key: "type", Value: "mieru"},
					{Key: "server", Value: tc.server},
				}
				if tc.port > 0 {
					proxy = append(proxy, gyaml.MapItem{Key: "port", Value: tc.port})
				}
				proxy = append(proxy, tc.extra...)
				content := mihomoSSPluginConfig(t, proxy, "bad-mieru")
				output, err := runMihomoConfigTest(t, bin, content)
				if err == nil {
					t.Fatalf("Mihomo 1.19.31 应拒绝无效 Mieru 结构 %s\n%s", tc.name, content)
				}
				if tc.wantOutput != "" && !strings.Contains(string(output), tc.wantOutput) {
					t.Fatalf("Mihomo 1.19.31 拒绝原因异常，期望包含 %q，实际:\n%s", tc.wantOutput, output)
				}
			})
		}
	})
}
