package links

import (
	"net/url"
	"strings"
	"testing"
)

func TestVlessTransportAndRealityQuery(t *testing.T) {
	params := map[string]any{
		"uuid": "11111111-2222-3333-4444-555555555555", "network": "ws", "servername": "sni.example.com",
		"client-fingerprint": "chrome", "reality-opts": map[string]any{"public-key": "public", "short-id": "abcd"},
		"ws-opts": map[string]any{"path": "/socket", "headers": map[string]any{"Host": "cdn.example.com"}},
	}
	link, err := Render("vless", "节点", "example.com", 443, params, true)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	q := parsed.Query()
	for key, want := range map[string]string{"security": "reality", "pbk": "public", "sid": "abcd", "type": "ws", "path": "/socket", "host": "cdn.example.com"} {
		if q.Get(key) != want {
			t.Errorf("%s: want %q got %q", key, want, q.Get(key))
		}
	}
}

func TestVlessGRPCAndSRCommonQuery(t *testing.T) {
	params := map[string]any{"uuid": "u", "network": "grpc", "tfo": true, "grpc-opts": map[string]any{"grpc-service-name": "svc"}}
	for _, generic := range []bool{true, false} {
		link, err := Render("vless", "节点", "example.com", 443, params, generic)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := url.Parse(link)
		if err != nil {
			t.Fatal(err)
		}
		if parsed.Query().Get("serviceName") != "svc" {
			t.Fatalf("gRPC serviceName 缺失: %s", link)
		}
		if !generic && parsed.Query().Get("tfo") != "1" {
			t.Fatalf("SR tfo 缺失: %s", link)
		}
	}
}

func TestRealityDoesNotReadLegacyTopLevelKeys(t *testing.T) {
	if got := realityOpts(map[string]any{"public-key": "legacy", "short-id": "old"}); got != nil {
		t.Fatalf("不应读取旧顶层 REALITY 字段: %#v", got)
	}
	link, err := Render("ss", "节点", "example.com", 443, map[string]any{
		"cipher": "aes-256-gcm", "password": "p", "plugin": "shadow-tls", "plugin-opts": map[string]any{"host": "cdn.example.com"},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(link, "plugin=") || !strings.Contains(link, "shadow-tls") {
		t.Fatalf("SS 插件参数缺失: %s", link)
	}
}
