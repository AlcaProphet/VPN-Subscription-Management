package proxygroup

import (
	"context"
	"errors"
	"testing"

	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/migrations"
)

func newTestService(t *testing.T) (*Service, *store.Store) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), migrations.FS); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	svc := NewService(st, log.New("error", "console"))
	return svc, st
}

func insertNode(t *testing.T, st *store.Store, name string) {
	t.Helper()
	if _, err := st.DB().ExecContext(context.Background(),
		`INSERT INTO nodes (source, name, protocol, host, port, protocol_json) VALUES ('manual', ?, 'vless', 'example.com', 443, '{}')`, name); err != nil {
		t.Fatalf("插入节点失败: %v", err)
	}
}

func TestListPresetSeeds(t *testing.T) {
	svc, _ := newTestService(t)
	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("读取代理组列表失败: %v", err)
	}
	if len(list) != 9 {
		t.Fatalf("预设种子应为 9 组，实际 %d", len(list))
	}
	for _, g := range list {
		if g.Type != "preset" || !g.Enabled {
			t.Fatalf("预设种子应 type=preset enabled=true: %+v", g)
		}
	}
}

func TestCreateCustomSuccess(t *testing.T) {
	svc, st := newTestService(t)
	insertNode(t, st, "节点A")
	g, err := svc.CreateCustom(context.Background(), "自建组", "select", Definition{
		Nodes:  []string{"节点A"},
		Groups: []string{"🚀直接连接"},
	})
	if err != nil {
		t.Fatalf("创建自建组失败: %v", err)
	}
	if g.Name != "自建组" || g.Type != "custom" || !g.Enabled {
		t.Fatalf("创建结果异常: %+v", g)
	}
}

func TestCreateCustomNameConflicts(t *testing.T) {
	svc, st := newTestService(t)
	insertNode(t, st, "节点A")
	// 与节点有效渲染名冲突
	_, err := svc.CreateCustom(context.Background(), "节点A", "select", Definition{Groups: []string{"🚀直接连接"}})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("与节点名冲突应 ErrConflict，实际 %v", err)
	}
	// 与预设组名冲突
	_, err = svc.CreateCustom(context.Background(), "🎬YouTube", "select", Definition{Groups: []string{"🚀直接连接"}})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("与预设组名冲突应 ErrConflict，实际 %v", err)
	}
	// 与强制组名冲突
	_, err = svc.CreateCustom(context.Background(), "🚀直接连接", "select", Definition{Groups: []string{"🚀直接连接"}})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("与强制组名冲突应 ErrConflict，实际 %v", err)
	}
	// 与内建保留名冲突
	_, err = svc.CreateCustom(context.Background(), "DIRECT", "select", Definition{Groups: []string{"🚀直接连接"}})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("与内建保留名冲突应 ErrConflict，实际 %v", err)
	}
}

func TestValidateReferencesAndContent(t *testing.T) {
	svc, st := newTestService(t)
	insertNode(t, st, "节点A")
	// 引用不存在子组
	_, err := svc.CreateCustom(context.Background(), "组B", "select", Definition{Groups: []string{"不存在组"}})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("引用不存在子组应 ErrBadRequest，实际 %v", err)
	}
	// 内容为空
	_, err = svc.CreateCustom(context.Background(), "组C", "select", Definition{})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("内容为空应 ErrBadRequest，实际 %v", err)
	}
	// 至少一个子组即可：自定义子组在装配时再选节点
	if _, err := svc.CreateCustom(context.Background(), "底层组", "select", Definition{Groups: []string{"🚀直接连接"}}); err != nil {
		t.Fatalf("创建底层组失败: %v", err)
	}
	if _, err := svc.CreateCustom(context.Background(), "上层组", "select", Definition{Groups: []string{"底层组"}}); err != nil {
		t.Fatalf("只含自定义子组应允许创建: %v", err)
	}
}

func TestDAGCycle(t *testing.T) {
	svc, st := newTestService(t)
	insertNode(t, st, "节点A")
	if _, err := svc.CreateCustom(context.Background(), "组A", "select", Definition{Groups: []string{"🚀直接连接"}}); err != nil {
		t.Fatalf("创建组A失败: %v", err)
	}
	if _, err := svc.CreateCustom(context.Background(), "组B", "select", Definition{Groups: []string{"🚀直接连接"}}); err != nil {
		t.Fatalf("创建组B失败: %v", err)
	}
	// A -> B
	list, _ := svc.List(context.Background())
	var idA, idB int64
	for _, g := range list {
		if g.Name == "组A" {
			idA = g.ID
		}
		if g.Name == "组B" {
			idB = g.ID
		}
	}
	if _, err := svc.Update(context.Background(), idA, "select", Definition{Groups: []string{"组B"}}); err != nil {
		t.Fatalf("更新组A引用组B失败: %v", err)
	}
	// B -> A 形成环
	_, err := svc.Update(context.Background(), idB, "select", Definition{Groups: []string{"组A"}})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("环应 ErrBadRequest，实际 %v", err)
	}
	// 自环
	_, err = svc.Update(context.Background(), idA, "select", Definition{Groups: []string{"组A"}})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("自环应 ErrBadRequest，实际 %v", err)
	}
}

func TestGroupTypeUpdateAndPreset(t *testing.T) {
	svc, st := newTestService(t)
	insertNode(t, st, "节点A")
	g, err := svc.CreateCustom(context.Background(), "类型组", "select", Definition{Groups: []string{"🚀直接连接"}})
	if err != nil {
		t.Fatalf("创建类型组失败: %v", err)
	}
	// 类型可修改
	g, err = svc.Update(context.Background(), g.ID, "url-test", Definition{Groups: []string{"🚀直接连接"}})
	if err != nil {
		t.Fatalf("修改组类型失败: %v", err)
	}
	if g.Definition.GroupType != "url-test" {
		t.Fatalf("组类型未更新: %+v", g.Definition)
	}
	// 非法类型
	_, err = svc.Update(context.Background(), g.ID, "invalid", Definition{Groups: []string{"🚀直接连接"}})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("非法类型应 ErrBadRequest，实际 %v", err)
	}
	// 预设不可删
	list, _ := svc.List(context.Background())
	var presetID int64
	for _, item := range list {
		if item.Type == "preset" {
			presetID = item.ID
			break
		}
	}
	if err := svc.Delete(context.Background(), presetID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("删除预设组应 ErrForbidden，实际 %v", err)
	}
	// 预设可切换启用
	pg, err := svc.SetPresetEnabled(context.Background(), presetID, false)
	if err != nil {
		t.Fatalf("停用预设组失败: %v", err)
	}
	if pg.Enabled {
		t.Fatal("预设组应已停用")
	}
}

// TestRejectForceFallbackSubgroup 代理组不允许引用「🛟无法归属的流量」作为子组
func TestRejectForceFallbackSubgroup(t *testing.T) {
	svc, st := newTestService(t)
	insertNode(t, st, "节点A")
	_, err := svc.CreateCustom(context.Background(), "兜底引用组", "select", Definition{
		Groups: []string{"🛟无法归属的流量"},
	})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("引用兜底组应 ErrBadRequest，实际 %v", err)
	}
}

