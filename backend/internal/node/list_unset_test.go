package node

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

// 显式空列表必须经过检查、保存和真实数据库回读；省略继续沿用旧值。
func TestOptionalTextListUnsetRoundTrip(t *testing.T) {
	hostKey := sshTestAuthorizedKey(t)
	otherHostKey := sshTestAuthorizedKey(t)
	fields := []struct {
		protocol, key string
		old, next     []string
	}{
		{"wireguard", "allowed-ips", []string{"10.0.0.0/24"}, []string{"10.0.1.0/24"}},
		{"ssh", "host-key", []string{hostKey}, []string{otherHostKey}},
		{"ssh", "host-key-algorithms", []string{"ssh-ed25519"}, []string{"rsa-sha2-256"}},
		{"shadowquic", "quic-versions", []string{"v1"}, []string{"v2"}},
		{"openvpn", "data-ciphers", []string{"AES-256-GCM"}, []string{"CHACHA20-POLY1305"}},
	}
	for _, field := range fields {
		for _, mode := range []string{"omit", "empty-any", "empty-string-list", "blank-text", "blank-items", "replace", "invalid"} {
			t.Run(field.protocol+"/"+field.key+"/"+mode, func(t *testing.T) {
				svc, st, _ := newTestService(t)
				ctx := context.Background()
				var params map[string]any
				switch field.protocol {
				case "wireguard":
					params = wireGuardSingleParams()
					params["workers"], params["reserved"] = 2, []any{1, 2, 3}
				case "ssh":
					params = map[string]any{"username": "qa", "password": "synthetic-only"}
				case "shadowquic":
					params = shadowquicBaseParams()
				case "openvpn":
					params = openvpnBaseParams()
				}
				params[field.key] = field.old
				n, err := svc.CreateManual(ctx, CreateManualInput{Name: "list-roundtrip", Protocol: field.protocol, Host: "example.test", Port: 443, ProtocolJSON: params})
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
				switch mode {
				case "empty-any":
					incoming[field.key] = []any{}
				case "empty-string-list":
					incoming[field.key] = []string{}
				case "blank-text":
					incoming[field.key] = " \n\t"
				case "blank-items":
					incoming[field.key] = []any{" ", "\t"}
				case "replace":
					incoming[field.key] = []any{" " + field.next[0] + " ", field.next[0]}
				case "invalid":
					incoming[field.key] = []any{7}
				}
				inputSnapshot := cloneJSONMap(incoming)
				var checked map[string]any
				svc.SetCheckRenderer(func(_ context.Context, _, _, _, _ string, _ int, active map[string]any) (CheckRenderResult, error) {
					checked = cloneJSONMap(active)
					return CheckRenderResult{Preview: "safe-preview"}, nil
				})
				response, err := svc.Check(ctx, CheckRequest{NodeID: n.ID, BaseRevision: revision, Protocol: n.Protocol, Host: n.Host, Port: n.Port, CurrentState: &n.CurrentState, ProtocolJSON: incoming, Targets: []string{"clash-yaml"}})
				if err != nil {
					t.Fatal(err)
				}
				if raw, _, rev := read(); raw != beforeJSON || rev != revision {
					t.Fatal("检查修改了节点内容或修订")
				}
				updated, saveErr := svc.UpdateManual(ctx, n.ID, UpdateManualInput{Protocol: n.Protocol, Host: n.Host, Port: n.Port, CurrentState: &n.CurrentState, BaseRevision: revision, ProtocolJSON: incoming})
				if !reflect.DeepEqual(incoming, inputSnapshot) {
					t.Fatal("检查或保存修改了调用方输入")
				}
				result := response.Targets["clash-yaml"]
				if mode == "invalid" {
					if !errors.Is(saveErr, ErrBadRequest) || result.Status != "error" || result.Preview != nil || checked != nil {
						t.Fatalf("非法列表必须在渲染和写入前拒绝: save=%v status=%s", saveErr, result.Status)
					}
					if raw, _, rev := read(); raw != beforeJSON || rev != revision {
						t.Fatal("失败保存发生了部分写入")
					}
					return
				}
				if saveErr != nil || result.Status != "ok" || checked == nil {
					t.Fatalf("合法列表编辑失败: save=%v status=%s", saveErr, result.Status)
				}
				_, stored, nextRevision := read()
				if nextRevision != revision+1 {
					t.Fatal("成功保存必须只递增一次修订")
				}
				clear := mode != "omit" && mode != "replace"
				for label, values := range map[string]map[string]any{"check": checked, "storage": stored, "detail": updated.ProtocolJSON} {
					value, exists := values[field.key]
					if clear {
						if exists {
							t.Errorf("%s 清空后仍含 %s", label, field.key)
						}
					} else {
						want := field.old
						if mode == "replace" {
							want = field.next
						}
						got, ok := stringListItems(value)
						if !ok || !reflect.DeepEqual(got, want) {
							t.Errorf("%s 列表省略或替换结果不符", label)
						}
					}
				}
				delete(before, field.key)
				delete(stored, field.key)
				if !reflect.DeepEqual(before, stored) {
					t.Fatal("列表编辑改变了邻接参数或已保存凭据")
				}
				if clear {
					if _, err := svc.UpdateManual(ctx, n.ID, UpdateManualInput{Protocol: n.Protocol, Host: n.Host, Port: n.Port, CurrentState: &n.CurrentState, BaseRevision: nextRevision}); err != nil {
						t.Fatal(err)
					}
					_, reread, _ := read()
					if _, exists := reread[field.key]; exists {
						t.Fatal("后续省略更新恢复了已清空列表")
					}
				}
			})
		}
	}
}

