package mail

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type blockingTransport struct {
	mu        sync.Mutex
	active    int
	maxActive int
	started   chan string
	release   chan struct{}
	panicOnce bool
}

func newBlockingTransport(capacity int) *blockingTransport {
	return &blockingTransport{
		started: make(chan string, capacity+8),
		release: make(chan struct{}),
	}
}

func (t *blockingTransport) availability(context.Context, string) (Availability, error) {
	return Availability{Available: true}, nil
}
func (t *blockingTransport) siteContext(context.Context) (string, string, error) {
	return "测试站点", "https://app.example.com", nil
}
func (t *blockingTransport) siteName(context.Context) (string, error) { return "测试站点", nil }
func (t *blockingTransport) frontendURL(context.Context) (string, error) {
	return "https://app.example.com", nil
}
func (t *blockingTransport) testSMTP(context.Context, string) error { return nil }
func (t *blockingTransport) sendJob(ctx context.Context, _ TemplateKind, to string, _ RenderValues) error {
	t.mu.Lock()
	t.active++
	if t.active > t.maxActive {
		t.maxActive = t.active
	}
	t.mu.Unlock()
	select {
	case t.started <- to:
	default:
	}
	if t.panicOnce {
		t.mu.Lock()
		t.panicOnce = false
		t.mu.Unlock()
		panic("test panic with secret@example.com")
	}
	select {
	case <-t.release:
		t.mu.Lock()
		t.active--
		t.mu.Unlock()
		return nil
	case <-ctx.Done():
		t.mu.Lock()
		t.active--
		t.mu.Unlock()
		return ctx.Err()
	}
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("等待超时: %s", msg)
}

func waitStarted(t *testing.T, ch <-chan string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		select {
		case <-ch:
		case <-time.After(2 * time.Second):
			t.Fatalf("等待第 %d 个发送任务开始超时", i+1)
		}
	}
}

func TestDispatcherConcurrencyAndQueueCapacity(t *testing.T) {
	tr := newBlockingTransport(dispatcherQueueSize + dispatcherWorkers)
	logs := NewActivityLog(ActivityLogCapacity)
	d := newDispatcherWithTransport(tr, logs, nil)
	t.Cleanup(d.Stop)

	// 先占满 2 个活动 worker。
	for i := 0; i < dispatcherWorkers; i++ {
		res := d.DispatchWelcome(context.Background(), int64(i+1), fmt.Sprintf("u%d@example.com", i), "selfreg")
		if res.Status != DispatchQueued {
			t.Fatalf("前两个任务应 queued: %+v", res)
		}
	}
	waitStarted(t, tr.started, dispatcherWorkers)

	// 等待队列恰好 100 项。
	for i := 0; i < dispatcherQueueSize; i++ {
		res := d.DispatchWelcome(context.Background(), int64(i+100), fmt.Sprintf("q%d@example.com", i), "selfreg")
		if res.Status != DispatchQueued {
			t.Fatalf("队列内任务应 queued: i=%d res=%+v", i, res)
		}
	}

	// 第 103 项立即被拒绝。
	res := d.DispatchWelcome(context.Background(), 999, "overflow@example.com", "selfreg")
	if res.Status != DispatchRejected || res.Reason != ReasonQueueFull || res.LogID == 0 {
		t.Fatalf("满队列必须立即 rejected/queue_full 且有 failed 日志: %+v", res)
	}
	rec := findActivity(logs, res.LogID)
	if rec == nil || rec.Status != ActivityFailed || rec.FailureStage == nil || *rec.FailureStage != FailureQueueFull {
		t.Fatalf("队列拒绝日志状态异常: %+v", rec)
	}
	if tr.maxActive > dispatcherWorkers {
		t.Fatalf("活动发送数不得超过 %d，实际 %d", dispatcherWorkers, tr.maxActive)
	}

	close(tr.release)
	waitFor(t, 3*time.Second, func() bool {
		return countStatus(logs, ActivityAccepted) == dispatcherWorkers+dispatcherQueueSize
	}, "队列任务应全部发送成功")
}

