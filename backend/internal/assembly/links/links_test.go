package links

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"vpn-sub/internal/ssplugin"
	"vpn-sub/internal/uriparse"
)

func TestShadowrocketTLSMappingsAndRoundTrip(t *testing.T) {
	cases := []struct {
		name       string
		protocol   string
		params     map[string]any
		wantQuery  map[string]string
		absentKeys []string
		wantParams map[string]any
	}{
		{
			name: "vmess tls", protocol: "vmess",
			params: map[string]any{
				"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp", "tls": true,
				"servername": "sni.example.com", "alpn": []string{"h2", "http/1.1"},
				"client-fingerprint": "chrome", "skip-cert-verify": true,
			},
			wantQuery: map[string]string{
				"tls": "1", "peer": "sni.example.com", "alpn": "h2,http/1.1", "fp": "chrome", "allowInsecure": "1",
			},
			wantParams: map[string]any{
				"tls": true, "servername": "sni.example.com", "alpn": []string{"h2", "http/1.1"},
				"client-fingerprint": "chrome", "skip-cert-verify": true,
			},
		},
		{
			name: "vless tls", protocol: "vless",
			params: map[string]any{
				"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp", "tls": true,
				"servername": "sni.example.com", "alpn": []any{"h2", "http/1.1"},
				"client-fingerprint": "chrome", "flow": "xtls-rprx-vision", "skip-cert-verify": true,
			},
			wantQuery: map[string]string{
				"tls": "1", "peer": "sni.example.com", "alpn": "h2,http/1.1", "fp": "chrome",
				"flow": "xtls-rprx-vision", "allowInsecure": "1",
			},
			absentKeys: []string{"xtls", "pbk", "sid"},
			wantParams: map[string]any{
				"tls": true, "servername": "sni.example.com", "alpn": []string{"h2", "http/1.1"},
				"client-fingerprint": "chrome", "flow": "xtls-rprx-vision", "skip-cert-verify": true,
			},
		},
		{
			name: "vless reality", protocol: "vless",
			params: map[string]any{
				"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp", "tls": true,
				"servername": "sni.example.com", "alpn": []string{"h2"}, "client-fingerprint": "chrome",
				"flow": "xtls-rprx-vision", "skip-cert-verify": true,
				"reality-opts": map[string]any{"public-key": "public", "short-id": "abcd"},
			},
			wantQuery: map[string]string{
				"tls": "1", "xtls": "2", "peer": "sni.example.com", "alpn": "h2", "fp": "chrome",
				"flow": "xtls-rprx-vision", "pbk": "public", "sid": "abcd",
			},
			absentKeys: []string{"allowInsecure"},
			wantParams: map[string]any{
				"tls": true, "servername": "sni.example.com", "alpn": []string{"h2"},
				"client-fingerprint": "chrome", "flow": "xtls-rprx-vision",
				"reality-opts": map[string]any{"public-key": "public", "short-id": "abcd"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before, err := json.Marshal(tc.params)
			if err != nil {
				t.Fatal(err)
			}
			link, err := Render(tc.protocol, "节点", "example.com", 443, tc.params, false)
			if err != nil {
				t.Fatal(err)
			}
			after, err := json.Marshal(tc.params)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(after, before) {
				t.Fatalf("渲染器修改了输入参数: before=%s after=%s", before, after)
			}
			parsedURL, err := url.Parse(link)
			if err != nil {
				t.Fatal(err)
			}
			for key, want := range tc.wantQuery {
				if got := parsedURL.Query().Get(key); got != want {
					t.Errorf("%s: want %q got %q; link=%s", key, want, got, link)
				}
			}
			for _, key := range tc.absentKeys {
				if parsedURL.Query().Has(key) {
					t.Errorf("不应输出 %s: %s", key, link)
				}
			}
			parsed, err := uriparse.Parse(link)
			if err != nil {
				t.Fatalf("生成链接无法回读: %v", err)
			}
			for key, want := range tc.wantParams {
				if got := parsed.Params[key]; !reflect.DeepEqual(got, want) {
					t.Errorf("回读 %s: want %#v got %#v", key, want, got)
				}
			}
		})
	}
}

func TestShadowrocketTLSFieldsRequireActiveTLS(t *testing.T) {
	for _, protocol := range []string{"vmess", "vless"} {
		t.Run(protocol, func(t *testing.T) {
			link, err := Render(protocol, "节点", "example.com", 443, map[string]any{
				"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp", "tls": false,
				"servername": "stale.example.com", "alpn": []string{"h2"}, "client-fingerprint": "chrome",
				"flow": "xtls-rprx-vision", "skip-cert-verify": true,
				"reality-opts": map[string]any{"public-key": "stale", "short-id": "stale"},
			}, false)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := url.Parse(link)
			if err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"tls", "peer", "alpn", "fp", "flow", "allowInsecure", "xtls", "pbk", "sid"} {
				if parsed.Query().Has(key) {
					t.Errorf("TLS 关闭时不应输出残留参数 %s: %s", key, link)
				}
			}
		})
	}
}

