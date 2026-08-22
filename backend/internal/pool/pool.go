// Package pool 提供规则素材池业务层：池/条目 CRUD、URL 同步任务与两段排序。
package pool

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"vpn-sub/internal/store"
)

// 关键参数（Design2 §2.2/§2.4）
const (
	URLBase         = 1 << 30 // url 段排序起点：manual 段恒小于 url 段
	MaxPoolNameLen  = 100
	MaxURLCount     = 50
	DefaultPageSize = 20
	MaxPageSize     = 100
	DefaultSyncTime = "04:00"
	// SyncTaskTimeout 素材池同步整体超时（用户决策 2026-08-22）。
	SyncTaskTimeout = 30 * time.Minute
)

// 业务错误
var (
	ErrNotFound      = errors.New("素材池不存在")
	ErrNameConflict  = errors.New("素材池名称已存在")
	ErrEntryConflict = errors.New("同类型同匹配值条目已存在")
	ErrBadRequest    = errors.New("参数错误")
	ErrSyncRunning   = errors.New("同步进行中，请等待完成")
)

var (
	errSyncTimeout  = errors.New("同步任务超时（30 分钟）")
	errSyncCancelled = errors.New("同步任务已取消")
)

// Service 规则素材池服务
type Service struct {
	store *store.Store
	log   *slog.Logger

	mu      sync.Mutex
	cancels map[int64]context.CancelCauseFunc // taskID → 取消函数（仅内存态，重启后任务由启动逻辑置 failed）
}

func NewService(st *store.Store, lg *slog.Logger) *Service {
	return &Service{store: st, log: lg, cancels: map[int64]context.CancelCauseFunc{}}
}

// Pool 素材池
type Pool struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	URLs         []string   `json:"urls"`
	EntryCount   int64      `json:"entry_count"`
	LastSyncedAt *time.Time `json:"last_synced_at"`
	SyncStatus   string     `json:"sync_status"`
	SyncError    string     `json:"sync_error"`
	AutoSync     bool       `json:"auto_sync"`
	SyncTime     string     `json:"sync_time"`
}

// Entry 素材池条目
type Entry struct {
	ID         int64  `json:"id"`
	PoolID     int64  `json:"pool_id"`
	RuleType   string `json:"rule_type"`
	MatchValue string `json:"match_value"`
	Source     string `json:"source"` // url / manual
	SortOrder  int64  `json:"sort_order"`
}

var syncTimeRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// ValidateURLs 校验 URL 列表：http/https、去重、禁止控制字符
func ValidateURLs(urls []string) ([]string, error) {
	out := make([]string, 0, len(urls))
	seen := map[string]bool{}
	for _, raw := range urls {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue // 空行忽略
		}
		if containsControl(v) {
			return nil, errors.New("URL 含控制字符")
		}
		u, err := url.Parse(v)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, fmt.Errorf("URL 仅支持 http/https 地址: %s", v)
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out, nil
}

// Create 创建素材池：名称唯一、URL/sync_time 校验
func (s *Service) Create(ctx context.Context, name string, urls []string, autoSync bool, syncTime string) (*Pool, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > MaxPoolNameLen {
		return nil, fmt.Errorf("%w: 名称不能为空且不超过 %d 字符", ErrBadRequest, MaxPoolNameLen)
	}
	urls, err := ValidateURLs(urls)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	if syncTime == "" {
		syncTime = DefaultSyncTime
	}
	if !syncTimeRe.MatchString(syncTime) {
		return nil, fmt.Errorf("%w: 同步时刻须为 HH:MM", ErrBadRequest)
	}
	var created *Pool
	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var dup int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM rule_pools WHERE name = ?`, name).Scan(&dup); err != nil {
			return err
		}
		if dup > 0 {
			return ErrNameConflict
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO rule_pools (name, urls_json, auto_sync, sync_time) VALUES (?,?,?,?)`,
			name, toJSON(urls), boolInt(autoSync), syncTime)
		if err != nil {
			return fmt.Errorf("创建素材池失败: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		created = &Pool{ID: id, Name: name, URLs: urls, AutoSync: autoSync, SyncTime: syncTime}
		return nil
	})
	return created, err
}

// Update 更新素材池：改名唯一、URL 列表、定时开关与时刻
func (s *Service) Update(ctx context.Context, id int64, name string, urls []string, autoSync bool, syncTime string) error {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > MaxPoolNameLen {
		return fmt.Errorf("%w: 名称不能为空且不超过 %d 字符", ErrBadRequest, MaxPoolNameLen)
	}
	urls, err := ValidateURLs(urls)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	if syncTime == "" {
		syncTime = DefaultSyncTime
	}
	if !syncTimeRe.MatchString(syncTime) {
		return fmt.Errorf("%w: 同步时刻须为 HH:MM", ErrBadRequest)
	}
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM rule_pools WHERE id = ?`, id).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return ErrNotFound
		}
		var dup int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM rule_pools WHERE name = ? AND id != ?`, name, id).Scan(&dup); err != nil {
			return err
		}
		if dup > 0 {
			return ErrNameConflict
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE rule_pools SET name = ?, urls_json = ?, auto_sync = ?, sync_time = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			name, toJSON(urls), boolInt(autoSync), syncTime, id); err != nil {
			return fmt.Errorf("更新素材池失败: %w", err)
		}
		return nil
	})
}