func TestDispatcherStopCancelsActiveAndDrainsWaiting(t *testing.T) {
	tr := newBlockingTransport(dispatcherQueueSize + dispatcherWorkers)
	logs := NewActivityLog(ActivityLogCapacity)
	d := newDispatcherWithTransport(tr, logs, nil)

	var ids []int64
	for i := 0; i < dispatcherWorkers+3; i++ {
		res := d.DispatchWelcome(context.Background(), int64(i+1), fmt.Sprintf("s%d@example.com", i), "selfreg")
		if res.Status != DispatchQueued {
			t.Fatalf("任务应 queued: %+v", res)
		}
		ids = append(ids, res.LogID)
	}
	waitStarted(t, tr.started, dispatcherWorkers)
	d.Stop()
	d.Stop() // 幂等

	for _, id := range ids {
		rec := findActivity(logs, id)
		if rec == nil || rec.Status != ActivityFailed || rec.FailureStage == nil || *rec.FailureStage != FailureCanceled {
			t.Fatalf("Stop 后任务应 failed/canceled: id=%d rec=%+v", id, rec)
		}
	}
	if got := tr.maxActive; got > dispatcherWorkers {
		t.Fatalf("活动发送数超限: %d", got)
	}
}

func TestDispatcherPauseResumeGenerationIsolation(t *testing.T) {
	tr := newBlockingTransport(dispatcherQueueSize + dispatcherWorkers)
	logs := NewActivityLog(ActivityLogCapacity)
	d := newDispatcherWithTransport(tr, logs, nil)
	t.Cleanup(d.Stop)

	res1 := d.DispatchWelcome(context.Background(), 1, "old@example.com", "selfreg")
	if res1.Status != DispatchQueued {
		t.Fatalf("旧任务应 queued: %+v", res1)
	}
	waitStarted(t, tr.started, 1)

	if err := d.PauseAndDrain(); err != nil {
		t.Fatalf("PauseAndDrain 失败: %v", err)
	}
	oldRec := findActivity(logs, res1.LogID)
	if oldRec == nil || oldRec.Status != ActivityFailed || oldRec.FailureStage == nil || *oldRec.FailureStage != FailureCanceled {
		t.Fatalf("旧任务应 failed/canceled: %+v", oldRec)
	}

	resPaused := d.DispatchWelcome(context.Background(), 2, "paused@example.com", "selfreg")
	if resPaused.Status != DispatchRejected || resPaused.Reason != ReasonDispatcherUnavailable {
		t.Fatalf("暂停期间必须 rejected/dispatcher_unavailable: %+v", resPaused)
	}

	if err := d.Resume(); err != nil {
		t.Fatalf("Resume 失败: %v", err)
	}
	// 释放旧代次阻塞传输；旧 worker 已退出，新代次只处理新任务。
	tr.mu.Lock()
	tr.release = make(chan struct{})
	tr.mu.Unlock()
	close(tr.release)

	res2 := d.DispatchWelcome(context.Background(), 3, "new@example.com", "selfreg")
	if res2.Status != DispatchQueued || res2.LogID <= resPaused.LogID {
		t.Fatalf("Resume 后新任务应 queued 且 ID 单调: %+v", res2)
	}
	waitFor(t, 2*time.Second, func() bool {
		rec := findActivity(logs, res2.LogID)
		return rec != nil && rec.Status == ActivityAccepted
	}, "新代次任务应发送成功")
	if rec := findActivity(logs, res1.LogID); rec == nil || rec.Status != ActivityFailed {
		t.Fatalf("旧任务不得被新代次改写: %+v", rec)
	}
}

func TestDispatcherSkippedAndPreReadFailure(t *testing.T) {
	logs := NewActivityLog(ActivityLogCapacity)
	tr := &staticTransport{availabilityFn: func(context.Context, string) (Availability, error) {
		return Availability{Reason: ReasonScopeDisabled}, nil
	}}
	d := newDispatcherWithTransport(tr, logs, nil)
	t.Cleanup(d.Stop)
	res := d.DispatchWelcome(context.Background(), 1, "a@example.com", "selfreg")
	if res.Status != DispatchSkipped || res.Reason != ReasonScopeDisabled || res.LogID != 0 {
		t.Fatalf("scope 关闭应 skipped 且不写日志: %+v", res)
	}
	if got := len(logs.Snapshot()); got != 0 {
		t.Fatalf("skipped 不应写日志: %d", got)
	}

	tr.availabilityFn = func(context.Context, string) (Availability, error) {
		return Availability{}, errors.New("read failed")
	}
	res = d.DispatchWelcome(context.Background(), 1, "a@example.com", "selfreg")
	if res.Status != DispatchRejected || res.Reason != ReasonConfigReadFailed || res.LogID == 0 {
		t.Fatalf("严格读取失败应 rejected/config_read_failed: %+v", res)
	}
	rec := findActivity(logs, res.LogID)
	if rec == nil || rec.FailureStage == nil || *rec.FailureStage != FailureConfig {
		t.Fatalf("读取失败日志阶段应为 config: %+v", rec)
	}
}

