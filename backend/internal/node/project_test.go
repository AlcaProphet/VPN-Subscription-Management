package node

import (
	"encoding/json"
	"reflect"
	"sort"
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

func TestSSUnknownPluginSchemaUsesComplementCondition(t *testing.T) {
	ss, err := GetProtocol("ss")
	if err != nil {
		t.Fatal(err)
	}
	field := findSchemaFieldMust(t, ss.FormSchema, "plugin-opts")
	wantExcluded := []string{"", "obfs", "v2ray-plugin", "shadow-tls", "restls"}
	if field.ObjectKind != "map" || field.MapValueType != "string" || field.When == nil || !sameStringSet(field.When.PluginNot, wantExcluded) || !field.ShouldReset("plugin") {
		t.Fatalf("未知插件字符串映射 schema 异常: %+v", field)
	}
	for _, plugin := range wantExcluded {
		plugin := plugin
		if field.Matches(CurrentState{Plugin: &plugin}, "") {
			t.Errorf("排除插件 %q 不应激活未知参数字段", plugin)
		}
	}
	custom := "custom-plugin"
	if !field.Matches(CurrentState{Plugin: &custom}, "") {
		t.Fatal("未知插件应激活 plugin-opts")
	}
	raw, err := json.Marshal(field)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{`"plugin_not"`, `"map_value_type":"string"`} {
		if !strings.Contains(string(raw), fragment) {
			t.Fatalf("schema 序列化缺少 %s: %s", fragment, raw)
		}
	}
}

func TestPluginNotParticipatesInProjection(t *testing.T) {
	field := FieldSchema{
		Name: "plugin-opts", Type: "object", Label: "插件参数", ObjectKind: "map", MapValueType: "string",
		When: &ConditionRule{PluginNot: []string{"", "known"}},
	}
	proto := Protocol{Protocol: "test", FormSchema: []FieldSchema{field}}
	params := map[string]any{"plugin-opts": map[string]any{"mode": "custom"}}
	known := "known"
	if projected := ProjectActive(proto, CurrentState{Plugin: &known}, params); projected["plugin-opts"] != nil {
		t.Fatalf("已知插件不应投影补集字段: %+v", projected)
	}
	custom := "custom"
	projected := ProjectActive(proto, CurrentState{Plugin: &custom}, params)
	if got, ok := projected["plugin-opts"].(map[string]any); !ok || got["mode"] != "custom" {
		t.Fatalf("未知插件未投影字符串映射: %+v", projected)
	}
}

func TestStringMapRejectsNonStringLeaves(t *testing.T) {
	plugin := "custom"
	field := FieldSchema{
		Name: "plugin-opts", Type: "object", Label: "插件参数", ObjectKind: "map", MapValueType: "string",
		When: &ConditionRule{PluginNot: []string{"", "known"}},
	}
	proto := Protocol{Protocol: "test", FormSchema: []FieldSchema{field}}
	valid := map[string]any{"plugin": "custom", "plugin-opts": map[string]any{"mode": "", "host": "cdn.example.com"}}
	if err := ValidateCurrentState(proto, CurrentState{Plugin: &plugin}, valid); err != nil {
		t.Fatalf("字符串映射被拒绝: %v", err)
	}
	for _, value := range []any{true, float64(1), []any{"x"}, map[string]any{"nested": "x"}} {
		params := map[string]any{"plugin": "custom", "plugin-opts": map[string]any{"bad": value}}
		err := ValidateCurrentState(proto, CurrentState{Plugin: &plugin}, params)
		if err == nil || !strings.Contains(err.Error(), "plugin-opts.bad") || !strings.Contains(err.Error(), "string") {
			t.Errorf("非字符串叶子未精确拒绝: value=%#v err=%v", value, err)
		}
		if err := validateFieldType(field, params["plugin-opts"]); err == nil || !strings.Contains(err.Error(), "plugin-opts.bad") {
			t.Errorf("基础字段校验未拒绝非字符串叶子: value=%#v err=%v", value, err)
		}
	}
}

func TestFieldSchemaAllowCustomTriState(t *testing.T) {
	cases := []struct {
		name    string
		value   *bool
		present bool
		want    bool
	}{
		{name: "允许", value: boolPtr(true), present: true, want: true},
		{name: "禁止", value: boolPtr(false), present: true, want: false},
		{name: "未声明", value: nil, present: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(FieldSchema{Name: "mode", Type: "select", Label: "模式", AllowCustom: tc.value})
			if err != nil {
				t.Fatal(err)
			}
			var decoded map[string]any
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatal(err)
			}
			got, present := decoded["allow_custom"]
			if present != tc.present {
				t.Fatalf("allow_custom 字段存在性异常: %s", raw)
			}
			if present && got != tc.want {
				t.Fatalf("allow_custom 值异常: got=%v want=%v, raw=%s", got, tc.want, raw)
			}
		})
	}

	field := FieldSchema{OptionItems: []OptionItem{{Value: "known"}}}
	if err := validateOption(field, "custom", "mode"); err == nil {
		t.Fatal("未声明 allow_custom 的枚举字段不应接受清单外值")
	}
	field.AllowCustom = boolPtr(false)
	if err := validateOption(field, "custom", "mode"); err == nil {
		t.Fatal("allow_custom=false 的枚举字段不应接受清单外值")
	}
	field.AllowCustom = boolPtr(true)
	if err := validateOption(field, "custom", "mode"); err != nil {
		t.Fatalf("allow_custom=true 应接受清单外值: %v", err)
	}
}

