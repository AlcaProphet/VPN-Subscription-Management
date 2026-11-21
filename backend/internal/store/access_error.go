package store

import (
	"errors"
	"io/fs"
	"syscall"

	"modernc.org/sqlite"
	"modernc.org/sqlite/lib"
)

// IsAccessError 识别权限、只读挂载、空间和存储访问失败，供启动层与损坏救援区分。
func IsAccessError(err error) bool {
	if errors.Is(err, fs.ErrPermission) || errors.Is(err, syscall.EROFS) ||
		errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EDQUOT) || errors.Is(err, syscall.EIO) {
		return true
	}
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	// 扩展错误码的低八位保留 SQLite 主错误码。
	switch sqliteErr.Code() & 0xff {
	case sqlite3.SQLITE_PERM, sqlite3.SQLITE_READONLY, sqlite3.SQLITE_IOERR, sqlite3.SQLITE_FULL, sqlite3.SQLITE_CANTOPEN:
		return true
	}
	return false
}
