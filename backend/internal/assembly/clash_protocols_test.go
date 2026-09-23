package assembly

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	gyaml "github.com/goccy/go-yaml"

	"vpn-sub/internal/node"
)

// TestLegacyAdapterPendingCountAfterMigratedProtocols 锁定 Step 4～20 全部迁移完成后的绝对计数：
// 19 个 manual 协议必须全部经过显式 adapter，legacy 待迁移计数归零。
func TestLegacyAdapterPendingCountAfterMigratedProtocols(t *testing.T) {
	if got := legacyAdapterPendingCount(); got != 0 {
		t.Fatalf("19 个 manual 协议全部迁移后 legacy adapter 数量应为 0，实际 %d", got)
	}
}

// TestTUICClashAdapterWireShape 锁定 TUIC adapter 的 v1.19.31 TuicOption 形状：
// v4/v5 凭据互斥、UOT 版本只在开启时以整数输出、disable-sni 清空 SNI 并给出风险 warn。
func TestTUICClashAdapterWireShape(t *testing.T) {
	svc := &Service{}
	proxyFields := func(params map[string]any, state node.CurrentState) (map[string]any, []node.TargetDiagnostic) {
		res, err := svc.CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
			Target: "clash-yaml", Protocol: "tuic", RenderName: "tuic-node",
			Host: "example.com", Port: 443, Params: params, State: state,
		})
		if err != nil {
			t.Fatalf("TUIC 目标检查失败: %v", err)
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
		return decoded.Proxies[0], res.Diagnostics
	}

	v4State := node.CurrentState{Selectors: map[string]string{"auth_mode": "v4"}}
	v4, _ := proxyFields(map[string]any{"token": "token-secret", "uuid": "legacy-uuid", "password": "legacy-pw"}, v4State)
	if _, ok := v4["token"]; !ok {
		t.Fatalf("v4 必须输出 token: %+v", v4)
	}
	for _, key := range []string{"uuid", "password", "auth-mode", "auth_mode"} {
		if _, ok := v4[key]; ok {
			t.Fatalf("v4 wire 不应输出 %s: %+v", key, v4)
		}
	}

	v5State := node.CurrentState{Selectors: map[string]string{"auth_mode": "v5"}}
	v5, _ := proxyFields(map[string]any{"token": "legacy-token", "uuid": "11111111-2222-3333-4444-555555555555",
		"password": "pw-secret", "alpn": []any{"h3"},
		"udp-over-stream": true, "udp-over-stream-version": "2"}, v5State)
	for _, key := range []string{"uuid", "password", "udp-over-stream", "udp-over-stream-version", "alpn"} {
		if _, ok := v5[key]; !ok {
			t.Fatalf("v5 wire 缺少活动字段 %s: %+v", key, v5)
		}
	}
	if _, ok := v5["token"]; ok {
		t.Fatalf("v5 wire 不得输出 token: %+v", v5)
	}
	if fmt.Sprint(v5["udp-over-stream-version"]) != "2" {
		t.Fatalf("UOT 版本必须以整数输出: %+v", v5["udp-over-stream-version"])
	}
	if alpn, ok := v5["alpn"].([]any); !ok || len(alpn) != 1 {
		t.Fatalf("TUIC alpn 必须为 YAML 数组: %+v", v5["alpn"])
	}

	// UOT 关闭时不输出版本。
	uotOff, _ := proxyFields(map[string]any{"uuid": "11111111-2222-3333-4444-555555555555",
		"password": "pw-secret", "udp-over-stream": false}, v5State)
	if _, ok := uotOff["udp-over-stream-version"]; ok {
		t.Fatalf("UOT 关闭不得输出版本: %+v", uotOff)
	}

	// disable-sni：清空 SNI 并给出风险 warn。
	disabled, diagnostics := proxyFields(map[string]any{"uuid": "11111111-2222-3333-4444-555555555555",
		"password": "pw-secret", "disable-sni": true, "sni": "s.example.com"}, v5State)
	if _, ok := disabled["sni"]; ok {
		t.Fatalf("disable-sni 不得保留冲突 SNI: %+v", disabled)
	}
	foundRisk := false
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "tuic_disable_sni_risk" && diagnostic.Severity == "warn" {
			foundRisk = true
		}
	}
	if !foundRisk {
		t.Fatalf("disable-sni 必须给出风险 warn: %+v", diagnostics)
	}
}

// TestHysteria2ClashAdapterWireShape 锁定 Hysteria2 adapter 的 v1.19.31 Hysteria2Option 形状：
// ports 模式不输出顶层 port、obfs 由 selector 注入、禁用 Realm 不写入 wire。
func TestHysteria2ClashAdapterWireShape(t *testing.T) {
	svc := &Service{}
	proxyFields := func(params map[string]any, state node.CurrentState, port int) map[string]any {
		res, err := svc.CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
			Target: "clash-yaml", Protocol: "hysteria2", RenderName: "hysteria2-node",
			Host: "example.com", Port: port, Params: params, State: state,
		})
		if err != nil {
			t.Fatalf("Hysteria2 目标检查失败: %v", err)
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

	singleState := node.CurrentState{Selectors: map[string]string{"endpoint_mode": "single", "obfs_mode": "gecko"}}
	single := proxyFields(map[string]any{
		"password": "hy2-secret", "obfs-password": "obfs-secret",
		"obfs-min-packet-size": 20, "obfs-max-packet-size": 100,
		"sni": "s.example.com", "alpn": []any{"h3"},
	}, singleState, 443)
	if fmt.Sprint(single["port"]) != "443" {
		t.Fatalf("single 模式必须输出顶层 port: %+v", single)
	}
	if single["obfs"] != "gecko" {
		t.Fatalf("obfs 必须由 selector 注入 wire: %+v", single)
	}
	if _, ok := single["endpoint-mode"]; ok {
		t.Fatalf("wire 不得输出 endpoint_mode: %+v", single)
	}

	portsState := node.CurrentState{Selectors: map[string]string{"endpoint_mode": "ports", "obfs_mode": "none"}}
	ports := proxyFields(map[string]any{"password": "hy2-secret", "ports": "1000-2000", "hop-interval": "10-20"}, portsState, 0)
	if _, ok := ports["port"]; ok {
		t.Fatalf("ports 模式不得输出顶层 port: %+v", ports)
	}
	if ports["ports"] != "1000-2000" || ports["hop-interval"] != "10-20" {
		t.Fatalf("ports 模式必须输出端口组: %+v", ports)
	}
	if _, ok := ports["obfs"]; ok {
		t.Fatalf("obfs_mode=none 不得输出 obfs: %+v", ports)
	}

	realmState := node.CurrentState{Selectors: map[string]string{"endpoint_mode": "single", "obfs_mode": "none"}}
	disabled := proxyFields(map[string]any{"password": "hy2-secret",
		"realm-opts": map[string]any{"enable": false, "server-url": "https://realm.example.com", "token": "realm-token"}}, realmState, 443)
	if _, ok := disabled["realm-opts"]; ok {
		t.Fatalf("禁用的 Realm 不得写入 wire: %+v", disabled)
	}
}

// TestHysteriaClashAdapterWireShape 锁定 Hysteria adapter 的 v1.19.31 HysteriaOption 形状：
// 只输出当前认证分支、兼容别名不进入 wire、端口跳跃作为 port 的补充字段保留。
func TestHysteriaClashAdapterWireShape(t *testing.T) {
	svc := &Service{}
	proxyFields := func(params map[string]any, state node.CurrentState) map[string]any {
		res, err := svc.CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
			Target: "clash-yaml", Protocol: "hysteria", RenderName: "hysteria-node",
			Host: "example.com", Port: 443, Params: params, State: state,
		})
		if err != nil {
			t.Fatalf("Hysteria 目标检查失败: %v", err)
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

	base64State := node.CurrentState{Selectors: map[string]string{"auth_mode": "base64"}}
	full := proxyFields(map[string]any{
		"up": "100 Mbps", "down": "50 Mbps", "ports": "1000-2000", "protocol": "wechat-video",
		"auth": "dGVzdC1hdXRo", "auth-str": "legacy-str", "obfs": "obfs-secret",
		"sni": "s.example.com", "skip-cert-verify": true, "name-cert-verify": "verify",
		"fingerprint": "aa:bb", "alpn": []any{"hysteria"},
		"recv-window-conn": 1024, "recv-window": 4096, "hop-interval": 15,
		"obfs-protocol": "tcp", "up-speed": 999, "down-speed": 999, "auth-mode": "base64",
	}, base64State)
	for _, key := range []string{"up", "down", "ports", "protocol", "auth", "obfs", "sni",
		"skip-cert-verify", "name-cert-verify", "fingerprint", "alpn",
		"recv-window-conn", "recv-window", "hop-interval"} {
		if _, ok := full[key]; !ok {
			t.Fatalf("Hysteria wire 缺少活动字段 %s: %+v", key, full)
		}
	}
	for _, key := range []string{"auth-str", "auth-mode", "auth_mode", "obfs-protocol", "up-speed", "down-speed"} {
		if _, ok := full[key]; ok {
			t.Fatalf("Hysteria wire 不应输出 %s: %+v", key, full)
		}
	}
	if fmt.Sprint(full["port"]) != "443" {
		t.Fatalf("端口跳跃不得替代顶层 port: %+v", full)
	}
	if alpn, ok := full["alpn"].([]any); !ok || len(alpn) != 1 {
		t.Fatalf("Hysteria alpn 必须为 YAML 数组: %+v", full["alpn"])
	}

	stringState := node.CurrentState{Selectors: map[string]string{"auth_mode": "string"}}
	stringBranch := proxyFields(map[string]any{
		"up": "100 Mbps", "down": "50 Mbps", "auth-str": "plain-auth",
	}, stringState)
	if _, ok := stringBranch["auth-str"]; !ok {
		t.Fatalf("string 分支必须输出 auth-str: %+v", stringBranch)
	}
	if _, ok := stringBranch["auth"]; ok {
		t.Fatalf("string 分支不得输出 auth: %+v", stringBranch)
	}
}

