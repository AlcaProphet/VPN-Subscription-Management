package pool

import (
	"context"
	"encoding/json"
	"fmt"

	"vpn-sub/internal/redact"
)

// NormalizeDiagnostics 在持久化/展示边界统一脱敏与限额。
// 空输入返回非 nil 空切片，保证 JSON 序列化为 [] 而不是 null。
func NormalizeDiagnostics(diags []ParseDiagnostic) []ParseDiagnostic {
	if len(diags) == 0 {
		return []ParseDiagnostic{}
	}
	out := make([]ParseDiagnostic, 0, len(diags))
	for _, d := range diags {
		d.Message = redact.TruncateText(redact.RedactText(d.Message), redact.MaxFieldRunes)
		d.Raw = redact.TruncateText(redact.RedactText(d.Raw), redact.MaxFieldRunes)
		out = append(out, d)
	}
	if len(out) > redact.MaxDiagnostics {
		extra := len(out) - (redact.MaxDiagnostics - 1)
		out = append(out[:redact.MaxDiagnostics-1], ParseDiagnostic{
			Line:    0,
			Kind:    "truncated",
			Message: fmt.Sprintf("另有 %d 条诊断未展示", extra),
			Raw:     "",
		})
	}
	return out
}

// SanitizePerURLResult 清洗单 URL 回执：URL 使用展示用脱敏 URL，Error 使用脱敏+限长文本。
func SanitizePerURLResult(r PerURLResult) PerURLResult {
	r.URL = redact.RedactDisplayURL(r.URL)
	r.Error = redact.TruncateText(redact.RedactText(r.Error), redact.MaxFieldRunes)
	return r
}

// SanitizeTaskError 清洗任务/池级错误文本。
func SanitizeTaskError(err string) string {
	return redact.TruncateText(redact.RedactText(err), redact.MaxFieldRunes)
}

// SanitizeSyncTask 返回清洗后的任务副本；不修改底层 task。
func SanitizeSyncTask(t *SyncTask) *SyncTask {
	if t == nil {
		return nil
	}
	out := *t
	out.PerURL = make([]PerURLResult, len(t.PerURL))
	for i, r := range t.PerURL {
		out.PerURL[i] = SanitizePerURLResult(r)
	}
	out.Error = SanitizeTaskError(t.Error)
	return &out
}

// SanitizePoolSyncError 仅清洗池的 sync_error；sources[].url 与 urls[] 保持原始值以支持编辑。
func SanitizePoolSyncError(p *Pool) {
	if p == nil {
		return
	}
	p.SyncError = SanitizeTaskError(p.SyncError)
}

// SanitizeStoredSyncOutputs 以幂等、非破坏性方式清洗存量同步任务和池错误。
// 不删除记录，不修改 rule_pool_sources.url，不触碰快照统计/诊断证据。
func (s *Service) SanitizeStoredSyncOutputs(ctx context.Context) error {
	rows, err := s.store.DB().QueryContext(ctx, `SELECT id, per_url_json, error FROM pool_sync_tasks`)
	if err != nil {
		return err
	}
	type taskRow struct {
		id  int64
		per string
		err string
	}
	var tasks []taskRow
	for rows.Next() {
		var tr taskRow
		if err := rows.Scan(&tr.id, &tr.per, &tr.err); err != nil {
			_ = rows.Close()
			return err
		}
		tasks = append(tasks, tr)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, tr := range tasks {
		var results []PerURLResult
		if err := json.Unmarshal([]byte(tr.per), &results); err == nil {
			cleaned := make([]PerURLResult, len(results))
			for i, r := range results {
				cleaned[i] = SanitizePerURLResult(r)
			}
			raw, _ := json.Marshal(cleaned)
			tr.per = string(raw)
		} else {
			tr.per = redact.RedactText(tr.per)
		}
		tr.err = SanitizeTaskError(tr.err)
		if _, err := s.store.DB().ExecContext(ctx,
			`UPDATE pool_sync_tasks SET per_url_json=?, error=? WHERE id=?`, tr.per, tr.err, tr.id); err != nil {
			return err
		}
	}
	poolRows, err := s.store.DB().QueryContext(ctx, `SELECT id, sync_error FROM rule_pools`)
	if err != nil {
		return err
	}
	type poolRow struct {
		id  int64
		err string
	}
	var pools []poolRow
	for poolRows.Next() {
		var pr poolRow
		if err := poolRows.Scan(&pr.id, &pr.err); err != nil {
			_ = poolRows.Close()
			return err
		}
		pools = append(pools, pr)
	}
	_ = poolRows.Close()
	if err := poolRows.Err(); err != nil {
		return err
	}
	for _, pr := range pools {
		cleaned := SanitizeTaskError(pr.err)
		if _, err := s.store.DB().ExecContext(ctx,
			`UPDATE rule_pools SET sync_error=? WHERE id=?`, cleaned, pr.id); err != nil {
			return err
		}
	}
	return nil
}
