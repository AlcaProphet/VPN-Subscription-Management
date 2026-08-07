package platform

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/internal/version"
)

// zeroReader 无限零流（模拟大文件上传，避免占用真实内存）
type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

// newTestService 创建临时库（含 groups/platforms 表）+ 平台服务
func newTestService(t *testing.T) (*store.Store, *Service) {
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
		"0003_groups_platforms.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS groups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL UNIQUE,
			is_default INTEGER NOT NULL DEFAULT 0,
			needs_reselect INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS platforms (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			schemes TEXT NOT NULL DEFAULT '[]',
			extra_headers TEXT NOT NULL DEFAULT '{}',
			installer_file TEXT,
			installer_url TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		// 平台删除完整级联所需的关联表（订阅/版本/自定义/Token/组选定）
		"1002_subscriptions_versions.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS subscriptions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
			platform_id INTEGER NOT NULL, current_version INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS versions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				owner_type TEXT NOT NULL CHECK (owner_type IN ('subscription','rule','custom','share')),
				owner_id INTEGER NOT NULL, version_no INTEGER NOT NULL, file_path TEXT NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				UNIQUE (owner_type, owner_id, version_no));`)},
		"1003_groups.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS group_selections (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			platform_id INTEGER NOT NULL REFERENCES platforms(id) ON DELETE CASCADE,
			subscription_id INTEGER,
			UNIQUE (group_id, platform_id));`)},
		"1004_tokens.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS download_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token TEXT NOT NULL UNIQUE,
			user_id INTEGER NOT NULL,
			platform_id INTEGER NOT NULL,
			custom_sub_id INTEGER,
			subscription_id INTEGER,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"1005_custom_share.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS custom_subscriptions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT NOT NULL UNIQUE,
			user_id INTEGER NOT NULL,
			platform_id INTEGER NOT NULL,
			current_version INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (user_id, platform_id));`)},
	}
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	dataDir := t.TempDir()
	verSvc := version.NewService(st, dataDir, log.New("error", "console"))
	svc := NewService(st, dataDir, verSvc, log.New("error", "console"))
	return st, svc
}

// TestValidateExtraHeaders 附加头：控制字符拒绝、非法头名拒绝、合法值通过
func TestValidateExtraHeaders(t *testing.T) {
	// \r\n 注入值拒绝
	if err := ValidateExtraHeaders(map[string]string{"X-A": "a\r\nb"}); err == nil {
		t.Error("含 \\r\\n 的值应拒绝")
	}
	// \r\n 注入键拒绝
	if err := ValidateExtraHeaders(map[string]string{"X-A\r\nX-B": "v"}); err == nil {
		t.Error("含 \\r\\n 的键应拒绝")
	}
	// 非法头名（含空格）拒绝
	if err := ValidateExtraHeaders(map[string]string{"Bad Header": "v"}); err == nil {
		t.Error("含空格的键应拒绝")
	}
	// {frontend_url} 占位符合法值通过
	if err := ValidateExtraHeaders(map[string]string{"profile-web-page-url": "{frontend_url}"}); err != nil {
		t.Errorf("合法头应通过: %v", err)
	}
}

// TestUploadInstallerTooLarge 超限流被拒且无残留文件
func TestUploadInstallerTooLarge(t *testing.T) {
	_, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "测试平台", "", []string{"clash://{url}"}, nil)
	if err != nil {
		t.Fatalf("创建平台失败: %v", err)
	}
	big := io.LimitReader(zeroReader{}, MaxInstallerSize+1) // 300MB + 1 字节
	if err := svc.UploadInstaller(ctx, p.ID, big, "huge.exe"); err != ErrInstallerTooLarge {
		t.Fatalf("超限应返回 ErrInstallerTooLarge: %v", err)
	}
	// 无残留文件
	dir := filepath.Join(svc.dataDir, installerDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取安装包目录失败: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("超限上传不应残留文件: %v", entries)
	}
}

// TestUploadInstallerOverwrite 覆盖上传后旧时间戳文件被删
func TestUploadInstallerOverwrite(t *testing.T) {
	_, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "测试平台", "", nil, nil)
	if err != nil {
		t.Fatalf("创建平台失败: %v", err)
	}
	if err := svc.UploadInstaller(ctx, p.ID, strings.NewReader("v1"), "a.exe"); err != nil {
		t.Fatalf("首次上传失败: %v", err)
	}
	got, err := svc.Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("读取平台失败: %v", err)
	}
	oldName := got.InstallerFile
	if oldName == "" {
		t.Fatal("首次上传后 installer_file 应为空串以外的值")
	}
	if err := svc.UploadInstaller(ctx, p.ID, strings.NewReader("v2"), "b.exe"); err != nil {
		t.Fatalf("覆盖上传失败: %v", err)
	}
	// 旧文件已删，新文件存在
	if _, err := os.Stat(filepath.Join(svc.dataDir, installerDir, oldName)); !os.IsNotExist(err) {
		t.Errorf("旧安装包文件应被删除: %v", err)
	}
	got, err = svc.Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("读取平台失败: %v", err)
	}
	if got.InstallerFile == oldName {
		t.Error("installer_file 应已更新为新文件名")
	}
	if _, err := os.Stat(filepath.Join(svc.dataDir, installerDir, got.InstallerFile)); err != nil {
		t.Errorf("新安装包文件应存在: %v", err)
	}
}

// TestUploadInstallerConcurrent 并发上传串行完成且仅最新文件存活（BEGIN IMMEDIATE 防互删）
func TestUploadInstallerConcurrent(t *testing.T) {
	_, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "测试平台", "", nil, nil)
	if err != nil {
		t.Fatalf("创建平台失败: %v", err)
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = svc.UploadInstaller(ctx, p.ID, strings.NewReader("v"), "c.exe")
		}(i)
	}
	wg.Wait()
	for i, e := range errs {
		if e != nil {
			t.Fatalf("并发上传 %d 失败: %v", i, e)
		}
	}
	// 目录中应仅存 1 个文件（最后一次上传的），且与 DB 记录一致
	dir := filepath.Join(svc.dataDir, installerDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取安装包目录失败: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("并发上传后应仅存活 1 个文件: %d", len(entries))
	}
	got, err := svc.Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("读取平台失败: %v", err)
	}
	if got.InstallerFile != entries[0].Name() {
		t.Errorf("DB 记录与存活文件不一致: db=%s file=%s", got.InstallerFile, entries[0].Name())
	}
}

// TestUpdateKeepsSlug 创建后 slug 不可改（业务层 Update 不触碰 slug 列）
func TestUpdateKeepsSlug(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "原名", "", nil, nil)
	if err != nil {
		t.Fatalf("创建平台失败: %v", err)
	}
	if err := svc.Update(ctx, p.ID, "新名", "描述", []string{"v2rayng://{url}"}, map[string]string{"X-A": "1"}); err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	var slug string
	if err := st.DB().QueryRow(`SELECT slug FROM platforms WHERE id = ?`, p.ID).Scan(&slug); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if slug != p.Slug {
		t.Errorf("slug 不应被修改: got=%s want=%s", slug, p.Slug)
	}
}

// TestDeleteCascadesInstaller 删除平台级联删安装包文件
func TestDeleteCascadesInstaller(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "测试平台", "", nil, nil)
	if err != nil {
		t.Fatalf("创建平台失败: %v", err)
	}
	if err := svc.UploadInstaller(ctx, p.ID, bytes.NewReader([]byte("v")), "a.exe"); err != nil {
		t.Fatalf("上传失败: %v", err)
	}
	if err := svc.Delete(ctx, p.ID); err != nil {
		t.Fatalf("删除平台失败: %v", err)
	}
	// 平台行已删
	var n int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM platforms WHERE id = ?`, p.ID).Scan(&n); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if n != 0 {
		t.Error("平台行应已删除")
	}
	// 安装包文件已级联删除
	dir := filepath.Join(svc.dataDir, installerDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取安装包目录失败: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("删除平台后安装包文件应级联删除: %v", entries)
	}
}