// TestSnellClashAdapterWireShape 锁定 Snell adapter 的 v1.19.31 SnellOption 形状：
// version 为整数、v2 固定 reuse、v1/v2 不输出 udp、obfs-opts.mode 由 selector 注入，
// 且检查预览与正式装配对 http/tls 这类无法由字段反推的分支保持一致。
func TestSnellClashAdapterWireShape(t *testing.T) {
	svc := &Service{}
	proxyFields := func(params map[string]any, state node.CurrentState) (map[string]any, []node.TargetDiagnostic) {
		res, err := svc.CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
			Target: "clash-yaml", Protocol: "snell", RenderName: "snell-node",
			Host: "example.com", Port: 443, Params: params, State: state,
		})
		if err != nil {
			t.Fatalf("Snell 目标检查失败: %v", err)
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
		return decoded.Proxies[0], res.Diagnostics
	}

	// v4 + HTTPS 混淆：host 唯一致命歧义分支（http/tls 无法反推），必须按显式 selector 输出 tls。
	tlsState := node.CurrentState{Selectors: map[string]string{"version": "4", "obfs_mode": "tls"}}
	tlsFields, _ := proxyFields(map[string]any{
		"psk": "psk-secret", "version": "4", "udp": true,
		"obfs-opts": map[string]any{"host": "bing.com"},
	}, tlsState)
	if fmt.Sprint(tlsFields["version"]) != "4" {
		t.Fatalf("Snell wire version 必须为整数: %+v", tlsFields)
	}
	if tlsFields["udp"] != true {
		t.Fatalf("v4 必须输出 udp: %+v", tlsFields)
	}
	obfs, ok := tlsFields["obfs-opts"].(map[string]any)
	if !ok || obfs["mode"] != "tls" {
		t.Fatalf("检查预览必须按显式 selector 输出 obfs-opts.mode=tls: %+v", tlsFields)
	}

	// v2：固定 reuse，不输出 udp。
	v2State := node.CurrentState{Selectors: map[string]string{"version": "2", "obfs_mode": "none"}}
	v2Fields, _ := proxyFields(map[string]any{"psk": "psk-secret", "version": "2", "udp": false}, v2State)
	if v2Fields["reuse"] != true {
		t.Fatalf("v2 必须固定输出 reuse=true: %+v", v2Fields)
	}
	if _, exists := v2Fields["udp"]; exists {
		t.Fatalf("v2 不得输出 udp: %+v", v2Fields)
	}
	if _, exists := v2Fields["obfs-opts"]; exists {
		t.Fatalf("obfs_mode=none 必须删除整个 obfs-opts: %+v", v2Fields)
	}

	// v5：wire 保留 version: 5 并给出 v4 兼容诊断。
	v5State := node.CurrentState{Selectors: map[string]string{"version": "5", "obfs_mode": "none"}}
	v5Fields, v5Diagnostics := proxyFields(map[string]any{"psk": "psk-secret", "version": "5"}, v5State)
	if fmt.Sprint(v5Fields["version"]) != "5" {
		t.Fatalf("v5 wire 必须保留 version: 5: %+v", v5Fields)
	}
	foundCompat := false
	for _, diagnostic := range v5Diagnostics {
		if diagnostic.Code == "snell_v5_v4_compat" && diagnostic.Severity == "info" {
			foundCompat = true
		}
	}
	if !foundCompat {
		t.Fatalf("v5 必须给出 v4 客户端兼容诊断: %+v", v5Diagnostics)
	}
}

