// anytls_protocol_test.go：Build32 Step 15 AnyTLS 协议合同测试。
// 覆盖 security_mode selector、三种附加伪装互斥与切换清空、主密码生命周期、TLS／mTLS／ECH、
// 会话参数与固定 tag 的 idle-session 语义，以及 Reality 排除边界。
package node

import (
	"context"
	"strings"
	"testing"
)

func anytlsProto(t *testing.T) Protocol {
	t.Helper()
	proto, err := GetProtocol("anytls")
	if err != nil {
		t.Fatal(err)
	}
	return proto
}

func anytlsField(t *testing.T, name string) FieldSchema {
	t.Helper()
	for _, field := range anytlsProto(t).FormSchema {
		if field.Name == name {
			return field
		}
	}
	t.Fatalf("AnyTLS schema 缺少字段 %s", name)
	return FieldSchema{}
}

func anytlsBaseParams() map[string]any {
	return map[string]any{"password": "anytls-secret", "sni": "example.com"}
}

func anytlsState(mode string) *CurrentState {
	return &CurrentState{Selectors: map[string]string{"security_mode": mode}}
}

func createAnyTLS(t *testing.T, svc *Service, name string, params map[string]any, state *CurrentState) (*Node, error) {
	t.Helper()
	return svc.CreateManual(context.Background(), CreateManualInput{
		Name: name, Protocol: "anytls", Host: "example.com", Port: 443,
		ProtocolJSON: params, CurrentState: state,
	})
}

// TestAnyTLSSecurityModeSelectorDeclaration 覆盖 security_mode selector 与 Reality 排除。
func TestAnyTLSSecurityModeSelectorDeclaration(t *testing.T) {
	proto := anytlsProto(t)
	selector, ok := selectorSchemaByName(proto, "security_mode")
	if !ok || selector.SourceField != "" || selector.Default != "plain" {
		t.Fatalf("AnyTLS 必须声明 state_only security_mode selector: %+v", selector)
	}
	if strings.Join(selector.Values, ",") != "plain,shadow_tls,restls,jls" {
		t.Fatalf("security_mode 允许值异常: %+v", selector.Values)
	}
	for _, field := range proto.FormSchema {
		if strings.Contains(field.Name, "reality") {
			t.Fatalf("AnyTLS 不得开放 Reality：%s", field.Name)
		}
	}
	if got := DeriveCurrentState(proto, anytlsBaseParams()).Selectors["security_mode"]; got != "plain" {
		t.Fatalf("无附加安全对象应派生 plain，实际 %q", got)
	}
}

// TestAnyTLSMainPasswordSurvivesModeSwitch 覆盖主密码不随 security_mode 切换清除。
func TestAnyTLSMainPasswordSurvivesModeSwitch(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	created, err := createAnyTLS(t, svc, "anytls-main-password", anytlsBaseParams(), anytlsState("plain"))
	if err != nil {
		t.Fatalf("plain 创建失败: %v", err)
	}
	if _, exists := created.ProtocolJSON["shadow-tls-opts"]; exists {
		t.Fatalf("plain 模式不得存在附加安全对象: %+v", created.ProtocolJSON)
	}

	// 切到 shadow_tls：主密码必须保留（留空表示保留）。
	switched, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "anytls", Host: "example.com", Port: 443, BaseRevision: created.EditRevision,
		ProtocolJSON: map[string]any{"password": "", "sni": "example.com",
			"shadow-tls-opts": map[string]any{"password": "shadow-secret"}},
		CurrentState: anytlsState("shadow_tls"),
	})
	if err != nil {
		t.Fatalf("切换到 shadow_tls 失败: %v", err)
	}
	if len(switched.SavedSensitivePaths) == 0 || !contains(switched.SavedSensitivePaths, "password") {
		t.Fatalf("主密码必须保留为已保存密文: %v", switched.SavedSensitivePaths)
	}
	raw, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	main, _ := GetPath(raw.ProtocolJSON, "password")
	text, ok := main.(string)
	if !ok || !strings.HasPrefix(text, encPrefix) {
		t.Fatalf("主密码密文丢失: %v", main)
	}
}

