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

func TestSSPluginDiagnosticsMatchFormalAssemblyGates(t *testing.T) {
	t.Run("clash blocks missing required field", func(t *testing.T) {
		svc, st, _ := newTestService(t)
		pid := insertPlatform(t, st, "yaml")
		insertManualNode(t, st, "BadRestls", "ss", map[string]any{
			"cipher": "aes-256-gcm", "password": "main-secret", "plugin": "restls",
			"restls-opts": map[string]any{"password": "restls-secret", "version-hint": "tls13"},
		})
		_, err := svc.Preview(context.Background(), GenerateInput{
			TargetSyntax: ClashYAML, PlatformID: pid, NodeNames: []string{"BadRestls"},
			OverseasMembers: []string{"BadRestls"}, FallbackGroupMembers: []string{"🚀直接连接", "🌎国外流量"},
		})
		if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "restls-opts.host") {
			t.Fatalf("Clash 应按 SS 插件诊断阻断并返回字段路径: %v", err)
		}
	})

	t.Run("generic skips only unexpressible plugin node", func(t *testing.T) {
		svc, st, _ := newTestService(t)
		pid := insertPlatform(t, st, "generic-subs")
		insertManualNode(t, st, "ShadowTLS", "ss", map[string]any{
			"cipher": "aes-256-gcm", "password": "main-secret", "plugin": "shadow-tls",
			"shadow-tls-opts": map[string]any{"host": "cdn.example.com", "password": "shadow-secret", "version": float64(3)},
		})
		insertManualNode(t, st, "PlainSS", "ss", map[string]any{"cipher": "aes-256-gcm", "password": "plain-secret"})
		res, err := svc.Preview(context.Background(), GenerateInput{TargetSyntax: GenericSubs, PlatformID: pid, NodeNames: []string{"ShadowTLS", "PlainSS"}})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(res.Content), "ss://") || len(res.Skipped) != 1 || res.Skipped[0].Name != "ShadowTLS" {
			t.Fatalf("generic 应仅跳过不支持的插件节点: content=%s skipped=%+v", res.Content, res.Skipped)
		}
	})

	t.Run("generic rejects zero plugin output", func(t *testing.T) {
		svc, st, _ := newTestService(t)
		pid := insertPlatform(t, st, "generic-subs")
		insertManualNode(t, st, "OnlyShadowTLS", "ss", map[string]any{
			"cipher": "aes-256-gcm", "password": "main-secret", "plugin": "shadow-tls",
			"shadow-tls-opts": map[string]any{"host": "cdn.example.com", "password": "shadow-secret", "version": float64(3)},
		})
		_, err := svc.Preview(context.Background(), GenerateInput{TargetSyntax: GenericSubs, PlatformID: pid, NodeNames: []string{"OnlyShadowTLS"}})
		if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "至少需要 1 个可转换链接") {
			t.Fatalf("全部 SS 插件节点被跳过时应命中零输出门槛: %v", err)
		}
	})

	t.Run("sr keeps warning and output", func(t *testing.T) {
		svc, st, _ := newTestService(t)
		pid := insertPlatform(t, st, "subs")
		insertManualNode(t, st, "V2rayPlugin", "ss", map[string]any{
			"cipher": "aes-256-gcm", "password": "main-secret", "plugin": "v2ray-plugin",
			"v2ray-plugin-opts": map[string]any{"mode": "websocket", "host": "cdn.example.com", "path": "/ss", "tls": true},
		})
		res, err := svc.Preview(context.Background(), GenerateInput{TargetSyntax: SrSubs, PlatformID: pid, NodeNames: []string{"V2rayPlugin"}})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(res.Content), "ss://") || !containsWarning(res.Warnings, "plugin_partial_mapping") {
			t.Fatalf("SR 应保留可表达链接及部分映射 warning: content=%s warnings=%+v", res.Content, res.Warnings)
		}
	})

	t.Run("clash keeps unknown plugin warning and output", func(t *testing.T) {
		svc, st, _ := newTestService(t)
		pid := insertPlatform(t, st, "yaml")
		insertManualNode(t, st, "UnknownPlugin", "ss", map[string]any{
			"cipher": "aes-256-gcm", "password": "main-secret", "plugin": "custom-plugin",
			"plugin-opts": map[string]any{"flag": "", "mode": "custom"},
		})
		res, err := svc.Preview(context.Background(), GenerateInput{
			TargetSyntax: ClashYAML, PlatformID: pid, NodeNames: []string{"UnknownPlugin"},
			OverseasMembers: []string{"UnknownPlugin"}, FallbackGroupMembers: []string{"🚀直接连接", "🌎国外流量"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(res.Content), "plugin: custom-plugin") || !containsWarning(res.Warnings, "plugin_no_verified_mapping") {
			t.Fatalf("Clash 应保留未知插件结构与 warning: content=%s warnings=%+v", res.Content, res.Warnings)
		}
	})
}
