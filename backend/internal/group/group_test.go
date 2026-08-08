package group

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
)

// testMigrateFS 构造含 users/groups/platforms/subscriptions/versions/group_selections 表的迁移集
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
				is_default INTEGER NOT NULL DEFAULT 0, needs_reselect INTEGER NOT NULL DEFAULT 0,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS platforms (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '', schemes TEXT NOT NULL DEFAULT '[]',
				extra_headers TEXT NOT NULL DEFAULT '{}', installer_file TEXT, installer_url TEXT,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"1002_subscriptions_versions.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS subscriptions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
				platform_id INTEGER NOT NULL, current_version INTEGER NOT NULL DEFAULT 0,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS versions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				owner_type TEXT NOT NULL CHECK (owner_type IN ('subscription','rule','custom','share')),
				owner_id INTEGER NOT NULL, version_no INTEGER NOT NULL, file_path TEXT NOT NULL,
								file_name TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				UNIQUE (owner_type, owner_id, version_no));
			CREATE TABLE IF NOT EXISTS subscription_group_rel (
				subscription_id INTEGER NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
				group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
				PRIMARY KEY (subscription_id, group_id));`)},
		"1003_groups.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS group_selections (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
				platform_id INTEGER NOT NULL REFERENCES platforms(id) ON DELETE CASCADE,
				subscription_id INTEGER REFERENCES subscriptions(id) ON DELETE SET NULL,
				UNIQUE (group_id, platform_id));`)},
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

// seedDefaultGroup 预置默认组与一个普通组，返回 ID
func seedGroups(t *testing.T, st *store.Store) (defaultID, normalID int64) {
	t.Helper()
	res, err := st.DB().Exec(`INSERT INTO groups (slug, name, is_default) VALUES ('group-default', '默认组', 1)`)
	if err != nil {
		t.Fatalf("创建默认组失败: %v", err)
	}
	defaultID, _ = res.LastInsertId()
	res, err = st.DB().Exec(`INSERT INTO groups (slug, name) VALUES ('group-normal', '普通组')`)
	if err != nil {
		t.Fatalf("创建普通组失败: %v", err)
	}
	normalID, _ = res.LastInsertId()
	return defaultID, normalID
}

// seedSubscription 预置平台与订阅，返回 (platformID, subscriptionID)；平台 slug 随订阅独立生成避免唯一冲突
func seedSubscription(t *testing.T, st *store.Store, slug string) (int64, int64) {
	t.Helper()
	res, err := st.DB().Exec(`INSERT INTO platforms (slug, name) VALUES (?, '测试平台')`, "platform-"+slug)
	if err != nil {
		t.Fatalf("创建平台失败: %v", err)
	}
	pid, _ := res.LastInsertId()
	res, err = st.DB().Exec(`INSERT INTO subscriptions (slug, name, platform_id) VALUES (?, '测试订阅', ?)`, slug, pid)
	if err != nil {
		t.Fatalf("创建订阅失败: %v", err)
	}
	sid, _ := res.LastInsertId()
	return pid, sid
}

