package group

import (
	"context"
	"errors"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
)

// testMigrateFS 构造含 users/groups/group_nodes 表的新模型迁移集
func testMigrateFS() fstest.MapFS {
	return fstest.MapFS{
		"0001_init.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY, value TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"0002_users.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT, oidc_subject TEXT UNIQUE,
			username TEXT NOT NULL, email TEXT UNIQUE,
			role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin','user')),
			group_id INTEGER, password_hash TEXT,
			user_source TEXT NOT NULL CHECK (user_source IN ('oidc','local','selfreg')),
			status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','active','disabled')),
			credential_version INTEGER NOT NULL DEFAULT 0, oidc_claims TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"0003_groups_platforms.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS groups (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL UNIQUE,
				is_default INTEGER NOT NULL DEFAULT 0,
				default_quota REAL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS platforms (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL);`)},
		"1009_xray.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS nodes (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				source TEXT NOT NULL, name TEXT NOT NULL UNIQUE,
				instance_id INTEGER);
			CREATE TABLE IF NOT EXISTS group_nodes (
				group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
				node_id INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
				sort_order INTEGER NOT NULL DEFAULT 0,
				PRIMARY KEY (group_id, node_id));`)},
	}
}

// newTestGroupService 临时库 + 组服务
func newTestGroupService(t *testing.T) (*store.Store, *Service) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), testMigrateFS()); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	svc := NewService(st, log.New("error", "console"))
	return st, svc
}

// seedGroups 预置默认组与一个普通组，返回 ID
func seedGroups(t *testing.T, st *store.Store) (defaultID, normalID int64) {
	t.Helper()
	res, err := st.DB().Exec(`INSERT INTO groups (slug, name, is_default) VALUES ('group-default', '默认组', 1)`)
	if err != nil {
		t.Fatalf("创建默认组失败: %v", err)
	}
	defaultID, _ = res.LastInsertId()
	res, err = st.DB().Exec(`INSERT INTO groups (slug, name, is_default, default_quota) VALUES ('group-normal', '普通组', 0, 100.5)`)
	if err != nil {
		t.Fatalf("创建普通组失败: %v", err)
	}
	normalID, _ = res.LastInsertId()
	return defaultID, normalID
}

// TestNameUnique 组名唯一：同名创建/改名冲突，改名同名自身成功
func TestNameUnique(t *testing.T) {
	_, svc := newTestGroupService(t)
	ctx := context.Background()
	if _, err := svc.Create(ctx, "研发组"); err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.Create(ctx, "研发组"); !errors.Is(err, ErrNameConflict) {
		t.Errorf("同名创建应返回 ErrNameConflict: %v", err)
	}
	g, err := svc.Create(ctx, "测试组")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if err := svc.Update(ctx, g.ID, "研发组"); !errors.Is(err, ErrNameConflict) {
		t.Errorf("改名冲突应返回 ErrNameConflict: %v", err)
	}
	if err := svc.Update(ctx, g.ID, "测试组"); err != nil {
		t.Errorf("改名同名自身应成功: %v", err)
	}
}

// TestDefaultGroupNotDeletable 默认组不可删
func TestDefaultGroupNotDeletable(t *testing.T) {
	st, svc := newTestGroupService(t)
	ctx := context.Background()
	defaultID, _ := seedGroups(t, st)
	if err := svc.Delete(ctx, defaultID); !errors.Is(err, ErrDefaultGroup) {
		t.Errorf("默认组删除应拒绝: %v", err)
	}
}

// TestDeleteGroupMigratesUsers 删组迁入默认组：组内用户 group_id 全部指向默认组
func TestDeleteGroupMigratesUsers(t *testing.T) {
	st, svc := newTestGroupService(t)
	ctx := context.Background()
	defaultID, normalID := seedGroups(t, st)
	for i := 0; i < 2; i++ {
		if _, err := st.DB().Exec(
			`INSERT INTO users (username, email, group_id, user_source, status) VALUES (?,?,?, 'local', 'active')`,
			"user"+string(rune('a'+i)), "u"+string(rune('a'+i))+"@x.com", normalID); err != nil {
			t.Fatalf("创建用户失败: %v", err)
		}
	}
	if err := svc.Delete(ctx, normalID); err != nil {
		t.Fatalf("删除组失败: %v", err)
	}
	var migrated int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM users WHERE group_id = ?`, defaultID).Scan(&migrated); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if migrated != 2 {
		t.Errorf("组内用户应迁入默认组: %d", migrated)
	}
}

// TestListAndGetNewModel 列表/详情返回 default_quota、node_count 与 user_count（Build4 新模型）
func TestListAndGetNewModel(t *testing.T) {
	st, svc := newTestGroupService(t)
	ctx := context.Background()
	defaultID, normalID := seedGroups(t, st)
	if _, err := st.DB().Exec(`INSERT INTO nodes (id, source, name) VALUES (1, 'xray', 'node-1')`); err != nil {
		t.Fatalf("创建节点失败: %v", err)
	}
	if _, err := st.DB().Exec(`INSERT INTO group_nodes (group_id, node_id, sort_order) VALUES (?, 1, 0)`, normalID); err != nil {
		t.Fatalf("创建组分配失败: %v", err)
	}
	if _, err := st.DB().Exec(`INSERT INTO users (username, email, group_id, user_source, status) VALUES ('u1','u1@x.com',?, 'local','active')`, normalID); err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	g, err := svc.Get(ctx, normalID)
	if err != nil {
		t.Fatalf("读取组失败: %v", err)
	}
	if g.DefaultQuota == nil || *g.DefaultQuota != 100.5 {
		t.Errorf("default_quota 回显异常: %+v", g.DefaultQuota)
	}
	if g.NodeCount != 1 {
		t.Errorf("node_count 应为 1: %d", g.NodeCount)
	}
	if g.UserCount != 1 {
		t.Errorf("user_count 应为 1: %d", g.UserCount)
	}

	dg, err := svc.Get(ctx, defaultID)
	if err != nil {
		t.Fatalf("读取默认组失败: %v", err)
	}
	if dg.DefaultQuota != nil {
		t.Errorf("默认组 default_quota 应为 NULL: %+v", dg.DefaultQuota)
	}

	list, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("列表失败: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("应列出 2 组: %d", len(list))
	}
}
