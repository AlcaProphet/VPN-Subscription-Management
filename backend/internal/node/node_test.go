package node

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/migrations"
)

func newTestService(t *testing.T) (*Service, *store.Store, *config.Service) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), migrations.FS); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	if err := cfg.Set(context.Background(), config.KeySigningKey, "test-signing-key-0123456789abcdef"); err != nil {
		t.Fatalf("写入签名密钥失败: %v", err)
	}
	svc := NewService(st, cfg, log.New("error", "console"))
	return svc, st, cfg
}

func createManual(t *testing.T, svc *Service, name string) *Node {
	t.Helper()
	n, err := svc.CreateManual(context.Background(), CreateManualInput{
		Name: name, Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp"},
	})
	if err != nil {
		t.Fatalf("创建 manual 节点失败: %v", err)
	}
	return n
}

func TestLegacyWSReadCheckAndSaveUseCanonicalFields(t *testing.T) {
	for _, canonical := range []bool{false, true} {
		name := "alias-only"
		if canonical {
			name = "canonical-wins"
		}
		t.Run(name, func(t *testing.T) {
			svc, st, _ := newTestService(t)
			ctx := context.Background()
			created, err := svc.CreateManual(ctx, CreateManualInput{
				Name: "旧WS节点", Protocol: "vless", Host: "example.com", Port: 443,
				ProtocolJSON: map[string]any{"uuid": "ws-secret", "network": "ws", "security": "tls"},
			})
			if err != nil {
				t.Fatal(err)
			}
			raw, err := svc.getRaw(ctx, created.ID)
			if err != nil {
				t.Fatal(err)
			}
			raw.ProtocolJSON["ws-path"] = "/legacy"
			raw.ProtocolJSON["ws-headers"] = map[string]any{"Host": "old.example.com", "X-Keep": "yes"}
			wantPath, wantHost := "/legacy", "old.example.com"
			if canonical {
				wantPath, wantHost = "/current", "current.example.com"
				raw.ProtocolJSON["ws-opts"] = map[string]any{"path": wantPath, "headers": map[string]any{"Host": wantHost}}
			}
			before, err := json.Marshal(raw.ProtocolJSON)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := st.DB().ExecContext(ctx, `UPDATE nodes SET protocol_json=? WHERE id=?`, string(before), created.ID); err != nil {
				t.Fatal(err)
			}
			assertCanonical := func(params map[string]any) {
				t.Helper()
				for _, key := range []string{"ws-path", "ws-headers"} {
					if _, exists := params[key]; exists {
						t.Errorf("仍含旧字段 %s", key)
					}
				}
				for path, want := range map[string]string{"ws-opts.path": wantPath, "ws-opts.headers.Host": wantHost, "ws-opts.headers.X-Keep": "yes"} {
					if value, ok := GetPath(params, path); !ok || value != want {
						t.Errorf("%s = %v，期望 %s", path, value, want)
					}
				}
			}
			detail, err := svc.Get(ctx, created.ID)
			if err != nil {
				t.Fatal(err)
			}
			assertCanonical(detail.ProtocolJSON)
			list, err := svc.List(ctx, "manual")
			if err != nil || len(list) != 1 {
				t.Fatalf("列表读取失败: %v", err)
			}
			assertCanonical(list[0].ProtocolJSON)
			if detail.ProtocolJSON["uuid"] != "" || list[0].ProtocolJSON["uuid"] != "" || detail.CurrentState.Security != "tls" {
				t.Fatal("读取破坏了凭据脱敏或当前状态")
			}
			svc.SetCheckRenderer(func(_ context.Context, _, _, _, _ string, _ int, params map[string]any) (CheckRenderResult, error) {
				assertCanonical(params)
				if params["tls"] != true || params["uuid"] != "REDACTED" {
					t.Error("检查破坏了 TLS 或凭据脱敏")
				}
				return CheckRenderResult{}, nil
			})
			draft := cloneJSONMap(detail.ProtocolJSON)
			draft["security"] = "tls"
			delete(draft, "tls")
			checked, err := svc.Check(ctx, CheckRequest{
				NodeID: created.ID, BaseRevision: created.EditRevision, Protocol: "vless", Host: "example.com", Port: 443,
				ProtocolJSON: draft, CurrentState: &detail.CurrentState, Targets: []string{"clash-yaml"},
			})
			if err != nil {
				t.Fatal(err)
			}
			if checked.Targets["clash-yaml"].Status != "ok" {
				t.Fatalf("旧节点规范草稿检查失败: %+v", checked)
			}
			var after string
			if err := st.DB().QueryRowContext(ctx, `SELECT protocol_json FROM nodes WHERE id=?`, created.ID).Scan(&after); err != nil {
				t.Fatal(err)
			}
			if after != string(before) {
				t.Fatal("读取或检查回写了数据库")
			}
			updated, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
				BaseRevision: created.EditRevision, Protocol: "vless", Host: "example.com", Port: 443,
				ProtocolJSON: draft, CurrentState: &detail.CurrentState,
			})
			if err != nil {
				t.Fatal(err)
			}
			assertCanonical(updated.ProtocolJSON)
			stored, err := svc.getRaw(ctx, created.ID)
			if err != nil {
				t.Fatal(err)
			}
			assertCanonical(stored.ProtocolJSON)
			if stored.ProtocolJSON["uuid"] != raw.ProtocolJSON["uuid"] || stored.EditRevision != 2 {
				t.Fatal("更新改变了已保存凭据或未递增修订")
			}
		})
	}
}

