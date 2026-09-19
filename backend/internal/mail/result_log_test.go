package mail

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"vpn-sub/internal/store"
	"vpn-sub/migrations"
)

func newTestResultLog(t *testing.T) (*store.Store, *ResultLog) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := st.Migrate(context.Background(), migrations.FS); err != nil {
		_ = st.Close()
		t.Fatalf("迁移失败: %v", err)
	}
	r, err := NewResultLog(st.DB(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		_ = st.Close()
		t.Fatalf("创建结果日志失败: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = r.Stop(ctx)
		_ = st.Close()
	})
	return st, r
}

func TestResultLogWriteQueryAndConstraints(t *testing.T) {
	st, resultLog := newTestResultLog(t)
	uid := int64(7)
	now := time.Now().UTC().Truncate(time.Microsecond)
	stage := FailureAuth
	resultLog.TryRecord(ResultRecord{Kind: TemplateWelcomeLocal, Source: "selfreg", UserID: &uid,
		RecipientMasked: "k***@example.com", Result: ActivityAccepted, RecordedAt: now})
	resultLog.TryRecord(ResultRecord{Kind: TemplatePasswordReset, Source: SourceAdminSingle, UserID: &uid,
		RecipientMasked: "k***@example.com", Result: ActivityFailed, FailureStage: &stage, RecordedAt: now.Add(time.Second)})
	if err := resultLog.Flush(context.Background()); err != nil {
		t.Fatalf("等待写入失败: %v", err)
	}

	list, total, err := resultLog.Query(context.Background(), 1, 20, string(TemplatePasswordReset), ActivityFailed)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].FailureStage == nil || *list[0].FailureStage != FailureAuth {
		t.Fatalf("筛选结果异常: total=%d list=%+v", total, list)
	}
	if list[0].RecipientMasked != "k***@example.com" || list[0].UserID == nil || *list[0].UserID != uid {
		t.Fatalf("安全字段往返异常: %+v", list[0])
	}

	list, total, err = resultLog.Query(context.Background(), 2, 20, "", "")
	if err != nil || total != 2 || list == nil || len(list) != 0 {
		t.Fatalf("越界分页应返回空数组: total=%d list=%v err=%v", total, list, err)
	}
	if _, err := st.DB().Exec(`INSERT INTO mail_result_logs
		(kind, source, recipient_masked, result, failure_stage, recorded_at)
		VALUES ('smtp_test','smtp_test','***','accepted','auth',?)`, now); err == nil {
		t.Fatal("accepted 不得保存失败阶段")
	}
}

func TestResultLogClearUsesTerminalTimeBoundary(t *testing.T) {
	_, resultLog := newTestResultLog(t)
	oldTime := time.Now().Add(-time.Minute)
	resultLog.TryRecord(ResultRecord{Kind: ActivityKindSMTPTest, Source: SourceSMTPTest,
		RecipientMasked: "a***@example.com", Result: ActivityAccepted, RecordedAt: oldTime})
	if err := resultLog.Clear(context.Background()); err != nil {
		t.Fatalf("清空失败: %v", err)
	}
	// 模拟清空命令之后才进入 writer、但实际在清空前已经形成的旧终态。
	resultLog.TryRecord(ResultRecord{Kind: ActivityKindSMTPTest, Source: SourceSMTPTest,
		RecipientMasked: "b***@example.com", Result: ActivityAccepted, RecordedAt: oldTime})
	resultLog.TryRecord(ResultRecord{Kind: ActivityKindSMTPTest, Source: SourceSMTPTest,
		RecipientMasked: "c***@example.com", Result: ActivityAccepted, RecordedAt: time.Now().Add(time.Second)})
	if err := resultLog.Flush(context.Background()); err != nil {
		t.Fatalf("等待写入失败: %v", err)
	}
	list, total, err := resultLog.Query(context.Background(), 1, 20, "", "")
	if err != nil || total != 1 || len(list) != 1 || list[0].RecipientMasked != "c***@example.com" {
		t.Fatalf("清空时间边界异常: total=%d list=%+v err=%v", total, list, err)
	}
}

func TestResultLogFailureNeverBlocksCaller(t *testing.T) {
	st, resultLog := newTestResultLog(t)
	if err := st.Close(); err != nil {
		t.Fatalf("关闭数据库失败: %v", err)
	}
	start := time.Now()
	for i := 0; i < ResultLogQueueSize*3; i++ {
		resultLog.TryRecord(ResultRecord{Kind: ActivityKindSMTPTest, Source: SourceSMTPTest,
			RecipientMasked: "***", Result: ActivityAccepted, RecordedAt: time.Now()})
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("日志失败或缓冲满不应阻塞调用方: %v", elapsed)
	}
}
