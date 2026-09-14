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
	"unicode/utf8"

	"vpn-sub/internal/redact"
	"vpn-sub/internal/rulespec"
	"vpn-sub/internal/store"
)

const (
	urlTimeout        = 60 * time.Second
	maxURLContentSize = 50 << 20
	taskRetentionDays = 7
	terminalWriteTime = 5 * time.Second
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

// sourceFailure 携带稳定 reason_code 的同时保留用户可见错误文本。
// 具体 cause 可为 sentinel error，便于 errors.Is/As 继续分类。
type sourceFailure struct {
	reason  string
	message string
	cause   error
}

func (e *sourceFailure) Error() string { return e.message }
func (e *sourceFailure) Unwrap() error { return e.cause }

func newSourceFailure(reason, message string, cause error) error {
	return &sourceFailure{reason: reason, message: message, cause: cause}
}

// parseFailureReason 将 ParseSource 的 sentinel error 映射为失败 reason_code。
func parseFailureReason(err error) string {
	switch {
	case errors.Is(err, ErrUnrecognizedSource):
		return "unrecognized_source"
	case errors.Is(err, ErrAmbiguousSourceFormat):
		return "ambiguous_format"
	case errors.Is(err, ErrConflictingDocumentFormat):
		return "conflicting_format"
	case errors.Is(err, ErrMixedPlatformSource):
		return "mixed_platform"
	case errors.Is(err, ErrHTMLSource):
		return "html_source"
	case errors.Is(err, ErrNoAcceptedRules):
		return "no_accepted_rules"
	case errors.Is(err, ErrThresholdNotMet):
		return "recognition_threshold_not_met"
	default:
		return "parse_error"
	}
}

// wrapParseFailure 在保留原始错误文本的同时冻结 ParseSource 失败原因。
func wrapParseFailure(err error) error {
	return newSourceFailure(parseFailureReason(err), err.Error(), err)
}

// failureReason 优先读取 sourceFailure 的显式 reason，再按 sentinel 兜底。
func failureReason(err error) string {
	var sf *sourceFailure
	if errors.As(err, &sf) {
		return sf.reason
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "network_error"
	}
	return parseFailureReason(err)
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
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE rule_pools SET sync_status='running', sync_error='', updated_at=CURRENT_TIMESTAMP WHERE id=?`, poolID)
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
	s.mu.Lock()
	cancel, ok := s.cancels[taskID]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("%w: 任务无法取消（可能已结束或服务重启）", ErrBadRequest)
	}
	const cancelMessage = "同步任务已取消"
	// 先中断网络读取/解析后的写入，再争用 SQLite 写锁落终态；避免等待中的写事务继续提交。
	cancel(errSyncCancelled)
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var status, taskError string
		if err := tx.QueryRowContext(ctx,
			`SELECT status, error FROM pool_sync_tasks WHERE id=? AND pool_id=?`, taskID, poolID).Scan(&status, &taskError); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		} else if status == "failed" && taskError == cancelMessage {
			return nil
		} else if status != "running" {
			return fmt.Errorf("%w: 任务不在运行中", ErrBadRequest)
		}
		res, err := tx.ExecContext(ctx,
			`UPDATE pool_sync_tasks SET status='failed', error=?, finished_at=CURRENT_TIMESTAMP
			 WHERE id=? AND pool_id=? AND status='running'`, cancelMessage, taskID, poolID)
		if err != nil {
			return err
		}
		changed, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("%w: 任务不在运行中", ErrBadRequest)
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE rule_pools SET sync_status='failed', sync_error=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, cancelMessage, poolID)
		return err
	})
	if err != nil {
		return err
	}
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

// CleanupOldTasks 全局清理超过保留期的终态同步历史、未被引用 failed 快照并回收孤儿 Canonical。
func (s *Service) CleanupOldTasks(ctx context.Context) (int64, error) {
	// 启动/清理入口同时执行存量同步输出清洗（幂等、非破坏性）。
	if err := s.SanitizeStoredSyncOutputs(ctx); err != nil {
		s.log.Error("清洗存量同步输出失败", "err", err)
	}
	var cleaned int64
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`DELETE FROM pool_sync_tasks WHERE finished_at IS NOT NULL
			 AND finished_at < datetime('now', ?)`, fmt.Sprintf("-%d days", taskRetentionDays))
		if err != nil {
			return err
		}
		cleaned, err = res.RowsAffected()
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM pool_source_snapshots WHERE status='failed'
			 AND created_at < datetime('now', ?)
			 AND id NOT IN (
			   SELECT COALESCE(active_snapshot_id,0) FROM rule_pool_sources
			   UNION
			   SELECT COALESCE(pending_snapshot_id,0) FROM rule_pool_sources
			 )`, fmt.Sprintf("-%d days", taskRetentionDays)); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx,
			`DELETE FROM pool_canonical_rules WHERE id NOT IN (
			   SELECT DISTINCT canonical_rule_id FROM pool_rule_origins)`)
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("清理过期同步历史失败: %w", err)
	}
	return cleaned, nil
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
		if s.finishInterrupted(ctx, bgStore, poolID, taskID, results) {
			return
		}
		r := s.syncOne(ctx, bgStore, client, poolID, src.id, src.url, SourceMode(src.mode))
		results = append(results, r)
		if s.finishInterrupted(ctx, bgStore, poolID, taskID, results) {
			return
		}
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
		return s.failSourceWithError(ctx, bgStore, poolID, sourceID, r, newSourceFailure("request_invalid", err.Error(), err))
	}
	resp, err := client.Do(req)
	if err != nil {
		if cause := context.Cause(ctx); cause != nil {
			r.Error = SanitizeTaskError(cause.Error())
			return r
		}
		return s.failSourceWithError(ctx, bgStore, poolID, sourceID, r, newSourceFailure("network_error", err.Error(), err))
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := fmt.Sprintf("HTTP %d", resp.StatusCode)
		return s.failSourceWithError(ctx, bgStore, poolID, sourceID, r, newSourceFailure("http_status_error", msg, nil))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxURLContentSize+1))
	if err != nil {
		if cause := context.Cause(ctx); cause != nil {
			r.Error = SanitizeTaskError(cause.Error())
			return r
		}
		return s.failSourceWithError(ctx, bgStore, poolID, sourceID, r, newSourceFailure("body_read_error", err.Error(), err))
	}
	if len(body) > maxURLContentSize {
		return s.failSourceWithError(ctx, bgStore, poolID, sourceID, r, newSourceFailure("body_too_large", "内容超过 50MB", nil))
	}
	parsed, err := ParseSource(body, mode)
	if err != nil {
		if cause := context.Cause(ctx); cause != nil {
			r.Error = SanitizeTaskError(cause.Error())
			return r
		}
		return s.failSourceWithError(ctx, bgStore, poolID, sourceID, r, wrapParseFailure(err))
	}
	if cause := context.Cause(ctx); cause != nil {
		r.Error = SanitizeTaskError(cause.Error())
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
		_, pending, err := applyParseResultTxWithMarshal(ctx, tx, poolID, sourceID, parsed, s.marshalJSON)
		if err != nil {
			return err
		}
		r.Pending = pending
		return nil
	})
	if err != nil {
		r.OK = false
		r.Error = SanitizeTaskError(err.Error())
	}
	return r
}

