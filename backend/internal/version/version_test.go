package version

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
)

// testMigrateFS 构造含 subscriptions/versions 表的迁移集
func testMigrateFS(withSubscriptions bool) fstest.MapFS {
	fsys := fstest.MapFS{
		"0001_init.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY, value TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"1002_subscriptions_versions.sql": &fstest.MapFile{Data: []byte(`
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
			CREATE INDEX IF NOT EXISTS idx_versions_owner ON versions(owner_type, owner_id, version_no);`)},
	}
	if withSubscriptions {
		// 与真实 1002 迁移一致：subscriptions/versions/关联表同版本迁移（版本号唯一）
		fsys["1002_subscriptions_versions.sql"] = &fstest.MapFile{Data: []byte(`
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
			CREATE TABLE IF NOT EXISTS subscriptions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
				platform_id INTEGER NOT NULL, current_version INTEGER NOT NULL DEFAULT 0,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)}
		fsys["0003_groups_platforms.sql"] = &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS groups (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL UNIQUE,
				is_default INTEGER NOT NULL DEFAULT 0, needs_reselect INTEGER NOT NULL DEFAULT 0,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS platforms (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '', schemes TEXT NOT NULL DEFAULT '[]',
				extra_headers TEXT NOT NULL DEFAULT '{}', installer_files TEXT NOT NULL DEFAULT '[]', installer_urls TEXT NOT NULL DEFAULT '[]',
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)}
	}
	return fsys
}

// newTestVersionService 临时库 + 版本服务
func newTestVersionService(t *testing.T, withSubscriptions bool) (*store.Store, *Service) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), testMigrateFS(withSubscriptions)); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	dataDir := t.TempDir()
	svc := NewService(st, dataDir, log.New("error", "console"))
	return st, svc
}

