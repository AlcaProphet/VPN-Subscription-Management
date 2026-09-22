// hysteria_protocol_test.go：Build32 Step 8 Hysteria v1 协议合同测试。
// 覆盖 auth/auth-str 二选一、base64 校验、up/down 唯一编辑入口与兼容别名归一化、
// protocol 枚举与 obfs-protocol 别名、端口跳跃语法、TLS 与 mTLS、窗口关系及敏感路径。
package node

import (
	"context"
	"strings"
	"testing"
)

func hysteriaCreateInput(name string, params map[string]any, state *CurrentState) CreateManualInput {
	return CreateManualInput{
		Name: name, Protocol: "hysteria", Host: "example.com", Port: 443,
		ProtocolJSON: params, CurrentState: state,
	}
}

func hysteriaAuthState(mode string) *CurrentState {
	return &CurrentState{Selectors: map[string]string{"auth_mode": mode}}
}

func hysteriaBaseParams() map[string]any {
	return map[string]any{"up": "100 Mbps", "down": "100 Mbps"}
}

func TestHysteriaAuthModeSelectorDeclaration(t *testing.T) {
	proto, err := GetProtocol("hysteria")
	if err != nil {
		t.Fatal(err)
	}
	selector, ok := selectorSchemaByName(proto, "auth_mode")
	if !ok || selector.SourceField != "" || selector.Default != "none" {
		t.Fatalf("Hysteria 必须声明 state_only auth_mode selector: %+v", selector)
	}
	want := []string{"none", "base64", "string"}
	if len(selector.Values) != len(want) {
		t.Fatalf("auth_mode 允许值异常: %+v", selector.Values)
	}
	for i := range want {
		if selector.Values[i] != want[i] {
			t.Fatalf("auth_mode 允许值顺序异常: %+v", selector.Values)
		}
	}
	for _, name := range []string{"auth", "auth-str"} {
		field, ok := findSchemaField(proto.FormSchema, name)
		if !ok || field.When == nil || field.RequiredWhen == nil {
			t.Fatalf("%s 必须声明分支条件与条件必填: %+v", name, field)
		}
	}
}

func TestHysteriaAuthModeDerivation(t *testing.T) {
	proto, _ := GetProtocol("hysteria")
	if got := DeriveCurrentState(proto, hysteriaBaseParams()).Selectors["auth_mode"]; got != "none" {
		t.Fatalf("无认证参数应派生 none，实际 %q", got)
	}
	base64Params := hysteriaBaseParams()
	base64Params["auth"] = "dGVzdC1hdXRo"
	if got := DeriveCurrentState(proto, base64Params).Selectors["auth_mode"]; got != "base64" {
		t.Fatalf("存在 auth 应派生 base64，实际 %q", got)
	}
	stringParams := hysteriaBaseParams()
	stringParams["auth-str"] = "plain-auth"
	if got := DeriveCurrentState(proto, stringParams).Selectors["auth_mode"]; got != "string" {
		t.Fatalf("存在 auth-str 应派生 string，实际 %q", got)
	}
}

func TestHysteriaAuthBranchExclusion(t *testing.T) {
	svc, _, _ := newTestService(t)

	base64Params := hysteriaBaseParams()
	base64Params["auth"] = "dGVzdC1hdXRo"
	base64Params["auth-str"] = "plain-auth"
	created, err := svc.CreateManual(context.Background(), hysteriaCreateInput("hy-auth-base64", base64Params, hysteriaAuthState("base64")))
	if err != nil {
		t.Fatalf("base64 分支创建失败: %v", err)
	}
	if _, exists := created.ProtocolJSON["auth-str"]; exists {
		t.Fatalf("base64 分支必须清空 auth-str: %+v", created.ProtocolJSON)
	}
	keptAuth := false
	for _, saved := range created.SavedSensitivePaths {
		if saved == "auth" {
			keptAuth = true
		}
	}
	if !keptAuth {
		t.Fatalf("base64 分支必须保留 auth 密文: %+v", created.SavedSensitivePaths)
	}

	stringParams := hysteriaBaseParams()
	stringParams["auth"] = "dGVzdC1hdXRo"
	stringParams["auth-str"] = "plain-auth"
	created, err = svc.CreateManual(context.Background(), hysteriaCreateInput("hy-auth-string", stringParams, hysteriaAuthState("string")))
	if err != nil {
		t.Fatalf("string 分支创建失败: %v", err)
	}
	if _, exists := created.ProtocolJSON["auth"]; exists {
		t.Fatalf("string 分支必须清空 auth: %+v", created.ProtocolJSON)
	}
	keptAuthStr := false
	for _, saved := range created.SavedSensitivePaths {
		if saved == "auth-str" {
			keptAuthStr = true
		}
	}
	if !keptAuthStr {
		t.Fatalf("string 分支必须保留 auth-str 密文: %+v", created.SavedSensitivePaths)
	}

	noneParams := hysteriaBaseParams()
	noneParams["auth"] = "dGVzdC1hdXRo"
	noneParams["auth-str"] = "plain-auth"
	created, err = svc.CreateManual(context.Background(), hysteriaCreateInput("hy-auth-none", noneParams, hysteriaAuthState("none")))
	if err != nil {
		t.Fatalf("none 分支创建失败: %v", err)
	}
	for _, key := range []string{"auth", "auth-str"} {
		if _, exists := created.ProtocolJSON[key]; exists {
			t.Fatalf("none 分支必须清空 %s: %+v", key, created.ProtocolJSON)
		}
	}
}

