// Package version 提供四类资源（订阅/规则/自定义/分享）共用的版本管理事务组件。
// 关键约束（Design1 §4.1）：版本号计算与列表更新在单个 BEGIN IMMEDIATE 事务 + 库级写锁内完成；
// 当前指针以 DB 记录为准，symlink 仅作文件组织，启动时自检重建。
package version

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"vpn-sub/internal/store"
)

// 关键参数（Design1 §4.1/6.3，禁止修改）
const (
	MaxVersions    = 5        // 每份资源最多 5 个版本（含当前激活）
	MaxContentSize = 50 << 20 // 订阅/规则/自定义/分享内容 ≤50MB
	currentLink    = "current"
)

// 业务错误（接入层映射 HTTP 状态码）
var (
	ErrVersionNotFound = errors.New("版本不存在")
	ErrLastVersion     = errors.New("不可删除最后一个版本")
	ErrCurrentVersion  = errors.New("不可删除当前激活版本，请先切换")
	ErrContentTooLarge = errors.New("内容超过 50MB 限制")
)

// OwnerType 资源类型（四类共用版本表）
type OwnerType string

const (
	OwnerSubscription OwnerType = "subscription"
	OwnerRule         OwnerType = "rule"
	OwnerCustom       OwnerType = "custom"
	OwnerShare        OwnerType = "share"
)

// Service 版本管理服务
type Service struct {
	store   *store.Store
	dataDir string
	log     *slog.Logger
}

func NewService(st *store.Store, dataDir string, lg *slog.Logger) *Service {
	return &Service{store: st, dataDir: dataDir, log: lg}
}

