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

	assemblylinks "vpn-sub/internal/assembly/links"
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
				targets := []string{"clash-yaml", "generic-subs", "sr-subs"}
				state := node.CurrentState{Network: "tcp", Security: security}
				params := map[string]any{"uuid": "", "network": "tcp", "security": security}
				if security != "none" {
					params["servername"] = "sni.example.com"
					params["alpn"] = []string{"h2", "http/1.1"}
					params["client-fingerprint"] = "chrome"
				}
				if security == "tls" {
					params["skip-cert-verify"] = true
				}
				if protocol == "vless" && security != "none" {
					params["flow"] = "xtls-rprx-vision"
				}
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
						if _, exists := payload["skip-cert-verify"]; exists {
							t.Fatalf("generic VMess 不应输出 skip-cert-verify: %s", raw)
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
						} else {
							if (q.Get("tls") == "1") != (security != "none") || (q.Get("xtls") == "2") != (security == "reality") {
								t.Fatalf("SR URI 安全语义错误: %s", preview)
							}
							if security != "none" && (q.Get("peer") != "sni.example.com" || q.Get("alpn") != "h2,http/1.1" || q.Get("fp") != "chrome") {
								t.Fatalf("SR URI TLS 身份参数错误: %s", preview)
							}
							if (q.Get("allowInsecure") == "1") != (security == "tls") {
								t.Fatalf("SR URI skip-cert-verify 语义错误: %s", preview)
							}
							if protocol == "vless" && security != "none" && q.Get("flow") != "xtls-rprx-vision" {
								t.Fatalf("SR VLESS Flow 缺失: %s", preview)
							}
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
			// Step 20 起显式 adapter 只输出活动字段：未启用的 smux 连同其子参数都不得残留。
			if len(decoded.Proxies) != 1 {
				t.Fatalf("检查预览代理数量异常: %s", result.Preview)
			}
			if _, ok := decoded.Proxies[0]["smux"]; ok {
				t.Fatalf("输出残留已关闭的 smux 参数: %s", result.Preview)
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
		warnClash  string
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
		{name: "ss-obfs.json", warnURI: true, expectCode: "plugin_partial_mapping"},
		{name: "ss-v2ray-plugin.json", warnURI: true, expectCode: "plugin_partial_mapping"},
		{name: "ss-2022-pending.json", warnURI: true, expectCode: "unverified_compatibility"},
		{name: "http-basic-tls.json", warnURI: true, expectCode: "uri_partial_fields"},
		{name: "http-mtls.json", skipURI: true, expectCode: "core_semantic_unexpressible"},
		{name: "socks5-basic-tls.json"},
		{name: "socks5-mtls.json", skipURI: true, expectCode: "core_semantic_unexpressible"},
		{name: "ssh-password.json", skipURI: true, expectCode: "target_unsupported"},
		{name: "ssh-private-key.json", skipURI: true, warnClash: "ssh_host_key_unverified", expectCode: "target_unsupported"},
		{name: "snell-http.json", skipURI: true, expectCode: "target_unsupported"},
		{name: "snell-shadow-tls.json", skipURI: true, expectCode: "target_unsupported"},
		{name: "hysteria-basic.json"},
		{name: "hysteria-auth-str.json", skipURI: true, expectCode: "core_semantic_unexpressible"},
		{name: "hysteria2-single.json"},
		{name: "hysteria2-ports.json", skipURI: true, expectCode: "core_semantic_unexpressible"},
		{name: "hysteria2-realm.json", skipURI: true, expectCode: "core_semantic_unexpressible"},
		{name: "tuic-v5.json"},
		{name: "tuic-v4.json", skipURI: true, expectCode: "core_semantic_unexpressible"},
		{name: "wireguard-single.json", warnURI: true, expectCode: "uri_partial_fields"},
		{name: "wireguard-peers.json", skipURI: true, expectCode: "core_semantic_unexpressible"},
		{name: "mieru-single.json", skipURI: true, expectCode: "target_unsupported"},
		{name: "mieru-range.json", skipURI: true, expectCode: "target_unsupported"},
		{name: "masque-quic.json", skipURI: true, expectCode: "target_unsupported"},
		{name: "masque-l4proxy.json", skipURI: true, expectCode: "target_unsupported"},
		{name: "tailscale-draft.json", skipURI: true, expectCode: "target_unsupported"},
		{name: "tailscale-no-auth.json", skipURI: true, warnClash: "tailscale_auth_key_interactive_login", expectCode: "target_unsupported"},
		{name: "anytls-plain.json"},
		{name: "anytls-shadow-tls.json", skipURI: true, expectCode: "core_semantic_unexpressible"},
		{name: "shadowquic-basic.json", skipURI: true, warnClash: "shadowquic_zero_rtt_replay_risk", expectCode: "target_unsupported"},
		{name: "trusttunnel-connections.json", skipURI: true, expectCode: "target_unsupported"},
		{name: "trusttunnel-streams.json", skipURI: true, expectCode: "target_unsupported"},
		{name: "openvpn-userpass.json", skipURI: true, expectCode: "target_unsupported"},
		{name: "openvpn-cert-userpass.json", skipURI: true, expectCode: "target_unsupported"},
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
			for _, secret := range []string{"trojan-password", "inner-password", "shadowsocks-password", "shadowsocks-2022-password", "http-password", "socks-password", "ssh-password", "11111111-2222-3333-4444-555555555555", "MC4CAQAwBQYDK2VwBCIEIEt4q5YVLyauvDa3VGquPz1AG5LyzbQQzIEFaSeIDjZl", "snell-password", "snell-obfs-password", "hysteria-auth-str", "hysteria2-password", "hysteria2-obfs-password", "hysteria2-realm-token", "tuic-v5-password", "tuic-v4-token",
				"d2lyZWd1YXJkLXByaXZhdGUta2V5LWZpeHR1cmUtMzI=", "d2lyZWd1YXJkLXBzay1maXh0dXJlLXNlY3JldC0zMng=",
				"mieru-password", "ts-fixture-auth-secret",
				"anytls-plain-password", "anytls-shadow-password", "anytls-shadow-tls-secret",
				"shadowquic-password", "trusttunnel-password", "trusttunnel-private-key",
				"openvpn-password", "openvpn-private-key", "openvpn-tls-auth", "openvpn-tls-crypt"} {
				if strings.Contains(string(encoded), secret) {
					t.Fatalf("固定夹具响应泄漏凭据 %q: %s", secret, encoded)
				}
			}
			if fixture.invalid {
				assertFixtureDiagnostic(t, resp.Targets["generic-subs"], "error", fixture.expectCode)
				return
			}
			if fixture.warnClash != "" {
				assertFixtureStatus(t, resp.Targets["clash-yaml"], "warn")
				assertFixtureDiagnostic(t, resp.Targets["clash-yaml"], "warn", fixture.warnClash)
			} else {
				assertFixtureStatus(t, resp.Targets["clash-yaml"], "ok")
			}
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
		Targets:      []string{"clash-yaml", "sr-subs", "generic-subs"},
	})
	if err != nil {
		t.Fatalf("未知插件检查失败: %v", err)
	}
	for _, target := range []string{"clash-yaml", "sr-subs"} {
		assertFixtureStatus(t, resp.Targets[target], "warn")
		assertFixtureDiagnostic(t, resp.Targets[target], "warn", "plugin_no_verified_mapping")
	}
	assertFixtureStatus(t, resp.Targets["generic-subs"], "skip")
	assertFixtureDiagnostic(t, resp.Targets["generic-subs"], "error", "core_semantic_unexpressible")
	if resp.Targets["generic-subs"].Preview != nil {
		t.Fatal("generic 目标不得为未知插件返回预览")
	}
}