func TestDispatcherPanicRecovery(t *testing.T) {
	tr := &staticTransport{panicFirst: true, release: make(chan struct{})}
	close(tr.release)
	logs := NewActivityLog(ActivityLogCapacity)
	d := newDispatcherWithTransport(tr, logs, nil)
	t.Cleanup(d.Stop)

	first := d.DispatchWelcome(context.Background(), 1, "panic@example.com", "selfreg")
	second := d.DispatchWelcome(context.Background(), 2, "ok@example.com", "selfreg")
	waitFor(t, 2*time.Second, func() bool {
		a, b := findActivity(logs, first.LogID), findActivity(logs, second.LogID)
		if a == nil || b == nil {
			return false
		}
		internalFailed := (a.Status == ActivityFailed && a.FailureStage != nil && *a.FailureStage == FailureInternal) ||
			(b.Status == ActivityFailed && b.FailureStage != nil && *b.FailureStage == FailureInternal)
		accepted := a.Status == ActivityAccepted || b.Status == ActivityAccepted
		return internalFailed && accepted
	}, "panic 后应隔离并继续消费")
}

func TestDispatcherSyncSMTPTestLogs(t *testing.T) {
	tr := &staticTransport{}
	logs := NewActivityLog(ActivityLogCapacity)
	d := newDispatcherWithTransport(tr, logs, nil)
	t.Cleanup(d.Stop)

	if err := d.TestSMTP(context.Background(), "admin@example.com"); err != nil {
		t.Fatalf("同步测试成功路径失败: %v", err)
	}
	rec := logs.Snapshot()[0]
	if rec.Kind != ActivityKindSMTPTest || rec.Source != SourceSMTPTest || rec.Status != ActivityAccepted ||
		rec.StartedAt == nil || rec.FinishedAt == nil || rec.FailureStage != nil {
		t.Fatalf("成功测试日志异常: %+v", rec)
	}

	tr.testErr = errors.New("535 password=secret")
	if err := d.TestSMTP(context.Background(), "admin@example.com"); err == nil {
		t.Fatal("同步测试失败应返回安全错误")
	}
	rec = logs.Snapshot()[0]
	if rec.Status != ActivityFailed || rec.FailureStage == nil || *rec.FailureStage != FailureInternal {
		t.Fatalf("失败测试日志异常: %+v", rec)
	}
	if strings.Contains(rec.RecipientMasked, "admin@example.com") {
		t.Fatalf("日志不得保存完整收件人: %+v", rec)
	}
}

func countStatus(logs *ActivityLog, status ActivityStatus) int {
	n := 0
	for _, rec := range logs.Snapshot() {
		if rec.Status == status {
			n++
		}
	}
	return n
}

func findActivity(logs *ActivityLog, id int64) *ActivityRecord {
	for _, rec := range logs.Snapshot() {
		if rec.ID == id {
			out := rec
			return &out
		}
	}
	return nil
}

// staticTransport 可控可用性/发送结果，供不关心真实阻塞的派发测试使用。
type staticTransport struct {
	availabilityFn func(context.Context, string) (Availability, error)
	release        chan struct{}
	panicFirst     bool
	testErr        error
	mu             sync.Mutex
}

func (t *staticTransport) availability(ctx context.Context, scope string) (Availability, error) {
	if t.availabilityFn != nil {
		return t.availabilityFn(ctx, scope)
	}
	return Availability{Available: true}, nil
}
func (t *staticTransport) siteContext(context.Context) (string, string, error) {
	return "测试站点", "https://app.example.com", nil
}
func (t *staticTransport) siteName(context.Context) (string, error) { return "测试站点", nil }
func (t *staticTransport) frontendURL(context.Context) (string, error) {
	return "https://app.example.com", nil
}
func (t *staticTransport) testSMTP(context.Context, string) error { return t.testErr }
func (t *staticTransport) sendJob(ctx context.Context, _ TemplateKind, _ string, _ RenderValues) error {
	t.mu.Lock()
	if t.panicFirst {
		t.panicFirst = false
		t.mu.Unlock()
		panic("test panic")
	}
	release := t.release
	t.mu.Unlock()
	if release != nil {
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
