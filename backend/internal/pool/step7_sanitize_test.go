package pool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"vpn-sub/internal/redact"
)

func TestSanitizeStoredSyncOutputsIdempotentAndNonDestructive(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	rawURL := "https://example.com/rules?token=RAW_SECRET"
	p, err := svc.Create(ctx, "存量清洗池", []SourceInput{{URL: rawURL, SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建素材池失败: %v", err)
	}
	sourceID := p.Sources[0].ID

	rawPer := `[{"url":"https://example.com/rules?token=SECRET","source_id":1,"ok":false,"accepted":0,"error":"download token=SECRET"}]`
	longTaskErr := "任务 token=SECRET " + strings.Repeat("错", 250)
	res, err := st.DB().ExecContext(ctx,
		`INSERT INTO pool_sync_tasks (pool_id,status,per_url_json,error,started_at,finished_at)
		 VALUES (?, 'failed', ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, p.ID, rawPer, longTaskErr)
	if err != nil {
		t.Fatalf("插入同步任务失败: %v", err)
	}
	taskID, _ := res.LastInsertId()
	if _, err := st.DB().ExecContext(ctx,
		`UPDATE rule_pools SET sync_error=? WHERE id=?`, "pool token=SECRET", p.ID); err != nil {
		t.Fatalf("写入池错误失败: %v", err)
	}

	snapshotDiag := `[{"line":0,"kind":"error","message":"token=SECRET","raw":"raw=SECRET"}]`
	snapshotStats := `{"schema_version":1,"raw":"token=SECRET"}`
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO pool_source_snapshots
		   (source_id, status, input_count, recognized_count, accepted_count, excluded_count, rejected_count, duplicate_count, diagnostic_json, stats_json)
		 VALUES (?, 'failed', 0, 0, 0, 0, 0, 0, ?, ?)`,
		sourceID, snapshotDiag, snapshotStats); err != nil {
		t.Fatalf("插入快照失败: %v", err)
	}

	if err := svc.SanitizeStoredSyncOutputs(ctx); err != nil {
		t.Fatalf("存量清洗失败: %v", err)
	}

	var perRaw, taskErr string
	if err := st.DB().QueryRowContext(ctx,
		`SELECT per_url_json, error FROM pool_sync_tasks WHERE id=?`, taskID).Scan(&perRaw, &taskErr); err != nil {
		t.Fatalf("读取清洗后任务失败: %v", err)
	}
	if strings.Contains(perRaw, "SECRET") || strings.Contains(taskErr, "SECRET") {
		t.Fatalf("任务仍含敏感值: per=%s err=%s", perRaw, taskErr)
	}
	if utf8.RuneCountInString(taskErr) > redact.MaxFieldRunes {
		t.Fatalf("任务错误未限额: %d", utf8.RuneCountInString(taskErr))
	}
	var results []PerURLResult
	if err := json.Unmarshal([]byte(perRaw), &results); err != nil || len(results) != 1 {
		t.Fatalf("清洗后 per_url_json 不是合法单元素数组: %s err=%v", perRaw, err)
	}
	if results[0].URL != "https://example.com/rules?token=***" || strings.Contains(results[0].Error, "SECRET") {
		t.Fatalf("单 URL 回执未清洗: %+v", results[0])
	}

	var poolErr string
	if err := st.DB().QueryRowContext(ctx, `SELECT sync_error FROM rule_pools WHERE id=?`, p.ID).Scan(&poolErr); err != nil {
		t.Fatalf("读取池错误失败: %v", err)
	}
	if strings.Contains(poolErr, "SECRET") {
		t.Fatalf("池 sync_error 仍含敏感值: %s", poolErr)
	}

	var gotURL string
	if err := st.DB().QueryRowContext(ctx, `SELECT COALESCE(url,'') FROM rule_pool_sources WHERE id=?`, sourceID).Scan(&gotURL); err != nil {
		t.Fatalf("读取来源 URL 失败: %v", err)
	}
	if gotURL != rawURL {
		t.Fatalf("清洗不得改写 rule_pool_sources.url: got=%q want=%q", gotURL, rawURL)
	}
	var gotDiag, gotStats string
	if err := st.DB().QueryRowContext(ctx,
		`SELECT diagnostic_json, stats_json FROM pool_source_snapshots WHERE source_id=?`, sourceID).Scan(&gotDiag, &gotStats); err != nil {
		t.Fatalf("读取快照失败: %v", err)
	}
	if gotDiag != snapshotDiag || gotStats != snapshotStats {
		t.Fatalf("清洗不得改写快照证据: diag=%s stats=%s", gotDiag, gotStats)
	}

	// 第二次执行必须完全幂等，并且不再产生任何写回。
	if _, err := st.DB().ExecContext(ctx, `CREATE TABLE _sanitize_update_count (n INTEGER NOT NULL)`); err != nil {
		t.Fatalf("创建计数表失败: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO _sanitize_update_count(n) VALUES (0)`); err != nil {
		t.Fatalf("初始化计数表失败: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`CREATE TRIGGER _sanitize_task_update AFTER UPDATE ON pool_sync_tasks
		 BEGIN UPDATE _sanitize_update_count SET n=n+1; END`); err != nil {
		t.Fatalf("创建任务更新触发器失败: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`CREATE TRIGGER _sanitize_pool_update AFTER UPDATE ON rule_pools
		 BEGIN UPDATE _sanitize_update_count SET n=n+1; END`); err != nil {
		t.Fatalf("创建池更新触发器失败: %v", err)
	}
	if err := svc.SanitizeStoredSyncOutputs(ctx); err != nil {
		t.Fatalf("第二次存量清洗失败: %v", err)
	}
	var updates int
	if err := st.DB().QueryRowContext(ctx, `SELECT n FROM _sanitize_update_count`).Scan(&updates); err != nil {
		t.Fatalf("读取更新计数失败: %v", err)
	}
	if updates != 0 {
		t.Fatalf("幂等执行不应再写回，实际更新 %d 次", updates)
	}
}

func TestSanitizeStoredSyncOutputsTruncatesMalformedPerURL(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "脏历史池", nil, false, "04:00")
	if err != nil {
		t.Fatalf("创建素材池失败: %v", err)
	}
	malformed := "not-json token=SECRET " + strings.Repeat("x", 300)
	res, err := st.DB().ExecContext(ctx,
		`INSERT INTO pool_sync_tasks (pool_id,status,per_url_json,error,started_at,finished_at)
		 VALUES (?, 'failed', ?, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, p.ID, malformed)
	if err != nil {
		t.Fatalf("插入脏任务失败: %v", err)
	}
	taskID, _ := res.LastInsertId()
	if err := svc.SanitizeStoredSyncOutputs(ctx); err != nil {
		t.Fatalf("存量清洗失败: %v", err)
	}
	var perRaw string
	if err := st.DB().QueryRowContext(ctx, `SELECT per_url_json FROM pool_sync_tasks WHERE id=?`, taskID).Scan(&perRaw); err != nil {
		t.Fatalf("读取清洗结果失败: %v", err)
	}
	if strings.Contains(perRaw, "SECRET") {
		t.Fatalf("非法 per_url_json 回退路径泄漏敏感值: %s", perRaw)
	}
	if utf8.RuneCountInString(perRaw) > redact.MaxFieldRunes {
		t.Fatalf("非法 per_url_json 回退路径未按 %d rune 限额: %d", redact.MaxFieldRunes, utf8.RuneCountInString(perRaw))
	}
}