func TestSSPluginTargetDiagnosticsMatrix(t *testing.T) {
	tests := []struct {
		name       string
		plugin     string
		storageKey string
		opts       map[string]any
		srCode     string
	}{
		{name: "shadow-tls", plugin: "shadow-tls", storageKey: "shadow-tls-opts", opts: map[string]any{"host": "cdn.example.com", "password": "shadow-secret", "version": float64(3)}, srCode: "unverified_compatibility"},
		{name: "restls", plugin: "restls", storageKey: "restls-opts", opts: map[string]any{"host": "cdn.example.com", "password": "restls-secret", "version-hint": "tls13"}, srCode: "unverified_compatibility"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, st, cfg := newTestService(t)
			nodeSvc := node.NewService(st, cfg, log.New("error", "console"))
			nodeSvc.SetCheckRenderer(svc.CheckNodeTarget)
			resp, err := nodeSvc.Check(context.Background(), node.CheckRequest{
				Protocol: "ss", Host: "example.com", Port: 8388,
				ProtocolJSON: map[string]any{
					"cipher": "aes-256-gcm", "password": "main-secret", "plugin": tc.plugin,
					tc.storageKey: tc.opts,
				},
				Targets: []string{"clash-yaml", "sr-subs", "generic-subs"},
			})
			if err != nil {
				t.Fatal(err)
			}
			assertFixtureStatus(t, resp.Targets["clash-yaml"], "ok")
			assertFixtureStatus(t, resp.Targets["sr-subs"], "warn")
			assertFixtureDiagnostic(t, resp.Targets["sr-subs"], "warn", tc.srCode)
			assertFixtureStatus(t, resp.Targets["generic-subs"], "skip")
			assertFixtureDiagnostic(t, resp.Targets["generic-subs"], "error", "core_semantic_unexpressible")
			if resp.Targets["generic-subs"].Preview != nil {
				t.Fatal("generic 目标不得为不支持的插件返回预览")
			}
		})
	}
}

