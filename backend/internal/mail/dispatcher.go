package mail

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
)

// DispatchStatus 派发结果状态。
type DispatchStatus string

const (
	DispatchSkipped  DispatchStatus = "skipped"
	DispatchQueued   DispatchStatus = "queued"
	DispatchRejected DispatchStatus = "rejected"
)

// DispatchResult 派发方法返回的封闭结果；不暴露底层 SMTP/数据库错误。
type DispatchResult struct {
	Status DispatchStatus
	Reason DispatchReason
	LogID  int64
}

// JobKind 业务邮件固定类型；禁止新增任意回调或闭包任务。
type JobKind string

const (
	JobWelcomeLocal     JobKind = "welcome_local"
	JobWelcomeOIDC      JobKind = "welcome_oidc"
	JobApprovalApproved JobKind = "approval_approved"
	JobApprovalRejected JobKind = "approval_rejected"
	JobPasswordReset    JobKind = "password_reset"
)

// ActivityKindSMTPTest 同步 SMTP 测试日志类型，不创建业务 Job。
const ActivityKindSMTPTest TemplateKind = "smtp_test"

// 业务来源值；用于 ActivityLog source 展示，不参与权限判断。
const (
	SourcePublicForgot = "public_forgot"
	SourceAdminSingle  = "admin_single"
	SourceAdminBatch   = "admin_batch"
	SourceSMTPTest     = "smtp_test"
)

// Job 类型化邮件任务；字段不导出、不实现 JSON 序列化、不写入普通日志。
type Job struct {
	logID  int64
	kind   JobKind
	source string
	userID *int64
	to     string
	values RenderValues
}

func (k JobKind) templateKind() TemplateKind {
	return TemplateKind(k)
}

// ErrDispatcherStopped 派发器已永久停止，不能恢复。
var ErrDispatcherStopped = errors.New("邮件派发器已停止")

type dispatcherTransport interface {
	availability(ctx context.Context, scope string) (Availability, error)
	siteContext(ctx context.Context) (siteName, loginURL string, err error)
	siteName(ctx context.Context) (string, error)
	frontendURL(ctx context.Context) (string, error)
	sendJob(ctx context.Context, kind TemplateKind, to string, values RenderValues) error
	testSMTP(ctx context.Context, to string) error
}

type generation struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Dispatcher 统一异步邮件派发器：唯一实例由 server.New 持有。
// 固定 2 worker、等待队列容量 100；Stop/PauseAndDrain/Resume 并发安全且幂等。
type Dispatcher struct {
	svc  dispatcherTransport
	logs *ActivityLog
	log  *slog.Logger
	// queue 为运行代次共用；PauseAndDrain 会清空后才能 Resume。
	queue chan *Job

	// lifecycleMu 串行化 Stop/PauseAndDrain/Resume/ResumeAfterClear，避免并发生命周期操作交错；
	// mu 只保护 generation/paused/stopped 等短临界区状态，dispatch 不持有 lifecycleMu。
	lifecycleMu sync.Mutex
	mu          sync.Mutex
	generation  *generation
	paused      bool
	stopped     bool
}

const (
	dispatcherWorkers    = 2
	dispatcherQueueSize  = 100
	dispatcherQueueLimit = dispatcherQueueSize
)

// NewDispatcher 创建并启动派发器；依赖缺失返回错误，禁止部分装配。
func NewDispatcher(svc *Service, logs *ActivityLog, lg *slog.Logger) (*Dispatcher, error) {
	if svc == nil {
		return nil, errors.New("邮件派发器缺少 SMTP 服务")
	}
	if logs == nil {
		return nil, errors.New("邮件派发器缺少活动日志")
	}
	return newDispatcherWithTransport(svc, logs, lg), nil
}

// newDispatcherWithTransport 供包内测试注入可控传输器；生产入口使用 NewDispatcher。
func newDispatcherWithTransport(svc dispatcherTransport, logs *ActivityLog, lg *slog.Logger) *Dispatcher {
	if lg == nil {
		lg = slog.Default()
	}
	d := &Dispatcher{
		svc:   svc,
		logs:  logs,
		log:   lg,
		queue: make(chan *Job, dispatcherQueueSize),
	}
	if err := d.Resume(); err != nil {
		// 新建实例的 Resume 不应失败；仅防御性记录。
		lg.Error("启动邮件派发器失败", "err", err)
	}
	return d
}

