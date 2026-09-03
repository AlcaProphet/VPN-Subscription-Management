package node

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestCheckNewDraftUsesActiveProjectionAndDoesNotWrite(t *testing.T) {
	svc, st, _ := newTestService(t)
	var seen map[string]any
	calls := 0
	svc.SetCheckRenderer(func(_ context.Context, target, _ string, _ string, _ string, _ int, params map[string]any) (CheckRenderResult, error) {
		calls++
		seen = cloneJSONMap(params)
		return CheckRenderResult{Preview: "redacted-preview"}, nil
	})
	var before int
	if err := st.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM nodes`).Scan(&before); err != nil {
		t.Fatalf("读取检查前节点数量失败: %v", err)
	}
	resp, err := svc.Check(context.Background(), CheckRequest{
		Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{
			"uuid": "check-secret", "network": "grpc", "tls": true,
			"grpc-opts": map[string]any{"grpc-service-name": "service"},
		},
	})
	if err != nil {
		t.Fatalf("新建草稿检查失败: %v", err)
	}
	if calls != len(defaultCheckTargets) || len(resp.Targets) != len(defaultCheckTargets) {
		t.Fatalf("检查目标调用数量异常: calls=%d targets=%d", calls, len(resp.Targets))
	}
	for target, result := range resp.Targets {
		if result.Status != "ok" || result.Preview == nil || *result.Preview != "redacted-preview" {
			t.Fatalf("目标 %s 检查结果异常: %+v", target, result)
		}
	}
	if seen["uuid"] != "REDACTED" {
		t.Fatalf("检查适配器应只收到脱敏 UUID: %#v", seen["uuid"])
	}
	if _, ok := seen["security"]; ok {
		t.Fatalf("表单层 security 不应进入实际检查参数: %#v", seen)
	}
	if _, ok := seen["grpc-opts"]; !ok {
		t.Fatalf("当前 gRPC 分支参数未投影: %#v", seen)
	}
	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("序列化检查响应失败: %v", err)
	}
	if strings.Contains(string(encoded), "check-secret") {
		t.Fatalf("检查响应泄漏凭据: %s", encoded)
	}
	var after int
	if err := st.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM nodes`).Scan(&after); err != nil {
		t.Fatalf("读取检查后节点数量失败: %v", err)
	}
	if before != after {
		t.Fatalf("新建草稿检查不应写入 nodes: before=%d after=%d", before, after)
	}
}