func TestSSPluginTargetErrorsUsePreciseCodesAndPaths(t *testing.T) {
	tests := []struct {
		name     string
		params   map[string]any
		target   string
		status   string
		wantCode string
		wantPath string
	}{
		{
			name: "shape", target: "clash-yaml", status: "error", wantCode: "ss_plugin_shape_invalid", wantPath: "v2ray-plugin-opts",
			params: map[string]any{"cipher": "aes-256-gcm", "password": "secret", "plugin": "v2ray-plugin", "v2ray-plugin-opts": "bad"},
		},
		{
			name: "missing", target: "clash-yaml", status: "error", wantCode: "ss_plugin_required_field_missing", wantPath: "restls-opts.host",
			params: map[string]any{"cipher": "aes-256-gcm", "password": "secret", "plugin": "restls", "restls-opts": map[string]any{"password": "restls-secret", "version-hint": "tls13"}},
		},
		{
			name: "invalid-mode", target: "clash-yaml", status: "error", wantCode: "plugin_option_unexpressible", wantPath: "obfs-opts.mode",
			params: map[string]any{"cipher": "aes-256-gcm", "password": "secret", "plugin": "obfs", "obfs-opts": map[string]any{"mode": "quic"}},
		},
		{
			name: "uri-field", target: "sr-subs", status: "skip", wantCode: "plugin_option_unexpressible", wantPath: "v2ray-plugin-opts.headers",
			params: map[string]any{"cipher": "aes-256-gcm", "password": "secret", "plugin": "v2ray-plugin", "v2ray-plugin-opts": map[string]any{"mode": "websocket", "headers": map[string]any{"X-Test": "value"}}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, st, cfg := newTestService(t)
			nodeSvc := node.NewService(st, cfg, log.New("error", "console"))
			nodeSvc.SetCheckRenderer(svc.CheckNodeTarget)
			resp, err := nodeSvc.Check(context.Background(), node.CheckRequest{
				Protocol: "ss", Host: "example.com", Port: 8388, ProtocolJSON: tc.params, Targets: []string{tc.target},
			})
			if err != nil {
				t.Fatal(err)
			}
			result := resp.Targets[tc.target]
			if result.Status != tc.status || len(result.Diagnostics) == 0 || !hasDiagnosticAt(result.Diagnostics, "error", tc.wantCode, tc.wantPath) || result.Preview != nil {
				t.Fatalf("SS 插件目标错误不精确: want=%s/%s got=%+v", tc.wantCode, tc.wantPath, result)
			}
		})
	}
}