// failSource 将可归属到 URL 来源的失败写入 failed snapshot。
// 写入失败时不得伪称成功，单 URL 错误附带“失败快照写入失败”。
// failSource 保留旧签名，供包内兼容调用；主路径使用 failSourceWithError。
func (s *Service) failSource(ctx context.Context, bgStore *store.Store, poolID, sourceID int64, r PerURLResult, errMsg string) PerURLResult {
	return s.failSourceWithError(ctx, bgStore, poolID, sourceID, r,
		newSourceFailure(failedReasonCode(errMsg), errMsg, nil))
}

// failSourceWithError 将可归属到 URL 来源的失败写入 failed snapshot。
// 写入失败时不得伪称成功，单 URL 错误附带“失败快照写入失败”。
func (s *Service) failSourceWithError(ctx context.Context, bgStore *store.Store, poolID, sourceID int64, r PerURLResult, failure error) PerURLResult {
	if cause := context.Cause(ctx); cause != nil {
		r.Error = SanitizeTaskError(cause.Error())
		return r
	}
	errMsg := failure.Error()
	r.Error = SanitizeTaskError(errMsg)
	reason := failureReason(failure)
	if err := bgStore.TxImmediate(ctx, func(tx *sql.Tx) error {
		return recordFailedSnapshotTxWithReasonAndMarshal(ctx, tx, poolID, sourceID, errMsg, reason, s.marshalJSON)
	}); err != nil {
		s.log.Error("写入失败快照失败", "pool_id", poolID, "source_id", sourceID, "err", err)
		r.Error = taskErrorWithFailureSnapshotSuffix(errMsg)
	}
	return r
}

