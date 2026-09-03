package node

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func smuxFixture() map[string]any {
	return map[string]any{
		"enabled": true, "max-connections": float64(7), "padding": true,
		"future":      map[string]any{"value": "old"},
		"brutal-opts": map[string]any{"enabled": true, "up": "100 Mbps", "down": "200 Mbps", "future": "old"},
	}
}

func featureParams(protocol string) map[string]any {
	if protocol == "ss" {
		return map[string]any{"cipher": "aes-128-gcm", "password": "main-secret", "smux": smuxFixture()}
	}
	return map[string]any{"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp", "smux": smuxFixture()}
}

func TestDisabledSMuxIsCleanedForStorageAndProjection(t *testing.T) {
	for _, protocol := range []string{"ss", "vless", "vmess"} {
		t.Run(protocol, func(t *testing.T) {
			proto, err := GetProtocol(protocol)
			if err != nil {
				t.Fatal(err)
			}
			params := featureParams(protocol)
			params["smux"].(map[string]any)["enabled"] = false
			want := map[string]any{"enabled": false}
			for name, result := range map[string]map[string]any{
				"storage":    protocolParamsForStorage(proto, params),
				"projection": ProjectActive(proto, DeriveCurrentState(proto, params), params),
			} {
				if !reflect.DeepEqual(result["smux"], want) {
					t.Fatalf("%s 残留关闭功能参数: %#v", name, result)
				}
			}
			if params["smux"].(map[string]any)["max-connections"] != float64(7) {
				t.Fatal("清理不能修改输入")
			}
		})
	}
}

func TestBrutalResetPreservesParentAndAcceptsNewValues(t *testing.T) {
	proto, err := GetProtocol("vless")
	if err != nil {
		t.Fatal(err)
	}
	params := featureParams("vless")
	merged := mergeProtocolJSON(params, map[string]any{"smux": map[string]any{
		"brutal-opts": map[string]any{"enabled": true, "up": "50 Mbps"},
	}}, proto, []string{"feature.smux.brutal"})
	smux := merged["smux"].(map[string]any)
	if smux["max-connections"] != float64(7) || smux["future"] == nil {
		t.Fatalf("子功能重置影响父配置: %#v", smux)
	}
	if !reflect.DeepEqual(smux["brutal-opts"], map[string]any{"enabled": true, "up": "50 Mbps"}) {
		t.Fatalf("旧 Brutal 数据复活: %#v", smux)
	}
}

func TestSMuxCloseCheckAndSaveDoNotRestoreOldData(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	created, err := svc.CreateManual(ctx, CreateManualInput{
		Name: "smux-lifecycle", Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: featureParams("vless"),
	})
	if err != nil {
		t.Fatal(err)
	}
	var checked map[string]any
	svc.SetCheckRenderer(func(_ context.Context, _, _, _, _ string, _ int, params map[string]any) (CheckRenderResult, error) {
		checked = params
		return CheckRenderResult{Preview: "preview"}, nil
	})
	incoming := map[string]any{"smux": map[string]any{"enabled": false}}
	_, err = svc.Check(ctx, CheckRequest{NodeID: created.ID, BaseRevision: created.EditRevision,
		Protocol: "vless", Host: "example.com", Port: 443, ProtocolJSON: incoming})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(checked["smux"], incoming["smux"]) {
		t.Fatalf("检查残留旧数据: %#v", checked)
	}
	before, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before.ProtocolJSON["smux"].(map[string]any)["max-connections"] != float64(7) {
		t.Fatal("检查不应落库")
	}
	updated, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: incoming, BaseRevision: created.EditRevision})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.UpdateManual(ctx, created.ID, UpdateManualInput{Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"smux": map[string]any{"enabled": true}}, BaseRevision: updated.EditRevision})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(raw.ProtocolJSON["smux"], map[string]any{"enabled": true}) {
		t.Fatalf("重开恢复旧参数: %#v", raw.ProtocolJSON)
	}
}

