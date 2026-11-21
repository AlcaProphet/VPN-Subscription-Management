package store

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestIsAccessError 覆盖真实 SQLite 打开失败、只读迁移及损坏文件的分类。
func TestIsAccessError(t *testing.T) {
	t.Run("permission", func(t *testing.T) {
		for _, cause := range []error{fs.ErrPermission, syscall.EROFS, syscall.ENOSPC, syscall.EDQUOT, syscall.EIO} {
			err := fmt.Errorf("创建目录: %w", &os.PathError{Op: "mkdir", Path: "/data", Err: cause})
			if !IsAccessError(err) {
				t.Fatalf("目录访问错误应归为存储访问失败: %v", err)
			}
		}
	})
	t.Run("cannot open", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "test.db"), 0o755); err != nil {
			t.Fatal(err)
		}
		_, err := Open(dir, "test.db")
		if err == nil || !IsAccessError(err) {
			t.Fatalf("SQLite 无法打开路径应归为存储访问失败: %v", err)
		}
	})
	t.Run("read only migration", func(t *testing.T) {
		st, err := Open(t.TempDir(), "test.db")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := st.Close(); err != nil {
				t.Error(err)
			}
		})
		if _, err := st.DB().Exec("PRAGMA query_only=ON"); err != nil {
			t.Fatal(err)
		}
		err = st.Migrate(context.Background(), testFS())
		if err == nil || !IsAccessError(err) {
			t.Fatalf("只读迁移应归为存储访问失败: %v", err)
		}
	})
	t.Run("corrupt database", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "test.db"), []byte("invalid SQLite file"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := Open(dir, "test.db")
		if err == nil || IsAccessError(err) {
			t.Fatalf("损坏文件应保留应急恢复分支: %v", err)
		}
	})
	for _, err := range []error{nil, errors.New("其他错误")} {
		if IsAccessError(err) {
			t.Fatalf("错误分类异常: %v", err)
		}
	}
}