// taskErrorWithFailureSnapshotSuffix 预留后缀长度，保证 200 rune 上限内仍保留失败快照写入提示。
func taskErrorWithFailureSnapshotSuffix(base string) string {
	const suffix = "；失败快照写入失败"
	maxBase := redact.MaxFieldRunes - utf8.RuneCountInString(suffix)
	if maxBase < 0 {
		maxBase = 0
	}
	return redact.TruncateText(redact.RedactText(base), maxBase) + suffix
}

func applyParseResultTx(ctx context.Context, tx *sql.Tx, poolID, sourceID int64, parsed *ParseResult) (int64, bool, error) {
	return applyParseResultTxWithMarshal(ctx, tx, poolID, sourceID, parsed, json.Marshal)
}

// applyParseResultTxWithMarshal 与 applyParseResultTx 相同，但允许注入 JSON 序列化函数以覆盖失败路径。
func applyParseResultTxWithMarshal(ctx context.Context, tx *sql.Tx, poolID, sourceID int64, parsed *ParseResult, marshal func(any) ([]byte, error)) (int64, bool, error) {
	// 读取旧 active 信息，用于异常保护。
	var oldFormat, oldProfile sql.NullString
	var oldAccepted sql.NullInt64
	var oldActiveID sql.NullInt64
	var sourceMode string
	if err := tx.QueryRowContext(ctx,
		`SELECT src.source_mode, s.id, s.format, s.profile, s.accepted_count
		 FROM rule_pool_sources src LEFT JOIN pool_source_snapshots s ON s.id = src.active_snapshot_id
		 WHERE src.id=?`, sourceID).Scan(&sourceMode, &oldActiveID, &oldFormat, &oldProfile, &oldAccepted); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return 0, false, err
		}
	}

	formatChanged := false
	profileChanged := false
	dropTriggered := false
	pending := false
	if oldActiveID.Valid {
		formatChanged = oldFormat.String != string(parsed.Format)
		profileChanged = oldProfile.String != parsed.Profile
		if formatChanged || profileChanged {
			pending = true
		}
		if oldAccepted.Int64 >= 20 && int64(parsed.Accepted)*10 < oldAccepted.Int64*7 {
			dropTriggered = true
			pending = true
		}
	}
	status := "active"
	if pending {
		status = "pending"
	}
	reasons := make([]string, 0, 3)
	if !oldActiveID.Valid {
		reasons = append(reasons, "first_success")
	} else if !pending {
		reasons = append(reasons, "normal")
	}
	if formatChanged {
		reasons = append(reasons, "format_changed")
	}
	if profileChanged {
		reasons = append(reasons, "profile_changed")
	}
	if dropTriggered {
		reasons = append(reasons, "accepted_below_threshold")
	}

	ruleCounts := parsed.RuleCounts
	if ruleCounts == nil {
		ruleCounts = []RuleCountStat{}
	}
	threshold := 90
	if parsed.Input < 10 {
		threshold = 100
	}
	detection := &DetectionStats{
		EvidenceCodes:              parsed.EvidenceCodes,
		RecognitionRequiredPercent: &threshold,
	}
	if detection.EvidenceCodes == nil {
		detection.EvidenceCodes = []string{}
	}
	comparison := &ComparisonStats{AcceptedDropThresholdPercent: 70}
	if oldActiveID.Valid {
		comparison.PreviousActive = &PreviousActiveSnapshot{
			SnapshotID: oldActiveID.Int64,
			Format:     oldFormat.String,
			Profile:    oldProfile.String,
			Accepted:   oldAccepted.Int64,
		}
		comparison.FormatChanged = formatChanged
		comparison.ProfileChanged = profileChanged
		comparison.AcceptedDropTriggered = dropTriggered
	}
	stats := SnapshotStats{
		SchemaVersion:        1,
		SourceMode:           sourceMode,
		Detection:            detection,
		RuleCounts:           ruleCounts,
		UnclassifiedRejected: parsed.UnclassifiedRejected,
		Comparison:           comparison,
		Decision:             &DecisionStats{InitialStatus: status, ReasonCodes: reasons},
	}

	diagJSON, err := marshal(NormalizeDiagnostics(parsed.Diagnostics))
	if err != nil {
		return 0, false, fmt.Errorf("序列化来源快照诊断失败: %w", err)
	}
	statsJSON, err := marshal(stats)
	if err != nil {
		return 0, false, fmt.Errorf("序列化来源快照统计失败: %w", err)
	}
	res, err := tx.ExecContext(ctx,
		`INSERT INTO pool_source_snapshots
		   (source_id, format, profile, status, input_count, recognized_count, accepted_count, excluded_count, rejected_count, duplicate_count, diagnostic_json, stats_json)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		sourceID, string(parsed.Format), parsed.Profile, status, parsed.Input, parsed.Recognized,
		parsed.Accepted, parsed.Excluded, parsed.Rejected, parsed.Duplicates, string(diagJSON), string(statsJSON))
	if err != nil {
		return 0, false, err
	}
	snapshotID, err := res.LastInsertId()
	if err != nil {
		return 0, false, err
	}

	for _, parsedRule := range parsed.Items {
		canonicalID, err := ensureCanonicalTx(ctx, tx, poolID, parsedRule.Rule)
		if err != nil {
			return 0, false, err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO pool_rule_origins (pool_id, canonical_rule_id, source_id, snapshot_id, sort_order, raw_line, line_no)
			 VALUES (?,?,?,?,?,?,?)`,
			poolID, canonicalID, sourceID, snapshotID,
			int64(parsedRule.Origin.Order), parsedRule.Origin.Raw, parsedRule.Origin.Line); err != nil {
			return 0, false, err
		}
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

