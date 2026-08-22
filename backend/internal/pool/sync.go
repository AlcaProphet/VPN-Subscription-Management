// sync.go：URL 同步任务（异步执行 + 持久化 + 差量更新 + 来源隔离，Design2 §2.4）
package pool

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"vpn-sub/internal/store"
)

// 关键参数（Design2 §2.4）
const (
	urlTimeout        = 60 * time.Second // 单 URL 拉取超时 60s
	maxURLContentSize = 50 << 20         // 单 URL 内容上限 50MB
	taskRetentionDays = 7
)

// PerURLResult 单 URL 同步结果（UI §5.2.3 回执）
type PerURLResult struct {
	URL         string   `json:"url"`
	OK          bool     `json:"ok"`
	Added       int      `json:"added"`
	Removed     int      `json:"removed"`
	Skipped     int      `json:"skipped"`
	SkipReasons []string `json:"skip_reasons,omitempty"`
	Error       string   `json:"error"`

	entries []parsedEntry // 内存态解析结果，不序列化
}

// SyncTask 同步任务（状态端点/历史列表共用形状）
type SyncTask struct {
	ID         int64          `json:"task_id"`
	PoolID     int64          `json:"pool_id"`
	Status     string         `json:"status"` // running / succeeded / failed / partial
	PerURL     []PerURLResult `json:"per_url"`
	Error      string         `json:"error"`
	StartedAt  *time.Time     `json:"started_at"`
	FinishedAt *time.Time     `json:"finished_at"`
}

type parsedEntry struct {
	RuleType   string
	MatchValue string
}

// SubmitSync 提交同步任务：池存在性 + 无 running 检查 + 读取 URL 快照 + 插入任务在同一 BEGIN IMMEDIATE 事务内完成；
// 事务提交后启动 goroutine 执行，返回任务 ID。URL 快照在提交时固化，编辑池 URL 不影响已启动任务。
func (s *Service) SubmitSync(ctx context.Context, poolID int64) (int64, error) {
	var taskID int64
	var urls []string
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if err := s.checkPoolTx(ctx, tx, poolID); err != nil {
			return err
		}
		var running int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM pool_sync_tasks WHERE pool_id = ? AND status = 'running'`, poolID).Scan(&running); err != nil {
			return err
		}
		if running > 0 {
			return ErrSyncRunning
		}
		var urlsRaw string
		if err := tx.QueryRowContext(ctx,
			`SELECT urls_json FROM rule_pools WHERE id = ?`, poolID).Scan(&urlsRaw); err != nil {
			return err
		}
		urls = parseStringSlice(urlsRaw)
		res, err := tx.ExecContext(ctx,
			`INSERT INTO pool_sync_tasks (pool_id, status, per_url_json, started_at) VALUES (?, 'running', '[]', CURRENT_TIMESTAMP)`,
			poolID)
		if err != nil {
			return fmt.Errorf("创建同步任务失败: %w", err)
		}
		taskID, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return 0, err
	}
	// 后台任务使用可取消 context + 整体超时，避免无限挂起；终态回写使用脱离该 context 的连接，确保取消后仍能落 failed。
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
		s.runSyncTask(ctx, poolID, taskID, urls) // 后台执行，不阻塞请求；只使用提交时快照
	}()
	return taskID, nil
}

// CancelSync 取消指定素材池的运行中同步任务。取消后任务终态复用 failed，error 写“同步任务已取消”。
func (s *Service) CancelSync(ctx context.Context, poolID, taskID int64) error {
	var status string
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT status FROM pool_sync_tasks WHERE id = ? AND pool_id = ?`, taskID, poolID).Scan(&status)
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

