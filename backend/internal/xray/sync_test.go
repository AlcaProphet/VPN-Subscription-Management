package xray

import (
	"context"
	"sync"
	"testing"

	"github.com/xtls/xray-core/common/protocol"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/internal/tasks"
	"vpn-sub/migrations"
)

type fakeAPI struct {
	mu      sync.Mutex
	added   map[string]bool
	removed map[string]bool
	failAdd error
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{added: map[string]bool{}, removed: map[string]bool{}}
}

func (f *fakeAPI) AddUser(_ context.Context, tag string, u *protocol.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failAdd != nil {
		return f.failAdd
	}
	f.added[tag+"|"+u.Email] = true
	return nil
}

func (f *fakeAPI) RemoveUser(_ context.Context, tag, email string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed[tag+"|"+email] = true
	return nil
}

func newSyncTestEnv(t *testing.T) (*store.Store, *config.Service, *SyncService, *fakeAPI, int64) {
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
	// 组
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO groups (slug, name, is_default) VALUES ('group-default', '默认组', 1)`); err != nil {
		t.Fatal(err)
	}
	var groupID int64
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM groups WHERE slug='group-default'`).Scan(&groupID); err != nil {
		t.Fatal(err)
	}
	// 用户
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO users (username, email, role, user_source, status, group_id) VALUES ('u1','u1@x.com','user','local','active',?)`, groupID); err != nil {
		t.Fatal(err)
	}
	var userID int64
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM users WHERE email='u1@x.com'`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	// 平台/订阅/版本/蓝图
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO platforms (slug, name) VALUES ('plat','平台')`); err != nil {
		t.Fatal(err)
	}
	var platformID int64
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM platforms WHERE slug='plat'`).Scan(&platformID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO subscriptions (slug, name, platform_id, current_version) VALUES ('sub','订阅',?,1)`, platformID); err != nil {
		t.Fatal(err)
	}
	var subID int64
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM subscriptions WHERE slug='sub'`).Scan(&subID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO versions (owner_type, owner_id, version_no, file_path) VALUES ('subscription',?,1,'x.yaml')`, subID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO assembly_blueprints (version_id, target_syntax, selection_json) VALUES ((SELECT id FROM versions WHERE owner_type='subscription' AND owner_id=? AND version_no=1), 'clash-yaml', '{"xray_candidates":["node-a"]}')`, subID); err != nil {
		t.Fatal(err)
	}
	// 实例/节点
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO xray_instances (name, slug, api_addr) VALUES ('inst','instance-abc','127.0.0.1:10086')`); err != nil {
		t.Fatal(err)
	}
	var instID int64
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM xray_instances WHERE slug='instance-abc'`).Scan(&instID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO nodes (source, name, instance_id, tag, protocol, host, port, protocol_json, enabled, allocatable, missing)
		 VALUES ('xray','node-a',?,'in-a','vless','h',443,'{}',1,1,0)`, instID); err != nil {
		t.Fatal(err)
	}
	var nodeID int64
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM nodes WHERE name='node-a'`).Scan(&nodeID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO group_nodes (group_id, node_id, sort_order) VALUES (?,?,0)`, groupID, nodeID); err != nil {
		t.Fatal(err)
	}

	instSvc := NewInstanceService(st, log.New("error", "console"), tasks.NewRegistry())
	creds := NewCredentialService(st, cfg)
	syncSvc := NewSyncService(st, cfg, creds, instSvc, tasks.NewRegistry(), log.New("error", "console"))
	fake := newFakeAPI()
	syncSvc.SetAPIFactory(func(_ context.Context, _ int64) (API, error) { return fake, nil })
	return st, cfg, syncSvc, fake, userID
}

func TestSyncPushUser(t *testing.T) {
	_, _, syncSvc, fake, userID := newSyncTestEnv(t)
	ctx := context.Background()
	synced, failed, err := syncSvc.PushUser(ctx, userID)
	if err != nil {
		t.Fatalf("PushUser 失败: %v", err)
	}
	if synced != 1 || failed != 0 {
		t.Fatalf("计数异常 synced=%d failed=%d", synced, failed)
	}
	if len(fake.added) != 1 {
		t.Fatalf("fake 应收到 1 个 AddUser: %+v", fake.added)
	}
	// 幂等：再次推送仍成功（fake 不报 already exists 也视为成功）
	synced, failed, err = syncSvc.PushUser(ctx, userID)
	if err != nil || synced != 1 || failed != 0 {
		t.Fatalf("重复推送异常: %v %d %d", err, synced, failed)
	}
}

func TestSyncRemoveStale(t *testing.T) {
	_, _, syncSvc, fake, userID := newSyncTestEnv(t)
	ctx := context.Background()
	if _, _, err := syncSvc.PushUser(ctx, userID); err != nil {
		t.Fatal(err)
	}
	// 将节点移出候选集（清空蓝图候选），ReconcileUser 应移除 stale
	if _, err := syncSvc.store.DB().ExecContext(ctx,
		`UPDATE assembly_blueprints SET selection_json = '{"xray_candidates":[]}'`); err != nil {
		t.Fatal(err)
	}
	if err := syncSvc.ReconcileUser(ctx, userID); err != nil {
		t.Fatalf("ReconcileUser 失败: %v", err)
	}
	if len(fake.removed) != 1 {
		t.Fatalf("fake 应收到 RemoveUser: %+v", fake.removed)
	}
}