// 条件必填 DNS 和 Peer 列表清空必须拒绝，不能从旧节点回填并绕过校验。
func TestRequiredTextListUnsetIsAtomic(t *testing.T) {
	for _, kind := range []string{"wireguard-dns", "openvpn-dns", "peer"} {
		t.Run(kind, func(t *testing.T) {
			svc, _, _ := newTestService(t)
			ctx := context.Background()
			protocol, params := "wireguard", wireGuardSingleParams()
			if kind == "openvpn-dns" {
				protocol, params = "openvpn", openvpnBaseParams()
			}
			if kind == "peer" {
				params = wireGuardPeersParams()
				params["peers"].([]any)[0].(map[string]any)["pre-shared-key"] = wgPSK
			} else {
				params["remote-dns-resolve"], params["dns"] = true, []any{"1.1.1.1"}
			}
			n, err := svc.CreateManual(ctx, CreateManualInput{Name: "required-list", Protocol: protocol, Host: "example.test", Port: 443, ProtocolJSON: params})
			if err != nil {
				t.Fatal(err)
			}
			before, err := svc.getRaw(ctx, n.ID)
			if err != nil {
				t.Fatal(err)
			}
			incoming, path := map[string]any{"remote-dns-resolve": true, "dns": []any{}}, "dns"
			if kind == "peer" {
				incoming, path = cloneJSONMap(n.ProtocolJSON), "peers[0].allowed-ips"
				incoming["peers"].([]any)[0].(map[string]any)["allowed-ips"] = []any{}
			}
			rendered := false
			svc.SetCheckRenderer(func(_ context.Context, _, _, _, _ string, _ int, _ map[string]any) (CheckRenderResult, error) {
				rendered = true
				return CheckRenderResult{Preview: "must-not-render"}, nil
			})
			response, err := svc.Check(ctx, CheckRequest{NodeID: n.ID, BaseRevision: n.EditRevision, Protocol: n.Protocol, Host: n.Host, Port: n.Port, CurrentState: &n.CurrentState, ProtocolJSON: incoming, Targets: []string{"clash-yaml"}})
			if err != nil {
				t.Fatal(err)
			}
			result := response.Targets["clash-yaml"]
			if result.Status != "error" || result.Preview != nil || rendered || len(result.Diagnostics) == 0 || result.Diagnostics[0].FieldPath != path {
				t.Errorf("必填列表检查未精确拒绝: status=%s path=%s", result.Status, path)
			}
			_, err = svc.UpdateManual(ctx, n.ID, UpdateManualInput{Protocol: n.Protocol, Host: "must-not-write.test", Port: 443, CurrentState: &n.CurrentState, BaseRevision: n.EditRevision, ProtocolJSON: incoming})
			if !errors.Is(err, ErrBadRequest) {
				t.Errorf("必填列表保存必须拒绝: %v", err)
			}
			after, err := svc.getRaw(ctx, n.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatal("必填列表失败检查或保存修改了节点")
			}
		})
	}
}