// CheckAvailability 查询 scope 可用性；供业务调用方在写 token 前统一判断。
func (d *Dispatcher) CheckAvailability(ctx context.Context, scope string) (Availability, error) {
	return d.svc.availability(ctx, scope)
}

// PasswordResetAvailable 统一密码重置邮件可用性：SMTP 完整配置且 password_reset scope 启用。
func (d *Dispatcher) PasswordResetAvailable(ctx context.Context) (bool, error) {
	avail, err := d.svc.availability(ctx, ScopePasswordReset)
	if err != nil {
		return false, err
	}
	return avail.Available, nil
}

// TestSMTP 同步 SMTP 测试：不进入业务队列，但写 sending→accepted/failed 短期日志。
func (d *Dispatcher) TestSMTP(ctx context.Context, to string) error {
	id := d.logs.BeginSending(ActivityKindSMTPTest, SourceSMTPTest, nil, to)
	if err := d.svc.testSMTP(ctx, to); err != nil {
		stage := FailureStageOf(err)
		if ctx.Err() != nil {
			stage = FailureCanceled
		}
		d.logs.MarkSendFailed(id, stage)
		return err
	}
	d.logs.MarkAccepted(id)
	return nil
}

// DispatchWelcome 欢迎邮件；source 仅沿用既有本地/OIDC 来源。
func (d *Dispatcher) DispatchWelcome(ctx context.Context, userID int64, to, source string) DispatchResult {
	kind := JobWelcomeLocal
	if source == "oidc" {
		kind = JobWelcomeOIDC
	}
	return d.dispatch(ctx, kind, source, userID, to, RenderValues{}, true)
}

// DispatchApprovalApproved 审批通过通知。
func (d *Dispatcher) DispatchApprovalApproved(ctx context.Context, userID int64, to string) DispatchResult {
	return d.dispatch(ctx, JobApprovalApproved, "approval", userID, to, RenderValues{}, true)
}

// DispatchApprovalRejected 审批拒绝通知。
func (d *Dispatcher) DispatchApprovalRejected(ctx context.Context, userID int64, to string) DispatchResult {
	return d.dispatch(ctx, JobApprovalRejected, "approval", userID, to, RenderValues{}, true)
}

// DispatchPasswordReset 密码重置邮件；source 区分公共/管理员单用户/管理员批量。
// 先通过 preflight 判定 scope/派发器状态，再与前端地址拼成绝对 URL 并随任务快照固定；
// 前端地址缺失属已知配置不可用，避免 scope 关闭时仍读取前端地址。
func (d *Dispatcher) DispatchPasswordReset(ctx context.Context, userID int64, to, resetURL, source string) DispatchResult {
	if r := d.preflight(ctx, JobPasswordReset, source, userID, to); r != nil {
		return *r
	}
	if strings.HasPrefix(resetURL, "/") {
		base, err := d.svc.frontendURL(ctx)
		if err != nil {
			return d.rejectBeforeEnqueue(JobPasswordReset, source, &userID, to)
		}
		base = strings.TrimRight(strings.TrimSpace(base), "/")
		if base == "" {
			return DispatchResult{Status: DispatchSkipped, Reason: ReasonConfigUnavailable}
		}
		resetURL = base + resetURL
	}
	return d.enqueuePrepared(JobPasswordReset, source, userID, to, RenderValues{ResetURL: resetURL})
}

func (d *Dispatcher) dispatch(ctx context.Context, kind JobKind, source string, userID int64, to string, values RenderValues, needSite bool) DispatchResult {
	if r := d.preflight(ctx, kind, source, userID, to); r != nil {
		return *r
	}
	if needSite {
		if kind == JobApprovalRejected {
			siteName, err := d.svc.siteName(ctx)
			if err != nil {
				return d.rejectBeforeEnqueue(kind, source, &userID, to)
			}
			values.SiteName = siteName
		} else {
			siteName, loginURL, err := d.svc.siteContext(ctx)
			if err != nil {
				return d.rejectBeforeEnqueue(kind, source, &userID, to)
			}
			values.SiteName = siteName
			values.LoginURL = loginURL
		}
	}
	return d.enqueuePrepared(kind, source, userID, to, values)
}