// newOwner 创建订阅 owner（返回 ID）
func newOwner(t *testing.T, st *store.Store) int64 {
	t.Helper()
	res, err := st.DB().Exec(`INSERT INTO subscriptions (slug, name, platform_id) VALUES ('sub-1', '测试订阅', 1)`)
	if err != nil {
		t.Fatalf("创建订阅失败: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// TestVersionNumberNotReused 创建 3 版删 v2 后再建 → 新号为 4（最大编号 +1，不复用）
func TestVersionNumberNotReused(t *testing.T) {
	st, svc := newTestVersionService(t, true)
	ctx := context.Background()
	owner := newOwner(t, st)
	for i := 0; i < 3; i++ {
		if _, err := svc.CreateVersion(ctx, OwnerSubscription, owner, BytesContent([]byte("v"))); err != nil {
			t.Fatalf("创建版本 %d 失败: %v", i+1, err)
		}
	}
	if err := svc.DeleteVersion(ctx, OwnerSubscription, owner, 2); err != nil {
		t.Fatalf("删除 v2 失败: %v", err)
	}
	v, err := svc.CreateVersion(ctx, OwnerSubscription, owner, BytesContent([]byte("v4")))
	if err != nil {
		t.Fatalf("创建 v4 失败: %v", err)
	}
	if v.No != 4 {
		t.Errorf("新版本号应为 4（最大编号+1），got %d", v.No)
	}
}

// TestMaxVersionsEvictOldest 5 版上限：连续上传 6 版 → 仅存 5 版且最旧被删（文件 + 记录）
func TestMaxVersionsEvictOldest(t *testing.T) {
	st, svc := newTestVersionService(t, true)
	ctx := context.Background()
	owner := newOwner(t, st)
	for i := 1; i <= 6; i++ {
		if _, err := svc.CreateVersion(ctx, OwnerSubscription, owner, BytesContent([]byte("v"))); err != nil {
			t.Fatalf("创建版本 %d 失败: %v", i, err)
		}
	}
	var count int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM versions WHERE owner_type='subscription' AND owner_id=?`, owner).Scan(&count); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if count != MaxVersions {
		t.Errorf("版本数应限制为 %d，got %d", MaxVersions, count)
	}
	// 最旧 v1 的文件与记录应被删除
	var rel string
	if err := st.DB().QueryRow(`SELECT file_path FROM versions WHERE owner_type='subscription' AND owner_id=? AND version_no=1`, owner).Scan(&rel); err == nil {
		t.Error("最旧版本 v1 记录应被删除")
	}
	if _, err := os.Stat(filepath.Join(svc.dataDir, "contents", "subscription", strconv.FormatInt(owner, 10), "v1")); !os.IsNotExist(err) {
		t.Errorf("最旧版本 v1 文件应被删除: %v", err)
	}
}

// TestDeleteConstraints 删除约束：删最后一个拒绝；删当前激活拒绝
func TestDeleteConstraints(t *testing.T) {
	st, svc := newTestVersionService(t, true)
	ctx := context.Background()
	owner := newOwner(t, st)
	if _, err := svc.CreateVersion(ctx, OwnerSubscription, owner, BytesContent([]byte("v1"))); err != nil {
		t.Fatalf("创建 v1 失败: %v", err)
	}
	if err := svc.DeleteVersion(ctx, OwnerSubscription, owner, 1); !errors.Is(err, ErrLastVersion) {
		t.Errorf("删最后一个应拒绝 ErrLastVersion: %v", err)
	}
	if _, err := svc.CreateVersion(ctx, OwnerSubscription, owner, BytesContent([]byte("v2"))); err != nil {
		t.Fatalf("创建 v2 失败: %v", err)
	}
	if err := svc.DeleteVersion(ctx, OwnerSubscription, owner, 2); !errors.Is(err, ErrCurrentVersion) {
		t.Errorf("删当前激活应拒绝 ErrCurrentVersion: %v", err)
	}
}

// TestConcurrentCreate 并发创建版本 → 版本号连续不重复（BEGIN IMMEDIATE 串行化；并发数低于 5 版上限）
func TestConcurrentCreate(t *testing.T) {
	st, svc := newTestVersionService(t, true)
	ctx := context.Background()
	owner := newOwner(t, st)
	const n = 4 // 低于 MaxVersions，避免并发下触发驱逐影响连续性断言
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = svc.CreateVersion(ctx, OwnerSubscription, owner, BytesContent([]byte("v")))
		}(i)
	}
	wg.Wait()
	for i, e := range errs {
		if e != nil {
			t.Fatalf("并发创建 %d 失败: %v", i, e)
		}
	}
	// 版本号应为 1..n 各一份（无重复无空洞）
	rows, err := st.DB().Query(`SELECT version_no FROM versions WHERE owner_type='subscription' AND owner_id=? ORDER BY version_no`, owner)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	defer rows.Close()
	seen := map[int64]bool{}
	for rows.Next() {
		var no int64
		if err := rows.Scan(&no); err != nil {
			t.Fatalf("扫描失败: %v", err)
		}
		if seen[no] {
			t.Fatalf("版本号重复: %d", no)
		}
		seen[no] = true
	}
	if len(seen) != n {
		t.Errorf("版本号应连续 1..%d，实际 %d 个", n, len(seen))
	}
}

// TestStartupCheckRebuildSymlink 手工破坏 symlink 后 StartupCheck 以 DB 为准重建
func TestStartupCheckRebuildSymlink(t *testing.T) {
	st, svc := newTestVersionService(t, true)
	ctx := context.Background()
	owner := newOwner(t, st)
	if _, err := svc.CreateVersion(ctx, OwnerSubscription, owner, BytesContent([]byte("v1"))); err != nil {
		t.Fatalf("创建 v1 失败: %v", err)
	}
	if _, err := svc.CreateVersion(ctx, OwnerSubscription, owner, BytesContent([]byte("v2"))); err != nil {
		t.Fatalf("创建 v2 失败: %v", err)
	}
	dir := filepath.Join(svc.dataDir, "contents", "subscription", strconv.FormatInt(owner, 10))
	// 破坏 symlink（指向不存在的 v9）
	if err := os.Remove(filepath.Join(dir, currentLink)); err != nil {
		t.Fatalf("删除指针失败: %v", err)
	}
	if err := os.Symlink("v9", filepath.Join(dir, currentLink)); err != nil {
		t.Fatalf("伪造错误指针失败: %v", err)
	}
	if err := svc.StartupCheck(ctx); err != nil {
		t.Fatalf("启动自检失败: %v", err)
	}
	// 指针应重建指向当前版本（v2，DB 为准）
	target, err := os.Readlink(filepath.Join(dir, currentLink))
	if err != nil {
		t.Fatalf("读取指针失败: %v", err)
	}
	if target != "v2" {
		t.Errorf("指针应以 DB 为准重建指向 v2，got %s", target)
	}
}

// TestCreateRollbackCleanup 失败回滚：owner 表缺失（setCurrentLocked 失败）→ 版本文件无残留
func TestCreateRollbackCleanup(t *testing.T) {
	// 仅建 versions 表，无 subscriptions 表 → INSERT versions 成功但 setCurrentLocked 失败 → 整体回滚
	_, svc := newTestVersionService(t, false)
	ctx := context.Background()
	_, err := svc.CreateVersion(ctx, OwnerSubscription, 1, BytesContent([]byte("v")))
	if err == nil {
		t.Fatal("owner 表缺失场景应失败")
	}
	// 版本记录无残留
	st := svc.store
	var count int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM versions`).Scan(&count); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if count != 0 {
		t.Errorf("事务回滚后版本记录应无残留: %d", count)
	}
	// 版本文件无残留（空目录残留无害，仅断言版本文件不存在）
	if _, err := os.Stat(filepath.Join(svc.dataDir, "contents", "subscription", "1", "v1")); !os.IsNotExist(err) {
		t.Errorf("事务回滚后版本文件应无残留: %v", err)
	}
}

// TestListVersionsFileName 版本列表返回 file_name（R08-03：文件模式记录原始名，文本模式补类型默认名）
func TestListVersionsFileName(t *testing.T) {
	st, svc := newTestVersionService(t, true)
	ctx := context.Background()
	owner := newOwner(t, st)
	// 文件模式：ReaderContent 携带原始文件名
	if _, err := svc.CreateVersion(ctx, OwnerSubscription, owner, ReaderContent{R: strings.NewReader("v1"), Max: 1024, Name: "my-sub.yaml"}); err != nil {
		t.Fatalf("创建文件版本失败: %v", err)
	}
	// 文本模式：无原始文件名 → 补类型默认名 subscription.yaml
	if _, err := svc.CreateVersion(ctx, OwnerSubscription, owner, BytesContent([]byte("v2"))); err != nil {
		t.Fatalf("创建文本版本失败: %v", err)
	}
	list, err := svc.ListVersions(ctx, OwnerSubscription, owner, 2)
	if err != nil {
		t.Fatalf("读取版本列表失败: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("版本数应为 2，got %d", len(list))
	}
	if list[0].FileName != "my-sub.yaml" {
		t.Errorf("文件模式 file_name 应为 my-sub.yaml，got %q", list[0].FileName)
	}
	if list[1].FileName != "subscription.yaml" {
		t.Errorf("文本模式 file_name 应为 subscription.yaml，got %q", list[1].FileName)
	}
	if list[1].Current != true {
		t.Errorf("v2 应标记为当前激活版本")
	}
}

// TestYamlWarning 两段式判定：YAML 语法错误提示、非 YAML 静默跳过、合法 YAML 无警告
func TestYamlWarning(t *testing.T) {
	// 启发式 YAML 但语法错误 → 警告
	if w := YamlWarning([]byte("proxies:\n  - name: a\n    type: [broken")); w == "" {
		t.Error("YAML 语法错误应返回警告")
	}
	// 合法 YAML → 无警告
	if w := YamlWarning([]byte("port: 7890\nproxies: []")); w != "" {
		t.Errorf("合法 YAML 不应警告: %s", w)
	}
	// 非 YAML（base64）→ 静默跳过
	if w := YamlWarning([]byte("aGVsbG8gd29ybGQ=")); w != "" {
		t.Errorf("base64 内容应跳过: %s", w)
	}
	// 非 YAML（协议前缀）→ 静默跳过
	if w := YamlWarning([]byte("v2ray://abc123")); w != "" {
		t.Errorf("v2ray 链接应跳过: %s", w)
	}
	// 普通文本（无键值对结构）→ 静默跳过
	if w := YamlWarning([]byte("hello world\njust a plain line")); w != "" {
		t.Errorf("普通文本应跳过: %s", w)
	}
}