// TestSSHClashAdapterWireShape 锁定 SSH adapter 的 v1.19.31 SshOption 形状：
// host-key／host-key-algorithms 为数组、SSH 永不输出 udp、selector 不进入 wire，
// 空 host-key 产生安全 warn。
func TestSSHClashAdapterWireShape(t *testing.T) {
	svc := &Service{}
	proxyFields := func(params map[string]any, state node.CurrentState) (map[string]any, []node.TargetDiagnostic) {
		res, err := svc.CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
			Target: "clash-yaml", Protocol: "ssh", RenderName: "ssh-node",
			Host: "example.com", Port: 22, Params: params, State: state,
		})
		if err != nil {
			t.Fatalf("SSH 目标检查失败: %v", err)
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
		return decoded.Proxies[0], res.Diagnostics
	}

	keyState := node.CurrentState{Selectors: map[string]string{"auth_mode": "private_key"}}
	full, _ := proxyFields(map[string]any{
		"username": "u", "password": "p",
		"private-key":            "-----BEGIN PRIVATE KEY-----\nkey\n-----END PRIVATE KEY-----",
		"private-key-passphrase": "pass",
		"host-key":               []any{"ssh-ed25519 AAAA host"},
		"host-key-algorithms":    []any{"ssh-ed25519", "rsa-sha2-256"},
		"tfo":                    true, "auth-mode": "private_key",
	}, keyState)
	for _, key := range []string{"username", "private-key", "private-key-passphrase", "host-key", "host-key-algorithms", "tfo"} {
		if _, ok := full[key]; !ok {
			t.Fatalf("SSH wire 缺少活动字段 %s: %+v", key, full)
		}
	}
	for _, key := range []string{"password", "udp", "auth-mode", "auth_mode"} {
		if _, ok := full[key]; ok {
			t.Fatalf("SSH wire 不应输出 %s: %+v", key, full)
		}
	}
	hostKeys, ok := full["host-key"].([]any)
	if !ok || len(hostKeys) != 1 {
		t.Fatalf("SSH host-key 必须以 YAML 数组输出: %+v", full["host-key"])
	}

	passwordState := node.CurrentState{Selectors: map[string]string{"auth_mode": "password"}}
	passwordBranch, diagnostics := proxyFields(map[string]any{
		"username": "u", "password": "p",
		"host-key":  []any{"ssh-ed25519 AAAA host"},
		"auth-mode": "password",
	}, passwordState)
	if _, ok := passwordBranch["password"]; !ok {
		t.Fatalf("password 分支必须输出 password: %+v", passwordBranch)
	}
	if _, ok := passwordBranch["private-key"]; ok {
		t.Fatalf("password 分支不得输出 private-key: %+v", passwordBranch)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "ssh_host_key_unverified" {
			t.Fatalf("已配置 Host Key 不应产生未验证警告: %+v", diagnostics)
		}
	}

	_, emptyDiagnostics := proxyFields(map[string]any{
		"username": "u", "password": "p", "auth-mode": "password",
	}, passwordState)
	foundWarn := false
	for _, diagnostic := range emptyDiagnostics {
		if diagnostic.Code == "ssh_host_key_unverified" && diagnostic.Severity == "warn" && diagnostic.FieldPath == "host-key" {
			foundWarn = true
		}
	}
	if !foundWarn {
		t.Fatalf("空 host-key 必须产生安全 warn: %+v", emptyDiagnostics)
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

// TestManualProtocolAdapterManifestComplete 断言 19 个 manual 协议全部注册显式 adapter：
// 缺失集合必须为空（而不是只断言计数为 0），注册表也不得包含 manual 清单之外的协议。
func TestManualProtocolAdapterManifestComplete(t *testing.T) {
	missing := []string{}
	known := map[string]bool{}
	for _, protocol := range node.ManualProtocols() {
		known[protocol.Protocol] = true
		if _, ok := clashProtocolAdapters[protocol.Protocol]; !ok {
			missing = append(missing, protocol.Protocol)
		}
	}
	if len(missing) != 0 {
		t.Fatalf("manual 协议缺少显式 Clash adapter: %v", missing)
	}
	if got := legacyAdapterPendingCount(); got != 0 {
		t.Fatalf("Step 20 完成后 legacy adapter 待迁移数量必须为 0，实际 %d", got)
	}
	for name := range clashProtocolAdapters {
		if !known[name] {
			t.Fatalf("注册了非 manual 协议 adapter: %s", name)
		}
	}
}

// TestClashAdapterRegistryRejectsUnknownProtocol 断言未注册 adapter 的协议不再走 legacy 投影，
// 而是返回显式错误；这是「正式装配不能绕过检查」的前置条件。
func TestClashAdapterRegistryRejectsUnknownProtocol(t *testing.T) {
	svc := &Service{}
	nd := &nodeData{Protocol: "no-such-protocol", RenderName: "unknown-node", Host: "example.com", Port: 443}
	proxy, diagnostics, err := svc.buildClashProxy(nd, true)
	if err == nil {
		t.Fatalf("未注册 adapter 必须返回显式错误而不是 legacy 投影: %+v diagnostics=%+v", proxy, diagnostics)
	}
	if proxy != nil {
		t.Fatalf("未注册 adapter 不得返回任何代理条目: %+v", proxy)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "legacy_adapter_pending" {
			t.Fatalf("legacy_adapter_pending 必须彻底消失: %+v", diagnostics)
		}
	}
}

// TestCheckAndFormalAssemblyUseSameRegisteredAdapterDraft 锁定 check 与正式装配注入同一份草稿：
// 服务端填充的 NodeID/Persisted/CurrentState 必须一致，且两条路径都经过同一 adapter。
func TestCheckAndFormalAssemblyUseSameRegisteredAdapterDraft(t *testing.T) {
	const protocol = "vmess"
	orig, had := clashProtocolAdapters[protocol]
	var captured ClashNodeDraft
	clashProtocolAdapters[protocol] = func(draft ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
		captured = draft
		return map[string]any{"custom": "value"}, nil, nil
	}
	defer func() {
		if had {
			clashProtocolAdapters[protocol] = orig
		} else {
			delete(clashProtocolAdapters, protocol)
		}
	}()

	svc := &Service{}
	_, err := svc.CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
		Target: "clash-yaml", Protocol: protocol, RenderName: "adapter-node",
		Host: "example.com", Port: 1080, Params: map[string]any{}, NodeID: 42, Persisted: true,
		State: node.CurrentState{Security: "tls"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if captured.NodeID != 42 || !captured.Persisted || captured.State.Security != "tls" {
		t.Fatalf("check 路径未注入完整生命周期状态: %+v", captured)
	}

	nd := &nodeData{NodeID: 42, Protocol: protocol, RenderName: "adapter-node", Host: "example.com", Port: 1080, CurrentState: node.CurrentState{Security: "tls"}}
	p, _, err := svc.buildClashProxy(nd, true)
	if err != nil {
		t.Fatal(err)
	}
	if value, ok := p.Get("custom"); !ok || value != "value" {
		t.Fatalf("正式装配未使用同一 adapter: %+v", p)
	}
}

// TestLegacyAdapterPendingCountTracksRegistry 断言缺失集合随注册收敛：
// 临时移除任一协议 adapter 后，缺失集合必须恰好重新包含该协议。
func TestLegacyAdapterPendingCountTracksRegistry(t *testing.T) {
	if before := legacyAdapterPendingCount(); before != 0 {
		t.Fatalf("基线 legacy 计数必须为 0，实际 %d", before)
	}
	const protocol = "trojan"
	orig, had := clashProtocolAdapters[protocol]
	delete(clashProtocolAdapters, protocol)
	defer func() {
		if had {
			clashProtocolAdapters[protocol] = orig
		}
	}()
	missing := []string{}
	for _, item := range node.ManualProtocols() {
		if _, ok := clashProtocolAdapters[item.Protocol]; !ok {
			missing = append(missing, item.Protocol)
		}
	}
	if len(missing) != 1 || missing[0] != protocol {
		t.Fatalf("缺失集合未随注册收敛: %v", missing)
	}
	if after := legacyAdapterPendingCount(); after != 1 {
		t.Fatalf("legacy 计数未随注册变化: after=%d", after)
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

// TestWireGuardClashAdapterWireShape 锁定 WireGuard adapter 的 v1.19.31 WireGuardOption 形状：
// single 输出顶层 peer 字段；peers 只输出 peers[]；reserved 以三整数数组输出；
// selector、_credential_id 与内核不消费的 tfo／mptcp 均不得进入 wire。
func TestWireGuardClashAdapterWireShape(t *testing.T) {
	svc := &Service{}
	proxyFields := func(params map[string]any, state node.CurrentState, host string, port int) (map[string]any, []node.TargetDiagnostic) {
		res, err := svc.CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
			Target: "clash-yaml", Protocol: "wireguard", RenderName: "wg-node",
			Host: host, Port: port, Params: params, State: state,
		})
		if err != nil {
			t.Fatalf("WireGuard 目标检查失败: %v", err)
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
		return decoded.Proxies[0], res.Diagnostics
	}

	singleState := node.CurrentState{Selectors: map[string]string{"peer_mode": "single"}}
	single, _ := proxyFields(map[string]any{
		"private-key": "priv-cipher", "public-key": "pub-key", "pre-shared-key": "psk-cipher",
		"reserved": []int{1, 2, 3}, "allowed-ips": []any{"0.0.0.0/0"},
		"ip": "192.0.2.2/32", "mtu": 1420, "workers": 4, "udp": true,
		"ip-stack":           map[string]any{"mode": "gvisor", "congestion-controller": "bbr3"},
		"remote-dns-resolve": true, "dns": []any{"1.1.1.1"},
		"tfo": true, "mptcp": true, "interface-name": "utun0",
	}, singleState, "example.com", 51820)
	if single["server"] != "example.com" || fmt.Sprint(single["port"]) != "51820" {
		t.Fatalf("single 模式必须输出顶层 endpoint: %+v", single)
	}
	for _, key := range []string{"public-key", "pre-shared-key", "allowed-ips", "ip", "ip-stack", "dns"} {
		if _, ok := single[key]; !ok {
			t.Fatalf("single 模式缺少关键字段 %s: %+v", key, single)
		}
	}
	reserved, ok := single["reserved"].([]any)
	if !ok || len(reserved) != 3 {
		t.Fatalf("reserved 必须以三整数数组输出: %#v", single["reserved"])
	}
	for _, key := range []string{"peer-mode", "peer_mode", "tfo", "mptcp"} {
		if _, ok := single[key]; ok {
			t.Fatalf("wire 不应输出 %s: %+v", key, single)
		}
	}

	peersState := node.CurrentState{Selectors: map[string]string{"peer_mode": "peers"}}
	peersProxy, _ := proxyFields(map[string]any{
		"private-key": "priv-cipher", "ip": "192.0.2.2/32",
		"peers": []any{
			map[string]any{"_credential_id": "11111111-1111-1111-1111-111111111111",
				"server": "peer-a", "port": 51820, "public-key": "pub-a",
				"allowed-ips": []any{"10.0.0.0/24"}, "pre-shared-key": "psk-a", "reserved": []int{4, 5, 6}},
			map[string]any{"_credential_id": "22222222-2222-2222-2222-222222222222",
				"server": "peer-b", "port": 51821, "public-key": "pub-b",
				"allowed-ips": []any{"10.0.1.0/24"}},
		},
	}, peersState, "example.com", 51820)
	for _, key := range []string{"server", "port", "public-key", "pre-shared-key", "allowed-ips"} {
		if _, ok := peersProxy[key]; ok {
			t.Fatalf("peers 模式不得输出顶层 %s: %+v", key, peersProxy)
		}
	}
	peers, ok := peersProxy["peers"].([]any)
	if !ok || len(peers) != 2 {
		t.Fatalf("peers 模式必须输出两条 Peer: %+v", peersProxy)
	}
	for _, value := range peers {
		peer := value.(map[string]any)
		if _, ok := peer["_credential_id"]; ok {
			t.Fatalf("内部 Peer 身份不得进入 wire: %+v", peer)
		}
		if peer["server"] == "" || peer["public-key"] == "" {
			t.Fatalf("Peer 必填字段缺失: %+v", peer)
		}
	}
}

// TestMieruClashAdapterWireShape 锁定 Mieru adapter 的 v1.19.31 MieruOption 形状：
// port 与 port-range 严格二选一，selector 与 state_only 永不进入 wire。
func TestMieruClashAdapterWireShape(t *testing.T) {
	svc := &Service{}
	proxyFields := func(params map[string]any, state node.CurrentState, host string, port int) map[string]any {
		res, err := svc.CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
			Target: "clash-yaml", Protocol: "mieru", RenderName: "mieru-node",
			Host: host, Port: port, Params: params, State: state,
		})
		if err != nil {
			t.Fatalf("Mieru 目标检查失败: %v", err)
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

	singleState := node.CurrentState{Selectors: map[string]string{"endpoint_mode": "single"}}
	single := proxyFields(map[string]any{
		"username": "user", "password": "pw-cipher", "transport": "TCP",
		"multiplexing": "MULTIPLEXING_HIGH", "handshake-mode": "HANDSHAKE_NO_WAIT",
	}, singleState, "example.com", 8964)
	if fmt.Sprint(single["port"]) != "8964" || single["server"] != "example.com" {
		t.Fatalf("single 模式必须输出顶层 endpoint: %+v", single)
	}
	for _, key := range []string{"port-range", "endpoint-mode", "endpoint_mode"} {
		if _, ok := single[key]; ok {
			t.Fatalf("single 模式不应输出 %s: %+v", key, single)
		}
	}
	if single["transport"] != "TCP" || single["multiplexing"] != "MULTIPLEXING_HIGH" {
		t.Fatalf("Mieru 枚举未按原值输出: %+v", single)
	}

	rangeState := node.CurrentState{Selectors: map[string]string{"endpoint_mode": "range"}}
	rng := proxyFields(map[string]any{
		"username": "user", "password": "pw-cipher", "transport": "UDP", "port-range": "1000-2000",
	}, rangeState, "example.com", 8964)
	if _, ok := rng["port"]; ok {
		t.Fatalf("range 模式不得输出顶层 port: %+v", rng)
	}
	if rng["port-range"] != "1000-2000" {
		t.Fatalf("range 模式必须输出 port-range: %+v", rng)
	}
	// range 模式下未设置的枚举不得被强写默认值。
	for _, key := range []string{"multiplexing", "handshake-mode", "traffic-pattern"} {
		if _, ok := rng[key]; ok {
			t.Fatalf("未设置的 %s 不得写入 wire: %+v", key, rng)
		}
	}
}

// TestMASQUEClashAdapterWireShape 锁定 MASQUE adapter 的 v1.19.31 MasqueOption 形状：
// network 由 selector 注入（quic 不写）、h3-l4proxy 不输出 udp 与 QUIC 调优、
// name-cert-verify 永不出现。
func TestMASQUEClashAdapterWireShape(t *testing.T) {
	svc := &Service{}
	proxyFields := func(params map[string]any, state node.CurrentState) map[string]any {
		res, err := svc.CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
			Target: "clash-yaml", Protocol: "masque", RenderName: "masque-node",
			Host: "example.com", Port: 443, Params: params, State: state,
		})
		if err != nil {
			t.Fatalf("MASQUE 目标检查失败: %v", err)
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

	base := map[string]any{"private-key": "priv-cipher", "public-key": "pub-key", "ip": "192.0.2.2/32",
		"sni": "example.com", "mtu": 1280, "skip-cert-verify": true}

	quic := proxyFields(map[string]any{"private-key": "priv-cipher", "public-key": "pub-key",
		"ip": "192.0.2.2/32", "sni": "example.com", "mtu": 1280, "skip-cert-verify": true,
		"udp": true, "congestion-controller": "bbr_meta_v2", "cwnd": 64, "bbr-profile": "standard",
		"ip-stack": map[string]any{"mode": "gvisor"},
	}, node.CurrentState{Selectors: map[string]string{"network_mode": "quic"}})
	if _, ok := quic["network"]; ok {
		t.Fatalf("quic 是内核默认分支，不得写入 network: %+v", quic)
	}
	for _, key := range []string{"udp", "congestion-controller", "cwnd", "bbr-profile", "ip-stack", "sni"} {
		if _, ok := quic[key]; !ok {
			t.Fatalf("quic 分支缺少 %s: %+v", key, quic)
		}
	}

	h2 := proxyFields(map[string]any{"private-key": "priv-cipher", "public-key": "pub-key",
		"ip": "192.0.2.2/32", "udp": true}, node.CurrentState{Selectors: map[string]string{"network_mode": "h2"}})
	if h2["network"] != "h2" {
		t.Fatalf("h2 分支必须输出 network: h2：%+v", h2)
	}
	if _, ok := h2["congestion-controller"]; ok {
		t.Fatalf("h2 分支不得输出 QUIC 调优字段: %+v", h2)
	}

	l4 := proxyFields(base, node.CurrentState{Selectors: map[string]string{"network_mode": "h3_l4proxy"}})
	if l4["network"] != "h3-l4proxy" {
		t.Fatalf("h3_l4proxy 必须映射为固定 tag 的 h3-l4proxy：%+v", l4)
	}
	for _, key := range []string{"udp", "congestion-controller", "cwnd", "bbr-profile", "network-mode", "network_mode", "name-cert-verify"} {
		if _, ok := l4[key]; ok {
			t.Fatalf("h3_l4proxy 分支不得输出 %s: %+v", key, l4)
		}
	}
}

// TestAnyTLSClashAdapterWireShape 锁定 AnyTLS adapter 的 v1.19.31 AnyTLSOption 形状：
// selector 不进入 wire，三种伪装对象互斥，主密码与 TLS 字段始终输出。
func TestAnyTLSClashAdapterWireShape(t *testing.T) {
	svc := &Service{}
	proxyFields := func(params map[string]any, state node.CurrentState, host string, port int) map[string]any {
		res, err := svc.CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
			Target: "clash-yaml", Protocol: "anytls", RenderName: "anytls-node",
			Host: host, Port: port, Params: params, State: state,
		})
		if err != nil {
			t.Fatalf("AnyTLS 目标检查失败: %v", err)
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

	plain := proxyFields(map[string]any{
		"password": "pw-cipher", "sni": "example.com", "alpn": []any{"h2"},
		"client-fingerprint": "chrome", "skip-cert-verify": true,
		"idle-session-check-interval": 30, "idle-session-timeout": 60, "min-idle-session": 2,
		"client-metadata": "meta", "disable-reuse": true, "udp": false,
	}, node.CurrentState{Selectors: map[string]string{"security_mode": "plain"}}, "example.com", 443)
	for _, key := range []string{"password", "sni", "alpn", "client-fingerprint", "skip-cert-verify",
		"idle-session-check-interval", "idle-session-timeout", "min-idle-session", "client-metadata", "disable-reuse"} {
		if _, ok := plain[key]; !ok {
			t.Fatalf("plain 分支缺少 %s: %+v", key, plain)
		}
	}
	for _, key := range []string{"security-mode", "security_mode", "shadow-tls-opts", "restls-opts", "jls-opts"} {
		if _, ok := plain[key]; ok {
			t.Fatalf("plain 分支不得输出 %s: %+v", key, plain)
		}
	}

	shadow := proxyFields(map[string]any{"password": "pw-cipher",
		"shadow-tls-opts": map[string]any{"password": "shadow-cipher", "version": "3"}},
		node.CurrentState{Selectors: map[string]string{"security_mode": "shadow_tls"}}, "example.com", 443)
	if options, ok := shadow["shadow-tls-opts"].(map[string]any); !ok || options["password"] != "REDACTED" || fmt.Sprint(options["version"]) != "3" {
		t.Fatalf("shadow_tls 分支必须输出当前伪装对象（凭据已脱敏）: %+v", shadow)
	}
	for _, key := range []string{"restls-opts", "jls-opts", "security-mode"} {
		if _, ok := shadow[key]; ok {
			t.Fatalf("shadow_tls 分支不得输出 %s: %+v", key, shadow)
		}
	}

	jls := proxyFields(map[string]any{"password": "pw-cipher",
		"jls-opts": map[string]any{"username": "user", "password": "jls-cipher"}},
		node.CurrentState{Selectors: map[string]string{"security_mode": "jls"}}, "example.com", 443)
	if options, ok := jls["jls-opts"].(map[string]any); !ok || options["username"] != "user" {
		t.Fatalf("jls 分支必须输出当前伪装对象: %+v", jls)
	}
}

// TestAnyTLSURIDiagnosticsNeverSilentlyDropActiveFields 覆盖 URI 不可表达活动字段的诊断。
func TestAnyTLSURIDiagnosticsNeverSilentlyDropActiveFields(t *testing.T) {
	svc := &Service{}
	check := func(params map[string]any, target string) node.CheckRenderResult {
		t.Helper()
		res, err := svc.CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
			Target: target, Protocol: "anytls", RenderName: "anytls-uri",
			Host: "example.com", Port: 443, Params: params,
		})
		if err != nil {
			t.Fatalf("AnyTLS URI 检查失败: %v", err)
		}
		return res
	}
	hasDiagnostic := func(res node.CheckRenderResult, severity, code, path string) bool {
		for _, diagnostic := range res.Diagnostics {
			if diagnostic.Severity == severity && diagnostic.Code == code && diagnostic.FieldPath == path {
				return true
			}
		}
		return false
	}

	// 可无损表达的活动字段：不得产生阻断诊断。
	clean := check(map[string]any{"password": "pw-cipher", "sni": "example.com", "alpn": []any{"h2"},
		"client-fingerprint": "chrome", "skip-cert-verify": true, "udp": true}, "sr-subs")
	if hasBlockingTargetDiagnostic(clean.Diagnostics) {
		t.Fatalf("可表达字段不应阻断 URI: %+v", clean.Diagnostics)
	}
	if clean.Preview == "" {
		t.Fatal("可表达字段必须生成 URI 预览")
	}

	// mTLS 与三种伪装对象属于核心语义，无法表达时必须 skip 而不是静默丢弃。
	for _, tc := range []struct {
		name   string
		params map[string]any
		path   string
	}{
		{name: "mtls", params: map[string]any{"password": "p", "certificate": "CERT", "private-key": "KEY"}, path: "certificate"},
		{name: "shadow tls", params: map[string]any{"password": "p", "shadow-tls-opts": map[string]any{"password": "s"}}, path: "shadow-tls-opts"},
		{name: "restls", params: map[string]any{"password": "p", "restls-opts": map[string]any{"password": "r", "version-hint": "tls13"}}, path: "restls-opts"},
		{name: "jls", params: map[string]any{"password": "p", "jls-opts": map[string]any{"username": "u", "password": "j"}}, path: "jls-opts"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := check(tc.params, "sr-subs")
			if !hasDiagnostic(res, "error", "core_semantic_unexpressible", tc.path) {
				t.Fatalf("%s 必须返回核心语义不可表达诊断: %+v", tc.path, res.Diagnostics)
			}
			if res.Preview != "" {
				t.Fatalf("阻断时必须不返回预览: %s", res.Preview)
			}
		})
	}

	// 高级调优字段可表达但会丢失：warn 且保留预览。
	partial := check(map[string]any{"password": "p", "sni": "example.com",
		"idle-session-timeout": 60, "client-metadata": "meta"}, "sr-subs")
	if !hasDiagnostic(partial, "warn", "uri_partial_fields", "idle-session-timeout") {
		t.Fatalf("不可表达的高级字段必须返回 warn: %+v", partial.Diagnostics)
	}
	if partial.Preview == "" {
		t.Fatal("仅 warn 时必须保留 URI 预览")
	}
}

