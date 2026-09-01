// Package pool 提供规则素材池业务层：池/来源/Canonical Rule CRUD、快照查询与同步入口。
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

	"vpn-sub/internal/rulespec"
	"vpn-sub/internal/store"
)

// 关键参数（Design2 §2.2/§2.4 与 Design3 §6.4）
const (
	MaxPoolNameLen  = 100
	MaxURLCount     = 50
	DefaultPageSize = 20
	MaxPageSize     = 100
	DefaultSyncTime = "04:00"
	SyncTaskTimeout = 30 * time.Minute
	URLBase         = 1 << 30 // 兼容排序：manual 段恒小于 url 段
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
	errSyncTimeout   = errors.New("同步任务超时（30 分钟）")
	errSyncCancelled = errors.New("同步任务已取消")
)

// Service 规则素材池服务
type Service struct {
	store *store.Store
	log   *slog.Logger

	mu      sync.Mutex
	cancels map[int64]context.CancelCauseFunc
}

func NewService(st *store.Store, lg *slog.Logger) *Service {
	return &Service{store: st, log: lg, cancels: map[int64]context.CancelCauseFunc{}}
}

// Pool 素材池
type Pool struct {
	ID           int64        `json:"id"`
	Name         string       `json:"name"`
	Sources      []PoolSource `json:"sources"`
	URLs         []string     `json:"urls"`
	EntryCount   int64        `json:"entry_count"`
	LastSyncedAt *time.Time   `json:"last_synced_at"`
	SyncStatus   string       `json:"sync_status"`
	SyncError    string       `json:"sync_error"`
	AutoSync     bool         `json:"auto_sync"`
	SyncTime     string       `json:"sync_time"`
}

// PoolSource 来源
type PoolSource struct {
	ID                int64      `json:"id"`
	PoolID            int64      `json:"pool_id"`
	Kind              string     `json:"kind"` // manual / url
	URL               string     `json:"url,omitempty"`
	SourceMode        SourceMode `json:"source_mode"`
	SortOrder         int64      `json:"sort_order"`
	ActiveSnapshotID  *int64     `json:"active_snapshot_id,omitempty"`
	PendingSnapshotID *int64     `json:"pending_snapshot_id,omitempty"`
}

// SourceInput 创建/更新时的来源输入。
type SourceInput struct {
	URL        string     `json:"url"`
	SourceMode SourceMode `json:"source_mode"`
}

// Entry 兼容层条目（旧 API/装配读取用；Step 5 后改为 Canonical 读取）。
type Entry struct {
	ID         int64  `json:"id"`
	PoolID     int64  `json:"pool_id"`
	RuleType   string `json:"rule_type"`
	MatchValue string `json:"match_value"`
	Source     string `json:"source"`
	SortOrder  int64  `json:"sort_order"`
}

var syncTimeRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// ValidateURLs 校验 URL 列表（兼容旧入口）。
func ValidateURLs(urls []string) ([]string, error) {
	out := make([]string, 0, len(urls))
	seen := map[string]bool{}
	for _, raw := range urls {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
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

func validateSources(sources []SourceInput) ([]SourceInput, error) {
	if len(sources) > MaxURLCount {
		return nil, fmt.Errorf("%w: URL 数量不能超过 %d", ErrBadRequest, MaxURLCount)
	}
	out := make([]SourceInput, 0, len(sources))
	seen := map[string]bool{}
	for _, s := range sources {
		v := strings.TrimSpace(s.URL)
		if v == "" {
			continue
		}
		if containsControl(v) {
			return nil, fmt.Errorf("%w: URL 含控制字符", ErrBadRequest)
		}
		u, err := url.Parse(v)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, fmt.Errorf("%w: URL 仅支持 http/https 地址: %s", ErrBadRequest, v)
		}
		if seen[v] {
			return nil, fmt.Errorf("%w: URL 重复: %s", ErrBadRequest, v)
		}
		seen[v] = true
		mode := s.SourceMode
		if mode == "" {
			mode = SourceModeAuto
		}
		if mode != SourceModeClash && mode != SourceModeShadowrocket && mode != SourceModeAuto {
			return nil, fmt.Errorf("%w: 来源模式非法: %s", ErrBadRequest, mode)
		}
		out = append(out, SourceInput{URL: v, SourceMode: mode})
	}
	return out, nil
}

// Create 创建素材池：名称唯一、来源列表事务校验。
func (s *Service) Create(ctx context.Context, name string, sources []SourceInput, autoSync bool, syncTime string) (*Pool, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > MaxPoolNameLen {
		return nil, fmt.Errorf("%w: 名称不能为空且不超过 %d 字符", ErrBadRequest, MaxPoolNameLen)
	}
	sources, err := validateSources(sources)
	if err != nil {
		return nil, err
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
			`INSERT INTO rule_pools (name, auto_sync, sync_time) VALUES (?,?,?)`,
			name, boolInt(autoSync), syncTime)
		if err != nil {
			return fmt.Errorf("创建素材池失败: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		created = &Pool{ID: id, Name: name, AutoSync: autoSync, SyncTime: syncTime}
		for i, src := range sources {
			sid, err := insertSourceTx(ctx, tx, id, "url", src.URL, src.SourceMode, int64(i))
			if err != nil {
				return err
			}
			created.Sources = append(created.Sources, PoolSource{ID: sid, PoolID: id, Kind: "url", URL: src.URL, SourceMode: src.SourceMode, SortOrder: int64(i)})
			created.URLs = append(created.URLs, src.URL)
		}
		return nil
	})
	return created, err
}

func insertSourceTx(ctx context.Context, tx *sql.Tx, poolID int64, kind, u string, mode SourceMode, sortOrder int64) (int64, error) {
	res, err := tx.ExecContext(ctx,
		`INSERT INTO rule_pool_sources (pool_id, kind, url, source_mode, sort_order) VALUES (?,?,?,?,?)`,
		poolID, kind, nullIfEmpty(u), string(mode), sortOrder)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Update 更新素材池：全量替换 URL 来源；手工来源/规则保留。
func (s *Service) Update(ctx context.Context, id int64, name string, sources []SourceInput, autoSync bool, syncTime string) error {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > MaxPoolNameLen {
		return fmt.Errorf("%w: 名称不能为空且不超过 %d 字符", ErrBadRequest, MaxPoolNameLen)
	}
	sources, err := validateSources(sources)
	if err != nil {
		return err
	}
	if syncTime == "" {
		syncTime = DefaultSyncTime
	}
	if !syncTimeRe.MatchString(syncTime) {
		return fmt.Errorf("%w: 同步时刻须为 HH:MM", ErrBadRequest)
	}
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if err := s.checkPoolTx(ctx, tx, id); err != nil {
			return err
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
			`UPDATE rule_pools SET name = ?, auto_sync = ?, sync_time = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			name, boolInt(autoSync), syncTime, id); err != nil {
			return fmt.Errorf("更新素材池失败: %w", err)
		}
		// 全量替换 URL 来源，并清理无手工 origin 的孤立 canonical。
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM rule_pool_sources WHERE pool_id = ? AND kind = 'url'`, id); err != nil {
			return err
		}
		if err := s.cleanupOrphanCanonicalTx(ctx, tx, id); err != nil {
			return err
		}
		for i, src := range sources {
			if _, err := insertSourceTx(ctx, tx, id, "url", src.URL, src.SourceMode, int64(i)); err != nil {
				return err
			}
		}
		return nil
	})
}

