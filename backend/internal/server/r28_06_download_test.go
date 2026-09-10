package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"vpn-sub/internal/assembly"
	"vpn-sub/internal/log"
	"vpn-sub/internal/node"
	"vpn-sub/internal/tasks"
	"vpn-sub/internal/xray"
)

// R28-06 审计补强：带未知扩展的 manual 节点生成 Clash 蓝图后，用户下载重渲染路径不得引入扩展明文或密文。
func TestR28_06DownloadRerenderDoesNotExposeExtensionSentinel(t *testing.T) {
	ctx := context.Background()
	_, st, cfg := newAssemblyTestEnv(t)
	lg := log.New("error", "console")
	const (
		plain      = "r28-06-download-plain-sentinel"
		label      = "r28-06-download-label-sentinel"
		nodeName   = "R28-06下载重渲染节点"
		renderName = "R28-06下载重渲染节点"
	)

	nodeSvc := node.NewService(st, cfg, lg)
	created, err := nodeSvc.CreateManual(ctx, node.CreateManualInput{
		Name: nodeName, Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp"},
		Extensions: []node.ExtensionInput{{
			ID: "ext-download", Scope: "node", Targets: []string{}, Label: label, Payload: plain,
		}},
	})
	if err != nil {
		t.Fatalf("创建带扩展节点失败: %v", err)
	}

	var extensionsRaw string
	if err := st.DB().QueryRowContext(ctx, `SELECT extensions_json FROM nodes WHERE id = ?`, created.ID).Scan(&extensionsRaw); err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Entries []struct {
			PayloadEnc string `json:"payload_encrypted"`
		} `json:"entries"`
	}
	if err := json.Unmarshal([]byte(extensionsRaw), &envelope); err != nil {
		t.Fatalf("解析扩展存储失败: %v", err)
	}
	if len(envelope.Entries) != 1 || envelope.Entries[0].PayloadEnc == "" {
		t.Fatalf("测试前提失败：扩展密文缺失: %s", extensionsRaw)
	}
	ciphertext := envelope.Entries[0].PayloadEnc

	res, err := st.DB().ExecContext(ctx,
		`INSERT INTO platforms (slug, name, product_type) VALUES ('r28-06-download', 'R28-06下载', 'yaml')`)
	if err != nil {
		t.Fatalf("插入平台失败: %v", err)
	}
	platformID, _ := res.LastInsertId()
	res, err = st.DB().ExecContext(ctx,
		`INSERT INTO subscriptions (slug, name, platform_id, product_type, current_version) VALUES ('r28-06-download-sub', 'R28-06下载订阅', ?, 'yaml', 1)`,
		platformID)
	if err != nil {
		t.Fatalf("插入订阅失败: %v", err)
	}
	subscriptionID, _ := res.LastInsertId()

	assemblySvc := assembly.NewService(st, cfg, lg)
	rendered, err := assemblySvc.Render(ctx, assembly.GenerateInput{
		TargetSyntax: assembly.ClashYAML, PlatformID: platformID,
		NodeNames:            []string{nodeName},
		OverseasMembers:      []string{renderName},
		FallbackGroupMembers: []string{node.ForceDirect, node.ForceOverseas},
	})
	if err != nil {
		t.Fatalf("生成 Clash 蓝图失败: %v", err)
	}
	assertNoR2806DownloadSentinel(t, "render plan", string(rendered.RenderPlan), plain, ciphertext)

	res, err = st.DB().ExecContext(ctx,
		`INSERT INTO versions (owner_type, owner_id, version_no, file_path) VALUES ('subscription', ?, 1, 'r28-06-download.yaml')`,
		subscriptionID)
	if err != nil {
		t.Fatalf("插入版本失败: %v", err)
	}
	versionID, _ := res.LastInsertId()
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO assembly_blueprints (version_id, target_syntax, fixed_params_json, selection_json, custom_rules_json, render_plan_json)
		 VALUES (?, 'clash-yaml', '{}', '{"xray_candidates":[]}', '[]', ?)`,
		versionID, string(rendered.RenderPlan)); err != nil {
		t.Fatalf("插入蓝图失败: %v", err)
	}

	instSvc := xray.NewInstanceService(st, lg, tasks.NewRegistry())
	creds := xray.NewCredentialService(st, cfg)
	syncSvc := xray.NewSyncService(st, cfg, creds, instSvc, tasks.NewRegistry(), lg)
	output, err := renderUserSubscription(ctx, st, cfg, syncSvc, creds, subscriptionID, 0, rendered.Content, "r28-06-download.yaml")
	if err != nil {
		t.Fatalf("下载重渲染失败: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, renderName) {
		t.Fatalf("下载重渲染结果缺少 manual 节点名，测试前提不成立:\n%s", text)
	}
	assertNoR2806DownloadSentinel(t, "download output", text, plain, ciphertext)
	assertNoR2806DownloadSentinel(t, "download output label", text, label, "")
}

func assertNoR2806DownloadSentinel(t *testing.T, label, value, plain, ciphertext string) {
	t.Helper()
	if plain != "" && strings.Contains(value, plain) {
		t.Fatalf("%s 泄漏扩展明文 sentinel", label)
	}
	if ciphertext != "" && strings.Contains(value, ciphertext) {
		t.Fatalf("%s 泄漏扩展密文", label)
	}
}