// TestShadowQUICClashAdapterWireShape 锁定 ShadowQUIC adapter 的 v1.19.31 ShadowQuicOption 形状：
// TLS 只有 sni／alpn、quic-versions 有序去重输出、UOT 无附加版本、0-RTT 返回重放风险 warn。
func TestShadowQUICClashAdapterWireShape(t *testing.T) {
	svc := &Service{}
	proxyFields := func(params map[string]any) (map[string]any, []node.TargetDiagnostic) {
		res, err := svc.CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
			Target: "clash-yaml", Protocol: "shadowquic", RenderName: "squic-node",
			Host: "example.com", Port: 443, Params: params,
		})
		if err != nil {
			t.Fatalf("ShadowQUIC 目标检查失败: %v", err)
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
		return decoded.Proxies[0], res.Diagnostics
	}

	proxy, diagnostics := proxyFields(map[string]any{
		"username": "squic-user", "password": "pw-cipher", "sni": "example.com",
		"alpn": []any{"h3"}, "quic-versions": []any{"v2", "v1"},
		"udp-over-stream": true, "zero-rtt": true, "keep-alive-interval": 30,
		"congestion-controller": "bbr_meta_v2", "up": "100 Mbps", "down": "100 Mbps",
		"cwnd": 64, "bbr-profile": "standard", "recv-window-conn": 1024, "recv-window": 2048,
		"disable-mtu-discovery": true, "max-datagram-frame-size": 1400, "max-open-streams": 1024,
		"tfo": true, "mptcp": true,
	})
	for _, key := range []string{"username", "password", "sni", "alpn", "quic-versions",
		"udp-over-stream", "zero-rtt", "keep-alive-interval", "congestion-controller",
		"up", "down", "cwnd", "bbr-profile", "recv-window-conn", "recv-window",
		"disable-mtu-discovery", "max-datagram-frame-size", "max-open-streams"} {
		if _, ok := proxy[key]; !ok {
			t.Fatalf("ShadowQUIC wire 缺少 %s: %+v", key, proxy)
		}
	}
	versions, ok := proxy["quic-versions"].([]any)
	if !ok || len(versions) != 2 || versions[0] != "v2" {
		t.Fatalf("quic-versions 必须保序输出: %#v", proxy["quic-versions"])
	}
	// 固定 tag 没有证书／ECH／skip-cert-verify，也没有 UOT 版本字段。
	for _, key := range []string{"skip-cert-verify", "certificate", "private-key", "ech-opts",
		"udp-over-stream-version", "tfo", "mptcp"} {
		if _, ok := proxy[key]; ok {
			t.Fatalf("ShadowQUIC wire 不得输出 %s: %+v", key, proxy)
		}
	}
	foundRisk := false
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "shadowquic_zero_rtt_replay_risk" {
			foundRisk = true
			if diagnostic.Severity != "warn" || diagnostic.FieldPath != "zero-rtt" {
				t.Fatalf("0-RTT 风险提示的级别／路径异常: %+v", diagnostic)
			}
			if !strings.Contains(diagnostic.Message, "重放") {
				t.Fatalf("0-RTT 风险提示缺少重放语义: %s", diagnostic.Message)
			}
		}
	}
	if !foundRisk {
		t.Fatalf("开启 0-RTT 必须返回重放风险 warn: %+v", diagnostics)
	}

	// 未开启 0-RTT 时不得产生风险提示，且缺省字段不写入 wire。
	quiet, quietDiagnostics := proxyFields(map[string]any{"username": "u", "password": "p"})
	for _, diagnostic := range quietDiagnostics {
		if diagnostic.Code == "shadowquic_zero_rtt_replay_risk" {
			t.Fatalf("未开启 0-RTT 不应提示重放风险: %+v", quietDiagnostics)
		}
	}
	for _, key := range []string{"quic-versions", "udp-over-stream", "zero-rtt", "cwnd",
		"bbr-profile", "up", "down", "max-datagram-frame-size", "max-open-streams"} {
		if _, ok := quiet[key]; ok {
			t.Fatalf("未设置的 %s 不得写入 wire: %+v", key, quiet)
		}
	}
}