// TestAnyTLSCamouflageObjectsMutuallyExclusive 覆盖三种附加伪装互斥与 A→B→A 不恢复。
func TestAnyTLSCamouflageObjectsMutuallyExclusive(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	created, err := createAnyTLS(t, svc, "anytls-camouflage", map[string]any{
		"password": "anytls-secret", "sni": "example.com",
		"shadow-tls-opts": map[string]any{"password": "shadow-secret", "version": "3"},
	}, anytlsState("shadow_tls"))
	if err != nil {
		t.Fatalf("shadow_tls 创建失败: %v", err)
	}
	if _, exists := created.ProtocolJSON["restls-opts"]; exists {
		t.Fatalf("shadow_tls 模式下不得存在 restls 对象: %+v", created.ProtocolJSON)
	}
	if _, exists := created.ProtocolJSON["jls-opts"]; exists {
		t.Fatalf("shadow_tls 模式下不得存在 jls 对象: %+v", created.ProtocolJSON)
	}

	// 切到 restls：旧 shadow-tls 对象必须清空且切回不恢复。
	restls, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "anytls", Host: "example.com", Port: 443, BaseRevision: created.EditRevision,
		ProtocolJSON: map[string]any{"password": "", "sni": "example.com",
			"restls-opts": map[string]any{"password": "restls-secret", "version-hint": "tls13"}},
		CurrentState: anytlsState("restls"),
	})
	if err != nil {
		t.Fatalf("切换到 restls 失败: %v", err)
	}
	if _, exists := restls.ProtocolJSON["shadow-tls-opts"]; exists {
		t.Fatalf("切换后必须清空 shadow-tls 对象: %+v", restls.ProtocolJSON)
	}

	// A→B→A：切回 shadow_tls 时旧对象已被清空，必须重新填写才能保存（不得复活旧伪装密码）。
	_, err = svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "anytls", Host: "example.com", Port: 443, BaseRevision: restls.EditRevision,
		ProtocolJSON: map[string]any{"password": "", "sni": "example.com"},
		CurrentState: anytlsState("shadow_tls"),
	})
	if err == nil || !strings.Contains(err.Error(), "shadow-tls-opts") {
		t.Fatalf("切回 shadow_tls 必须要求重新填写伪装对象，实际: %v", err)
	}
	back, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "anytls", Host: "example.com", Port: 443, BaseRevision: restls.EditRevision,
		ProtocolJSON: map[string]any{"password": "", "sni": "example.com",
			"shadow-tls-opts": map[string]any{"password": "shadow-new"}},
		CurrentState: anytlsState("shadow_tls"),
	})
	if err != nil {
		t.Fatalf("重新填写后切回 shadow_tls 应成功: %v", err)
	}
	raw, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	options, _ := raw.ProtocolJSON["shadow-tls-opts"].(map[string]any)
	if options == nil || options["password"] == "shadow-secret" {
		t.Fatalf("切回 shadow_tls 不得恢复旧伪装密码: %+v", back.ProtocolJSON)
	}
}

// TestAnyTLSCamouflageBranchRequirements 覆盖三种伪装分支的条件必填与枚举。
func TestAnyTLSCamouflageBranchRequirements(t *testing.T) {
	svc, _, _ := newTestService(t)
	for _, tc := range []struct {
		name   string
		mode   string
		params map[string]any
		want   string
	}{
		{name: "shadow_tls missing object", mode: "shadow_tls", params: map[string]any{}, want: "shadow-tls-opts"},
		{name: "restls missing version hint", mode: "restls", want: "version-hint",
			params: map[string]any{"restls-opts": map[string]any{"password": "p"}}},
		{name: "jls missing password", mode: "jls", want: "password",
			params: map[string]any{"jls-opts": map[string]any{"username": "u"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := anytlsBaseParams()
			for key, value := range tc.params {
				params[key] = value
			}
			_, err := createAnyTLS(t, svc, "anytls-branch-"+strings.ReplaceAll(tc.name, " ", "-"), params, anytlsState(tc.mode))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%s 必须按 %s 拒绝，实际: %v", tc.name, tc.want, err)
			}
		})
	}

	// 非法的伪装枚举必须被字段级拒绝。
	params := anytlsBaseParams()
	params["shadow-tls-opts"] = map[string]any{"password": "p", "version": "9"}
	if _, err := createAnyTLS(t, svc, "anytls-shadow-version", params, anytlsState("shadow_tls")); err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("非法 ShadowTLS 版本必须按字段拒绝，实际: %v", err)
	}
	params = anytlsBaseParams()
	params["restls-opts"] = map[string]any{"password": "p", "version-hint": "tls11"}
	if _, err := createAnyTLS(t, svc, "anytls-restls-hint", params, anytlsState("restls")); err == nil || !strings.Contains(err.Error(), "version-hint") {
		t.Fatalf("非法 Restls 版本提示必须按字段拒绝，实际: %v", err)
	}
}

