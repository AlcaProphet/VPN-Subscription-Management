package node

import (
	"context"
	"strings"
	"testing"

	"vpn-sub/internal/log"
)

// R28-06 审计补强：扩展 payload 明文、密文和 label 不得进入节点服务日志。
func TestExtensionPayloadAndCiphertextNotWrittenToLogs(t *testing.T) {
	svc, st, cfg := newTestService(t)
	buf := log.NewRingBuffer()
	svc = NewService(st, cfg, log.New("debug", "console", buf))
	ctx := context.Background()
	const (
		plain = "r28-06-log-plain-sentinel"
		label = "r28-06-log-label-sentinel"
	)

	created, err := svc.CreateManual(ctx, CreateManualInput{
		Name: "R28-06日志边界节点", Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp"},
		Extensions: []ExtensionInput{{
			ID: "ext-log", Scope: "node", Targets: []string{}, Label: label, Payload: plain,
		}},
	})
	if err != nil {
		t.Fatalf("创建带扩展节点失败: %v", err)
	}

	raw, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw.extensionRecords) != 1 || raw.extensionRecords[0].PayloadEnc == "" {
		t.Fatalf("测试前提失败：扩展密文缺失: %+v", raw.extensionRecords)
	}
	ciphertext := raw.extensionRecords[0].PayloadEnc

	// 触发创建、读取、更新、检查、删除等常见日志路径。
	if _, err := svc.Get(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "vless", Host: "updated.example.com", Port: 443, BaseRevision: created.EditRevision,
		ProtocolJSON: map[string]any{"uuid": "", "network": "tcp"},
		ExtensionOps: []ExtensionOp{{Op: "keep", ID: "ext-log"}},
	}); err != nil {
		t.Fatal(err)
	}
	rawAfter, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetCheckRenderer(func(_ context.Context, _, _, _, _ string, _ int, _ map[string]any) (CheckRenderResult, error) {
		return CheckRenderResult{Status: "ok"}, nil
	})
	if _, err := svc.Check(ctx, CheckRequest{
		NodeID: created.ID, BaseRevision: rawAfter.EditRevision, Protocol: "vless",
		Host: "updated.example.com", Port: 443,
		ProtocolJSON: map[string]any{"uuid": "", "network": "tcp"},
		ExtensionOps: []ExtensionOp{{Op: "keep", ID: "ext-log"}},
		Targets:      []string{"clash-yaml"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}

	for _, entry := range buf.History() {
		combined := entry.Message + "\n" + entry.Attrs
		if strings.Contains(combined, plain) {
			t.Fatalf("日志泄漏扩展明文 sentinel: %+v", entry)
		}
		if strings.Contains(combined, label) {
			t.Fatalf("日志泄漏扩展 label sentinel: %+v", entry)
		}
		if strings.Contains(combined, ciphertext) {
			t.Fatalf("日志泄漏扩展密文: %+v", entry)
		}
	}
}