// TestTrustTunnelClashAdapterWireShape 锁定 TrustTunnel adapter 的 v1.19.31 TrustTunnelOption 形状：
// reuse_mode 的分支数字互斥、selector 不进入 wire、quic 关闭不输出 QUIC 调优字段、
// ECH 关闭不输出对象，且固定 tag 消费 TFO／MPTCP。
func TestTrustTunnelClashAdapterWireShape(t *testing.T) {
	svc := &Service{}
	proxyFields := func(params map[string]any, state node.CurrentState) (map[string]any, []node.TargetDiagnostic) {
		res, err := svc.CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
			Target: "clash-yaml", Protocol: "trusttunnel", RenderName: "tt-node",
			Host: "example.com", Port: 443, Params: params, State: state,
		})
		if err != nil {
			t.Fatalf("TrustTunnel 目标检查失败: %v", err)
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
		return decoded.Proxies[0], res.Diagnostics
	}

	// connections 分支：只输出 max-connections／min-streams。
	connections, _ := proxyFields(map[string]any{
		"username": "tt-user", "password": "tt-secret", "sni": "example.com",
		"alpn": []any{"h2"}, "client-fingerprint": "chrome", "skip-cert-verify": true,
		"health-check": true, "udp": true,
		"max-connections": 8, "min-streams": 5, "tfo": true, "mptcp": true,
	}, node.CurrentState{Selectors: map[string]string{"reuse_mode": "connections"}})
	for _, key := range []string{"username", "password", "sni", "alpn", "client-fingerprint",
		"skip-cert-verify", "health-check", "udp", "max-connections", "min-streams", "tfo", "mptcp"} {
		if _, ok := connections[key]; !ok {
			t.Fatalf("connections 分支 wire 缺少 %s: %+v", key, connections)
		}
	}
	for _, key := range []string{"max-streams", "reuse-mode", "reuse_mode", "quic",
		"congestion-controller", "cwnd", "bbr-profile", "ech-opts"} {
		if _, ok := connections[key]; ok {
			t.Fatalf("connections 分支不得输出 %s: %+v", key, connections)
		}
	}

	// streams 分支：只输出 max-streams。
	streams, _ := proxyFields(map[string]any{"max-streams": 4, "alpn": []any{"h3"}, "quic": true},
		node.CurrentState{Selectors: map[string]string{"reuse_mode": "streams"}})
	if _, ok := streams["max-streams"]; !ok {
		t.Fatalf("streams 分支必须输出 max-streams: %+v", streams)
	}
	for _, key := range []string{"max-connections", "min-streams", "reuse-mode", "reuse_mode"} {
		if _, ok := streams[key]; ok {
			t.Fatalf("streams 分支不得输出 %s: %+v", key, streams)
		}
	}

	// quic 开启：输出 QUIC 调优字段；ECH 关闭不输出对象。
	quicOn, _ := proxyFields(map[string]any{
		"quic": true, "congestion-controller": "bbr_meta_v2", "cwnd": 64, "bbr-profile": "standard",
		"ech-opts": map[string]any{"enable": false, "config": "should-not-appear"},
	}, node.CurrentState{Selectors: map[string]string{"reuse_mode": "none"}})
	for _, key := range []string{"quic", "congestion-controller", "cwnd", "bbr-profile"} {
		if _, ok := quicOn[key]; !ok {
			t.Fatalf("quic 开启必须输出 %s: %+v", key, quicOn)
		}
	}
	if _, ok := quicOn["ech-opts"]; ok {
		t.Fatalf("ECH 关闭不得输出 ech-opts: %+v", quicOn)
	}
	if strings.Contains(quicOn["congestion-controller"].(string), "should-not-appear") {
		t.Fatalf("ECH 子字段不得泄漏到 wire: %+v", quicOn)
	}

	// quic 关闭：不输出三个 QUIC 调优字段。
	quicOff, _ := proxyFields(map[string]any{"quic": false},
		node.CurrentState{Selectors: map[string]string{"reuse_mode": "none"}})
	for _, key := range []string{"quic", "congestion-controller", "cwnd", "bbr-profile"} {
		if _, ok := quicOff[key]; ok {
			t.Fatalf("quic 关闭不得输出 %s: %+v", key, quicOff)
		}
	}
}