// recordFailedSnapshotTx 写入 failed snapshot，不修改 active/pending 指针。
// recordFailedSnapshotTx 保留旧签名，供包内兼容调用；主路径使用带 reason 的版本。
func recordFailedSnapshotTx(ctx context.Context, tx *sql.Tx, poolID, sourceID int64, errMsg string) error {
	return recordFailedSnapshotTxWithReason(ctx, tx, poolID, sourceID, errMsg, failedReasonCode(errMsg))
}

// recordFailedSnapshotTxWithReason 写入 failed snapshot，不修改 active/pending 指针。
func recordFailedSnapshotTxWithReason(ctx context.Context, tx *sql.Tx, poolID, sourceID int64, errMsg, reasonCode string) error {
	return recordFailedSnapshotTxWithReasonAndMarshal(ctx, tx, poolID, sourceID, errMsg, reasonCode, json.Marshal)
}

// recordFailedSnapshotTxWithReasonAndMarshal 允许注入 JSON 序列化函数，覆盖序列化失败路径。
func recordFailedSnapshotTxWithReasonAndMarshal(ctx context.Context, tx *sql.Tx, poolID, sourceID int64, errMsg, reasonCode string, marshal func(any) ([]byte, error)) error {
	var mode string
	var oldID sql.NullInt64
	var oldFormat, oldProfile sql.NullString
	var oldAccepted sql.NullInt64
	err := tx.QueryRowContext(ctx,
		`SELECT src.source_mode, s.id, s.format, s.profile, s.accepted_count
		 FROM rule_pool_sources src LEFT JOIN pool_source_snapshots s ON s.id = src.active_snapshot_id
		 WHERE src.id=?`, sourceID).Scan(&mode, &oldID, &oldFormat, &oldProfile, &oldAccepted)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("读取失败快照旧 active 状态失败: %w", err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("读取失败快照旧 active 状态失败: 来源不存在: %w", err)
	}
	comparison := &ComparisonStats{AcceptedDropThresholdPercent: 70}
	if oldID.Valid {
		comparison.PreviousActive = &PreviousActiveSnapshot{
			SnapshotID: oldID.Int64,
			Format:     oldFormat.String,
			Profile:    oldProfile.String,
			Accepted:   oldAccepted.Int64,
		}
	}
	stats := SnapshotStats{
		SchemaVersion: 1,
		SourceMode:    mode,
		Detection:     &DetectionStats{EvidenceCodes: []string{}},
		RuleCounts:    []RuleCountStat{},
		Comparison:    comparison,
		Decision: &DecisionStats{
			InitialStatus: "failed",
			ReasonCodes:   []string{reasonCode},
		},
	}
	diag := []ParseDiagnostic{{Kind: "error", Message: SanitizeTaskError(errMsg)}}
	diagJSON, err := marshal(NormalizeDiagnostics(diag))
	if err != nil {
		return fmt.Errorf("序列化失败快照诊断失败: %w", err)
	}
	statsJSON, err := marshal(stats)
	if err != nil {
		return fmt.Errorf("序列化失败快照统计失败: %w", err)
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO pool_source_snapshots
		   (source_id, format, profile, status, input_count, recognized_count, accepted_count, excluded_count, rejected_count, duplicate_count, diagnostic_json, stats_json)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		sourceID, "", "", "failed", 0, 0, 0, 0, 0, 0, string(diagJSON), string(statsJSON))
	return err
}

