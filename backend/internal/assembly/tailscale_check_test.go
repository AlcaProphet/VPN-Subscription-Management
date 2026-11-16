package assembly

import (
	"context"
	"os"
	"strings"
	"testing"

	gyaml "github.com/goccy/go-yaml"

	"vpn-sub/internal/log"
	"vpn-sub/internal/node"
	"vpn-sub/internal/store"
)

// tailscaleFields 覆盖 Tailscale adapter 的 wire 形状、state-dir 生命周期、warn 诊断与零副作用。
func tailscaleFields(t *testing.T, params map[string]any, nodeID int64, persisted bool) (map[string]any, []node.TargetDiagnostic) {
	t.Helper()
	res, err := new(Service).CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
		Target: "clash-yaml", Protocol: "tailscale", RenderName: "ts-node",
		Params: params, NodeID: nodeID, Persisted: persisted,
	})
	if err != nil {
		t.Fatalf("Tailscale 目标检查失败: %v", err)
	}
	if res.Preview == "" {
		t.Fatalf("已保存节点检查必须返回产物: %+v", res)
	}
	proxy, ok := tailscalePreviewProxy(t, res.Preview)
	if !ok {
		t.Fatalf("检查预览解析失败: %s", res.Preview)
	}
	return proxy, res.Diagnostics
}

func tailscalePreviewProxy(t *testing.T, preview string) (map[string]any, bool) {
	t.Helper()
	var decoded struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := gyaml.Unmarshal([]byte(preview), &decoded); err != nil {
		return nil, false
	}
	if len(decoded.Proxies) != 1 {
		return nil, false
	}
	return decoded.Proxies[0], true
}

// TestTailscaleClashAdapterStateDirLifecycle 覆盖 state-dir 只由服务端注入的稳定 NodeID 派生。
func TestTailscaleClashAdapterStateDirLifecycle(t *testing.T) {
	params := map[string]any{"hostname": "node-a", "auth-key": "ts-secret", "exit-node": "100.64.0.1"}

	draft, _ := tailscaleFields(t, params, 0, false)
	if _, exists := draft["state-dir"]; exists {
		t.Fatalf("未保存草稿不得输出 state-dir: %+v", draft)
	}

	saved, _ := tailscaleFields(t, params, 42, true)
	if saved["state-dir"] != "tailscale/node-42" {
		t.Fatalf("已保存节点必须派生 tailscale/node-<id>: %+v", saved["state-dir"])
	}
	again, _ := tailscaleFields(t, params, 42, true)
	if again["state-dir"] != saved["state-dir"] {
		t.Fatalf("同一 NodeID 的 state-dir 必须稳定: %v vs %v", saved["state-dir"], again["state-dir"])
	}

	for _, tc := range []struct {
		name      string
		nodeID    int64
		persisted bool
	}{
		{name: "persisted without id", nodeID: 0, persisted: true},
		{name: "draft with id", nodeID: 7, persisted: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := new(Service).CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
				Target: "clash-yaml", Protocol: "tailscale", RenderName: "ts-node",
				Params: params, NodeID: tc.nodeID, Persisted: tc.persisted,
			})
			if err == nil {
				t.Fatalf("非法生命周期必须阻断: %+v", res)
			}
		})
	}
}

