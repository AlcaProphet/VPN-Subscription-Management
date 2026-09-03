package assembly

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	gyaml "github.com/goccy/go-yaml"

	"vpn-sub/internal/log"
	"vpn-sub/internal/node"
)

func TestCanonicalEditorSecurityCheckSaveAndOutput(t *testing.T) {
	for _, protocol := range []string{"vless", "vmess"} {
		t.Run(protocol, func(t *testing.T) {
			svc, st, cfg := newTestService(t)
			nodeSvc := node.NewService(st, cfg, log.New("error", "console"))
			nodeSvc.SetCheckRenderer(svc.CheckNodeTarget)
			ctx := context.Background()
			created, err := nodeSvc.CreateManual(ctx, node.CreateManualInput{
				Name: "安全切换", Protocol: protocol, Host: "example.com", Port: 443,
				ProtocolJSON: map[string]any{"uuid": "editor-secret", "network": "tcp", "security": "tls"},
			})
			if err != nil {
				t.Fatal(err)
			}
			securities := []string{"tls", "none", "tls"}
			if protocol == "vless" {
				securities = append(securities, "reality", "none")
			}
			for i, security := range securities {
				// VMess SR 映射目前未显式输出 TLS 参数，作为独立核验项，不在 R27-04 改写。
				targets := []string{"clash-yaml", "generic-subs"}
				if protocol == "vless" {
					targets = append(targets, "sr-subs")
				}
				state := node.CurrentState{Network: "tcp", Security: security}
				params := map[string]any{"uuid": "", "network": "tcp", "security": security}
				if security == "reality" {
					params["reality-opts"] = map[string]any{"public-key": "public-key", "short-id": "abcd"}
				}
				var resets []string
				if i > 0 {
					resets = []string{"security"}
				}
				checked, err := nodeSvc.Check(ctx, node.CheckRequest{
					NodeID: created.ID, BaseRevision: created.EditRevision, Protocol: protocol, Host: "example.com", Port: 443,
					ProtocolJSON: params, CurrentState: &state, ResetScopes: resets,
					Targets: targets,
				})
				if err != nil {
					t.Fatal(err)
				}
				for target, result := range checked.Targets {
					if (result.Status != "ok" && result.Status != "warn") || result.Preview == nil {
						t.Fatalf("%s/%s 检查失败: %+v", security, target, result)
					}
					preview := *result.Preview
					switch {
					case target == "clash-yaml":
						var decoded struct {
							Proxies []map[string]any `yaml:"proxies"`
						}
						if err := gyaml.Unmarshal([]byte(preview), &decoded); err != nil {
							t.Fatal(err)
						}
						if len(decoded.Proxies) != 1 {
							t.Fatalf("节点数量错误: %s", preview)
						}
						proxy := decoded.Proxies[0]
						_, hasReality := proxy["reality-opts"]
						if (proxy["tls"] == true) != (security != "none") || hasReality != (security == "reality") {
							t.Fatalf("%s YAML 安全语义错误: %s", security, preview)
						}
						if _, exists := proxy["security"]; exists {
							t.Fatal("表单 security 泄漏到 YAML")
						}
					case target == "generic-subs" && protocol == "vmess":
						raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(preview, "vmess://"))
						if err != nil {
							t.Fatal(err)
						}
						var payload map[string]any
						if err := json.Unmarshal(raw, &payload); err != nil {
							t.Fatal(err)
						}
						if (payload["tls"] == "tls") != (security != "none") {
							t.Fatalf("VMess URI TLS 语义错误: %s", raw)
						}
					default:
						link, err := url.Parse(preview)
						if err != nil {
							t.Fatal(err)
						}
						q := link.Query()
						if target == "generic-subs" {
							want := security
							if want == "none" {
								want = ""
							}
							if q.Get("security") != want {
								t.Fatalf("标准 URI 安全语义错误: %s", preview)
							}
						} else if (q.Get("tls") == "1") != (security != "none") || (q.Get("xtls") == "2") != (security == "reality") {
							t.Fatalf("SR URI 安全语义错误: %s", preview)
						}
					}
				}
				updated, err := nodeSvc.UpdateManual(ctx, created.ID, node.UpdateManualInput{
					BaseRevision: created.EditRevision, Protocol: protocol, Host: "example.com", Port: 443,
					ProtocolJSON: params, CurrentState: &state, ResetScopes: resets,
				})
				if err != nil {
					t.Fatal(err)
				}
				if updated.CurrentState.Security != security || (updated.ProtocolJSON["tls"] == true) != (security != "none") {
					t.Fatalf("保存后的安全状态与检查不同: %+v", updated)
				}
				created = updated
			}
		})
	}
}

