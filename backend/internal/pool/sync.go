// sync.go：per-source 快照同步、异常保护与 pending 操作。
package pool

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"vpn-sub/internal/rulespec"
	"vpn-sub/internal/store"
)

const (
	urlTimeout        = 60 * time.Second
	maxURLContentSize = 50 << 20
	taskRetentionDays = 7
)

// PerURLResult 单 URL 同步结果。
type PerURLResult struct {
	URL        string `json:"url"`
	SourceID   int64  `json:"source_id"`
	OK         bool   `json:"ok"`
	Format     string `json:"format,omitempty"`
	Profile    string `json:"profile,omitempty"`
	Accepted   int    `json:"accepted"`
	Excluded   int    `json:"excluded"`
	Rejected   int    `json:"rejected"`
	Duplicates int    `json:"duplicates"`
	Pending    bool   `json:"pending,omitempty"`
	Error      string `json:"error,omitempty"`
}

// SyncTask 同步任务。
type SyncTask struct {
	ID         int64          `json:"task_id"`
	PoolID     int64          `json:"pool_id"`
	Status     string         `json:"status"`
	PerURL     []PerURLResult `json:"per_url"`
	Error      string         `json:"error"`
	StartedAt  *time.Time     `json:"started_at"`
	FinishedAt *time.Time     `json:"finished_at"`
}

// SubmitSync 提交同步任务。
func (s *Service) SubmitSync(ctx context.Context, poolID int64) (int64, error) {
	var taskID int64
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if err := s.checkPoolTx(ctx, tx, poolID); err != nil {
			return err
		}
		var running int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM pool_sync_tasks WHERE pool_id=? AND status='running'`, poolID).Scan(&running); err != nil {
			return err
		}
		if running > 0 {
			return ErrSyncRunning
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO pool_sync_tasks (pool_id, status, per_url_json, started_at) VALUES (?,'running','[]',CURRENT_TIMESTAMP)`, poolID)
		if err != nil {
			return err
		}
		taskID, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	timer := time.AfterFunc(SyncTaskTimeout, func() { cancel(errSyncTimeout) })
	s.mu.Lock()
	s.cancels[taskID] = cancel
	s.mu.Unlock()
	go func() {
		defer func() {
			timer.Stop()
			s.mu.Lock()
			delete(s.cancels, taskID)
			s.mu.Unlock()
		}()
		s.runSyncTask(ctx, poolID, taskID)
	}()
	return taskID, nil
}