// TestOpenVPNClashAdapterWireShape 锁定 OpenVPN adapter 的 v1.19.31 OpenVPNOption 形状：
// selector 与导入元数据不进入 wire、非活动认证／TLS key 分支凭据不输出、
// tran-window 显式 0 必须保留，且 client-config 永不出现。
func TestOpenVPNClashAdapterWireShape(t *testing.T) {
	svc := &Service{}
	proxyFields := func(params map[string]any, state node.CurrentState) map[string]any {
		res, err := svc.CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
			Target: "clash-yaml", Protocol: "openvpn", RenderName: "ovpn-node",
			Host: "vpn.example.com", Port: 1194, Params: params, State: state,
		})
		if err != nil {
			t.Fatalf("OpenVPN 目标检查失败: %v", err)
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

	ca := "-----BEGIN CERTIFICATE-----\nTUlJQmZUA==\n-----END CERTIFICATE-----"
	userpassState := node.CurrentState{Selectors: map[string]string{"auth_mode": "userpass", "tls_key_mode": "tls_auth"}}
	proxy := proxyFields(map[string]any{
		"proto": "tcp", "dev": "tun", "cipher": "CHACHA20-POLY1305",
		"data-ciphers": []any{"AES-256-GCM", "CHACHA20-POLY1305"}, "data-ciphers-fallback": "AES-128-CBC",
		"auth": "SHA256", "comp-lzo": "no", "ca": ca,
		"username": "ovpn-user", "password": "ovpn-secret",
		"tls-auth": "ovpn-tls-auth", "key-direction": "1",
		"ping": 10, "ping-restart": 60, "tran-window": 0, "handshake-timeout": 30, "mtu": 1400,
		"udp": true, "peer-info": map[string]any{"IV_VER": "2.6"}, "tfo": true, "mptcp": true,
	}, userpassState)
	for _, key := range []string{"proto", "dev", "cipher", "data-ciphers", "auth", "comp-lzo", "ca",
		"username", "password", "tls-auth", "key-direction", "ping", "ping-restart", "handshake-timeout",
		"mtu", "udp", "peer-info", "tfo", "mptcp", "tran-window"} {
		if _, ok := proxy[key]; !ok {
			t.Fatalf("OpenVPN userpass wire 缺少 %s: %+v", key, proxy)
		}
	}
	if value, ok := proxy["tran-window"].(uint64); !ok || value != 0 {
		t.Fatalf("显式 tran-window=0 必须写入 wire: %#v", proxy["tran-window"])
	}
	for _, key := range []string{"auth-mode", "tls-key-mode", "auth_mode", "tls_key_mode",
		"cert", "key", "tls-crypt", "tls-crypt-v2", "client-config", "remote-dns-resolve", "dns"} {
		if _, ok := proxy[key]; ok {
			t.Fatalf("OpenVPN userpass wire 不得输出 %s: %+v", key, proxy)
		}
	}

	// cert_userpass 分支输出证书、私钥与用户名密码，且不输出 tls-auth。
	certState := node.CurrentState{Selectors: map[string]string{"auth_mode": "cert_userpass", "tls_key_mode": "tls_crypt"}}
	certProxy := proxyFields(map[string]any{
		"ca": ca, "cert": "-----BEGIN CERTIFICATE-----\nQ0VSVA==\n-----END CERTIFICATE-----",
		"key":      "-----BEGIN PRIVATE KEY-----\nS0VZ\n-----END PRIVATE KEY-----",
		"username": "ovpn-user", "password": "ovpn-secret", "tls-crypt": "ovpn-tls-crypt",
		"remote-dns-resolve": true, "dns": []any{"1.1.1.1"},
		"ip-stack": map[string]any{"mode": "gvisor", "congestion-controller": "bbr"},
	}, certState)
	for _, key := range []string{"ca", "cert", "key", "username", "password", "tls-crypt",
		"remote-dns-resolve", "dns", "ip-stack"} {
		if _, ok := certProxy[key]; !ok {
			t.Fatalf("OpenVPN cert_userpass wire 缺少 %s: %+v", key, certProxy)
		}
	}
	for _, key := range []string{"tls-auth", "key-direction", "tls-crypt-v2", "tran-window"} {
		if _, ok := certProxy[key]; ok {
			t.Fatalf("OpenVPN cert_userpass wire 不得输出 %s: %+v", key, certProxy)
		}
	}
	// 未设置的可选整数不得写出 0。
	for _, key := range []string{"ping", "ping-restart", "handshake-timeout", "mtu"} {
		if _, ok := certProxy[key]; ok {
			t.Fatalf("未设置的 %s 不得写入 wire: %+v", key, certProxy)
		}
	}
}

// ===== Build32 Step 20：首批四协议显式 adapter 与 check／正式装配一致性 =====

// step20ProxyFields 通过节点检查路径取出单节点 Clash 字段，供 wire shape 断言复用。
func step20ProxyFields(t *testing.T, protocol, host string, port int, params map[string]any) (map[string]any, []node.TargetDiagnostic) {
	t.Helper()
	res, err := (&Service{}).CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
		Target: "clash-yaml", Protocol: protocol, RenderName: protocol + "-node",
		Host: host, Port: port, Params: params,
	})
	if err != nil {
		t.Fatalf("%s 目标检查失败: %v", protocol, err)
	}
	return step20FirstProxy(t, res.Preview), res.Diagnostics
}

// step20FirstProxy 解析单节点检查预览并返回第一个代理映射。
func step20FirstProxy(t *testing.T, preview string) map[string]any {
	t.Helper()
	var decoded struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := gyaml.Unmarshal([]byte(preview), &decoded); err != nil {
		t.Fatalf("解析检查预览失败: %v\n%s", err, preview)
	}
	if len(decoded.Proxies) != 1 {
		t.Fatalf("检查预览代理数量异常: %s", preview)
	}
	return decoded.Proxies[0]
}

// step20MaskSecrets 按值把已构造结构中的敏感叶子替换为 REDACTED，用于比较两侧语义。
func step20MaskSecrets(value any, secrets []string) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = step20MaskSecrets(item, secrets)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i := range typed {
			out[i] = step20MaskSecrets(typed[i], secrets)
		}
		return out
	case string:
		for _, secret := range secrets {
			if secret != "" && typed == secret {
				return "REDACTED"
			}
		}
		return typed
	default:
		return typed
	}
}

// TestSSClashAdapterWireShape 锁定 SS adapter 的 v1.19.31 ShadowSocksOption 形状：
// 只输出点名 wire key，插件走既有的存储键 → plugin／plugin-opts 投影，
// 禁用 feature 对象与显式 false 不得进入 wire。
func TestSSClashAdapterWireShape(t *testing.T) {
	fields, _ := step20ProxyFields(t, "ss", "example.com", 8388, map[string]any{
		"cipher": "aes-256-gcm", "password": "ss-secret",
		"plugin": "shadow-tls", "shadow-tls-opts": map[string]any{"host": "cdn.example.com", "password": "st-secret", "version": "3"},
		"udp-over-tcp": true, "udp-over-tcp-version": "2", "client-fingerprint": "chrome",
		"smux": map[string]any{"enabled": false, "protocol": "smux"},
		"tfo":  true, "mptcp": true, "interface-name": "utun0",
	})
	if fields["cipher"] != "aes-256-gcm" || fields["password"] != "REDACTED" {
		t.Fatalf("SS 必填字段缺失或未脱敏: %+v", fields)
	}
	opts, ok := fields["plugin-opts"].(map[string]any)
	if !ok || fields["plugin"] != "shadow-tls" || opts["host"] != "cdn.example.com" {
		t.Fatalf("SS 插件投影异常: %+v", fields)
	}
	for _, key := range []string{"obfs-opts", "v2ray-plugin-opts", "shadow-tls-opts", "restls-opts", "plugin-opts-storage", "security", "smux"} {
		if _, ok := fields[key]; ok {
			t.Fatalf("SS wire 不得输出 %s: %+v", key, fields)
		}
	}
	for _, key := range []string{"udp-over-tcp", "udp-over-tcp-version", "client-fingerprint", "tfo", "mptcp", "interface-name"} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("SS wire 缺少活动字段 %s: %+v", key, fields)
		}
	}

	disabled, _ := step20ProxyFields(t, "ss", "example.com", 8388, map[string]any{
		"cipher": "aes-256-gcm", "password": "ss-secret", "udp": false,
	})
	for _, key := range []string{"udp", "plugin", "plugin-opts"} {
		if _, ok := disabled[key]; ok {
			t.Fatalf("SS 未启用字段不得进入 wire: %s %+v", key, disabled)
		}
	}

	unknown, _ := step20ProxyFields(t, "ss", "example.com", 8388, map[string]any{
		"cipher": "aes-256-gcm", "password": "ss-secret",
		"plugin": "custom-plugin", "plugin-opts": map[string]any{"flag": "on"},
	})
	if unknown["plugin"] != "custom-plugin" {
		t.Fatalf("未知插件必须原样保留 plugin: %+v", unknown)
	}
	if unknown["plugin-opts"].(map[string]any)["flag"] != "on" {
		t.Fatalf("未知插件参数必须原样保留: %+v", unknown)
	}
}