func TestSSPluginDiagnosticsDoNotMaskUnrelatedDraftValidation(t *testing.T) {
	svc, st, cfg := newTestService(t)
	nodeSvc := node.NewService(st, cfg, log.New("error", "console"))
	nodeSvc.SetCheckRenderer(svc.CheckNodeTarget)
	resp, err := nodeSvc.Check(context.Background(), node.CheckRequest{
		Protocol: "ss", Host: "example.com", Port: 8388,
		ProtocolJSON: map[string]any{
			"password": "secret", "plugin": "shadow-tls",
			"shadow-tls-opts": map[string]any{"host": "cdn.example.com", "password": "shadow-secret"},
		},
		Targets: []string{"generic-subs"},
	})
	if err != nil {
		t.Fatal(err)
	}
	result := resp.Targets["generic-subs"]
	if result.Status != "error" || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "invalid_node_draft" || result.Diagnostics[0].FieldPath != "cipher" {
		t.Fatalf("插件目标不支持不得掩盖无关草稿错误: %+v", result)
	}
}

func hasDiagnosticAt(diagnostics []node.TargetDiagnostic, severity, code, path string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == severity && diagnostic.Code == code && diagnostic.FieldPath == path {
			return true
		}
	}
	return false
}

// ===== Build32 Step 20：URI 能力矩阵、canonical field path 与检查零落库 =====

// step20URISupported / step20URIUnsupported 是 Build32 冻结的 URI 能力集合。
var (
	step20URISupported = []string{
		"ss", "vmess", "vless", "trojan", "anytls",
		"hysteria", "hysteria2", "tuic", "wireguard", "http", "socks5",
	}
	step20URIUnsupported = []string{
		"snell", "mieru", "masque", "openvpn", "ssh", "shadowquic", "trusttunnel", "tailscale",
	}
)

// TestURICapabilityMatrixMatchesFixedProtocolSets 断言 URI 能力集合与 manual 清单完全对齐：
// 11 个具备映射、8 个稳定 skip，两者并集恰好是 19 个 manual 协议。
func TestURICapabilityMatrixMatchesFixedProtocolSets(t *testing.T) {
	manual := map[string]bool{}
	for _, protocol := range node.ManualProtocols() {
		manual[protocol.Protocol] = true
	}
	if len(manual) != 19 {
		t.Fatalf("manual 协议清单必须为 19 项，实际 %d", len(manual))
	}
	covered := map[string]bool{}
	for _, protocol := range step20URISupported {
		if !assemblylinks.SupportsURI(protocol) {
			t.Fatalf("协议 %s 应具备 URI 映射", protocol)
		}
		covered[protocol] = true
	}
	for _, protocol := range step20URIUnsupported {
		if assemblylinks.SupportsURI(protocol) {
			t.Fatalf("协议 %s 不应具备 URI 映射", protocol)
		}
		covered[protocol] = true
	}
	for protocol := range manual {
		if !covered[protocol] {
			t.Fatalf("协议 %s 未登记在 URI 能力矩阵中", protocol)
		}
	}
	for protocol := range covered {
		if !manual[protocol] {
			t.Fatalf("URI 能力矩阵包含非 manual 协议 %s", protocol)
		}
	}
	if len(covered) != len(manual) {
		t.Fatalf("URI 能力矩阵与 manual 清单不一致: covered=%d manual=%d", len(covered), len(manual))
	}
}

// TestUnsupportedURITargetsReturnStableSkipWithoutPreview 断言 8 个无 URI 映射的协议
// 对两个 URI 目标都返回稳定 skip、target_unsupported、空预览，且不返回错误。
func TestUnsupportedURITargetsReturnStableSkipWithoutPreview(t *testing.T) {
	svc := &Service{}
	for _, protocol := range step20URIUnsupported {
		for _, target := range []string{"sr-subs", "generic-subs"} {
			t.Run(protocol+"/"+target, func(t *testing.T) {
				res, err := svc.CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
					Target: target, Protocol: protocol, RenderName: protocol + "-node",
					Host: "example.com", Port: 443,
				})
				if err != nil {
					t.Fatalf("不支持 URI 的协议不得返回错误: %v", err)
				}
				if res.Status != "skip" {
					t.Fatalf("无 URI 映射必须稳定 skip，实际 %q", res.Status)
				}
				if res.Preview != "" {
					t.Fatalf("skip 不得返回任何预览: %s", res.Preview)
				}
				found := false
				for _, diagnostic := range res.Diagnostics {
					if diagnostic.Code == "target_unsupported" && diagnostic.Severity == "error" {
						found = true
					}
				}
				if !found {
					t.Fatalf("缺少 target_unsupported 诊断: %+v", res.Diagnostics)
				}
			})
		}
	}
}