// TestNameUnique 组名唯一：同名创建/改名 409
func TestNameUnique(t *testing.T) {
	_, svc := newTestGroupService(t)
	ctx := context.Background()
	if _, err := svc.Create(ctx, "研发组"); err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.Create(ctx, "研发组"); !errors.Is(err, ErrNameConflict) {
		t.Errorf("同名创建应返回 ErrNameConflict: %v", err)
	}
	// 改名冲突（排除自身）
	g, err := svc.Create(ctx, "测试组")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if err := svc.Update(ctx, g.ID, "研发组", nil, nil); !errors.Is(err, ErrNameConflict) {
		t.Errorf("改名冲突应返回 ErrNameConflict: %v", err)
	}
	// 自身改名（同名）不冲突
	if err := svc.Update(ctx, g.ID, "测试组", nil, nil); err != nil {
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

// TestUnlinkSelectedSubscriptionRejected 取消关联时选定校验：组正选定订阅 A，移除 A 的关联 → 拒绝；
// 先在选定区改选后再移除 → 成功
func TestUnlinkSelectedSubscriptionRejected(t *testing.T) {
	st, svc := newTestGroupService(t)
	ctx := context.Background()
	_, normalID := seedGroups(t, st)
	pid, sidA := seedSubscription(t, st, "sub-a")
	_, sidB := seedSubscription(t, st, "sub-b")
	// 关联 A/B 并选定 A
	if err := svc.Update(ctx, normalID, "普通组", []int64{sidA, sidB}, []Selection{{PlatformID: pid, SubscriptionID: sidA}}); err != nil {
		t.Fatalf("关联+选定失败: %v", err)
	}
	// 移除 A 的关联（仍选定 A）→ 拒绝
	if err := svc.Update(ctx, normalID, "普通组", []int64{sidB}, []Selection{{PlatformID: pid, SubscriptionID: sidA}}); !errors.Is(err, ErrSubInSelection) {
		t.Errorf("移除正被选定的订阅应拒绝: %v", err)
	}
	// 先改选 B 再移除 A → 成功
	if err := svc.Update(ctx, normalID, "普通组", []int64{sidB}, []Selection{{PlatformID: pid, SubscriptionID: sidB}}); err != nil {
		t.Fatalf("改选后移除应成功: %v", err)
	}
	var linked int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM subscription_group_rel WHERE group_id=? AND subscription_id=?`, normalID, sidA).Scan(&linked); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if linked != 0 {
		t.Error("A 的关联应已移除")
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
	var residual int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM users WHERE group_id = ?`, normalID).Scan(&residual); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if residual != 0 {
		t.Error("原组不应残留用户")
	}
}

// TestOnSubscriptionDeletedSetsReselect 删订阅置 needs_reselect：选定清空、标记置 1；重新选定后清除
func TestOnSubscriptionDeletedSetsReselect(t *testing.T) {
	st, svc := newTestGroupService(t)
	ctx := context.Background()
	_, normalID := seedGroups(t, st)
	pid, sid := seedSubscription(t, st, "sub-del")
	if err := svc.Update(ctx, normalID, "普通组", []int64{sid}, []Selection{{PlatformID: pid, SubscriptionID: sid}}); err != nil {
		t.Fatalf("关联+选定失败: %v", err)
	}
	// 直接调用 OnSubscriptionDeleted（模拟 subscription.Delete 事务内调用）
	if err := st.TxImmediate(ctx, func(tx *sql.Tx) error {
		return svc.OnSubscriptionDeleted(ctx, tx, sid)
	}); err != nil {
		t.Fatalf("级联失败: %v", err)
	}
	// 选定清空
	var sel int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM group_selections WHERE subscription_id = ?`, sid).Scan(&sel); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if sel != 0 {
		t.Error("选定应清空")
	}
	// needs_reselect 置 1
	var flag int
	if err := st.DB().QueryRow(`SELECT needs_reselect FROM groups WHERE id = ?`, normalID).Scan(&flag); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if flag != 1 {
		t.Errorf("受影响组应置 needs_reselect: %d", flag)
	}
	// 重新选定后清除（新建订阅替代）
	_, sid2 := seedSubscription(t, st, "sub-new")
	if err := svc.Update(ctx, normalID, "普通组", []int64{sid2}, []Selection{{PlatformID: pid, SubscriptionID: sid2}}); err != nil {
		t.Fatalf("重新选定失败: %v", err)
	}
	if err := st.DB().QueryRow(`SELECT needs_reselect FROM groups WHERE id = ?`, normalID).Scan(&flag); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if flag != 0 {
		t.Errorf("重新选定后标记应清除: %d", flag)
	}
}

// TestUniqueSelectionPerPlatform 每组每平台唯一选定：重复 UPSERT 不产生多行
func TestUniqueSelectionPerPlatform(t *testing.T) {
	st, svc := newTestGroupService(t)
	ctx := context.Background()
	_, normalID := seedGroups(t, st)
	pid, sid := seedSubscription(t, st, "sub-unique")
	// 先关联订阅（选定必须来自关联范围）
	if err := svc.Update(ctx, normalID, "普通组", []int64{sid}, nil); err != nil {
		t.Fatalf("关联失败: %v", err)
	}
	// 同组同平台两次选定（第一次选 sid，第二次重新选定同平台不同订阅）
	if err := svc.SetSelections(ctx, normalID, []Selection{{PlatformID: pid, SubscriptionID: sid}}); err != nil {
		t.Fatalf("首次选定失败: %v", err)
	}
	_, sid2 := seedSubscription(t, st, "sub-unique-2")
	if err := svc.Update(ctx, normalID, "普通组", []int64{sid, sid2}, nil); err != nil {
		t.Fatalf("关联 sid2 失败: %v", err)
	}
	if err := svc.SetSelections(ctx, normalID, []Selection{{PlatformID: pid, SubscriptionID: sid2}}); err != nil {
		t.Fatalf("二次选定失败: %v", err)
	}
	var rows int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM group_selections WHERE group_id = ? AND platform_id = ?`, normalID, pid).Scan(&rows); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if rows != 1 {
		t.Errorf("每组每平台应仅一行选定: %d", rows)
	}
	var current int64
	if err := st.DB().QueryRow(`SELECT subscription_id FROM group_selections WHERE group_id = ? AND platform_id = ?`, normalID, pid).Scan(&current); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if current != sid2 {
		t.Errorf("选定应更新为最新: %d", current)
	}
}