// GetStatus 读取最近一次任务状态（任务持久化于 pool_sync_tasks）。
// 池不存在返回 ErrNotFound；池存在但无任务返回 (nil,nil)，供接入层输出空状态。
func (s *Service) GetStatus(ctx context.Context, poolID int64) (*SyncTask, error) {
	var exists int
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM rule_pools WHERE id = ?`, poolID).Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, ErrNotFound
	}
	row := s.store.DB().QueryRowContext(ctx,
		`SELECT id, pool_id, status, per_url_json, error, started_at, finished_at
		 FROM pool_sync_tasks WHERE pool_id = ? ORDER BY id DESC LIMIT 1`, poolID)
	t, err := scanSyncTask(row)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	return t, err
}

// ListTasks 历史任务分页（id DESC，保留 7 天）；池不存在返回 ErrNotFound
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
		`SELECT COUNT(*) FROM pool_sync_tasks WHERE pool_id = ?`, poolID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT id, pool_id, status, per_url_json, error, started_at, finished_at
		 FROM pool_sync_tasks WHERE pool_id = ? ORDER BY id DESC LIMIT ? OFFSET ?`,
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

// runSyncTask 后台执行：串行拉取提交时 URL 快照 → 成功项 upsert → 全部成功才差量删除；
// 池已删除时仅记日志不崩溃；编辑 URL 只影响下一次同步。
func (s *Service) runSyncTask(ctx context.Context, poolID, taskID int64, urls []string) {
	if len(urls) == 0 {
		s.failTask(context.WithoutCancel(ctx), poolID, taskID, nil, "未配置 URL")
		return
	}
	client := &http.Client{Timeout: urlTimeout}
	results := make([]PerURLResult, 0, len(urls))
	partial := false
	for _, target := range urls {
		r := s.syncURL(ctx, client, target)
		if !r.OK {
			partial = true
		}
		results = append(results, r)
	}

	status := "succeeded"
	if partial {
		status = "partial"
	}
	finalErr := summarizePartial(results)
	// 后台同步使用独立数据库连接，避免长事务占用 API 共用连接。
	bgStore, err := store.Open(filepath.Dir(s.store.DBPath()), filepath.Base(s.store.DBPath()))
	if err != nil {
		s.log.Error("素材池同步打开后台数据库连接失败", "pool_id", poolID, "task_id", taskID, "err", err)
		s.failTask(context.WithoutCancel(ctx), poolID, taskID, results, "打开后台数据库连接失败: "+err.Error())
		return
	}
	defer bgStore.Close()
	err = bgStore.TxImmediate(ctx, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM rule_pools WHERE id = ?`, poolID).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return nil // 池已删除：任务行已随外键级联，终态写回自然跳过
		}
		removedByURL := map[string]int{}
		// 成功 URL 条目照常插入（manual 冲突行自动忽略，manual 段永不受同步改写；
		// 使用 INSERT OR IGNORE 使 RowsAffected 仅反映真正新增行，用于 added 统计）
		// 一次性取得 URL 段当前最大排序值，后续在内存中递增，避免逐条 SELECT MAX。
		var nextOrder int64
		if err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(sort_order), ?) FROM pool_entries WHERE pool_id = ? AND sort_order >= ?`,
			URLBase-1, poolID, URLBase).Scan(&nextOrder); err != nil {
			return err
		}
		nextOrder++
		const batchSize = 500
		for i := range results {
			r := &results[i]
			if !r.OK {
				continue
			}
			for start := 0; start < len(r.entries); start += batchSize {
				end := start + batchSize
				if end > len(r.entries) {
					end = len(r.entries)
				}
				var sb strings.Builder
				sb.WriteString(`INSERT OR IGNORE INTO pool_entries (pool_id, rule_type, match_value, source, sort_order, source_url) VALUES `)
				args := make([]any, 0, (end-start)*5)
				for j := start; j < end; j++ {
					if j > start {
						sb.WriteString(",")
					}
					sb.WriteString(`(?,?,?,'url',?,?)`)
					args = append(args, poolID, r.entries[j].RuleType, r.entries[j].MatchValue, nextOrder, r.URL)
					nextOrder++
				}
				res, err := tx.ExecContext(ctx, sb.String(), args...)
				if err != nil {
					return err
				}
				if n, _ := res.RowsAffected(); n > 0 {
					r.Added += int(n)
				}
			}
		}
		// 差量删除：仅全部 URL 成功时执行（部分失败不删，防不完整结果误删）
		if !partial {
			keepTable := fmt.Sprintf("_pool_sync_keep_%d", taskID)
			if _, err := tx.ExecContext(ctx,
				`CREATE TEMP TABLE `+keepTable+` (rule_type TEXT NOT NULL, match_value TEXT NOT NULL)`); err != nil {
				return err
			}
			allEntries := make([]parsedEntry, 0)
			for _, r := range results {
				if !r.OK {
					continue
				}
				allEntries = append(allEntries, r.entries...)
			}
			for start := 0; start < len(allEntries); start += batchSize {
				end := start + batchSize
				if end > len(allEntries) {
					end = len(allEntries)
				}
				var sb strings.Builder
				sb.WriteString(`INSERT OR IGNORE INTO ` + keepTable + ` (rule_type, match_value) VALUES `)
				args := make([]any, 0, (end-start)*2)
				for j := start; j < end; j++ {
					if j > start {
						sb.WriteString(",")
					}
					sb.WriteString(`(?,?)`)
					args = append(args, allEntries[j].RuleType, allEntries[j].MatchValue)
				}
				if _, err := tx.ExecContext(ctx, sb.String(), args...); err != nil {
					return err
				}
			}
			// 删除前按 source_url 统计将被删除的行数，用于 per-URL removed 精确回执
			delRows, err := tx.QueryContext(ctx,
				`SELECT source_url, COUNT(*) FROM pool_entries
				 WHERE pool_id = ? AND source = 'url'
				   AND NOT EXISTS (
				     SELECT 1 FROM `+keepTable+` k
				     WHERE k.rule_type = pool_entries.rule_type AND k.match_value = pool_entries.match_value
				   )
				 GROUP BY source_url`, poolID)
			if err != nil {
				return err
			}
			for delRows.Next() {
				var u string
				var c int
				if err := delRows.Scan(&u, &c); err != nil {
					_ = delRows.Close()
					return err
				}
				removedByURL[u] = c
			}
			if err := delRows.Close(); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx,
				`DELETE FROM pool_entries
				 WHERE pool_id = ? AND source = 'url'
				   AND NOT EXISTS (
				     SELECT 1 FROM `+keepTable+` k
				     WHERE k.rule_type = pool_entries.rule_type AND k.match_value = pool_entries.match_value
				   )`, poolID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DROP TABLE `+keepTable); err != nil {
				return err
			}
		}
		// 回写 removed（历史空 source_url 无法归属，不写入任何 URL）
		for i := range results {
			if n, ok := removedByURL[results[i].URL]; ok {
				results[i].Removed = n
			}
		}
		// 终态写回 + 池快照 + 顺手清理 7 天前历史任务
		if _, err := tx.ExecContext(ctx,
			`UPDATE pool_sync_tasks SET status = ?, per_url_json = ?, error = ?, finished_at = CURRENT_TIMESTAMP WHERE id = ?`,
			status, toJSON(stripEntries(results)), finalErr, taskID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE rule_pools SET last_synced_at = CURRENT_TIMESTAMP, sync_status = ?, sync_error = ? WHERE id = ?`,
			status, finalErr, poolID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM pool_sync_tasks WHERE pool_id = ? AND finished_at IS NOT NULL AND finished_at < datetime('now', '-7 days')`,
			poolID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		s.log.Error("素材池同步任务失败", "pool_id", poolID, "task_id", taskID, "err", err)
		msg := err.Error()
		if errors.Is(context.Cause(ctx), errSyncTimeout) {
			msg = errSyncTimeout.Error()
		} else if errors.Is(context.Cause(ctx), errSyncCancelled) {
			msg = errSyncCancelled.Error()
		}
		s.failTask(context.WithoutCancel(ctx), poolID, taskID, results, msg)
	}
}

// failTask 将任务与池快照置为 failed；池已删除时静默跳过。
func (s *Service) failTask(ctx context.Context, poolID, taskID int64, results []PerURLResult, msg string) {
	if err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var exists int
		if e := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM rule_pools WHERE id = ?`, poolID).Scan(&exists); e != nil {
			return e
		}
		if exists == 0 {
			return nil
		}
		if _, e := tx.ExecContext(ctx,
			`UPDATE pool_sync_tasks SET status = 'failed', per_url_json = ?, error = ?, finished_at = CURRENT_TIMESTAMP WHERE id = ?`,
			toJSON(stripEntries(results)), truncateError(msg, 200), taskID); e != nil {
			return e
		}
		_, e := tx.ExecContext(ctx,
			`UPDATE rule_pools SET last_synced_at = CURRENT_TIMESTAMP, sync_status = 'failed', sync_error = ? WHERE id = ?`,
			truncateError(msg, 200), poolID)
		return e
	}); err != nil {
		s.log.Error("素材池任务终态写回失败", "pool_id", poolID, "task_id", taskID, "err", err)
	}
}