// TestAnyTLSTLSKeyPairAndECH 覆盖 mTLS 成对与 ECH 关闭清空。
func TestAnyTLSTLSKeyPairAndECH(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	params := anytlsBaseParams()
	params["certificate"] = "CERT"
	if _, err := createAnyTLS(t, svc, "anytls-mtls-partial", params, anytlsState("plain")); err == nil || !strings.Contains(err.Error(), "private-key") {
		t.Fatalf("mTLS 缺半必须按 private-key 拒绝，实际: %v", err)
	}
	params["private-key"] = "KEY"
	if _, err := createAnyTLS(t, svc, "anytls-mtls-paired", params, anytlsState("plain")); err != nil {
		t.Fatalf("成对 mTLS 应通过: %v", err)
	}

	// ECH 关闭清空 config／query-server-name。
	created, err := createAnyTLS(t, svc, "anytls-ech-off", map[string]any{
		"password": "anytls-secret", "sni": "example.com",
		"ech-opts": map[string]any{"enable": false, "config": "cfg", "query-server-name": "q.example.com"},
	}, anytlsState("plain"))
	if err != nil {
		t.Fatalf("ECH 关闭创建失败: %v", err)
	}
	if options, ok := created.ProtocolJSON["ech-opts"].(map[string]any); ok {
		for _, key := range []string{"config", "query-server-name"} {
			if _, exists := options[key]; exists {
				t.Fatalf("ech-opts.enable=false 必须清空 %s: %+v", key, created.ProtocolJSON)
			}
		}
	}
	// 启用时必须保留。
	enabled, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "anytls", Host: "example.com", Port: 443, BaseRevision: created.EditRevision,
		ProtocolJSON: map[string]any{"password": "", "sni": "example.com",
			"ech-opts": map[string]any{"enable": true, "config": "cfg"}},
		CurrentState: &CurrentState{Selectors: map[string]string{"security_mode": "plain"}, Features: []string{"ech"}},
	})
	if err != nil {
		t.Fatalf("ECH 启用更新失败: %v", err)
	}
	if options, ok := enabled.ProtocolJSON["ech-opts"].(map[string]any); !ok || options["config"] != "cfg" {
		t.Fatalf("ech-opts.enable=true 必须保留 config: %+v", enabled.ProtocolJSON)
	}
}