func TestValidateNodeName(t *testing.T) {
	cases := []struct {
		name string
		ok   bool
	}{
		{"节点A", true},
		{"🇺🇸US-1", true},
		{"节点 A", false},
		{"节点,A", false},
		{" 节点", false},
		{"节点 ", false},
		{"节\u0000点", false},
		{"", false},
	}
	for _, c := range cases {
		err := ValidateNodeName(c.name)
		if (err == nil) != c.ok {
			t.Errorf("ValidateNodeName(%q) = %v, want ok=%v", c.name, err, c.ok)
		}
	}
}

func TestValidateProxyGroupNameAllowsSpace(t *testing.T) {
	if err := ValidateProxyGroupName("My Group"); err != nil {
		t.Errorf("代理组名应允许空格: %v", err)
	}
	if err := ValidateProxyGroupName("My,Group"); err == nil {
		t.Error("代理组名应禁止逗号")
	}
}

func TestRegistryCompleteness(t *testing.T) {
	protos := ManualProtocols()
	if len(protos) != 19 {
		t.Fatalf("协议数量应为 19，实际 %d", len(protos))
	}
	for _, p := range protos {
		if p.Protocol == "ssr" {
			t.Error("ssr 不应在注册表")
		}
		if len(p.SensitiveFields) == 0 && p.Protocol != "openvpn" {
			t.Errorf("协议 %s 应有敏感字段", p.Protocol)
		}
		if !HasProtocol(p.Protocol) {
			t.Errorf("HasProtocol(%s) 应为 true", p.Protocol)
		}
		for _, field := range p.FormSchema {
			if field.Section == "" {
				t.Errorf("协议 %s 字段 %s 缺少表单分区", p.Protocol, field.Name)
			}
			if field.Type == "bool" && field.Section != "switches" {
				t.Errorf("协议 %s 布尔字段 %s 应归入 switches", p.Protocol, field.Name)
			}
			if field.Type == "object" && field.ObjectKind == "" {
				t.Errorf("协议 %s 对象字段 %s 缺少对象形态", p.Protocol, field.Name)
			}
		}
	}
	if HasProtocol("ssr") {
		t.Error("HasProtocol(ssr) 应为 false")
	}
}