// TestTailscaleClashAdapterWireShape 覆盖 Tailscale wire 形状：无 endpoint、三态 bool 保真、无内部泄漏。
func TestTailscaleClashAdapterWireShape(t *testing.T) {
	params := map[string]any{
		"hostname": "node-a", "auth-key": "ts-secret", "control-url": "https://headscale.example.com",
		"ephemeral": true, "udp": false, "exit-node": "auto:any",
		"accept-routes": false, "exit-node-allow-lan-access": false,
		"interface-name": "utun0", "tfo": true, "mptcp": true,
	}
	proxy, _ := tailscaleFields(t, params, 9, true)
	for _, key := range []string{"server", "port"} {
		if _, exists := proxy[key]; exists {
			t.Fatalf("Tailscale wire 不得输出 %s: %+v", key, proxy)
		}
	}
	if proxy["state-dir"] != "tailscale/node-9" {
		t.Fatalf("state-dir 派生异常: %+v", proxy)
	}
	for _, key := range []string{"hostname", "auth-key", "control-url", "ephemeral", "exit-node", "interface-name"} {
		if _, exists := proxy[key]; !exists {
			t.Fatalf("Tailscale wire 缺少 %s: %+v", key, proxy)
		}
	}
	for _, key := range []string{"accept-routes", "exit-node-allow-lan-access"} {
		value, exists := proxy[key]
		if !exists || value != false {
			t.Fatalf("三态 bool %s 的显式 false 必须输出: %+v", key, proxy)
		}
	}
	if value, exists := proxy["udp"]; exists && value != false {
		t.Fatalf("udp=false 时不得输出 true: %+v", proxy)
	}
	udpOn, _ := tailscaleFields(t, map[string]any{"hostname": "node-a", "udp": true}, 9, true)
	if udpOn["udp"] != true {
		t.Fatalf("udp=true 必须输出: %+v", udpOn)
	}
	for _, key := range []string{"tfo", "mptcp", "state_dir", "stateDir"} {
		if _, exists := proxy[key]; exists {
			t.Fatalf("Tailscale wire 不得输出 %s: %+v", key, proxy)
		}
	}

	unset, _ := tailscaleFields(t, map[string]any{"hostname": "node-a"}, 9, true)
	for _, key := range []string{"accept-routes", "exit-node-allow-lan-access"} {
		if _, exists := unset[key]; exists {
			t.Fatalf("未设置的三态 bool %s 不得输出: %+v", key, unset)
		}
	}
}

// TestTailscaleClashAdapterAuthKeyWarn 覆盖空 auth-key 只返回交互登录 warn 且不含登录 URL。
func TestTailscaleClashAdapterAuthKeyWarn(t *testing.T) {
	noKey, diagnostics := tailscaleFields(t, map[string]any{"hostname": "node-a"}, 9, true)
	if _, exists := noKey["auth-key"]; exists {
		t.Fatalf("空 auth-key 不得写入 wire: %+v", noKey)
	}
	found := false
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "tailscale_auth_key_interactive_login" {
			found = true
			if diagnostic.Severity != "warn" || diagnostic.FieldPath != "auth-key" {
				t.Fatalf("交互登录提示的级别／路径异常: %+v", diagnostic)
			}
			if strings.Contains(diagnostic.Message, "://") {
				t.Fatalf("静态检查不得生成或回显登录地址: %s", diagnostic.Message)
			}
		}
	}
	if !found {
		t.Fatalf("空 auth-key 必须返回交互登录 warn: %+v", diagnostics)
	}

	withKey, diagnostics := tailscaleFields(t, map[string]any{"hostname": "node-a", "auth-key": "ts-secret"}, 9, true)
	if withKey["auth-key"] != "REDACTED" {
		t.Fatalf("auth-key 必须进入 wire 且预览已脱敏: %+v", withKey)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "tailscale_auth_key_interactive_login" {
			t.Fatalf("已配置 auth-key 不应再提示交互登录: %+v", diagnostics)
		}
	}
}

// TestTailscaleClashAdapterControlURLHint 覆盖非 HTTPS 控制面只给安全提示。
func TestTailscaleClashAdapterControlURLHint(t *testing.T) {
	for _, tc := range []struct {
		name     string
		url      string
		wantWarn bool
	}{
		{name: "http local headscale", url: "http://headscale.local:8080", wantWarn: true},
		{name: "https headscale", url: "https://headscale.example.com", wantWarn: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proxy, diagnostics := tailscaleFields(t, map[string]any{"hostname": "node-a", "control-url": tc.url}, 9, true)
			if proxy["control-url"] != tc.url {
				t.Fatalf("control-url 必须原样输出: %+v", proxy)
			}
			warned := false
			for _, diagnostic := range diagnostics {
				if diagnostic.Code == "tailscale_control_url_insecure" {
					warned = true
					if diagnostic.Severity != "warn" {
						t.Fatalf("非 HTTPS 控制面必须是 warn 而非阻断: %+v", diagnostic)
					}
				}
			}
			if warned != tc.wantWarn {
				t.Fatalf("control-url 安全提示与预期不符: url=%s warned=%v diagnostics=%+v", tc.url, warned, diagnostics)
			}
		})
	}
}

