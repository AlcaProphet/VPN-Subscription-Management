package uriparse

import (
	"encoding/base64"
	"net/url"
	"reflect"
	"testing"

	"vpn-sub/internal/ssplugin"
)

func TestParseSSAndVLESS(t *testing.T) {
	ss := "ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ@example.com:8388#MySS"
	r, err := Parse(ss)
	if err != nil {
		t.Fatalf("ss 解析失败: %v", err)
	}
	if r.Protocol != "ss" || r.Host != "example.com" || r.Port != 8388 || r.Name != "MySS" {
		t.Fatalf("ss 结果异常: %+v", r)
	}
	if r.Params["cipher"] != "aes-256-gcm" || r.Params["password"] != "password" {
		t.Fatalf("ss 参数异常: %+v", r.Params)
	}

	vl := "vless://11111111-2222-3333-4444-555555555555@example.com:443?type=ws&security=tls&sni=cdn.example.com&path=%2Fws#VLESS"
	r, err = Parse(vl)
	if err != nil {
		t.Fatalf("vless 解析失败: %v", err)
	}
	if r.Protocol != "vless" || r.Host != "example.com" || r.Port != 443 || r.Name != "VLESS" {
		t.Fatalf("vless 结果异常: %+v", r)
	}
	if r.Params["network"] != "ws" || r.Params["servername"] != "cdn.example.com" || r.Params["tls"] != true {
		t.Fatalf("vless 参数异常: %+v", r.Params)
	}
	if _, ok := r.Params["ws-opts"]; !ok {
		t.Fatalf("vless ws-opts 缺失: %+v", r.Params)
	}
}

func TestParseSSPluginToSplitOpts(t *testing.T) {
	uri := "ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ@example.com:8388?plugin=obfs-local%3Bobfs%3Dhttp%3Bobfs-host%3Dcdn.example.com#SS"
	r, err := Parse(uri)
	if err != nil {
		t.Fatalf("ss 插件解析失败: %v", err)
	}
	if r.Params["plugin"] != "obfs" {
		t.Fatalf("ss 插件名未归一化: %#v", r.Params)
	}
	obfs, ok := r.Params["obfs-opts"].(map[string]any)
	if !ok || obfs["mode"] != "http" || obfs["host"] != "cdn.example.com" {
		t.Fatalf("ss 插件参数未拆到 obfs-opts: %#v", r.Params)
	}
	if _, exists := r.Params["plugin-opts"]; exists {
		t.Fatalf("旧 plugin-opts 不应再直接生成")
	}
}