func TestRegistryUsesMihomoNativeFields(t *testing.T) {
	for _, protocol := range []string{"vmess", "vless", "hysteria", "hysteria2", "tuic", "wireguard", "anytls"} {
		p, err := GetProtocol(protocol)
		if err != nil {
			t.Fatal(err)
		}
		forbidden := map[string]bool{"path": true, "host": true, "mport": true, "insecure": true, "allow_insecure": true, "allowInsecure": true, "address": true}
		for _, schema := range p.FormSchema {
			if forbidden[schema.Name] {
				t.Errorf("%s 不应声明旧字段 %s", protocol, schema.Name)
			}
		}
	}
	ss, _ := GetProtocol("ss")
	if !contains(ss.SensitiveFields, "shadow-tls-opts.password") || !contains(ss.SensitiveFields, "restls-opts.password") {
		t.Fatal("SS 应声明各插件独立的嵌套插件密码")
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestCreateManualConflictAndNamespace(t *testing.T) {
	svc, _, _ := newTestService(t)
	createManual(t, svc, "节点A")
	// 重名
	_, err := svc.CreateManual(context.Background(), CreateManualInput{
		Name: "节点A", Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"uuid": "11111111-2222-3333-4444-555555555555"},
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("重名应 ErrConflict，实际 %v", err)
	}
	// 与代理组名冲突（迁移种子含 🎬YouTube）
	_, err = svc.CreateManual(context.Background(), CreateManualInput{
		Name: "🎬YouTube", Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"uuid": "11111111-2222-3333-4444-555555555555"},
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("与代理组名冲突应 ErrConflict，实际 %v", err)
	}
	// 与内建保留名冲突
	_, err = svc.CreateManual(context.Background(), CreateManualInput{
		Name: "DIRECT", Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"uuid": "11111111-2222-3333-4444-555555555555"},
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("与内建保留名冲突应 ErrConflict，实际 %v", err)
	}
}

func TestCreateManualEncryptionAndRedact(t *testing.T) {
	svc, st, _ := newTestService(t)
	n, err := svc.CreateManual(context.Background(), CreateManualInput{
		Name: "加密节点", Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"uuid": "secret-uuid-123", "network": "tcp"},
	})
	if err != nil {
		t.Fatalf("创建节点失败: %v", err)
	}
	var raw string
	if err := st.DB().QueryRowContext(context.Background(), `SELECT protocol_json FROM nodes WHERE id = ?`, n.ID).Scan(&raw); err != nil {
		t.Fatalf("读取节点参数失败: %v", err)
	}
	if strings.Contains(raw, "secret-uuid-123") {
		t.Fatal("库内不应出现明文凭据")
	}
	if !strings.Contains(raw, encPrefix) {
		t.Fatal("库内应包含密文前缀")
	}
	got, err := svc.Get(context.Background(), n.ID)
	if err != nil {
		t.Fatalf("读取节点失败: %v", err)
	}
	if got.ProtocolJSON["uuid"] != "" {
		t.Fatalf("列表/详情敏感字段应脱敏为空，实际 %v", got.ProtocolJSON["uuid"])
	}
}

func TestUpdateManualPreserveSensitive(t *testing.T) {
	svc, st, _ := newTestService(t)
	createManual(t, svc, "保留节点")
	var id int64
	if err := st.DB().QueryRowContext(context.Background(), `SELECT id FROM nodes WHERE name='保留节点'`).Scan(&id); err != nil {
		t.Fatalf("查询节点失败: %v", err)
	}
	_, err := svc.UpdateManual(context.Background(), id, UpdateManualInput{
		Protocol: "vless", Host: "new.example.com", Port: 8443,
		ProtocolJSON: map[string]any{"uuid": "", "network": "ws"}, BaseRevision: 1,
	})
	if err != nil {
		t.Fatalf("更新节点失败: %v", err)
	}
	raw, err := svc.getRaw(context.Background(), id)
	if err != nil {
		t.Fatalf("读取节点失败: %v", err)
	}
	if raw.Host != "new.example.com" || raw.Port != 8443 {
		t.Fatalf("非敏感字段未更新: %+v", raw)
	}
	enc, _ := raw.ProtocolJSON["uuid"].(string)
	if !strings.HasPrefix(enc, encPrefix) {
		t.Fatalf("留空敏感字段应保留原密文，实际 %v", raw.ProtocolJSON["uuid"])
	}
	// 编辑时名称只读
	_, err = svc.UpdateManual(context.Background(), id, UpdateManualInput{
		Name: "改名", Protocol: "vless", Host: "new.example.com", Port: 8443,
		ProtocolJSON: map[string]any{"uuid": "new-uuid"}, BaseRevision: 2,
	})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("改名应 ErrBadRequest，实际 %v", err)
	}
}

func TestNestedSensitiveEncryptRedactAndPreserve(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	created, err := svc.CreateManual(ctx, CreateManualInput{
		Name: "插件节点", Protocol: "ss", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{
			"cipher": "aes-256-gcm", "password": "main-secret",
			"plugin": "shadow-tls", "shadow-tls-opts": map[string]any{"host": "cdn.example.com", "password": "nested-secret"},
		},
	})
	if err != nil {
		t.Fatalf("创建插件节点失败: %v", err)
	}
	raw, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	nested, ok := GetPath(raw.ProtocolJSON, "shadow-tls-opts.password")
	if !ok || !strings.HasPrefix(nested.(string), encPrefix) {
		t.Fatalf("嵌套密码未加密: %#v", nested)
	}
	visible, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if value, _ := GetPath(visible.ProtocolJSON, "shadow-tls-opts.password"); value != "" {
		t.Fatalf("嵌套密码未脱敏: %#v", value)
	}
	_, err = svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "ss", Host: "new.example.com", Port: 8443,
		BaseRevision: 1,
		ProtocolJSON: map[string]any{
			"cipher": "aes-256-gcm", "password": "",
			"plugin": "shadow-tls", "shadow-tls-opts": map[string]any{"host": "next.example.com", "password": ""},
		},
	})
	if err != nil {
		t.Fatalf("更新插件节点失败: %v", err)
	}
	updated, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	preserved, _ := GetPath(updated.ProtocolJSON, "shadow-tls-opts.password")
	if preserved != nested {
		t.Fatalf("留空未保留嵌套密文: old=%v new=%v", nested, preserved)
	}
}

func TestValidateProtocolFieldTypes(t *testing.T) {
	wg, _ := GetProtocol("wireguard")
	valid := map[string]any{"private-key": "secret", "public-key": "pub", "allowed-ips": "0.0.0.0/0,::/0", "reserved": "1,2,3", "peers": []any{map[string]any{"server": "peer"}}}
	if err := validateProtocolFields(wg, valid, false); err != nil {
		t.Fatalf("合法列表/对象字段被拒绝: %v", err)
	}
	valid["reserved"] = true
	if err := validateProtocolFields(wg, valid, false); err == nil {
		t.Fatal("错误 int-list 类型应被拒绝")
	}
	valid["reserved"] = "1,2,3"
	valid["peers"] = []any{map[string]any{"server": "peer", "port": true}}
	if err := validateProtocolFields(wg, valid, false); err == nil || !strings.Contains(err.Error(), "peers[0].port") {
		t.Fatalf("嵌套字段类型错误应定位完整路径，实际 %v", err)
	}
}