// Version 版本记录
type Version struct {
	ID        int64     `json:"id"`
	No        int64     `json:"version_no"`
	FilePath  string    `json:"file_path"`
	FileName  string    `json:"file_name"` // 上传时的原始文件名（文本模式为类型默认名）；下载文件名扩展名来源
	Current   bool      `json:"current"`   // 由调用方对照 owner 的 current_version 填充
	Blueprint bool      `json:"blueprint"` // 装配生成版本（assembly_blueprints 存在）
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// --- 版本创建来源策略（预留第三种扩展点，Design1 §3.9）---

// ContentProvider 版本内容来源抽象；当前实现「文件上传」「文本编辑」两种，
// 「装配生成」在 DesignOnHold 恢复开发时新增实现，不改本组件
// FileName 返回上传时的原始文件名（文本编辑为空，CreateVersion 按类型补默认）
type ContentProvider interface {
	Content() ([]byte, error)
	FileName() string
}

// BytesContent 文本编辑来源
// 文本模式无原始文件名：CreateVersion 按资源类型补默认名（扩展名来源）
type BytesContent []byte

func (b BytesContent) Content() ([]byte, error) { return b, nil }
func (b BytesContent) FileName() string         { return "" }

// ReaderContent 文件上传来源（流式，限大小）；Name 为上传原始文件名
// 接入层从 multipart 的 file.Filename 传入
// 非上传场景（如测试）Name 可为空，扩展名按类型默认补齐
type ReaderContent struct {
	R    io.Reader
	Max  int64
	Name string
}

func (r ReaderContent) Content() ([]byte, error) {
	content, err := io.ReadAll(io.LimitReader(r.R, r.Max+1))
	if err != nil {
		return nil, fmt.Errorf("读取上传内容失败: %w", err)
	}
	if int64(len(content)) > r.Max {
		return nil, ErrContentTooLarge
	}
	return content, nil
}

func (r ReaderContent) FileName() string { return r.Name }

// versionRelPath 版本相对路径：{ownerType}/{ownerID}/v{n}
func versionRelPath(ot OwnerType, ownerID, versionNo int64) string {
	return filepath.Join(string(ot), strconv.FormatInt(ownerID, 10), fmt.Sprintf("v%d", versionNo))
}

// ownerDir 资源内容目录：{dataDir}/contents/{ownerType}/{ownerID}
func (s *Service) ownerDir(ot OwnerType, ownerID int64) string {
	return filepath.Join(s.dataDir, "contents", string(ot), strconv.FormatInt(ownerID, 10))
}

// ContentsRoot 版本内容根目录（级联删除场景路径边界校验用，Build3 用户管理）
func (s *Service) ContentsRoot() string {
	return filepath.Join(s.dataDir, "contents")
}

// CreateOptions 版本创建选项（Design2 §4.4）：
// Activate=true 保持 Design1「创建即激活」；Activate=false 仅入池，由显式分发切换。
// AfterCreate 在新版本记录插入后、setCurrent 之前调用，用于 assembly_blueprints 等与 versions.id 1:1 的关联写入。
type CreateOptions struct {
	Activate    bool
	AfterCreate func(tx *sql.Tx, versionID int64, content []byte) error
}

// CreateVersion 单个 BEGIN IMMEDIATE 事务内：计算版本号（已有最大编号 + 1，禁止列表长度 + 1）
// → 写版本文件 → 写版本记录 → AfterCreate → 按 activate 语义切换当前指针 → 5 版上限驱逐。
// current==0（首版）时无论 Activate 取值均自动激活；该判定与切换在同一事务内完成（防双首版并发）。
// 返回 (新版本, 是否激活)。
func (s *Service) CreateVersion(ctx context.Context, ot OwnerType, ownerID int64, src ContentProvider, opts CreateOptions) (*Version, bool, error) {
	if src == nil {
		return nil, false, errors.New("版本内容来源缺失")
	}
	content, err := src.Content()
	if err != nil {
		return nil, false, err
	}
	if int64(len(content)) > MaxContentSize {
		return nil, false, ErrContentTooLarge
	}
	var created *Version
	activated := false
	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		// 版本号 = 已有最大编号 + 1（删除后不复用，Design1 §4.1）
		var maxNo int64
		if err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(version_no), 0) FROM versions WHERE owner_type = ? AND owner_id = ?`, ot, ownerID).
			Scan(&maxNo); err != nil {
			return err
		}
		newNo := maxNo + 1
		rel := versionRelPath(ot, ownerID, newNo)
		full := filepath.Join(s.dataDir, "contents", rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return fmt.Errorf("创建版本目录失败: %w", err)
		}
		if err := writeFileAtomic(full, content, 0o644); err != nil {
			return fmt.Errorf("写版本文件失败: %w", err)
		}
		// 文本模式/无原始文件名 → 按资源类型补默认名（下载文件名扩展名来源）
		fileName := src.FileName()
		if fileName == "" {
			fileName = defaultFileName(ot)
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO versions (owner_type, owner_id, version_no, file_path, file_name) VALUES (?,?,?,?,?)`,
			ot, ownerID, newNo, rel, fileName)
		if err != nil {
			_ = os.Remove(full) // 失败清理：删文件
			return fmt.Errorf("写版本记录失败: %w", err)
		}
		versionID, err := res.LastInsertId()
		if err != nil {
			_ = os.Remove(full)
			return err
		}
		// 关联写入先于 setCurrent：失败时 current 指针尚未改动，仅需删刚写文件
		if opts.AfterCreate != nil {
			if err := opts.AfterCreate(tx, versionID, content); err != nil {
				_ = os.Remove(full)
				return err
			}
		}
		// 激活语义：首版自动激活；非首版按 opts.Activate
		current, err := ownerCurrent(ctx, tx, ot, ownerID)
		if err != nil {
			_ = os.Remove(full)
			return err
		}
		effectiveCurrent := current
		if current == 0 || opts.Activate {
			if err := s.setCurrentLocked(ctx, tx, ot, ownerID, newNo); err != nil {
				_ = os.Remove(full)
				return err
			}
			activated = true
			effectiveCurrent = newNo
		}
		// 5 版上限：超出自动删最旧（文件 + 记录，不含当前激活版本）
		if err := s.evictOldest(ctx, tx, ot, ownerID, effectiveCurrent); err != nil {
			return err
		}
		created = &Version{ID: versionID, No: newNo, FilePath: rel, FileName: fileName, Current: activated}
		return nil
	})
	return created, activated, err
}