// failedReasonCode 是旧字符串分类的兼容实现；新主路径使用 sourceFailure + sentinel。
func failedReasonCode(errMsg string) string {
	msg := strings.ToLower(errMsg)
	switch {
	case strings.Contains(msg, "conflicting"):
		return "conflicting_format"
	case strings.Contains(msg, "mixed platform"):
		return "mixed_platform"
	case strings.Contains(msg, "ambiguous"):
		return "ambiguous_format"
	case strings.Contains(msg, "http "), strings.Contains(msg, "http_status"):
		return "http_status_error"
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "context canceled"),
		strings.Contains(msg, "deadline exceeded"), strings.Contains(msg, "网络"):
		return "network_error"
	case strings.Contains(msg, "50mb"), strings.Contains(msg, "内容超过"):
		return "body_too_large"
	case strings.Contains(msg, "read"), strings.Contains(msg, "connection reset"),
		strings.Contains(msg, "unexpected eof"), strings.Contains(msg, "读取"):
		return "body_read_error"
	case strings.Contains(msg, "html"):
		return "html_source"
	case strings.Contains(msg, "unrecognized"):
		return "unrecognized_source"
	case strings.Contains(msg, "no accepted"):
		return "no_accepted_rules"
	case strings.Contains(msg, "threshold"):
		return "recognition_threshold_not_met"
	case strings.Contains(msg, "parse"), strings.Contains(msg, "解析"):
		return "parse_error"
	default:
		return "request_invalid"
	}
}

func (s *Service) finishInterrupted(ctx context.Context, bgStore *store.Store, poolID, taskID int64, results []PerURLResult) bool {
	cause := context.Cause(ctx)
	if cause == nil {
		return false
	}
	s.finishTask(ctx, bgStore, poolID, taskID, "failed", cause.Error(), results)
	return true
}

