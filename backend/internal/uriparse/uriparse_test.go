package uriparse

import (
	"encoding/base64"
	"testing"
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
