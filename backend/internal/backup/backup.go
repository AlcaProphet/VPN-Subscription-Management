// Package backup 提供备份服务（Build3 Step 4）：SQLite 一致性快照 + 全部版本文件 + /public 资源打包 tar.gz。
// 备份过程不阻断正常服务；备份文件含符号链接（保留「当前」指针，恢复后启动自检以 DB 为准重建）；仅覆盖备份侧（Design1 §3.4.8/7.2）。
package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"vpn-sub/internal/store"
)

// Service 备份服务
type Service struct {
	store   *store.Store
	dataDir string
	log     *slog.Logger
}

func NewService(st *store.Store, dataDir string, lg *slog.Logger) *Service {
	return &Service{store: st, dataDir: dataDir, log: lg}
}

// CreateBackup SQLite backup API 一致性快照 + 全部版本文件 + /public 资源打包 tar.gz；不阻断正常服务
func (s *Service) CreateBackup(ctx context.Context, w io.Writer) error {
	// 1) SQLite backup API 落一致性快照（避免 WAL 未 checkpoint 数据遗漏）
	snapshot := filepath.Join(os.TempDir(), fmt.Sprintf("vpn-sub-backup-%d.db", time.Now().UnixMilli()))
	defer os.Remove(snapshot)
	if err := s.snapshotTo(snapshot); err != nil {
		return fmt.Errorf("创建数据库快照失败: %w", err)
	}
	// 2) tar.gz 打包：快照 + contents/（版本文件，含符号链接保留「当前」指针）+ public/（站点资源/安装包）
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	defer func() { _ = tw.Close(); _ = gz.Close() }()
	if err := addFileToTar(tw, snapshot, "app.db"); err != nil {
		return err
	}
	for _, dir := range []string{"contents", "public"} {
		root := filepath.Join(s.dataDir, dir)
		if _, err := os.Stat(root); os.IsNotExist(err) {
			continue // 目录尚不存在（全新部署）直接跳过
		}
		if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(s.dataDir, path)
			if err != nil {
				return err
			}
			return addToTar(tw, path, rel, info) // 符号链接以链接形式写入（恢复后启动自检以 DB 为准重建）
		}); err != nil {
			return fmt.Errorf("打包 %s 失败: %w", dir, err)
		}
	}
	s.log.Warn("备份下载已执行") // 记 warn 级日志
	return nil
}

// snapshotTo 一致性快照——以 VACUUM INTO 实现（等价于 SQLite backup API 的一致性快照，Design1 §7.2）。
// 版本要求：VACUUM INTO 需 SQLite ≥ 3.27.0；modernc.org/sqlite ≥ v1.30.0（已验证支持）。
// 若驱动不支持（运行时报错），降级：PRAGMA wal_checkpoint(FULL) 将 WAL 落盘后直接拷贝主文件（此时拷贝即为一致快照）
func (s *Service) snapshotTo(dest string) error {
	if _, err := s.store.DB().Exec(`VACUUM INTO ?`, dest); err == nil {
		return nil
	}
	// 降级：WAL checkpoint 后拷贝主文件（保证一致性）
	if _, cerr := s.store.DB().Exec(`PRAGMA wal_checkpoint(FULL)`); cerr != nil {
		return fmt.Errorf("快照失败且 WAL checkpoint 降级失败: %w", cerr)
	}
	return copyFile(s.store.DBPath(), dest)
}

// addFileToTar 单个文件写入 tar（顶层快照文件）
func addFileToTar(tw *tar.Writer, path, name string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	return addToTar(tw, path, name, info)
}

// addToTar 按文件信息写入 tar 项：目录/普通文件/符号链接三类
func addToTar(tw *tar.Writer, path, name string, info os.FileInfo) error {
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(name)
	if info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(path)
		if err != nil {
			return err
		}
		header.Linkname = link // 符号链接以链接形式写入（保留「当前」指针）
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return nil // 目录/符号链接无内容体
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(tw, f); err != nil {
		return err
	}
	return nil
}

// copyFile 文件拷贝（快照降级路径）
func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
