package providertraffic

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"vpn-sub/internal/config"
)

const (
	// collectionInterval 是正常服务下的固定采集周期。
	collectionInterval = 15 * time.Minute
	// messageCollectionFailed 是续期结果未能入库时对外的受控短文案。
	messageCollectionFailed = "服务器流量采集失败"
	// hintRotationUnsaved 是续期结果未能入库时的安全操作提示。
	hintRotationUnsaved = "续期结果未保存，可能需要重新绑定"
)

// ResultStatus 是采集结果状态；waiting 不写入结果槽，由读取时生成。
type ResultStatus string

const (
	// ResultWaiting 表示该平台尚未完成首次采集。
	ResultWaiting ResultStatus = "waiting"
	// ResultOK 表示最近一次采集成功。
	ResultOK ResultStatus = "ok"
	// ResultError 表示最近一次采集失败，旧数值不得回填。
	ResultError ResultStatus = "error"
)

// Result 是某平台最近一次采集结果；不包含实例 ID、绑定修订或凭据。
// Hint 仅供管理端展示，首页映射不得输出；revision 只用于与当前绑定比较，不序列化。
type Result struct {
	Provider  Provider
	Status    ResultStatus
	Remaining float64
	Unit      string
	SampledAt time.Time
	Message   string
	Hint      string
	revision  string
}

// 采集与手动请求的受控错误；Step 4 据此映射站内状态码。
var (
	// ErrCollectorBusy 表示同家已有一条在途请求链。
	ErrCollectorBusy = errors.New("正在采集，请稍后再试")
	// ErrCollectorUnavailable 表示采集服务未运行或已开始停止。
	ErrCollectorUnavailable = errors.New("采集服务不可用")
	// ErrCollectorStopped 表示生命周期已结束，不能重新启动。
	ErrCollectorStopped = errors.New("采集服务已停止")
	// ErrNotReady 表示当前绑定未启用或 Setup／总开关不满足条件。
	ErrNotReady = errors.New("当前绑定未启用")
	// ErrModeUnavailable 表示当前模式不允许管理设置或开始采集。
	ErrModeUnavailable = errors.New("当前模式不可用")
	// ErrDiscarded 表示本轮结果因绑定修订变化或关停而被丢弃。
	ErrDiscarded = errors.New("本轮采集结果已丢弃")
)

// triggerSource 只用于安全日志，不改变采集入口。
type triggerSource string

const (
	triggerStartup triggerSource = "startup"
	triggerCycle   triggerSource = "cycle"
	triggerConfig  triggerSource = "config"
	triggerManual  triggerSource = "manual"
)

// allProviders 是固定展示与采集顺序。
var allProviders = [...]Provider{ProviderKiwiVM, ProviderVPS}

func providerIndex(provider Provider) int {
	switch provider {
	case ProviderKiwiVM:
		return 0
	case ProviderVPS:
		return 1
	default:
		return -1
	}
}

// kiwiVMFetcher 与 vpsFetcher 是采集层对两个固定上游客户端的最小依赖；
// 生产入口使用真实客户端，包内测试可注入可阻塞的合成响应。
type kiwiVMFetcher interface {
	FetchRemaining(ctx context.Context, instanceID, apiKey string) (TrafficSample, error)
}

type vpsFetcher interface {
	RefreshToken(ctx context.Context, refreshToken string) (TokenPair, error)
	FetchRemaining(ctx context.Context, instanceID, accessToken string) (TrafficSample, error)
}

type bindingSnapshot struct {
	enabled    bool
	instanceID string
	revision   string
}

type providerState struct {
	busy    bool
	pending bool
	result  *Result
	token   cachedAccessToken
}

type chainOutcome struct {
	result Result
	err    error
}

type manualCollection struct {
	requestCtx context.Context
	revision   string
	outcome    chan chainOutcome
}