// syncURL 拉取并解析单个 URL；空响应/零有效条目视为失败（保留旧数据）
func (s *Service) syncURL(ctx context.Context, client *http.Client, target string) (r PerURLResult) {
	r.URL = target
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	resp, err := client.Do(req)
	if err != nil {
		r.Error = "拉取失败: " + err.Error()
		return r
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		r.Error = fmt.Sprintf("HTTP 状态码 %d", resp.StatusCode)
		return r
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxURLContentSize+1))
	if err != nil {
		r.Error = "读取响应失败: " + err.Error()
		return r
	}
	if int64(len(body)) > maxURLContentSize {
		r.Error = "内容超过 50MB 限制"
		return r
	}
	if len(bytes.TrimSpace(body)) == 0 {
		r.Error = "空响应，已保留旧数据"
		return r
	}
	entries, skipped, reasons, err := parseURLBody(body)
	if err != nil {
		r.Error = "解析响应失败: " + err.Error()
		r.Skipped = skipped
		r.SkipReasons = reasons
		return r
	}
	if len(entries) == 0 {
		r.Error = "响应无有效规则条目，已保留旧数据"
		r.Skipped = skipped
		r.SkipReasons = reasons
		return r
	}
	r.entries = entries
	r.Skipped = skipped
	r.SkipReasons = reasons
	r.OK = true
	return r
}

