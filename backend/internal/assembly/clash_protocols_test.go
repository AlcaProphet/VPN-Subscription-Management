package assembly

import (
	"context"
	"fmt"
	"testing"

	gyaml "github.com/goccy/go-yaml"

	"vpn-sub/internal/node"
)

// TestLegacyAdapterPendingCountAfterMigratedProtocols 锁定当前已迁移协议的绝对计数：
// Step 4 HTTP、5 SOCKS5、6 SSH、7 Snell、8 Hysteria、9 Hysteria2、10 TUIC，待迁移协议为 12；
// 后续每个协议 Step 必须继续递减，Step 20 归零。
func TestLegacyAdapterPendingCountAfterMigratedProtocols(t *testing.T) {
	if got := legacyAdapterPendingCount(); got != 12 {
		t.Fatalf("已迁移 7 个协议后 legacy adapter 数量应为 12，实际 %d", got)
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
