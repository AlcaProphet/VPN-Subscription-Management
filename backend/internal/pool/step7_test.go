package pool

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPFailureWritesFailedSnapshot(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer bad.Close()
	p, err := svc.Create(ctx, "失败快照池", []SourceInput{{URL: bad.URL, SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.SubmitSync(ctx, p.ID); err != nil {
		t.Fatalf("提交同步失败: %v", err)
	}
	waitSync(t, svc, p.ID, "failed")
	var failedCount int
	if err := st.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pool_source_snapshots WHERE status='failed'`).Scan(&failedCount); err != nil {
		t.Fatalf("查询 failed snapshot 失败: %v", err)
	}
	if failedCount != 1 {
		t.Fatalf("HTTP 失败应持久化 failed snapshot: %d", failedCount)
	}
	if activeSnapshotID(t, st, p.ID) != 0 {
		t.Fatal("失败同步不应产生 active")
	}
}

func TestListSourceStatusesUsesDisplayURL(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "状态池", []SourceInput{{URL: "https://example.com/rules?token=SECRET", SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	applyBody(t, st, svc, p.ID, p.Sources[0].ID, "DOMAIN,a.com\n")
	statuses, err := svc.ListSourceStatuses(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListSourceStatuses 失败: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("状态数量应为 1: %d", len(statuses))
	}
	if statuses[0].DisplayURL != "https://example.com/rules?token=***" {
		t.Fatalf("DisplayURL 应脱敏: %q", statuses[0].DisplayURL)
	}
	if statuses[0].LatestAttempt == nil || statuses[0].LatestAttempt.Accepted != 1 {
		t.Fatalf("latest_attempt 异常: %+v", statuses[0].LatestAttempt)
	}
	var rawURL string
	if err := st.DB().QueryRowContext(ctx, `SELECT COALESCE(url,'') FROM rule_pool_sources WHERE id=?`, p.Sources[0].ID).Scan(&rawURL); err != nil {
		t.Fatalf("查询原始 URL 失败: %v", err)
	}
	if rawURL != "https://example.com/rules?token=SECRET" {
		t.Fatalf("rule_pool_sources.url 不得被脱敏覆盖: %q", rawURL)
	}
}

func TestListSourceSnapshotsPaginationAndTotal(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "历史池", []SourceInput{{URL: "https://example.com/rules", SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	// 第一次成功
	applyBody(t, st, svc, p.ID, p.Sources[0].ID, "DOMAIN,a.com\n")
	// 第二次也成功（同一事务直接 apply 会再生成一条 active）
	applyBody(t, st, svc, p.ID, p.Sources[0].ID, "DOMAIN,b.com\n")
	list, total, err := svc.ListSourceSnapshots(ctx, p.ID, p.Sources[0].ID, 1, 1)
	if err != nil {
		t.Fatalf("ListSourceSnapshots 失败: %v", err)
	}
	if total != 2 || len(list) != 1 {
		t.Fatalf("分页异常: total=%d len=%d", total, len(list))
	}
	if list[0].Accepted != 1 {
		t.Fatalf("最新快照 accepted 异常: %+v", list[0])
	}
}