// Collector 是基础模式服务器剩余流量的独立采集服务。
// 启动首轮、固定周期、配置提交后触发和手动触发都复用同一条采集入口；
// 每家最多一条在途请求链，两个平台独立执行。
type Collector struct {
	svc  *Service
	cfg  *config.Service
	log  *slog.Logger
	kiwi kiwiVMFetcher
	vps  vpsFetcher
	now  func() time.Time

	// settingsMu 串行化本模块设置提交和调度交接。
	settingsMu sync.Mutex

	mu       sync.Mutex
	running  bool
	stopped  bool
	root     context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	stopOnce sync.Once
	state    [2]providerState

	// testCycle/testOnCycle 仅供包内测试注入可控周期与观察点；生产为零值。
	testCycle   chan time.Time
	testOnCycle func()
}

// NewCollector 构造采集服务；不自动启动，由正常服务装配在 Run 生命周期内调用 Start。
func NewCollector(svc *Service, cfg *config.Service, lg *slog.Logger) *Collector {
	return &Collector{
		svc:  svc,
		cfg:  cfg,
		log:  lg,
		kiwi: NewKiwiVMClient(lg),
		vps:  NewVPSClient(lg),
		now:  time.Now,
	}
}

// Start 启动首轮采集与固定周期循环。重复启动为空操作；停止后不可重新启动。
// ctx 应为正常服务的运行上下文，服务停止时应已取消或由 Stop 取消。
func (c *Collector) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return ErrCollectorStopped
	}
	if c.running {
		c.mu.Unlock()
		return nil
	}
	if err := ctx.Err(); err != nil {
		c.mu.Unlock()
		return err
	}
	c.running = true
	c.root, c.cancel = context.WithCancel(ctx)
	c.wg.Add(1)
	root := c.root
	c.mu.Unlock()
	c.log.Info("服务器剩余流量采集服务已启动")
	go c.loop(root)
	return nil
}

// Stop 取消运行上下文并等待循环与所有在途链结束；重复调用等待首次停止完成。
func (c *Collector) Stop() {
	c.stopOnce.Do(func() {
		c.mu.Lock()
		c.stopped = true
		c.running = false
		cancel := c.cancel
		c.mu.Unlock()

		if cancel != nil {
			cancel()
		}
		c.wg.Wait()
		c.log.Info("服务器剩余流量采集服务已停止")
	})
}

// SetEnabled 保存总开关，提交后安排符合条件的平台采集。
func (c *Collector) SetEnabled(ctx context.Context, enabled bool) (Settings, error) {
	return c.applySettingsChange(func() (settingsChange, error) { return c.svc.setEnabled(ctx, enabled) })
}

// SaveBinding 保存单家绑定，提交后按本次实际变更安排采集。
func (c *Collector) SaveBinding(ctx context.Context, in SaveBindingInput) (Settings, error) {
	return c.applySettingsChange(func() (settingsChange, error) { return c.svc.saveBinding(ctx, in) })
}

// ClearBinding 解除单家绑定，提交后清理该修订的内存状态。
func (c *Collector) ClearBinding(ctx context.Context, provider Provider, revision string) (Settings, error) {
	return c.applySettingsChange(func() (settingsChange, error) { return c.svc.clearBinding(ctx, provider, revision) })
}

func (c *Collector) applySettingsChange(save func() (settingsChange, error)) (Settings, error) {
	c.settingsMu.Lock()
	defer c.settingsMu.Unlock()
	change, err := save()
	if err != nil {
		return Settings{}, err
	}
	for _, provider := range allProviders {
		before, _ := bindingFromSettings(change.before, provider)
		current, bound := bindingFromSettings(change.after, provider)
		if before.revision != current.revision {
			c.mu.Lock()
			c.clearProviderStateLocked(providerIndex(provider), current)
			c.mu.Unlock()
		}
		if !change.after.Enabled || !bound || !current.enabled || (change.before.Enabled && before == current) {
			continue
		}
		if err := c.startChain(nil, provider, triggerConfig, nil); err != nil {
			c.log.Debug("服务器剩余流量配置触发跳过", "provider", string(provider), "reason", err.Error())
		}
	}
	return change.after, nil
}

