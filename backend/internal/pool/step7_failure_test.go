package pool

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"vpn-sub/internal/redact"
)

func TestFailedSnapshotWriteFailureKeepsPointersAndDoesNotLeakDBError(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	var mu sync.Mutex
	statusCode := http.StatusOK
	body := "DOMAIN,a.com\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		code, payload := statusCode, body
		mu.Unlock()
		w.WriteHeader(code)
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	p, err := svc.Create(ctx, "写失败池", []SourceInput{{URL: srv.URL, SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建素材池失败: %v", err)
	}
	sourceID := p.Sources[0].ID
	if _, err := svc.SubmitSync(ctx, p.ID); err != nil {
		t.Fatalf("首次同步失败: %v", err)
	}
	waitSync(t, svc, p.ID, "succeeded")
	activeID := activeSnapshotID(t, st, p.ID)
	applyBody(t, st, svc, p.ID, sourceID, "b.com\n")
	pendingID := pendingSnapshotID(t, st, p.ID)
	if activeID == 0 || pendingID == 0 {
		t.Fatalf("测试前提失败: active=%d pending=%d", activeID, pendingID)
	}

	if _, err := st.DB().ExecContext(ctx,
		`CREATE TRIGGER _fail_snapshot_insert BEFORE INSERT ON pool_source_snapshots
		 BEGIN SELECT RAISE(ABORT, 'DB_SECRET'); END;`); err != nil {
		t.Fatalf("创建失败触发器失败: %v", err)
	}

	mu.Lock()
	statusCode = http.StatusInternalServerError
	body = "boom"
	mu.Unlock()
	if _, err := svc.SubmitSync(ctx, p.ID); err != nil {
		t.Fatalf("失败同步提交失败: %v", err)
	}
	task := waitSync(t, svc, p.ID, "failed")
	if len(task.PerURL) != 1 {
		t.Fatalf("失败回执数量异常: %+v", task)
	}
	if !strings.HasSuffix(task.PerURL[0].Error, "失败快照写入失败") {
		t.Fatalf("单 URL 错误应附带失败快照写入失败: %q", task.PerURL[0].Error)
	}
	if utf8.RuneCountInString(task.PerURL[0].Error) > redact.MaxFieldRunes {
		t.Fatalf("失败提示超出 %d rune: %q", redact.MaxFieldRunes, task.PerURL[0].Error)
	}
	if strings.Contains(task.PerURL[0].Error, "DB_SECRET") || strings.Contains(task.Error, "DB_SECRET") {
		t.Fatalf("DB 错误不得进入任务/API 字段: per=%q task=%q", task.PerURL[0].Error, task.Error)
	}

	var gotActive, gotPending int64
	if err := st.DB().QueryRowContext(ctx,
		`SELECT COALESCE(active_snapshot_id,0), COALESCE(pending_snapshot_id,0) FROM rule_pool_sources WHERE id=?`, sourceID).
		Scan(&gotActive, &gotPending); err != nil {
		t.Fatalf("读取指针失败: %v", err)
	}
	if gotActive != activeID || gotPending != pendingID {
		t.Fatalf("失败写快照不得改变 active/pending: active=%d/%d pending=%d/%d", gotActive, activeID, gotPending, pendingID)
	}
	var failedCount, totalCount int
	if err := st.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pool_source_snapshots WHERE source_id=? AND status='failed'`, sourceID).Scan(&failedCount); err != nil {
		t.Fatalf("读取 failed 数量失败: %v", err)
	}
	if err := st.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pool_source_snapshots WHERE source_id=?`, sourceID).Scan(&totalCount); err != nil {
		t.Fatalf("读取快照总数失败: %v", err)
	}
	if failedCount != 0 || totalCount != 2 {
		t.Fatalf("失败写库不得虚报 snapshot: failed=%d total=%d", failedCount, totalCount)
	}

	var poolErr string
	if err := st.DB().QueryRowContext(ctx, `SELECT sync_error FROM rule_pools WHERE id=?`, p.ID).Scan(&poolErr); err != nil {
		t.Fatalf("读取池错误失败: %v", err)
	}
	if strings.Contains(poolErr, "DB_SECRET") {
		t.Fatalf("DB 错误不得进入 Pool.sync_error: %q", poolErr)
	}
}