func TestObjectSchemaKeepsExtensionsAndSmuxShape(t *testing.T) {
	vless, _ := GetProtocol("vless")
	params := map[string]any{
		"uuid": "secret",
		"ws-opts": map[string]any{
			"path":          "/ws",
			"headers":       map[string]any{"Host": []any{"cdn.example.com"}},
			"future-option": map[string]any{"enabled": true},
		},
		"smux": map[string]any{
			"enabled":     true,
			"protocol":    "smux",
			"brutal-opts": map[string]any{"enabled": true, "up": "100 Mbps"},
		},
	}
	if err := validateProtocolFields(vless, params, false); err != nil {
		t.Fatalf("已知嵌套字段与未知扩展键应兼容: %v", err)
	}
	params["smux"] = true
	if err := validateProtocolFields(vless, params, false); err == nil {
		t.Fatal("smux 应按嵌套对象校验")
	}
}

func TestXrayDisplayNameAndPublic(t *testing.T) {
	svc, st, _ := newTestService(t)
	// 插入 xray 实例与节点
	res, err := st.DB().ExecContext(context.Background(),
		`INSERT INTO xray_instances (name, slug, api_addr) VALUES ('实例','instance-a','https://example.com')`)
	if err != nil {
		t.Fatalf("插入实例失败: %v", err)
	}
	instID, _ := res.LastInsertId()
	res, err = st.DB().ExecContext(context.Background(),
		`INSERT INTO nodes (source,name,instance_id,tag,protocol,host,port,protocol_json,allocatable,missing)
		 VALUES ('xray','instance-a-vless',?,'vless','vless','example.com',443,'{"uuid":"enc:v1:abc"}',1,0)`, instID)
	if err != nil {
		t.Fatalf("插入 xray 节点失败: %v", err)
	}
	nodeID, _ := res.LastInsertId()
	// manual 节点不允许 display_name
	m := createManual(t, svc, "手工节点")
	_, err = svc.SetDisplayName(context.Background(), m.ID, "新名字")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("manual 设置显示名应 ErrForbidden，实际 %v", err)
	}
	// xray 设置显示名成功
	n, err := svc.SetDisplayName(context.Background(), nodeID, "日本节点")
	if err != nil {
		t.Fatalf("设置显示名失败: %v", err)
	}
	if n.RenderName != "日本节点" {
		t.Fatalf("RenderName 异常: %s", n.RenderName)
	}
	// 清空回退
	n, err = svc.SetDisplayName(context.Background(), nodeID, "")
	if err != nil {
		t.Fatalf("清空显示名失败: %v", err)
	}
	if n.RenderName != "instance-a-vless" {
		t.Fatalf("清空后应回退系统名: %s", n.RenderName)
	}
	// manual 不允许 is_public
	_, err = svc.SetPublic(context.Background(), m.ID, true)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("manual 设置公共应 ErrForbidden，实际 %v", err)
	}
	// 不可分配 xray 节点不允许 is_public
	if _, err := st.DB().ExecContext(context.Background(), `UPDATE nodes SET allocatable=0, missing=0 WHERE id=?`, nodeID); err != nil {
		t.Fatalf("更新节点失败: %v", err)
	}
	_, err = svc.SetPublic(context.Background(), nodeID, true)
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("不可分配节点设置公共应 ErrBadRequest，实际 %v", err)
	}
	// 非 missing xray 不可删除
	if err := svc.Delete(context.Background(), nodeID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("非 missing xray 删除应 ErrForbidden，实际 %v", err)
	}
}

func TestDisplayNameConflictAcrossNodes(t *testing.T) {
	svc, st, _ := newTestService(t)
	createManual(t, svc, "节点A")
	res, err := st.DB().ExecContext(context.Background(),
		`INSERT INTO xray_instances (name, slug, api_addr) VALUES ('实例','instance-b','https://example.com')`)
	if err != nil {
		t.Fatalf("插入实例失败: %v", err)
	}
	instID, _ := res.LastInsertId()
	res, err = st.DB().ExecContext(context.Background(),
		`INSERT INTO nodes (source,name,instance_id,tag,protocol,host,port,protocol_json,allocatable,missing)
		 VALUES ('xray','instance-b-vless',?,'vless','vless','example.com',443,'{"uuid":"enc:v1:abc"}',1,0)`, instID)
	if err != nil {
		t.Fatalf("插入 xray 节点失败: %v", err)
	}
	nodeID, _ := res.LastInsertId()
	// display_name 与已有 manual 有效渲染名冲突
	_, err = svc.SetDisplayName(context.Background(), nodeID, "节点A")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("显示名冲突应 ErrConflict，实际 %v", err)
	}
	// 与强制组名冲突
	_, err = svc.SetDisplayName(context.Background(), nodeID, ForceDirect)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("显示名与强制组冲突应 ErrConflict，实际 %v", err)
	}
	// 与代理组名冲突
	_, err = svc.SetDisplayName(context.Background(), nodeID, "🎬YouTube")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("显示名与代理组冲突应 ErrConflict，实际 %v", err)
	}
}

