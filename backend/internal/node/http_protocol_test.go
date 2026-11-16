// http_protocol_test.go：Build32 Step 4 HTTP 协议合同测试。
// 覆盖 auth_mode selector、none/basic 分支、TLS 条件字段、认证与 mTLS 成对校验、
// 分支清空、headers 规则与敏感路径。
package node

import (
	"context"
	"strings"
	"testing"
)

func httpCreateInput(name string, params map[string]any, state *CurrentState) CreateManualInput {
	return CreateManualInput{
		Name: name, Protocol: "http", Host: "example.com", Port: 8080,
		ProtocolJSON: params, CurrentState: state,
	}
}

func TestHTTPAuthModeSelectorDeclaration(t *testing.T) {
	proto, err := GetProtocol("http")
	if err != nil {
		t.Fatal(err)
	}
	selector, ok := selectorSchemaByName(proto, "auth_mode")
	if !ok {
		t.Fatal("HTTP 必须声明 auth_mode selector")
	}
	if selector.Default != "none" || len(selector.Values) != 2 || selector.Values[0] != "none" || selector.Values[1] != "basic" {
		t.Fatalf("auth_mode selector 允许值/默认值异常: %+v", selector)
	}
	if selector.SourceField != "" {
		t.Fatalf("auth_mode 必须为 state_only selector: %+v", selector)
	}
	var modeField *FieldSchema
	for i := range proto.FormSchema {
		if proto.FormSchema[i].SelectorName == "auth_mode" {
			modeField = &proto.FormSchema[i]
		}
	}
	if modeField == nil || !modeField.StateOnly || modeField.Type != "select" {
		t.Fatalf("auth_mode 字段必须为 state_only select: %+v", modeField)
	}
}

func TestHTTPAuthModeDerivation(t *testing.T) {
	proto, _ := GetProtocol("http")
	if got := DeriveCurrentState(proto, map[string]any{}).Selectors["auth_mode"]; got != "none" {
		t.Fatalf("无凭据应派生 none，实际 %q", got)
	}
	withUser := DeriveCurrentState(proto, map[string]any{"username": "u", "password": "p"})
	if withUser.Selectors["auth_mode"] != "basic" {
		t.Fatalf("存在 username/password 应派生 basic，实际 %q", withUser.Selectors["auth_mode"])
	}
}

func TestHTTPCreateDerivesAuthModeFromCredentials(t *testing.T) {
	svc, _, _ := newTestService(t)
	created, err := svc.CreateManual(context.Background(), httpCreateInput("http-basic-derived",
		map[string]any{"username": "u", "password": "p"}, nil))
	if err != nil {
		t.Fatalf("创建 HTTP basic 节点失败: %v", err)
	}
	if got := created.CurrentState.Selectors["auth_mode"]; got != "basic" {
		t.Fatalf("创建后 auth_mode 应为 basic，实际 %q（state=%+v）", got, created.CurrentState)
	}
}

func TestHTTPBasicRequiresPairedCredentials(t *testing.T) {
	svc, _, _ := newTestService(t)
	for _, tc := range []struct {
		name   string
		params map[string]any
		want   string
	}{
		{name: "missing password", params: map[string]any{"username": "u"}, want: "password"},
		{name: "missing username", params: map[string]any{"password": "p"}, want: "username"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := CurrentState{Selectors: map[string]string{"auth_mode": "basic"}}
			_, err := svc.CreateManual(context.Background(), httpCreateInput("http-partial-"+strings.ReplaceAll(tc.name, " ", "-"), tc.params, &state))
			if err == nil {
				t.Fatal("basic 认证缺半对应被拒绝")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("错误应定位到 %s，实际: %v", tc.want, err)
			}
		})
	}
}

func TestHTTPAuthModeNoneClearsCredentials(t *testing.T) {
	svc, _, _ := newTestService(t)
	noneState := CurrentState{Selectors: map[string]string{"auth_mode": "none"}}
	created, err := svc.CreateManual(context.Background(), httpCreateInput("http-none-clears",
		map[string]any{"username": "u", "password": "p"}, &noneState))
	if err != nil {
		t.Fatalf("auth_mode=none 创建失败: %v", err)
	}
	if _, exists := created.ProtocolJSON["username"]; exists {
		t.Fatalf("auth_mode=none 必须清空 username: %+v", created.ProtocolJSON)
	}
	for _, saved := range created.SavedSensitivePaths {
		if saved == "password" {
			t.Fatalf("auth_mode=none 不得保留 password 密文: %+v", created.SavedSensitivePaths)
		}
	}

	basicState := CurrentState{Selectors: map[string]string{"auth_mode": "basic"}}
	kept, err := svc.CreateManual(context.Background(), httpCreateInput("http-basic-keeps",
		map[string]any{"username": "u", "password": "p"}, &basicState))
	if err != nil {
		t.Fatalf("auth_mode=basic 创建失败: %v", err)
	}
	if kept.ProtocolJSON["username"] != "u" {
		t.Fatalf("auth_mode=basic 必须保留 username: %+v", kept.ProtocolJSON)
	}
	foundPassword := false
	for _, saved := range kept.SavedSensitivePaths {
		if saved == "password" {
			foundPassword = true
		}
	}
	if !foundPassword {
		t.Fatalf("auth_mode=basic 必须保留 password 密文: %+v", kept.SavedSensitivePaths)
	}
}

