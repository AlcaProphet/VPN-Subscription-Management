package xray

import (
	"context"
	"testing"

	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/internal/tasks"
	"vpn-sub/migrations"
)

func newTestInstanceService(t *testing.T) (*InstanceService, *store.Store) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), migrations.FS); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	svc := NewInstanceService(st, log.New("error", "console"), tasks.NewRegistry())
	return svc, st
}

func TestInstanceCRUD(t *testing.T) {
	svc, _ := newTestInstanceService(t)
	ctx := context.Background()
	inst, err := svc.Create(ctx, "东京一", "127.0.0.1:10086", "primary", true)
	if err != nil {
		t.Fatalf("创建实例失败: %v", err)
	}
	if inst.Slug == "" || inst.ID == 0 {
		t.Fatalf("实例字段异常: %+v", inst)
	}
	got, err := svc.Get(ctx, inst.ID)
	if err != nil {
		t.Fatalf("获取实例失败: %v", err)
	}
	if got.Name != "东京一" || got.APIAddr != "127.0.0.1:10086" || !got.Enabled {
		t.Fatalf("实例内容异常: %+v", got)
	}
	upd, err := svc.Update(ctx, inst.ID, "东京二", "example.com:443", "backup", false)
	if err != nil {
		t.Fatalf("更新实例失败: %v", err)
	}
	if upd.Name != "东京二" || upd.APIAddr != "example.com:443" || upd.Enabled {
		t.Fatalf("更新后实例异常: %+v", upd)
	}
	list, err := svc.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("列表异常: %v %+v", err, list)
	}
}

func TestInstanceNameConflict(t *testing.T) {
	svc, _ := newTestInstanceService(t)
	ctx := context.Background()
	if _, err := svc.Create(ctx, "same", "127.0.0.1:10086", "", true); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, "same", "127.0.0.1:10087", "", true); err == nil {
		t.Fatal("重名应返回冲突")
	}
}

func TestInstanceDeleteAsync(t *testing.T) {
	svc, st := newTestInstanceService(t)
	ctx := context.Background()
	inst, err := svc.Create(ctx, "del", "127.0.0.1:10086", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.deleteAndClean(ctx, inst.ID); err != nil {
		t.Fatalf("同步删除失败: %v", err)
	}
	var n int
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM xray_instances WHERE id = ?`, inst.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("实例未删除")
	}
}

func TestProtocolDetectionHelpers(t *testing.T) {
	cases := map[string]string{
		"type.googleapis.com/xray.proxy.vless.inbound.Config":     "vless",
		"type.googleapis.com/xray.proxy.vmess.inbound.Config":     "vmess",
		"type.googleapis.com/xray.proxy.trojan.ServerConfig":      "trojan",
		"type.googleapis.com/xray.proxy.shadowsocks.ServerConfig": "shadowsocks",
	}
	for typ, want := range cases {
		if got := protocolFromType(typ); got != want {
			t.Errorf("protocolFromType(%q) = %q, want %q", typ, got, want)
		}
	}
	if !isAllocatable("vless") || !isAllocatable("vmess") || !isAllocatable("trojan") || !isAllocatable("shadowsocks") {
		t.Error("四协议应 allocatable")
	}
	if isAllocatable("http") {
		t.Error("http 不应 allocatable")
	}
	if got := hostFromAddr("example.com:443"); got != "example.com" {
		t.Errorf("hostFromAddr = %q", got)
	}
}