// ManualRefresh 在调用方提交的修订仍匹配已保存绑定时，同步执行一轮采集。
// 绑定未启用、模式不允许或同家忙碌时返回受控错误；上游失败仍以 Result 返回。
func (c *Collector) ManualRefresh(ctx context.Context, provider Provider, revision string) (Result, error) {
	if providerIndex(provider) < 0 {
		return Result{}, ErrUnknownProvider
	}
	binding, err := c.resolveBinding(ctx, provider)
	if err != nil {
		return Result{}, err
	}
	if binding.revision != revision {
		return Result{}, ErrRevisionConflict
	}

	c.mu.Lock()
	root := c.root
	running := c.running
	stopped := c.stopped
	c.mu.Unlock()
	if stopped {
		return Result{}, ErrCollectorStopped
	}
	if !running || root == nil {
		return Result{}, ErrCollectorUnavailable
	}

	chainCtx, cancel := context.WithCancel(root)
	defer cancel()
	stopAfter := context.AfterFunc(ctx, cancel)
	defer stopAfter()

	manual := &manualCollection{requestCtx: ctx, revision: revision, outcome: make(chan chainOutcome, 1)}
	if err := c.startChain(chainCtx, provider, triggerManual, manual); err != nil {
		return Result{}, err
	}

	select {
	case out := <-manual.outcome:
		// 工作链已发布的结果优先交付。仅已受理链的服务关停归为丢弃。
		if out.err != nil && ctx.Err() == nil &&
			(errors.Is(out.err, ErrCollectorUnavailable) || errors.Is(out.err, root.Err())) {
			return Result{}, ErrDiscarded
		}
		return out.result, out.err
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
}

// Results 返回首页可展示的本地结果；按实时基础模式、Setup、总开关与单家开关过滤。
// 无匹配结果的已启用绑定生成 waiting；本方法从不触发上游请求。
func (c *Collector) Results(ctx context.Context) ([]Result, error) {
	configured, err := c.cfg.GetBoolStrict(ctx, config.KeyConfigured, false)
	if err != nil {
		return nil, err
	}
	if !configured {
		return []Result{}, nil
	}
	advanced, err := c.cfg.GetBoolStrict(ctx, config.KeyAdvancedMode, false)
	if err != nil {
		return nil, err
	}
	if advanced {
		return []Result{}, nil
	}
	settings, err := c.svc.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	if !settings.Enabled {
		return []Result{}, nil
	}

	results := make([]Result, 0, 2)
	for _, provider := range allProviders {
		binding, bound := bindingFromSettings(settings, provider)
		if !bound || !binding.enabled {
			continue
		}
		result := c.ResultForRevision(provider, binding.revision)
		if result == nil {
			results = append(results, Result{Provider: provider, Status: ResultWaiting})
			continue
		}
		results = append(results, *result)
	}
	return results, nil
}

// ResultForRevision 返回与调用方已读取的绑定修订匹配的内存结果。
// 未采集或修订不匹配时返回 nil，且不读取数据库或发起上游请求。
func (c *Collector) ResultForRevision(provider Provider, revision string) *Result {
	idx := providerIndex(provider)
	if idx < 0 || revision == "" {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	stored := c.state[idx].result
	if stored == nil || stored.revision != revision {
		return nil
	}
	result := *stored
	return &result
}

func (c *Collector) loop(ctx context.Context) {
	defer c.wg.Done()
	c.runCycle(triggerStartup)

	var ticker *time.Ticker
	var ticks <-chan time.Time
	if c.testCycle != nil {
		ticks = c.testCycle
	} else {
		ticker = time.NewTicker(collectionInterval)
		ticks = ticker.C
	}
	if ticker != nil {
		defer ticker.Stop()
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			c.runCycle(triggerCycle)
		}
	}
}

func (c *Collector) runCycle(source triggerSource) {
	for _, provider := range allProviders {
		if err := c.startChain(nil, provider, source, nil); err != nil {
			c.log.Debug("服务器剩余流量触发跳过", "provider", string(provider), "trigger", string(source), "reason", err.Error())
		}
	}
	if c.testOnCycle != nil {
		c.testOnCycle()
	}
}

// startChain 尝试为单家启动一条在途链。
// 配置触发遇忙只设置一次待执行信号；手动与定时触发遇忙分别返回错误或跳过。
func (c *Collector) startChain(chainCtx context.Context, provider Provider, source triggerSource, manual *manualCollection) error {
	idx := providerIndex(provider)
	if idx < 0 {
		return ErrUnknownProvider
	}

	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return ErrCollectorStopped
	}
	if !c.running || c.root == nil || c.root.Err() != nil {
		c.mu.Unlock()
		return ErrCollectorUnavailable
	}
	state := &c.state[idx]
	if state.busy {
		if source == triggerConfig {
			state.pending = true
			c.mu.Unlock()
			return nil
		}
		c.mu.Unlock()
		return ErrCollectorBusy
	}
	if chainCtx == nil {
		chainCtx = c.root
	}
	state.busy = true
	c.wg.Add(1)
	c.mu.Unlock()

	go c.runChain(chainCtx, provider, source, manual)
	return nil
}

