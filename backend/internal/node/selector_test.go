package node

import (
	"context"
	"strings"
	"testing"
)

// selectorTestProtocol 构造仅用于 Step 2 通用 selector 合同的协议。
func selectorTestProtocol() Protocol {
	return Protocol{
		Protocol: "selector-test",
		FormSchema: []FieldSchema{
			{Name: "branch", Type: "select", Required: true, Options: []string{"a", "b"}, SelectorName: "mode"},
			{Name: "branch-value", Type: "text", ResetOn: []string{"selector.mode"}},
			{Name: "secret", Type: "password", ResetOn: []string{"selector.mode"}},
			{Name: "auth-mode", Type: "select", StateOnly: true, Options: []string{"none", "basic"}, SelectorName: "auth_mode"},
			{Name: "auth-value", Type: "text", ResetOn: []string{"selector.auth_mode"}},
		},
		Selectors: []SelectorSchema{
			{Name: "mode", Values: []string{"a", "b"}, Default: "a", SourceField: "branch"},
			{Name: "auth_mode", Values: []string{"none", "basic"}, Default: "none"},
		},
		SensitiveFields: []string{"secret"},
	}
}

func TestValidateProtocolSelectors(t *testing.T) {
	if err := validateProtocolSelectors(selectorTestProtocol()); err != nil {
		t.Fatalf("合法 selector 协议注册失败: %v", err)
	}
	cases := []struct {
		name  string
		proto Protocol
	}{
		{
			name: "未声明 selector",
			proto: Protocol{Protocol: "bad", FormSchema: []FieldSchema{
				{Name: "mode", Type: "select", SelectorName: "mode"},
			}},
		},
		{
			name: "重复 selector",
			proto: Protocol{Protocol: "bad", FormSchema: []FieldSchema{
				{Name: "mode", Type: "select", SelectorName: "mode"},
			}, Selectors: []SelectorSchema{
				{Name: "mode", Values: []string{"a"}, Default: "a", SourceField: "mode"},
				{Name: "mode", Values: []string{"b"}, Default: "b", SourceField: "mode"},
			}},
		},
		{
			name: "source_field 路径不一致",
			proto: Protocol{Protocol: "bad", FormSchema: []FieldSchema{
				{Name: "wire", Type: "select", SelectorName: "mode"},
			}, Selectors: []SelectorSchema{
				{Name: "mode", Values: []string{"a"}, Default: "a", SourceField: "other"},
			}},
		},
		{
			name: "state_only 字段类型错误",
			proto: Protocol{Protocol: "bad", FormSchema: []FieldSchema{
				{Name: "auth-mode", Type: "text", StateOnly: true, SelectorName: "auth_mode"},
			}, Selectors: []SelectorSchema{
				{Name: "auth_mode", Values: []string{"none"}, Default: "none"},
			}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateProtocolSelectors(tc.proto); err == nil {
				t.Fatal("非法 selector 注册应失败")
			}
		})
	}
}

func TestSelectorMatchesAndProjection(t *testing.T) {
	proto := selectorTestProtocol()
	proto.FormSchema = append(proto.FormSchema, FieldSchema{
		Name: "b-value", Type: "text", When: &ConditionRule{Selectors: map[string][]string{"mode": {"b"}}},
	})
	stateA := CurrentState{Selectors: map[string]string{"mode": "a"}}
	stateB := CurrentState{Selectors: map[string]string{"mode": "b"}}
	if !proto.FormSchema[len(proto.FormSchema)-1].Matches(stateB, "") {
		t.Fatal("selector=b 时字段应活动")
	}
	if proto.FormSchema[len(proto.FormSchema)-1].Matches(stateA, "") {
		t.Fatal("selector=a 时字段不应活动")
	}
	params := map[string]any{"branch": "a", "branch-value": "keep", "b-value": "hidden", "auth-mode": "none"}
	projected := ProjectActive(proto, stateA, params)
	if _, exists := projected["auth-mode"]; exists {
		t.Fatal("state_only 字段不能进入活动投影")
	}
	if _, exists := projected["b-value"]; exists {
		t.Fatal("非活动 selector 分支不能进入活动投影")
	}
}

func TestResolveCurrentStateSelectors(t *testing.T) {
	proto := selectorTestProtocol()
	params := map[string]any{}
	state, err := resolveCurrentState(proto, nil, params)
	if err != nil {
		t.Fatal(err)
	}
	if state.Selectors["mode"] != "a" || state.Selectors["auth_mode"] != "none" {
		t.Fatalf("selector 默认派生错误: %+v", state.Selectors)
	}
	if params["branch"] != "a" {
		t.Fatalf("普通 selector 应同步写入 source_field: %+v", params)
	}
	if _, exists := params["auth-mode"]; exists {
		t.Fatal("state_only selector 不得写入 protocol_json")
	}

	_, err = resolveCurrentState(proto, &CurrentState{Selectors: map[string]string{"mode": "b"}}, map[string]any{"branch": "a"})
	if err == nil || !strings.Contains(err.Error(), "与 protocol_json 不一致") {
		t.Fatalf("wire selector 与 protocol_json 冲突应失败: %v", err)
	}
	_, err = resolveCurrentState(proto, &CurrentState{Selectors: map[string]string{"unknown": "x"}}, map[string]any{"branch": "a"})
	if err == nil || !strings.Contains(err.Error(), "未声明 selector") {
		t.Fatalf("未声明 selector 应失败: %v", err)
	}
	state, err = resolveCurrentState(proto, &CurrentState{Selectors: map[string]string{"auth_mode": "basic"}}, map[string]any{"branch": "b"})
	if err != nil {
		t.Fatal(err)
	}
	if state.Selectors["auth_mode"] != "basic" || state.Selectors["mode"] != "b" {
		t.Fatalf("state_only selector 归一化错误: %+v", state.Selectors)
	}
}

