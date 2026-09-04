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

func TestNormalizeProtocolJSONSSPluginOpts(t *testing.T) {
	proto, err := GetProtocol("ss")
	if err != nil {
		t.Fatal(err)
	}
	out, err := NormalizeProtocolJSON(proto, map[string]any{
		"cipher": "aes-256-gcm", "password": "pw", "plugin": "obfs",
		"plugin-opts": map[string]any{"mode": "http", "host": "cdn.example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	obfs, ok := out["obfs-opts"].(map[string]any)
	if !ok || obfs["mode"] != "http" || obfs["host"] != "cdn.example.com" {
		t.Fatalf("SS plugin-opts 未映射到 obfs-opts: %#v", out)
	}
	if _, exists := out["plugin-opts"]; exists {
		t.Fatalf("旧 plugin-opts 未清理")
	}
}

func TestNormalizeProtocolJSONSSPluginOptsNewObjectWinsAndIsIdempotent(t *testing.T) {
	proto, err := GetProtocol("ss")
	if err != nil {
		t.Fatal(err)
	}
	in := map[string]any{
		"cipher": "aes-256-gcm", "password": "pw", "plugin": "v2ray-plugin",
		"plugin-opts": map[string]any{
			"mode": "quic", "host": "legacy.example.com", "path": "/legacy",
			"headers": map[string]any{"X-Legacy": "yes", "X-Shared": "legacy"},
		},
		"v2ray-plugin-opts": map[string]any{
			"mode": "websocket", "host": "current.example.com",
			"headers": map[string]any{"X-Shared": "current", "X-New": "yes"},
		},
	}
	out, err := NormalizeProtocolJSON(proto, in)
	if err != nil {
		t.Fatal(err)
	}
	opts, ok := out["v2ray-plugin-opts"].(map[string]any)
	if !ok {
		t.Fatalf("v2ray-plugin 规范对象缺失: %#v", out)
	}
	if opts["mode"] != "websocket" || opts["host"] != "current.example.com" || opts["path"] != "/legacy" {
		t.Fatalf("新对象应优先且旧对象只补缺: %#v", opts)
	}
	headers, ok := opts["headers"].(map[string]any)
	if !ok || headers["X-Legacy"] != "yes" || headers["X-Shared"] != "current" || headers["X-New"] != "yes" {
		t.Fatalf("嵌套对象未按新值优先递归补缺: %#v", headers)
	}
	if _, exists := out["plugin-opts"]; exists {
		t.Fatal("已知插件的旧 plugin-opts 应在补缺后清理")
	}
	twice, err := NormalizeProtocolJSON(proto, out)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out, twice) {
		t.Fatalf("SS 规范化应幂等:\n一次=%#v\n两次=%#v", out, twice)
	}
}

func TestNormalizeProtocolJSONSSUnknownPluginOptsPreservedAndIsIdempotent(t *testing.T) {
	proto, err := GetProtocol("ss")
	if err != nil {
		t.Fatal(err)
	}
	in := map[string]any{
		"cipher": "aes-256-gcm", "password": "pw", "plugin": "custom-plugin",
		"plugin-opts": map[string]any{
			"mode": "custom", "host": "cdn.example.com", "flag": "",
			"password": "ordinary-password", "token": "ordinary-token", "secret": "ordinary-secret",
		},
	}
	out, err := NormalizeProtocolJSON(proto, in)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out["plugin-opts"], in["plugin-opts"]) {
		t.Fatalf("未知插件参数不应被删除或改写: %#v", out)
	}
	twice, err := NormalizeProtocolJSON(proto, out)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out, twice) {
		t.Fatalf("未知插件规范化应幂等:\n一次=%#v\n两次=%#v", out, twice)
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