func (c *Collector) runChain(ctx context.Context, provider Provider, source triggerSource, manual *manualCollection) {
	defer c.wg.Done()
	finished := false
	defer func() {
		if !finished {
			c.finishChain(provider)
		}
	}()

	result, err := c.collect(ctx, provider, source, manual)
	if err != nil && source != triggerManual && !expectedChainError(err) {
		c.log.Warn("服务器流量采集失败", "provider", string(provider), "trigger", string(source), "err", err)
	}
	c.finishChain(provider)
	finished = true
	if manual != nil {
		manual.outcome <- chainOutcome{result: result, err: err}
	}
}

func (c *Collector) finishChain(provider Provider) {
	idx := providerIndex(provider)
	if idx < 0 {
		return
	}

	c.mu.Lock()
	state := &c.state[idx]
	state.busy = false
	launch := false
	var root context.Context
	if state.pending && c.running && !c.stopped {
		state.pending = false
		state.busy = true
		c.wg.Add(1)
		launch = true
		root = c.root
	}
	c.mu.Unlock()

	if launch {
		go c.runChain(root, provider, triggerConfig, nil)
	}
}

func (c *Collector) collect(ctx context.Context, provider Provider, source triggerSource, manual *manualCollection) (Result, error) {
	binding, err := c.resolveBinding(ctx, provider)
	if err != nil {
		if manual != nil {
			return Result{}, discardOnIneligible(err)
		}
		return Result{}, err
	}
	var requestCtx context.Context
	if manual != nil {
		requestCtx = manual.requestCtx
		if binding.revision != manual.revision {
			return Result{}, ErrDiscarded
		}
	}
	if err := chainContextError(ctx, requestCtx); err != nil {
		return Result{}, err
	}

	c.log.Debug("服务器流量采集开始", "provider", string(provider), "trigger", string(source))

	instanceID, revision, credential, err := c.svc.LoadCredential(ctx, provider)
	if err != nil {
		if ctxErr := chainContextError(ctx, requestCtx); ctxErr != nil {
			return Result{}, ctxErr
		}
		if errors.Is(err, ErrNotBound) {
			return Result{}, ErrDiscarded
		}
		return Result{}, err
	}
	if revision != binding.revision || instanceID != binding.instanceID {
		return Result{}, ErrDiscarded
	}

	switch provider {
	case ProviderKiwiVM:
		return c.collectKiwiVM(ctx, requestCtx, binding, credential)
	case ProviderVPS:
		return c.collectVPS(ctx, requestCtx, binding, credential)
	default:
		return Result{}, ErrUnknownProvider
	}
}