// preflight 入队前置：空收件人/派发器状态/scope 可用性按固定顺序判定。
// 返回 nil 表示可继续准备 values；返回 skipped/rejected 结果表示不得创建任务。
// 状态检查先于任何配置读取，保证暂停/停止时立即拒绝且不等待清空事务。
func (d *Dispatcher) preflight(ctx context.Context, kind JobKind, source string, userID int64, to string) *DispatchResult {
	if strings.TrimSpace(to) == "" {
		return &DispatchResult{Status: DispatchSkipped, Reason: ReasonEmptyRecipient}
	}
	d.mu.Lock()
	unavailable := d.stopped || d.paused || d.generation == nil
	d.mu.Unlock()
	if unavailable {
		res := d.rejectUnavailable(kind, source, &userID, to)
		return &res
	}
	avail, err := d.svc.availability(ctx, scopeForJob(kind))
	if err != nil {
		res := d.rejectBeforeEnqueue(kind, source, &userID, to)
		return &res
	}
	if !avail.Available {
		return &DispatchResult{Status: DispatchSkipped, Reason: avail.Reason}
	}
	return nil
}

// enqueuePrepared 在最终临界区内复核派发器状态，创建 queued 日志并非阻塞入队。
func (d *Dispatcher) enqueuePrepared(kind JobKind, source string, userID int64, to string, values RenderValues) DispatchResult {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopped || d.paused || d.generation == nil {
		return d.rejectUnavailableLocked(kind, source, &userID, to)
	}
	j := &Job{kind: kind, source: source, userID: cloneInt64Ptr(&userID), to: to, values: values}
	id := d.logs.BeginQueued(kind.templateKind(), source, &userID, to)
	j.logID = id
	select {
	case d.queue <- j:
		return DispatchResult{Status: DispatchQueued, LogID: id}
	default:
		d.logs.MarkQueuedFailed(id, FailureQueueFull)
		return DispatchResult{Status: DispatchRejected, Reason: ReasonQueueFull, LogID: id}
	}
}

// rejectBeforeEnqueue 入队前严格读取失败：按当前派发器状态归类，并创建终态失败日志。
func (d *Dispatcher) rejectBeforeEnqueue(kind JobKind, source string, userID *int64, to string) DispatchResult {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopped || d.paused || d.generation == nil {
		return d.rejectUnavailableLocked(kind, source, userID, to)
	}
	id := d.logs.RecordTerminalFailed(kind.templateKind(), source, userID, to, FailureConfig)
	return DispatchResult{Status: DispatchRejected, Reason: ReasonConfigReadFailed, LogID: id}
}

// rejectUnavailable 拒绝派发并创建 dispatcher_unavailable 终态日志。
func (d *Dispatcher) rejectUnavailable(kind JobKind, source string, userID *int64, to string) DispatchResult {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.rejectUnavailableLocked(kind, source, userID, to)
}

// rejectUnavailableLocked 在已持有 d.mu 时记录 dispatcher_unavailable 终态。
func (d *Dispatcher) rejectUnavailableLocked(kind JobKind, source string, userID *int64, to string) DispatchResult {
	id := d.logs.RecordTerminalFailed(kind.templateKind(), source, userID, to, FailureDispatcherUnavailable)
	return DispatchResult{Status: DispatchRejected, Reason: ReasonDispatcherUnavailable, LogID: id}
}

func scopeForJob(kind JobKind) string {
	switch kind {
	case JobWelcomeLocal, JobWelcomeOIDC:
		return ScopeWelcome
	case JobApprovalApproved, JobApprovalRejected:
		return ScopeApprovalNotify
	case JobPasswordReset:
		return ScopePasswordReset
	default:
		return ""
	}
}

