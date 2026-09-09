package assembly

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	gyaml "github.com/goccy/go-yaml"
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

func TestClashSSPluginProjectionIsStructuredAndImmutable(t *testing.T) {
	cases := []struct {
		name   string
		params map[string]any
		want   map[string]any
	}{
		{
			name: "none",
			params: map[string]any{
				"cipher": "aes-128-gcm", "password": "secret", "plugin": "",
				"obfs-opts": map[string]any{"mode": "tls"},
			},
			want: map[string]any{"cipher": "aes-128-gcm", "password": "secret"},
		},
		{
			name: "obfs",
			params: map[string]any{
				"plugin": "obfs", "obfs-opts": map[string]any{"mode": "tls", "host": "cdn.example.com"},
				"restls-opts": map[string]any{"host": "inactive.example.com"},
			},
			want: map[string]any{"plugin": "obfs", "plugin-opts": map[string]any{"mode": "tls", "host": "cdn.example.com"}},
		},
		{
			name: "v2ray-plugin-default-and-nested-options",
			params: map[string]any{
				"plugin": "v2ray-plugin",
				"v2ray-plugin-opts": map[string]any{
					"host": "cdn.example.com", "skip-cert-verify": true,
					"headers":  map[string]any{"X-Test": "value"},
					"ech-opts": map[string]any{"enable": true, "query-server-name": "ech.example.com"},
				},
			},
			want: map[string]any{
				"plugin": "v2ray-plugin",
				"plugin-opts": map[string]any{
					"mode": "websocket", "host": "cdn.example.com", "skip-cert-verify": true,
					"headers":  map[string]any{"X-Test": "value"},
					"ech-opts": map[string]any{"enable": true, "query-server-name": "ech.example.com"},
				},
			},
		},
		{
			name:   "shadow-tls",
			params: map[string]any{"plugin": "shadow-tls", "shadow-tls-opts": map[string]any{"host": "tls.example.com", "version": 3}},
			want:   map[string]any{"plugin": "shadow-tls", "plugin-opts": map[string]any{"host": "tls.example.com", "version": 3}},
		},
		{
			name: "restls",
			params: map[string]any{
				"plugin": "restls", "restls-opts": map[string]any{"password": "p", "host": "tls.example.com", "version-hint": "tls13"},
			},
			want: map[string]any{"plugin": "restls", "plugin-opts": map[string]any{"password": "p", "host": "tls.example.com", "version-hint": "tls13"}},
		},
		{
			name: "unknown",
			params: map[string]any{
				"plugin": "custom-plugin", "plugin-opts": map[string]any{"flag": "", "special": "a;b=c"},
				"obfs-opts": map[string]any{"mode": "http"},
			},
			want: map[string]any{"plugin": "custom-plugin", "plugin-opts": map[string]any{"flag": "", "special": "a;b=c"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := deepCopyMapForTest(t, tc.params)
			first := normalizeClashFields("ss", tc.params)
			second := normalizeClashFields("ss", tc.params)
			if !reflect.DeepEqual(first, tc.want) {
				t.Fatalf("Clash SS 插件投影异常:\n got=%#v\nwant=%#v", first, tc.want)
			}
			if !reflect.DeepEqual(second, first) {
				t.Fatalf("重复投影结果不稳定:\nfirst=%#v\nsecond=%#v", first, second)
			}
			if !reflect.DeepEqual(tc.params, before) {
				t.Fatalf("Clash 投影修改了输入:\n got=%#v\nwant=%#v", tc.params, before)
			}

			proxy := new(Service).clashProxy(&nodeData{RenderName: "节点", Protocol: "ss", Host: "example.com", Port: 443, ProtocolJSON: tc.params})
			raw, err := gyaml.Marshal(orderedMapToMapSlice(proxy))
			if err != nil {
				t.Fatal(err)
			}
			var decoded any
			if err := gyaml.UnmarshalWithOptions(raw, &decoded, gyaml.UseOrderedMap()); err != nil {
				t.Fatal(err)
			}
			plain, ok := plainYAMLValue(decoded).(map[string]any)
			if !ok {
				t.Fatalf("YAML 节点不是映射: %#v", decoded)
			}
			wantProxy := map[string]any{"name": "节点", "type": "ss", "server": "example.com", "port": 443}
			for key, value := range tc.want {
				wantProxy[key] = value
			}
			if !reflect.DeepEqual(plain, wantProxy) {
				t.Fatalf("YAML 解码后的 SS 节点异常:\n got=%#v\nwant=%#v\nYAML:\n%s", plain, wantProxy, raw)
			}
		})
	}
}

func deepCopyMapForTest(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	out := make(map[string]any, len(value))
	for key, item := range value {
		switch typed := item.(type) {
		case map[string]any:
			out[key] = deepCopyMapForTest(t, typed)
		case []any:
			items := make([]any, len(typed))
			copy(items, typed)
			out[key] = items
		default:
			out[key] = typed
		}
	}
	return out
}

func plainYAMLValue(value any) any {
	switch typed := value.(type) {
	case gyaml.MapSlice:
		out := make(map[string]any, len(typed))
		for _, item := range typed {
			key, ok := item.Key.(string)
			if ok {
				out[key] = plainYAMLValue(item.Value)
			}
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i := range typed {
			out[i] = plainYAMLValue(typed[i])
		}
		return out
	case uint64:
		return int(typed)
	default:
		return typed
	}
}