func (c *Collector) collectKiwiVM(ctx, requestCtx context.Context, binding bindingSnapshot, apiKey string) (Result, error) {
	if err := c.checkEligible(ctx, ProviderKiwiVM, binding.revision); err != nil {
		return Result{}, err
	}
	if err := chainContextError(ctx, requestCtx); err != nil {
		return Result{}, err
	}
	sample, err := c.kiwi.FetchRemaining(ctx, binding.instanceID, apiKey)
	if err != nil {
		if ctxErr := chainContextError(ctx, requestCtx); ctxErr != nil {
			return Result{}, ctxErr
		}
		result, ok := upstreamErrorResult(ProviderKiwiVM, binding.revision, err)
		if !ok {
			return Result{}, err
		}
		if err := c.publish(ctx, requestCtx, ProviderKiwiVM, binding.revision, result); err != nil {
			return Result{}, err
		}
		return result, nil
	}
	if ctxErr := chainContextError(ctx, requestCtx); ctxErr != nil {
		return Result{}, ctxErr
	}
	result := Result{
		Provider:  ProviderKiwiVM,
		Status:    ResultOK,
		Remaining: sample.Remaining,
		Unit:      sample.Unit,
		SampledAt: c.now().UTC(),
		revision:  binding.revision,
	}
	if err := c.publish(ctx, requestCtx, ProviderKiwiVM, binding.revision, result); err != nil {
		return Result{}, err
	}
	return result, nil
}

func (c *Collector) collectVPS(ctx, requestCtx context.Context, binding bindingSnapshot, refreshToken string) (Result, error) {
	if err := c.checkEligible(ctx, ProviderVPS, binding.revision); err != nil {
		return Result{}, err
	}
	// 先完成可能阻塞的资格复核，再以当前时间判断缓存是否可复用。
	accessToken, cached := c.cachedToken(binding.revision)
	if !cached {
		if err := chainContextError(ctx, requestCtx); err != nil {
			return Result{}, err
		}
		pair, err := c.vps.RefreshToken(ctx, refreshToken)
		if err != nil {
			if ctxErr := chainContextError(ctx, requestCtx); ctxErr != nil {
				return Result{}, ctxErr
			}
			result, ok := upstreamErrorResult(ProviderVPS, binding.revision, err)
			if !ok {
				return Result{}, err
			}
			if err := c.publish(ctx, requestCtx, ProviderVPS, binding.revision, result); err != nil {
				return Result{}, err
			}
			return result, nil
		}

		updated, err := c.updateRotatedToken(ctx, binding.revision, pair.Refresh)
		if err != nil {
			c.log.Warn("服务器流量续期结果未能入库", "provider", string(ProviderVPS), "err", err)
			if ctxErr := chainContextError(ctx, requestCtx); ctxErr != nil {
				return Result{}, ctxErr
			}
			result := Result{
				Provider: ProviderVPS,
				Status:   ResultError,
				Message:  messageCollectionFailed,
				Hint:     hintRotationUnsaved,
				revision: binding.revision,
			}
			if err := c.publish(ctx, requestCtx, ProviderVPS, binding.revision, result); err != nil {
				return Result{}, err
			}
			return result, nil
		}
		if !updated {
			return Result{}, ErrDiscarded
		}

		// 已提交新 Refresh Token；此时若请求已取消、功能关闭、切换模式或改绑，结束本轮，不再查询资源。
		if err := chainContextError(ctx, requestCtx); err != nil {
			return Result{}, err
		}
		if err := c.checkEligible(ctx, ProviderVPS, binding.revision); err != nil {
			return Result{}, err
		}
		c.cacheToken(binding.revision, pair.Token)
		accessToken = pair.Token
	}

	if err := chainContextError(ctx, requestCtx); err != nil {
		return Result{}, err
	}
	sample, err := c.vps.FetchRemaining(ctx, binding.instanceID, accessToken)
	if err != nil {
		var upstream *UpstreamError
		if errors.As(err, &upstream) && upstream.Category == UpstreamAccessTokenRejected {
			c.clearToken(binding.revision)
		}
		if ctxErr := chainContextError(ctx, requestCtx); ctxErr != nil {
			return Result{}, ctxErr
		}
		result, ok := upstreamErrorResult(ProviderVPS, binding.revision, err)
		if !ok {
			return Result{}, err
		}
		if err := c.publish(ctx, requestCtx, ProviderVPS, binding.revision, result); err != nil {
			return Result{}, err
		}
		return result, nil
	}
	if ctxErr := chainContextError(ctx, requestCtx); ctxErr != nil {
		return Result{}, ctxErr
	}
	result := Result{
		Provider:  ProviderVPS,
		Status:    ResultOK,
		Remaining: sample.Remaining,
		Unit:      sample.Unit,
		SampledAt: c.now().UTC(),
		revision:  binding.revision,
	}
	if err := c.publish(ctx, requestCtx, ProviderVPS, binding.revision, result); err != nil {
		return Result{}, err
	}
	return result, nil
}