// writeFileAtomic 使用同目录唯一临时文件 + rename 原子替换，降低版本文件半写风险。
func writeFileAtomic(full string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(full)
	tmp := filepath.Join(dir, fmt.Sprintf(".tmp-%d-%s", time.Now().UnixNano(), strconv.Itoa(os.Getpid())))
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmp)
		}
	}()
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp, full); err != nil {
		return err
	}
	cleanup = false
	return nil
}

// TextContent 文本内容来源（指定文件名；供订阅 product_type 默认文件名与后续装配复用）
type TextContent struct {
	Text []byte
	Name string
}

func (t TextContent) Content() ([]byte, error) { return t.Text, nil }
func (t TextContent) FileName() string         { return t.Name }

// evictOldest 版本数 > MaxVersions 时删最旧（不删当前激活；文件 + 记录同步删，事务内完成）
func (s *Service) evictOldest(ctx context.Context, tx *sql.Tx, ot OwnerType, ownerID, currentNo int64) error {
	var total int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM versions WHERE owner_type = ? AND owner_id = ?`, ot, ownerID).Scan(&total); err != nil {
		return err
	}
	excess := total - MaxVersions
	if excess <= 0 {
		return nil
	}
	rows, err := tx.QueryContext(ctx,
		`SELECT version_no, file_path FROM versions
		 WHERE owner_type = ? AND owner_id = ? AND version_no != ?
		 ORDER BY version_no ASC LIMIT ?`, ot, ownerID, currentNo, excess)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var no int64
		var rel string
		if err := rows.Scan(&no, &rel); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM versions WHERE owner_type = ? AND owner_id = ? AND version_no = ?`, ot, ownerID, no); err != nil {
			return err
		}
		if err := os.Remove(filepath.Join(s.dataDir, "contents", rel)); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.log.Warn("驱逐最旧版本文件失败", "path", rel, "err", err) // 不阻断
		}
	}
	return rows.Err()
}

// SwitchVersion 原子切换——切 symlink（临时指针 + rename）→ 事务内更新 DB「当前」+ 刷新该版本时间戳
// （切回旧版本也刷新，首页反映「分发内容最近变动」，Design1 §4.1）
func (s *Service) SwitchVersion(ctx context.Context, ot OwnerType, ownerID, versionNo int64) error {
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM versions WHERE owner_type = ? AND owner_id = ? AND version_no = ?`, ot, ownerID, versionNo).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return ErrVersionNotFound
		}
		return s.setCurrentLocked(ctx, tx, ot, ownerID, versionNo)
	})
}

// setCurrentLocked DB 更新 owner 表 current_version + 版本行 updated_at；symlink 临时指针 + rename 原子替换
func (s *Service) setCurrentLocked(ctx context.Context, tx *sql.Tx, ot OwnerType, ownerID, versionNo int64) error {
	if err := updateOwnerCurrent(ctx, tx, ot, ownerID, versionNo); err != nil { // 按 ownerType 更新对应 owner 表
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE versions SET updated_at = CURRENT_TIMESTAMP WHERE owner_type = ? AND owner_id = ? AND version_no = ?`,
		ot, ownerID, versionNo); err != nil {
		return err
	}
	return s.rebuildSymlink(ot, ownerID, versionNo) // 临时文件 current.tmp + os.Rename 原子替换
}