// TestVMessClashAdapterWireShape 锁定 VMess adapter 的 v1.19.31 VmessOption 形状：
// alterId／cipher 无 omitempty，必须始终输出；h2-opts.host 是内核数组；
// 禁用 feature 与显式 false 不得进入 wire。
func TestVMessClashAdapterWireShape(t *testing.T) {
	fields, _ := step20ProxyFields(t, "vmess", "example.com", 443, map[string]any{
		"uuid": "vmess-uuid", "network": "ws", "tls": true, "servername": "example.com",
		"ws-opts": map[string]any{"path": "/vmess", "headers": map[string]any{"Host": "cdn.example.com"}},
		"alpn":    "h2,http/1.1", "packet-addr": true, "xudp": true,
		"smux": map[string]any{"enabled": false},
	})
	for _, key := range []string{"uuid", "alterId", "cipher", "network", "tls", "servername", "ws-opts", "packet-addr", "xudp"} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("VMess wire 缺少 %s: %+v", key, fields)
		}
	}
	if fmt.Sprint(fields["alterId"]) != "0" || fmt.Sprint(fields["cipher"]) != "auto" {
		t.Fatalf("VMess 必须补齐内核必需的 alterId=0／cipher=auto: %+v", fields)
	}
	alpn, ok := fields["alpn"].([]any)
	if !ok || len(alpn) != 2 {
		t.Fatalf("VMess alpn 必须为 YAML 数组: %#v", fields["alpn"])
	}
	for _, key := range []string{"security", "smux"} {
		if _, ok := fields[key]; ok {
			t.Fatalf("VMess wire 不得输出 %s: %+v", key, fields)
		}
	}

	plain, _ := step20ProxyFields(t, "vmess", "example.com", 443, map[string]any{
		"uuid": "vmess-uuid", "network": "tcp", "tls": false,
		"skip-cert-verify": false, "servername": "example.com",
	})
	for _, key := range []string{"tls", "skip-cert-verify", "servername"} {
		if _, ok := plain[key]; ok {
			t.Fatalf("VMess TLS 关闭时不得输出 %s: %+v", key, plain)
		}
	}

	h2, _ := step20ProxyFields(t, "vmess", "example.com", 443, map[string]any{
		"uuid": "vmess-uuid", "network": "h2", "tls": true,
		"h2-opts": map[string]any{"path": "/h2", "host": "h2.example.com"},
	})
	hosts, ok := h2["h2-opts"].(map[string]any)["host"].([]any)
	if !ok || len(hosts) != 1 || hosts[0] != "h2.example.com" {
		t.Fatalf("h2-opts.host 必须按固定 tag 的 []string 输出: %#v", h2["h2-opts"])
	}
}

// TestVLESSClashAdapterWireShape 锁定 VLESS adapter 的 v1.19.31 VlessOption 形状：
// 固定 tag 没有的 TLS／ECH／伪装字段不得因共享 schema 被输出，ws 旧别名只进 ws-opts。
func TestVLESSClashAdapterWireShape(t *testing.T) {
	fields, _ := step20ProxyFields(t, "vless", "example.com", 443, map[string]any{
		"uuid": "vless-uuid", "network": "tcp", "tls": true, "servername": "example.com",
		"flow":        "xtls-rprx-vision",
		"encryption":  "none",
		"ech-opts":    map[string]any{"enable": true, "config": "ech-config"},
		"certificate": "client-cert", "private-key": "client-key", "name-cert-verify": "verify.example.com",
		"shadow-tls-opts": map[string]any{"password": "st-secret", "version": "3"},
		"jls-opts":        map[string]any{"username": "u", "password": "p"},
		"smux":            map[string]any{"enabled": false},
	})
	for _, key := range []string{"uuid", "network", "tls", "servername", "flow", "encryption"} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("VLESS wire 缺少 %s: %+v", key, fields)
		}
	}
	for _, key := range []string{"security", "ws-path", "ws-headers", "ech-opts", "certificate", "private-key",
		"name-cert-verify", "shadow-tls-opts", "jls-opts", "smux"} {
		if _, ok := fields[key]; ok {
			t.Fatalf("VLESS wire 不得输出 %s: %+v", key, fields)
		}
	}

	ws, _ := step20ProxyFields(t, "vless", "example.com", 443, map[string]any{
		"uuid": "vless-uuid", "network": "ws", "tls": true, "servername": "example.com",
		"ws-opts":    map[string]any{"path": "/vless", "headers": map[string]any{"Host": "cdn.example.com"}},
		"ws-headers": map[string]any{"X-Alias": "1"},
		"ws-path":    "/legacy",
	})
	for _, key := range []string{"ws-path", "ws-headers"} {
		if _, ok := ws[key]; ok {
			t.Fatalf("VLESS ws 旧别名不得单独进入 wire: %s %+v", key, ws)
		}
	}
	headers, ok := ws["ws-opts"].(map[string]any)["headers"].(map[string]any)
	if !ok || headers["Host"] != "cdn.example.com" || headers["X-Alias"] != "1" {
		t.Fatalf("VLESS ws 旧别名必须收敛进 ws-opts.headers: %#v", ws["ws-opts"])
	}
}

// TestTrojanClashAdapterWireShape 锁定 Trojan adapter 的 v1.19.31 TrojanOption 形状：
// 内层 SS 只在启用时输出，固定 tag 没有的 TLS／ECH／伪装字段不得输出。
func TestTrojanClashAdapterWireShape(t *testing.T) {
	fields, _ := step20ProxyFields(t, "trojan", "example.com", 443, map[string]any{
		"password": "trojan-secret", "network": "tcp", "sni": "example.com", "alpn": "h2,http/1.1",
		"skip-cert-verify": false, "client-fingerprint": "chrome",
		"ss-opts":          map[string]any{"enabled": true, "method": "aes-128-gcm", "password": "inner-secret"},
		"ech-opts":         map[string]any{"enable": true},
		"certificate":      "client-cert",
		"private-key":      "client-key",
		"name-cert-verify": "verify.example.com",
	})
	for _, key := range []string{"password", "network", "sni", "alpn", "client-fingerprint", "ss-opts"} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("Trojan wire 缺少 %s: %+v", key, fields)
		}
	}
	for _, key := range []string{"skip-cert-verify", "security", "ech-opts", "certificate", "private-key",
		"name-cert-verify", "reality-opts", "shadow-tls-opts", "restls-opts", "jls-opts"} {
		if _, ok := fields[key]; ok {
			t.Fatalf("Trojan wire 不得输出 %s: %+v", key, fields)
		}
	}
	ssOpts, ok := fields["ss-opts"].(map[string]any)
	if !ok || ssOpts["method"] != "aes-128-gcm" || ssOpts["password"] != "REDACTED" {
		t.Fatalf("Trojan 内层 SS 形状异常: %#v", fields["ss-opts"])
	}

	disabled, _ := step20ProxyFields(t, "trojan", "example.com", 443, map[string]any{
		"password": "trojan-secret", "network": "tcp",
		"ss-opts": map[string]any{"enabled": false, "method": "aes-128-gcm", "password": "inner-secret"},
	})
	if _, ok := disabled["ss-opts"]; ok {
		t.Fatalf("内层 SS 未启用时不得进入 wire: %+v", disabled)
	}
}

// TestFirstBatchCheckPreviewMatchesFormalAssembly 证明 check 预览与正式装配单节点片段
// 在脱敏差异之外语义一致：name／type／endpoint／协议字段与 endpoint policy 完全一致。
func TestFirstBatchCheckPreviewMatchesFormalAssembly(t *testing.T) {
	cases := []struct {
		protocol string
		host     string
		port     int
		params   map[string]any
		secrets  []string
	}{
		{"ss", "example.com", 8388, map[string]any{
			"cipher": "aes-256-gcm", "password": "ss-secret",
			"plugin": "obfs", "obfs-opts": map[string]any{"mode": "http", "host": "cdn.example.com"},
		}, []string{"ss-secret"}},
		{"vmess", "example.com", 443, map[string]any{
			"uuid": "vmess-uuid", "network": "ws", "tls": true, "servername": "example.com",
			"ws-opts": map[string]any{"path": "/vmess", "headers": map[string]any{"Host": "cdn.example.com"}},
		}, []string{"vmess-uuid"}},
		{"vless", "example.com", 443, map[string]any{
			"uuid": "vless-uuid", "network": "tcp", "tls": true, "servername": "example.com",
			"reality-opts": map[string]any{"public-key": "reality-pk", "short-id": "01234567"},
		}, []string{"vless-uuid"}},
		{"trojan", "example.com", 443, map[string]any{
			"password": "trojan-secret", "network": "grpc", "sni": "example.com",
			"grpc-opts": map[string]any{"grpc-service-name": "trojan"},
		}, []string{"trojan-secret"}},
	}
	const nodeID int64 = 7
	for _, tc := range cases {
		t.Run(tc.protocol, func(t *testing.T) {
			svc := &Service{}
			res, err := svc.CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
				Target: "clash-yaml", Protocol: tc.protocol, RenderName: tc.protocol + "-node",
				Host: tc.host, Port: tc.port, Params: tc.params, NodeID: nodeID, Persisted: true,
			})
			if err != nil {
				t.Fatalf("节点检查失败: %v", err)
			}
			nd := &nodeData{
				NodeID: nodeID, Protocol: tc.protocol, RenderName: tc.protocol + "-node",
				Host: tc.host, Port: tc.port, ProtocolJSON: tc.params,
			}
			proxy, _, err := svc.buildClashProxy(nd, true)
			if err != nil {
				t.Fatalf("正式装配失败: %v", err)
			}
			content, err := marshalClashYAML(gyaml.MapSlice{{Key: "proxies", Value: []any{orderedMapToMapSlice(proxy)}}}, nil)
			if err != nil {
				t.Fatalf("序列化正式装配片段失败: %v", err)
			}
			formal := step20MaskSecrets(step20FirstProxy(t, string(content)), tc.secrets)
			preview := step20FirstProxy(t, res.Preview)
			if !reflect.DeepEqual(formal, preview) {
				t.Fatalf("check 预览与正式装配语义不一致:\nformal  = %#v\npreview = %#v", formal, preview)
			}
		})
	}
}

