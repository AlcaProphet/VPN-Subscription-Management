package node

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// R28-06A：扩展 targets 合法集合、空 targets、失败零写入与 revision 不变。
func TestExtensionTargetsExplicitWhitelist(t *testing.T) {
	svc, st, _ := newTestService(t)
	ctx := context.Background()
	baseParams := map[string]any{"uuid": "r28-06-base-secret", "network": "tcp"}

	t.Run("create allows empty targets", func(t *testing.T) {
		created, err := svc.CreateManual(ctx, CreateManualInput{
			Name: "空targets扩展", Protocol: "vless", Host: "example.com", Port: 443,
			ProtocolJSON: cloneJSONMap(baseParams),
			Extensions: []ExtensionInput{{
				ID: "ext-empty", Scope: "node", Targets: []string{}, Payload: "empty-target-payload",
			}},
		})
		if err != nil {
			t.Fatalf("空 targets 扩展应允许创建: %v", err)
		}
		raw, err := svc.getRaw(ctx, created.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(raw.extensionRecords) != 1 || len(raw.extensionRecords[0].Targets) != 0 {
			t.Fatalf("空 targets 应原样保存为空数组: %+v", raw.extensionRecords)
		}
	})

	t.Run("create allows legal targets and deduplicates", func(t *testing.T) {
		created, err := svc.CreateManual(ctx, CreateManualInput{
			Name: "合法targets扩展", Protocol: "vless", Host: "example.com", Port: 443,
			ProtocolJSON: cloneJSONMap(baseParams),
			Extensions: []ExtensionInput{{
				ID: "ext-legal", Scope: "node",
				Targets: []string{" clash-yaml ", "clash-yaml", "sr-subs"},
				Payload: "legal-target-payload",
			}},
		})
		if err != nil {
			t.Fatalf("合法 targets 扩展应允许创建: %v", err)
		}
		if !reflect.DeepEqual(created.Extensions[0].Targets, []string{"clash-yaml", "sr-subs"}) {
			t.Fatalf("合法 targets 应 trim/去重: %+v", created.Extensions[0].Targets)
		}
	})

	t.Run("create rejects illegal target with zero write", func(t *testing.T) {
		_, err := svc.CreateManual(ctx, CreateManualInput{
			Name: "非法targets扩展", Protocol: "vless", Host: "example.com", Port: 443,
			ProtocolJSON: cloneJSONMap(baseParams),
			Extensions: []ExtensionInput{{
				ID: "ext-bad", Scope: "node", Targets: []string{"sr-conf"}, Payload: "bad-target-payload",
			}},
		})
		if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "sr-conf") {
			t.Fatalf("非法 target 应返回明确校验错误: %v", err)
		}
		var count int
		if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM nodes WHERE name = ?`, "非法targets扩展").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("非法 target 创建失败后不应写入节点，实际 count=%d", count)
		}
	})

	t.Run("update rejects illegal target without revision or record changes", func(t *testing.T) {
		created := createManual(t, svc, "非法targets更新")
		before, err := svc.getRaw(ctx, created.ID)
		if err != nil {
			t.Fatal(err)
		}
		_, err = svc.UpdateManual(ctx, created.ID, UpdateManualInput{
			Protocol: "vless", Host: "example.com", Port: 443, BaseRevision: created.EditRevision,
			ProtocolJSON: map[string]any{"uuid": "", "network": "tcp"},
			ExtensionOps: []ExtensionOp{{
				Op: "add", ID: "ext-bad", Scope: "node", Targets: []string{"sr-conf"}, Payload: "bad",
			}},
		})
		if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "sr-conf") {
			t.Fatalf("非法 target 更新应返回明确校验错误: %v", err)
		}
		after, err := svc.getRaw(ctx, created.ID)
		if err != nil {
			t.Fatal(err)
		}
		if after.EditRevision != before.EditRevision || len(after.extensionRecords) != 0 {
			t.Fatalf("非法 target 更新失败不应递增 revision 或写入扩展: before_rev=%d after_rev=%d records=%+v",
				before.EditRevision, after.EditRevision, after.extensionRecords)
		}
	})

	t.Run("replace rejects illegal target and keeps old ciphertext", func(t *testing.T) {
		created, err := svc.CreateManual(ctx, CreateManualInput{
			Name: "非法targets替换", Protocol: "vless", Host: "example.com", Port: 443,
			ProtocolJSON: cloneJSONMap(baseParams),
			Extensions: []ExtensionInput{{
				ID: "ext-keep", Scope: "node", Targets: []string{"clash-yaml"}, Payload: "old-payload",
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		before, err := svc.getRaw(ctx, created.ID)
		if err != nil {
			t.Fatal(err)
		}
		_, err = svc.UpdateManual(ctx, created.ID, UpdateManualInput{
			Protocol: "vless", Host: "example.com", Port: 443, BaseRevision: created.EditRevision,
			ProtocolJSON: map[string]any{"uuid": "", "network": "tcp"},
			ExtensionOps: []ExtensionOp{{
				Op: "replace", ID: "ext-keep", Targets: []string{"generic-subs", "sr-conf"}, Payload: "new-payload",
			}},
		})
		if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "sr-conf") {
			t.Fatalf("非法 replace target 应返回明确校验错误: %v", err)
		}
		after, err := svc.getRaw(ctx, created.ID)
		if err != nil {
			t.Fatal(err)
		}
		if after.EditRevision != before.EditRevision || !reflect.DeepEqual(after.extensionRecords, before.extensionRecords) {
			t.Fatalf("非法 replace 不应改变 revision 或扩展密文: before=%+v after=%+v", before.extensionRecords, after.extensionRecords)
		}
	})
}

// R28-06A：空 targets、命中 targets、非命中 targets 的诊断矩阵与 status。
func TestExtensionDiagnosticsTargetMatrix(t *testing.T) {
	svc, _, _ := newTestService(t)
	svc.SetCheckRenderer(func(_ context.Context, _, _, _, _ string, _ int, _ map[string]any) (CheckRenderResult, error) {
		return CheckRenderResult{Status: "ok"}, nil
	})
	resp, err := svc.Check(context.Background(), CheckRequest{
		Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"uuid": "check-extension-secret", "network": "tcp", "tls": false},
		CurrentState: &CurrentState{Network: "tcp", Security: "none", Features: []string{}},
		Extensions: []ExtensionInput{
			{ID: "ext-empty", Scope: "node", Targets: []string{}, Payload: "empty"},
			{ID: "ext-clash", Scope: "node", Targets: []string{"clash-yaml"}, Payload: "clash"},
			{ID: "ext-sr", Scope: "node", Targets: []string{"sr-subs"}, Payload: "sr"},
		},
		Targets: []string{"clash-yaml", "sr-subs", "generic-subs"},
	})
	if err != nil {
		t.Fatalf("扩展诊断检查失败: %v", err)
	}

	wantCodes := map[string]map[string]string{
		"clash-yaml": {
			"extensions.ext-empty": "unknown_extension_not_targeted",
			"extensions.ext-clash": "unknown_extension_not_rendered",
		},
		"sr-subs": {
			"extensions.ext-empty": "unknown_extension_not_targeted",
			"extensions.ext-sr":    "unknown_extension_not_rendered",
		},
		"generic-subs": {
			"extensions.ext-empty": "unknown_extension_not_targeted",
		},
	}
	for target, want := range wantCodes {
		result, ok := resp.Targets[target]
		if !ok {
			t.Fatalf("缺少目标 %s 的结果: %+v", target, resp.Targets)
		}
		if result.Diagnostics == nil {
			t.Fatalf("目标 %s diagnostics 必须为数组，不能为 null", target)
		}
		if result.Status != "warn" {
			t.Fatalf("扩展诊断存在时目标 %s 不得返回虚假 ok，实际 status=%s", target, result.Status)
		}
		got := make(map[string]string, len(result.Diagnostics))
		for _, diag := range result.Diagnostics {
			if diag.Severity != "warn" || diag.Target != target {
				t.Fatalf("目标 %s 扩展诊断级别/target 异常: %+v", target, diag)
			}
			got[diag.FieldPath] = diag.Code
			if strings.Contains(diag.Message, "empty") || strings.Contains(diag.Message, "clash") || strings.Contains(diag.Message, "sr") {
				t.Fatalf("诊断不应包含载荷 sentinel: %+v", diag)
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("目标 %s 扩展诊断矩阵异常: got=%+v want=%+v", target, got, want)
		}
	}
}

// R28-06A：check 草稿的扩展 targets 也必须拒绝非法值；请求 targets 本身另按节点检查集合拒绝。
func TestExtensionCheckDraftRejectsIllegalTargets(t *testing.T) {
	svc, _, _ := newTestService(t)
	svc.SetCheckRenderer(func(_ context.Context, _, _, _, _ string, _ int, _ map[string]any) (CheckRenderResult, error) {
		return CheckRenderResult{Status: "ok"}, nil
	})
	resp, err := svc.Check(context.Background(), CheckRequest{
		Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"uuid": "check-extension-secret", "network": "tcp"},
		Extensions:   []ExtensionInput{{ID: "ext-bad", Scope: "node", Targets: []string{"sr-conf"}, Payload: "bad"}},
		Targets:      []string{"clash-yaml", "sr-subs", "generic-subs"},
	})
	if err != nil {
		t.Fatalf("扩展 targets 非法应在 check 内返回字段诊断，而不是协议层 500: %v", err)
	}
	for _, target := range []string{"clash-yaml", "sr-subs", "generic-subs"} {
		result := resp.Targets[target]
		if len(result.Diagnostics) == 0 || !strings.Contains(result.Diagnostics[0].Message, "sr-conf") {
			t.Fatalf("target %s 应定位非法扩展 target: %+v", target, result.Diagnostics)
		}
	}
	_, err = svc.Check(context.Background(), CheckRequest{
		Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"uuid": "check-extension-secret", "network": "tcp"},
		Targets:      []string{"sr-conf"},
	})
	if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "sr-conf") {
		t.Fatalf("请求检查目标非法应返回明确校验错误: %v", err)
	}
}

func TestExtensionSummaryDoesNotExposeCiphertextOrPayload(t *testing.T) {
	svc, st, _ := newTestService(t)
	ctx := context.Background()
	const plain = "r28-06-plain-sentinel"
	created, err := svc.CreateManual(ctx, CreateManualInput{
		Name: "扩展摘要边界", Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: cloneJSONMap(map[string]any{"uuid": "summary-secret", "network": "tcp"}),
		Extensions:   []ExtensionInput{{ID: "ext-summary", Scope: "node", Targets: []string{}, Payload: plain}},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw.extensionRecords) != 1 || !strings.HasPrefix(raw.extensionRecords[0].PayloadEnc, extEncPrefix) {
		t.Fatalf("扩展负载必须整体加密: %+v", raw.extensionRecords)
	}
	var dbRaw string
	if err := st.DB().QueryRowContext(ctx, `SELECT extensions_json FROM nodes WHERE id = ?`, created.ID).Scan(&dbRaw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(dbRaw, plain) || !strings.Contains(dbRaw, extEncPrefix) {
		t.Fatalf("数据库扩展存储不应含明文: %s", dbRaw)
	}
	seen, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(mustJSON(t, seen), plain) || strings.Contains(mustJSON(t, seen), "payload_encrypted") {
		t.Fatalf("节点 API 响应泄漏扩展明文或密文: %s", mustJSON(t, seen))
	}
	if len(seen.Extensions) != 1 || seen.Extensions[0].ID != "ext-summary" || !seen.Extensions[0].Configured {
		t.Fatalf("扩展摘要异常: %+v", seen.Extensions)
	}
}

func TestExtensionResetScopeRemovesRecords(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	created, err := svc.CreateManual(ctx, CreateManualInput{
		Name: "扩展reset边界", Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"uuid": "reset-secret", "network": "ws"},
		Extensions:   []ExtensionInput{{ID: "ext-ws", Scope: "transport.ws", Targets: []string{"clash-yaml"}, Payload: "ws-payload"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "vless", Host: "example.com", Port: 443, BaseRevision: created.EditRevision,
		ProtocolJSON: map[string]any{"uuid": "", "network": "tcp"},
		ResetScopes:  []string{"network"},
		ExtensionOps: []ExtensionOp{{Op: "keep", ID: "ext-ws"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Extensions) != 0 {
		t.Fatalf("reset scope 后扩展应清除: %+v", updated.Extensions)
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