// rebuildSymlink 重建当前版本 symlink：目标用相对路径（v{n}），临时指针 + rename 原子替换
func (s *Service) rebuildSymlink(ot OwnerType, ownerID, versionNo int64) error {
	dir := s.ownerDir(ot, ownerID)
	tmp := filepath.Join(dir, currentLink+".tmp")
	target := filepath.Join(dir, fmt.Sprintf("v%d", versionNo))
	// 清理残留临时指针
	if err := os.Remove(tmp); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("清理临时指针失败: %w", err)
	}
	if err := os.Symlink(filepath.Base(target), tmp); err != nil {
		return fmt.Errorf("创建临时指针失败: %w", err)
	}
	if err := os.Rename(tmp, filepath.Join(dir, currentLink)); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("原子替换指针失败: %w", err)
	}
	return nil
}

// updateOwnerCurrent 按 ownerType 更新对应 owner 表的 current_version
func updateOwnerCurrent(ctx context.Context, tx *sql.Tx, ot OwnerType, ownerID, versionNo int64) error {
	table := map[OwnerType]string{
		OwnerSubscription: "subscriptions",
		OwnerRule:         "rules",
		OwnerCustom:       "custom_subscriptions",
		OwnerShare:        "share_subscriptions",
	}[ot]
	if table == "" {
		return fmt.Errorf("非法资源类型: %s", ot)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE `+table+` SET current_version = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, versionNo, ownerID); err != nil {
		return fmt.Errorf("更新 %s 当前版本失败: %w", table, err)
	}
	return nil
}

// ownerCurrent 读取 owner 表当前版本号（无版本时 0）
func ownerCurrent(ctx context.Context, tx *sql.Tx, ot OwnerType, ownerID int64) (int64, error) {
	table := map[OwnerType]string{
		OwnerSubscription: "subscriptions",
		OwnerRule:         "rules",
		OwnerCustom:       "custom_subscriptions",
		OwnerShare:        "share_subscriptions",
	}[ot]
	if table == "" {
		return 0, fmt.Errorf("非法资源类型: %s", ot)
	}
	var current int64
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(current_version, 0) FROM `+table+` WHERE id = ?`, ownerID).Scan(&current); err != nil {
		return 0, err
	}
	return current, nil
}

// DeleteVersion 不可删最后一个；不可删当前激活版本（须先切换）；级联删文件（记录删除在事务内）
func (s *Service) DeleteVersion(ctx context.Context, ot OwnerType, ownerID, versionNo int64) error {
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var count int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM versions WHERE owner_type = ? AND owner_id = ?`, ot, ownerID).Scan(&count); err != nil {
			return err
		}
		if count <= 1 {
			return ErrLastVersion // 「不可删最后一个」，接入层映射 400
		}
		current, err := ownerCurrent(ctx, tx, ot, ownerID)
		if err != nil {
			return err
		}
		if current == versionNo {
			return ErrCurrentVersion // 「不可删当前激活版本（须先切换）」
		}
		var rel string
		if err := tx.QueryRowContext(ctx,
			`SELECT file_path FROM versions WHERE owner_type = ? AND owner_id = ? AND version_no = ?`, ot, ownerID, versionNo).Scan(&rel); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrVersionNotFound
			}
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM versions WHERE owner_type = ? AND owner_id = ? AND version_no = ?`, ot, ownerID, versionNo); err != nil {
			return err
		}
		if err := os.Remove(filepath.Join(s.dataDir, "contents", rel)); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.log.Warn("删除版本文件失败", "path", rel, "err", err) // 不阻断
		}
		return nil
	})
}

// CurrentNo 读取资源当前版本号（以 DB 记录为准，Design1 §4.1；供列表填充当前激活标记）
func (s *Service) CurrentNo(ctx context.Context, ot OwnerType, ownerID int64) (int64, error) {
	var current int64
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT COALESCE(current_version, 0) FROM `+ownerTable(ot)+` WHERE id = ?`, ownerID).Scan(&current); err != nil {
		return 0, err
	}
	return current, nil
}

