package node

import (
	"context"
	"reflect"
	"testing"
)

func TestFirstBatchEditorLayout(t *testing.T) {
	cases := []struct {
		protocol string
		state    CurrentState
		want     []string
	}{
		{"vless", CurrentState{Network: "tcp", Security: "reality"}, []string{"network", "security", "flow", "servername", "client-fingerprint", "reality-opts", "alpn"}},
		{"vless", CurrentState{Network: "ws", Security: "tls"}, []string{"network", "security", "ws-opts", "servername", "client-fingerprint", "alpn", "fingerprint"}},
		{"vmess", CurrentState{Network: "grpc", Security: "tls"}, []string{"network", "security", "cipher", "grpc-opts", "servername", "client-fingerprint", "alpn", "fingerprint"}},
		{"trojan", CurrentState{Network: "ws", Security: "tls"}, []string{"network", "ws-opts", "sni", "client-fingerprint", "alpn", "fingerprint"}},
		{"ss", CurrentState{Security: "none"}, []string{"cipher", "plugin"}},
	}
	for _, tc := range cases {
		t.Run(tc.protocol+"/"+tc.state.Network+"/"+tc.state.Security, func(t *testing.T) {
			proto, err := GetProtocol(tc.protocol)
			if err != nil {
				t.Fatal(err)
			}
			var names []string
			for _, field := range editorFormSchema(proto) {
				if field.Group == "connection" && field.Matches(tc.state, "") {
					names = append(names, field.Name)
				}
			}
			if !reflect.DeepEqual(names, tc.want) {
				t.Fatalf("连接区顺序不符: got %v want %v", names, tc.want)
			}
			for _, name := range []string{"mptcp", "tfo", "udp"} {
				field := findSchemaFieldMust(t, proto.FormSchema, name)
				if field.Group != "switches" || field.Advanced != (name == "mptcp") {
					t.Fatalf("开关分层不符: %+v", field)
				}
			}
		})
	}
	for _, name := range []string{"vless", "vmess", "ss"} {
		proto, err := GetProtocol(name)
		if err != nil {
			t.Fatal(err)
		}
		smux := findSchemaFieldMust(t, proto.FormSchema, "smux")
		for _, child := range []string{"enabled", "padding", "statistic", "only-tcp"} {
			field := findNestedFieldMust(t, smux, child)
			if field.Group != "switches" || !field.Advanced || field.CanonicalPath != "smux."+child {
				t.Fatalf("嵌套开关应保留路径并进入更多开关: %+v", field)
			}
		}
	}
	for _, name := range []string{"vless", "vmess", "trojan"} {
		proto, err := GetProtocol(name)
		if err != nil {
			t.Fatal(err)
		}
		ws := findSchemaFieldMust(t, proto.FormSchema, "ws-opts")
		for _, child := range ws.Properties {
			wantAdvanced := child.Name != "path" && child.Name != "headers"
			if child.Advanced != wantAdvanced {
				t.Fatalf("WS 字段分层不符: %+v", child)
			}
		}
	}
}

func TestSSClientFingerprintFollowsPlugin(t *testing.T) {
	proto, err := GetProtocol("ss")
	if err != nil {
		t.Fatal(err)
	}
	field := findSchemaFieldMust(t, proto.FormSchema, "client-fingerprint")
	if !field.ShouldReset("plugin") {
		t.Fatal("SS 客户端指纹必须随插件切换清除")
	}
	for _, plugin := range []string{"", "obfs", "v2ray-plugin", "shadow-tls", "restls"} {
		state := CurrentState{Security: "none", Plugin: &plugin}
		want := plugin == "shadow-tls" || plugin == "restls"
		if field.Matches(state, "") != want {
			t.Fatalf("%s 指纹显示条件错误", plugin)
		}
		projected := ProjectActive(proto, state, map[string]any{"cipher": "aes-128-gcm", "password": "synthetic", "plugin": plugin, "client-fingerprint": "chrome"})
		_, exists := projected["client-fingerprint"]
		if exists != want {
			t.Fatalf("%s 指纹活动投影错误: %+v", plugin, projected)
		}
	}
}

func TestSSPluginSwitchClearsFingerprintAndKeepsMainCredential(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	created, err := svc.CreateManual(ctx, CreateManualInput{
		Name: "指纹归属测试", Protocol: "ss", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"cipher": "aes-128-gcm", "password": "synthetic-main", "plugin": "shadow-tls", "client-fingerprint": "chrome"},
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "ss", Host: "example.com", Port: 443, BaseRevision: created.EditRevision,
		ProtocolJSON: map[string]any{"cipher": "aes-128-gcm", "plugin": "obfs", "obfs-opts": map[string]any{"mode": "http"}},
		ResetScopes:  []string{"plugin"},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := svc.getRaw(ctx, updated.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := raw.ProtocolJSON["client-fingerprint"]; exists {
		t.Fatal("已切离插件的指纹不应继续保存")
	}
	password, err := svc.decryptSecret(ctx, raw.ProtocolJSON["password"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if password != "synthetic-main" {
		t.Fatal("插件切换不应清除 SS 主密码")
	}
}