// CancelSync 取消运行中任务。
func (s *Service) CancelSync(ctx context.Context, poolID, taskID int64) error {
	var status string
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT status FROM pool_sync_tasks WHERE id=? AND pool_id=?`, taskID, poolID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "running" {
		return fmt.Errorf("%w: 任务不在运行中", ErrBadRequest)
	}
	s.mu.Lock()
	cancel, ok := s.cancels[taskID]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("%w: 任务无法取消（可能已结束或服务重启）", ErrBadRequest)
	}
	cancel(errSyncCancelled)
	return nil
}

// ClearFinishedTasks 手动清理当前素材池的全部终态同步历史（含成功/失败/部分成功）。
func (s *Service) ClearFinishedTasks(ctx context.Context, poolID int64) (int64, error) {
	if err := s.ensureExists(ctx, poolID); err != nil {
		return 0, err
	}
	res, err := s.store.DB().ExecContext(ctx,
		`DELETE FROM pool_sync_tasks WHERE pool_id=? AND finished_at IS NOT NULL
		 AND status IN ('succeeded','failed','partial')`, poolID)
	if err != nil {
		return 0, fmt.Errorf("清理同步历史失败: %w", err)
	}
	return res.RowsAffected()
}

// CleanupOldTasks 全局清理超过保留期的终态同步历史，不删除运行中任务和 active/pending 快照。
func (s *Service) CleanupOldTasks(ctx context.Context) (int64, error) {
	res, err := s.store.DB().ExecContext(ctx,
		`DELETE FROM pool_sync_tasks WHERE finished_at IS NOT NULL
		 AND finished_at < datetime('now', ?)`, fmt.Sprintf("-%d days", taskRetentionDays))
	if err != nil {
		return 0, fmt.Errorf("清理过期同步历史失败: %w", err)
	}
	return res.RowsAffected()
}

// GetStatus 读取最近一次任务。
func (s *Service) GetStatus(ctx context.Context, poolID int64) (*SyncTask, error) {
	if err := s.ensureExists(ctx, poolID); err != nil {
		return nil, err
	}
	row := s.store.DB().QueryRowContext(ctx,
		`SELECT id, pool_id, status, per_url_json, error, started_at, finished_at
		 FROM pool_sync_tasks WHERE pool_id=? ORDER BY id DESC LIMIT 1`, poolID)
	t, err := scanSyncTask(row)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	return t, err
}

// ListTasks 历史任务分页。
func (s *Service) ListTasks(ctx context.Context, poolID int64, page, pageSize int64) ([]SyncTask, int64, error) {
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
	var total int64
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pool_sync_tasks WHERE pool_id=?`, poolID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT id, pool_id, status, per_url_json, error, started_at, finished_at
		 FROM pool_sync_tasks WHERE pool_id=? ORDER BY id DESC LIMIT ? OFFSET ?`,
		poolID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]SyncTask, 0)
	for rows.Next() {
		t, err := scanSyncTask(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *t)
	}
	return out, total, rows.Err()
}

func scanSyncTask(row rowScanner) (*SyncTask, error) {
	var t SyncTask
	var perRaw string
	var started, finished sql.NullString
	err := row.Scan(&t.ID, &t.PoolID, &t.Status, &perRaw, &t.Error, &started, &finished)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t.PerURL = parsePerURL(perRaw)
	if started.Valid {
		if ts, err := parseDBTime(started.String); err == nil {
			t.StartedAt = &ts
		}
	}
	if finished.Valid {
		if ts, err := parseDBTime(finished.String); err == nil {
			t.FinishedAt = &ts
		}
	}
	return &t, nil
}

func parsePerURL(raw string) []PerURLResult {
	var out []PerURLResult
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return []PerURLResult{}
	}
	return out
}

func (s *Service) runSyncTask(ctx context.Context, poolID, taskID int64) {
	results := make([]PerURLResult, 0)
	allOK := true
	anyOK := false
	bgStore, err := store.Open(filepath.Dir(s.store.DBPath()), filepath.Base(s.store.DBPath()))
	if err != nil {
		s.failTask(ctx, poolID, taskID, results, "打开后台数据库连接失败: "+err.Error())
		return
	}
	defer bgStore.Close()

	var exists int
	if err := bgStore.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM rule_pools WHERE id=?`, poolID).Scan(&exists); err != nil {
		s.failTask(ctx, poolID, taskID, results, "检查素材池失败: "+err.Error())
		return
	}
	if exists == 0 {
		return
	}

	client := &http.Client{Timeout: urlTimeout}
	type urlSource struct {
		id   int64
		url  string
		mode string
	}
	var sources []urlSource
	rows, err := bgStore.DB().QueryContext(ctx,
		`SELECT id, COALESCE(url,''), source_mode FROM rule_pool_sources WHERE pool_id=? AND kind='url' ORDER BY sort_order,id`, poolID)
	if err != nil {
		s.failTask(ctx, poolID, taskID, results, "读取 URL 来源失败: "+err.Error())
		return
	}
	for rows.Next() {
		var src urlSource
		if err := rows.Scan(&src.id, &src.url, &src.mode); err != nil {
			_ = rows.Close()
			s.failTask(ctx, poolID, taskID, results, "读取 URL 来源失败: "+err.Error())
			return
		}
		sources = append(sources, src)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		s.failTask(ctx, poolID, taskID, results, "读取 URL 来源失败: "+err.Error())
		return
	}
	for _, src := range sources {
		r := s.syncOne(ctx, bgStore, client, poolID, src.id, src.url, SourceMode(src.mode))
		results = append(results, r)
		if r.OK {
			anyOK = true
		} else {
			allOK = false
		}
	}

	status := "failed"
	if anyOK {
		status = "partial"
	}
	if allOK && len(results) > 0 {
		status = "succeeded"
	}
	s.finishTask(ctx, bgStore, poolID, taskID, status, summarizeResults(results), results)
}

