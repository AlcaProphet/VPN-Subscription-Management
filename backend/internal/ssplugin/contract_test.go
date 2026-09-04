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

func TestContractResultsAreDefensiveCopies(t *testing.T) {
	definition, _ := Lookup("obfs")
	definition.StorageKey = "changed"
	definition.Targets[TargetClash] = TargetContract{Support: SupportUnsupported}
	definition, _ = Lookup("obfs")
	clash, _ := definition.Target(TargetClash)
	if definition.StorageKey != "obfs-opts" || clash.Support != SupportComplete {
		t.Fatalf("调用方修改污染了固定合同: %+v", definition)
	}
}