// TestFirstBatchURIDiagnosticsDeclarePerTargetGaps 断言 SR 与 generic 表达能力不同时
// 必须分别判断：能无损表达就不产生诊断，不能表达的活动字段必须给出稳定 partial code。
func TestFirstBatchURIDiagnosticsDeclarePerTargetGaps(t *testing.T) {
	cases := []struct {
		name      string
		protocol  string
		target    string
		params    map[string]any
		fieldPath string
		wantGap   bool
	}{
		{"vmess SR 可表达 skip-cert-verify", "vmess", "sr-subs",
			map[string]any{"uuid": "u", "tls": true, "servername": "example.com", "skip-cert-verify": true}, "skip-cert-verify", false},
		{"vmess generic 不表达 skip-cert-verify", "vmess", "generic-subs",
			map[string]any{"uuid": "u", "tls": true, "servername": "example.com", "skip-cert-verify": true}, "skip-cert-verify", true},
		{"vmess 高级字段两侧都不表达", "vmess", "sr-subs",
			map[string]any{"uuid": "u", "packet-addr": true}, "packet-addr", true},
		{"ss SR 不表达 UDP over TCP", "ss", "sr-subs",
			map[string]any{"cipher": "aes-256-gcm", "password": "p", "udp-over-tcp": true}, "udp-over-tcp", true},
		{"ss generic 不表达 UDP over TCP", "ss", "generic-subs",
			map[string]any{"cipher": "aes-256-gcm", "password": "p", "udp-over-tcp": true}, "udp-over-tcp", true},
		{"vless SR 不表达 packet-addr", "vless", "sr-subs",
			map[string]any{"uuid": "u", "tls": true, "packet-addr": true}, "packet-addr", true},
		{"vless generic 不表达 xudp", "vless", "generic-subs",
			map[string]any{"uuid": "u", "tls": true, "xudp": true}, "xudp", true},
		{"trojan SR 不表达 client-fingerprint", "trojan", "sr-subs",
			map[string]any{"password": "p", "client-fingerprint": "chrome"}, "client-fingerprint", true},
		{"trojan generic 不表达 client-fingerprint", "trojan", "generic-subs",
			map[string]any{"password": "p", "client-fingerprint": "chrome"}, "client-fingerprint", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := linkTargetDiagnostics(tc.target, tc.protocol, tc.params)
			found := false
			for _, diagnostic := range diagnostics {
				if diagnostic.FieldPath != tc.fieldPath {
					continue
				}
				found = true
				if diagnostic.Severity != "warn" || diagnostic.Code != "uri_partial_fields" {
					t.Fatalf("字段丢失必须使用稳定 partial code: %+v", diagnostic)
				}
			}
			if found != tc.wantGap {
				t.Fatalf("字段 %s 的诊断结论不符合目标表达能力: want=%v got=%+v", tc.fieldPath, tc.wantGap, diagnostics)
			}
		})
	}
}

// step20EvidenceSources 是目标诊断允许引用的证据来源；任何新来源都必须在此登记，
// 避免 evidence 退化为无法核验的展示标签。
var step20EvidenceSources = map[string]bool{
	"mihomo-1.19.31-yaml":    true,
	"cvr-2.5.2-uri":          true,
	"project-uri-capability": true,
	"project-uri-common":     true,
	"project-uri-anytls":     true,
	"project-uri-wireguard":  true,
	"build32-uri-registry":   true,
	"build18-check-v1":       true,
	"project-unknown":        true,
}

// step20SchemaPathExists 判断点路径能否在协议 schema 中解析（含对象属性与开放 Map）。
func step20SchemaPathExists(fields []node.FieldSchema, segments []string) bool {
	if len(segments) == 0 {
		return true
	}
	for _, field := range fields {
		if field.Name != segments[0] {
			continue
		}
		if len(segments) == 1 {
			return true
		}
		if field.Type != "object" {
			return false
		}
		if field.AllowUnknown {
			return true
		}
		return step20SchemaPathExists(field.Properties, segments[1:])
	}
	return false
}

