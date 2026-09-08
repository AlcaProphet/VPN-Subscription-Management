package ssplugin

import (
	"reflect"
	"testing"
)

func TestKnownPluginContracts(t *testing.T) {
	wantStorage := map[string]string{
		"obfs":         "obfs-opts",
		"v2ray-plugin": "v2ray-plugin-opts",
		"shadow-tls":   "shadow-tls-opts",
		"restls":       "restls-opts",
	}
	if got := KnownNames(); !reflect.DeepEqual(got, []string{"obfs", "v2ray-plugin", "shadow-tls", "restls"}) {
		t.Fatalf("已知插件顺序异常: %v", got)
	}
	for name, storageKey := range wantStorage {
		definition, ok := Lookup(name)
		if !ok || definition.StorageKey != storageKey {
			t.Fatalf("插件 %s 合同异常: %+v", name, definition)
		}
		for _, target := range []string{TargetClash, TargetShadowrocket, TargetGeneric} {
			if _, ok := definition.Target(target); !ok {
				t.Errorf("插件 %s 缺少目标 %s 合同", name, target)
			}
		}
	}
	if _, ok := Lookup("custom-plugin"); ok {
		t.Fatal("未知插件不应被识别为固定合同")
	}
}

func TestFixedTargetSupportAndRequirements(t *testing.T) {
	cases := []struct {
		plugin         string
		clashRequired  []string
		clashDefaults  map[string]string
		shadowSupport  SupportLevel
		genericSupport SupportLevel
	}{
		{plugin: "obfs", clashRequired: []string{"mode"}, clashDefaults: map[string]string{"mode": "http"}, shadowSupport: SupportPartial, genericSupport: SupportPartial},
		{plugin: "v2ray-plugin", clashRequired: []string{"mode"}, clashDefaults: map[string]string{"mode": "websocket"}, shadowSupport: SupportPartial, genericSupport: SupportPartial},
		{plugin: "shadow-tls", clashRequired: []string{"host"}, shadowSupport: SupportUnverified, genericSupport: SupportUnsupported},
		{plugin: "restls", clashRequired: []string{"password", "host", "version-hint"}, shadowSupport: SupportUnverified, genericSupport: SupportUnsupported},
	}
	for _, tc := range cases {
		t.Run(tc.plugin, func(t *testing.T) {
			definition, _ := Lookup(tc.plugin)
			clash, _ := definition.Target(TargetClash)
			if clash.Support != SupportComplete || !reflect.DeepEqual(clash.RequiredFields, tc.clashRequired) || !reflect.DeepEqual(clash.Defaults, tc.clashDefaults) {
				t.Fatalf("Clash 合同异常: %+v", clash)
			}
			shadowrocket, _ := definition.Target(TargetShadowrocket)
			generic, _ := definition.Target(TargetGeneric)
			if shadowrocket.Support != tc.shadowSupport || generic.Support != tc.genericSupport {
				t.Fatalf("目标支持等级异常: sr=%s generic=%s", shadowrocket.Support, generic.Support)
			}
			if len(clash.ExpressibleFields) == 0 || len(shadowrocket.ExpressibleFields) == 0 && tc.shadowSupport != SupportUnsupported {
				t.Fatal("受支持目标必须声明可表达字段")
			}
		})
	}
}

func TestClashExpressibleFieldsMatchMihomo11929(t *testing.T) {
	want := map[string][]string{
		"obfs":         {"mode", "host"},
		"v2ray-plugin": {"mode", "host", "path", "headers", "tls", "ech-opts", "mux", "v2ray-http-upgrade", "v2ray-http-upgrade-fast-open", "fingerprint", "certificate", "private-key", "skip-cert-verify", "name-cert-verify"},
		"shadow-tls":   {"host", "password", "version", "alpn", "fingerprint", "certificate", "private-key", "skip-cert-verify", "name-cert-verify"},
		"restls":       {"password", "host", "version-hint", "restls-script", "fingerprint", "skip-cert-verify", "name-cert-verify"},
	}
	for plugin, fields := range want {
		definition, _ := Lookup(plugin)
		clash, _ := definition.Target(TargetClash)
		if !reflect.DeepEqual(clash.ExpressibleFields, fields) {
			t.Fatalf("%s Clash 字段合同异常: got=%v want=%v", plugin, clash.ExpressibleFields, fields)
		}
	}
}

