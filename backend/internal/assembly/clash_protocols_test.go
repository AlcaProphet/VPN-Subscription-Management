package assembly

import (
	"context"
	"fmt"
	"testing"

	gyaml "github.com/goccy/go-yaml"

	"vpn-sub/internal/node"
)

// TestLegacyAdapterPendingCountAfterMigratedProtocols 锁定当前已迁移协议的绝对计数：
// Step 4 迁移 HTTP、Step 5 迁移 SOCKS5，待迁移协议为 17；
// 后续每个协议 Step 必须继续递减，Step 20 归零。
func TestLegacyAdapterPendingCountAfterMigratedProtocols(t *testing.T) {
	if got := legacyAdapterPendingCount(); got != 17 {
		t.Fatalf("HTTP 与 SOCKS5 迁移后 legacy adapter 数量应为 17，实际 %d", got)
	}
}

// TestSocks5ClashAdapterWireShape 锁定 SOCKS5 adapter 的 v1.19.31 Socks5Option 形状：
// 无独立 sni，TLS 关闭时不含 TLS 子字段，UDP 独立输出。
func TestSocks5ClashAdapterWireShape(t *testing.T) {
	svc := &Service{}
	proxyFields := func(params map[string]any, state node.CurrentState) map[string]any {
		res, err := svc.CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
			Target: "clash-yaml", Protocol: "socks5", RenderName: "socks5-node",
			Host: "example.com", Port: 1080, Params: params, State: state,
		})
		if err != nil {
			t.Fatalf("SOCKS5 目标检查失败: %v", err)
		}
		var decoded struct {
			Proxies []map[string]any `yaml:"proxies"`
		}
		if err := gyaml.Unmarshal([]byte(res.Preview), &decoded); err != nil {
			t.Fatalf("解析检查预览失败: %v\n%s", err, res.Preview)
		}
		if len(decoded.Proxies) != 1 {
			t.Fatalf("检查预览代理数量异常: %s", res.Preview)
		}
		return decoded.Proxies[0]
	}

	basicState := node.CurrentState{Security: "tls", Features: []string{"tls"}, Selectors: map[string]string{"auth_mode": "basic"}}
	full := proxyFields(map[string]any{
		"username": "u", "password": "p", "udp": true, "tls": true,
		"skip-cert-verify": true, "name-cert-verify": "verify-name", "fingerprint": "aa:bb",
		"certificate": "CERT", "private-key": "KEY", "auth-mode": "basic",
	}, basicState)
	for _, key := range []string{"username", "password", "udp", "tls", "skip-cert-verify", "name-cert-verify", "fingerprint", "certificate", "private-key"} {
		if _, ok := full[key]; !ok {
			t.Fatalf("SOCKS5 wire 缺少活动字段 %s: %+v", key, full)
		}
	}
	for _, key := range []string{"sni", "auth-mode", "auth_mode"} {
		if _, ok := full[key]; ok {
			t.Fatalf("SOCKS5 wire 不应输出 %s: %+v", key, full)
		}
	}

	noneState := node.CurrentState{Security: "none", Selectors: map[string]string{"auth_mode": "none"}}
	off := proxyFields(map[string]any{
		"tls": false, "skip-cert-verify": true, "fingerprint": "aa:bb",
		"certificate": "CERT", "private-key": "KEY", "udp": true,
	}, noneState)
	for _, key := range []string{"tls", "skip-cert-verify", "name-cert-verify", "fingerprint", "certificate", "private-key"} {
		if _, ok := off[key]; ok {
			t.Fatalf("tls=false 时 SOCKS5 wire 不应输出 %s: %+v", key, off)
		}
	}
	if off["udp"] != true {
		t.Fatalf("UDP 是独立开关，必须与 TLS 状态无关地输出: %+v", off)
	}
}

// 只输出活动字段，不输出 state_only selector，TLS 关闭时不含任何 TLS 子字段。
func TestHTTPClashAdapterWireShape(t *testing.T) {
	svc := &Service{}
	proxyFields := func(params map[string]any, state node.CurrentState) map[string]any {
		res, err := svc.CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
			Target: "clash-yaml", Protocol: "http", RenderName: "http-node",
			Host: "example.com", Port: 8080, Params: params, State: state,
		})
		if err != nil {
			t.Fatalf("HTTP 目标检查失败: %v", err)
		}
		var decoded struct {
			Proxies []map[string]any `yaml:"proxies"`
		}
		if err := gyaml.Unmarshal([]byte(res.Preview), &decoded); err != nil {
			t.Fatalf("解析检查预览失败: %v\n%s", err, res.Preview)
		}
		if len(decoded.Proxies) != 1 {
			t.Fatalf("检查预览代理数量异常: %s", res.Preview)
		}
		return decoded.Proxies[0]
	}

	basicState := node.CurrentState{Security: "tls", Features: []string{"tls"}, Selectors: map[string]string{"auth_mode": "basic"}}
	full := proxyFields(map[string]any{
		"username": "u", "password": "p", "tls": true, "sni": "sni.example.com",
		"skip-cert-verify": true, "name-cert-verify": "verify-name", "fingerprint": "aa:bb",
		"certificate": "CERT", "private-key": "KEY",
		"headers": map[string]any{"X-Test": "1"},
		"tfo":     true, "mptcp": false, "ip-version": "ipv4",
		"auth-mode": "basic",
	}, basicState)
	for _, key := range []string{"username", "password", "tls", "sni", "skip-cert-verify", "name-cert-verify",
		"fingerprint", "certificate", "private-key", "headers", "tfo", "ip-version"} {
		if _, ok := full[key]; !ok {
			t.Fatalf("HTTP wire 缺少活动字段 %s: %+v", key, full)
		}
	}
	for _, key := range []string{"auth-mode", "auth_mode", "mptcp"} {
		if _, ok := full[key]; ok {
			t.Fatalf("HTTP wire 不应输出 %s: %+v", key, full)
		}
	}

	noneState := node.CurrentState{Security: "none", Selectors: map[string]string{"auth_mode": "none"}}
	off := proxyFields(map[string]any{
		"tls": false, "sni": "sni.example.com", "skip-cert-verify": true,
		"fingerprint": "aa:bb", "certificate": "CERT", "private-key": "KEY",
	}, noneState)
	for _, key := range []string{"tls", "sni", "skip-cert-verify", "name-cert-verify", "fingerprint", "certificate", "private-key"} {
		if _, ok := off[key]; ok {
			t.Fatalf("tls=false 时 HTTP wire 不应输出 %s: %+v", key, off)
		}
	}
	if off["server"] != "example.com" || fmt.Sprint(off["port"]) != "8080" {
		t.Fatalf("HTTP 顶层 endpoint 丢失: %+v", off)
	}
}