// TestTargetDiagnosticFieldPathsResolveToSchemaCanonicalPath 断言所有目标诊断的
// field_path 要么为空、要么是节点级路径、要么可回指协议 schema 的规范路径。
func TestTargetDiagnosticFieldPathsResolveToSchemaCanonicalPath(t *testing.T) {
	nodeLevel := map[string]bool{"protocol": true, "host": true, "port": true}
	entries, err := os.ReadDir(filepath.Join("testdata", "node_check"))
	if err != nil {
		t.Fatalf("读取固定夹具目录失败: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			svc, st, cfg := newTestService(t)
			nodeSvc := node.NewService(st, cfg, log.New("error", "console"))
			nodeSvc.SetCheckRenderer(svc.CheckNodeTarget)
			raw, err := os.ReadFile(filepath.Join("testdata", "node_check", entry.Name()))
			if err != nil {
				t.Fatalf("读取固定夹具失败: %v", err)
			}
			var req node.CheckRequest
			if err := json.Unmarshal(raw, &req); err != nil {
				t.Fatalf("解析固定夹具失败: %v", err)
			}
			proto, err := node.GetProtocol(req.Protocol)
			if err != nil {
				t.Fatalf("读取协议注册表失败: %v", err)
			}
			resp, err := nodeSvc.Check(context.Background(), req)
			if err != nil {
				t.Fatalf("执行固定夹具检查失败: %v", err)
			}
			for target, result := range resp.Targets {
				for _, diagnostic := range result.Diagnostics {
					if !step20EvidenceSources[diagnostic.Evidence] {
						t.Fatalf("目标 %s 诊断 %s 缺少可核验的证据来源: %+v",
							target, diagnostic.Code, diagnostic)
					}
					path := diagnostic.FieldPath
					if path == "" || nodeLevel[path] || strings.HasPrefix(path, "extensions.") {
						continue
					}
					if !step20SchemaPathExists(proto.FormSchema, strings.Split(path, ".")) {
						t.Fatalf("目标 %s 诊断 %s 的 field_path=%q 无法回指 schema 规范路径: %+v",
							target, diagnostic.Code, path, diagnostic)
					}
				}
			}
		})
	}
}

// TestNodeCheckDoesNotWriteBusinessTables 断言节点检查前后所有业务表行数完全一致。
func TestNodeCheckDoesNotWriteBusinessTables(t *testing.T) {
	svc, st, cfg := newTestService(t)
	nodeSvc := node.NewService(st, cfg, log.New("error", "console"))
	nodeSvc.SetCheckRenderer(svc.CheckNodeTarget)

	snapshot := func() map[string]int {
		t.Helper()
		rows, err := st.DB().QueryContext(context.Background(),
			`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
		if err != nil {
			t.Fatalf("读取表清单失败: %v", err)
		}
		var names []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				t.Fatalf("扫描表名失败: %v", err)
			}
			names = append(names, name)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("遍历表清单失败: %v", err)
		}
		if err := rows.Close(); err != nil {
			t.Fatalf("关闭表清单失败: %v", err)
		}
		counts := make(map[string]int, len(names))
		for _, name := range names {
			var count int
			if err := st.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM "`+name+`"`).Scan(&count); err != nil {
				t.Fatalf("统计表 %s 行数失败: %v", name, err)
			}
			counts[name] = count
		}
		return counts
	}

	before := snapshot()
	cases := []struct {
		protocol string
		port     int
		params   map[string]any
	}{
		{"ss", 8388, map[string]any{"cipher": "aes-256-gcm", "password": "ss-secret",
			"plugin": "shadow-tls", "shadow-tls-opts": map[string]any{"host": "cdn.example.com", "password": "st-secret"}}},
		{"vmess", 443, map[string]any{"uuid": "11111111-2222-3333-4444-555555555555", "network": "ws", "tls": true}},
		{"vless", 443, map[string]any{"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp", "tls": true}},
		{"trojan", 443, map[string]any{"password": "trojan-secret", "network": "tcp", "sni": "example.com"}},
	}
	for _, tc := range cases {
		if _, err := nodeSvc.Check(context.Background(), node.CheckRequest{
			Protocol: tc.protocol, Host: "example.com", Port: tc.port, ProtocolJSON: tc.params,
		}); err != nil {
			t.Fatalf("%s 检查失败: %v", tc.protocol, err)
		}
	}
	after := snapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("节点检查写入了业务表: before=%v after=%v", before, after)
	}
}

