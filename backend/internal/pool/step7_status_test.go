package pool

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestLatestFailedRecoversAfterSuccessfulRetry(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	var mu sync.Mutex
	statusCode := http.StatusOK
	body := "DOMAIN,a.com\n"
	setResponse := func(code int, payload string) {
		mu.Lock()
		defer mu.Unlock()
		statusCode, body = code, payload
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		code, payload := statusCode, body
		mu.Unlock()
		w.WriteHeader(code)
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	p, err := svc.Create(ctx, "恢复池", []SourceInput{{URL: srv.URL, SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建素材池失败: %v", err)
	}
	if _, err := svc.SubmitSync(ctx, p.ID); err != nil {
		t.Fatalf("首次同步失败: %v", err)
	}
	waitSync(t, svc, p.ID, "succeeded")
	activeID := activeSnapshotID(t, st, p.ID)
	if activeID == 0 {
		t.Fatal("首次成功应产生 active")
	}

	setResponse(http.StatusInternalServerError, "boom")
	if _, err := svc.SubmitSync(ctx, p.ID); err != nil {
		t.Fatalf("失败同步提交失败: %v", err)
	}
	waitSync(t, svc, p.ID, "failed")
	statuses, err := svc.ListSourceStatuses(ctx, p.ID)
	if err != nil || len(statuses) != 1 {
		t.Fatalf("读取失败状态失败: %+v err=%v", statuses, err)
	}
	failed := statuses[0]
	if failed.LatestAttempt == nil || failed.LatestAttempt.Status != "failed" {
		t.Fatalf("最新尝试应为 failed: %+v", failed.LatestAttempt)
	}
	if failed.LatestFailed == nil || failed.LatestFailed.ID != failed.LatestAttempt.ID {
		t.Fatalf("latest_failed 应指向本次失败: %+v", failed.LatestFailed)
	}
	if failed.Active == nil || failed.Active.ID != activeID {
		t.Fatalf("最近失败后旧 active 应继续返回: %+v", failed.Active)
	}
	failedID := failed.LatestAttempt.ID

	// 失败后再次成功：主状态恢复 active，旧 failed 只作为历史。
	setResponse(http.StatusOK, "DOMAIN,a.com\n")
	if _, err := svc.SubmitSync(ctx, p.ID); err != nil {
		t.Fatalf("恢复同步提交失败: %v", err)
	}
	waitSync(t, svc, p.ID, "succeeded")
	statuses, err = svc.ListSourceStatuses(ctx, p.ID)
	if err != nil || len(statuses) != 1 {
		t.Fatalf("读取恢复状态失败: %+v err=%v", statuses, err)
	}
	recovered := statuses[0]
	if recovered.LatestAttempt == nil || recovered.LatestAttempt.Status != "active" {
		t.Fatalf("恢复后主状态应为 active: %+v", recovered.LatestAttempt)
	}
	if recovered.Active == nil || recovered.Active.ID != recovered.LatestAttempt.ID {
		t.Fatalf("恢复后 active 指针应指向最新快照: %+v", recovered.Active)
	}
	if recovered.LatestFailed == nil || recovered.LatestFailed.ID != failedID {
		t.Fatalf("旧 failed 应保留为历史: %+v", recovered.LatestFailed)
	}
}

func TestSourceSnapshotStableOrderingWithSameTimestamp(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "同时间戳池", []SourceInput{{URL: "https://example.com/rules", SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建素材池失败: %v", err)
	}
	sourceID := p.Sources[0].ID
	insert := func(status string) int64 {
		t.Helper()
		res, err := st.DB().ExecContext(ctx,
			`INSERT INTO pool_source_snapshots
			   (source_id, status, format, profile, input_count, recognized_count, accepted_count, excluded_count, rejected_count, duplicate_count, diagnostic_json, stats_json, created_at)
			 VALUES (?, ?, '', '', 0, 0, 0, 0, 0, 0, '[]', '{}', '2026-09-10 12:00:00')`, sourceID, status)
		if err != nil {
			t.Fatalf("插入快照失败: %v", err)
		}
		id, _ := res.LastInsertId()
		return id
	}
	firstFailed := insert("failed")
	secondFailed := insert("failed")
	thirdActive := insert("active")

	list, total, err := svc.ListSourceSnapshots(ctx, p.ID, sourceID, 1, 20)
	if err != nil || total != 3 || len(list) != 3 {
		t.Fatalf("读取快照历史失败: total=%d len=%d err=%v", total, len(list), err)
	}
	if list[0].ID != thirdActive || list[1].ID != secondFailed || list[2].ID != firstFailed {
		t.Fatalf("同时间戳应按 id DESC 稳定排序: %+v", list)
	}
	statuses, err := svc.ListSourceStatuses(ctx, p.ID)
	if err != nil || len(statuses) != 1 {
		t.Fatalf("读取状态失败: %+v err=%v", statuses, err)
	}
	if statuses[0].LatestAttempt == nil || statuses[0].LatestAttempt.ID != thirdActive {
		t.Fatalf("latest_attempt 应按 id DESC 选择: %+v", statuses[0].LatestAttempt)
	}
	if statuses[0].LatestFailed == nil || statuses[0].LatestFailed.ID != secondFailed {
		t.Fatalf("latest_failed 应按 id DESC 选择: %+v", statuses[0].LatestFailed)
	}
}