// 两个阶段只改变已提交空列表的表达，不制造缺失字段、不吞类型错误、不修改输入。
func TestTextListNormalizationStages(t *testing.T) {
	proto := wireGuardProto(t)
	for _, tc := range []struct {
		name  string
		input map[string]any
		empty bool
	}{
		{"omitted", map[string]any{}, false},
		{"null", map[string]any{"allowed-ips": nil}, false},
		{"empty-any", map[string]any{"allowed-ips": []any{}}, true},
		{"empty-string-list", map[string]any{"allowed-ips": []string{}}, true},
		{"blank", map[string]any{"allowed-ips": "\n \t"}, true},
		{"blank-items", map[string]any{"allowed-ips": []any{" ", "\t"}}, true},
		{"invalid", map[string]any{"allowed-ips": []any{false}}, false},
		{"ordered", map[string]any{"allowed-ips": []any{" ::/0 ", "10.0.0.0/24", "::/0"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.input["mtu"] = nil
			tc.input["future"] = map[string]any{"list": []any{}}
			before := cloneJSONMap(tc.input)
			incoming, err := normalizeProtocolJSONForMerge(proto, tc.input)
			if err != nil {
				t.Fatal(err)
			}
			canonical, err := NormalizeProtocolJSON(proto, incoming)
			if err != nil {
				t.Fatal(err)
			}
			if tc.empty {
				items, ok := stringListItems(incoming["allowed-ips"])
				if !ok || len(items) != 0 {
					t.Fatal("合并输入丢失空列表清空信号")
				}
				if _, exists := canonical["allowed-ips"]; exists {
					t.Fatal("规范参数不得保留空列表键")
				}
			} else if tc.name == "ordered" {
				items, ok := stringListItems(canonical["allowed-ips"])
				if !ok || !reflect.DeepEqual(items, []string{"::/0", "10.0.0.0/24"}) {
					t.Fatal("非空列表未去重保序")
				}
			} else if !reflect.DeepEqual(canonical, tc.input) {
				t.Fatal("归一化改变了省略、null 或非法类型的既有边界")
			}
			if _, exists := canonical["mtu"]; !exists || !reflect.DeepEqual(canonical["future"], before["future"]) || !reflect.DeepEqual(tc.input, before) {
				t.Fatal("归一化越界清理或修改了输入")
			}
			again, err := NormalizeProtocolJSON(proto, canonical)
			if err != nil || !reflect.DeepEqual(again, canonical) {
				t.Fatal("规范归一化不幂等")
			}
		})
	}
}

func TestTextListUnsetRevisionConflictIsAtomic(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	params := wireGuardSingleParams()
	params["allowed-ips"] = []any{"10.0.0.0/24"}
	n, err := createWireGuard(t, svc, "list-conflict", params, nil)
	if err != nil {
		t.Fatal(err)
	}
	before, err := svc.getRaw(ctx, n.ID)
	if err != nil {
		t.Fatal(err)
	}
	incoming := map[string]any{"allowed-ips": []any{}}
	_, err = svc.Check(ctx, CheckRequest{NodeID: n.ID, Protocol: n.Protocol, BaseRevision: n.EditRevision - 1, ProtocolJSON: incoming})
	if !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("检查必须拒绝旧修订: %v", err)
	}
	_, err = svc.UpdateManual(ctx, n.ID, UpdateManualInput{Protocol: n.Protocol, BaseRevision: n.EditRevision - 1, ProtocolJSON: incoming})
	if !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("保存必须拒绝旧修订: %v", err)
	}
	after, err := svc.getRaw(ctx, n.ID)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("旧修订清空请求修改了节点")
	}
}