// parseURLBody 逐行解析并按（类型,值）去重合并；返回有效条目、跳过计数、跳过原因与 Scanner 错误。
// Scanner 错误（如超长行）必须显式返回，调用方将 URL 记为失败并保留旧数据，避免差量删除误删。
func parseURLBody(body []byte) ([]parsedEntry, int, []string, error) {
	sc := bufio.NewScanner(bytes.NewReader(body))
	sc.Buffer(make([]byte, 64*1024), 1024*1024) // 支持长行
	seen := map[string]bool{}
	out := make([]parsedEntry, 0)
	skipped := 0
	var reasons []string
	for sc.Scan() {
		typ, val, reason, ok := ParseLine(sc.Text())
		if !ok {
			skipped++
			if reason != "" {
				reasons = append(reasons, reason)
			}
			continue
		}
		normType, norm, err := ValidateEntry(typ, val)
		if err != nil {
			skipped++ // 非法条目跳过并计数，不阻断同步
			reasons = append(reasons, err.Error())
			continue
		}
		key := normType + "\x00" + norm
		if seen[key] {
			skipped++
			reasons = append(reasons, "重复条目已忽略: "+normType+","+norm)
			continue
		}
		seen[key] = true
		out = append(out, parsedEntry{RuleType: normType, MatchValue: norm})
	}
	if err := sc.Err(); err != nil {
		return out, skipped, reasons, err
	}
	return out, skipped, reasons, nil
}

// summarizePartial 汇总部分失败原因（pool 快照 sync_error）
func summarizePartial(results []PerURLResult) string {
	if len(results) == 0 {
		return ""
	}
	var parts []string
	for _, r := range results {
		if !r.OK {
			parts = append(parts, r.URL+": "+r.Error)
		}
	}
	return strings.Join(parts, "；")
}

// stripEntries 移除 PerURLResult 中的内存 entries 字段（避免 JSON 输出额外字段）
func stripEntries(in []PerURLResult) []PerURLResult {
	out := make([]PerURLResult, len(in))
	copy(out, in)
	for i := range out {
		out[i].entries = nil
	}
	return out
}

func truncateError(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