// TestURICapabilityDrivesTargetStatus 证明目标状态确实由实际能力判断产生：
// 存在无法表达的活动字段时，节点检查不得继续给出 ok，而是降级为 warn（非阻断丢失）。
func TestURICapabilityDrivesTargetStatus(t *testing.T) {
	cases := []struct {
		protocol string
		port     int
		params   map[string]any
	}{
		{"ss", 8388, map[string]any{"cipher": "aes-256-gcm", "password": "ss-secret", "udp-over-tcp": true}},
		{"vmess", 443, map[string]any{"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp", "tls": true, "packet-addr": true}},
		{"vless", 443, map[string]any{"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp", "tls": true, "packet-addr": true}},
		{"trojan", 443, map[string]any{"password": "trojan-secret", "network": "tcp", "sni": "example.com", "client-fingerprint": "chrome"}},
	}
	for _, tc := range cases {
		t.Run(tc.protocol, func(t *testing.T) {
			svc, st, cfg := newTestService(t)
			nodeSvc := node.NewService(st, cfg, log.New("error", "console"))
			nodeSvc.SetCheckRenderer(svc.CheckNodeTarget)
			resp, err := nodeSvc.Check(context.Background(), node.CheckRequest{
				Protocol: tc.protocol, Host: "example.com", Port: tc.port,
				ProtocolJSON: tc.params, Targets: []string{"sr-subs", "generic-subs"},
			})
			if err != nil {
				t.Fatalf("节点检查失败: %v", err)
			}
			for _, target := range []string{"sr-subs", "generic-subs"} {
				result := resp.Targets[target]
				if result.Status != "warn" {
					t.Fatalf("目标 %s 存在不可表达字段时必须降级为 warn，实际 %q: %+v",
						target, result.Status, result.Diagnostics)
				}
			}
		})
	}
}

// TestClashIssuePathsMapToSchemaCanonicalPath 断言 YAML 自检问题被映射回 schema 规范路径，
// 使 target evidence 的 field_path 可以回指协议字段而不是只给出 YAML 位置。
func TestClashIssuePathsMapToSchemaCanonicalPath(t *testing.T) {
	orig, had := clashProtocolAdapters["ss"]
	defer func() {
		if had {
			clashProtocolAdapters["ss"] = orig
		}
	}()
	cases := []struct {
		name     string
		fields   map[string]any
		params   map[string]any
		wantPath string
	}{
		{
			name:     "缺少必填字段",
			fields:   map[string]any{"cipher": "aes-256-gcm"},
			params:   map[string]any{"cipher": "aes-256-gcm", "password": "ss-secret"},
			wantPath: "password",
		},
		{
			name: "插件参数缺失映射回存储对象",
			fields: map[string]any{"cipher": "aes-256-gcm", "password": "ss-secret",
				"plugin": "obfs", "plugin-opts": map[string]any{"host": "cdn.example.com"}},
			params: map[string]any{"cipher": "aes-256-gcm", "password": "ss-secret",
				"plugin": "obfs", "obfs-opts": map[string]any{"host": "cdn.example.com"}},
			wantPath: "obfs-opts.mode",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fields := tc.fields
			clashProtocolAdapters["ss"] = func(ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
				return fields, nil, nil
			}
			res, err := (&Service{}).CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
				Target: "clash-yaml", Protocol: "ss", RenderName: "ss-node",
				Host: "example.com", Port: 8388, Params: tc.params,
			})
			if err != nil {
				t.Fatalf("节点检查失败: %v", err)
			}
			for _, diagnostic := range res.Diagnostics {
				if diagnostic.Severity == "error" && diagnostic.FieldPath == tc.wantPath {
					return
				}
			}
			t.Fatalf("缺少 canonical field_path=%s 的自检诊断: %+v", tc.wantPath, res.Diagnostics)
		})
	}
}