func (s *Service) syncOne(ctx context.Context, bgStore *store.Store, client *http.Client, poolID, sourceID int64, u string, mode SourceMode) PerURLResult {
	r := PerURLResult{URL: u, SourceID: sourceID}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	resp, err := client.Do(req)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		r.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return r
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxURLContentSize+1))
	if err != nil {
		r.Error = err.Error()
		return r
	}
	if len(body) > maxURLContentSize {
		r.Error = "内容超过 50MB"
		return r
	}
	parsed, err := ParseSource(body, mode)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	r.Format = string(parsed.Format)
	r.Profile = parsed.Profile
	r.Accepted = parsed.Accepted
	r.Excluded = parsed.Excluded
	r.Rejected = parsed.Rejected
	r.Duplicates = parsed.Duplicates
	r.OK = true

	err = bgStore.TxImmediate(ctx, func(tx *sql.Tx) error {
		id, pending, err := applyParseResultTx(ctx, tx, poolID, sourceID, parsed)
		if err != nil {
			return err
		}
		r.Pending = pending
		_ = id
		return nil
	})
	if err != nil {
		r.OK = false
		r.Error = err.Error()
	}
	return r
}

func applyParseResultTx(ctx context.Context, tx *sql.Tx, poolID, sourceID int64, parsed *ParseResult) (int64, bool, error) {
	// 读取旧 active 信息，用于异常保护。
	var oldFormat, oldProfile sql.NullString
	var oldAccepted sql.NullInt64
	var oldActiveID sql.NullInt64
	if err := tx.QueryRowContext(ctx,
		`SELECT s.id, s.format, s.profile, s.accepted_count
		 FROM rule_pool_sources src LEFT JOIN pool_source_snapshots s ON s.id = src.active_snapshot_id
		 WHERE src.id=?`, sourceID).Scan(&oldActiveID, &oldFormat, &oldProfile, &oldAccepted); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return 0, false, err
		}
	}

	diagJSON, _ := json.Marshal(parsed.Diagnostics)
	statsJSON, _ := json.Marshal(map[string]any{
		"input": parsed.Input, "recognized": parsed.Recognized,
		"accepted": parsed.Accepted, "excluded": parsed.Excluded,
		"rejected": parsed.Rejected, "duplicates": parsed.Duplicates,
	})
	res, err := tx.ExecContext(ctx,
		`INSERT INTO pool_source_snapshots
		   (source_id, format, profile, status, input_count, recognized_count, accepted_count, excluded_count, rejected_count, duplicate_count, diagnostic_json, stats_json)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		sourceID, string(parsed.Format), parsed.Profile, "staging", parsed.Input, parsed.Recognized,
		parsed.Accepted, parsed.Excluded, parsed.Rejected, parsed.Duplicates, string(diagJSON), string(statsJSON))
	if err != nil {
		return 0, false, err
	}
	snapshotID, err := res.LastInsertId()
	if err != nil {
		return 0, false, err
	}

	for _, rule := range parsed.Rules {
		canonicalID, err := ensureCanonicalTx(ctx, tx, poolID, rule)
		if err != nil {
			return 0, false, err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO pool_rule_origins (pool_id, canonical_rule_id, source_id, snapshot_id, sort_order, raw_line, line_no)
			 VALUES (?,?,?,?,?,?,0)`,
			poolID, canonicalID, sourceID, snapshotID, int64(len(parsed.Rules)), rule.SemanticKey()); err != nil {
			return 0, false, err
		}
	}

	pending := false
	if oldActiveID.Valid {
		if oldFormat.String != string(parsed.Format) || oldProfile.String != parsed.Profile {
			pending = true
		}
		if oldAccepted.Int64 >= 20 && int64(parsed.Accepted)*10 < oldAccepted.Int64*7 {
			pending = true
		}
	}
	status := "active"
	if pending {
		status = "pending"
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE pool_source_snapshots SET status=? WHERE id=?`, status, snapshotID); err != nil {
		return 0, false, err
	}
	if pending {
		if _, err := tx.ExecContext(ctx,
			`UPDATE rule_pool_sources SET pending_snapshot_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, snapshotID, sourceID); err != nil {
			return 0, false, err
		}
	} else {
		if _, err := tx.ExecContext(ctx,
			`UPDATE rule_pool_sources SET active_snapshot_id=?, pending_snapshot_id=NULL, updated_at=CURRENT_TIMESTAMP WHERE id=?`, snapshotID, sourceID); err != nil {
			return 0, false, err
		}
	}
	return snapshotID, pending, nil
}

