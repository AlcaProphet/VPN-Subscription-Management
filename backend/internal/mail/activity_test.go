package mail

import (
	"context"
	"sync"
	"testing"
)

type captureResultRecorder struct {
	mu      sync.Mutex
	records []ResultRecord
}

func (r *captureResultRecorder) TryRecord(record ResultRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, cloneResultRecord(record))
}

func (r *captureResultRecorder) snapshot() []ResultRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]ResultRecord(nil), r.records...)
}

func TestMaskRecipient(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "kyle@example.com", want: "k***@example.com"},
		{in: "A@Example.COM", want: "A***@example.com"},
		{in: "  kyle@example.com  ", want: "k***@example.com"},
		{in: "", want: "***"},
		{in: "not-an-email", want: "***"},
		{in: "@example.com", want: "***"},
		{in: "kyle@", want: "***"},
		{in: `"Kyle" <kyle@example.com>`, want: "***"},
	}
	for _, tc := range tests {
		if got := MaskRecipient(tc.in); got != tc.want {
			t.Fatalf("MaskRecipient(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestActivityLogStateMachineAndCapacity(t *testing.T) {
	log := NewActivityLog(3)
	uid := int64(7)

	queuedID := log.BeginQueued(TemplateWelcomeLocal, "selfreg", &uid, "kyle@example.com")
	recs := log.Snapshot()
	if len(recs) != 1 || recs[0].Status != ActivityQueued || recs[0].RecipientMasked != "k***@example.com" {
		t.Fatalf("queued 记录异常: %+v", recs)
	}
	if recs[0].StartedAt != nil || recs[0].QueueDurationMS != nil || recs[0].SendDurationMS != nil {
		t.Fatalf("queued 阶段耗时字段应为 null: %+v", recs[0])
	}

	log.MarkSending(queuedID)
	log.MarkAccepted(queuedID)
	log.MarkAccepted(queuedID) // 重复终态无效果
	recs = log.Snapshot()
	if recs[0].Status != ActivityAccepted || recs[0].FinishedAt == nil || recs[0].QueueDurationMS == nil || recs[0].SendDurationMS == nil {
		t.Fatalf("accepted 记录异常: %+v", recs[0])
	}

	failedID := log.BeginQueued(TemplateApprovalApproved, "approval", &uid, "b@example.com")
	log.MarkQueuedFailed(failedID, FailureQueueFull)
	recs = log.Snapshot()
	if recs[0].Status != ActivityFailed || recs[0].FailureStage == nil || *recs[0].FailureStage != FailureQueueFull {
		t.Fatalf("队列失败记录异常: %+v", recs[0])
	}
	if recs[0].StartedAt != nil || recs[0].QueueDurationMS != nil || recs[0].SendDurationMS != nil {
		t.Fatalf("队列失败不应伪造耗时: %+v", recs[0])
	}

	// 容量 3：再写 3 条后只保留最近 3 条，旧 ID 更新静默跳过。
	id4 := log.BeginQueued(TemplatePasswordReset, "public_forgot", &uid, "c@example.com")
	id5 := log.BeginQueued(TemplatePasswordReset, "admin_single", &uid, "d@example.com")
	id6 := log.BeginQueued(TemplatePasswordReset, "admin_batch", &uid, "e@example.com")
	log.MarkSending(id4)
	log.MarkSendFailed(id4, FailureConnect)
	recs = log.Snapshot()
	if len(recs) != 3 || recs[0].ID != id6 || recs[1].ID != id5 || recs[2].ID != id4 {
		t.Fatalf("容量淘汰或排序异常: %+v", recs)
	}
	log.MarkSending(queuedID) // 已淘汰 ID：不得复活
	for _, rec := range log.Snapshot() {
		if rec.ID == queuedID {
			t.Fatal("已淘汰 ID 不应复活")
		}
	}

	// Clear 保留单调 ID；清空后的迟到更新不得重新插入。
	log.Clear()
	if got := log.Snapshot(); len(got) != 0 {
		t.Fatalf("Clear 后应为空: %+v", got)
	}
	log.MarkAccepted(id6)
	if got := log.Snapshot(); len(got) != 0 {
		t.Fatalf("迟到更新不得重新插入: %+v", got)
	}
	id7 := log.BeginQueued(TemplatePasswordReset, "public_forgot", &uid, "f@example.com")
	if id7 <= id6 {
		t.Fatalf("Clear 后 ID 必须继续单调: id6=%d id7=%d", id6, id7)
	}
}

// TestActivityLogRejectsIllegalTransitions 锁定封闭状态机：非法前置状态不得直接进入终态。
func TestActivityLogRejectsIllegalTransitions(t *testing.T) {
	log := NewActivityLog(10)
	uid := int64(1)

	queued := log.BeginQueued(TemplateWelcomeLocal, "selfreg", &uid, "queued@example.com")
	log.MarkAccepted(queued)
	rec := log.Snapshot()[0]
	if rec.Status != ActivityQueued || rec.StartedAt != nil || rec.FinishedAt != nil ||
		rec.QueueDurationMS != nil || rec.SendDurationMS != nil || rec.FailureStage != nil {
		t.Fatalf("queued 直接 MarkAccepted 必须无效果: %+v", rec)
	}
	log.MarkSendFailed(queued, FailureConnect)
	rec = log.Snapshot()[0]
	if rec.Status != ActivityQueued || rec.FailureStage != nil {
		t.Fatalf("queued 直接 MarkSendFailed 必须无效果: %+v", rec)
	}

	sending := log.BeginQueued(TemplateWelcomeLocal, "selfreg", &uid, "sending@example.com")
	log.MarkSending(sending)
	log.MarkAccepted(sending)
	rec = log.Snapshot()[0]
	if rec.Status != ActivityAccepted || rec.StartedAt == nil || rec.FinishedAt == nil ||
		rec.QueueDurationMS == nil || rec.SendDurationMS == nil {
		t.Fatalf("sending 合法 accepted 路径异常: %+v", rec)
	}

	// 已进入终态后所有非法/重复更新均不得回退或改写。
	before := log.Snapshot()[0]
	log.MarkQueuedFailed(sending, FailureQueueFull)
	log.MarkSendFailed(sending, FailureConnect)
	log.MarkSending(sending)
	after := log.Snapshot()[0]
	if after.Status != ActivityAccepted || after.FinishedAt == nil || after.FailureStage != nil ||
		after.ID != before.ID || !after.FinishedAt.Equal(*before.FinishedAt) {
		t.Fatalf("终态记录不得回退/改写: before=%+v after=%+v", before, after)
	}

	// queued 只能通过 MarkQueuedFailed 进入 failed，且不伪造发送耗时。
	queueFail := log.BeginQueued(TemplateApprovalRejected, "approval", &uid, "queuefail@example.com")
	log.MarkQueuedFailed(queueFail, FailureQueueFull)
	rec = log.Snapshot()[0]
	if rec.Status != ActivityFailed || rec.FailureStage == nil || *rec.FailureStage != FailureQueueFull ||
		rec.StartedAt != nil || rec.QueueDurationMS != nil || rec.SendDurationMS != nil {
		t.Fatalf("queued→failed 队列拒绝路径异常: %+v", rec)
	}
}

func TestActivityLogConcurrent(t *testing.T) {
	log := NewActivityLog(500)
	const goroutines = 16
	const perGoroutine = 80
	var wg sync.WaitGroup
	ctx := context.Background()
	_ = ctx
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			uid := int64(1)
			for i := 0; i < perGoroutine; i++ {
				id := log.BeginQueued(TemplatePasswordReset, "public_forgot", &uid, "a@example.com")
				log.MarkSending(id)
				if i%2 == 0 {
					log.MarkAccepted(id)
				} else {
					log.MarkSendFailed(id, FailureConnect)
				}
				log.Snapshot()
			}
		}()
	}
	wg.Wait()
	if got := len(log.Snapshot()); got != ActivityLogCapacity {
		t.Fatalf("并发写入后容量应为 %d，实际 %d", ActivityLogCapacity, got)
	}
	log.Clear()
}

