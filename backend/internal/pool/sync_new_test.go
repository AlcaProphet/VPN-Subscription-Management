package pool

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"vpn-sub/internal/store"
)

func TestR2902ProvidedURLsConcurrentIntegration(t *testing.T) {
	url1 := os.Getenv("R29_02_URL_1")
	url2 := os.Getenv("R29_02_URL_2")
	if url1 == "" || url2 == "" {
		t.Skip("未提供 R29_02_URL_1/R29_02_URL_2，跳过真实链接集成测试")
	}
	_, svc := newTestService(t)
	ctx := context.Background()
	p1, err := svc.Create(ctx, "真实链接池一", []SourceInput{{URL: url1, SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建真实链接池一失败: %v", err)
	}
	p2, err := svc.Create(ctx, "真实链接池二", []SourceInput{{URL: url2, SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建真实链接池二失败: %v", err)
	}
	if _, err := svc.SubmitSync(ctx, p1.ID); err != nil {
		t.Fatalf("提交真实链接池一失败: %v", err)
	}
	if _, err := svc.SubmitSync(ctx, p2.ID); err != nil {
		t.Fatalf("池一运行期间提交真实链接池二失败: %v", err)
	}
	for _, item := range []struct {
		poolID int64
		url    string
	}{{p1.ID, url1}, {p2.ID, url2}} {
		task := waitSyncWithin(t, svc, item.poolID, "succeeded", 30*time.Second)
		if task.PoolID != item.poolID || len(task.PerURL) != 1 || task.PerURL[0].URL != item.url || !task.PerURL[0].OK || task.PerURL[0].Accepted == 0 {
			t.Fatalf("真实链接池 %d 回执异常: %+v", item.poolID, task)
		}
		t.Logf("真实链接池 %d: status=%s format=%s profile=%s accepted=%d excluded=%d rejected=%d duplicates=%d",
			item.poolID, task.Status, task.PerURL[0].Format, task.PerURL[0].Profile, task.PerURL[0].Accepted,
			task.PerURL[0].Excluded, task.PerURL[0].Rejected, task.PerURL[0].Duplicates)
	}
}

func TestDifferentPoolsSyncConcurrentlyAndSamePoolStillRejectsReentry(t *testing.T) {
	_, svc := newTestService(t)
	ctx := context.Background()
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		<-release
		_, _ = w.Write([]byte("a.com\n"))
	}))
	defer srv.Close()
	p1, err := svc.Create(ctx, "并发池一", []SourceInput{{URL: srv.URL, SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建池一失败: %v", err)
	}
	p2, err := svc.Create(ctx, "并发池二", []SourceInput{{URL: srv.URL, SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建池二失败: %v", err)
	}
	if _, err := svc.SubmitSync(ctx, p1.ID); err != nil {
		t.Fatalf("提交池一同步失败: %v", err)
	}
	if _, err := svc.SubmitSync(ctx, p1.ID); !errors.Is(err, ErrSyncRunning) {
		t.Fatalf("同池重复同步应返回 ErrSyncRunning，实际 %v", err)
	}
	if _, err := svc.SubmitSync(ctx, p2.ID); err != nil {
		t.Fatalf("池一运行时提交池二同步失败: %v", err)
	}
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("两个不同素材池未同时进入 HTTP 拉取阶段")
		}
	}
	close(release)
	waitSync(t, svc, p1.ID, "succeeded")
	waitSync(t, svc, p2.ID, "succeeded")
}

func waitSync(t *testing.T, svc *Service, poolID int64, terminal string) *SyncTask {
	return waitSyncWithin(t, svc, poolID, terminal, 5*time.Second)
}

func waitSyncWithin(t *testing.T, svc *Service, poolID int64, terminal string, timeout time.Duration) *SyncTask {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		task, err := svc.GetStatus(ctx, poolID)
		if err != nil {
			t.Fatalf("查询任务失败: %v", err)
		}
		if task != nil && task.Status == terminal {
			return task
		}
		time.Sleep(20 * time.Millisecond)
	}
	task, _ := svc.GetStatus(ctx, poolID)
	t.Fatalf("等待任务终态超时: %+v", task)
	return nil
}

func activeSnapshotID(t *testing.T, st *store.Store, poolID int64) int64 {
	t.Helper()
	var id int64
	if err := st.DB().QueryRow(`SELECT COALESCE(active_snapshot_id,0) FROM rule_pool_sources WHERE pool_id=? AND kind='url'`, poolID).Scan(&id); err != nil {
		t.Fatalf("读取 active snapshot 失败: %v", err)
	}
	return id
}

func TestSyncURLSuccessCreatesSnapshot(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("a.com\nb.com\n"))
	}))
	defer srv.Close()
	p, err := svc.Create(ctx, "同步池", []SourceInput{{URL: srv.URL, SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.SubmitSync(ctx, p.ID); err != nil {
		t.Fatalf("提交同步失败: %v", err)
	}
	task := waitSync(t, svc, p.ID, "succeeded")
	if len(task.PerURL) != 1 || !task.PerURL[0].OK || task.PerURL[0].Accepted != 2 {
		t.Fatalf("同步回执异常: %+v", task.PerURL)
	}
	if activeSnapshotID(t, st, p.ID) <= 0 {
		t.Fatal("active snapshot 应存在")
	}
	_, total, err := svc.ListEntries(ctx, p.ID, 1, 20, "")
	if err != nil || total != 2 {
		t.Fatalf("同步后条目数应为 2: total=%d err=%v", total, err)
	}
}

func TestSyncHardFailureNoActive(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer bad.Close()
	p, err := svc.Create(ctx, "失败池", []SourceInput{{URL: bad.URL, SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.SubmitSync(ctx, p.ID); err != nil {
		t.Fatalf("提交同步失败: %v", err)
	}
	task := waitSync(t, svc, p.ID, "failed")
	if len(task.PerURL) != 1 || task.PerURL[0].OK || task.PerURL[0].Error == "" {
		t.Fatalf("失败回执异常: %+v", task.PerURL)
	}
	if activeSnapshotID(t, st, p.ID) != 0 {
		t.Fatal("失败同步不应产生 active")
	}
}

func TestClearFinishedTasksKeepsActiveSnapshot(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("a.com\nb.com\n"))
	}))
	defer srv.Close()
	p, err := svc.Create(ctx, "清理池", []SourceInput{{URL: srv.URL, SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.SubmitSync(ctx, p.ID); err != nil {
		t.Fatalf("提交同步失败: %v", err)
	}
	waitSync(t, svc, p.ID, "succeeded")
	n, err := svc.ClearFinishedTasks(ctx, p.ID)
	if err != nil {
		t.Fatalf("清理历史失败: %v", err)
	}
	if n == 0 {
		t.Fatal("应清理至少一条终态任务")
	}
	_, total, err := svc.ListTasks(ctx, p.ID, 1, 20)
	if err != nil || total != 0 {
		t.Fatalf("清理后历史应为空: total=%d err=%v", total, err)
	}
	if activeSnapshotID(t, st, p.ID) <= 0 {
		t.Fatal("手动清理历史不应删除 active 快照")
	}
}

func TestCleanupOldTasksOnlyRemovesExpiredTerminal(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "过期池", nil, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO pool_sync_tasks (pool_id,status,per_url_json,error,started_at,finished_at)
		 VALUES (?, 'succeeded', '[]', '', datetime('now','-8 days'), datetime('now','-8 days'))`, p.ID); err != nil {
		t.Fatalf("插入过期任务失败: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO pool_sync_tasks (pool_id,status,per_url_json,error,started_at,finished_at)
		 VALUES (?, 'succeeded', '[]', '', datetime('now'), datetime('now'))`, p.ID); err != nil {
		t.Fatalf("插入近期任务失败: %v", err)
	}
	n, err := svc.CleanupOldTasks(ctx)
	if err != nil {
		t.Fatalf("全局清理失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("应只清理 1 条过期任务，实际 %d", n)
	}
	_, total, err := svc.ListTasks(ctx, p.ID, 1, 20)
	if err != nil || total != 1 {
		t.Fatalf("清理后应保留近期任务: total=%d err=%v", total, err)
	}
}
