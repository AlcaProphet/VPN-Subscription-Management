package assembly

import (
	"context"
	"testing"

	"vpn-sub/internal/node"
)

func TestClashAdapterRegistryLegacyPendingObservable(t *testing.T) {
	svc := &Service{}
	res, err := svc.CheckNodeTarget(context.Background(), "clash-yaml", "http", "legacy-node", "example.com", 443, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, diagnostic := range res.Diagnostics {
		if diagnostic.Code == "legacy_adapter_pending" && diagnostic.Severity == "info" {
			found = true
		}
	}
	if !found {
		t.Fatalf("legacy adapter 必须返回 legacy_adapter_pending 证据: %+v", res.Diagnostics)
	}
}

func TestCheckAndFormalAssemblyUseSameRegisteredAdapterDraft(t *testing.T) {
	orig, had := clashProtocolAdapters["http"]
	var captured ClashNodeDraft
	clashProtocolAdapters["http"] = func(draft ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
		captured = draft
		return map[string]any{"custom": "value"}, nil, nil
	}
	defer func() {
		if had {
			clashProtocolAdapters["http"] = orig
		} else {
			delete(clashProtocolAdapters, "http")
		}
	}()

	svc := &Service{}
	_, err := svc.CheckNodeTargetDraft(context.Background(), node.CheckTargetDraft{
		Target: "clash-yaml", Protocol: "http", RenderName: "adapter-node",
		Host: "example.com", Port: 443, Params: map[string]any{}, NodeID: 42, Persisted: true,
		State: node.CurrentState{Security: "tls"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if captured.NodeID != 42 || !captured.Persisted || captured.State.Security != "tls" {
		t.Fatalf("check 路径未注入完整生命周期状态: %+v", captured)
	}

	nd := &nodeData{NodeID: 42, Protocol: "http", RenderName: "adapter-node", Host: "example.com", Port: 443, CurrentState: node.CurrentState{Security: "tls"}}
	p, _, err := svc.buildClashProxy(nd, true)
	if err != nil {
		t.Fatal(err)
	}
	if value, ok := p.Get("custom"); !ok || value != "value" {
		t.Fatalf("正式装配未使用同一 adapter: %+v", p)
	}
}

func TestLegacyAdapterPendingCountTracksRegistry(t *testing.T) {
	before := legacyAdapterPendingCount()
	orig, had := clashProtocolAdapters["http"]
	clashProtocolAdapters["http"] = func(ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error) {
		return map[string]any{}, nil, nil
	}
	defer func() {
		if had {
			clashProtocolAdapters["http"] = orig
		} else {
			delete(clashProtocolAdapters, "http")
		}
	}()
	after := legacyAdapterPendingCount()
	if before <= 0 || after != before-1 {
		t.Fatalf("legacy adapter 计数未随注册下降: before=%d after=%d", before, after)
	}
}

func TestOrderedMapFromClashFieldsHonorsHiddenEndpointPolicy(t *testing.T) {
	p := orderedMapFromClashFields("node", "custom", "example.com", 443, map[string]any{
		"name": "ignored", "server": "ignored", "port": 1, "custom": "value",
	}, node.EndpointPolicy{
		HostMode: "hidden", PortMode: "hidden", EmitHost: false, EmitPort: false,
	})
	if _, ok := p.Get("server"); ok {
		t.Fatal("hidden host 不应输出 server")
	}
	if _, ok := p.Get("port"); ok {
		t.Fatal("hidden port 不应输出 port")
	}
	if value, ok := p.Get("custom"); !ok || value != "value" {
		t.Fatalf("adapter 普通字段丢失: %+v", value)
	}
}