func TestProtocolJSONRoundTrip(t *testing.T) {
	svc, _, _ := newTestService(t)
	n, err := svc.CreateManual(context.Background(), CreateManualInput{
		Name: "SS节点", Protocol: "ss", Host: "1.2.3.4", Port: 8388,
		ProtocolJSON: map[string]any{"cipher": "aes-256-gcm", "password": "pw"},
	})
	if err != nil {
		t.Fatalf("创建 ss 节点失败: %v", err)
	}
	raw, err := svc.getRaw(context.Background(), n.ID)
	if err != nil {
		t.Fatalf("读取原始节点失败: %v", err)
	}
	dec, err := svc.decryptSecret(context.Background(), raw.ProtocolJSON["password"].(string))
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}
	if dec != "pw" {
		t.Fatalf("解密结果异常: %s", dec)
	}
}

func TestListFilter(t *testing.T) {
	svc, st, _ := newTestService(t)
	createManual(t, svc, "手工A")
	_, err := st.DB().ExecContext(context.Background(),
		`INSERT INTO xray_instances (name, slug, api_addr) VALUES ('实例','instance-c','https://example.com')`)
	if err != nil {
		t.Fatalf("插入实例失败: %v", err)
	}
	list, err := svc.List(context.Background(), "manual")
	if err != nil {
		t.Fatalf("读取 manual 列表失败: %v", err)
	}
	if len(list) != 1 || list[0].Source != "manual" {
		t.Fatalf("manual 列表异常: %+v", list)
	}
	// 反序列化确认 JSON 可编码
	if _, err := json.Marshal(list); err != nil {
		t.Fatalf("列表 JSON 序列化失败: %v", err)
	}
}

// TestProtocolChangeDropsOldSensitiveAndRedactsWrite 协议变更清理旧敏感字段，写响应脱敏
func TestProtocolChangeDropsOldSensitiveAndRedactsWrite(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	n := createManual(t, svc, "协议变更节点")
	updated, err := svc.UpdateManual(ctx, n.ID, UpdateManualInput{
		Protocol: "ss", Host: "1.2.3.4", Port: 8388,
		BaseRevision: n.EditRevision,
		ProtocolJSON: map[string]any{"cipher": "aes-256-gcm", "password": "new-pw"},
	})
	if err != nil {
		t.Fatalf("协议变更失败: %v", err)
	}
	raw, err := svc.getRaw(ctx, n.ID)
	if err != nil {
		t.Fatalf("读取原始节点失败: %v", err)
	}
	if _, ok := raw.ProtocolJSON["uuid"]; ok {
		t.Fatal("旧协议敏感字段 uuid 不应残留")
	}
	if updated.ProtocolJSON["password"] != "" {
		t.Fatalf("写响应应脱敏敏感字段，实际 %v", updated.ProtocolJSON["password"])
	}
}

func TestNodeEditorMigrationColumns(t *testing.T) {
	_, st, _ := newTestService(t)
	ctx := context.Background()
	rows, err := st.DB().QueryContext(ctx, `PRAGMA table_info(nodes)`)
	if err != nil {
		t.Fatalf("读取 nodes schema 失败: %v", err)
	}
	defer rows.Close()
	want := map[string]bool{
		"edit_revision":        false,
		"state_format_version": false,
		"current_state_json":   false,
		"extensions_json":      false,
	}
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("解析 nodes schema 失败: %v", err)
		}
		if _, ok := want[name]; ok {
			want[name] = true
			if notNull != 1 {
				t.Errorf("%s 应为 NOT NULL", name)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("遍历 nodes schema 失败: %v", err)
	}
	for name, found := range want {
		if !found {
			t.Errorf("缺少迁移列 %s", name)
		}
	}
	var count int
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='node_edit_states'`).Scan(&count); err != nil {
		t.Fatalf("检查独立状态表失败: %v", err)
	}
	if count != 0 {
		t.Fatal("Build17 不应创建独立 node_edit_states 表")
	}
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO nodes (source, name, protocol, host, port) VALUES ('manual', '迁移默认值节点', 'vless', 'example.com', 443)`); err != nil {
		t.Fatalf("写入迁移默认值节点失败: %v", err)
	}
	var revision, stateVersion int
	var stateRaw, extensionsRaw string
	if err := st.DB().QueryRowContext(ctx,
		`SELECT edit_revision, state_format_version, current_state_json, extensions_json FROM nodes WHERE name='迁移默认值节点'`).Scan(&revision, &stateVersion, &stateRaw, &extensionsRaw); err != nil {
		t.Fatalf("读取迁移默认值失败: %v", err)
	}
	if revision != 0 || stateVersion != 1 || stateRaw != "{}" || extensionsRaw != "{}" {
		t.Fatalf("迁移默认值异常: revision=%d state_version=%d state=%q extensions=%q", revision, stateVersion, stateRaw, extensionsRaw)
	}
}