// List 池列表（含来源与活跃规则数）。
func (s *Service) List(ctx context.Context) ([]Pool, error) {
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT p.id, p.name, p.last_synced_at, p.sync_status, p.sync_error, p.auto_sync, p.sync_time,
		        (SELECT COUNT(DISTINCT cr.id) FROM pool_canonical_rules cr
		          JOIN pool_rule_origins o ON o.canonical_rule_id = cr.id
		          JOIN rule_pool_sources src ON src.id = o.source_id
		          WHERE cr.pool_id = p.id
		            AND ((src.kind='manual' AND o.snapshot_id IS NULL)
		              OR (src.kind='url' AND o.snapshot_id = src.active_snapshot_id)))
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if err := s.loadSources(ctx, &out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPoolRow(row rowScanner) (Pool, error) {
	var p Pool
	var last sql.NullString
	var auto int
	if err := row.Scan(&p.ID, &p.Name, &last, &p.SyncStatus, &p.SyncError, &auto, &p.SyncTime, &p.EntryCount); err != nil {
		return Pool{}, err
	}
	p.AutoSync = auto == 1
	if last.Valid && last.String != "" {
		if t, err := parseDBTime(last.String); err == nil {
			p.LastSyncedAt = &t
		}
	}
	return p, nil
}

func (s *Service) loadSources(ctx context.Context, p *Pool) error {
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT id, pool_id, kind, COALESCE(url,''), source_mode, sort_order, active_snapshot_id, pending_snapshot_id
		 FROM rule_pool_sources WHERE pool_id = ? ORDER BY sort_order, id`, p.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	p.Sources = nil
	p.URLs = nil
	for rows.Next() {
		var src PoolSource
		var mode string
		var active, pending sql.NullInt64
		if err := rows.Scan(&src.ID, &src.PoolID, &src.Kind, &src.URL, &mode, &src.SortOrder, &active, &pending); err != nil {
			return err
		}
		src.SourceMode = SourceMode(mode)
		if active.Valid {
			v := active.Int64
			src.ActiveSnapshotID = &v
		}
		if pending.Valid {
			v := pending.Int64
			src.PendingSnapshotID = &v
		}
		p.Sources = append(p.Sources, src)
		if src.Kind == "url" {
			p.URLs = append(p.URLs, src.URL)
		}
	}
	return rows.Err()
}

// Delete 删除素材池（新表级联）。
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if err := s.checkPoolTx(ctx, tx, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM rule_pools WHERE id = ?`, id); err != nil {
			return fmt.Errorf("删除素材池失败: %w", err)
		}
		return nil
	})
}

// ListEntries 读取活跃 Canonical Rule（兼容旧条目形状）。
func (s *Service) ListEntries(ctx context.Context, poolID, page, pageSize int64, source string) ([]Entry, int64, error) {
	if source != "" && source != "manual" && source != "url" {
		return nil, 0, fmt.Errorf("%w: 条目来源仅支持 manual 或 url", ErrBadRequest)
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	if err := s.ensureExists(ctx, poolID); err != nil {
		return nil, 0, err
	}
	where := `cr.pool_id = ? AND ((src.kind='manual' AND o.snapshot_id IS NULL) OR (src.kind='url' AND o.snapshot_id = src.active_snapshot_id))`
	args := []any{poolID}
	if source != "" {
		where += ` AND src.kind = ?`
		args = append(args, source)
	}
	var total int64
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT cr.id) FROM pool_canonical_rules cr
		 JOIN pool_rule_origins o ON o.canonical_rule_id = cr.id
		 JOIN rule_pool_sources src ON src.id = o.source_id
		 WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT cr.id, cr.pool_id, cr.family, cr.matcher, cr.value, cr.options_json, src.kind, src.sort_order
		 FROM pool_canonical_rules cr
		 JOIN pool_rule_origins o ON o.canonical_rule_id = cr.id
		 JOIN rule_pool_sources src ON src.id = o.source_id
		 WHERE `+where+`
		 ORDER BY CASE WHEN src.kind='manual' THEN 0 ELSE 1 END, src.sort_order, cr.id
		 LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]Entry, 0, pageSize)
	seen := map[int64]bool{}
	for rows.Next() {
		var id, poolID2 int64
		var family, matcher, value, optionsRaw, kind string
		var sortOrder int64
		if err := rows.Scan(&id, &poolID2, &family, &matcher, &value, &optionsRaw, &kind, &sortOrder); err != nil {
			return nil, 0, err
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		var opts rulespec.RuleOptions
		_ = json.Unmarshal([]byte(optionsRaw), &opts)
		rule := rulespec.CanonicalRule{Family: rulespec.Family(family), Matcher: rulespec.Matcher(matcher), Value: value, Options: opts}
		out = append(out, Entry{
			ID: id, PoolID: poolID, RuleType: legacyTypeForCanonical(rule), MatchValue: value,
			Source: kind, SortOrder: sortOrder,
		})
	}
	return out, total, rows.Err()
}