// ListVersions 资源版本列表（当前激活标记由调用方传入 current 版本号填充）
func (s *Service) ListVersions(ctx context.Context, ot OwnerType, ownerID, currentNo int64) ([]Version, error) {
	hasBlueprint := hasTable(ctx, s.store.DB(), "assembly_blueprints")
	query := `SELECT v.id, v.version_no, v.file_path, v.file_name, v.created_at, v.updated_at`
	if hasBlueprint {
		query += `, EXISTS(SELECT 1 FROM assembly_blueprints b WHERE b.version_id = v.id)`
	}
	query += ` FROM versions v WHERE v.owner_type = ? AND v.owner_id = ? ORDER BY v.version_no`
	rows, err := s.store.DB().QueryContext(ctx, query, ot, ownerID)
	if err != nil {
		return nil, fmt.Errorf("读取版本列表失败: %w", err)
	}
	defer rows.Close()
	out := make([]Version, 0) // 空列表返回 [] 而非 null（前端 .map 安全）
	for rows.Next() {
		var v Version
		var blueprint int
		if hasBlueprint {
			if err := rows.Scan(&v.ID, &v.No, &v.FilePath, &v.FileName, &v.CreatedAt, &v.UpdatedAt, &blueprint); err != nil {
				return nil, fmt.Errorf("解析版本行失败: %w", err)
			}
		} else {
			if err := rows.Scan(&v.ID, &v.No, &v.FilePath, &v.FileName, &v.CreatedAt, &v.UpdatedAt); err != nil {
				return nil, fmt.Errorf("解析版本行失败: %w", err)
			}
		}
		v.Current = v.No == currentNo
		v.Blueprint = blueprint == 1
		out = append(out, v)
	}
	return out, rows.Err()
}