func (c *Collector) updateRotatedToken(ctx context.Context, revision, refresh string) (bool, error) {
	commitCtx, cancel := rotationCommitContext(ctx)
	defer cancel()
	return c.svc.UpdateCredentialByRevision(commitCtx, ProviderVPS, revision, refresh)
}

func (c *Collector) cachedToken(revision string) (string, bool) {
	idx := providerIndex(ProviderVPS)
	c.mu.Lock()
	defer c.mu.Unlock()
	cached := c.state[idx].token
	if cached.revision == "" {
		return "", false
	}
	if cached.revision != revision || !c.now().Before(cached.expiresAt) {
		c.state[idx].token = cachedAccessToken{}
		return "", false
	}
	return cached.token, true
}

func (c *Collector) cacheToken(revision, token string) {
	expiresAt, ok := accessTokenExpiry(token)
	if !ok {
		return
	}
	idx := providerIndex(ProviderVPS)
	c.mu.Lock()
	defer c.mu.Unlock()
	if existing := c.state[idx].token; existing.revision != "" && existing.revision != revision {
		return
	}
	c.state[idx].token = cachedAccessToken{revision: revision, token: token, expiresAt: expiresAt}
}

func (c *Collector) clearToken(revision string) {
	idx := providerIndex(ProviderVPS)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state[idx].token.revision == revision {
		c.state[idx].token = cachedAccessToken{}
	}
}

// publish 先取 SQLite 写事务，再取结果锁。事务结束前读取者看不到暂存值。
func (c *Collector) publish(ctx, requestCtx context.Context, provider Provider, revision string, result Result) error {
	idx := providerIndex(provider)
	var previous *Result
	var locked bool
	defer func() {
		if locked {
			c.mu.Unlock()
		}
	}()
	err := c.svc.withPublicationSettings(ctx, func(settings Settings) error {
		binding, bound := bindingFromSettings(settings, provider)
		if !settings.Enabled || !bound || !binding.enabled || binding.revision != revision {
			return ErrDiscarded
		}
		c.mu.Lock()
		locked = true
		previous = c.state[idx].result
		if err := chainContextError(ctx, requestCtx); err != nil {
			return err
		}
		if c.stopped || !c.running {
			return ErrCollectorUnavailable
		}
		result.revision = revision
		c.state[idx].result = &result
		return nil
	})
	if err != nil && locked {
		c.state[idx].result = previous
	}
	return err
}

func (c *Collector) checkEligible(ctx context.Context, provider Provider, revision string) error {
	binding, err := c.resolveBinding(ctx, provider)
	if err != nil {
		return discardOnIneligible(err)
	}
	if binding.revision != revision {
		return ErrDiscarded
	}
	return nil
}

