package share

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

// testMigrateFS 构造含 share_subscriptions/versions/share_tokens 表的迁移集
func testMigrateFS() fstest.MapFS {
	return fstest.MapFS{
		"0001_init.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY, value TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"1002_subscriptions_versions.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS versions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				owner_type TEXT NOT NULL CHECK (owner_type IN ('subscription','rule','custom','share')),
				owner_id INTEGER NOT NULL, version_no INTEGER NOT NULL, file_path TEXT NOT NULL,
								file_name TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				UNIQUE (owner_type, owner_id, version_no));`)},
		"1004_tokens.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS share_tokens (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				token TEXT NOT NULL UNIQUE,
				share_id INTEGER NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"1005_custom_share.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS share_subscriptions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE,
				name TEXT NOT NULL,
				current_version INTEGER NOT NULL DEFAULT 0,
				token_status TEXT NOT NULL DEFAULT 'active' CHECK (token_status IN ('active','revoked')),
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
	}
}

// newTestShareService 临时库 + 分享服务
func newTestShareService(t *testing.T) (*store.Store, *Service, *version.Service, string) {
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
	return st, svc, ver, dataDir
}

// TestCreate 创建分享：标识（share- 前缀）+ Token 自动生成 + 首版本
func TestCreate(t *testing.T) {
	_, svc, _, _ := newTestShareService(t)
	ctx := context.Background()
	sh, err := svc.Create(ctx, "我的分享", version.BytesContent([]byte("rules: []")))
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if len(sh.Slug) <= len("share-") || sh.Slug[:6] != "share-" {
		t.Errorf("标识应为 share- 前缀: %s", sh.Slug)
	}
	if sh.Token == "" || sh.TokenStatus != "active" {
		t.Errorf("创建后应生成有效 Token: %+v", sh)
	}
	if sh.CurrentVersion != 1 {
		t.Errorf("首版本应切换为当前: %d", sh.CurrentVersion)
	}
}

// TestRevokeRefresh 吊销：Token 记录删除 + 标记 revoked；刷新恢复（清标记 + 新建 Token）
func TestRevokeRefresh(t *testing.T) {
	st, svc, _, _ := newTestShareService(t)
	ctx := context.Background()
	sh, err := svc.Create(ctx, "我的分享", version.BytesContent([]byte("v1")))
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	// 吊销
	if err := svc.RevokeToken(ctx, sh.ID); err != nil {
		t.Fatalf("吊销失败: %v", err)
	}
	var tkCount int
	var status string
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM share_tokens WHERE share_id = ?`, sh.ID).Scan(&tkCount); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if tkCount != 0 {
		t.Errorf("吊销后 Token 记录应物理删除: %d", tkCount)
	}
	if err := st.DB().QueryRow(`SELECT token_status FROM share_subscriptions WHERE id = ?`, sh.ID).Scan(&status); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if status != "revoked" {
		t.Errorf("吊销后状态应为 revoked: %s", status)
	}
	// 列表不返回 Token
	list, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("列表失败: %v", err)
	}
	if len(list) != 1 || list[0].Token != "" || list[0].TokenStatus != "revoked" {
		t.Errorf("吊销后列表不应返回 Token: %+v", list)
	}
	// 刷新恢复
	newToken, err := svc.RefreshToken(ctx, sh.ID)
	if err != nil {
		t.Fatalf("刷新失败: %v", err)
	}
	if newToken == "" {
		t.Error("刷新应返回新 Token")
	}
	if err := st.DB().QueryRow(`SELECT token_status FROM share_subscriptions WHERE id = ?`, sh.ID).Scan(&status); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if status != "active" {
		t.Errorf("刷新后状态应恢复 active: %s", status)
	}
}

// TestRenameOnly 创建后仅可改名
func TestRenameOnly(t *testing.T) {
	_, svc, _, _ := newTestShareService(t)
	ctx := context.Background()
	sh, err := svc.Create(ctx, "原名", version.BytesContent([]byte("v1")))
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if err := svc.Rename(ctx, sh.ID, "新名"); err != nil {
		t.Fatalf("改名失败: %v", err)
	}
	got, err := svc.Get(ctx, sh.ID)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if got.Name != "新名" {
		t.Errorf("改名未生效: %s", got.Name)
	}
	if got.Slug != sh.Slug {
		t.Errorf("改名不应影响标识: %s", got.Slug)
	}
}

// TestDeleteCascade 删除分享级联：Token + 版本文件清理
func TestDeleteCascade(t *testing.T) {
	st, svc, _, dataDir := newTestShareService(t)
	ctx := context.Background()
	sh, err := svc.Create(ctx, "待删除", version.BytesContent([]byte("v1")))
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if err := svc.Delete(ctx, sh.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	var tkCount int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM share_tokens WHERE share_id = ?`, sh.ID).Scan(&tkCount); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if tkCount != 0 {
		t.Errorf("Token 应级联删除: %d", tkCount)
	}
	var vcount int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM versions WHERE owner_type='share' AND owner_id=?`, sh.ID).Scan(&vcount); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if vcount != 0 {
		t.Errorf("版本记录应清理: %d", vcount)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "contents", "share", strconv.FormatInt(sh.ID, 10))); !os.IsNotExist(err) {
		t.Errorf("版本目录应删除: %v", err)
	}
}

// TestCreateEmptyName 名称为空 → 参数错误
func TestCreateEmptyName(t *testing.T) {
	_, svc, _, _ := newTestShareService(t)
	ctx := context.Background()
	if _, err := svc.Create(ctx, "", version.BytesContent([]byte("v1"))); !errors.Is(err, ErrBadRequest) {
		t.Errorf("空名称应报参数错误: %v", err)
	}
}