// TestTailscaleFormalAssemblyBlocksMissingStableID 覆盖装配阶段节点缺稳定身份时的阻断。
func TestTailscaleFormalAssemblyBlocksMissingStableID(t *testing.T) {
	svc := &Service{}
	nd := &nodeData{
		NodeID: 0, Protocol: "tailscale", RenderName: "ts-missing-id",
		ProtocolJSON: map[string]any{"hostname": "node-a"},
	}
	diagnostics := svc.diagnoseNodeForTarget("clash-yaml", nd)
	if !hasCoreBlockingNodeDiagnostic(diagnostics) {
		t.Fatalf("缺失稳定身份的 Tailscale 节点必须在装配阶段阻断: %+v", diagnostics)
	}

	ok := &nodeData{
		NodeID: 12, Protocol: "tailscale", RenderName: "ts-with-id",
		ProtocolJSON: map[string]any{"hostname": "node-a", "auth-key": "ts-secret"},
	}
	if diagnostics := svc.diagnoseNodeForTarget("clash-yaml", ok); hasCoreBlockingNodeDiagnostic(diagnostics) {
		t.Fatalf("携带稳定 ID 的 Tailscale 节点不应阻断: %+v", diagnostics)
	}
}

// TestTailscaleCheckHasNoDatabaseOrFilesystemSideEffects 覆盖新建草稿检查不落库、不触碰文件系统。
func TestTailscaleCheckHasNoDatabaseOrFilesystemSideEffects(t *testing.T) {
	svc, st, cfg := newTestService(t)
	nodeSvc := node.NewService(st, cfg, log.New("error", "console"))
	nodeSvc.SetCheckRenderer(svc.CheckNodeTarget)
	ctx := context.Background()

	beforeNodes := tailscaleCountNodes(t, st)
	beforeEntries := dirEntryNames(t, ".")

	resp, err := nodeSvc.Check(ctx, node.CheckRequest{
		Protocol:     "tailscale",
		ProtocolJSON: map[string]any{"hostname": "node-a", "exit-node": "100.64.0.1"},
		Targets:      []string{"clash-yaml"},
	})
	if err != nil {
		t.Fatalf("新建草稿检查失败: %v", err)
	}
	result := resp.Targets["clash-yaml"]
	if result.Preview == nil {
		t.Fatalf("Tailscale 草稿检查必须返回 Clash 预览: %+v", result)
	}
	if strings.Contains(*result.Preview, "state-dir") {
		t.Fatalf("新建草稿检查不得输出 state-dir: %s", *result.Preview)
	}
	proxy, ok := tailscalePreviewProxy(t, *result.Preview)
	if !ok {
		t.Fatalf("草稿检查预览解析失败: %s", *result.Preview)
	}
	for _, key := range []string{"server", "port"} {
		if _, exists := proxy[key]; exists {
			t.Fatalf("Tailscale 草稿检查不得输出 %s: %+v", key, proxy)
		}
	}
	if afterNodes := tailscaleCountNodes(t, st); afterNodes != beforeNodes {
		t.Fatalf("新建草稿检查不得写库: before=%d after=%d", beforeNodes, afterNodes)
	}
	if after := dirEntryNames(t, "."); after != beforeEntries {
		t.Fatalf("检查不得创建目录或文件: before=%s after=%s", beforeEntries, after)
	}
}

func tailscaleCountNodes(t *testing.T, st *store.Store) int {
	t.Helper()
	var count int
	if err := st.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM nodes`).Scan(&count); err != nil {
		t.Fatalf("读取节点数量失败: %v", err)
	}
	return count
}

func dirEntryNames(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取目录失败: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return strings.Join(names, ",")
}