func TestDeriveCurrentStateUsesRealityParamsAsActiveSecurity(t *testing.T) {
	proto, err := GetProtocol("vless")
	if err != nil {
		t.Fatal(err)
	}
	state := DeriveCurrentState(proto, map[string]any{
		"network":      "tcp",
		"reality-opts": map[string]any{"public-key": "stale"},
	})
	if state.Security != "reality" {
		t.Fatalf("存在非空 reality-opts 时应推导 REALITY: %+v", state)
	}
	state = DeriveCurrentState(proto, map[string]any{
		"network":      "tcp",
		"tls":          true,
		"reality-opts": map[string]any{"public-key": "active"},
	})
	if state.Security != "reality" {
		t.Fatalf("tls + reality-opts 应推导 REALITY: %+v", state)
	}
}

func TestCreateWithCurrentStateAndExtensions(t *testing.T) {
	svc, st, _ := newTestService(t)
	ctx := context.Background()
	created, err := svc.CreateManual(ctx, CreateManualInput{
		Name: "状态扩展节点", Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{
			"uuid": "create-secret", "network": "ws", "tls": true,
			"ws-opts": map[string]any{"path": "/ws"},
		},
		CurrentState: &CurrentState{Network: "ws", Security: "tls", Features: []string{}},
		Extensions: []ExtensionInput{{
			ID: "ext-1", Scope: "transport.ws", Targets: []string{"clash-yaml"},
			Label: "WebSocket 扩展", Payload: `{"unknown":true}`,
		}},
	})
	if err != nil {
		t.Fatalf("创建状态扩展节点失败: %v", err)
	}
	if created.EditRevision != 1 || created.StateFormatVersion != 1 {
		t.Fatalf("创建修订/格式版本异常: %+v", created)
	}
	if created.CurrentState.Network != "ws" || created.CurrentState.Security != "tls" || len(created.Extensions) != 1 {
		t.Fatalf("创建后状态/扩展摘要异常: %+v", created)
	}
	if created.Extensions[0].ID != "ext-1" || !created.Extensions[0].Configured {
		t.Fatalf("扩展摘要异常: %+v", created.Extensions[0])
	}
	encoded, err := json.Marshal(created)
	if err != nil {
		t.Fatalf("节点响应序列化失败: %v", err)
	}
	if strings.Contains(string(encoded), "payload_encrypted") || strings.Contains(string(encoded), `{"unknown":true}`) {
		t.Fatalf("节点响应泄漏扩展负载: %s", encoded)
	}
	var protocolRaw, extensionsRaw string
	if err := st.DB().QueryRowContext(ctx, `SELECT protocol_json, extensions_json FROM nodes WHERE id=?`, created.ID).Scan(&protocolRaw, &extensionsRaw); err != nil {
		t.Fatalf("读取节点密文失败: %v", err)
	}
	if strings.Contains(protocolRaw, "create-secret") || strings.Contains(extensionsRaw, `{"unknown":true}`) || !strings.Contains(extensionsRaw, extEncPrefix) {
		t.Fatalf("节点密文存储不符合预期: protocol=%s extensions=%s", protocolRaw, extensionsRaw)
	}
}