func TestGenericTLSOutputBoundaries(t *testing.T) {
	vless, err := Render("vless", "节点", "example.com", 443, map[string]any{
		"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp", "tls": true,
		"servername": "sni.example.com", "alpn": []string{"h2"}, "client-fingerprint": "chrome",
		"flow": "xtls-rprx-vision", "skip-cert-verify": true,
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	parsedVLESS, err := url.Parse(vless)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"security": "tls", "sni": "sni.example.com", "alpn": "h2", "fp": "chrome",
		"flow": "xtls-rprx-vision", "allowInsecure": "1",
	} {
		if got := parsedVLESS.Query().Get(key); got != want {
			t.Errorf("generic VLESS %s: want %q got %q", key, want, got)
		}
	}

	vmess, err := Render("vmess", "节点", "example.com", 443, map[string]any{
		"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp", "tls": true,
		"skip-cert-verify": true,
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(vmess, "vmess://"))
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if _, exists := payload["skip-cert-verify"]; exists {
		t.Fatalf("generic VMess 不应输出 skip-cert-verify: %#v", payload)
	}
}

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
	}, false)
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

func TestSSPluginTargetSplitAndSemanticRoundTrip(t *testing.T) {
	base := map[string]any{"cipher": "aes-256-gcm", "password": "password"}
	cases := []struct {
		name      string
		plugin    string
		storage   string
		opts      map[string]any
		generic   bool
		wantError bool
	}{
		{name: "sr obfs", plugin: "obfs", storage: "obfs-opts", opts: map[string]any{"mode": "http", "host": `cdn:443;edge=1\x`}},
		{name: "generic obfs", plugin: "obfs", storage: "obfs-opts", opts: map[string]any{"mode": "tls", "host": "cdn.example.com"}, generic: true},
		{name: "sr v2ray", plugin: "v2ray-plugin", storage: "v2ray-plugin-opts", opts: map[string]any{"mode": "websocket", "host": "cdn.example.com", "path": "/ws;a=b", "tls": true}},
		{name: "generic v2ray", plugin: "v2ray-plugin", storage: "v2ray-plugin-opts", opts: map[string]any{"mode": "websocket", "path": "/ws", "tls": true}, generic: true},
		{name: "sr shadow tls", plugin: "shadow-tls", storage: "shadow-tls-opts", opts: map[string]any{"host": "cdn.example.com", "version": float64(3), "alpn": []any{"h2", "http/1.1"}}},
		{name: "generic shadow tls blocked", plugin: "shadow-tls", storage: "shadow-tls-opts", opts: map[string]any{"host": "cdn.example.com"}, generic: true, wantError: true},
		{name: "sr restls", plugin: "restls", storage: "restls-opts", opts: map[string]any{"host": "cdn.example.com", "password": "secret", "skip-cert-verify": true}},
		{name: "generic restls blocked", plugin: "restls", storage: "restls-opts", opts: map[string]any{"host": "cdn.example.com"}, generic: true, wantError: true},
		{name: "sr unknown", plugin: "custom-plugin", storage: "plugin-opts", opts: map[string]any{"flag": "", "token": `a:;=\`}},
		{name: "generic unknown blocked", plugin: "custom-plugin", storage: "plugin-opts", opts: map[string]any{"flag": ""}, generic: true, wantError: true},
		{name: "v2ray extra field blocked", plugin: "v2ray-plugin", storage: "v2ray-plugin-opts", opts: map[string]any{"headers": map[string]any{"Host": "cdn.example.com"}}, wantError: true},
		{name: "unknown non string blocked", plugin: "custom-plugin", storage: "plugin-opts", opts: map[string]any{"enabled": true}, wantError: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := make(map[string]any, len(base)+2)
			for key, value := range base {
				params[key] = value
			}
			params["plugin"] = tc.plugin
			params[tc.storage] = tc.opts
			link, err := Render("ss", "节点", "example.com", 8388, params, tc.generic)
			if tc.wantError {
				if err == nil {
					t.Fatalf("目标应拒绝无法无损表达的插件参数: %s", link)
				}
				return
			}
			if err != nil {
				t.Fatalf("目标插件 URI 生成失败: %v", err)
			}
			parsed, err := url.Parse(link)
			if err != nil {
				t.Fatal(err)
			}
			pluginName, pluginOpts, err := ssplugin.ParsePluginString(parsed.Query().Get("plugin"))
			if err != nil {
				t.Fatalf("生成的插件串无法回读: %v", err)
			}
			if pluginName == "" || pluginOpts == nil {
				t.Fatalf("生成的插件语义为空: name=%q opts=%#v", pluginName, pluginOpts)
			}
		})
	}
}

func TestPluginOptsReadsUnknownStringMap(t *testing.T) {
	want := map[string]any{"flag": "", "key": "value"}
	if got := PluginOpts(map[string]any{"plugin-opts": want}, "custom-plugin"); !reflect.DeepEqual(got, want) {
		t.Fatalf("未知插件参数读取异常: got=%#v want=%#v", got, want)
	}
}

func TestSSPluginURLAndSIP002EscapingCompose(t *testing.T) {
	wantName := `custom:plugin;=\`
	wantOpts := map[string]string{"flag": "", `key;=\`: `值:;=\ %2F`}
	params := map[string]any{
		"cipher": "aes-256-gcm", "password": "password", "plugin": wantName,
		"plugin-opts": map[string]any{"flag": "", `key;=\`: `值:;=\ %2F`},
	}
	link, err := Render("ss", "节点", "example.com", 8388, params, false)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	name, opts, err := ssplugin.ParsePluginString(parsed.Query().Get("plugin"))
	if err != nil {
		t.Fatal(err)
	}
	if name != wantName || !reflect.DeepEqual(opts, wantOpts) {
		t.Fatalf("URL query 与 SIP002 组合往返异常: name=%q opts=%#v link=%s", name, opts, link)
	}
}