func TestFeatureMetadataSurvivesProtocolSerialization(t *testing.T) {
	for _, protocol := range []string{"ss", "vless", "vmess"} {
		proto, err := GetProtocol(protocol)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(proto)
		if err != nil {
			t.Fatal(err)
		}
		var decoded Protocol
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatal(err)
		}
		smux := findSchemaFieldMust(t, decoded.FormSchema, "smux")
		brutal := findNestedFieldMust(t, smux, "brutal-opts")
		if smux.Feature == nil || smux.Feature.Toggle != "enabled" || !smux.ShouldReset("feature.smux") {
			t.Fatalf("%s 缺少 SMux 功能声明", protocol)
		}
		if brutal.Feature == nil || brutal.Feature.Name != "smux.brutal" || !brutal.ShouldReset("feature.smux.brutal") {
			t.Fatalf("%s 缺少 Brutal 声明", protocol)
		}
		if !findNestedFieldMust(t, smux, "enabled").Matches(CurrentState{}, "") || findNestedFieldMust(t, smux, "max-connections").Matches(CurrentState{}, "") {
			t.Fatal("关闭时只应显示启用入口")
		}
		state := CurrentState{Features: []string{"smux"}}
		if !brutal.Matches(state, "") || findNestedFieldMust(t, brutal, "up").Matches(state, "") {
			t.Fatal("Brutal 子参数需单独启用")
		}
	}
}

func TestFeatureResetClearsExtensionsWithoutRestoringOnReenable(t *testing.T) {
	for _, scope := range []string{"feature.smux", "feature.smux.brutal"} {
		t.Run(scope, func(t *testing.T) {
			svc, _, _ := newTestService(t)
			ctx := context.Background()
			created, err := svc.CreateManual(ctx, CreateManualInput{Name: "feature-extensions", Protocol: "vless", Host: "example.com", Port: 443,
				ProtocolJSON: featureParams("vless"), Extensions: []ExtensionInput{
					{ID: "common", Scope: "node", Payload: "common"},
					{ID: "parent", Scope: "feature.smux", Payload: "parent"},
					{ID: "child", Scope: "feature.smux.brutal", Payload: "child"},
				}})
			if err != nil {
				t.Fatal(err)
			}
			before, err := svc.getRaw(ctx, created.ID)
			if err != nil {
				t.Fatal(err)
			}
			// 同次编辑关闭再开启，当前功能相同也不能保留重置范围的旧扩展。
			_, err = svc.UpdateManual(ctx, created.ID, UpdateManualInput{Protocol: "vless", Host: "example.com", Port: 443,
				BaseRevision: created.EditRevision, ResetScopes: []string{scope}, ProtocolJSON: map[string]any{
					"smux": map[string]any{"enabled": true, "brutal-opts": map[string]any{"enabled": true, "up": "new"}},
				}, ExtensionOps: []ExtensionOp{{ID: "parent", Op: "keep"}, {ID: "child", Op: "keep"},
					{Op: "add", ID: "new-child", Scope: "feature.smux.brutal", Payload: "new-child"}}})
			if err != nil {
				t.Fatal(err)
			}
			after, err := svc.getRaw(ctx, created.ID)
			if err != nil {
				t.Fatal(err)
			}
			if after.ProtocolJSON["uuid"] != before.ProtocolJSON["uuid"] {
				t.Fatal("功能重置不应改动主凭据")
			}
			ids := []string{}
			for _, record := range after.extensionRecords {
				ids = append(ids, record.ID)
			}
			want := []string{"common", "new-child"}
			if scope == "feature.smux.brutal" {
				want = []string{"common", "parent", "new-child"}
			}
			if !reflect.DeepEqual(ids, want) {
				t.Fatalf("范围 %s 扩展结果错误: %v", scope, ids)
			}
		})
	}
}

