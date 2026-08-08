package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
)

// newTestBackup 创建临时库 + 备份服务（含 contents/public 数据）
func newTestBackup(t *testing.T) (*store.Store, *Service, string) {
	t.Helper()
	dataDir := t.TempDir()
	st, err := store.Open(dataDir, "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	fsys := fstest.MapFS{
		"0001_init.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY, value TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
	}
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	// 造数据文件：版本文件（含当前指针符号链接）+ public 资源
	verDir := filepath.Join(dataDir, "contents", "subscription", "1")
	if err := os.MkdirAll(verDir, 0o755); err != nil {
		t.Fatalf("创建版本目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(verDir, "v1"), []byte("proxies: []"), 0o644); err != nil {
		t.Fatalf("写版本文件失败: %v", err)
	}
	if err := os.Symlink("v1", filepath.Join(verDir, "current")); err != nil { // 「当前」指针符号链接
		t.Fatalf("创建符号链接失败: %v", err)
	}
	pubDir := filepath.Join(dataDir, "public", "site")
	if err := os.MkdirAll(pubDir, 0o755); err != nil {
		t.Fatalf("创建 public 目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pubDir, "icon.png"), []byte("png"), 0o644); err != nil {
		t.Fatalf("写 public 文件失败: %v", err)
	}
	// 造库数据（快照应包含）
	if _, err := st.DB().Exec(`INSERT INTO system_config (key, value) VALUES ('site_name','快照验证')`); err != nil {
		t.Fatalf("写配置失败: %v", err)
	}
	svc := NewService(st, dataDir, log.New("error", "console"))
	return st, svc, dataDir
}

// TestCreateBackup tar.gz 解包含 app.db 快照 + contents/ + public/；符号链接保留；快照含库数据
func TestCreateBackup(t *testing.T) {
	_, svc, _ := newTestBackup(t)
	ctx := context.Background()
	var buf bytes.Buffer
	if err := svc.CreateBackup(ctx, &buf); err != nil {
		t.Fatalf("创建备份失败: %v", err)
	}
	gz, err := gzip.NewReader(&buf)
	if err != nil {
		t.Fatalf("解压失败: %v", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	entries := map[string]string{} // name → 类型（file/dir/symlink）
	var symlinkTarget string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("读取 tar 失败: %v", err)
		}
		switch {
		case hdr.Typeflag == tar.TypeSymlink:
			entries[hdr.Name] = "symlink"
			if strings.HasSuffix(hdr.Name, "/current") {
				symlinkTarget = hdr.Linkname
			}
		case hdr.Typeflag == tar.TypeDir:
			entries[hdr.Name] = "dir"
		default:
			entries[hdr.Name] = "file"
		}
	}
	// 关键条目存在（目录条目名无尾斜杠）
	for _, want := range []string{"app.db", "contents", "contents/subscription", "contents/subscription/1", "contents/subscription/1/v1", "contents/subscription/1/current", "public", "public/site", "public/site/icon.png"} {
		if _, ok := entries[want]; !ok {
			t.Errorf("备份缺少条目 %q（现有: %v）", want, keysOf(entries))
		}
	}
	// 符号链接保留「当前」指针
	if entries["contents/subscription/1/current"] != "symlink" || symlinkTarget != "v1" {
		t.Errorf("符号链接应保留: type=%q target=%q", entries["contents/subscription/1/current"], symlinkTarget)
	}
}

// keysOf map 键列表（错误信息辅助）
func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestSnapshotTo 一致性快照：VACUUM INTO 生成首部合法的 SQLite 文件
func TestSnapshotTo(t *testing.T) {
	_, svc, _ := newTestBackup(t)
	dest := filepath.Join(t.TempDir(), "snap.db")
	if err := svc.snapshotTo(dest); err != nil {
		t.Fatalf("快照失败: %v", err)
	}
	f, err := os.Open(dest)
	if err != nil {
		t.Fatalf("打开快照失败: %v", err)
	}
	defer f.Close()
	head := make([]byte, 16)
	if _, err := f.Read(head); err != nil {
		t.Fatalf("读取快照失败: %v", err)
	}
	if !bytes.HasPrefix(head, []byte("SQLite format 3")) {
		t.Error("快照应为首部合法的 SQLite 文件")
	}
}

// TestBackupEmptyDirs 全新部署（无 contents/public 目录）备份不报错
func TestBackupEmptyDirs(t *testing.T) {
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
	}
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	svc := NewService(st, t.TempDir(), log.New("error", "console"))
	var buf bytes.Buffer
	if err := svc.CreateBackup(context.Background(), &buf); err != nil {
		t.Fatalf("空目录备份失败: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("备份内容为空")
	}
}