// CreateEntry 新增手工 Canonical Rule（确保 manual source 存在）。
func (s *Service) CreateEntry(ctx context.Context, poolID int64, ruleType, matchValue string) (*Entry, error) {
	canonical, legacyType, value, err := canonicalFromLegacyInput(ruleType, matchValue)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	var created *Entry
	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if err := s.checkPoolTx(ctx, tx, poolID); err != nil {
			return err
		}
		sourceID, err := ensureManualSourceTx(ctx, tx, poolID)
		if err != nil {
			return err
		}
		semanticKey := canonical.SemanticKey()
		canonicalID, err := ensureCanonicalTx(ctx, tx, poolID, canonical)
		if err != nil {
			return err
		}
		var existing int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM pool_rule_origins WHERE pool_id=? AND canonical_rule_id=? AND source_id=? AND snapshot_id IS NULL`,
			poolID, canonicalID, sourceID).Scan(&existing); err != nil {
			return err
		}
		if existing > 0 {
			return ErrEntryConflict
		}
		order, err := nextManualOrderTx(ctx, tx, poolID)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO pool_rule_origins (pool_id, canonical_rule_id, source_id, snapshot_id, sort_order, raw_line, line_no)
			 VALUES (?,?,?,NULL,?,?,0)`,
			poolID, canonicalID, sourceID, order, ruleType+","+matchValue); err != nil {
			return err
		}
		created = &Entry{ID: canonicalID, PoolID: poolID, RuleType: legacyType, MatchValue: value, Source: "manual", SortOrder: order}
		_ = semanticKey
		return nil
	})
	return created, err
}

// UpdateEntry 修改手工 Canonical Rule（简单实现：更新共用 canonical；后续 Step 5 再收口 origin）。
func (s *Service) UpdateEntry(ctx context.Context, entryID int64, ruleType, matchValue string) error {
	canonical, legacyType, value, err := canonicalFromLegacyInput(ruleType, matchValue)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	_ = legacyType
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var poolID int64
		var source string
		if err := tx.QueryRowContext(ctx,
			`SELECT cr.pool_id, src.kind
			 FROM pool_canonical_rules cr
			 JOIN pool_rule_origins o ON o.canonical_rule_id = cr.id
			 JOIN rule_pool_sources src ON src.id = o.source_id
			 WHERE cr.id = ? AND o.snapshot_id IS NULL AND src.kind='manual'
			 LIMIT 1`, entryID).Scan(&poolID, &source); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if source != "manual" {
			return fmt.Errorf("%w: 仅 manual 条目可编辑", ErrBadRequest)
		}
		optsJSON, _ := json.Marshal(canonical.Options)
		res, err := tx.ExecContext(ctx,
			`UPDATE pool_canonical_rules SET family=?, matcher=?, value=?, options_json=?, semantic_key=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
			string(canonical.Family), string(canonical.Matcher), canonical.Value, string(optsJSON), canonical.SemanticKey(), entryID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		_ = value
		return nil
	})
}

// DeleteEntry 删除手工 Canonical Rule origin。
func (s *Service) DeleteEntry(ctx context.Context, entryID int64) error {
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var poolID int64
		var source string
		var canonicalID int64
		err := tx.QueryRowContext(ctx,
			`SELECT cr.pool_id, src.kind, cr.id
			 FROM pool_canonical_rules cr
			 JOIN pool_rule_origins o ON o.canonical_rule_id = cr.id
			 JOIN rule_pool_sources src ON src.id = o.source_id
			 WHERE cr.id = ? AND o.snapshot_id IS NULL AND src.kind='manual'
			 LIMIT 1`, entryID).Scan(&poolID, &source, &canonicalID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if source != "manual" {
			return fmt.Errorf("%w: 仅 manual 条目可删除", ErrBadRequest)
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM pool_rule_origins WHERE canonical_rule_id = ? AND snapshot_id IS NULL AND source_id IN (SELECT id FROM rule_pool_sources WHERE pool_id=? AND kind='manual')`,
			canonicalID, poolID); err != nil {
			return err
		}
		return s.cleanupOrphanCanonicalTx(ctx, tx, poolID)
	})
}

