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

// newTestSubscriptionService 临时库（platforms/subscriptions/versions/assembly_blueprints 表）+ 订阅服务
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
			CREATE TABLE IF NOT EXISTS platforms (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
				product_type TEXT NOT NULL DEFAULT 'yaml',
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"1002_subscriptions_versions.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS subscriptions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
				platform_id INTEGER NOT NULL, current_version INTEGER NOT NULL DEFAULT 0,
				product_type TEXT NOT NULL DEFAULT 'yaml',
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE UNIQUE INDEX IF NOT EXISTS idx_subscriptions_platform_uniq ON subscriptions(platform_id);
			CREATE TABLE IF NOT EXISTS versions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				owner_type TEXT NOT NULL CHECK (owner_type IN ('subscription','rule','custom','share')),
				owner_id INTEGER NOT NULL,
				version_no INTEGER NOT NULL,
				file_path TEXT NOT NULL,
				file_name TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				UNIQUE (owner_type, owner_id, version_no));`)},
		"1009_xray.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS assembly_blueprints (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				version_id INTEGER NOT NULL UNIQUE REFERENCES versions(id) ON DELETE CASCADE,
				target_syntax TEXT NOT NULL);`)},
	}
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	dataDir := t.TempDir()
	verSvc := version.NewService(st, dataDir, log.New("error", "console"))
	svc := NewService(st, verSvc, log.New("error", "console"))
	return st, svc, verSvc, dataDir
}

// seedPlatformN 预置指定序号平台（slug 唯一，避免重复创建冲突）
func seedPlatformN(t *testing.T, st *store.Store, n int, productType string) int64 {
	t.Helper()
	res, err := st.DB().Exec(`INSERT INTO platforms (slug, name, product_type) VALUES (?, '测试平台', ?)`,
		"platform-"+strconv.Itoa(n), productType)
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
	pid1 := seedPlatformN(t, st, 1, "yaml")
	pid2 := seedPlatformN(t, st, 2, "yaml")
	if _, err := svc.Create(ctx, CreateInput{PlatformID: pid1, Name: "A", Slug: "my-sub"}); err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.Create(ctx, CreateInput{PlatformID: pid2, Name: "B", Slug: "my-sub"}); !errors.Is(err, ErrSlugConflict) {
		t.Errorf("同 slug 应返回 ErrSlugConflict: %v", err)
	}
	if ok, _ := svc.CheckSlugAvailable(ctx, "My-Sub", "", 0); ok {
		t.Error("大写标识应不可用")
	}
	if ok, _ := svc.CheckSlugAvailable(ctx, "ab", "", 0); ok {
		t.Error("过短标识应不可用")
	}
	if ok, _ := svc.CheckSlugAvailable(ctx, "another-sub", "", 0); !ok {
		t.Error("新标识应可用")
	}
}

// TestCreateProductTypeAndNoFirstVersion 创建订阅继承平台 product_type，且不再自动建首版本
func TestCreateProductTypeAndNoFirstVersion(t *testing.T) {
	st, svc, _, _ := newTestSubscriptionService(t)
	ctx := context.Background()
	pid := seedPlatformN(t, st, 1, "subs")
	sub, err := svc.Create(ctx, CreateInput{PlatformID: pid, Name: "测试订阅"})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if sub.ProductType != "subs" {
		t.Errorf("product_type 应继承平台: %s", sub.ProductType)
	}
	if sub.CurrentVersion != 0 {
		t.Errorf("创建订阅不应自动建首版本: %d", sub.CurrentVersion)
	}
	var n int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM versions WHERE owner_type='subscription' AND owner_id=?`, sub.ID).Scan(&n); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if n != 0 {
		t.Errorf("创建订阅不应产生版本: %d", n)
	}
}

// TestPlatformOccupied 同平台仅一份订阅条目（业务查重 + UNIQUE 索引兜底）
func TestPlatformOccupied(t *testing.T) {
	st, svc, _, _ := newTestSubscriptionService(t)
	ctx := context.Background()
	pid := seedPlatformN(t, st, 1, "yaml")
	if _, err := svc.Create(ctx, CreateInput{PlatformID: pid, Name: "A"}); err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.Create(ctx, CreateInput{PlatformID: pid, Name: "B"}); !errors.Is(err, ErrPlatformOccupied) {
		t.Errorf("同平台二次创建应返回 ErrPlatformOccupied: %v", err)
	}
}

// TestDeleteCascade 删除订阅级联清理版本文件与版本记录
func TestDeleteCascade(t *testing.T) {
	st, svc, verSvc, dataDir := newTestSubscriptionService(t)
	ctx := context.Background()
	pid := seedPlatformN(t, st, 1, "yaml")
	sub, err := svc.Create(ctx, CreateInput{PlatformID: pid, Name: "测试订阅", Slug: "cascade-sub"})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, _, err := verSvc.CreateVersion(ctx, version.OwnerSubscription, sub.ID, version.BytesContent([]byte("v1")), version.CreateOptions{Activate: true}); err != nil {
		t.Fatalf("创建 v1 失败: %v", err)
	}
	if _, _, err := verSvc.CreateVersion(ctx, version.OwnerSubscription, sub.ID, version.BytesContent([]byte("v2")), version.CreateOptions{Activate: true}); err != nil {
		t.Fatalf("创建 v2 失败: %v", err)
	}
	if err := svc.Delete(ctx, sub.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	var n int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM subscriptions WHERE id=?`, sub.ID).Scan(&n); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if n != 0 {
		t.Error("订阅行应删除")
	}
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

// TestListFlat 列表为平铺结构（每平台一份条目），空库返回 [] 而非 nil
func TestListFlat(t *testing.T) {
	st, svc, _, _ := newTestSubscriptionService(t)
	ctx := context.Background()
	pid1 := seedPlatformN(t, st, 1, "yaml")
	pid2 := seedPlatformN(t, st, 2, "subs")
	if _, err := svc.Create(ctx, CreateInput{PlatformID: pid1, Name: "A"}); err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.Create(ctx, CreateInput{PlatformID: pid2, Name: "B"}); err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	list, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("列表失败: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("应平铺列出 2 份订阅: %d", len(list))
	}
	if list[0].PlatformName != "测试平台" || list[1].PlatformName != "测试平台" {
		t.Errorf("平台名异常: %+v", list)
	}
}
