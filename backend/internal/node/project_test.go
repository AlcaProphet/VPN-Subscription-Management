package node

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSchemaConditionAndOptionMetadata(t *testing.T) {
	vless, err := GetProtocol("vless")
	if err != nil {
		t.Fatal(err)
	}
	security, ok := findSchemaField(vless.FormSchema, "security")
	if !ok || len(security.OptionItems) != 3 || security.OptionItems[2].Value != "reality" {
		t.Fatalf("VLESS security 元数据异常: %+v", security)
	}
	ws, ok := findSchemaField(vless.FormSchema, "ws-opts")
	if !ok || !ws.Matches(CurrentState{Network: "ws", Security: "none"}, "") || ws.Matches(CurrentState{Network: "grpc", Security: "none"}, "") || !ws.ShouldReset("network") {
		t.Fatalf("WS 条件元数据异常: %+v", ws)
	}
	if ws.Matches(CurrentState{Network: "ws"}, "sr-subs") && len(ws.When.Targets) != 0 {
		t.Fatal("无目标限定的 WS 字段不应因输出目标改变")
	}

	raw, err := json.Marshal(vless)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"when", "required_when", "reset_on", "option_items", "canonical_path"} {
		if !strings.Contains(string(raw), `"`+key+`"`) {
			t.Fatalf("协议序列化缺少新 schema 字段 %s: %s", key, raw)
		}
	}
}

func TestFirstBatchProtocolMetadata(t *testing.T) {
	ss, _ := GetProtocol("ss")
	vmess, _ := GetProtocol("vmess")
	trojan, _ := GetProtocol("trojan")

	cipher, _ := findSchemaField(ss.FormSchema, "cipher")
	if !cipher.AllowCustom || len(cipher.OptionItems) < 6 || !hasOption(cipher, "2022-blake3-aes-128-gcm") {
		t.Fatalf("SS 算法目录异常: %+v", cipher)
	}
	vmessCipher, _ := findSchemaField(vmess.FormSchema, "cipher")
	if !vmessCipher.AllowCustom || !hasOption(vmessCipher, "chacha20-poly1305") || !hasOption(vmessCipher, "zero") {
		t.Fatalf("VMess 算法目录异常: %+v", vmessCipher)
	}
	vlessForXHTTP, _ := GetProtocol("vless")
	xhttp, _ := findSchemaField(findSchemaFieldMust(t, vlessForXHTTP.FormSchema, "xhttp-opts").Properties, "mode")
	if hasOption(xhttp, "none") || xhttp.Default != nil {
		t.Fatalf("XHTTP mode 不应使用 none 默认: %+v", xhttp)
	}
	inner := findSchemaFieldMust(t, trojan.FormSchema, "ss-opts")
	if !hasNestedField(inner, "enabled") || !hasNestedField(inner, "method") || hasNestedField(inner, "cipher") {
		t.Fatalf("Trojan 内层 SS schema 异常: %+v", inner)
	}
	method := findNestedFieldMust(t, inner, "method")
	if len(method.Aliases) != 1 || method.Aliases[0] != "cipher" || !hasOption(method, "chacha20-ietf-poly1305") {
		t.Fatalf("Trojan 内层 SS method 别名/选项异常: %+v", method)
	}
	reality := findSchemaFieldMust(t, vmess.FormSchema, "reality-opts")
	if reality.When != nil || len(reality.OptionItems) != 0 || len(reality.TargetEvidence) == 0 {
		t.Fatalf("VMess REALITY 不应出现首批条件入口: %+v", reality)
	}
}

func TestProjectActiveDropsInactiveBranchesAndPreservesUnknown(t *testing.T) {
	proto, _ := GetProtocol("vless")
	state := CurrentState{Network: "grpc", Security: "tls"}
	params := map[string]any{
		"uuid":     "u",
		"security": "tls",
		"tls":      true,
		"ws-opts": map[string]any{
			"path": "/stale",
		},
		"grpc-opts": map[string]any{
			"grpc-service-name": "svc",
			"future":            map[string]any{"enabled": true},
		},
	}
	projected := ProjectActive(proto, state, params)
	if _, ok := projected["ws-opts"]; ok {
		t.Fatalf("切到 gRPC 后不应投影 ws-opts: %+v", projected)
	}
	grpc := projected["grpc-opts"].(map[string]any)
	if grpc["grpc-service-name"] != "svc" || grpc["future"] == nil {
		t.Fatalf("活动 gRPC 参数/未知键丢失: %+v", grpc)
	}
	if _, ok := projected["security"]; ok {
		t.Fatalf("统一 security 元数据不应进入实际活动输出: %+v", projected)
	}
	if projected["tls"] != true {
		t.Fatalf("TLS 活动参数未转换为既有 tls 字段: %+v", projected)
	}
}

func TestValidateCurrentStateRejectsKnownInvalidCombinations(t *testing.T) {
	cases := []struct {
		name   string
		proto  string
		state  CurrentState
		params map[string]any
		path   string
	}{
		{"VLESS XHTTP none", "vless", CurrentState{Network: "xhttp", Security: "tls"}, map[string]any{
			"uuid": "u", "tls": true, "network": "xhttp", "xhttp-opts": map[string]any{"mode": "none"},
		}, "xhttp-opts.mode"},
		{"SS auto", "ss", CurrentState{Security: "none"}, map[string]any{
			"cipher": "auto", "password": "p",
		}, "cipher"},
		{"Trojan h2", "trojan", CurrentState{Network: "h2", Security: "tls"}, map[string]any{
			"password": "p", "network": "h2",
		}, "network"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proto, _ := GetProtocol(tc.proto)
			err := ValidateCurrentState(proto, tc.state, tc.params)
			if err == nil || !strings.Contains(err.Error(), tc.path) {
				t.Fatalf("应定位 %s，实际 %v", tc.path, err)
			}
		})
	}
}

func hasOption(field FieldSchema, value string) bool {
	for _, item := range field.OptionItems {
		if item.Value == value {
			return true
		}
	}
	return false
}

func hasNestedField(field FieldSchema, name string) bool {
	_, ok := findSchemaField(field.Properties, name)
	return ok
}

func findNestedFieldMust(t *testing.T, field FieldSchema, name string) FieldSchema {
	t.Helper()
	return findSchemaFieldMust(t, field.Properties, name)
}

func findSchemaFieldMust(t *testing.T, fields []FieldSchema, name string) FieldSchema {
	t.Helper()
	field, ok := findSchemaField(fields, name)
	if !ok {
		t.Fatalf("缺少 schema 字段 %s", name)
	}
	return field
}
