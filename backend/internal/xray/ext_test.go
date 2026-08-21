package xray

import (
	"context"
	"testing"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/internal/tasks"
	"vpn-sub/migrations"
)

// newExtTestEnv 构造独立账号服务测试环境（含一个可用 Xray 节点）。
func newExtTestEnv(t *testing.T) (*store.Store, *ExtService, *fakeAPI, int64, int64) {
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
	ctx := context.Background()
	if err := cfg.Set(ctx, config.KeySigningKey, "0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Set(ctx, config.KeyAdvancedMode, "true"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO xray_instances (name, slug, api_addr) VALUES ('inst','instance-abc','127.0.0.1:10086')`); err != nil {
		t.Fatal(err)
	}
	var instID int64
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM xray_instances WHERE slug='instance-abc'`).Scan(&instID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO nodes (source, name, instance_id, tag, protocol, host, port, protocol_json, enabled, allocatable, missing)
		 VALUES ('xray','instance-abc-in-a',?,'in-a','vless','h',443,'{}',1,1,0)`, instID); err != nil {
		t.Fatal(err)
	}
	var nodeID int64
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM nodes WHERE name='instance-abc-in-a'`).Scan(&nodeID); err != nil {
		t.Fatal(err)
	}
	instSvc := NewInstanceService(st, log.New("error", "console"), tasks.NewRegistry())
	extSvc := NewExtService(st, cfg, instSvc, log.New("error", "console"))
	fake := newFakeAPI()
	extSvc.SetAPIFactory(func(_ context.Context, _ int64) (API, error) { return fake, nil })
	return st, extSvc, fake, instID, nodeID
}

// TestExtQuotaExceedKeepsPushTarget 超限摘除不删除本地期望集，重置后可恢复推送。
func TestExtQuotaExceedKeepsPushTarget(t *testing.T) {
	st, extSvc, fake, instID, _ := newExtTestEnv(t)
	ctx := context.Background()
	quota := 0.000001 // 约 1MB，便于触发超限
	acc, _, err := extSvc.CreateExt(ctx, "ext1", "manual", "uuid-1", "secret-1", &quota, []ExtPushTarget{{InstanceID: instID, InboundTag: "in-a"}})
	if err != nil {
		t.Fatalf("创建独立账号失败: %v", err)
	}
	if len(fake.added) != 1 {
		t.Fatalf("创建后应推送 1 个目标: %+v", fake.added)
	}
	// 写入超过配额的当月流量
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO xray_ext_traffic (ext_account_id, ym, uplink, downlink) VALUES (?,?,1048576,0)`, acc.ID, currentYM()); err != nil {
		t.Fatal(err)
	}
	if err := extSvc.CheckExtQuota(ctx, acc.ID); err != nil {
		t.Fatalf("CheckExtQuota 失败: %v", err)
	}
	var exceeded int
	if err := st.DB().QueryRowContext(ctx, `SELECT quota_exceeded FROM xray_ext_accounts WHERE id=?`, acc.ID).Scan(&exceeded); err != nil {
		t.Fatal(err)
	}
	if exceeded != 1 {
		t.Fatalf("超限后 quota_exceeded 应为 1，实际 %d", exceeded)
	}
	// 关键断言：本地目标行保留
	var targetCount int
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM xray_ext_users WHERE ext_account_id=?`, acc.ID).Scan(&targetCount); err != nil {
		t.Fatal(err)
	}
	if targetCount != 1 {
		t.Fatalf("超限摘除不应删除本地推送目标，目标数=%d", targetCount)
	}
	if len(fake.removed) != 1 {
		t.Fatalf("超限后应从 Xray 移除 1 个目标: %+v", fake.removed)
	}
	// 重置后应恢复推送
	if err := extSvc.ResetExtQuota(ctx, acc.ID); err != nil {
		t.Fatalf("ResetExtQuota 失败: %v", err)
	}
	if err := st.DB().QueryRowContext(ctx, `SELECT quota_exceeded FROM xray_ext_accounts WHERE id=?`, acc.ID).Scan(&exceeded); err != nil {
		t.Fatal(err)
	}
	if exceeded != 0 {
		t.Fatalf("重置后 quota_exceeded 应为 0，实际 %d", exceeded)
	}
	var syncStatus string
	if err := st.DB().QueryRowContext(ctx, `SELECT sync_status FROM xray_ext_users WHERE ext_account_id=?`, acc.ID).Scan(&syncStatus); err != nil {
		t.Fatal(err)
	}
	if syncStatus != "synced" {
		t.Fatalf("重置后推送目标状态应为 synced，实际 %s", syncStatus)
	}
}

// TestExtManualAlreadyExistsAsSuccess manual 接管模式遇到 already exists 应视为成功。
func TestExtManualAlreadyExistsAsSuccess(t *testing.T) {
	_, extSvc, fake, instID, _ := newExtTestEnv(t)
	ctx := context.Background()
	fake.failAdd = errAlreadyExists
	quota := float64(0)
	acc, _, err := extSvc.CreateExt(ctx, "ext2", "manual", "uuid-2", "secret-2", &quota, []ExtPushTarget{{InstanceID: instID, InboundTag: "in-a"}})
	if err != nil {
		t.Fatalf("创建独立账号失败: %v", err)
	}
	var syncStatus, action string
	if err := extSvc.store.DB().QueryRowContext(ctx,
		`SELECT sync_status, action FROM xray_ext_users WHERE ext_account_id=?`, acc.ID).Scan(&syncStatus, &action); err != nil {
		t.Fatal(err)
	}
	if syncStatus != "synced" || action != "add" {
		t.Fatalf("manual already exists 应视为接管成功，实际 status=%s action=%s", syncStatus, action)
	}
	if len(fake.added) != 0 {
		t.Fatalf("already exists 场景不应计入新增: %+v", fake.added)
	}
}
