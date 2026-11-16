package node

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

// 数字清空必须经过真实检查、保存和 SQLite 回读；省略仍保留，显式零仍是数值。
func TestOptionalNumberUnsetRoundTrip(t *testing.T) {
	for _, old := range []float64{0, 1420} {
		for _, mode := range []string{"omit", "clear", "zero", "replace"} {
			t.Run(mode+map[float64]string{0: "-old-zero", 1420: "-old-positive"}[old], func(t *testing.T) {
				svc, st, _ := newTestService(t)
				ctx := context.Background()
				params := wireGuardPeersParams()
				params["mtu"] = old
				n, err := createWireGuard(t, svc, "scalar-roundtrip", params, &CurrentState{Selectors: map[string]string{"peer_mode": "peers"}})
				if err != nil {
					t.Fatal(err)
				}
				read := func() (string, map[string]any, int64) {
					t.Helper()
					var raw string
					var revision int64
					if err := st.DB().QueryRowContext(ctx, `SELECT protocol_json, edit_revision FROM nodes WHERE id=?`, n.ID).Scan(&raw, &revision); err != nil {
						t.Fatal(err)
					}
					var decoded map[string]any
					if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
						t.Fatal(err)
					}
					return raw, decoded, revision
				}
				beforeJSON, before, revision := read()
				incoming := map[string]any{}
				want := old
				switch mode {
				case "clear":
					incoming["mtu"] = nil
				case "zero":
					incoming["mtu"] = float64(0)
					want = 0
				case "replace":
					incoming["mtu"] = float64(1280)
					want = 1280
				}
				var checked map[string]any
				svc.SetCheckRenderer(func(_ context.Context, _, _, _, _ string, _ int, params map[string]any) (CheckRenderResult, error) {
					checked = params
					return CheckRenderResult{Preview: "safe-preview"}, nil
				})
				if _, err := svc.Check(ctx, CheckRequest{NodeID: n.ID, BaseRevision: revision, Protocol: n.Protocol, CurrentState: &n.CurrentState, ProtocolJSON: incoming, Targets: []string{"clash-yaml"}}); err != nil {
					t.Fatal(err)
				}
				if checked == nil {
					t.Fatal("检查没有到达目标适配器")
				}
				if raw, _, rev := read(); raw != beforeJSON || rev != revision {
					t.Fatal("检查不得写入节点")
				}
				updated, err := svc.UpdateManual(ctx, n.ID, UpdateManualInput{Protocol: n.Protocol, CurrentState: &n.CurrentState, BaseRevision: revision, ProtocolJSON: incoming})
				if err != nil {
					t.Fatal(err)
				}
				_, stored, nextRevision := read()
				if nextRevision != revision+1 {
					t.Fatal("成功保存必须仅递增一次修订号")
				}
				if stored["private-key"] != before["private-key"] || !reflect.DeepEqual(stored["peers"], before["peers"]) {
					t.Fatal("普通数字编辑不得改变主凭据或 Peer")
				}
				for name, values := range map[string]map[string]any{"check": checked, "stored": stored, "detail": updated.ProtocolJSON} {
					value, exists := values["mtu"]
					if mode == "clear" {
						if exists {
							t.Errorf("%s 清空后 MTU 必须缺失，实际 %#v", name, value)
						}
					} else if !exists || value != want {
						t.Errorf("%s MTU=%v，期望 %v", name, value, want)
					}
				}
				if mode == "clear" {
					// 清空后再次省略和重读不得恢复旧值。
					if _, err := svc.UpdateManual(ctx, n.ID, UpdateManualInput{Protocol: n.Protocol, CurrentState: &n.CurrentState, BaseRevision: nextRevision}); err != nil {
						t.Fatal(err)
					}
					_, reread, _ := read()
					if _, exists := reread["mtu"]; exists {
						t.Fatal("后续局部更新恢复了已清空数字")
					}
				}
			})
		}
	}
}

func TestNestedNumberUnsetKeepsSiblingAndCredential(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	n, err := svc.CreateManual(ctx, CreateManualInput{Name: "nested-unset", Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"uuid": "synthetic-only", "network": "tcp", "smux": map[string]any{"enabled": true, "max-connections": 7, "min-streams": 3}}})
	if err != nil {
		t.Fatal(err)
	}
	before, err := svc.getRaw(ctx, n.ID)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := svc.UpdateManual(ctx, n.ID, UpdateManualInput{Protocol: n.Protocol, Host: n.Host, Port: n.Port, BaseRevision: n.EditRevision,
		ProtocolJSON: map[string]any{"smux": map[string]any{"min-streams": nil}}})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := svc.getRaw(ctx, n.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, params := range []map[string]any{updated.ProtocolJSON, raw.ProtocolJSON} {
		object := params["smux"].(map[string]any)
		if _, exists := object["min-streams"]; exists {
			t.Error("嵌套清空必须删除数字键")
		}
		if object["enabled"] != true || object["max-connections"] != float64(7) {
			t.Error("嵌套清空误改邻接参数")
		}
	}
	if raw.ProtocolJSON["uuid"] != before.ProtocolJSON["uuid"] {
		t.Fatal("嵌套清空误改凭据")
	}
}