func (s *Service) finishTask(ctx context.Context, bgStore *store.Store, poolID, taskID int64, status, errMsg string, results []PerURLResult) {
	perJSON, _ := json.Marshal(results)
	if status == "succeeded" {
		if _, err := bgStore.DB().ExecContext(ctx,
			`UPDATE pool_sync_tasks SET status=?, per_url_json=?, error='', finished_at=CURRENT_TIMESTAMP WHERE id=?`,
			status, string(perJSON), taskID); err != nil {
			s.log.Error("回写同步任务失败", "task_id", taskID, "err", err)
			return
		}
		if _, err := bgStore.DB().ExecContext(ctx,
			`UPDATE rule_pools SET last_synced_at=CURRENT_TIMESTAMP, sync_status='succeeded', sync_error='', updated_at=CURRENT_TIMESTAMP WHERE id=?`, poolID); err != nil {
			s.log.Error("回写池同步状态失败", "pool_id", poolID, "err", err)
		}
	} else {
		if _, err := bgStore.DB().ExecContext(ctx,
			`UPDATE pool_sync_tasks SET status=?, per_url_json=?, error=?, finished_at=CURRENT_TIMESTAMP WHERE id=?`,
			status, string(perJSON), errMsg, taskID); err != nil {
			s.log.Error("回写同步任务失败", "task_id", taskID, "err", err)
		}
		if _, err := bgStore.DB().ExecContext(ctx,
			`UPDATE rule_pools SET sync_status=?, sync_error=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, status, errMsg, poolID); err != nil {
			s.log.Error("回写池同步状态失败", "pool_id", poolID, "err", err)
		}
	}
	// 清理 7 天前的终态任务，不删除 active/pending 快照。
	_, _ = bgStore.DB().ExecContext(ctx,
		`DELETE FROM pool_sync_tasks WHERE pool_id=? AND finished_at IS NOT NULL AND finished_at < datetime('now', ?)`, poolID, fmt.Sprintf("-%d days", taskRetentionDays))
}

func (s *Service) failTask(ctx context.Context, poolID, taskID int64, results []PerURLResult, msg string) {
	perJSON, _ := json.Marshal(results)
	if _, err := s.store.DB().ExecContext(ctx,
		`UPDATE pool_sync_tasks SET status='failed', per_url_json=?, error=?, finished_at=CURRENT_TIMESTAMP WHERE id=?`,
		string(perJSON), msg, taskID); err != nil {
		s.log.Error("回写失败任务", "task_id", taskID, "err", err)
	}
	_, _ = s.store.DB().ExecContext(ctx,
		`UPDATE rule_pools SET sync_status='failed', sync_error=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, msg, poolID)
}

func summarizeResults(results []PerURLResult) string {
	var msgs []string
	for _, r := range results {
		if r.Error != "" {
			msgs = append(msgs, r.URL+": "+r.Error)
		}
	}
	return strings.Join(msgs, "；")
}

// ActivatePending 人工激活 pending 快照（Step 6 API 使用）。
func (s *Service) ActivatePending(ctx context.Context, poolID, sourceID, snapshotID int64) error {
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var current int64
		if err := tx.QueryRowContext(ctx,
			`SELECT pending_snapshot_id FROM rule_pool_sources WHERE id=? AND pool_id=?`, sourceID, poolID).Scan(&current); err != nil {
			return ErrNotFound
		}
		if current != snapshotID {
			return fmt.Errorf("%w: pending 快照已过期", ErrBadRequest)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE rule_pool_sources SET active_snapshot_id=?, pending_snapshot_id=NULL, updated_at=CURRENT_TIMESTAMP WHERE id=?`, snapshotID, sourceID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE pool_source_snapshots SET status='active' WHERE id=?`, snapshotID); err != nil {
			return err
		}
		return nil
	})
}

// DiscardPending 丢弃 pending 快照。
func (s *Service) DiscardPending(ctx context.Context, poolID, sourceID, snapshotID int64) error {
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var current int64
		if err := tx.QueryRowContext(ctx,
			`SELECT pending_snapshot_id FROM rule_pool_sources WHERE id=? AND pool_id=?`, sourceID, poolID).Scan(&current); err != nil {
			return ErrNotFound
		}
		if current != snapshotID {
			return fmt.Errorf("%w: pending 快照已过期", ErrBadRequest)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE rule_pool_sources SET pending_snapshot_id=NULL, updated_at=CURRENT_TIMESTAMP WHERE id=?`, sourceID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM pool_source_snapshots WHERE id=? AND status='pending'`, snapshotID); err != nil {
			return err
		}
		return s.cleanupOrphanCanonicalTx(ctx, tx, poolID)
	})
}

var _ = rulespec.CanonicalRule{}
var _ = slog.Logger{}