func (c *Collector) resolveBinding(ctx context.Context, provider Provider) (bindingSnapshot, error) {
	if providerIndex(provider) < 0 {
		return bindingSnapshot{}, ErrUnknownProvider
	}
	if !c.isRunning() {
		return bindingSnapshot{}, ErrCollectorUnavailable
	}
	configured, err := c.cfg.GetBoolStrict(ctx, config.KeyConfigured, false)
	if err != nil {
		return bindingSnapshot{}, err
	}
	if !configured {
		return bindingSnapshot{}, ErrNotReady
	}
	advanced, err := c.cfg.GetBoolStrict(ctx, config.KeyAdvancedMode, false)
	if err != nil {
		return bindingSnapshot{}, err
	}
	if advanced {
		return bindingSnapshot{}, ErrModeUnavailable
	}
	settings, err := c.svc.GetSettings(ctx)
	if err != nil {
		return bindingSnapshot{}, err
	}
	if !settings.Enabled {
		return bindingSnapshot{}, ErrNotReady
	}
	binding, bound := bindingFromSettings(settings, provider)
	if !bound || !binding.enabled {
		return bindingSnapshot{}, ErrNotReady
	}
	return binding, nil
}

func (c *Collector) isRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running && !c.stopped
}

func discardOnIneligible(err error) error {
	if errors.Is(err, ErrNotReady) || errors.Is(err, ErrModeUnavailable) {
		return ErrDiscarded
	}
	return err
}

func (c *Collector) clearProviderStateLocked(idx int, current bindingSnapshot) {
	state := &c.state[idx]
	if state.result != nil && state.result.revision != current.revision {
		state.result = nil
	}
	if state.token.revision != "" && state.token.revision != current.revision {
		state.token = cachedAccessToken{}
	}
}

func bindingFromSettings(settings Settings, provider Provider) (bindingSnapshot, bool) {
	idx := providerIndex(provider)
	if idx < 0 || idx >= len(settings.Providers) {
		return bindingSnapshot{}, false
	}
	binding := settings.Providers[idx]
	return bindingSnapshot{
			enabled:    binding.Enabled,
			instanceID: binding.InstanceID,
			revision:   binding.Revision,
		},
		binding.InstanceID != "" && binding.Revision != ""
}

func upstreamErrorResult(provider Provider, revision string, err error) (Result, bool) {
	var upstream *UpstreamError
	if !errors.As(err, &upstream) {
		return Result{}, false
	}
	return Result{
		Provider: provider,
		Status:   ResultError,
		Message:  upstream.Category.Message(),
		Hint:     hintFor(upstream.Category),
		revision: revision,
	}, true
}

func hintFor(category UpstreamErrorCategory) string {
	switch category {
	case UpstreamUnavailable:
		return "请核对部署出网"
	case UpstreamRateLimited:
		return "请等待下次周期或稍后手动采集"
	case UpstreamMultiplierCheck:
		return "请核对面板与官方口径"
	case UpstreamInvalidData:
		return "请核对服务商响应"
	case UpstreamCredentialRejected:
		return "请重新填写对应凭据"
	case UpstreamAccessTokenRejected:
		return "已作废 Access Token，下轮将重新续期"
	default:
		return "请稍后重试"
	}
}

// chainContextError 同时检查采集链上下文与手动请求上下文。
// 调用方取消是同步可见的，不能只依赖 AfterFunc 对子上下文的异步传播。
func chainContextError(ctx, requestCtx context.Context) error {
	if requestCtx != nil {
		if err := requestCtx.Err(); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func expectedChainError(err error) bool {
	return errors.Is(err, ErrNotReady) ||
		errors.Is(err, ErrModeUnavailable) ||
		errors.Is(err, ErrDiscarded) ||
		errors.Is(err, ErrCollectorUnavailable) ||
		errors.Is(err, ErrCollectorStopped) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}
