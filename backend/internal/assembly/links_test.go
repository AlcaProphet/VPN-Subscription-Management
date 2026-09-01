package assembly

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestGenericVmessTLSAndSNI(t *testing.T) {
	nd := &nodeData{
		Protocol: "vmess", Host: "example.com", Port: 443, RenderName: "节点",
		ProtocolJSON: map[string]any{
			"uuid":       "11111111-2222-3333-4444-555555555555",
			"tls":        true,
			"servername": "sni.example.com",
			"network":    "ws",
			"ws-opts":    map[string]any{"path": "/path", "headers": map[string]any{"Host": "cdn.example.com"}},
		},
	}
	link, err := genericLink(nd)
	if err != nil {
		t.Fatalf("genericLink 失败: %v", err)
	}
	payload := strings.TrimPrefix(link, "vmess://")
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("解析 vmess base64 失败: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("解析 vmess JSON 失败: %v", err)
	}
	if obj["tls"] != "tls" {
		t.Errorf("tls=true 应输出 \"tls\"，实际 %#v", obj["tls"])
	}
	if obj["sni"] != "sni.example.com" {
		t.Errorf("servername 非空时应输出 sni，实际 %#v", obj["sni"])
	}
	if obj["path"] != "/path" || obj["host"] != "cdn.example.com" {
		t.Errorf("ws-opts 未映射到 vmess 链接: %#v", obj)
	}
}

func TestGenericVmessTLSFalse(t *testing.T) {
	nd := &nodeData{
		Protocol: "vmess", Host: "example.com", Port: 443, RenderName: "节点",
		ProtocolJSON: map[string]any{"uuid": "u", "tls": false},
	}
	link, err := genericLink(nd)
	if err != nil {
		t.Fatalf("genericLink 失败: %v", err)
	}
	payload := strings.TrimPrefix(link, "vmess://")
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("解析 vmess base64 失败: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("解析 vmess JSON 失败: %v", err)
	}
	if obj["tls"] != "" {
		t.Errorf("tls=false 应输出空串，实际 %#v", obj["tls"])
	}
}

func TestClashProxyKeyOrder(t *testing.T) {
	nd := &nodeData{
		RenderName: "节点A", Protocol: "vmess", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{
			"uuid": "u", "tls": true, "network": "ws", "alpn": "h2,http/1.1",
		},
	}
	p := new(Service).clashProxy(nd)
	got := p.Keys()
	want := []string{"name", "type", "server", "port", "alpn", "network", "tls", "uuid"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("proxies 条目键序不稳定：got %v, want %v", got, want)
	}
}

func TestNormalizeClashListFields(t *testing.T) {
	got := normalizeClashFields("wireguard", map[string]any{
		"allowed-ips": "0.0.0.0/0, ::/0", "reserved": "1, 2,3", "peers": []any{map[string]any{"server": "peer"}},
	})
	if !reflect.DeepEqual(got["allowed-ips"], []string{"0.0.0.0/0", "::/0"}) {
		t.Fatalf("text-list 未归一化: %#v", got["allowed-ips"])
	}
	if !reflect.DeepEqual(got["reserved"], []int{1, 2, 3}) {
		t.Fatalf("int-list 未归一化: %#v", got["reserved"])
	}
}