func TestParseSSPluginPreservesEscapedUnknownOptions(t *testing.T) {
	rawPlugin, err := ssplugin.SerializePluginString(`custom:plugin`, map[string]string{
		"flag":   "",
		`key;=\`: `值:;=\`,
	})
	if err != nil {
		t.Fatal(err)
	}
	uri := "ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ@example.com:8388?plugin=" + url.QueryEscape(rawPlugin) + "#SS"
	r, err := Parse(uri)
	if err != nil {
		t.Fatalf("未知 SS 插件解析失败: %v", err)
	}
	if r.Params["plugin"] != `custom:plugin` {
		t.Fatalf("未知插件名未保留: %#v", r.Params)
	}
	want := map[string]any{"flag": "", `key;=\`: `值:;=\`}
	if !reflect.DeepEqual(r.Params["plugin-opts"], want) {
		t.Fatalf("未知插件参数未无损保留: got=%#v want=%#v", r.Params["plugin-opts"], want)
	}
}

func TestParseSSPluginRejectsMalformedOptions(t *testing.T) {
	for _, rawPlugin := range []string{`plugin;key=one;key=two`, `plugin;key=value\`, `plugin;=value`} {
		t.Run(rawPlugin, func(t *testing.T) {
			uri := "ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ@example.com:8388?plugin=" + url.QueryEscape(rawPlugin)
			if _, err := Parse(uri); err == nil {
				t.Fatalf("应拒绝坏 SIP002 插件参数 %q", rawPlugin)
			}
		})
	}
}

func TestParseSSKnownPluginOptionTypes(t *testing.T) {
	cases := []struct {
		name       string
		rawPlugin  string
		plugin     string
		storageKey string
		want       map[string]any
	}{
		{name: "obfs alias", rawPlugin: `simple-obfs;obfs=http;obfs-host=cdn.example.com`, plugin: "obfs", storageKey: "obfs-opts", want: map[string]any{"mode": "http", "host": "cdn.example.com"}},
		{name: "v2ray bool", rawPlugin: `v2ray-plugin;mode=websocket;path=/ws;tls`, plugin: "v2ray-plugin", storageKey: "v2ray-plugin-opts", want: map[string]any{"mode": "websocket", "path": "/ws", "tls": true}},
		{name: "shadow tls typed", rawPlugin: `shadow-tls;host=cdn.example.com;version=3;alpn=h2,http/1.1;skip-cert-verify=false`, plugin: "shadow-tls", storageKey: "shadow-tls-opts", want: map[string]any{"host": "cdn.example.com", "version": float64(3), "alpn": []string{"h2", "http/1.1"}, "skip-cert-verify": false}},
		{name: "restls bool", rawPlugin: `restls;host=cdn.example.com;skip-cert-verify=true`, plugin: "restls", storageKey: "restls-opts", want: map[string]any{"host": "cdn.example.com", "skip-cert-verify": true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uri := "ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ@example.com:8388?plugin=" + url.QueryEscape(tc.rawPlugin)
			r, err := Parse(uri)
			if err != nil {
				t.Fatalf("已知插件导入失败: %v", err)
			}
			if r.Params["plugin"] != tc.plugin || !reflect.DeepEqual(r.Params[tc.storageKey], tc.want) {
				t.Fatalf("已知插件导入类型异常: %#v", r.Params)
			}
		})
	}
}

func TestParseSSKnownPluginRejectsInvalidTypedValue(t *testing.T) {
	for _, rawPlugin := range []string{
		`v2ray-plugin;tls=maybe`,
		`shadow-tls;version=three`,
		`restls;skip-cert-verify=maybe`,
	} {
		t.Run(rawPlugin, func(t *testing.T) {
			uri := "ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ@example.com:8388?plugin=" + url.QueryEscape(rawPlugin)
			if _, err := Parse(uri); err == nil {
				t.Fatalf("应拒绝无法恢复类型的已知插件参数 %q", rawPlugin)
			}
		})
	}
}

func TestParseVMessBothForms(t *testing.T) {
	v2json := `{"v":"2","ps":"VMess","add":"example.com","port":"443","id":"11111111-2222-3333-4444-555555555555","aid":"0","scy":"auto","net":"ws","host":"cdn.example.com","path":"/ws","tls":"tls"}`
	raw := base64.StdEncoding.EncodeToString([]byte(v2json))
	r, err := Parse("vmess://" + raw)
	if err != nil {
		t.Fatalf("vmess json 解析失败: %v", err)
	}
	if r.Protocol != "vmess" || r.Host != "example.com" || r.Port != 443 || r.Name != "VMess" {
		t.Fatalf("vmess json 结果异常: %+v", r)
	}
	if r.Params["uuid"] != "11111111-2222-3333-4444-555555555555" || r.Params["network"] != "ws" || r.Params["tls"] != true {
		t.Fatalf("vmess json 参数异常: %+v", r.Params)
	}

	sr := base64.StdEncoding.EncodeToString([]byte("auto:11111111-2222-3333-4444-555555555555@example.com:443"))
	r, err = Parse("vmess://" + sr + "?remarks=SRVMess&udp=1")
	if err != nil {
		t.Fatalf("vmess sr 解析失败: %v", err)
	}
	if r.Name != "SRVMess" || r.Params["uuid"] != "11111111-2222-3333-4444-555555555555" {
		t.Fatalf("vmess sr 结果异常: %+v", r)
	}
}

func TestParseMoreSchemes(t *testing.T) {
	cases := []struct {
		uri      string
		protocol string
		name     string
	}{
		{"trojan://password@example.com:443?sni=example.com#Trojan", "trojan", "Trojan"},
		{"hysteria2://password@example.com:443?insecure=1#Hy2", "hysteria2", "Hy2"},
		{"tuic://uuid:password@example.com:443#TUIC", "tuic", "TUIC"},
		{"http://user:pass@example.com:8080#HTTP", "http", "HTTP"},
		{"socks5://user:pass@example.com:1080#SOCKS", "socks5", "SOCKS"},
		{"wireguard://key@example.com:51820?public-key=pk&allowed-ips=0.0.0.0/0#WG", "wireguard", "WG"},
	}
	for _, c := range cases {
		r, err := Parse(c.uri)
		if err != nil {
			t.Fatalf("%s 解析失败: %v", c.protocol, err)
		}
		if r.Protocol != c.protocol || r.Name != c.name {
			t.Fatalf("%s 结果异常: %+v", c.protocol, r)
		}
	}
}

func TestParseVLESSShadowrocketAndBlockBase64(t *testing.T) {
	uuid := "11111111-2222-3333-4444-555555555555"
	b := base64.StdEncoding.EncodeToString([]byte(":" + uuid + "@example.com:443"))
	r, err := Parse("vless://" + b + "?type=ws&security=tls&sni=cdn#SR")
	if err != nil {
		t.Fatalf("vless SR 解析失败: %v", err)
	}
	if r.Name != "SR" || r.Params["uuid"] != uuid || r.Params["tls"] != true {
		t.Fatalf("vless SR 结果异常: %+v", r)
	}

	blockText := "ss://" + base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:pw")) + "@a.example.com:8388#A\n" +
		"ss://" + base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:pw2")) + "@b.example.com:8388#B"
	encoded := base64.StdEncoding.EncodeToString([]byte(blockText))
	results, skips := ParseBlock(encoded)
	if len(results) != 2 || len(skips) != 0 {
		t.Fatalf("整块 Base64 解析异常: results=%d skips=%d", len(results), len(skips))
	}
}

func TestParseBlockSkipsInvalid(t *testing.T) {
	block := "ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ@example.com:8388#ok\nnot-a-uri\n"
	results, skips := ParseBlock(block)
	if len(results) != 1 || len(skips) != 1 {
		t.Fatalf("ParseBlock 结果异常: results=%d skips=%d", len(results), len(skips))
	}
	if results[0].Name != "ok" || skips[0].Reason == "" {
		t.Fatalf("ParseBlock 内容异常: %+v %+v", results[0], skips[0])
	}
}