func TestFirstBatchProtocolMetadata(t *testing.T) {
	ss, _ := GetProtocol("ss")
	vmess, _ := GetProtocol("vmess")
	trojan, _ := GetProtocol("trojan")

	cipher, _ := findSchemaField(ss.FormSchema, "cipher")
	if !allowsCustom(cipher) || len(cipher.OptionItems) < 6 || !hasOption(cipher, "2022-blake3-aes-128-gcm") {
		t.Fatalf("SS 算法目录异常: %+v", cipher)
	}
	vmessCipher, _ := findSchemaField(vmess.FormSchema, "cipher")
	if !allowsCustom(vmessCipher) || !hasOption(vmessCipher, "chacha20-poly1305") || !hasOption(vmessCipher, "zero") {
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

func TestSSPluginFieldsMatchMihomo11929Contract(t *testing.T) {
	ss, err := GetProtocol("ss")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name       string
		fields     []string
		forbidden  string
		required   []string
		defaults   map[string]any
		fieldTypes map[string]string
	}{
		{
			name: "obfs-opts", fields: []string{"host", "mode"}, required: []string{"mode"},
			defaults: map[string]any{"mode": "http"}, fieldTypes: map[string]string{"mode": "select"},
		},
		{
			name:      "v2ray-plugin-opts",
			fields:    []string{"certificate", "fingerprint", "headers", "host", "mode", "mux", "name-cert-verify", "path", "private-key", "tls", "v2ray-http-upgrade", "v2ray-http-upgrade-fast-open"},
			forbidden: "version", required: []string{"mode"}, defaults: map[string]any{"mode": "websocket"},
			fieldTypes: map[string]string{"headers": "object", "private-key": "password", "tls": "bool"},
		},
		{
			name:     "shadow-tls-opts",
			fields:   []string{"alpn", "certificate", "fingerprint", "host", "name-cert-verify", "password", "private-key", "skip-cert-verify", "version"},
			required: []string{"host"}, fieldTypes: map[string]string{"alpn": "text-list", "private-key": "password", "version": "number"},
		},
		{
			name:      "restls-opts",
			fields:    []string{"fingerprint", "host", "name-cert-verify", "password", "restls-script", "skip-cert-verify", "version-hint"},
			forbidden: "path", required: []string{"host", "password", "version-hint"},
			fieldTypes: map[string]string{"password": "password", "version-hint": "text"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			object := findSchemaFieldMust(t, ss.FormSchema, tc.name)
			gotFields := make([]string, 0, len(object.Properties))
			for _, property := range object.Properties {
				gotFields = append(gotFields, property.Name)
			}
			sort.Strings(gotFields)
			if !reflect.DeepEqual(gotFields, tc.fields) {
				t.Fatalf("%s 字段与固定合同不一致: got=%v want=%v", tc.name, gotFields, tc.fields)
			}
			if tc.forbidden != "" && hasNestedField(object, tc.forbidden) {
				t.Fatalf("%s 不应继续把 %s 声明为固定版本字段", tc.name, tc.forbidden)
			}
			for name, typ := range tc.fieldTypes {
				if field := findNestedFieldMust(t, object, name); field.Type != typ {
					t.Fatalf("%s.%s 类型异常: %+v", tc.name, name, field)
				}
			}
			for _, name := range tc.required {
				field := findNestedFieldMust(t, object, name)
				if _, hasDefault := tc.defaults[name]; hasDefault {
					if field.Default != tc.defaults[name] || field.RequiredWhen != nil {
						t.Fatalf("%s.%s 应由目标输出补默认值且不作为草稿必填: %+v", tc.name, name, field)
					}
					continue
				}
				if field.Required || field.RequiredWhen == nil || !field.RequiredWhen.Matches(CurrentState{}, "clash-yaml") || field.RequiredWhen.Matches(CurrentState{}, "generic-subs") {
					t.Fatalf("%s.%s 应仅在 Clash 目标必填: %+v", tc.name, name, field)
				}
			}
			if !hasTargetEvidence(object.TargetEvidence, "clash-yaml", "complete") {
				t.Fatalf("%s 缺少 Mihomo 1.19.29 完整证据: %+v", tc.name, object.TargetEvidence)
			}
			requiresObject := len(tc.required) > len(tc.defaults)
			plugin := object.When.Plugin[0]
			if requiresObject != (object.RequiredWhen != nil && object.RequiredWhen.Matches(CurrentState{Plugin: &plugin}, "clash-yaml")) {
				t.Fatalf("%s 对象级 Clash 必填条件异常: %+v", tc.name, object.RequiredWhen)
			}
		})
	}

	obfsMode := findNestedFieldMust(t, findSchemaFieldMust(t, ss.FormSchema, "obfs-opts"), "mode")
	if !hasOption(obfsMode, "http") || !hasOption(obfsMode, "tls") || len(obfsMode.OptionItems) != 2 {
		t.Fatalf("obfs mode 候选应严格为 http/tls: %+v", obfsMode)
	}
	v2rayMode := findNestedFieldMust(t, findSchemaFieldMust(t, ss.FormSchema, "v2ray-plugin-opts"), "mode")
	if !hasOption(v2rayMode, "websocket") || len(v2rayMode.OptionItems) != 1 {
		t.Fatalf("v2ray-plugin mode 应固定为 websocket: %+v", v2rayMode)
	}
	for _, path := range []string{"v2ray-plugin-opts.private-key", "shadow-tls-opts.private-key"} {
		if !contains(ss.SensitiveFields, path) {
			t.Fatalf("SS 缺少固定私钥敏感路径 %s: %v", path, ss.SensitiveFields)
		}
	}
	if contains(ss.SensitiveFields, "plugin-opts.private-key") {
		t.Fatal("未知插件 private-key 仍应按普通字符串参数处理")
	}
}

func hasTargetEvidence(evidence []TargetEvidence, target, status string) bool {
	for _, item := range evidence {
		if item.Target == target && item.Status == status && item.Client == "Mihomo" && item.Version == "1.19.29" {
			return true
		}
	}
	return false
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

func TestFirstBatchAllowCustomValidation(t *testing.T) {
	for _, protocolName := range []string{"vless", "vmess"} {
		t.Run(protocolName+" rejects custom security", func(t *testing.T) {
			proto, _ := GetProtocol(protocolName)
			params := map[string]any{"uuid": "u", "network": "tcp", "security": "custom-security"}
			err := ValidateCurrentState(proto, CurrentState{Network: "tcp", Security: "custom-security"}, params)
			if err == nil || !strings.Contains(err.Error(), "security") {
				t.Fatalf("自定义安全值应被拒绝并定位 security，实际 %v", err)
			}
		})

		t.Run(protocolName+" allows custom transport", func(t *testing.T) {
			proto, _ := GetProtocol(protocolName)
			params := map[string]any{"uuid": "u", "network": "custom-transport", "security": "none"}
			if err := ValidateCurrentState(proto, CurrentState{Network: "custom-transport", Security: "none"}, params); err != nil {
				t.Fatalf("明确允许的自定义传输不应被拒绝: %v", err)
			}
		})
	}

	ss, _ := GetProtocol("ss")
	plugin := "custom-plugin"
	params := map[string]any{"cipher": "custom-cipher", "password": "p", "plugin": plugin}
	if err := ValidateCurrentState(ss, CurrentState{Security: "none", Plugin: &plugin}, params); err != nil {
		t.Fatalf("SS 明确允许的自定义算法和插件不应被拒绝: %v", err)
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
