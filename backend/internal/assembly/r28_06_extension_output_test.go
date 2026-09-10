package assembly

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"vpn-sub/internal/log"
	nodesvc "vpn-sub/internal/node"
)

// R28-06A：扩展负载和密文不得出现在 Clash/SR/generic 的 preview/generate 结果中。
func TestExtensionPayloadAndCiphertextNeverEnterClientOutputs(t *testing.T) {
	svc, st, cfg := newTestService(t)
	ctx := context.Background()
	const plain = "r28-06-extension-plain-sentinel"

	nodeService := nodesvc.NewService(st, cfg, log.New("error", "console"))
	created, err := nodeService.CreateManual(ctx, nodesvc.CreateManualInput{
		Name: "扩展产物边界", Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp"},
		Extensions: []nodesvc.ExtensionInput{{
			ID: "ext-output", Scope: "node", Targets: []string{"clash-yaml", "sr-subs", "generic-subs"}, Payload: plain,
		}},
	})
	if err != nil {
		t.Fatalf("创建带扩展节点失败: %v", err)
	}

	var extensionsRaw string
	if err := st.DB().QueryRowContext(ctx, `SELECT extensions_json FROM nodes WHERE id = ?`, created.ID).Scan(&extensionsRaw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(extensionsRaw, plain) || !strings.Contains(extensionsRaw, "enc:ext:v1:") {
		t.Fatalf("测试前提失败：扩展密文/明文存储异常: %s", extensionsRaw)
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
		t.Fatalf("测试前提失败：扩展记录缺失: %s", extensionsRaw)
	}
	ciphertext := envelope.Entries[0].PayloadEnc

	inputs := []GenerateInput{
		{
			TargetSyntax: ClashYAML, PlatformID: insertPlatform(t, st, "yaml"),
			NodeNames:            []string{created.Name},
			OverseasMembers:      []string{created.Name},
			FallbackGroupMembers: []string{nodesvc.ForceDirect, nodesvc.ForceOverseas},
		},
		{
			TargetSyntax: SrSubs, PlatformID: insertPlatform(t, st, "subs"),
			NodeNames: []string{created.Name},
		},
		{
			TargetSyntax: GenericSubs, PlatformID: insertPlatform(t, st, "generic-subs"),
			NodeNames: []string{created.Name},
		},
	}
	for _, in := range inputs {
		t.Run(string(in.TargetSyntax)+"-preview", func(t *testing.T) {
			res, err := svc.Preview(ctx, in)
			if err != nil {
				t.Fatalf("preview 失败: %v", err)
			}
			assertNoExtensionSentinel(t, string(in.TargetSyntax)+" preview content", string(res.Content), plain, ciphertext)
			assertNoExtensionSentinel(t, string(in.TargetSyntax)+" preview result", mustMarshal(t, res), plain, ciphertext)
		})
		t.Run(string(in.TargetSyntax)+"-generate", func(t *testing.T) {
			res, err := svc.Render(ctx, in)
			if err != nil {
				t.Fatalf("generate 失败: %v", err)
			}
			assertNoExtensionSentinel(t, string(in.TargetSyntax)+" generate content", string(res.Content), plain, ciphertext)
			assertNoExtensionSentinel(t, string(in.TargetSyntax)+" generate result", mustMarshal(t, res), plain, ciphertext)
		})
	}

	detail, err := nodeService.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertNoExtensionSentinel(t, "node detail", mustMarshal(t, detail), plain, ciphertext)
}

func assertNoExtensionSentinel(t *testing.T, label, value, plain, ciphertext string) {
	t.Helper()
	if strings.Contains(value, plain) {
		t.Fatalf("%s 泄漏扩展明文 sentinel", label)
	}
	if strings.Contains(value, ciphertext) {
		t.Fatalf("%s 泄漏扩展密文", label)
	}
}

func mustMarshal(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