func TestClashOutputDropsDisabledFeatureParameters(t *testing.T) {
	for _, protocol := range []string{"ss", "vless", "vmess"} {
		t.Run(protocol, func(t *testing.T) {
			params := map[string]any{"uuid": "uuid", "network": "tcp"}
			if protocol == "ss" {
				params = map[string]any{"cipher": "aes-128-gcm", "password": "secret"}
			}
			params["smux"] = map[string]any{"enabled": false, "max-connections": 7, "future": "old",
				"brutal-opts": map[string]any{"enabled": true, "up": "100 Mbps"}}
			svc := &Service{}
			result, err := svc.CheckNodeTarget(context.Background(), "clash-yaml", protocol, "feature-node", "example.com", 443, params)
			if err != nil {
				t.Fatal(err)
			}
			var decoded struct {
				Proxies []map[string]any `yaml:"proxies"`
			}
			if err := gyaml.Unmarshal([]byte(result.Preview), &decoded); err != nil {
				t.Fatal(err)
			}
			if len(decoded.Proxies) != 1 || !reflect.DeepEqual(decoded.Proxies[0]["smux"], map[string]any{"enabled": false}) {
				t.Fatalf("输出残留已关闭参数: %s", result.Preview)
			}
		})
	}
}

func TestNodeCheckFixtures(t *testing.T) {
	fixtures := []struct {
		name       string
		invalid    bool
		warnURI    bool
		skipURI    bool
		expectCode string
	}{
		{name: "vless-tcp-tls.json"},
		{name: "vless-ws-tls.json"},
		{name: "vless-reality.json"},
		{name: "vless-xhttp-risk.json", invalid: true, expectCode: "invalid_node_draft"},
		{name: "vmess-tcp.json"},
		{name: "vmess-ws-tls.json"},
		{name: "vmess-cipher-risk.json", warnURI: true, expectCode: "uri_algorithm_rewrite"},
		{name: "trojan-tcp-tls.json"},
		{name: "trojan-ws-tls.json", skipURI: true, expectCode: "core_semantic_unexpressible"},
		{name: "trojan-grpc-tls.json", skipURI: true, expectCode: "core_semantic_unexpressible"},
		{name: "trojan-inner-ss.json", skipURI: true, expectCode: "core_semantic_unexpressible"},
		{name: "ss-aes-gcm.json"},
		{name: "ss-obfs.json", warnURI: true, expectCode: "plugin_name_mapping"},
		{name: "ss-v2ray-plugin.json"},
		{name: "ss-2022-pending.json", warnURI: true, expectCode: "unverified_compatibility"},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			svc, st, cfg := newTestService(t)
			nodeSvc := node.NewService(st, cfg, log.New("error", "console"))
			nodeSvc.SetCheckRenderer(svc.CheckNodeTarget)
			raw, err := os.ReadFile(filepath.Join("testdata", "node_check", fixture.name))
			if err != nil {
				t.Fatalf("读取固定夹具失败: %v", err)
			}
			var req node.CheckRequest
			if err := json.Unmarshal(raw, &req); err != nil {
				t.Fatalf("解析固定夹具失败: %v", err)
			}
			resp, err := nodeSvc.Check(context.Background(), req)
			if err != nil {
				t.Fatalf("执行固定夹具检查失败: %v", err)
			}
			encoded, err := json.Marshal(resp)
			if err != nil {
				t.Fatalf("序列化固定夹具响应失败: %v", err)
			}
			for _, secret := range []string{"trojan-password", "inner-password", "shadowsocks-password", "shadowsocks-2022-password", "11111111-2222-3333-4444-555555555555"} {
				if strings.Contains(string(encoded), secret) {
					t.Fatalf("固定夹具响应泄漏凭据 %q: %s", secret, encoded)
				}
			}
			if fixture.invalid {
				assertFixtureDiagnostic(t, resp.Targets["generic-subs"], "error", fixture.expectCode)
				return
			}
			assertFixtureStatus(t, resp.Targets["clash-yaml"], "ok")
			if fixture.skipURI {
				assertFixtureStatus(t, resp.Targets["sr-subs"], "skip")
				assertFixtureStatus(t, resp.Targets["generic-subs"], "skip")
				assertFixtureDiagnostic(t, resp.Targets["sr-subs"], "error", fixture.expectCode)
				if resp.Targets["sr-subs"].Preview != nil || resp.Targets["generic-subs"].Preview != nil {
					t.Fatal("不可表达的 URI 目标不应返回可能静默遗漏参数的预览")
				}
				return
			}
			if fixture.warnURI {
				assertFixtureStatus(t, resp.Targets["sr-subs"], "warn")
				assertFixtureStatus(t, resp.Targets["generic-subs"], "warn")
				assertFixtureDiagnostic(t, resp.Targets["sr-subs"], "warn", fixture.expectCode)
				assertFixtureDiagnostic(t, resp.Targets["generic-subs"], "warn", fixture.expectCode)
				return
			}
			assertFixtureStatus(t, resp.Targets["sr-subs"], "ok")
			assertFixtureStatus(t, resp.Targets["generic-subs"], "ok")
		})
	}
}