func TestHysteriaAuthBase64Validated(t *testing.T) {
	svc, _, _ := newTestService(t)
	params := hysteriaBaseParams()
	params["auth"] = "not base64!!"
	_, err := svc.CreateManual(context.Background(), hysteriaCreateInput("hy-auth-bad", params, hysteriaAuthState("base64")))
	if err == nil || !strings.Contains(err.Error(), "auth") {
		t.Fatalf("非法 base64 auth 必须返回字段级错误，实际: %v", err)
	}
}

func TestHysteriaUpDownRequiredAndNormalized(t *testing.T) {
	svc, _, _ := newTestService(t)
	// up/down 必填。
	for _, missing := range []string{"up", "down"} {
		params := hysteriaBaseParams()
		delete(params, missing)
		_, err := svc.CreateManual(context.Background(), hysteriaCreateInput("hy-missing-"+missing, params, hysteriaAuthState("none")))
		if err == nil || !strings.Contains(err.Error(), missing) {
			t.Fatalf("缺少 %s 必须返回字段级错误，实际: %v", missing, err)
		}
	}
	// 非法带宽字符串。
	for _, bad := range []string{"abc", "0", "100 mbps"} {
		params := hysteriaBaseParams()
		params["up"] = bad
		_, err := svc.CreateManual(context.Background(), hysteriaCreateInput("hy-bad-up-"+strings.ReplaceAll(bad, " ", "-"), params, hysteriaAuthState("none")))
		if err == nil || !strings.Contains(err.Error(), "up") {
			t.Fatalf("非法带宽 %q 必须返回字段级错误，实际: %v", bad, err)
		}
	}
	// 兼容别名 up-speed/down-speed 归一化为唯一入口 up/down，旧键不得保留。
	created, err := svc.CreateManual(context.Background(), hysteriaCreateInput("hy-speed-alias",
		map[string]any{"up-speed": 100, "down-speed": 200}, hysteriaAuthState("none")))
	if err != nil {
		t.Fatalf("兼容带宽输入应归一化后通过: %v", err)
	}
	if created.ProtocolJSON["up"] != "100 Mbps" || created.ProtocolJSON["down"] != "200 Mbps" {
		t.Fatalf("兼容带宽必须归一化为规范字符串: %+v", created.ProtocolJSON)
	}
	for _, key := range []string{"up-speed", "down-speed"} {
		if _, exists := created.ProtocolJSON[key]; exists {
			t.Fatalf("兼容带宽键 %s 不得落库: %+v", key, created.ProtocolJSON)
		}
	}
}

func TestHysteriaProtocolEnumAndAlias(t *testing.T) {
	svc, _, _ := newTestService(t)
	params := hysteriaBaseParams()
	params["protocol"] = "bogus"
	_, err := svc.CreateManual(context.Background(), hysteriaCreateInput("hy-protocol-bad", params, hysteriaAuthState("none")))
	if err == nil || !strings.Contains(err.Error(), "protocol") {
		t.Fatalf("非法 protocol 必须返回字段级错误，实际: %v", err)
	}
	// obfs-protocol 只作为兼容输入别名收敛到 protocol，不得成为第二套字段。
	created, err := svc.CreateManual(context.Background(), hysteriaCreateInput("hy-protocol-alias",
		map[string]any{"up": "100 Mbps", "down": "100 Mbps", "obfs-protocol": "wechat-video"}, hysteriaAuthState("none")))
	if err != nil {
		t.Fatalf("obfs-protocol 兼容输入应归一化后通过: %v", err)
	}
	if created.ProtocolJSON["protocol"] != "wechat-video" {
		t.Fatalf("obfs-protocol 必须收敛到 protocol: %+v", created.ProtocolJSON)
	}
	if _, exists := created.ProtocolJSON["obfs-protocol"]; exists {
		t.Fatalf("obfs-protocol 不得落库: %+v", created.ProtocolJSON)
	}
}

