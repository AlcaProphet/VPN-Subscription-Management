package platform

import (
	"bytes"
	"context"
	"errors"
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
			product_type TEXT NOT NULL DEFAULT 'yaml',
			is_default INTEGER NOT NULL DEFAULT 0,
			description TEXT NOT NULL DEFAULT '',
			schemes TEXT NOT NULL DEFAULT '[]',
			extra_headers TEXT NOT NULL DEFAULT '{}',
			installer_files TEXT NOT NULL DEFAULT '[]',
			installer_urls TEXT NOT NULL DEFAULT '[]',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		// 平台删除完整级联所需的关联表（订阅/版本/自定义/Token/组选定）
		"1002_subscriptions_versions.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS subscriptions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
			platform_id INTEGER NOT NULL, current_version INTEGER NOT NULL DEFAULT 0,
			product_type TEXT NOT NULL DEFAULT 'yaml',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS versions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				owner_type TEXT NOT NULL CHECK (owner_type IN ('subscription','rule','custom','share')),
				owner_id INTEGER NOT NULL, version_no INTEGER NOT NULL, file_path TEXT NOT NULL,
								file_name TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				UNIQUE (owner_type, owner_id, version_no));`)},
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
	for _, headers := range []map[string]string{
		{"profile-update-interval": "-1"},
		{"profile-update-interval": "abc"},
		{"profile-web-page-url": "ftp://example.com"},
		{"subscription-userinfo": "upload=-1; download=0"},
		{"subscription-userinfo": "upload=x"},
	} {
		if err := ValidateExtraHeaders(headers); err == nil {
			t.Errorf("非法生态头应拒绝: %v", headers)
		}
	}
	for _, headers := range []map[string]string{
		{"profile-update-interval": "0"},
		{"profile-web-page-url": "https://vpn.example.com/profile"},
		{"subscription-userinfo": "upload=0; download=1; total=2; expire=3"},
	} {
		if err := ValidateExtraHeaders(headers); err != nil {
			t.Errorf("合法生态头应通过: %v: %v", headers, err)
		}
	}
}

// TestUploadInstallerTooLarge 超限流被拒且无残留文件
func TestUploadInstallerTooLarge(t *testing.T) {
	_, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "测试平台", "", "yaml", []string{"clash://{url}"}, nil, nil)
	if err != nil {
		t.Fatalf("创建平台失败: %v", err)
	}
	big := io.LimitReader(zeroReader{}, MaxInstallerSize+1) // 300MB + 1 字节
	if _, err := svc.UploadInstaller(ctx, p.ID, big, "huge.exe"); err != ErrInstallerTooLarge {
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

// TestUploadInstallerAppend 多安装包并存：多次上传全部保留且列表按序返回
func TestUploadInstallerAppend(t *testing.T) {
	_, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "测试平台", "", "yaml", nil, nil, nil)
	if err != nil {
		t.Fatalf("创建平台失败: %v", err)
	}
	list, err := svc.UploadInstaller(ctx, p.ID, strings.NewReader("v1"), "a.exe")
	if err != nil {
		t.Fatalf("首次上传失败: %v", err)
	}
	first := list[0]
	if len(list) != 1 || first.Name != "a.exe" {
		t.Fatalf("首次上传后列表异常: %+v", list)
	}
	list, err = svc.UploadInstaller(ctx, p.ID, strings.NewReader("v2"), "b.exe")
	if err != nil {
		t.Fatalf("二次上传失败: %v", err)
	}
	if len(list) != 2 || list[0].File != first.File || list[1].Name != "b.exe" {
		t.Fatalf("二次上传后列表应追加而非覆盖: %+v", list)
	}
	// 两个文件都在磁盘上
	for _, it := range list {
		if _, err := os.Stat(filepath.Join(svc.dataDir, installerDir, it.File)); err != nil {
			t.Errorf("安装包文件应存在: %s => %v", it.File, err)
		}
	}
	// DB 与磁盘一致
	got, err := svc.Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("读取平台失败: %v", err)
	}
	if len(got.InstallerFiles) != 2 {
		t.Fatalf("DB 应存 2 个安装包: %+v", got.InstallerFiles)
	}
}

// TestUploadInstallerConcurrent 并发上传串行完成且两个文件都存活（BEGIN IMMEDIATE 防互删）
func TestUploadInstallerConcurrent(t *testing.T) {
	_, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "测试平台", "", "yaml", nil, nil, nil)
	if err != nil {
		t.Fatalf("创建平台失败: %v", err)
	}
	var wg sync.WaitGroup
	results := make([][]InstallerFileItem, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], _ = svc.UploadInstaller(ctx, p.ID, strings.NewReader("v"), "c.exe")
		}(i)
	}
	wg.Wait()
	for i, list := range results {
		if len(list) == 0 {
			t.Fatalf("并发上传 %d 未返回列表", i)
		}
	}
	// 目录中应存 2 个文件，与 DB 记录一致
	dir := filepath.Join(svc.dataDir, installerDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取安装包目录失败: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("并发上传后应存活 2 个文件: %d", len(entries))
	}
	got, err := svc.Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("读取平台失败: %v", err)
	}
	if len(got.InstallerFiles) != 2 {
		t.Errorf("DB 记录与磁盘不一致: db=%d file=%d", len(got.InstallerFiles), len(entries))
	}
}

// TestDeleteInstallerFile 单独删除指定安装包：只删目标文件，其余保留
func TestDeleteInstallerFile(t *testing.T) {
	_, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "测试平台", "", "yaml", nil, nil, nil)
	if err != nil {
		t.Fatalf("创建平台失败: %v", err)
	}
	list, err := svc.UploadInstaller(ctx, p.ID, strings.NewReader("v1"), "a.exe")
	if err != nil {
		t.Fatalf("上传 a 失败: %v", err)
	}
	list, err = svc.UploadInstaller(ctx, p.ID, strings.NewReader("v2"), "b.exe")
	if err != nil {
		t.Fatalf("上传 b 失败: %v", err)
	}
	target := list[0].File
	if err := svc.DeleteInstallerFile(ctx, p.ID, target); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(svc.dataDir, installerDir, target)); !os.IsNotExist(err) {
		t.Errorf("目标安装包文件应被删除: %v", err)
	}
	got, err := svc.Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("读取平台失败: %v", err)
	}
	if len(got.InstallerFiles) != 1 || got.InstallerFiles[0].File == target {
		t.Errorf("应仅剩另一个安装包: %+v", got.InstallerFiles)
	}
	// 幂等：重复删除不报错
	if err := svc.DeleteInstallerFile(ctx, p.ID, target); err != nil {
		t.Errorf("重复删除应幂等成功: %v", err)
	}
	// 路径穿越防护：非基本文件名拒绝
	if err := svc.DeleteInstallerFile(ctx, p.ID, "../evil.exe"); err != ErrBadRequest {
		t.Errorf("非基本文件名应拒绝: %v", err)
	}
}

// TestValidateInstallerURLs 外链校验：非 http/https 拒绝、控制字符拒绝、空地址拒绝、合法通过
func TestValidateInstallerURLs(t *testing.T) {
	if _, err := ValidateInstallerURLs([]InstallerURLItem{{URL: "javascript:alert(1)"}}); err == nil {
		t.Error("javascript 伪协议应拒绝")
	}
	if _, err := ValidateInstallerURLs([]InstallerURLItem{{URL: "ftp://x.com/a.exe"}}); err == nil {
		t.Error("ftp 协议应拒绝")
	}
	if _, err := ValidateInstallerURLs([]InstallerURLItem{{URL: "http://x.com/a\r\nX: 1"}}); err == nil {
		t.Error("含控制字符应拒绝")
	}
	if _, err := ValidateInstallerURLs([]InstallerURLItem{{URL: ""}}); err == nil {
		t.Error("空地址应拒绝")
	}
	out, err := ValidateInstallerURLs([]InstallerURLItem{{Name: "官网", URL: " https://x.com/a.exe "}})
	if err != nil {
		t.Fatalf("合法 http 地址应通过: %v", err)
	}
	if out[0].URL != "https://x.com/a.exe" {
		t.Errorf("应去除首尾空白: %+v", out[0])
	}
}

// TestUpdateKeepsSlug 创建后 slug 不可改（业务层 Update 不触碰 slug 列）
func TestUpdateKeepsSlug(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "原名", "", "yaml", nil, nil, nil)
	if err != nil {
		t.Fatalf("创建平台失败: %v", err)
	}
	if err := svc.Update(ctx, p.ID, "新名", "描述", "yaml", []string{"v2rayng://{url}"}, map[string]string{"X-A": "1"},
		[]InstallerURLItem{{Name: "官网", URL: "https://x.com/a.exe"}}); err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	var slug string
	if err := st.DB().QueryRow(`SELECT slug FROM platforms WHERE id = ?`, p.ID).Scan(&slug); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if slug != p.Slug {
		t.Errorf("slug 不应被修改: got=%s want=%s", slug, p.Slug)
	}
	got, err := svc.Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("读取平台失败: %v", err)
	}
	if len(got.InstallerURLs) != 1 || got.InstallerURLs[0].URL != "https://x.com/a.exe" {
		t.Errorf("外链应随更新保存: %+v", got.InstallerURLs)
	}
}

// TestDeleteCascadesInstaller 删除平台级联删全部安装包文件
func TestDeleteCascadesInstaller(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "测试平台", "", "yaml", nil, nil, nil)
	if err != nil {
		t.Fatalf("创建平台失败: %v", err)
	}
	if _, err := svc.UploadInstaller(ctx, p.ID, bytes.NewReader([]byte("v1")), "a.exe"); err != nil {
		t.Fatalf("上传 a 失败: %v", err)
	}
	if _, err := svc.UploadInstaller(ctx, p.ID, bytes.NewReader([]byte("v2")), "b.exe"); err != nil {
		t.Fatalf("上传 b 失败: %v", err)
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
	// 全部安装包文件已级联删除
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
	p, err := svc.Create(ctx, "测试平台", "", "yaml", nil, nil, nil)
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
	if _, _, err := svc.versions.CreateVersion(ctx, version.OwnerSubscription, subID, version.BytesContent([]byte("v1")), version.CreateOptions{Activate: true}); err != nil {
		t.Fatalf("创建订阅版本失败: %v", err)
	}
	if _, _, err := svc.versions.CreateVersion(ctx, version.OwnerSubscription, subID, version.BytesContent([]byte("v2")), version.CreateOptions{Activate: true}); err != nil {
		t.Fatalf("创建订阅版本失败: %v", err)
	}
	if _, err := st.DB().Exec(`INSERT INTO custom_subscriptions (slug, user_id, platform_id) VALUES ('custom-c', 1, ?)`, p.ID); err != nil {
		t.Fatalf("创建自定义失败: %v", err)
	}
	var customID int64
	if err := st.DB().QueryRow(`SELECT id FROM custom_subscriptions WHERE platform_id = ?`, p.ID).Scan(&customID); err != nil {
		t.Fatalf("查询自定义失败: %v", err)
	}
	if _, _, err := svc.versions.CreateVersion(ctx, version.OwnerCustom, customID, version.BytesContent([]byte("cv1")), version.CreateOptions{Activate: true}); err != nil {
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

// TestCreateProductType 创建平台携带 product_type，非法枚举拒绝
func TestCreateProductType(t *testing.T) {
	_, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "SR平台", "", "subs", nil, nil, nil)
	if err != nil {
		t.Fatalf("创建平台失败: %v", err)
	}
	if p.ProductType != "subs" {
		t.Errorf("product_type 异常: %s", p.ProductType)
	}
	if _, err := svc.Create(ctx, "非法平台", "", "ssr", nil, nil, nil); !errors.Is(err, ErrBadRequest) {
		t.Errorf("非法 product_type 应拒绝: %v", err)
	}
}

// TestUpdateProductTypeConflict 平台产物格式变更与既有订阅条目不一致时拒绝（文案含类型插值）
func TestUpdateProductTypeConflict(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "测试平台", "", "yaml", nil, nil, nil)
	if err != nil {
		t.Fatalf("创建平台失败: %v", err)
	}
	if _, err := st.DB().Exec(`INSERT INTO subscriptions (slug, name, platform_id, product_type) VALUES ('sub-p','订阅',?, 'yaml')`, p.ID); err != nil {
		t.Fatalf("创建订阅失败: %v", err)
	}
	err = svc.Update(ctx, p.ID, "测试平台", "", "subs", nil, nil, nil)
	if !errors.Is(err, ErrProductTypeInUse) {
		t.Fatalf("格式冲突应返回 ErrProductTypeInUse: %v", err)
	}
	if err.Error() != "该平台已有 yaml 订阅条目，请先处理后再变更产物格式" {
		t.Errorf("冲突文案异常: %s", err.Error())
	}
	// 与既有条目一致时成功
	if err := svc.Update(ctx, p.ID, "测试平台", "", "yaml", nil, nil, nil); err != nil {
		t.Errorf("一致格式应允许保存: %v", err)
	}
}

// TestUpdateDefaultPlatformProductTypeLocked 默认平台（is_default=1）产物格式不可修改（R22-04）
func TestUpdateDefaultPlatformProductTypeLocked(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "默认平台", "", "yaml", nil, nil, nil)
	if err != nil {
		t.Fatalf("创建平台失败: %v", err)
	}
	if _, err := st.DB().Exec(`UPDATE platforms SET is_default = 1 WHERE id = ?`, p.ID); err != nil {
		t.Fatalf("标记默认平台失败: %v", err)
	}
	err = svc.Update(ctx, p.ID, "默认平台", "", "subs", nil, nil, nil)
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("默认平台修改产物格式应返回 ErrBadRequest: %v", err)
	}
	// 保持同格式仍可保存（仅锁定格式，不锁定其他编辑）
	if err := svc.Update(ctx, p.ID, "默认平台改名", "", "yaml", nil, nil, nil); err != nil {
		t.Errorf("默认平台同格式/改名应允许: %v", err)
	}
}