func assertFixtureStatus(t *testing.T, result node.TargetCheckResult, status string) {
	t.Helper()
	if result.Status != status || (status != "skip" && result.Preview == nil) {
		t.Fatalf("固定夹具目标状态异常: want=%s got=%+v", status, result)
	}
}

func assertFixtureDiagnostic(t *testing.T, result node.TargetCheckResult, severity, code string) {
	t.Helper()
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Severity == severity && diagnostic.Code == code && diagnostic.FieldPath != "" && diagnostic.Evidence != "" {
			return
		}
	}
	t.Fatalf("固定夹具缺少诊断: want severity=%s code=%s got=%+v", severity, code, result.Diagnostics)
}

func TestNodeCheckTrojanCustomTransportDiagnosedByTarget(t *testing.T) {
	svc, st, cfg := newTestService(t)
	nodeSvc := node.NewService(st, cfg, log.New("error", "console"))
	nodeSvc.SetCheckRenderer(svc.CheckNodeTarget)
	resp, err := nodeSvc.Check(context.Background(), node.CheckRequest{
		Protocol: "trojan", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{
			"password": "trojan-secret", "network": "h2",
		},
		CurrentState: &node.CurrentState{Network: "h2", Security: "tls", Features: []string{}},
		Targets:      []string{"clash-yaml", "sr-subs"},
	})
	if err != nil {
		t.Fatalf("Trojan h2 自定义传输检查失败: %v", err)
	}
	clash := resp.Targets["clash-yaml"]
	if clash.Status != "warn" {
		t.Fatalf("Clash 目标应对 Trojan 自定义传输给出降级警告: %+v", clash)
	}
	assertFixtureDiagnostic(t, clash, "warn", "trojan_transport_fallback")
	sr := resp.Targets["sr-subs"]
	if sr.Status != "skip" {
		t.Fatalf("SR 目标应跳过无法表达的 Trojan 自定义传输: %+v", sr)
	}
	assertFixtureDiagnostic(t, sr, "error", "core_semantic_unexpressible")
}

func TestNodeCheckUnknownPluginDiagnosed(t *testing.T) {
	svc, st, cfg := newTestService(t)
	nodeSvc := node.NewService(st, cfg, log.New("error", "console"))
	nodeSvc.SetCheckRenderer(svc.CheckNodeTarget)
	plugin := "unknown-plugin"
	resp, err := nodeSvc.Check(context.Background(), node.CheckRequest{
		Protocol: "ss", Host: "example.com", Port: 8388,
		ProtocolJSON: map[string]any{
			"cipher": "aes-256-gcm", "password": "p", "plugin": plugin,
		},
		CurrentState: &node.CurrentState{Security: "none", Plugin: &plugin, Features: []string{}},
		Targets:      []string{"sr-subs"},
	})
	if err != nil {
		t.Fatalf("未知插件检查失败: %v", err)
	}
	assertFixtureDiagnostic(t, resp.Targets["sr-subs"], "warn", "plugin_no_verified_mapping")
}