func TestCheckWireGuardArrayCredentialIsRedactedAndInternalIDIsStripped(t *testing.T) {
	svc, _, _ := newTestService(t)
	var seen map[string]any
	svc.SetCheckRenderer(func(_ context.Context, _, _ string, _ string, _ string, _ int, params map[string]any) (CheckRenderResult, error) {
		seen = cloneJSONMap(params)
		return CheckRenderResult{Preview: "safe-preview"}, nil
	})
	resp, err := svc.Check(context.Background(), CheckRequest{
		Protocol: "wireguard", Host: "example.com", Port: 51820,
		ProtocolJSON: map[string]any{
			"private-key": "private-secret", "public-key": "server-public",
			"peers": []any{map[string]any{"server": "peer", "pre-shared-key": "peer-secret"}},
		},
		Targets: []string{"clash-yaml"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Targets["clash-yaml"].Status != "ok" {
		t.Fatalf("WireGuard 草稿检查失败: %+v", resp)
	}
	peers := seen["peers"].([]any)
	peer := peers[0].(map[string]any)
	if peer["pre-shared-key"] != "REDACTED" {
		t.Fatalf("Peer 凭据未脱敏: %+v", peer)
	}
	if _, ok := peer[sensitiveItemIDField]; ok {
		t.Fatalf("内部 Peer ID 进入检查适配器: %+v", peer)
	}
	encoded, _ := json.Marshal(resp)
	if strings.Contains(string(encoded), "peer-secret") || strings.Contains(string(encoded), sensitiveItemIDField) {
		t.Fatalf("检查响应泄漏 Peer 凭据或内部 ID: %s", encoded)
	}
}

func TestCheckEditAppliesResetInMemoryAndPreservesDatabase(t *testing.T) {
	svc, _, _ := newTestService(t)
	created, err := svc.CreateManual(context.Background(), CreateManualInput{
		Name: "检查编辑节点", Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{
			"uuid": "stored-secret", "network": "ws", "tls": true,
			"ws-opts": map[string]any{"path": "/old"},
		},
	})
	if err != nil {
		t.Fatalf("创建检查编辑节点失败: %v", err)
	}
	before, err := svc.getRaw(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("读取检查前节点失败: %v", err)
	}
	var seen map[string]any
	svc.SetCheckRenderer(func(_ context.Context, _ string, _ string, _ string, _ string, _ int, params map[string]any) (CheckRenderResult, error) {
		seen = cloneJSONMap(params)
		return CheckRenderResult{}, nil
	})
	resp, err := svc.Check(context.Background(), CheckRequest{
		NodeID: created.ID, BaseRevision: created.EditRevision,
		Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{
			"uuid": "", "network": "grpc", "tls": true,
			"grpc-opts": map[string]any{"grpc-service-name": "new-service"},
		},
		CurrentState: &CurrentState{Network: "grpc", Security: "tls", Features: []string{}},
		ResetScopes:  []string{"network"},
		Targets:      []string{"clash-yaml"},
	})
	if err != nil {
		t.Fatalf("编辑草稿检查失败: %v", err)
	}
	if resp.Targets["clash-yaml"].Status != "ok" {
		t.Fatalf("编辑草稿检查状态异常: %+v", resp.Targets["clash-yaml"])
	}
	if _, ok := seen["ws-opts"]; ok {
		t.Fatalf("网络重置后检查参数不应恢复旧 ws-opts: %#v", seen)
	}
	if _, ok := seen["grpc-opts"]; !ok || seen["uuid"] != "REDACTED" {
		t.Fatalf("编辑检查参数未使用新分支/脱敏凭据: %#v", seen)
	}
	after, err := svc.getRaw(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("读取检查后节点失败: %v", err)
	}
	beforeJSON, _ := json.Marshal(before.ProtocolJSON)
	afterJSON, _ := json.Marshal(after.ProtocolJSON)
	if string(beforeJSON) != string(afterJSON) || before.EditRevision != after.EditRevision || before.Host != after.Host || before.Port != after.Port {
		t.Fatalf("编辑草稿检查不应改变已保存节点: before=%+v after=%+v", before, after)
	}
}

func TestCheckValidationResponseIncludesFieldPath(t *testing.T) {
	svc, _, _ := newTestService(t)
	resp, err := svc.Check(context.Background(), CheckRequest{
		Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{
			"uuid": "uuid", "network": "xhttp", "tls": true,
			"xhttp-opts": map[string]any{"mode": "none"},
		},
		Targets: []string{"generic-subs"},
	})
	if err != nil {
		t.Fatalf("非法草稿检查不应以服务错误结束: %v", err)
	}
	result := resp.Targets["generic-subs"]
	if result.Status != "error" || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "invalid_node_draft" || result.Diagnostics[0].FieldPath != "xhttp-opts.mode" {
		t.Fatalf("非法草稿诊断异常: %+v", result)
	}
}

func TestCheckAppliesTargetSpecificRequiredFields(t *testing.T) {
	svc, _, _ := newTestService(t)
	svc.SetCheckRenderer(func(_ context.Context, _ string, _ string, _ string, _ string, _ int, _ map[string]any) (CheckRenderResult, error) {
		return CheckRenderResult{Preview: "preview"}, nil
	})
	resp, err := svc.Check(context.Background(), CheckRequest{
		Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{
			"uuid": "uuid", "network": "tcp", "tls": true,
			"reality-opts": map[string]any{"public-key": "public-key"},
		},
		Targets: []string{"clash-yaml", "sr-subs"},
	})
	if err != nil {
		t.Fatalf("目标条件必填检查不应以服务错误结束: %v", err)
	}
	clash := resp.Targets["clash-yaml"]
	if clash.Status != "error" || len(clash.Diagnostics) != 1 || clash.Diagnostics[0].FieldPath != "reality-opts.short-id" {
		t.Fatalf("Clash 目标条件必填诊断异常: %+v", clash)
	}
	if sr := resp.Targets["sr-subs"]; sr.Status != "ok" || sr.Preview == nil {
		t.Fatalf("SR 目标不应套用 Clash 专属必填: %+v", sr)
	}
}

func TestCheckRevisionConflict(t *testing.T) {
	svc, _, _ := newTestService(t)
	created := createManual(t, svc, "检查修订节点")
	_, err := svc.Check(context.Background(), CheckRequest{
		NodeID: created.ID, BaseRevision: created.EditRevision - 1,
		Protocol: "vless", Host: created.Host, Port: created.Port,
		ProtocolJSON: map[string]any{"uuid": "", "network": "tcp"},
	})
	if !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("检查旧修订应返回 ErrRevisionConflict: %v", err)
	}
}

func TestCheckEditUsesSameFieldValidationAsSave(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	created := createManual(t, svc, "检查保存一致性节点")

	req := CheckRequest{
		NodeID: created.ID, BaseRevision: created.EditRevision,
		Protocol: "vless", Host: created.Host, Port: created.Port,
		ProtocolJSON: map[string]any{
			"uuid": "", "network": "tcp", "ws-opts": "not-an-object",
		},
		CurrentState: &CurrentState{Network: "tcp", Security: "none", Features: []string{}},
		Targets:      []string{"generic-subs"},
	}
	resp, err := svc.Check(ctx, req)
	if err != nil {
		t.Fatalf("非法草稿检查不应以服务错误结束: %v", err)
	}
	result := resp.Targets["generic-subs"]
	if result.Status != "error" || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "invalid_node_draft" {
		t.Fatalf("检查应发现与保存相同的字段类型错误: %+v", result)
	}

	_, err = svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "vless", Host: created.Host, Port: created.Port, BaseRevision: created.EditRevision,
		ProtocolJSON: map[string]any{
			"uuid": "", "network": "tcp", "ws-opts": "not-an-object",
		},
		CurrentState: &CurrentState{Network: "tcp", Security: "none", Features: []string{}},
	})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("正式保存应拒绝同一草稿: %v", err)
	}
}