func TestClashPluginModeEnumsMatchMihomo11929(t *testing.T) {
	want := map[string][]string{
		"obfs":         {"http", "tls"},
		"v2ray-plugin": {"websocket"},
	}
	for plugin, values := range want {
		definition, _ := Lookup(plugin)
		clash, _ := definition.Target(TargetClash)
		if !reflect.DeepEqual(clash.AllowedValues["mode"], values) {
			t.Fatalf("%s Clash mode 枚举异常: got=%v want=%v", plugin, clash.AllowedValues["mode"], values)
		}
	}
}

func TestContractResultsAreDefensiveCopies(t *testing.T) {
	definition, _ := Lookup("obfs")
	definition.StorageKey = "changed"
	definition.Targets[TargetClash] = TargetContract{Support: SupportUnsupported}
	definition, _ = Lookup("v2ray-plugin")
	definition.Targets[TargetClash].AllowedValues["mode"][0] = "changed"
	definition, _ = Lookup("obfs")
	clash, _ := definition.Target(TargetClash)
	if definition.StorageKey != "obfs-opts" || clash.Support != SupportComplete {
		t.Fatalf("调用方修改污染了固定合同: %+v", definition)
	}
	v2ray, _ := Lookup("v2ray-plugin")
	v2rayClash, _ := v2ray.Target(TargetClash)
	if !reflect.DeepEqual(v2rayClash.AllowedValues["mode"], []string{"websocket"}) {
		t.Fatalf("调用方修改污染了固定枚举: %+v", v2rayClash.AllowedValues)
	}
}

func TestAssessTargetUsesActivePluginContract(t *testing.T) {
	params := map[string]any{
		"plugin":            "v2ray-plugin",
		"v2ray-plugin-opts": map[string]any{"mode": "websocket", "host": "cdn.example.com", "tls": true},
		"restls-opts":       "inactive-invalid-shape",
	}
	clash := AssessTarget("v2ray-plugin", params, TargetClash)
	if len(clash) != 0 {
		t.Fatalf("Clash 完整合同不应因非活动对象降级: %+v", clash)
	}
	for _, target := range []string{TargetShadowrocket, TargetGeneric} {
		issues := AssessTarget("v2ray-plugin", params, target)
		if !hasTargetIssue(issues, "warn", "plugin_partial_mapping", "plugin") || hasErrorIssue(issues) {
			t.Fatalf("%s 应仅报告部分映射 warning: %+v", target, issues)
		}
	}
}

func TestAssessTargetReportsPreciseBlockingIssues(t *testing.T) {
	tests := []struct {
		name     string
		plugin   string
		target   string
		params   map[string]any
		wantCode string
		wantPath string
	}{
		{name: "shape", plugin: "v2ray-plugin", target: TargetClash, params: map[string]any{"v2ray-plugin-opts": "bad"}, wantCode: "ss_plugin_shape_invalid", wantPath: "v2ray-plugin-opts"},
		{name: "required", plugin: "restls", target: TargetClash, params: map[string]any{"restls-opts": map[string]any{"password": "secret", "version-hint": "tls13"}}, wantCode: "ss_plugin_required_field_missing", wantPath: "restls-opts.host"},
		{name: "enum", plugin: "obfs", target: TargetClash, params: map[string]any{"obfs-opts": map[string]any{"mode": "quic"}}, wantCode: "plugin_option_unexpressible", wantPath: "obfs-opts.mode"},
		{name: "uri-field", plugin: "v2ray-plugin", target: TargetShadowrocket, params: map[string]any{"v2ray-plugin-opts": map[string]any{"headers": map[string]any{"X-Test": "value"}}}, wantCode: "plugin_option_unexpressible", wantPath: "v2ray-plugin-opts.headers"},
		{name: "generic-unsupported", plugin: "shadow-tls", target: TargetGeneric, params: map[string]any{"shadow-tls-opts": map[string]any{"host": "cdn.example.com"}}, wantCode: "core_semantic_unexpressible", wantPath: "plugin"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			issues := AssessTarget(tc.plugin, tc.params, tc.target)
			if !hasTargetIssue(issues, "error", tc.wantCode, tc.wantPath) {
				t.Fatalf("缺少精确目标错误 %s/%s: %+v", tc.wantCode, tc.wantPath, issues)
			}
		})
	}
}

func hasTargetIssue(issues []TargetIssue, severity, code, path string) bool {
	for _, issue := range issues {
		if issue.Severity == severity && issue.Code == code && issue.FieldPath == path {
			return true
		}
	}
	return false
}

func hasErrorIssue(issues []TargetIssue) bool {
	for _, issue := range issues {
		if issue.Severity == "error" {
			return true
		}
	}
	return false
}