// TestClashCheckPreviewRedactsSecretsAfterConstruction 证明脱敏发生在构造之后的副本上：
// 预览必须保留结构与非敏感值，同时不出现任何凭据明文。
func TestClashCheckPreviewRedactsSecretsAfterConstruction(t *testing.T) {
	cases := []struct {
		protocol string
		port     int
		params   map[string]any
		secrets  []string
	}{
		{"ss", 8388, map[string]any{
			"cipher": "aes-256-gcm", "password": "ss-secret-value",
			"plugin": "shadow-tls", "shadow-tls-opts": map[string]any{"host": "cdn.example.com", "password": "st-secret-value"},
		}, []string{"ss-secret-value", "st-secret-value"}},
		{"vmess", 443, map[string]any{
			"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp", "tls": true,
		}, []string{"11111111-2222-3333-4444-555555555555"}},
		{"vless", 443, map[string]any{
			"uuid": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "network": "tcp", "tls": true,
		}, []string{"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"}},
		{"trojan", 443, map[string]any{
			"password": "trojan-secret-value", "network": "tcp", "sni": "example.com",
			"ss-opts": map[string]any{"enabled": true, "method": "aes-128-gcm", "password": "inner-secret-value"},
		}, []string{"trojan-secret-value", "inner-secret-value"}},
	}
	for _, tc := range cases {
		t.Run(tc.protocol, func(t *testing.T) {
			res, err := (&Service{}).CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
				Target: "clash-yaml", Protocol: tc.protocol, RenderName: tc.protocol + "-node",
				Host: "example.com", Port: tc.port, Params: tc.params,
			})
			if err != nil {
				t.Fatalf("节点检查失败: %v", err)
			}
			for _, secret := range tc.secrets {
				if strings.Contains(res.Preview, secret) {
					t.Fatalf("检查预览泄漏凭据 %q:\n%s", secret, res.Preview)
				}
			}
			if !strings.Contains(res.Preview, "example.com") {
				t.Fatalf("脱敏不得破坏结构或丢弃非敏感字段:\n%s", res.Preview)
			}
			for _, diagnostic := range res.Diagnostics {
				for _, secret := range tc.secrets {
					if strings.Contains(diagnostic.Message, secret) {
						t.Fatalf("诊断泄漏凭据 %q: %+v", secret, diagnostic)
					}
				}
			}
		})
	}
}

// TestClashRenderPlanUsesSameAdapterAsGeneratedYAML 证明渲染计划中的 manual 节点
// 与生成时写入 YAML 的节点来自同一显式 adapter：下载重渲染不会退回第二套拼装。
func TestClashRenderPlanUsesSameAdapterAsGeneratedYAML(t *testing.T) {
	svc, st, _ := newTestService(t)
	pid := insertPlatform(t, st, "yaml")
	insertManualNode(t, st, "节点A", "vmess", map[string]any{
		"uuid": "11111111-2222-3333-4444-555555555555", "network": "ws", "tls": true,
		"servername": "example.com", "ws-opts": map[string]any{"path": "/vmess"},
		"smux": map[string]any{"enabled": false},
	})
	insertGroup(t, st, "组A", "select", []string{"节点A"}, nil, true, false)
	res, err := svc.Render(context.Background(), GenerateInput{
		TargetSyntax: ClashYAML, PlatformID: pid,
		NodeNames: []string{"节点A"}, GroupNames: []string{"组A"},
		OverseasMembers: []string{"节点A"}, FallbackGroupMembers: []string{"🚀直接连接", "🌎国外流量"},
	})
	if err != nil {
		t.Fatalf("Clash Render 失败: %v", err)
	}
	var decoded struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := gyaml.Unmarshal(res.Content, &decoded); err != nil {
		t.Fatalf("解析 Clash 产物失败: %v", err)
	}
	var plan struct {
		ManualProxies []map[string]any `json:"manual_proxies"`
	}
	if err := json.Unmarshal(res.RenderPlan, &plan); err != nil {
		t.Fatalf("解析渲染计划失败: %v", err)
	}
	if len(decoded.Proxies) != 1 || len(plan.ManualProxies) != 1 {
		t.Fatalf("产物与计划节点数量不一致: yaml=%d plan=%d", len(decoded.Proxies), len(plan.ManualProxies))
	}
	yamlProxy := decoded.Proxies[0]
	planProxy := plan.ManualProxies[0]
	if planProxy["name"] != "节点A" {
		t.Fatalf("渲染计划必须使用 nodes.name 稳定键: %+v", planProxy)
	}
	delete(planProxy, "name")
	delete(yamlProxy, "name")
	// YAML 与 JSON 的整数解码类型不同（uint64／float64），统一按 JSON 规范化比较。
	yamlJSON, err := json.Marshal(yamlProxy)
	if err != nil {
		t.Fatalf("序列化产物节点失败: %v", err)
	}
	planJSON, err := json.Marshal(planProxy)
	if err != nil {
		t.Fatalf("序列化计划节点失败: %v", err)
	}
	if string(yamlJSON) != string(planJSON) {
		t.Fatalf("渲染计划与生成产物不同源:\nyaml=%s\nplan=%s", yamlJSON, planJSON)
	}
	if _, ok := yamlProxy["smux"]; ok {
		t.Fatalf("未启用的 smux 不得进入产物: %+v", yamlProxy)
	}
}

// TestNoManualProtocolReportsLegacyAdapterPending 断言 19 个 manual 协议的目标诊断中
// 彻底不再出现 legacy_adapter_pending：该 info 证据不得继续掩盖其它诊断。
func TestNoManualProtocolReportsLegacyAdapterPending(t *testing.T) {
	svc := &Service{}
	for _, protocol := range node.ManualProtocols() {
		t.Run(protocol.Protocol, func(t *testing.T) {
			res, err := svc.CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
				Target: "clash-yaml", Protocol: protocol.Protocol, RenderName: protocol.Protocol + "-node",
				Host: "example.com", Port: 443,
			})
			if err != nil && len(res.Diagnostics) == 0 {
				// adapter 自身的组合错误不属于本断言范围（例如缺少端点策略要求的端口）。
				return
			}
			for _, diagnostic := range res.Diagnostics {
				if diagnostic.Code == "legacy_adapter_pending" {
					t.Fatalf("manual 协议不得再返回 legacy_adapter_pending: %+v", res.Diagnostics)
				}
			}
		})
	}
}

// TestFirstBatchAdaptersExcludeStateOnlyAndInternalMetadata 断言首批四协议的 wire
// 只包含固定 tag 的真实字段：state_only、内部编辑元数据与插件存储键一律不得输出。
func TestFirstBatchAdaptersExcludeStateOnlyAndInternalMetadata(t *testing.T) {
	cases := []struct {
		protocol  string
		port      int
		params    map[string]any
		forbidden []string
	}{
		{"ss", 8388, map[string]any{
			"cipher": "aes-256-gcm", "password": "ss-secret", "security": "tls",
			"plugin": "obfs", "obfs-opts": map[string]any{"mode": "http", "host": "cdn.example.com"},
			"_credential_id": "internal-id", "ovpn_source_lines": map[string]any{"ca": 3},
		}, []string{"security", "auth-mode", "auth_mode", "_credential_id", "ovpn_source_lines",
			"obfs-opts", "v2ray-plugin-opts", "shadow-tls-opts", "restls-opts"}},
		{"vmess", 443, map[string]any{
			"uuid": "vmess-uuid", "network": "tcp", "tls": true, "security": "tls",
			"_credential_id": "internal-id",
		}, []string{"security", "auth_mode", "_credential_id"}},
		{"vless", 443, map[string]any{
			"uuid": "vless-uuid", "network": "ws", "tls": true,
			"ws-path": "/legacy", "ws-headers": map[string]any{"X-Alias": "1"},
			"_credential_id": "internal-id",
		}, []string{"security", "ws-path", "ws-headers", "_credential_id", "ech-opts", "shadow-tls-opts"}},
		{"trojan", 443, map[string]any{
			"password": "trojan-secret", "network": "tcp", "sni": "example.com",
			"ss-opts":        map[string]any{"enabled": false, "method": "aes-128-gcm", "password": "inner"},
			"_credential_id": "internal-id",
		}, []string{"security", "ech-opts", "jls-opts", "certificate", "private-key", "_credential_id", "ss-opts"}},
	}
	for _, tc := range cases {
		t.Run(tc.protocol, func(t *testing.T) {
			fields, _ := step20ProxyFields(t, tc.protocol, "example.com", tc.port, tc.params)
			for _, key := range tc.forbidden {
				if _, ok := fields[key]; ok {
					t.Fatalf("wire 不得输出 %s: %+v", key, fields)
				}
			}
		})
	}
}
