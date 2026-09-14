package node

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestObjectAllowUnknownExplicitWhitelistSchema(t *testing.T) {
	vless, err := GetProtocol("vless")
	if err != nil {
		t.Fatal(err)
	}
	ws, ok := findSchemaField(vless.FormSchema, "ws-opts")
	if !ok {
		t.Fatal("缺少 vless.ws-opts")
	}
	if ws.AllowUnknown {
		t.Fatal("固定对象 ws-opts 不应允许未知键")
	}
	headers, ok := findSchemaField(ws.Properties, "headers")
	if !ok || !headers.AllowUnknown || headers.ObjectKind != "map" {
		t.Fatalf("ws-opts.headers 必须显式开放 Map: %+v", headers)
	}
	grpc, _ := findSchemaField(vless.FormSchema, "grpc-opts")
	if grpc.AllowUnknown {
		t.Fatal("固定对象 grpc-opts 不应允许未知键")
	}
	if !strings.Contains(mustJSON(t, ws), `"allow_unknown":false`) {
		t.Fatal("固定对象 schema JSON 必须固定下发 allow_unknown=false")
	}
	if !strings.Contains(mustJSON(t, headers), `"allow_unknown":true`) {
		t.Fatal("开放 Map schema JSON 必须固定下发 allow_unknown=true")
	}

	ss, err := GetProtocol("ss")
	if err != nil {
		t.Fatal(err)
	}
	pluginOpts, _ := findSchemaField(ss.FormSchema, "plugin-opts")
	if !pluginOpts.AllowUnknown || pluginOpts.ObjectKind != "map" || pluginOpts.MapValueType != "string" {
		t.Fatalf("未知 SS plugin-opts 必须是显式开放字符串 Map: %+v", pluginOpts)
	}
	v2ray, _ := findSchemaField(ss.FormSchema, "v2ray-plugin-opts")
	if v2ray.AllowUnknown {
		t.Fatal("v2ray-plugin-opts 固定对象不应允许未知键")
	}
	if !strings.Contains(mustJSON(t, v2ray), `"allow_unknown":false`) {
		t.Fatal("固定对象 schema JSON 必须固定下发 allow_unknown=false")
	}
}

func TestOpenMapsAcceptUnknownOrdinaryKeys(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	t.Run("headers map keeps arbitrary keys", func(t *testing.T) {
		created, err := svc.CreateManual(ctx, CreateManualInput{
			Name: "Headers开放Map", Protocol: "vless", Host: "example.com", Port: 443,
			ProtocolJSON: map[string]any{
				"uuid": "headers-secret", "network": "ws", "security": "none",
				"ws-opts": map[string]any{"path": "/ws", "headers": map[string]any{
					"X-Custom": "value", "X-Number": float64(1), "Host": "cdn.example.com",
				}},
			},
		})
		if err != nil {
			t.Fatalf("Headers 开放 Map 应允许普通未知键: %v", err)
		}
		detail, err := svc.Get(ctx, created.ID)
		if err != nil {
			t.Fatal(err)
		}
		for path, want := range map[string]any{
			"ws-opts.headers.X-Custom": "value", "ws-opts.headers.X-Number": float64(1), "ws-opts.headers.Host": "cdn.example.com",
		} {
			if got, _ := GetPath(detail.ProtocolJSON, path); got != want {
				t.Fatalf("%s = %#v，期望 %#v", path, got, want)
			}
		}
	})

	t.Run("top-level http headers map keeps arbitrary keys", func(t *testing.T) {
		_, err := svc.CreateManual(ctx, CreateManualInput{
			Name: "HTTP开放Map", Protocol: "http", Host: "example.com", Port: 443,
			ProtocolJSON: map[string]any{"username": "u", "password": "p", "headers": map[string]any{"X-Trace": "abc"}},
		})
		if err != nil {
			t.Fatalf("HTTP headers 开放 Map 应允许普通未知键: %v", err)
		}
	})

	t.Run("unknown ss plugin opts keep string map contract", func(t *testing.T) {
		opts := map[string]any{"mode": "custom", "password": "ordinary", "token": "also-ordinary"}
		created, err := svc.CreateManual(ctx, CreateManualInput{
			Name: "未知插件普通Map", Protocol: "ss", Host: "example.com", Port: 443,
			ProtocolJSON: map[string]any{"cipher": "aes-256-gcm", "password": "ss-main", "plugin": "custom-plugin", "plugin-opts": opts},
		})
		if err != nil {
			t.Fatalf("未知 SS plugin-opts 应保持普通字符串 Map: %v", err)
		}
		detail, err := svc.Get(ctx, created.ID)
		if err != nil {
			t.Fatal(err)
		}
		got, ok := detail.ProtocolJSON["plugin-opts"].(map[string]any)
		if !ok || got["mode"] != "custom" || got["password"] != "ordinary" {
			t.Fatalf("未知 SS plugin-opts 回显异常: %#v", detail.ProtocolJSON["plugin-opts"])
		}
		for _, path := range detail.SavedSensitivePaths {
			if strings.HasPrefix(path, "plugin-opts.") {
				t.Fatalf("未知插件参数不应进入凭据敏感路径: %+v", detail.SavedSensitivePaths)
			}
		}
	})
}