func (d *Dispatcher) worker(gen *generation) {
	defer gen.wg.Done()
	for {
		select {
		case <-gen.ctx.Done():
			d.drainCanceled()
			return
		case j := <-d.queue:
			if gen.ctx.Err() != nil {
				d.logs.MarkQueuedFailed(j.logID, FailureCanceled)
				d.drainCanceled()
				return
			}
			d.process(gen.ctx, j)
		}
	}
}

func (d *Dispatcher) process(ctx context.Context, j *Job) {
	defer func() {
		if r := recover(); r != nil {
			d.logs.MarkSendFailed(j.logID, FailureInternal)
			// 不记录 panic 值，避免其中夹带任务敏感字段。
			d.log.Warn("邮件任务 panic", "kind", j.kind, "log_id", j.logID)
		}
	}()
	d.logs.MarkSending(j.logID)
	err := d.svc.sendJob(ctx, j.kind.templateKind(), j.to, j.values)
	if err != nil {
		stage := FailureStageOf(err)
		if ctx.Err() != nil {
			stage = FailureCanceled
		}
		d.logs.MarkSendFailed(j.logID, stage)
		d.log.Warn("邮件发送失败", "kind", j.kind, "log_id", j.logID, "stage", stage)
		return
	}
	d.logs.MarkAccepted(j.logID)
}

func (d *Dispatcher) drainCanceled() {
	for {
		select {
		case j := <-d.queue:
			d.logs.MarkQueuedFailed(j.logID, FailureCanceled)
		default:
			return
		}
	}
}

// PauseAndDrain 停止入队并取消/丢弃当前代次；返回后队列为空、worker 已退出。
func (d *Dispatcher) PauseAndDrain() error {
	d.lifecycleMu.Lock()
	defer d.lifecycleMu.Unlock()

	d.mu.Lock()
	if d.stopped {
		d.mu.Unlock()
		return ErrDispatcherStopped
	}
	gen := d.generation
	d.paused = true
	if gen != nil {
		gen.cancel()
	}
	d.mu.Unlock()

	if gen != nil {
		gen.wg.Wait()
	}

	d.mu.Lock()
	if d.generation == gen {
		d.generation = nil
	}
	d.mu.Unlock()
	return nil
}

// Resume 清空失败/正常恢复用：保留现有日志并启动新代次。Stop 后不可恢复。
func (d *Dispatcher) Resume() error {
	d.lifecycleMu.Lock()
	defer d.lifecycleMu.Unlock()

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopped {
		return ErrDispatcherStopped
	}
	if d.generation != nil {
		d.paused = false
		return nil
	}
	d.startGenerationLocked()
	return nil
}

// ResumeAfterClear 清空成功用：在同一临界区内清空 ActivityLog 并启动新代次。
// 必须等待 PauseAndDrain 完成后调用；若仍有运行代次则返回错误，禁止重复启动 worker。
func (d *Dispatcher) ResumeAfterClear() error {
	d.lifecycleMu.Lock()
	defer d.lifecycleMu.Unlock()

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopped {
		return ErrDispatcherStopped
	}
	if d.generation != nil {
		return errors.New("邮件派发器尚未完成暂停，不能清空日志并启动新代次")
	}
	d.logs.Clear()
	d.startGenerationLocked()
	return nil
}

func (d *Dispatcher) startGenerationLocked() {
	ctx, cancel := context.WithCancel(context.Background())
	gen := &generation{ctx: ctx, cancel: cancel}
	d.generation = gen
	d.paused = false
	for i := 0; i < dispatcherWorkers; i++ {
		gen.wg.Add(1)
		go d.worker(gen)
	}
}

// Stop 永久停止派发器：取消当前代次、等待任务标 failed/canceled、等待 worker 退出；幂等。
func (d *Dispatcher) Stop() {
	d.lifecycleMu.Lock()
	defer d.lifecycleMu.Unlock()

	d.mu.Lock()
	if d.stopped {
		gen := d.generation
		d.mu.Unlock()
		if gen != nil {
			gen.wg.Wait()
		}
		return
	}
	d.stopped = true
	d.paused = true
	gen := d.generation
	if gen != nil {
		gen.cancel()
	}
	d.generation = nil
	d.mu.Unlock()
	if gen != nil {
		gen.wg.Wait()
	}
}