// TestClashAdapterRegistryLegacyPendingObservable 使用仍未迁移的协议验证 legacy 证据可观测，
// 同时确认 HTTP 已迁移为显式 adapter（Build32 Step 4 起 legacy 计数逐协议递减）。
// firstLegacyProtocol 返回当前仍未迁移到显式 adapter 的协议，避免每个协议 Step 反复改测试。
func firstLegacyProtocol(t *testing.T) string {
	t.Helper()
	for _, protocol := range node.ManualProtocols() {
		if _, ok := clashProtocolAdapters[protocol.Protocol]; !ok {
			return protocol.Protocol
		}
	}
	t.Fatal("没有待迁移协议可用于 legacy 断言")
	return ""
}

func TestClashAdapterRegistryLegacyPendingObservable(t *testing.T) {
	svc := &Service{}
	legacy := firstLegacyProtocol(t)
	res, err := svc.CheckNodeTarget(context.Background(), "clash-yaml", legacy, "legacy-node", "example.com", 1080, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, diagnostic := range res.Diagnostics {
		if diagnostic.Code == "legacy_adapter_pending" && diagnostic.Severity == "info" {
			found = true
		}
	}
	if !found {
		t.Fatalf("legacy adapter 必须返回 legacy_adapter_pending 证据: %+v", res.Diagnostics)
	}

	httpRes, err := svc.CheckNodeTarget(context.Background(), "clash-yaml", "http", "migrated-node", "example.com", 8080, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range httpRes.Diagnostics {
		if diagnostic.Code == "legacy_adapter_pending" {
			t.Fatalf("HTTP 已迁移为显式 adapter，不应再返回 legacy 证据: %+v", httpRes.Diagnostics)
		}
	}
}

func TestCheckAndFormalAssemblyUseSameRegisteredAdapterDraft(t *testing.T) {
	legacy := firstLegacyProtocol(t)
	orig, had := clashProtocolAdapters[legacy]
	var captured ClashNodeDraft
	clashProtocolAdapters[legacy] = func(draft ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
		captured = draft
		return map[string]any{"custom": "value"}, nil, nil
	}
	defer func() {
		if had {
			clashProtocolAdapters[legacy] = orig
		} else {
			delete(clashProtocolAdapters, legacy)
		}
	}()

	svc := &Service{}
	_, err := svc.CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
		Target: "clash-yaml", Protocol: legacy, RenderName: "adapter-node",
		Host: "example.com", Port: 1080, Params: map[string]any{}, NodeID: 42, Persisted: true,
		State: node.CurrentState{Security: "tls"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if captured.NodeID != 42 || !captured.Persisted || captured.State.Security != "tls" {
		t.Fatalf("check 路径未注入完整生命周期状态: %+v", captured)
	}

	nd := &nodeData{NodeID: 42, Protocol: legacy, RenderName: "adapter-node", Host: "example.com", Port: 1080, CurrentState: node.CurrentState{Security: "tls"}}
	p, _, err := svc.buildClashProxy(nd, true)
	if err != nil {
		t.Fatal(err)
	}
	if value, ok := p.Get("custom"); !ok || value != "value" {
		t.Fatalf("正式装配未使用同一 adapter: %+v", p)
	}
}

func TestLegacyAdapterPendingCountTracksRegistry(t *testing.T) {
	before := legacyAdapterPendingCount()
	legacy := firstLegacyProtocol(t)
	orig, had := clashProtocolAdapters[legacy]
	clashProtocolAdapters[legacy] = func(ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
		return map[string]any{}, nil, nil
	}
	defer func() {
		if had {
			clashProtocolAdapters[legacy] = orig
		} else {
			delete(clashProtocolAdapters, legacy)
		}
	}()
	after := legacyAdapterPendingCount()
	if before <= 0 || after != before-1 {
		t.Fatalf("legacy adapter 计数未随注册下降: before=%d after=%d", before, after)
	}
}

func TestOrderedMapFromClashFieldsHonorsHiddenEndpointPolicy(t *testing.T) {
	p := orderedMapFromClashFields("node", "custom", "example.com", 443, map[string]any{
		"name": "ignored", "server": "ignored", "port": 1, "custom": "value",
	}, node.EndpointPolicy{
		HostMode: "hidden", PortMode: "hidden", EmitHost: false, EmitPort: false,
	})
	if _, ok := p.Get("server"); ok {
		t.Fatal("hidden host 不应输出 server")
	}
	if _, ok := p.Get("port"); ok {
		t.Fatal("hidden port 不应输出 port")
	}
	if value, ok := p.Get("custom"); !ok || value != "value" {
		t.Fatalf("adapter 普通字段丢失: %+v", value)
	}
}
