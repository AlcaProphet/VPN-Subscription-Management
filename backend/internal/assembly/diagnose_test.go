package assembly

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestGenericSubsSkipsCoreUnexpressibleNode(t *testing.T) {
	svc, st, _ := newTestService(t)
	gp := insertPlatform(t, st, "generic-subs")
	insertManualNode(t, st, "TrojanWS", "trojan", map[string]any{
		"password": "p", "network": "ws", "ws-opts": map[string]any{"path": "/ws"},
	})
	insertManualNode(t, st, "VlessTCP", "vless", map[string]any{
		"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp",
	})
	res, err := svc.Preview(context.Background(), GenerateInput{
		TargetSyntax: GenericSubs, PlatformID: gp,
		NodeNames: []string{"TrojanWS", "VlessTCP"},
	})
	if err != nil {
		t.Fatalf("generic subs 应允许保留可转换节点: %v", err)
	}
	if !strings.Contains(string(res.Content), "vless://") {
		t.Fatalf("应输出 VLESS 链接: %s", string(res.Content))
	}
	foundSkip := false
	for _, sk := range res.Skipped {
		if sk.Kind == "node" && sk.Name == "TrojanWS" {
			foundSkip = true
		}
	}
	if !foundSkip {
		t.Fatalf("应跳过 Trojan WS 核心不可表达节点: %+v", res.Skipped)
	}
}

func TestGenericSubsAllSkippedRejected(t *testing.T) {
	svc, st, _ := newTestService(t)
	gp := insertPlatform(t, st, "generic-subs")
	insertManualNode(t, st, "TrojanWS", "trojan", map[string]any{
		"password": "p", "network": "ws", "ws-opts": map[string]any{"path": "/ws"},
	})
	_, err := svc.Preview(context.Background(), GenerateInput{
		TargetSyntax: GenericSubs, PlatformID: gp,
		NodeNames: []string{"TrojanWS"},
	})
	if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "至少需要 1 个可转换链接") {
		t.Fatalf("全部节点不可表达时应拒绝生成: %v", err)
	}
}

func TestClashBlocksUnsupportedNode(t *testing.T) {
	svc, st, _ := newTestService(t)
	pid := insertPlatform(t, st, "yaml")
	insertManualNode(t, st, "BadNode", "not-a-real-protocol", map[string]any{})
	_, err := svc.Preview(context.Background(), GenerateInput{
		TargetSyntax: ClashYAML, PlatformID: pid,
		NodeNames:            []string{"BadNode"},
		OverseasMembers:      []string{"BadNode"},
		FallbackGroupMembers: []string{"🚀直接连接", "🌎国外流量"},
	})
	if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "目标检查未通过") {
		t.Fatalf("Clash 应阻止核心不可表达节点生成: %v", err)
	}
}