func canonicalFromLegacyInput(ruleType, matchValue string) (rulespec.CanonicalRule, string, string, error) {
	typ, normalized, err := rulespec.ValidateValue(ruleType, matchValue)
	if err != nil {
		return rulespec.CanonicalRule{}, "", "", err
	}
	family, matcher, ok := rulespec.CanonicalizeLegacyType(typ)
	if !ok {
		return rulespec.CanonicalRule{}, "", "", fmt.Errorf("不支持的类型: %s", typ)
	}
	rule := rulespec.CanonicalRule{Family: family, Matcher: matcher, Value: normalized}
	if strings.Contains(strings.ToLower(matchValue), "no-resolve") {
		rule.Options.NoResolve = true
	}
	return rule, typ, normalized, nil
}

func ensureManualSourceTx(ctx context.Context, tx *sql.Tx, poolID int64) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM rule_pool_sources WHERE pool_id=? AND kind='manual'`, poolID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	res, err := tx.ExecContext(ctx,
		`INSERT INTO rule_pool_sources (pool_id, kind, source_mode, sort_order) VALUES (?,'manual','auto',-1)`, poolID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func ensureCanonicalTx(ctx context.Context, tx *sql.Tx, poolID int64, rule rulespec.CanonicalRule) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM pool_canonical_rules WHERE pool_id=? AND semantic_key=?`,
		poolID, rule.SemanticKey()).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	opts, _ := json.Marshal(rule.Options)
	res, err := tx.ExecContext(ctx,
		`INSERT INTO pool_canonical_rules (pool_id, semantic_key, family, matcher, value, options_json) VALUES (?,?,?,?,?,?)`,
		poolID, rule.SemanticKey(), string(rule.Family), string(rule.Matcher), rule.Value, string(opts))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Service) cleanupOrphanCanonicalTx(ctx context.Context, tx *sql.Tx, poolID int64) error {
	_, err := tx.ExecContext(ctx,
		`DELETE FROM pool_canonical_rules WHERE pool_id=? AND id NOT IN (
		   SELECT DISTINCT canonical_rule_id FROM pool_rule_origins WHERE pool_id=?)`, poolID, poolID)
	return err
}

func nextManualOrderTx(ctx context.Context, tx *sql.Tx, poolID int64) (int64, error) {
	var max int64
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(o.sort_order), -1) FROM pool_rule_origins o
		 JOIN rule_pool_sources src ON src.id=o.source_id
		 WHERE o.pool_id=? AND src.kind='manual' AND o.snapshot_id IS NULL`,
		poolID).Scan(&max); err != nil {
		return 0, err
	}
	return max + 1, nil
}

func legacyTypeForCanonical(rule rulespec.CanonicalRule) string {
	switch rule.Family {
	case rulespec.FamilyDomain:
		switch rule.Matcher {
		case rulespec.MatcherExact:
			return "DOMAIN"
		case rulespec.MatcherSuffix:
			return "DOMAIN-SUFFIX"
		case rulespec.MatcherKeyword:
			return "DOMAIN-KEYWORD"
		case rulespec.MatcherRegex:
			return "DOMAIN-REGEX"
		}
	case rulespec.FamilyIP:
		switch rule.Matcher {
		case rulespec.MatcherCIDR:
			if strings.Contains(rule.Value, ":") {
				return "IP-CIDR6"
			}
			return "IP-CIDR"
		case rulespec.MatcherASN:
			return "IP-ASN"
		}
	case rulespec.FamilyUserAgent:
		return "USER-AGENT"
	case rulespec.FamilyProcess:
		if rule.Matcher == rulespec.MatcherRegex {
			return "PROCESS-NAME-REGEX"
		}
		return "PROCESS-NAME"
	}
	return ""
}

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

func (s *Service) ensureExists(ctx context.Context, id int64) error {
	var n int
	if err := s.store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM rule_pools WHERE id=?`, id).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
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

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
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

func parseDBTime(raw string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02 15:04:05", raw)
}