func TestHTTPTLSFieldsConditionalAndPaired(t *testing.T) {
	svc, _, _ := newTestService(t)
	// tls=false 时清空并禁止输出 TLS 子字段。
	created, err := svc.CreateManual(context.Background(), httpCreateInput("http-tls-off",
		map[string]any{"tls": false, "sni": "sni.example.com", "skip-cert-verify": true, "fingerprint": "aa:bb"}, nil))
	if err != nil {
		t.Fatalf("创建 tls=false 节点失败: %v", err)
	}
	for _, key := range []string{"sni", "skip-cert-verify", "fingerprint"} {
		if _, exists := created.ProtocolJSON[key]; exists {
			t.Fatalf("tls=false 必须清空 %s: %+v", key, created.ProtocolJSON)
		}
	}

	// tls=true 且证书/私钥缺半对 → 字段级错误。
	_, err = svc.CreateManual(context.Background(), httpCreateInput("http-mtls-partial",
		map[string]any{"tls": true, "certificate": "CERT"}, nil))
	if err == nil {
		t.Fatal("mTLS 缺半对应被拒绝")
	}
	if !strings.Contains(err.Error(), "private-key") {
		t.Fatalf("mTLS 错误应定位到 private-key，实际: %v", err)
	}

	// 成对提供 → 通过。
	if _, err := svc.CreateManual(context.Background(), httpCreateInput("http-mtls-paired",
		map[string]any{"tls": true, "sni": "sni.example.com", "certificate": "CERT", "private-key": "KEY"}, nil)); err != nil {
		t.Fatalf("成对 mTLS 应通过: %v", err)
	}
}

func TestHTTPHeadersRules(t *testing.T) {
	svc, _, _ := newTestService(t)
	cases := []struct {
		name    string
		headers map[string]any
		want    string
	}{
		{name: "empty key", headers: map[string]any{"": "v"}, want: "headers"},
		{name: "blank key", headers: map[string]any{"  ": "v"}, want: "headers"},
		{name: "case insensitive duplicate", headers: map[string]any{"Host": "a", "host": "b"}, want: "headers"},
		{name: "non string value", headers: map[string]any{"X-Test": 7}, want: "headers"},
		{name: "reserved authorization header", headers: map[string]any{"Proxy-Authorization": "Basic xx"}, want: "headers"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateManual(context.Background(), httpCreateInput("http-headers-"+strings.ReplaceAll(tc.name, " ", "-"),
				map[string]any{"headers": tc.headers}, nil))
			if err == nil {
				t.Fatal("非法 headers 应被拒绝")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("错误应包含 %q，实际: %v", tc.want, err)
			}
		})
	}
	if _, err := svc.CreateManual(context.Background(), httpCreateInput("http-headers-ok",
		map[string]any{"headers": map[string]any{"Host": "a", "X-Custom": "b"}}, nil)); err != nil {
		t.Fatalf("合法 headers 应通过: %v", err)
	}
}

func TestHTTPStateOnlyFieldRejectedInProtocolJSON(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.CreateManual(context.Background(), httpCreateInput("http-state-only",
		map[string]any{"auth-mode": "basic"}, nil))
	if err == nil {
		t.Fatal("state_only 字段不得写入 protocol_json")
	}
	if !strings.Contains(err.Error(), "auth") {
		t.Fatalf("错误应指出 auth 字段，实际: %v", err)
	}
}

func TestHTTPSensitivePathsDeclared(t *testing.T) {
	svc, _, _ := newTestService(t)
	created, err := svc.CreateManual(context.Background(), httpCreateInput("http-sensitive",
		map[string]any{"username": "u", "password": "p", "tls": true, "certificate": "CERT", "private-key": "KEY"}, nil))
	if err != nil {
		t.Fatalf("创建 HTTP mTLS 节点失败: %v", err)
	}
	for _, path := range []string{"password", "private-key"} {
		found := false
		for _, saved := range created.SavedSensitivePaths {
			if saved == path {
				found = true
			}
		}
		if !found {
			t.Fatalf("敏感路径 %s 未登记为已保存密文: %+v", path, created.SavedSensitivePaths)
		}
	}
}