func TestFeatureCredentialResetUsesNestedSchema(t *testing.T) {
	// 使用合成的嵌套凭据证明通用机制，真实 SMux schema 不添加客户端不存在的凭据。
	inner := featureObject("smux.brutal", obj("brutal-opts", "Brutal", "fields", f("enabled", "bool", "启用"), f("password", "password", "凭据")))
	proto := Protocol{FormSchema: []FieldSchema{featureObject("smux", obj("smux", "SMux", "fields", f("enabled", "bool", "启用"), inner))}, SensitiveFields: []string{"smux.brutal-opts.password"}}
	path := proto.SensitiveFields[0]
	existing := Node{ProtocolJSON: map[string]any{"smux": map[string]any{"enabled": true, "brutal-opts": map[string]any{"enabled": true, "password": "old-ciphertext"}}}}
	svc := &Service{}
	for _, scope := range []string{"feature.smux", "feature.smux.brutal"} {
		base := mergeProtocolJSON(existing.ProtocolJSON, map[string]any{"smux": map[string]any{"enabled": true, "brutal-opts": map[string]any{"enabled": true}}}, proto, []string{scope})
		out, err := svc.mergeSensitiveWithOps(context.Background(), existing, proto, base, []string{scope}, []CredentialOp{{Path: path, Op: "keep"}})
		if err != nil {
			t.Fatal(err)
		}
		if _, exists := GetPath(out, path); exists {
			t.Fatal("重置后 keep 不能复活旧凭据")
		}
		SetPath(base, path, "new-secret")
		out, err = svc.mergeSensitiveWithOps(context.Background(), existing, proto, base, []string{scope}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := GetPath(out, path); got != "new-secret" {
			t.Fatal("重置后需允许新凭据")
		}
	}
}

func TestFeatureLegacyStateAndDefaults(t *testing.T) {
	proto, err := GetProtocol("vless")
	if err != nil {
		t.Fatal(err)
	}
	params := featureParams("vless")
	// 老记录 features 只有 smux，不能因此丢失实际启用的 Brutal。
	projected := ProjectActive(proto, CurrentState{Network: "tcp", Security: "none", Features: []string{"smux"}}, params)
	if !reflect.DeepEqual(projected["smux"], params["smux"]) {
		t.Fatal("旧状态应保留启用功能的现有配置")
	}
	params["smux"].(map[string]any)["enabled"] = false
	merged := mergeProtocolJSON(params, map[string]any{"smux": map[string]any{"enabled": true}}, proto, nil)
	if !reflect.DeepEqual(merged["smux"], map[string]any{"enabled": true}) {
		t.Fatal("历史关闭记录的残留值不应在重开时恢复")
	}
	delete(params, "smux")
	if _, exists := protocolParamsForStorage(proto, params)["smux"]; exists {
		t.Fatal("不应把显示默认值批量写入存储")
	}
}

func TestFeatureCleanupValidationAndFailedSave(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	params := featureParams("vless")
	params["smux"] = map[string]any{"enabled": false, "max-connections": "stale-invalid"}
	created, err := svc.CreateManual(ctx, CreateManualInput{Name: "disabled-feature", Protocol: "vless", Host: "example.com", Port: 443, ProtocolJSON: params})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(created.ProtocolJSON["smux"], map[string]any{"enabled": false}) {
		t.Fatal("新建不应保存关闭参数")
	}
	before, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.UpdateManual(ctx, created.ID, UpdateManualInput{Protocol: "vless", Host: "example.com", Port: 443, BaseRevision: created.EditRevision,
		ProtocolJSON: map[string]any{"smux": map[string]any{"enabled": "false"}}})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("控制值类型错误应拒绝: %v", err)
	}
	after, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.EditRevision != before.EditRevision || !reflect.DeepEqual(after.ProtocolJSON, before.ProtocolJSON) {
		t.Fatal("失败保存不应部分更新")
	}
}

func TestScalarFeatureCleanupKeepsExistingContracts(t *testing.T) {
	for _, tc := range []struct{ protocol, toggle, child string }{
		{"ss", "udp-over-tcp", "udp-over-tcp-version"},
		{"tuic", "udp-over-stream", "udp-over-stream-version"},
	} {
		proto, err := GetProtocol(tc.protocol)
		if err != nil {
			t.Fatal(err)
		}
		params := map[string]any{tc.toggle: false, tc.child: 2, "udp": false}
		out := cleanDisabledFeatures(proto.FormSchema, params)
		if _, exists := out[tc.child]; exists || out[tc.toggle] != false || out["udp"] != false {
			t.Fatalf("顶层功能关闭行为异常: %#v", out)
		}
		params[tc.toggle] = true
		if !reflect.DeepEqual(cleanDisabledFeatures(proto.FormSchema, params), params) {
			t.Fatal("启用功能不应丢失有效参数")
		}
	}
	proto, err := GetProtocol("mieru")
	if err != nil {
		t.Fatal(err)
	}
	if len(DeriveCurrentState(proto, map[string]any{"multiplexing": "MULTIPLEXING_OFF"}).Features) != 0 {
		t.Fatal("OFF 不应判为启用")
	}
	if !reflect.DeepEqual(DeriveCurrentState(proto, map[string]any{"multiplexing": "HIGH"}).Features, []string{"multiplexing"}) {
		t.Fatal("既有选择功能应正常派生")
	}
}
