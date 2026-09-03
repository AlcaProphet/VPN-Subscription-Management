package links

import (
	"encoding/base64"
	"encoding/json"
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
		"cipher": "aes-256-gcm", "password": "p", "plugin": "shadow-tls", "shadow-tls-opts": map[string]any{"host": "cdn.example.com"},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(link, "plugin=") || !strings.Contains(link, "shadow-tls") {
		t.Fatalf("SS 插件参数缺失: %s", link)
	}
}

func TestBuild18FixedURIExamples(t *testing.T) {
	t.Run("VLESS REALITY", func(t *testing.T) {
		link, err := Render("vless", "节点", "example.com", 443, map[string]any{
			"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp",
			"tls": true, "servername": "www.example.com",
			"reality-opts": map[string]any{"public-key": "public", "short-id": "01234567"},
		}, true)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := url.Parse(link)
		if err != nil {
			t.Fatal(err)
		}
		if parsed.Scheme != "vless" || parsed.Query().Get("security") != "reality" || parsed.Query().Get("pbk") != "public" || parsed.Query().Get("sid") != "01234567" {
			t.Fatalf("固定 VLESS REALITY URI 观察异常: %s", link)
		}
	})

	t.Run("VMess cipher", func(t *testing.T) {
		link, err := Render("vmess", "节点", "example.com", 443, map[string]any{
			"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp", "cipher": "chacha20-poly1305",
		}, true)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(link, "vmess://"))
		if err != nil {
			t.Fatal(err)
		}
		var payload map[string]any
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatal(err)
		}
		if payload["scy"] != "chacha20-poly1305" {
			t.Fatalf("固定 VMess URI 未保留输入算法供诊断: %#v", payload)
		}
	})

	t.Run("Trojan WS", func(t *testing.T) {
		link, err := Render("trojan", "节点", "example.com", 443, map[string]any{
			"password": "password", "network": "ws", "ws-opts": map[string]any{"path": "/ws"},
		}, false)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := url.Parse(link)
		if err != nil {
			t.Fatal(err)
		}
		if parsed.Scheme != "trojan" || parsed.Query().Get("type") != "" || parsed.Query().Get("path") != "" {
			t.Fatalf("固定 Trojan WS URI 观察异常，应由上层诊断参数丢失: %s", link)
		}
	})

	t.Run("SS obfs", func(t *testing.T) {
		link, err := Render("ss", "节点", "example.com", 8388, map[string]any{
			"cipher": "aes-256-gcm", "password": "password", "plugin": "obfs",
			"obfs-opts": map[string]any{"mode": "http", "host": "cdn.example.com"},
		}, false)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(link, "plugin=obfs-local%3B") || !strings.Contains(link, "obfs%3Dhttp") || !strings.Contains(link, "obfs-host") {
			t.Fatalf("SS obfs 未按目标映射为 obfs-local: %s", link)
		}
	})
}