// TestDeleteFullCascade 平台删除完整级联：订阅/自定义订阅/Token/版本表与文件无残留（Step 5 补齐）
func TestDeleteFullCascade(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "测试平台", "", nil, nil)
	if err != nil {
		t.Fatalf("创建平台失败: %v", err)
	}
	// 订阅（2 版本）+ 自定义订阅（1 版本）+ Token
	if _, err := st.DB().Exec(`INSERT INTO subscriptions (slug, name, platform_id) VALUES ('sub-c', '订阅', ?)`, p.ID); err != nil {
		t.Fatalf("创建订阅失败: %v", err)
	}
	var subID int64
	if err := st.DB().QueryRow(`SELECT id FROM subscriptions WHERE platform_id = ?`, p.ID).Scan(&subID); err != nil {
		t.Fatalf("查询订阅失败: %v", err)
	}
	if _, err := svc.versions.CreateVersion(ctx, version.OwnerSubscription, subID, version.BytesContent([]byte("v1"))); err != nil {
		t.Fatalf("创建订阅版本失败: %v", err)
	}
	if _, err := svc.versions.CreateVersion(ctx, version.OwnerSubscription, subID, version.BytesContent([]byte("v2"))); err != nil {
		t.Fatalf("创建订阅版本失败: %v", err)
	}
	if _, err := st.DB().Exec(`INSERT INTO custom_subscriptions (slug, user_id, platform_id) VALUES ('custom-c', 1, ?)`, p.ID); err != nil {
		t.Fatalf("创建自定义失败: %v", err)
	}
	var customID int64
	if err := st.DB().QueryRow(`SELECT id FROM custom_subscriptions WHERE platform_id = ?`, p.ID).Scan(&customID); err != nil {
		t.Fatalf("查询自定义失败: %v", err)
	}
	if _, err := svc.versions.CreateVersion(ctx, version.OwnerCustom, customID, version.BytesContent([]byte("cv1"))); err != nil {
		t.Fatalf("创建自定义版本失败: %v", err)
	}
	if _, err := st.DB().Exec(`INSERT INTO download_tokens (token, user_id, platform_id, subscription_id) VALUES ('tk-1', 1, ?, ?)`, p.ID, subID); err != nil {
		t.Fatalf("创建 Token 失败: %v", err)
	}
	if _, err := st.DB().Exec(`INSERT INTO download_tokens (token, user_id, platform_id, custom_sub_id) VALUES ('tk-2', 1, ?, ?)`, p.ID, customID); err != nil {
		t.Fatalf("创建自定义 Token 失败: %v", err)
	}
	if err := svc.Delete(ctx, p.ID); err != nil {
		t.Fatalf("删除平台失败: %v", err)
	}
	// 四类表无残留
	for _, q := range []string{
		`SELECT COUNT(*) FROM subscriptions WHERE platform_id = ?`,
		`SELECT COUNT(*) FROM custom_subscriptions WHERE platform_id = ?`,
		`SELECT COUNT(*) FROM download_tokens WHERE platform_id = ?`,
	} {
		var n int
		if err := st.DB().QueryRow(q, p.ID).Scan(&n); err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		if n != 0 {
			t.Errorf("级联删除后应无残留: %s => %d", q, n)
		}
	}
	// 版本记录与文件无残留
	var vcount int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM versions WHERE owner_id IN (?, ?)`, subID, customID).Scan(&vcount); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if vcount != 0 {
		t.Errorf("版本记录应无残留: %d", vcount)
	}
	for _, dir := range []string{
		filepath.Join(svc.dataDir, "contents", "subscription", strconv.FormatInt(subID, 10)),
		filepath.Join(svc.dataDir, "contents", "custom", strconv.FormatInt(customID, 10)),
	} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Errorf("版本目录应删除: %s => %v", dir, err)
		}
	}
}

