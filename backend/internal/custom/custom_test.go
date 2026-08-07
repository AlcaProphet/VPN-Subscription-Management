package custom

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
	"vpn-sub/internal/token"
	"vpn-sub/internal/version"
)

// testMigrateFS 构造含 users/platforms/custom_subscriptions/versions/download_tokens 表的迁移集
func testMigrateFS() fstest.MapFS {
	return fstest.MapFS{
		"0001_init.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY, value TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"0003_groups_platforms.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS platforms (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '', schemes TEXT NOT NULL DEFAULT '[]',
				extra_headers TEXT NOT NULL DEFAULT '{}', installer_file TEXT, installer_url TEXT,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"1002_subscriptions_versions.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS versions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				owner_type TEXT NOT NULL CHECK (owner_type IN ('subscription','rule','custom','share')),
				owner_id INTEGER NOT NULL, version_no INTEGER NOT NULL, file_path TEXT NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				UNIQUE (owner_type, owner_id, version_no));
			CREATE TABLE IF NOT EXISTS subscriptions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
				platform_id INTEGER NOT NULL, current_version INTEGER NOT NULL DEFAULT 0,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"1004_tokens.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS download_tokens (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				token TEXT NOT NULL UNIQUE,
				user_id INTEGER NOT NULL,
				platform_id INTEGER NOT NULL,
				custom_sub_id INTEGER,
				subscription_id INTEGER,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"1005_custom_share.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS custom_subscriptions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE,
				user_id INTEGER NOT NULL,
				platform_id INTEGER NOT NULL,
				current_version INTEGER NOT NULL DEFAULT 0,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				UNIQUE (user_id, platform_id));`)},
	}
}

// newTestCustomService 临时库 + 自定义服务（含版本/Token 服务）
func newTestCustomService(t *testing.T) (*store.Store, *Service, *version.Service, *token.Service, string) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), testMigrateFS()); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	dataDir := t.TempDir()
	ver := version.NewService(st, dataDir, log.New("error", "console"))
	tk := token.NewService(st, log.New("error", "console"))
	svc := NewService(st, ver, tk, log.New("error", "console"))
	return st, svc, ver, tk, dataDir
}

// TestUpsertReuse 覆盖复用：同用户同平台二次 Upsert → 记录 ID 与 slug 不变，版本号 +1
func TestUpsertReuse(t *testing.T) {
	st, svc, _, _, _ := newTestCustomService(t)
	ctx := context.Background()
	if _, err := st.DB().Exec(`INSERT INTO platforms (slug, name) VALUES ('platform-1', '平台')`); err != nil {
		t.Fatalf("创建平台失败: %v", err)
	}
	c1, err := svc.Upsert(ctx, 1, 1, version.BytesContent([]byte("v1")))
	if err != nil {
		t.Fatalf("首次上传失败: %v", err)
	}
	c2, err := svc.Upsert(ctx, 1, 1, version.BytesContent([]byte("v2")))
	if err != nil {
		t.Fatalf("覆盖上传失败: %v", err)
	}
	if c1.ID != c2.ID || c1.Slug != c2.Slug {
		t.Errorf("覆盖应复用记录与标识: %+v vs %+v", c1, c2)
	}
	if c2.CurrentVersion != 2 {
		t.Errorf("覆盖后版本号应为 2: %d", c2.CurrentVersion)
	}
	var count int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM custom_subscriptions`).Scan(&count); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if count != 1 {
		t.Errorf("每用户每平台应仅一份记录: %d", count)
	}
}

// TestUpsertDeletesGroupToken 覆盖删无标识 Token：旧组解析链接立即失效
func TestUpsertDeletesGroupToken(t *testing.T) {
	st, svc, _, tk, _ := newTestCustomService(t)
	ctx := context.Background()
	if _, err := st.DB().Exec(`INSERT INTO platforms (slug, name) VALUES ('platform-1', '平台')`); err != nil {
		t.Fatalf("创建平台失败: %v", err)
	}
	groupTk, err := tk.GetOrCreateUserToken(ctx, 1, 1, 0, 0)
	if err != nil {
		t.Fatalf("创建无标识 Token 失败: %v", err)
	}
	if _, err := svc.Upsert(ctx, 1, 1, version.BytesContent([]byte("v1"))); err != nil {
		t.Fatalf("上传失败: %v", err)
	}
	if _, err := tk.FindByToken(ctx, groupTk.Token); err != token.ErrTokenNotFound {
		t.Errorf("覆盖后无标识 Token 应失效: %v", err)
	}
}

// TestDeleteCascade 删自定义级联：Token + 版本文件清理；之后可重新上传（新标识）
func TestDeleteCascade(t *testing.T) {
	st, svc, _, tk, dataDir := newTestCustomService(t)
	ctx := context.Background()
	if _, err := st.DB().Exec(`INSERT INTO platforms (slug, name) VALUES ('platform-1', '平台')`); err != nil {
		t.Fatalf("创建平台失败: %v", err)
	}
	c, err := svc.Upsert(ctx, 1, 1, version.BytesContent([]byte("v1")))
	if err != nil {
		t.Fatalf("上传失败: %v", err)
	}
	// 自定义 Token（复用键含 custom_sub_id）
	customTk, err := tk.GetOrCreateUserToken(ctx, 1, 1, c.ID, 0)
	if err != nil {
		t.Fatalf("创建自定义 Token 失败: %v", err)
	}
	if err := svc.Delete(ctx, 1, 1); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if _, err := tk.FindByToken(ctx, customTk.Token); err != token.ErrTokenNotFound {
		t.Errorf("删除后自定义 Token 应失效: %v", err)
	}
	var vcount int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM versions WHERE owner_type='custom' AND owner_id=?`, c.ID).Scan(&vcount); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if vcount != 0 {
		t.Errorf("版本记录应清理: %d", vcount)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "contents", "custom", strconv.FormatInt(c.ID, 10))); !os.IsNotExist(err) {
		t.Errorf("版本目录应删除: %v", err)
	}
	// 删除后再上传 → 新记录
	c2, err := svc.Upsert(ctx, 1, 1, version.BytesContent([]byte("v2")))
	if err != nil {
		t.Fatalf("重新上传失败: %v", err)
	}
	if c2.ID == c.ID {
		t.Error("删除后重新上传应产生新记录")
	}
}

// TestUpsertMissingPlatform 平台不存在 → 参数错误
func TestUpsertMissingPlatform(t *testing.T) {
	_, svc, _, _, _ := newTestCustomService(t)
	ctx := context.Background()
	if _, err := svc.Upsert(ctx, 1, 999, version.BytesContent([]byte("v1"))); !errors.Is(err, ErrBadRequest) {
		t.Errorf("平台不存在应报参数错误: %v", err)
	}
}