func (s *Service) finishTask(taskCtx context.Context, bgStore *store.Store, poolID, taskID int64, status, errMsg string, results []PerURLResult) {
	cleanResults := make([]PerURLResult, len(results))
	for i, r := range results {
		cleanResults[i] = SanitizePerURLResult(r)
	}
	perJSON, err := s.marshalJSON(cleanResults)
	if err != nil {
		s.log.Error("序列化同步任务结果失败", "pool_id", poolID, "task_id", taskID, "err", err)
		perJSON = []byte("[]")
	}
	errMsg = SanitizeTaskError(errMsg)
	writeCtx, cancel := context.WithTimeout(context.Background(), terminalWriteTime)
	defer cancel()
	err = bgStore.TxImmediate(writeCtx, func(tx *sql.Tx) error {
		if cause := context.Cause(taskCtx); cause != nil {
			status = "failed"
			errMsg = SanitizeTaskError(cause.Error())
		}
		res, err := tx.ExecContext(writeCtx,
			`UPDATE pool_sync_tasks SET status=?, per_url_json=?, error=?, finished_at=CURRENT_TIMESTAMP
			 WHERE id=? AND status='running'`, status, string(perJSON), errMsg, taskID)
		if err != nil {
			return err
		}
		changed, err := res.RowsAffected()
		if err != nil || changed == 0 {
			return err
		}
		if status == "succeeded" {
			_, err = tx.ExecContext(writeCtx,
				`UPDATE rule_pools SET last_synced_at=CURRENT_TIMESTAMP, sync_status='succeeded', sync_error='', updated_at=CURRENT_TIMESTAMP WHERE id=?`, poolID)
		} else {
			_, err = tx.ExecContext(writeCtx,
				`UPDATE rule_pools SET sync_status=?, sync_error=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, status, errMsg, poolID)
		}
		return err
	})
	if err != nil {
		s.log.Error("回写同步终态失败", "pool_id", poolID, "task_id", taskID, "err", err)
		return
	}
	// 清理 7 天前的终态任务，不删除 active/pending 快照；清理失败必须可见但不影响终态已提交。
	if _, err := bgStore.DB().ExecContext(writeCtx,
		`DELETE FROM pool_sync_tasks WHERE pool_id=? AND finished_at IS NOT NULL AND finished_at < datetime('now', ?)`, poolID, fmt.Sprintf("-%d days", taskRetentionDays)); err != nil {
		s.log.Error("清理终态同步任务失败", "pool_id", poolID, "task_id", taskID, "err", err)
	}
}

func (s *Service) failTask(taskCtx context.Context, poolID, taskID int64, results []PerURLResult, msg string) {
	cleanResults := make([]PerURLResult, len(results))
	for i, r := range results {
		cleanResults[i] = SanitizePerURLResult(r)
	}
	perJSON, err := s.marshalJSON(cleanResults)
	if err != nil {
		s.log.Error("序列化失败任务结果失败", "pool_id", poolID, "task_id", taskID, "err", err)
		perJSON = []byte("[]")
	}
	msg = SanitizeTaskError(msg)
	writeCtx, cancel := context.WithTimeout(context.Background(), terminalWriteTime)
	defer cancel()
	if err := s.store.TxImmediate(writeCtx, func(tx *sql.Tx) error {
		if cause := context.Cause(taskCtx); cause != nil {
			msg = SanitizeTaskError(cause.Error())
		}
		res, err := tx.ExecContext(writeCtx,
			`UPDATE pool_sync_tasks SET status='failed', per_url_json=?, error=?, finished_at=CURRENT_TIMESTAMP
			 WHERE id=? AND status='running'`, string(perJSON), msg, taskID)
		if err != nil {
			return err
		}
		changed, err := res.RowsAffected()
		if err != nil || changed == 0 {
			return err
		}
		_, err = tx.ExecContext(writeCtx,
			`UPDATE rule_pools SET sync_status='failed', sync_error=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, msg, poolID)
		return err
	}); err != nil {
		s.log.Error("回写失败任务", "pool_id", poolID, "task_id", taskID, "err", err)
	}
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
			`UPDATE pool_source_snapshots SET status='active', activated_at=CURRENT_TIMESTAMP WHERE id=?`, snapshotID); err != nil {
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
