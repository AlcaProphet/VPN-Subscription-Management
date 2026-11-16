// socks5_protocol_test.go：Build32 Step 5 SOCKS5 协议合同测试。
// 覆盖与 HTTP 共用的认证／TLS 语义、UDP 独立开关、无独立 SNI 与 URI 降级诊断。
package node

import (
	"context"
	"strings"
	"testing"
)

func socks5CreateInput(name string, params map[string]any, state *CurrentState) CreateManualInput {
	return CreateManualInput{
		Name: name, Protocol: "socks5", Host: "example.com", Port: 1080,
		ProtocolJSON: params, CurrentState: state,
	}
}

func TestSocks5AuthModeSelectorDeclaration(t *testing.T) {
	proto, err := GetProtocol("socks5")
	if err != nil {
		t.Fatal(err)
	}
	selector, ok := selectorSchemaByName(proto, "auth_mode")
	if !ok || selector.Default != "none" || selector.SourceField != "" {
		t.Fatalf("SOCKS5 必须声明 state_only auth_mode selector: %+v", selector)
	}
	if got := DeriveCurrentState(proto, map[string]any{"username": "u"}).Selectors["auth_mode"]; got != "basic" {
		t.Fatalf("存在凭据应派生 basic，实际 %q", got)
	}
	if got := DeriveCurrentState(proto, map[string]any{}).Selectors["auth_mode"]; got != "none" {
		t.Fatalf("无凭据应派生 none，实际 %q", got)
	}
}

func TestSocks5BasicRequiresPairedCredentials(t *testing.T) {
	svc, _, _ := newTestService(t)
	state := CurrentState{Selectors: map[string]string{"auth_mode": "basic"}}
	for _, tc := range []struct {
		name   string
		params map[string]any
		want   string
	}{
		{name: "missing password", params: map[string]any{"username": "u"}, want: "password"},
		{name: "missing username", params: map[string]any{"password": "p"}, want: "username"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateManual(context.Background(), socks5CreateInput("socks5-partial-"+strings.ReplaceAll(tc.name, " ", "-"), tc.params, &state))
			if err == nil {
				t.Fatal("basic 认证缺半对应被拒绝")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("错误应定位到 %s，实际: %v", tc.want, err)
			}
		})
	}
}

func TestSocks5AuthModeNoneClearsCredentials(t *testing.T) {
	svc, _, _ := newTestService(t)
	noneState := CurrentState{Selectors: map[string]string{"auth_mode": "none"}}
	created, err := svc.CreateManual(context.Background(), socks5CreateInput("socks5-none-clears",
		map[string]any{"username": "u", "password": "p", "udp": true}, &noneState))
	if err != nil {
		t.Fatalf("auth_mode=none 创建失败: %v", err)
	}
	if _, exists := created.ProtocolJSON["username"]; exists {
		t.Fatalf("auth_mode=none 必须清空 username: %+v", created.ProtocolJSON)
	}
	if created.ProtocolJSON["udp"] != true {
		t.Fatalf("UDP 是独立开关，不应被认证分支清空: %+v", created.ProtocolJSON)
	}
}

func TestSocks5TLSHasNoSniAndIsPaired(t *testing.T) {
	svc, _, _ := newTestService(t)
	// tls=false 清空全部 TLS 子字段，UDP 保留。
	created, err := svc.CreateManual(context.Background(), socks5CreateInput("socks5-tls-off",
		map[string]any{"tls": false, "skip-cert-verify": true, "fingerprint": "aa:bb", "udp": false}, nil))
	if err != nil {
		t.Fatalf("创建 tls=false 节点失败: %v", err)
	}
	for _, key := range []string{"skip-cert-verify", "fingerprint", "certificate", "private-key", "name-cert-verify"} {
		if _, exists := created.ProtocolJSON[key]; exists {
			t.Fatalf("tls=false 必须清空 %s: %+v", key, created.ProtocolJSON)
		}
	}
	if created.ProtocolJSON["udp"] != false {
		t.Fatalf("udp=false 必须独立保留: %+v", created.ProtocolJSON)
	}

	proto, _ := GetProtocol("socks5")
	for _, field := range proto.FormSchema {
		if field.Name == "sni" {
			t.Fatal("SOCKS5 固定 tag 的 Socks5Option 没有独立 sni，schema 不得声明该字段")
		}
	}

	_, err = svc.CreateManual(context.Background(), socks5CreateInput("socks5-mtls-partial",
		map[string]any{"tls": true, "certificate": "CERT"}, nil))
	if err == nil || !strings.Contains(err.Error(), "private-key") {
		t.Fatalf("mTLS 缺半对应返回 private-key 字段错误，实际: %v", err)
	}
	if _, err := svc.CreateManual(context.Background(), socks5CreateInput("socks5-mtls-paired",
		map[string]any{"tls": true, "certificate": "CERT", "private-key": "KEY", "udp": true}, nil)); err != nil {
		t.Fatalf("成对 mTLS 应通过: %v", err)
	}
}

func TestSocks5StateOnlyFieldRejectedInProtocolJSON(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.CreateManual(context.Background(), socks5CreateInput("socks5-state-only",
		map[string]any{"auth-mode": "basic"}, nil))
	if err == nil || !strings.Contains(err.Error(), "auth") {
		t.Fatalf("state_only 字段不得写入 protocol_json，实际: %v", err)
	}
}