// PreviewVersion 读指定版本内容（供预览；接入层 text/plain + 禁缓存）
func (s *Service) PreviewVersion(ctx context.Context, ot OwnerType, ownerID, versionNo int64) ([]byte, error) {
	var rel string
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT file_path FROM versions WHERE owner_type = ? AND owner_id = ? AND version_no = ?`, ot, ownerID, versionNo).Scan(&rel); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrVersionNotFound
		}
		return nil, err
	}
	return os.ReadFile(filepath.Join(s.dataDir, "contents", rel))
}

// defaultFileName 无原始文件名（文本编辑模式）时按资源类型补默认名（扩展名来源）
func defaultFileName(ot OwnerType) string {
	switch ot {
	case OwnerRule:
		return "rule.conf" // Shadowrocket 分流规则
	default:
		return "subscription.yaml" // 订阅/分享/自定义
	}
}

// ReadCurrent 下载分发用——先查 DB 当前版本号再读对应版本文件（以 DB 为准，Design1 §4.1）
func (s *Service) ReadCurrent(ctx context.Context, ot OwnerType, ownerID int64) ([]byte, error) {
	content, _, err := s.readCurrentWithName(ctx, ot, ownerID)
	return content, err
}

// ReadCurrentWithName 读当前版本内容 + 原始文件名（下载端点拼文件名用，Issue1 R03）
func (s *Service) ReadCurrentWithName(ctx context.Context, ot OwnerType, ownerID int64) ([]byte, string, error) {
	return s.readCurrentWithName(ctx, ot, ownerID)
}

// readCurrentWithName 实现：先查 DB 当前版本号再读对应版本文件（以 DB 为准）
func (s *Service) readCurrentWithName(ctx context.Context, ot OwnerType, ownerID int64) ([]byte, string, error) {
	var current, versionNo int64
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT COALESCE(current_version, 0) FROM `+ownerTable(ot)+` WHERE id = ?`, ownerID).Scan(&current); err != nil {
		return nil, "", err
	}
	versionNo = current
	if versionNo == 0 { // 无版本（current=0）→ 视为无内容
		return nil, "", ErrVersionNotFound
	}
	var rel, fileName string
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT file_path, file_name FROM versions WHERE owner_type = ? AND owner_id = ? AND version_no = ?`, ot, ownerID, versionNo).Scan(&rel, &fileName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", ErrVersionNotFound
		}
		return nil, "", err
	}
	content, err := os.ReadFile(filepath.Join(s.dataDir, "contents", rel))
	return content, fileName, err
}

// hasTable 检查表是否存在（供可选关联列兼容旧测试/迁移前状态）
func hasTable(ctx context.Context, db *sql.DB, name string) bool {
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&n); err != nil {
		return false
	}
	return n > 0
}

// ownerTable 资源类型 → owner 表名（仅白名单固定值，防动态 SQL 注入）
func ownerTable(ot OwnerType) string {
	return map[OwnerType]string{
		OwnerSubscription: "subscriptions",
		OwnerRule:         "rules",
		OwnerCustom:       "custom_subscriptions",
		OwnerShare:        "share_subscriptions",
	}[ot]
}

// StartupCheck 启动自检——DB「当前」与 symlink 不一致时以 DB 为准重建 symlink（Design1 §4.1）
func (s *Service) StartupCheck(ctx context.Context) error {
	rows, err := s.store.DB().QueryContext(ctx, `SELECT DISTINCT owner_type, owner_id FROM versions`)
	if err != nil {
		return fmt.Errorf("读取版本归属集合失败: %w", err)
	}
	defer rows.Close()
	type pair struct {
		ot OwnerType
		id int64
	}
	var pairs []pair
	for rows.Next() {
		var ot OwnerType
		var id int64
		if err := rows.Scan(&ot, &id); err != nil {
			return err
		}
		pairs = append(pairs, pair{ot, id})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, p := range pairs {
		var current int64
		err := s.store.DB().QueryRowContext(ctx,
			`SELECT COALESCE(current_version, 0) FROM `+ownerTable(p.ot)+` WHERE id = ?`, p.id).Scan(&current)
		if err != nil { // owner 表缺失或资源不存在（理论不出现）：跳过并记日志
			s.log.Warn("启动自检跳过（owner 记录缺失）", "owner_type", p.ot, "owner_id", p.id, "err", err)
			continue
		}
		if current == 0 {
			continue
		}
		// 校验 versions 行存在再重建（防 DB 篡改后指向不存在版本）
		var cnt int
		if err := s.store.DB().QueryRowContext(ctx,
			`SELECT COUNT(*) FROM versions WHERE owner_type = ? AND owner_id = ? AND version_no = ?`,
			p.ot, p.id, current).Scan(&cnt); err != nil || cnt == 0 {
			continue
		}
		if err := s.rebuildSymlink(p.ot, p.id, current); err != nil {
			s.log.Warn("启动自检重建指针失败", "owner_type", p.ot, "owner_id", p.id, "err", err)
		}
	}
	return nil
}

// RemoveOwnerDir 删除资源全部版本文件目录（级联删除用；事务提交后调用，失败仅记日志）
func (s *Service) RemoveOwnerDir(ot OwnerType, ownerID int64) error {
	return os.RemoveAll(s.ownerDir(ot, ownerID))
}

// CollectVersionFiles 收集资源全部版本文件路径（级联删除事务内收集，提交后统一删除）
func (s *Service) CollectVersionFiles(ctx context.Context, tx *sql.Tx, ot OwnerType, ownerID int64) ([]string, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT file_path FROM versions WHERE owner_type = ? AND owner_id = ?`, ot, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var files []string
	for rows.Next() {
		var rel string
		if err := rows.Scan(&rel); err != nil {
			return nil, err
		}
		files = append(files, filepath.Join(s.dataDir, "contents", rel))
	}
	return files, rows.Err()
}

// DeleteVersionsTx 删除资源全部版本记录（级联删除事务内调用）
func (s *Service) DeleteVersionsTx(ctx context.Context, tx *sql.Tx, ot OwnerType, ownerID int64) error {
	_, err := tx.ExecContext(ctx,
		`DELETE FROM versions WHERE owner_type = ? AND owner_id = ?`, ot, ownerID)
	return err
}