func TestActivityLogRecordsOnlyLegalTerminalTransitions(t *testing.T) {
	activity := NewActivityLog(10)
	recorder := &captureResultRecorder{}
	activity.SetResultRecorder(recorder)
	uid := int64(9)

	acceptedID := activity.BeginQueued(TemplateWelcomeLocal, "selfreg", &uid, "full@example.com")
	activity.MarkAccepted(acceptedID) // queued 不能直接进入 accepted，也不得旁路记录。
	activity.MarkSending(acceptedID)
	activity.MarkAccepted(acceptedID)
	activity.MarkAccepted(acceptedID) // 重复终态不得重复记录。

	failedID := activity.BeginSending(ActivityKindSMTPTest, SourceSMTPTest, nil, "smtp@example.com")
	activity.MarkSendFailed(failedID, FailureAuth)
	activity.MarkSendFailed(failedID, FailureConnect)
	activity.RecordTerminalFailed(TemplatePasswordReset, SourceAdminBatch, &uid, "batch@example.com", FailureQueueFull)

	records := recorder.snapshot()
	if len(records) != 3 {
		t.Fatalf("每个合法终态应恰好记录一次，实际 %+v", records)
	}
	if records[0].Result != ActivityAccepted || records[0].FailureStage != nil || records[0].RecipientMasked != "f***@example.com" {
		t.Fatalf("accepted 旁路记录异常: %+v", records[0])
	}
	if records[1].Result != ActivityFailed || records[1].FailureStage == nil || *records[1].FailureStage != FailureAuth {
		t.Fatalf("发送失败旁路记录异常: %+v", records[1])
	}
	if records[2].Result != ActivityFailed || records[2].FailureStage == nil || *records[2].FailureStage != FailureQueueFull {
		t.Fatalf("派发前失败旁路记录异常: %+v", records[2])
	}
}