func TestHydrateCurrentStateForReadKeepsV1MemoryOnly(t *testing.T) {
	proto := selectorTestProtocol()
	params := map[string]any{"branch": "b"}
	state := CurrentState{}
	derived := hydrateCurrentStateForRead(proto, state, params, 1)
	if derived.Selectors["mode"] != "b" {
		t.Fatalf("v1 读取应派生 selector: %+v", derived.Selectors)
	}
	if state.Selectors != nil {
		t.Fatal("v1 读取不得回写原状态")
	}
	if params["branch"] != "b" {
		t.Fatal("v1 读取不得改写 protocol_json")
	}
}

func TestNormalizeResetScopesSelectorWhitelist(t *testing.T) {
	proto := selectorTestProtocol()
	got, err := normalizeResetScopes(proto, []string{"selector.mode", "selector.auth_mode"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("selector reset scope 数量错误: %v", got)
	}
	if _, err := normalizeResetScopes(proto, []string{"selector.unknown"}); err == nil {
		t.Fatal("未知 selector reset scope 应失败")
	}
}

func TestSelectorSwitchClearsAndDoesNotRestore(t *testing.T) {
	proto := selectorTestProtocol()
	existing := map[string]any{"branch": "a", "branch-value": "old", "secret": "cipher"}
	oldState := hydrateCurrentStateForRead(proto, CurrentState{Selectors: map[string]string{"mode": "a"}}, existing, 2)

	incoming := map[string]any{"branch": "b"}
	newState, err := resolveCurrentState(proto, &CurrentState{Selectors: map[string]string{"mode": "b"}}, incoming)
	if err != nil {
		t.Fatal(err)
	}
	scopes := selectorResetScopes(proto, oldState, newState)
	if len(scopes) != 1 || scopes[0] != "selector.mode" {
		t.Fatalf("selector 切换未生成 reset scope: %v", scopes)
	}
	if !pathInResetScope("secret", "selector.mode", proto.FormSchema) {
		t.Fatal("selector reset scope 应能命中敏感字段")
	}
	merged := mergeProtocolJSON(existing, incoming, proto, scopes)
	if _, exists := merged["branch-value"]; exists {
		t.Fatalf("A→B 应清空旧分支参数: %+v", merged)
	}
	if merged["branch"] != "b" {
		t.Fatalf("新 selector source_field 缺失: %+v", merged)
	}

	back := map[string]any{"branch": "a"}
	backState, err := resolveCurrentState(proto, &CurrentState{Selectors: map[string]string{"mode": "a"}}, back)
	if err != nil {
		t.Fatal(err)
	}
	backScopes := selectorResetScopes(proto, newState, backState)
	mergedBack := mergeProtocolJSON(merged, back, proto, backScopes)
	if _, exists := mergedBack["branch-value"]; exists {
		t.Fatalf("A→B→A 不得恢复旧参数: %+v", mergedBack)
	}
}

func TestValidateStateOnlyParamsRejectsProtocolJSON(t *testing.T) {
	proto := selectorTestProtocol()
	if err := validateStateOnlyParams(proto, map[string]any{"auth-mode": "none"}); err == nil {
		t.Fatal("state_only 字段进入 protocol_json 应失败")
	}
	if err := validateStateOnlyParams(proto, map[string]any{"branch": "a"}); err != nil {
		t.Fatalf("普通字段不应被误判为 state_only: %v", err)
	}
}

func TestSelectorServiceSaveReopenAndClear(t *testing.T) {
	proto := selectorTestProtocol()
	proto.Protocol = "selector-test-api"
	proto.Label = "Selector Test"
	protocolIndex[proto.Protocol] = proto
	defer delete(protocolIndex, proto.Protocol)

	svc, st, _ := newTestService(t)
	ctx := context.Background()
	created, err := svc.CreateManual(ctx, CreateManualInput{
		Name: "selector-api", Protocol: proto.Protocol, Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"branch": "a", "branch-value": "old", "secret": "old-secret"},
		CurrentState: &CurrentState{Selectors: map[string]string{"auth_mode": "basic"}},
		Extensions: []ExtensionInput{{
			ID: "ext-selector", Scope: "selector.mode", Targets: []string{"clash-yaml"},
			Label: "selector 扩展", Payload: `{"old":true}`,
		}},
	})
	if err != nil {
		t.Fatalf("创建 selector 节点失败: %v", err)
	}
	if created.StateFormatVersion != currentStateFormatVersion || created.CurrentState.Selectors["mode"] != "a" || created.CurrentState.Selectors["auth_mode"] != "basic" {
		t.Fatalf("创建后 selector 状态异常: %+v", created)
	}

	updated, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		BaseRevision: created.EditRevision, Protocol: proto.Protocol, Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"branch": "b"},
		CurrentState: &CurrentState{Selectors: map[string]string{"mode": "b", "auth_mode": "basic"}},
	})
	if err != nil {
		t.Fatalf("selector A→B 更新失败: %v", err)
	}
	if updated.CurrentState.Selectors["mode"] != "b" || updated.ProtocolJSON["branch-value"] != nil {
		t.Fatalf("A→B 应清空旧 selector 字段并保存新值: %+v", updated)
	}
	if len(updated.Extensions) != 0 {
		t.Fatalf("selector 切换应清空所属扩展: %+v", updated.Extensions)
	}
	raw, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := raw.ProtocolJSON["secret"]; exists {
		t.Fatal("selector 切换后旧敏感字段不得保留")
	}

	var beforeRevision int
	var beforeProtocol, beforeState, beforeExtensions string
	if err := st.DB().QueryRowContext(ctx,
		`SELECT edit_revision, protocol_json, current_state_json, extensions_json FROM nodes WHERE id = ?`, created.ID).
		Scan(&beforeRevision, &beforeProtocol, &beforeState, &beforeExtensions); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		BaseRevision: updated.EditRevision, Protocol: proto.Protocol, Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"branch": "a"},
		CurrentState: &CurrentState{Selectors: map[string]string{"mode": "invalid"}},
	}); err == nil {
		t.Fatal("非法 selector 保存应失败")
	}
	var failedRevision int
	var failedProtocol, failedState, failedExtensions string
	if err := st.DB().QueryRowContext(ctx,
		`SELECT edit_revision, protocol_json, current_state_json, extensions_json FROM nodes WHERE id = ?`, created.ID).
		Scan(&failedRevision, &failedProtocol, &failedState, &failedExtensions); err != nil {
		t.Fatal(err)
	}
	if failedRevision != beforeRevision || failedProtocol != beforeProtocol || failedState != beforeState || failedExtensions != beforeExtensions {
		t.Fatal("失败保存不得改变数据库")
	}
	svc.SetCheckRenderer(func(context.Context, string, string, string, string, int, map[string]any) (CheckRenderResult, error) {
		return CheckRenderResult{}, nil
	})
	if _, err := svc.Check(ctx, CheckRequest{
		NodeID: created.ID, BaseRevision: updated.EditRevision, Protocol: proto.Protocol,
		Host: "example.com", Port: 443, ProtocolJSON: map[string]any{"branch": "b"},
		CurrentState: &CurrentState{Selectors: map[string]string{"mode": "b", "auth_mode": "basic"}},
	}); err != nil {
		t.Fatalf("selector 节点检查失败: %v", err)
	}
	var afterRevision int
	var afterProtocol, afterState, afterExtensions string
	if err := st.DB().QueryRowContext(ctx,
		`SELECT edit_revision, protocol_json, current_state_json, extensions_json FROM nodes WHERE id = ?`, created.ID).
		Scan(&afterRevision, &afterProtocol, &afterState, &afterExtensions); err != nil {
		t.Fatal(err)
	}
	if beforeRevision != afterRevision || beforeProtocol != afterProtocol || beforeState != afterState || beforeExtensions != afterExtensions {
		t.Fatal("节点检查不得写数据库")
	}

	back, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		BaseRevision: updated.EditRevision, Protocol: proto.Protocol, Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"branch": "a"},
		CurrentState: &CurrentState{Selectors: map[string]string{"mode": "a", "auth_mode": "basic"}},
	})
	if err != nil {
		t.Fatalf("selector B→A 更新失败: %v", err)
	}
	if back.CurrentState.Selectors["mode"] != "a" || back.ProtocolJSON["branch-value"] != nil {
		t.Fatalf("A→B→A 不得恢复旧参数: %+v", back)
	}

	got, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("重开 selector 节点失败: %v", err)
	}
	if got.StateFormatVersion != currentStateFormatVersion || got.CurrentState.Selectors["mode"] != "a" || got.CurrentState.Selectors["auth_mode"] != "basic" {
		t.Fatalf("重开后 selector 状态异常: %+v", got)
	}
}

func TestValidateCurrentStateSelectors(t *testing.T) {
	proto := selectorTestProtocol()
	params := map[string]any{"branch": "a"}
	if err := ValidateCurrentState(proto, CurrentState{Selectors: map[string]string{"mode": "b"}}, params); err == nil {
		t.Fatal("selector 与 wire 不一致应失败")
	}
	if err := ValidateCurrentState(proto, CurrentState{Selectors: map[string]string{"mode": "a", "auth_mode": "basic"}}, params); err != nil {
		t.Fatalf("合法 selector 状态不应失败: %v", err)
	}
}
