package subscription

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/internal/version"
)

// newTestSubscriptionService 临时库（含 groups/platforms/subscriptions/versions 表）+ 订阅服务
func newTestSubscriptionService(t *testing.T) (*store.Store, *Service, *version.Service, string) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	fsys := fstest.MapFS{
		"0001_init.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY, value TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
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
				owner_id INTEGER NOT NULL,
				version_no INTEGER NOT NULL,
				file_path TEXT NOT NULL,
								file_name TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				UNIQUE (owner_type, owner_id, version_no));
			CREATE INDEX IF NOT EXISTS idx_versions_owner ON versions(owner_type, owner_id, version_no);
			CREATE TABLE IF NOT EXISTS subscription_group_rel (
				subscription_id INTEGER NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
				group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
				PRIMARY KEY (subscription_id, group_id));`)},
	}
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	dataDir := t.TempDir()
	verSvc := version.NewService(st, dataDir, log.New("error", "console"))
	svc := NewService(st, verSvc, log.New("error", "console"))
	return st, svc, verSvc, dataDir
}

// seedPlatform 预置一个平台，返回 ID
func seedPlatform(t *testing.T, st *store.Store) int64 {
	t.Helper()
	res, err := st.DB().Exec(`INSERT INTO platforms (slug, name) VALUES ('platform-test', '测试平台')`)
	if err != nil {
		t.Fatalf("创建平台失败: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// TestSlugCrossCheck 四类标识交叉校验：同 slug 二次创建 → ErrSlugConflict；格式非法 → ErrBadRequest
func TestSlugCrossCheck(t *testing.T) {
	st, svc, _, _ := newTestSubscriptionService(t)
	ctx := context.Background()
	pid := seedPlatform(t, st)
	if _, err := svc.Create(ctx, CreateInput{PlatformID: pid, Name: "A", Slug: "my-sub"}); err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.Create(ctx, CreateInput{PlatformID: pid, Name: "B", Slug: "my-sub"}); !errors.Is(err, ErrSlugConflict) {
		t.Errorf("同 slug 应返回 ErrSlugConflict: %v", err)
	}
	// 格式非法（大写/过短）→ 不可用
	if ok, _ := svc.CheckSlugAvailable(ctx, "My-Sub", "", 0); ok {
		t.Error("大写标识应不可用")
	}
	if ok, _ := svc.CheckSlugAvailable(ctx, "ab", "", 0); ok {
		t.Error("过短标识应不可用")
	}
	// 新标识可用
	if ok, _ := svc.CheckSlugAvailable(ctx, "another-sub", "", 0); !ok {
		t.Error("新标识应可用")
	}
}

// TestCreateWithGroupsAndVersion 创建订阅（关联组 + 首版本）→ 关联与版本均落库
func TestCreateWithGroupsAndVersion(t *testing.T) {
	st, svc, _, _ := newTestSubscriptionService(t)
	ctx := context.Background()
	pid := seedPlatform(t, st)
	res, err := st.DB().Exec(`INSERT INTO groups (slug, name, is_default) VALUES ('group-1', '默认组', 1)`)
	if err != nil {
		t.Fatalf("创建组失败: %v", err)
	}
	gid, _ := res.LastInsertId()
	sub, err := svc.Create(ctx, CreateInput{
		PlatformID: pid, Name: "测试订阅", Slug: "test-sub",
		GroupIDs:     []int64{gid},
		FirstContent: version.BytesContent([]byte("proxies: []")),
	})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if sub.CurrentVersion != 1 {
		t.Errorf("首版本应切换为当前: got %d", sub.CurrentVersion)
	}
	// 关联组写入
	var rel int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM subscription_group_rel WHERE subscription_id=?`, sub.ID).Scan(&rel); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if rel != 1 {
		t.Errorf("关联组应写入: %d", rel)
	}
	// 编辑回显
	got, err := svc.Get(ctx, sub.ID)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if len(got.Groups) != 1 || got.Groups[0].ID != gid {
		t.Errorf("关联组回显异常: %+v", got.Groups)
	}
}

// TestDeleteCascade 删除订阅级联清理版本文件与关联
func TestDeleteCascade(t *testing.T) {
	st, svc, verSvc, dataDir := newTestSubscriptionService(t)
	ctx := context.Background()
	pid := seedPlatform(t, st)
	sub, err := svc.Create(ctx, CreateInput{
		PlatformID: pid, Name: "测试订阅", Slug: "cascade-sub",
		FirstContent: version.BytesContent([]byte("v1")),
	})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	// 再建一版（验证多版本清理）
	if _, err := verSvc.CreateVersion(ctx, version.OwnerSubscription, sub.ID, version.BytesContent([]byte("v2"))); err != nil {
		t.Fatalf("创建 v2 失败: %v", err)
	}
	if err := svc.Delete(ctx, sub.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	// 订阅行删除
	var n int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM subscriptions WHERE id=?`, sub.ID).Scan(&n); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if n != 0 {
		t.Error("订阅行应删除")
	}
	// 版本记录与文件清理
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM versions WHERE owner_type='subscription' AND owner_id=?`, sub.ID).Scan(&n); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if n != 0 {
		t.Error("版本记录应删除")
	}
	dir := filepath.Join(dataDir, "contents", "subscription", strconv.FormatInt(sub.ID, 10))
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("版本目录应删除: %v", err)
	}
}

// TestListGrouped 按平台分组列表
func TestListEmpty(t *testing.T) {
	_, svc, _, _ := newTestSubscriptionService(t)
	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("列表失败: %v", err)
	}
	// 空库必须返回空数组（[]）而非 nil：否则 JSON 序列化为 null，前端 .map 崩溃（Issue1 R02-01）
	if list == nil {
		t.Fatal("空库必须返回空数组（[]）而非 nil")
	}
	if len(list) != 0 {
		t.Fatalf("空库应返回空列表: %+v", list)
	}
}

func TestListGrouped(t *testing.T) {
	st, svc, _, _ := newTestSubscriptionService(t)
	ctx := context.Background()
	pid := seedPlatform(t, st)
	if _, err := svc.Create(ctx, CreateInput{PlatformID: pid, Name: "A", Slug: "grouped-a"}); err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.Create(ctx, CreateInput{PlatformID: pid, Name: "B", Slug: "grouped-b"}); err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	list, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("列表失败: %v", err)
	}
	if len(list) != 1 || len(list[0].Subscriptions) != 2 {
		t.Errorf("应按平台分组且含 2 份订阅: %+v", list)
	}
	if list[0].PlatformName != "测试平台" {
		t.Errorf("平台名异常: %s", list[0].PlatformName)
	}
}

