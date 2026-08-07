// Package store 提供 SQLite 数据层封装：连接管理、版本化迁移框架与事务助手。
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动（零 CGO）

	"vpn-sub/internal/log"
)

// migrationsFS 由 backend/migrations 目录 go:embed 嵌入（var FS embed.FS），
// 由 main 注入，保证单二进制分发不依赖磁盘 SQL 文件。
type Store struct {
	db         *sql.DB
	dbPath     string     // 数据库主文件路径（Open 时记录，备份快照降级路径使用）
	mu         sync.Mutex // 迁移串行化
	maxVersion int        // 迁移框架执行后回填：当前代码支持的最高版本
}

// Open 打开数据库：WAL 模式 + 外键 + busy_timeout + 单写者模型
func Open(dataDir, dbFile string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	// dbFile 仅允许基本文件名（防路径穿越；由启动参数传入）
	if filepath.Base(dbFile) != dbFile {
		return nil, fmt.Errorf("数据库文件名非法: %s", dbFile)
	}
	// DSN 追加 ?_txlock=immediate：使 db.BeginTx 直接发 BEGIN IMMEDIATE（TxImmediate 依赖此，Design1 §4.1）
	dsn := filepath.Join(dataDir, dbFile) + "?_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite 单写者模型：规避并发写 busy，配合 busy_timeout
	if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000;"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("初始化 PRAGMA 失败: %w", err)
	}
	return &Store{db: db, dbPath: filepath.Join(dataDir, dbFile)}, nil
}

func (s *Store) DB() *sql.DB { return s.db }

// DBPath 返回数据库主文件路径（备份快照降级路径使用）
func (s *Store) DBPath() string { return s.dbPath }

// Close 关闭数据库连接
func (s *Store) Close() error { return s.db.Close() }

// Migrate 版本化迁移（关键约束，Design1 §7.4）：
// 文件命名 NNNN_<name>.sql，按版本号升序执行未应用项；
// 单条迁移与其版本记录在同一事务内写入，任一失败即拒绝启动，不进入半迁移状态；
// 数据库版本高于代码支持版本 → 拒绝启动（回滚边界）
func (s *Store) Migrate(ctx context.Context, migrationsFS fs.FS) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// 1) 自建迁移登记表（与 0001_init.sql 中显式建表幂等共存）
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		return fmt.Errorf("创建 schema_migrations 失败: %w", err)
	}
	// 2) 读取已应用版本集合
	applied := map[int]bool{}
	rows, err := s.db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("读取迁移记录失败: %w", err)
	}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			_ = rows.Close()
			return fmt.Errorf("解析迁移记录失败: %w", err)
		}
		applied[v] = true
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("关闭迁移记录游标失败: %w", err)
	}
	// 3) 扫描嵌入迁移文件（按文件名排序），逐个应用
	names, err := sortedEntries(migrationsFS)
	if err != nil {
		return fmt.Errorf("读取迁移目录失败: %w", err)
	}
	for _, name := range names {
		version, err := parseVersion(name) // 前 4 位数字，解析失败视为非法迁移文件并报错
		if err != nil {
			return err
		}
		if applied[version] {
			if version > s.maxVersion {
				s.maxVersion = version
			}
			continue
		}
		content, err := fs.ReadFile(migrationsFS, name)
		if err != nil {
			return fmt.Errorf("读取迁移文件 %s 失败: %w", name, err)
		}
		if err := s.applyOne(ctx, version, string(content)); err != nil {
			return fmt.Errorf("迁移 %s 失败，拒绝启动: %w", name, err)
		}
		if version > s.maxVersion {
			s.maxVersion = version
		}
		log.Info("迁移已应用", "file", name, "version", version)
	}
	// 4) 回滚边界校验
	var dbVersion int
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&dbVersion); err != nil {
		return fmt.Errorf("读取数据库 schema 版本失败: %w", err)
	}
	if dbVersion > s.maxVersion {
		return fmt.Errorf("数据库 schema 版本 %d 高于当前代码支持版本 %d，拒绝启动（请升级程序，禁止降级运行）", dbVersion, s.maxVersion)
	}
	return nil
}

// applyOne 单条迁移与其版本记录在同一事务内写入
func (s *Store) applyOne(ctx context.Context, version int, sqlText string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, sqlText); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, version); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// TxImmediate 以 BEGIN IMMEDIATE 开启事务（先读后写场景专用，Design1 §4.1）：
// 开启即持有写锁，「读 → 判定 → 写」全程串行化；fn 返回非 nil 自动回滚。
// 实现要点：modernc.org/sqlite 驱动在 DSN 加 ?_txlock=immediate 后，db.BeginTx 即直接发 BEGIN IMMEDIATE；
// 禁止在 BeginTx 后再执行 "ROLLBACK; BEGIN IMMEDIATE"（会脱离 database/sql 事务对象管理）。
func (s *Store) TxImmediate(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{}) // DSN _txlock=immediate 使本条即为 BEGIN IMMEDIATE
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}
	return nil
}

// sortedEntries 读取迁移目录并按文件名排序
func sortedEntries(fsys fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// parseVersion 解析迁移文件名前缀版本号（前 4 位数字）
func parseVersion(name string) (int, error) {
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	numPart := base
	if idx := strings.Index(numPart, "_"); idx > 0 {
		numPart = numPart[:idx]
	}
	v, err := strconv.Atoi(numPart)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("非法迁移文件名 %s（须为 NNNN_name.sql 格式）: %w", name, err)
	}
	return v, nil
}

// IsNoRows 判断是否为无记录错误（供业务层复用）
func IsNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }
