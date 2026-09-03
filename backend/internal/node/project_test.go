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
	if reality.When == nil || !reality.ShouldReset("security") || len(reality.TargetEvidence) == 0 {
		t.Fatalf("VMess REALITY 应作为后续候选并仅在 REALITY 条件入口出现: %+v", reality)
	}
}

func TestFirstBatchGroupingAndSecurityOrdering(t *testing.T) {
	for _, protocolName := range []string{"vless", "vmess"} {
		proto, _ := GetProtocol(protocolName)
		security := findSchemaFieldMust(t, proto.FormSchema, "security")
		if security.Group != "connection" {
			t.Fatalf("%s security 应属于连接方式区: %+v", protocolName, security)
		}
		servername := findSchemaFieldMust(t, proto.FormSchema, "servername")
		if servername.Group != "connection" {
			t.Fatalf("%s servername 应属于连接方式区: %+v", protocolName, servername)
		}
		securityIndex := -1
		servernameIndex := -1
		for i, field := range proto.FormSchema {
			if field.Name == "security" {
				securityIndex = i
			}
			if field.Name == "servername" {
				servernameIndex = i
			}
		}
		if securityIndex < 0 || servernameIndex < 0 || securityIndex > servernameIndex {
			t.Fatalf("%s 安全选择应位于依赖参数之前: security=%d servername=%d", protocolName, securityIndex, servernameIndex)
		}
	}
	ss, _ := GetProtocol("ss")
	for _, name := range []string{"obfs-opts", "v2ray-plugin-opts", "shadow-tls-opts", "restls-opts"} {
		pluginOpts := findSchemaFieldMust(t, ss.FormSchema, name)
		if pluginOpts.Group != "connection" || !pluginOpts.ShouldReset("plugin") {
			t.Fatalf("SS %s 应跟随插件选择位于连接方式区并随插件清空: %+v", name, pluginOpts)
		}
		if pluginOpts.When == nil || len(pluginOpts.When.Plugin) != 1 {
			t.Fatalf("SS %s 应只属于单一插件: %+v", name, pluginOpts)
		}
	}
	obfsMode := findNestedFieldMust(t, findSchemaFieldMust(t, ss.FormSchema, "obfs-opts"), "mode")
	if !hasOption(obfsMode, "http") || !hasOption(obfsMode, "tls") || hasOption(obfsMode, "websocket") {
		t.Fatalf("obfs mode 候选应仅包含 http/tls: %+v", obfsMode)
	}
	v2rayMode := findNestedFieldMust(t, findSchemaFieldMust(t, ss.FormSchema, "v2ray-plugin-opts"), "mode")
	if !hasOption(v2rayMode, "websocket") || hasOption(v2rayMode, "http") || hasOption(v2rayMode, "tls") {
		t.Fatalf("v2ray-plugin mode 候选应仅包含 websocket: %+v", v2rayMode)
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

func TestProjectActiveSSOnlyProjectsActivePluginOpts(t *testing.T) {
	proto, _ := GetProtocol("ss")
	plugin := "obfs"
	state := CurrentState{Security: "none", Plugin: &plugin}
	params := map[string]any{
		"cipher": "aes-256-gcm", "password": "p", "plugin": "obfs",
		"obfs-opts":         map[string]any{"mode": "http", "host": "cdn.example.com"},
		"v2ray-plugin-opts": map[string]any{"mode": "websocket", "path": "/ws"},
		"shadow-tls-opts":   map[string]any{"password": "secret"},
		"restls-opts":       map[string]any{"path": "/restls"},
	}
	projected := ProjectActive(proto, state, params)
	if _, ok := projected["v2ray-plugin-opts"]; ok {
		t.Fatalf("不应投影非活动插件对象: %+v", projected)
	}
	if _, ok := projected["shadow-tls-opts"]; ok {
		t.Fatalf("不应投影非活动插件对象: %+v", projected)
	}
	if _, ok := projected["restls-opts"]; ok {
		t.Fatalf("不应投影非活动插件对象: %+v", projected)
	}
	obfs, ok := projected["obfs-opts"].(map[string]any)
	if !ok || obfs["mode"] != "http" || obfs["host"] != "cdn.example.com" {
		t.Fatalf("应投影当前插件对象: %+v", projected)
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

func TestValidateCurrentStateAllowsTrojanCustomTransport(t *testing.T) {
	proto, _ := GetProtocol("trojan")
	for _, network := range []string{"h2", "http", "xhttp", "custom-transport"} {
		t.Run(network, func(t *testing.T) {
			state := CurrentState{Network: network, Security: "tls"}
			params := map[string]any{"password": "p", "network": network}
			if err := ValidateCurrentState(proto, state, params); err != nil {
				t.Fatalf("Trojan %s 自定义传输不应在保存层被拒绝: %v", network, err)
			}
		})
	}
}

func TestProtocolParamsForStorageSecurityOverridesLegacyTLS(t *testing.T) {
	proto, _ := GetProtocol("vless")

	t.Run("security tls overrides stale false", func(t *testing.T) {
		stored := protocolParamsForStorage(proto, map[string]any{
			"network": "tcp", "security": "tls", "tls": false,
		})
		if stored["tls"] != true {
			t.Fatalf("security=tls 应覆盖旧 tls=false: %+v", stored)
		}
		if _, ok := stored["security"]; ok {
			t.Fatalf("表单层 security 不应落库: %+v", stored)
		}
	})

	t.Run("security none overrides stale true", func(t *testing.T) {
		stored := protocolParamsForStorage(proto, map[string]any{
			"network": "tcp", "security": "none", "tls": true,
		})
		if stored["tls"] != false {
			t.Fatalf("security=none 应覆盖旧 tls=true: %+v", stored)
		}
		if _, ok := stored["security"]; ok {
			t.Fatalf("表单层 security 不应落库: %+v", stored)
		}
	})

	t.Run("security tls without legacy tls writes true", func(t *testing.T) {
		stored := protocolParamsForStorage(proto, map[string]any{
			"network": "tcp", "security": "tls",
		})
		if stored["tls"] != true {
			t.Fatalf("security=tls 应写入 tls=true: %+v", stored)
		}
	})
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