func TestUpdateResetScopeDoesNotRestore(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	created, err := svc.CreateManual(ctx, CreateManualInput{
		Name: "分支清空节点", Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{
			"uuid": "branch-secret", "network": "ws", "tls": true,
			"ws-opts": map[string]any{"path": "/old"},
		},
		Extensions: []ExtensionInput{{ID: "ext-ws", Scope: "transport.ws", Payload: "old-extension"}},
	})
	if err != nil {
		t.Fatalf("创建分支清空节点失败: %v", err)
	}
	updated, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "vless", Host: "example.com", Port: 443, BaseRevision: created.EditRevision,
		CurrentState: &CurrentState{Network: "grpc", Security: "tls", Features: []string{}},
		ResetScopes:  []string{"network"},
		ProtocolJSON: map[string]any{
			"uuid": "", "network": "grpc", "tls": true,
			"grpc-opts": map[string]any{"grpc-service-name": "svc"},
		},
		ExtensionOps: []ExtensionOp{{Op: "keep", ID: "ext-ws"}},
	})
	if err != nil {
		t.Fatalf("切换到 gRPC 失败: %v", err)
	}
	if updated.EditRevision != 2 || updated.CurrentState.Network != "grpc" {
		t.Fatalf("切换后修订/状态异常: %+v", updated)
	}
	raw, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatalf("读取切换后节点失败: %v", err)
	}
	if _, ok := GetPath(raw.ProtocolJSON, "ws-opts"); ok {
		t.Fatal("切换网络后旧 ws-opts 不应残留")
	}
	if _, ok := GetPath(raw.ProtocolJSON, "uuid"); !ok {
		t.Fatal("网络切换不应清空无关的协议主凭据")
	}
	if len(raw.extensionRecords) != 0 {
		t.Fatalf("被重置传输扩展不应被 keep 复活: %+v", raw.extensionRecords)
	}

	back, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "vless", Host: "example.com", Port: 443, BaseRevision: updated.EditRevision,
		CurrentState: &CurrentState{Network: "ws", Security: "tls", Features: []string{}},
		ResetScopes:  []string{"network"},
		ProtocolJSON: map[string]any{"uuid": "", "network": "ws", "tls": true},
	})
	if err != nil {
		t.Fatalf("切回 WebSocket 失败: %v", err)
	}
	backRaw, err := svc.getRaw(ctx, back.ID)
	if err != nil {
		t.Fatalf("读取切回后节点失败: %v", err)
	}
	if _, ok := GetPath(backRaw.ProtocolJSON, "ws-opts"); ok {
		t.Fatal("A→B→A 不应恢复旧 ws-opts")
	}
	if len(backRaw.extensionRecords) != 0 {
		t.Fatal("A→B→A 不应恢复旧传输扩展")
	}
}

func TestUpdateCredentialKeepAndClear(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	created := createManual(t, svc, "凭据操作节点")
	rawBefore, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	oldCiphertext := rawBefore.ProtocolJSON["uuid"]
	kept, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "vless", Host: "new.example.com", Port: 8443, BaseRevision: created.EditRevision,
		ProtocolJSON:  map[string]any{"uuid": "", "network": "tcp"},
		CredentialOps: []CredentialOp{{Path: "uuid", Op: "keep"}},
	})
	if err != nil {
		t.Fatalf("默认保留凭据更新失败: %v", err)
	}
	rawKept, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rawKept.ProtocolJSON["uuid"] != oldCiphertext || kept.EditRevision != 2 {
		t.Fatalf("默认保留凭据异常: old=%v new=%v response=%+v", oldCiphertext, rawKept.ProtocolJSON["uuid"], kept)
	}
	_, err = svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "vless", Host: "should-not-write.example.com", Port: 9443, BaseRevision: kept.EditRevision,
		ProtocolJSON:  map[string]any{"uuid": ""},
		CredentialOps: []CredentialOp{{Path: "uuid", Op: "clear"}},
	})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("清空必填凭据后应拒绝保存: %v", err)
	}
	rawAfter, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rawAfter.EditRevision != kept.EditRevision || rawAfter.Host != "new.example.com" {
		t.Fatalf("清空必填凭据失败后不应部分写入: %+v", rawAfter)
	}
}

func TestUpdateRevisionConflict(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	created := createManual(t, svc, "修订冲突节点")
	updated, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "vless", Host: "first.example.com", Port: 443, BaseRevision: created.EditRevision,
		ProtocolJSON: map[string]any{"uuid": "", "network": "tcp"},
	})
	if err != nil {
		t.Fatalf("首次更新失败: %v", err)
	}
	_, err = svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "vless", Host: "stale.example.com", Port: 443, BaseRevision: created.EditRevision,
		ProtocolJSON: map[string]any{"uuid": "", "network": "tcp"},
	})
	if !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("旧修订应返回 ErrRevisionConflict: %v", err)
	}
	current, ok := CurrentRevisionFromError(err)
	if !ok || current != updated.EditRevision {
		t.Fatalf("冲突错误未携带当前修订号: current=%d ok=%v err=%v", current, ok, err)
	}
	raw, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if raw.Host != "first.example.com" || raw.EditRevision != updated.EditRevision {
		t.Fatalf("旧修订请求不应改变节点: %+v", raw)
	}
}