func TestTriStateBoolUnsetRoundTrip(t *testing.T) {
	for _, old := range []bool{false, true} {
		t.Run(map[bool]string{false: "false", true: "true"}[old], func(t *testing.T) {
			svc, _, _ := newTestService(t)
			ctx := context.Background()
			n, err := createTailscale(t, svc, "bool-unset", map[string]any{"hostname": "synthetic", "accept-routes": old})
			if err != nil {
				t.Fatal(err)
			}
			kept, err := svc.UpdateManual(ctx, n.ID, UpdateManualInput{Protocol: n.Protocol, BaseRevision: n.EditRevision})
			if err != nil {
				t.Fatal(err)
			}
			if kept.ProtocolJSON["accept-routes"] != old {
				t.Fatal("省略不得改变显式布尔值")
			}
			cleared, err := svc.UpdateManual(ctx, n.ID, UpdateManualInput{Protocol: n.Protocol, BaseRevision: kept.EditRevision, ProtocolJSON: map[string]any{"accept-routes": nil}})
			if err != nil {
				t.Fatal(err)
			}
			raw, err := svc.getRaw(ctx, n.ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, exists := raw.ProtocolJSON["accept-routes"]; exists {
				t.Fatal("三态 bool 清空后存储键必须缺失")
			}
			if _, exists := cleared.ProtocolJSON["accept-routes"]; exists {
				t.Fatal("三态 bool 清空后回显键必须缺失")
			}
			zero, err := svc.UpdateManual(ctx, n.ID, UpdateManualInput{Protocol: n.Protocol, BaseRevision: cleared.EditRevision, ProtocolJSON: map[string]any{"accept-routes": false}})
			if err != nil {
				t.Fatal(err)
			}
			if zero.ProtocolJSON["accept-routes"] != false {
				t.Fatal("显式 false 不得被删除")
			}
		})
	}
}

func TestUnsetStorageCreateAndSchemaBoundary(t *testing.T) {
	svc, _, _ := newTestService(t)
	params := wireGuardSingleParams()
	params["mtu"] = nil
	n, err := createWireGuard(t, svc, "unset-create", params, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := n.ProtocolJSON["mtu"]; exists {
		t.Error("新建未设置数字不应保存 null")
	}
	proto := Protocol{FormSchema: []FieldSchema{
		{Name: "number", Type: "number"}, {Name: "flag", Type: "bool"}, {Name: "default-flag", Type: "bool", Default: false},
		{Name: "secret", Type: "number"}, {Name: "map", Type: "object", ObjectKind: "map", AllowUnknown: true},
		{Name: "items", Type: "object", ObjectKind: "list", Properties: []FieldSchema{{Name: "number", Type: "number"}}},
	}, SensitiveFields: []string{"secret"}}
	input := map[string]any{"number": nil, "flag": nil, "default-flag": nil, "secret": nil, "map": map[string]any{"null": nil}, "items": []any{map[string]any{"number": nil, "unknown": nil}}, "unknown": nil}
	before := cloneJSONMap(input)
	out := protocolParamsForStorage(proto, input)
	for _, key := range []string{"number", "flag"} {
		if _, exists := out[key]; exists {
			t.Errorf("%s 未规范化", key)
		}
	}
	for _, key := range []string{"default-flag", "secret", "unknown"} {
		if _, exists := out[key]; !exists {
			t.Errorf("越界清理 %s", key)
		}
	}
	if _, exists := out["map"].(map[string]any)["null"]; !exists {
		t.Fatal("开放 Map 的 null 不得删除")
	}
	item := out["items"].([]any)[0].(map[string]any)
	if _, exists := item["number"]; exists {
		t.Error("对象列表内数字 null 未规范化")
	}
	if _, exists := item["unknown"]; !exists {
		t.Error("未知键不得静默删除")
	}
	if !reflect.DeepEqual(input, before) {
		t.Fatal("存储规范化修改了调用方对象")
	}
}

func TestUnsetValidationAndRevisionAreAtomic(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	n, err := createWireGuard(t, svc, "unset-atomic", wireGuardSingleParams(), nil)
	if err != nil {
		t.Fatal(err)
	}
	before, err := svc.getRaw(ctx, n.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"public-key", "private-key"} {
		_, err := svc.UpdateManual(ctx, n.ID, UpdateManualInput{Protocol: n.Protocol, Host: "must-not-write.example", Port: n.Port, BaseRevision: n.EditRevision, ProtocolJSON: map[string]any{field: nil}})
		if !errors.Is(err, ErrBadRequest) {
			t.Fatalf("必填字段 %s 清空必须拒绝: %v", field, err)
		}
	}
	_, err = svc.UpdateManual(ctx, n.ID, UpdateManualInput{Protocol: n.Protocol, Host: n.Host, Port: n.Port, BaseRevision: n.EditRevision - 1, ProtocolJSON: map[string]any{"mtu": nil}})
	if !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("旧修订必须拒绝: %v", err)
	}
	after, err := svc.getRaw(ctx, n.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("失败更新不得部分写入")
	}
}