// TestAnyTLSSessionFieldsIndependent 覆盖会话参数独立活动与非负约束。
func TestAnyTLSSessionFieldsIndependent(t *testing.T) {
	svc, _, _ := newTestService(t)
	params := anytlsBaseParams()
	params["udp"] = false
	params["client-metadata"] = "meta"
	params["idle-session-check-interval"] = 30
	params["idle-session-timeout"] = 60
	params["min-idle-session"] = 2
	params["disable-reuse"] = true
	params["shadow-tls-opts"] = map[string]any{"password": "shadow-secret"}
	created, err := createAnyTLS(t, svc, "anytls-session", params, anytlsState("shadow_tls"))
	if err != nil {
		t.Fatalf("会话参数创建失败: %v", err)
	}
	for _, key := range []string{"udp", "client-metadata", "idle-session-check-interval", "idle-session-timeout", "min-idle-session", "disable-reuse"} {
		if _, exists := created.ProtocolJSON[key]; !exists {
			t.Fatalf("会话参数 %s 必须与安全模式独立保存: %+v", key, created.ProtocolJSON)
		}
	}
	for _, tc := range []struct {
		name  string
		key   string
		value any
	}{
		{name: "check interval negative", key: "idle-session-check-interval", value: -1},
		{name: "timeout negative", key: "idle-session-timeout", value: -1},
		{name: "min idle negative", key: "min-idle-session", value: -1},
		{name: "check interval fractional", key: "idle-session-check-interval", value: 1.5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invalid := anytlsBaseParams()
			invalid[tc.key] = tc.value
			if _, err := createAnyTLS(t, svc, "anytls-session-"+strings.ReplaceAll(tc.name, " ", "-"), invalid, anytlsState("plain")); err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("%s 必须按字段拒绝，实际: %v", tc.key, err)
			}
		})
	}
}

// TestAnyTLSIdleSessionRelation 覆盖固定 tag 的 5 秒静默替换阈值与 timeout 关系。
func TestAnyTLSIdleSessionRelation(t *testing.T) {
	svc, _, _ := newTestService(t)
	for _, tc := range []struct {
		name    string
		key     string
		value   int
		wantErr string
	}{
		{name: "zero means unset", key: "idle-session-check-interval", value: 0},
		{name: "minimum allowed", key: "idle-session-check-interval", value: 6},
		{name: "below silent replace threshold", key: "idle-session-check-interval", value: 5, wantErr: "idle-session-check-interval"},
		{name: "timeout below threshold", key: "idle-session-timeout", value: 3, wantErr: "idle-session-timeout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := anytlsBaseParams()
			params[tc.key] = tc.value
			_, err := createAnyTLS(t, svc, "anytls-relation-"+strings.ReplaceAll(tc.name, " ", "-"), params, anytlsState("plain"))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("%s=%d 应通过: %v", tc.key, tc.value, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("%s=%d 必须按字段拒绝，实际: %v", tc.key, tc.value, err)
			}
		})
	}

	// timeout 不得小于 check interval。
	params := anytlsBaseParams()
	params["idle-session-check-interval"] = 30
	params["idle-session-timeout"] = 10
	if _, err := createAnyTLS(t, svc, "anytls-relation-order", params, anytlsState("plain")); err == nil || !strings.Contains(err.Error(), "idle-session-timeout") {
		t.Fatalf("timeout 小于 check interval 必须按字段拒绝，实际: %v", err)
	}
	params["idle-session-timeout"] = 30
	if _, err := createAnyTLS(t, svc, "anytls-relation-equal", params, anytlsState("plain")); err != nil {
		t.Fatalf("timeout 等于 check interval 应通过: %v", err)
	}
}

// TestAnyTLSCamouflageSensitivePaths 覆盖三类伪装的敏感路径登记。
func TestAnyTLSCamouflageSensitivePaths(t *testing.T) {
	sensitive := anytlsProto(t).SensitiveFields
	for _, path := range []string{"password", "private-key", "shadow-tls-opts.password", "restls-opts.password", "restls-opts.restls-script", "jls-opts.password"} {
		if !contains(sensitive, path) {
			t.Fatalf("AnyTLS 敏感路径缺少 %s: %v", path, sensitive)
		}
	}
	if err := validateProtocolFieldTypes(anytlsProto(t)); err != nil {
		t.Fatalf("AnyTLS 字段类型／敏感性门禁失败: %v", err)
	}
}

// TestAnyTLSStateOnlyFieldRejected 覆盖 security-mode 不得写入 protocol_json。
func TestAnyTLSStateOnlyFieldRejected(t *testing.T) {
	svc, _, _ := newTestService(t)
	params := anytlsBaseParams()
	params["security-mode"] = "plain"
	if _, err := createAnyTLS(t, svc, "anytls-state-only", params, nil); err == nil {
		t.Fatal("state_only 字段 security-mode 不得写入 protocol_json")
	}
}