func TestExtensionOpsAddReplaceClear(t *testing.T) {
	svc, _, cfg := newTestService(t)
	ctx := context.Background()
	created, err := svc.CreateManual(ctx, CreateManualInput{
		Name: "扩展操作节点", Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"uuid": "extension-secret", "network": "tcp"},
		Extensions:   []ExtensionInput{{ID: "ext-old", Scope: "node", Targets: []string{"clash-yaml"}, Payload: "old-payload"}},
	})
	if err != nil {
		t.Fatalf("创建扩展操作节点失败: %v", err)
	}
	updated, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "vless", Host: "example.com", Port: 443, BaseRevision: created.EditRevision,
		ProtocolJSON: map[string]any{"uuid": "", "network": "tcp"},
		ExtensionOps: []ExtensionOp{
			{Op: "clear", ID: "ext-old"},
			{Op: "add", ID: "ext-new", Scope: "node", Targets: []string{"generic-subs"}, Payload: "new-payload"},
		},
	})
	if err != nil {
		t.Fatalf("扩展 add/clear 更新失败: %v", err)
	}
	if len(updated.Extensions) != 1 || updated.Extensions[0].ID != "ext-new" {
		t.Fatalf("扩展 add/clear 摘要异常: %+v", updated.Extensions)
	}
	newRaw, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	newRecord := newRaw.extensionRecords[0]
	key, err := cfg.Get(ctx, config.KeySigningKey)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := config.Decrypt(strings.TrimPrefix(newRecord.PayloadEnc, extEncPrefix), []byte(key))
	if err != nil || string(plain) != "new-payload" {
		t.Fatalf("扩展 add 负载解密异常: plain=%q err=%v", plain, err)
	}

	replaced, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "vless", Host: "example.com", Port: 443, BaseRevision: updated.EditRevision,
		ProtocolJSON: map[string]any{"uuid": "", "network": "tcp"},
		ExtensionOps: []ExtensionOp{{Op: "replace", ID: "ext-new", Payload: "replaced-payload"}},
	})
	if err != nil {
		t.Fatalf("扩展 replace 更新失败: %v", err)
	}
	if len(replaced.Extensions) != 1 || replaced.Extensions[0].ID != "ext-new" {
		t.Fatalf("扩展 replace 摘要异常: %+v", replaced.Extensions)
	}
	rawReplaced, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rawReplaced.extensionRecords[0].PayloadEnc == newRecord.PayloadEnc {
		t.Fatal("replace 应生成新的扩展密文")
	}
}

func TestExtensionResetScopeCannotKeep(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	created, err := svc.CreateManual(ctx, CreateManualInput{
		Name: "扩展重置节点", Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"uuid": "reset-secret", "network": "ws"},
		Extensions:   []ExtensionInput{{ID: "ext-reset", Scope: "transport.ws", Payload: "must-clear"}},
	})
	if err != nil {
		t.Fatalf("创建扩展重置节点失败: %v", err)
	}
	if _, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "vless", Host: "example.com", Port: 443, BaseRevision: created.EditRevision,
		ProtocolJSON: map[string]any{"uuid": "", "network": "tcp"},
		ResetScopes:  []string{"network"},
		ExtensionOps: []ExtensionOp{{Op: "keep", ID: "ext-reset"}},
	}); err != nil {
		t.Fatalf("执行扩展作用域重置失败: %v", err)
	}
	raw, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw.extensionRecords) != 0 {
		t.Fatalf("被重置作用域的扩展不应被 keep 保留: %+v", raw.extensionRecords)
	}
}

func TestCreateRejectsUnknownTopLevelField(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.CreateManual(context.Background(), CreateManualInput{
		Name: "未知顶层创建", Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{
			"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp", "future-top": "x",
		},
	})
	if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "future-top") {
		t.Fatalf("创建应拒绝未登记顶层字段: %v", err)
	}
}

func TestUpdateRejectsUnknownTopLevelFieldAndKeepsDatabase(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	created := createManual(t, svc, "未知顶层更新")
	before, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "vless", Host: "example.com", Port: 443, BaseRevision: created.EditRevision,
		ProtocolJSON: map[string]any{
			"uuid": "", "network": "tcp", "future-top": "x",
		},
	})
	if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "future-top") {
		t.Fatalf("更新应拒绝未登记顶层字段: %v", err)
	}
	after, err := svc.getRaw(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	beforeJSON, _ := json.Marshal(before.ProtocolJSON)
	afterJSON, _ := json.Marshal(after.ProtocolJSON)
	if string(beforeJSON) != string(afterJSON) || before.EditRevision != after.EditRevision {
		t.Fatalf("拒绝未知顶层字段后不应发生任何写入: before=%+v after=%+v", before, after)
	}
}

func TestCreateManualSecurityTLSOverridesLegacyTLS(t *testing.T) {
	svc, st, _ := newTestService(t)
	created, err := svc.CreateManual(context.Background(), CreateManualInput{
		Name: "安全归一化节点", Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{
			"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp",
			"security": "tls", "tls": false,
		},
	})
	if err != nil {
		t.Fatalf("创建带 security/tls 冲突节点失败: %v", err)
	}
	raw, err := svc.getRaw(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if raw.ProtocolJSON["tls"] != true {
		t.Fatalf("security=tls 应覆盖旧 tls=false 落库: %+v", raw.ProtocolJSON)
	}
	if _, ok := raw.ProtocolJSON["security"]; ok {
		t.Fatalf("表单层 security 不应落库: %+v", raw.ProtocolJSON)
	}
	var rawJSON string
	if err := st.DB().QueryRowContext(context.Background(),
		`SELECT protocol_json FROM nodes WHERE id=?`, created.ID).Scan(&rawJSON); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rawJSON, `"tls":true`) {
		t.Fatalf("库内应保存 tls=true: %s", rawJSON)
	}
}
