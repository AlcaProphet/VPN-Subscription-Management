package node

import (
	"reflect"
	"testing"
)

func TestNormalizeProtocolJSONVlessAliases(t *testing.T) {
	proto, err := GetProtocol("vless")
	if err != nil {
		t.Fatal(err)
	}
	in := map[string]any{
		"uuid":              "u",
		"network":           "ws",
		"ws-path":           "/legacy",
		"ws-host":           "cdn.example.com",
		"ws-headers":        map[string]any{"X-Test": "yes"},
		"grpc-service-name": "svc",
		"public-key":        "pk",
		"short-id":          "abcd",
	}
	out, err := NormalizeProtocolJSON(proto, in)
	if err != nil {
		t.Fatal(err)
	}
	ws := out["ws-opts"].(map[string]any)
	if ws["path"] != "/legacy" {
		t.Fatalf("ws-path 未映射到 ws-opts.path: %#v", ws)
	}
	headers := ws["headers"].(map[string]any)
	if headers["Host"] != "cdn.example.com" || headers["X-Test"] != "yes" {
		t.Fatalf("ws-host/ws-headers 未映射到 ws-opts.headers: %#v", headers)
	}
	for _, key := range []string{"ws-path", "ws-host", "ws-headers", "grpc-service-name", "public-key", "short-id"} {
		if _, exists := out[key]; exists {
			t.Fatalf("旧顶层别名未清理: %s", key)
		}
	}
	grpc := out["grpc-opts"].(map[string]any)
	if grpc["grpc-service-name"] != "svc" {
		t.Fatalf("grpc-service-name 未映射: %#v", grpc)
	}
	reality := out["reality-opts"].(map[string]any)
	if reality["public-key"] != "pk" || reality["short-id"] != "abcd" {
		t.Fatalf("reality 旧别名未映射: %#v", reality)
	}
}

func TestNormalizeProtocolJSONTrojanInnerSS(t *testing.T) {
	proto, err := GetProtocol("trojan")
	if err != nil {
		t.Fatal(err)
	}
	out, err := NormalizeProtocolJSON(proto, map[string]any{
		"cipher": "aes-256-gcm",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := out["cipher"]; exists {
		t.Fatalf("Trojan 顶层 cipher 未清除")
	}
	ss := out["ss-opts"].(map[string]any)
	if ss["method"] != "aes-256-gcm" {
		t.Fatalf("Trojan cipher 未映射到 ss-opts.method: %#v", ss)
	}
}

func TestInitCurrentStateMinimalForProtocolWithoutSecurity(t *testing.T) {
	proto, err := GetProtocol("ss")
	if err != nil {
		t.Fatal(err)
	}
	state := InitCurrentState(proto, map[string]any{})
	if state.Network != "" || state.Security != "" {
		t.Fatalf("无 network/security 的协议不应生成这些字段: %#v", state)
	}
}

func TestInitCurrentStateKeepsKnownFirstBatchDimensions(t *testing.T) {
	proto, err := GetProtocol("vless")
	if err != nil {
		t.Fatal(err)
	}
	state := InitCurrentState(proto, map[string]any{"network": "ws", "security": "tls", "uuid": "u"})
	if !reflect.DeepEqual(state.Network, "ws") || state.Security != "tls" {
		t.Fatalf("首批协议应保留已存在维度: %#v", state)
	}
}
