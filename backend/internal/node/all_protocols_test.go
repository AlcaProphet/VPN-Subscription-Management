package node

import (
	"context"
	"testing"
)

func minimalProtocolParams(proto Protocol) map[string]any {
	out := map[string]any{}
	for _, field := range proto.FormSchema {
		if !field.Required {
			continue
		}
		switch field.Type {
		case "bool":
			out[field.Name] = true
		case "number":
			out[field.Name] = 1
		case "select":
			if len(field.OptionItems) > 0 {
				out[field.Name] = field.OptionItems[0].Value
			} else if len(field.Options) > 0 {
				out[field.Name] = field.Options[0]
			}
		case "object":
			out[field.Name] = map[string]any{}
		default:
			value := "test-value"
			if field.Name == "cipher" && proto.Protocol == "ss" {
				value = "aes-256-gcm"
			}
			out[field.Name] = value
		}
	}
	return out
}

func TestAllManualProtocolsUnifiedSaveContract(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	for _, proto := range ManualProtocols() {
		proto := proto
		t.Run(proto.Protocol, func(t *testing.T) {
			params := minimalProtocolParams(proto)
			name := "m-" + proto.Protocol + "-1"
			created, err := svc.CreateManual(ctx, CreateManualInput{
				Name: name, Protocol: proto.Protocol, Host: "example.com", Port: 443,
				ProtocolJSON: params,
			})
			if err != nil {
				t.Fatalf("创建 %s 失败: %v", proto.Protocol, err)
			}
			if created.EditRevision != 1 || created.CurrentState.Network == "" && proto.Protocol == "vless" {
				t.Fatalf("创建后修订/当前状态异常: %+v", created)
			}
			updated, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
				Name: name, Protocol: proto.Protocol, Host: "example.com", Port: 443,
				ProtocolJSON: params, BaseRevision: created.EditRevision,
			})
			if err != nil {
				t.Fatalf("更新 %s 失败: %v", proto.Protocol, err)
			}
			if updated.EditRevision != 2 {
				t.Fatalf("更新后修订应为 2，实际 %d", updated.EditRevision)
			}
			got, err := svc.Get(ctx, created.ID)
			if err != nil {
				t.Fatalf("读取 %s 失败: %v", proto.Protocol, err)
			}
			if got.EditRevision != 2 || got.StateFormatVersion != 1 {
				t.Fatalf("读取后修订/格式异常: %+v", got)
			}
		})
	}
}

func TestManualProtocolCount(t *testing.T) {
	protocols := ManualProtocols()
	if len(protocols) != 19 {
		t.Fatalf("manual 协议数应为 19，实际 %d: %v", len(protocols), protocols)
	}
	for i := range protocols {
		if protocols[i].Protocol == "ssr" {
			t.Fatal("ssr 不应在 manual 协议清单中")
		}
	}
}
