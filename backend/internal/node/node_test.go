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
	}
	if HasProtocol("ssr") {
		t.Error("HasProtocol(ssr) 应为 false")
	}
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
		ProtocolJSON: map[string]any{"uuid": "", "network": "ws"},
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
		ProtocolJSON: map[string]any{"uuid": "new-uuid"},
	})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("改名应 ErrBadRequest，实际 %v", err)
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

