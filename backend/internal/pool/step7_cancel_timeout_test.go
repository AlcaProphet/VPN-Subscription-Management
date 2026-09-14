package pool

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTaskCancellationDoesNotWriteFailedSnapshot(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	started := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		<-r.Context().Done()
	}))
	defer srv.Close()
	p, err := svc.Create(ctx, "取消边界池", []SourceInput{{URL: srv.URL, SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建素材池失败: %v", err)
	}
	taskID, err := svc.SubmitSync(ctx, p.ID)
	if err != nil {
		t.Fatalf("提交同步失败: %v", err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("同步请求未开始")
	}
	if err := svc.CancelSync(ctx, p.ID, taskID); err != nil {
		t.Fatalf("取消任务失败: %v", err)
	}
	task := waitSync(t, svc, p.ID, "failed")
	if task.Error != "同步任务已取消" {
		t.Fatalf("取消终态错误: %+v", task)
	}
	waitSyncWorkerStopped(t, svc, taskID)

	var snapshotCount int
	if err := st.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pool_source_snapshots WHERE source_id=?`, p.Sources[0].ID).Scan(&snapshotCount); err != nil {
		t.Fatalf("查询快照数量失败: %v", err)
	}
	if snapshotCount != 0 {
		t.Fatalf("任务级取消不应写 per-URL failed 快照: %d", snapshotCount)
	}
	var active, pending int64
	if err := st.DB().QueryRowContext(ctx,
		`SELECT COALESCE(active_snapshot_id,0), COALESCE(pending_snapshot_id,0) FROM rule_pool_sources WHERE id=?`, p.Sources[0].ID).
		Scan(&active, &pending); err != nil {
		t.Fatalf("读取指针失败: %v", err)
	}
	if active != 0 || pending != 0 {
		t.Fatalf("取消不得改变指针: active=%d pending=%d", active, pending)
	}
}

func TestURLTimeoutWritesFailedSnapshotWithNetworkReason(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()
	p, err := svc.Create(ctx, "超时池", []SourceInput{{URL: srv.URL, SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建素材池失败: %v", err)
	}
	sourceID := p.Sources[0].ID
	client := &http.Client{Timeout: 20 * time.Millisecond}
	result := svc.syncOne(ctx, st, client, p.ID, sourceID, srv.URL, SourceModeAuto)
	if result.OK {
		t.Fatalf("客户端超时不应视为成功: %+v", result)
	}
	stats := readLatestSnapshotStats(t, st, sourceID)
	if stats.SchemaVersion != 1 || stats.Decision == nil ||
		len(stats.Decision.ReasonCodes) != 1 || stats.Decision.ReasonCodes[0] != "network_error" {
		t.Fatalf("单 URL 超时应写 failed snapshot 且 reason_code=network_error: %+v", stats)
	}
	var status string
	if err := st.DB().QueryRowContext(ctx, `SELECT status FROM pool_source_snapshots WHERE source_id=? ORDER BY id DESC LIMIT 1`, sourceID).Scan(&status); err != nil {
		t.Fatalf("读取快照状态失败: %v", err)
	}
	if status != "failed" {
		t.Fatalf("超时快照状态应为 failed: %s", status)
	}
	if activeSnapshotID(t, st, p.ID) != 0 {
		t.Fatal("超时失败不得产生 active")
	}
}