func TestHysteriaPortsSyntax(t *testing.T) {
	svc, _, _ := newTestService(t)
	for _, valid := range []string{"1000", "1000-2000", "1000,2000-3000"} {
		params := hysteriaBaseParams()
		params["ports"] = valid
		if _, err := svc.CreateManual(context.Background(), hysteriaCreateInput("hy-ports-"+strings.ReplaceAll(valid, ",", "-"), params, hysteriaAuthState("none"))); err != nil {
			t.Fatalf("合法端口跳跃 %q 应通过: %v", valid, err)
		}
	}
	for _, invalid := range []string{"abc", "0", "70000", "2000-1000", "1000-", "1000,,2000"} {
		params := hysteriaBaseParams()
		params["ports"] = invalid
		_, err := svc.CreateManual(context.Background(), hysteriaCreateInput("hy-ports-bad-"+strings.NewReplacer(",", "-", " ", "-").Replace(invalid), params, hysteriaAuthState("none")))
		if err == nil || !strings.Contains(err.Error(), "ports") {
			t.Fatalf("非法端口跳跃 %q 必须返回字段级错误，实际: %v", invalid, err)
		}
	}
}

func TestHysteriaTLSKeyPairAndWindowRelation(t *testing.T) {
	svc, _, _ := newTestService(t)
	params := hysteriaBaseParams()
	params["certificate"] = "CERT"
	_, err := svc.CreateManual(context.Background(), hysteriaCreateInput("hy-mtls-partial", params, hysteriaAuthState("none")))
	if err == nil || !strings.Contains(err.Error(), "private-key") {
		t.Fatalf("mTLS 缺半对必须定位 private-key，实际: %v", err)
	}

	params = hysteriaBaseParams()
	params["recv-window-conn"] = 4096
	params["recv-window"] = 1024
	_, err = svc.CreateManual(context.Background(), hysteriaCreateInput("hy-window-bad", params, hysteriaAuthState("none")))
	if err == nil || !strings.Contains(err.Error(), "recv-window") {
		t.Fatalf("连接窗口小于流窗口必须返回字段级错误，实际: %v", err)
	}

	params = hysteriaBaseParams()
	params["recv-window-conn"] = 1024
	params["recv-window"] = 4096
	params["hop-interval"] = 10
	if _, err := svc.CreateManual(context.Background(), hysteriaCreateInput("hy-window-ok", params, hysteriaAuthState("none"))); err != nil {
		t.Fatalf("合法窗口关系应通过: %v", err)
	}
}

func TestHysteriaStateOnlyAuthModeRejectedInProtocolJSON(t *testing.T) {
	svc, _, _ := newTestService(t)
	params := hysteriaBaseParams()
	params["auth-mode"] = "base64"
	_, err := svc.CreateManual(context.Background(), hysteriaCreateInput("hy-state-only", params, nil))
	if err == nil || !strings.Contains(err.Error(), "auth") {
		t.Fatalf("state_only auth-mode 不得写入 protocol_json，实际: %v", err)
	}
}

func TestHysteriaSensitivePathsDeclared(t *testing.T) {
	svc, _, _ := newTestService(t)
	params := hysteriaBaseParams()
	params["auth"] = "dGVzdC1hdXRo"
	params["obfs"] = "obfs-secret"
	created, err := svc.CreateManual(context.Background(), hysteriaCreateInput("hy-sensitive", params, hysteriaAuthState("base64")))
	if err != nil {
		t.Fatalf("创建 Hysteria 节点失败: %v", err)
	}
	for _, want := range []string{"auth", "obfs"} {
		found := false
		for _, saved := range created.SavedSensitivePaths {
			if saved == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("敏感路径 %s 未登记为已保存密文: %+v", want, created.SavedSensitivePaths)
		}
	}
}