func TestFixedObjectUnknownKeysRejectedAndZeroWrite(t *testing.T) {
	svc, st, _ := newTestService(t)
	ctx := context.Background()
	cases := []struct {
		name         string
		protocol     string
		params       map[string]any
		wantPathPart string
	}{
		{"vless grpc", "vless", map[string]any{
			"uuid": "fixed-grpc", "network": "grpc", "grpc-opts": map[string]any{"grpc-service-name": "svc", "future": true},
		}, "grpc-opts.future"},
		{"vless ws", "vless", map[string]any{
			"uuid": "fixed-ws", "network": "ws", "ws-opts": map[string]any{"path": "/ws", "future": true},
		}, "ws-opts.future"},
		{"vless reality", "vless", map[string]any{
			"uuid": "fixed-reality", "network": "tcp", "security": "reality",
			"reality-opts": map[string]any{"public-key": "pk", "short-id": "abcd", "future": true},
		}, "reality-opts.future"},
		{"vless xhttp", "vless", map[string]any{
			"uuid": "fixed-xhttp", "network": "xhttp", "xhttp-opts": map[string]any{"path": "/x", "mode": "auto", "future": true},
		}, "xhttp-opts.future"},
		{"vless smux", "vless", map[string]any{
			"uuid": "fixed-smux", "network": "tcp", "smux": map[string]any{"enabled": true, "future": true},
		}, "smux.future"},
		{"vmess ws", "vmess", map[string]any{
			"uuid": "fixed-vmess", "network": "ws", "ws-opts": map[string]any{"future": true},
		}, "ws-opts.future"},
		{"ss obfs", "ss", map[string]any{
			"cipher": "aes-256-gcm", "password": "ss-secret", "plugin": "obfs",
			"obfs-opts": map[string]any{"mode": "http", "future": true},
		}, "obfs-opts.future"},
		{"ss v2ray plugin", "ss", map[string]any{
			"cipher": "aes-256-gcm", "password": "ss-secret", "plugin": "v2ray-plugin",
			"v2ray-plugin-opts": map[string]any{"mode": "websocket", "future": true},
		}, "v2ray-plugin-opts.future"},
		{"anytls ech", "anytls", map[string]any{
			"password": "any-secret", "ech-opts": map[string]any{"enable": true, "future": true},
		}, "ech-opts.future"},
		{"wireguard peers", "wireguard", map[string]any{
			"private-key": "wg-secret", "public-key": "wg-public",
			"peers": []any{map[string]any{"server": "peer.example.com", "future": true}},
		}, "peers[0].future"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name := "固定对象未知键-" + strings.ReplaceAll(tc.name, " ", "-")
			_, err := svc.CreateManual(ctx, CreateManualInput{
				Name: name, Protocol: tc.protocol, Host: "example.com", Port: 443, ProtocolJSON: cloneJSONMap(tc.params),
			})
			if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), tc.wantPathPart) {
				t.Fatalf("固定对象未知键应拒绝并定位 %s，实际: %v", tc.wantPathPart, err)
			}
			var count int
			if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM nodes WHERE name = ?`, name).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatalf("固定对象未知键创建失败后不应写入，count=%d", count)
			}
		})
	}
}

func TestFixedObjectUnknownKeyUpdateAndHistoricalDeleteBoundary(t *testing.T) {
	svc, st, _ := newTestService(t)
	ctx := context.Background()
	created, err := svc.CreateManual(ctx, CreateManualInput{
		Name: "历史未知键节点", Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"uuid": "legacy-secret", "network": "ws", "ws-opts": map[string]any{"path": "/ws"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	wsOpts := raw.ProtocolJSON["ws-opts"].(map[string]any)
	wsOpts["future"] = "legacy-unknown"
	encoded, err := json.Marshal(raw.ProtocolJSON)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx, `UPDATE nodes SET protocol_json = ? WHERE id = ?`, string(encoded), created.ID); err != nil {
		t.Fatal(err)
	}

	detail, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := GetPath(detail.ProtocolJSON, "ws-opts.future"); got != "legacy-unknown" {
		t.Fatalf("历史未知键应在读取时保留并显示: %#v", detail.ProtocolJSON["ws-opts"])
	}

	// 无关字段更新但未提交固定对象：旧未知键继续存在并被校验阻断。
	_, err = svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "vless", Host: "changed.example.com", Port: 443, BaseRevision: created.EditRevision,
		ProtocolJSON: map[string]any{"uuid": "", "network": "ws"},
	})
	if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "ws-opts.future") {
		t.Fatalf("历史未知键存在时无关更新应被阻断并定位: %v", err)
	}
	before, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before.EditRevision != created.EditRevision || before.Host != "example.com" {
		t.Fatalf("历史未知键阻断不得写库或递增 revision: %+v", before)
	}

	// 显式提交不含未知键的固定对象：允许删除历史未知键并保存。
	updated, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "vless", Host: "changed.example.com", Port: 443, BaseRevision: created.EditRevision,
		ProtocolJSON: map[string]any{"uuid": "", "network": "ws", "ws-opts": map[string]any{"path": "/ws"}},
	})
	if err != nil {
		t.Fatalf("高级 JSON 显式删除历史未知键后应允许保存: %v", err)
	}
	if updated.EditRevision != created.EditRevision+1 {
		t.Fatalf("显式删除后成功保存应递增 revision: %+v", updated)
	}
	reloaded, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := GetPath(reloaded.ProtocolJSON, "ws-opts.future"); exists {
		t.Fatalf("显式删除后历史未知键不应复活: %#v", reloaded.ProtocolJSON["ws-opts"])
	}
}

func TestProjectActiveDropsFixedUnknownButKeepsOpenMap(t *testing.T) {
	vless, err := GetProtocol("vless")
	if err != nil {
		t.Fatal(err)
	}
	active := ProjectActive(vless, CurrentState{Network: "ws", Security: "none"}, map[string]any{
		"uuid": "project-secret", "network": "ws",
		"ws-opts": map[string]any{
			"path": "/ws", "future": "must-not-output",
			"headers": map[string]any{"X-Custom": "kept"},
		},
	})
	if _, exists := GetPath(active, "ws-opts.future"); exists {
		t.Fatalf("固定对象历史未知键不得进入输出投影: %#v", active["ws-opts"])
	}
	if got, _ := GetPath(active, "ws-opts.headers.X-Custom"); got != "kept" {
		t.Fatalf("开放 Headers Map 的普通键应保留: %#v", active["ws-opts"])
	}

	ss, err := GetProtocol("ss")
	if err != nil {
		t.Fatal(err)
	}
	activeSS := ProjectActive(ss, CurrentState{Plugin: stringPtr("custom-plugin")}, map[string]any{
		"cipher": "aes-256-gcm", "password": "secret", "plugin": "custom-plugin",
		"plugin-opts": map[string]any{"mode": "custom", "token": "ordinary"},
	})
	if got, _ := GetPath(activeSS, "plugin-opts.mode"); got != "custom" {
		t.Fatalf("未知 SS plugin-opts 普通字符串键必须保留: %#v", activeSS["plugin-opts"])
	}
}

func stringPtr(value string) *string { return &value }