// List 池列表（含条目数）
func (s *Service) List(ctx context.Context) ([]Pool, error) {
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT p.id, p.name, p.urls_json, p.last_synced_at, p.sync_status, p.sync_error, p.auto_sync, p.sync_time,
		        (SELECT COUNT(*) FROM pool_entries e WHERE e.pool_id = p.id)
		 FROM rule_pools p ORDER BY p.id`)
	if err != nil {
		return nil, fmt.Errorf("读取素材池列表失败: %w", err)
	}
	defer rows.Close()
	out := make([]Pool, 0)
	for rows.Next() {
		p, err := scanPoolRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPoolRow(row rowScanner) (Pool, error) {
	var p Pool
	var urlsRaw string
	var last sql.NullString
	var auto int
	if err := row.Scan(&p.ID, &p.Name, &urlsRaw, &last, &p.SyncStatus, &p.SyncError, &auto, &p.SyncTime, &p.EntryCount); err != nil {
		return Pool{}, err
	}
	p.URLs = parseStringSlice(urlsRaw)
	p.AutoSync = auto == 1
	if last.Valid && last.String != "" {
		if t, err := parseDBTime(last.String); err == nil {
			p.LastSyncedAt = &t
		}
	}
	return p, nil
}

// Delete 删除素材池（条目与同步任务随外键级联）
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM rule_pools WHERE id = ?`, id).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return ErrNotFound
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM rule_pools WHERE id = ?`, id); err != nil {
			return fmt.Errorf("删除素材池失败: %w", err)
		}
		return nil
	})
}

// --- 条目 CRUD（仅 manual 来源） ---

// ListEntries 分页读取条目（渲染顺序 = sort_order,id；数万行不整表加载）；池不存在返回 ErrNotFound
func (s *Service) ListEntries(ctx context.Context, poolID, page, pageSize int64) ([]Entry, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	var exists int
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM rule_pools WHERE id = ?`, poolID).Scan(&exists); err != nil {
		return nil, 0, err
	}
	if exists == 0 {
		return nil, 0, ErrNotFound
	}
	var total int64
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pool_entries WHERE pool_id = ?`, poolID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT id, pool_id, rule_type, match_value, source, sort_order
		 FROM pool_entries WHERE pool_id = ? ORDER BY sort_order, id LIMIT ? OFFSET ?`,
		poolID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]Entry, 0)
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

func scanEntry(row rowScanner) (Entry, error) {
	var e Entry
	err := row.Scan(&e.ID, &e.PoolID, &e.RuleType, &e.MatchValue, &e.Source, &e.SortOrder)
	return e, err
}

// CreateEntry 手动新增条目（manual 段追加）
func (s *Service) CreateEntry(ctx context.Context, poolID int64, ruleType, matchValue string) (*Entry, error) {
	normType, value, err := ValidateEntry(ruleType, matchValue)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	var created *Entry
	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if err := s.checkPoolTx(ctx, tx, poolID); err != nil {
			return err
		}
		var dup int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM pool_entries WHERE pool_id = ? AND rule_type = ? AND match_value = ?`,
			poolID, normType, value).Scan(&dup); err != nil {
			return err
		}
		if dup > 0 {
			return ErrEntryConflict
		}
		order, err := nextManualOrderTx(ctx, tx, poolID)
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO pool_entries (pool_id, rule_type, match_value, source, sort_order) VALUES (?,?,?, 'manual', ?)`,
			poolID, normType, value, order)
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		created = &Entry{ID: id, PoolID: poolID, RuleType: normType, MatchValue: value, Source: "manual", SortOrder: order}
		return nil
	})
	return created, err
}

// UpdateEntry 修改 manual 条目（类型/匹配值可改；去重冲突 409）
func (s *Service) UpdateEntry(ctx context.Context, entryID int64, ruleType, matchValue string) error {
	normType, value, err := ValidateEntry(ruleType, matchValue)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var poolID int64
		var source string
		if err := tx.QueryRowContext(ctx,
			`SELECT pool_id, source FROM pool_entries WHERE id = ?`, entryID).Scan(&poolID, &source); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if source != "manual" {
			return fmt.Errorf("%w: 仅 manual 条目可编辑", ErrBadRequest)
		}
		var dup int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM pool_entries WHERE pool_id = ? AND rule_type = ? AND match_value = ? AND id != ?`,
			poolID, normType, value, entryID).Scan(&dup); err != nil {
			return err
		}
		if dup > 0 {
			return ErrEntryConflict
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE pool_entries SET rule_type = ?, match_value = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			normType, value, entryID); err != nil {
			return err
		}
		return nil
	})
}

// DeleteEntry 删除 manual 条目（url 条目不可手动删除）
func (s *Service) DeleteEntry(ctx context.Context, entryID int64) error {
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var source string
		if err := tx.QueryRowContext(ctx, `SELECT source FROM pool_entries WHERE id = ?`, entryID).Scan(&source); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if source != "manual" {
			return fmt.Errorf("%w: 仅 manual 条目可删除", ErrBadRequest)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM pool_entries WHERE id = ?`, entryID); err != nil {
			return err
		}
		return nil
	})
}

// checkPoolTx 事务内池存在性校验
func (s *Service) checkPoolTx(ctx context.Context, tx *sql.Tx, id int64) error {
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM rule_pools WHERE id = ?`, id).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// nextManualOrderTx manual 段内追加顺序（0 起；不穿越 url 段）
func nextManualOrderTx(ctx context.Context, tx *sql.Tx, poolID int64) (int64, error) {
	var max int64
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(sort_order), -1) FROM pool_entries WHERE pool_id = ? AND sort_order < ?`,
		poolID, URLBase).Scan(&max); err != nil {
		return 0, err
	}
	return max + 1, nil
}

func containsControl(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func toJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func parseStringSlice(raw string) []string {
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return []string{}
	}
	return out
}

// parseDBTime 兼容 SQLite 返回的 RFC3339 与 `YYYY-MM-DD HH:MM:SS` 形态
func parseDBTime(raw string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02 15:04:05", raw)
}
