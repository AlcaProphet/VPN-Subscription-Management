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
	HostID        string
	Name          string
	HalfRemaining bool
	Provider      Provider
	Status        ResultStatus
	Remaining     float64
	Unit          string
	SampledAt     time.Time
	Message       string
	Hint          string
	revision      string
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

type providerState struct {
	busy    bool
	pending bool
	result  *Result
	removed bool
}

type chainOutcome struct {
	result Result
	err    error
}

type manualCollection struct {
	requestCtx context.Context
	revision   collectionRevision
	outcome    chan chainOutcome
}

// collectionRevision 固定绑定身份；手动请求还固定发起时看到的编辑修订。
type collectionRevision struct {
	binding string
	edit    string
}

func (r collectionRevision) matches(binding ProviderBinding) bool {
	return binding.Revision == r.binding && (r.edit == "" || binding.EditRevision == r.edit)
}

// Collector 是基础模式服务器剩余流量的独立采集服务。
// 启动首轮、固定周期、配置提交后触发和手动触发都复用同一条采集入口；
// 每台主机最多一条在途请求链，两个平台各限两条并行请求链。
type Collector struct {
	svc  *Service
	cfg  *config.Service
	log  *slog.Logger
	kiwi kiwiVMFetcher
	vps  vpsFetcher
	now  func() time.Time

	// settingsMu 串行化本模块设置提交和调度交接。
	settingsMu sync.Mutex

	mu        sync.Mutex
	running   bool
	stopped   bool
	root      context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	stopOnce  sync.Once
	state     map[hostKey]*providerState
	token     cachedAccessToken
	tokenGate chan struct{}
	slots     [2]chan struct{}

	// testCycle/testOnCycle 仅供包内测试注入可控周期与观察点；生产为零值。
	testCycle   chan time.Time
	testOnCycle func()
}

// NewCollector 构造采集服务；不自动启动，由正常服务装配在 Run 生命周期内调用 Start。
func NewCollector(svc *Service, cfg *config.Service, lg *slog.Logger) *Collector {
	return &Collector{
		svc:       svc,
		cfg:       cfg,
		log:       lg,
		kiwi:      NewKiwiVMClient(lg),
		vps:       NewVPSClient(lg),
		now:       time.Now,
		state:     make(map[hostKey]*providerState),
		tokenGate: make(chan struct{}, 1),
		slots:     [2]chan struct{}{make(chan struct{}, 2), make(chan struct{}, 2)},
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
	return c.applySettingsChange(func() (settingsChange, error) {
		return c.svc.clearHost(ctx, legacyKey(provider), revision, true)
	})
}

// SaveHost 提交独立主机设置，并安排必要的采集。
func (c *Collector) SaveHost(ctx context.Context, in SaveHostInput) (Settings, error) {
	return c.applySettingsChange(func() (settingsChange, error) { return c.svc.saveHost(ctx, in, false) })
}

// ClearHost 解除指定主机；V.PS 的账号凭据由账号设置管理。
func (c *Collector) ClearHost(ctx context.Context, provider Provider, id, revision string) (Settings, error) {
	return c.applySettingsChange(func() (settingsChange, error) {
		return c.svc.clearHost(ctx, hostKey{provider: provider, id: id}, revision, false)
	})
}

// SaveVPSAccount 保存共享凭据。空凭据表示解除已无主机的账号。
func (c *Collector) SaveVPSAccount(ctx context.Context, revision, credential string) (Settings, error) {
	return c.applySettingsChange(func() (settingsChange, error) {
		return c.svc.saveVPSAccount(ctx, revision, credential)
	})
}

func (c *Collector) applySettingsChange(save func() (settingsChange, error)) (Settings, error) {
	c.settingsMu.Lock()
	defer c.settingsMu.Unlock()
	change, err := save()
	if err != nil {
		return Settings{}, err
	}
	c.syncStates(change.after)
	if !change.after.Enabled {
		return change.after, nil
	}
	before := hostBindings(change.before)
	for _, current := range change.after.Hosts {
		previous := before[current.key()]
		if current.Enabled && (!change.before.Enabled || !previous.Enabled || previous.Revision != current.Revision) {
			c.trigger(current.key(), triggerConfig)
		}
	}
	return change.after, nil
}

func hostBindings(settings Settings) map[hostKey]ProviderBinding {
	bindings := make(map[hostKey]ProviderBinding, len(settings.Hosts))
	for _, binding := range settings.Hosts {
		bindings[binding.key()] = binding
	}
	return bindings
}

func (c *Collector) syncStates(settings Settings) {
	bindings := hostBindings(settings)
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, state := range c.state {
		binding, exists := bindings[key]
		if !exists {
			state.pending = false
			state.removed = true
			if !state.busy {
				delete(c.state, key)
			}
		} else {
			state.removed = false
		}
		if state.result != nil && state.result.revision != binding.Revision {
			state.result = nil
		}
	}
	if c.token.revision != settings.VPSRevision {
		c.token = cachedAccessToken{}
	}
}

// ManualRefresh 对旧版平台绑定按编辑修订同步采集，上游失败仍通过 Result 返回。
func (c *Collector) ManualRefresh(ctx context.Context, provider Provider, revision string) (Result, error) {
	return c.manualRefresh(ctx, legacyKey(provider), revision)
}

// RefreshHost 采集指定主机，并校验页面提交的主机编辑修订。
func (c *Collector) RefreshHost(ctx context.Context, provider Provider, id, revision string) (Result, error) {
	return c.manualRefresh(ctx, hostKey{provider: provider, id: id}, revision)
}

func (c *Collector) manualRefresh(ctx context.Context, key hostKey, revision string) (Result, error) {
	binding, _, err := c.resolveBinding(ctx, key)
	if err != nil {
		return Result{}, err
	}
	if revision == "" || binding.EditRevision != revision {
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

	manual := &manualCollection{
		requestCtx: ctx,
		revision:   collectionRevision{binding: binding.Revision, edit: revision},
		outcome:    make(chan chainOutcome, 1),
	}
	if err := c.startHostChain(chainCtx, key, triggerManual, manual); err != nil {
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

	results := make([]Result, 0, len(settings.Hosts))
	for _, binding := range settings.Hosts {
		if !binding.Enabled {
			continue
		}
		result := c.ResultForBinding(binding)
		if result == nil {
			results = append(results, projectResult(binding, Result{Status: ResultWaiting}))
			continue
		}
		results = append(results, *result)
	}
	return results, nil
}

// ResultForRevision 返回旧版绑定的最近结果，供兼容调用方读取。
func (c *Collector) ResultForRevision(provider Provider, revision string) *Result {
	return c.resultForKey(legacyKey(provider), revision)
}

// ResultForBinding 以当前主机设置显示原始样本的余量。
func (c *Collector) ResultForBinding(binding ProviderBinding) *Result {
	result := c.resultForKey(binding.key(), binding.Revision)
	if result == nil {
		return nil
	}
	display := projectResult(binding, *result)
	return &display
}

func (c *Collector) resultForKey(key hostKey, revision string) *Result {
	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.state[key]
	if state == nil || state.result == nil || state.result.revision != revision {
		return nil
	}
	result := *state.result
	return &result
}

func projectResult(binding ProviderBinding, result Result) Result {
	result.Provider = binding.Provider
	result.HostID = binding.ID
	result.Name = binding.Name
	result.HalfRemaining = binding.HalfRemaining
	if result.Status == ResultOK && binding.HalfRemaining {
		result.Remaining /= 2
	}
	return result
}

func (c *Collector) loop(ctx context.Context) {
	defer c.wg.Done()
	c.runCycle(ctx, triggerStartup)

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
			c.runCycle(ctx, triggerCycle)
		}
	}
}

func (c *Collector) runCycle(ctx context.Context, source triggerSource) {
	c.settingsMu.Lock()
	settings, err := c.svc.GetSettings(ctx)
	if err != nil {
		c.log.Warn("读取流量采集设置失败", "err", err)
	} else {
		c.syncStates(settings)
		if settings.Enabled {
			for _, binding := range settings.Hosts {
				if binding.Enabled {
					c.trigger(binding.key(), source)
				}
			}
		}
	}
	c.settingsMu.Unlock()
	if c.testOnCycle != nil {
		c.testOnCycle()
	}
}

func (c *Collector) trigger(key hostKey, source triggerSource) {
	if err := c.startHostChain(nil, key, source, nil); err != nil {
		c.log.Debug("服务器剩余流量触发跳过", "provider", key.provider, "trigger", string(source), "reason", err.Error())
	}
}

func (c *Collector) startHostChain(chainCtx context.Context, key hostKey, source triggerSource, manual *manualCollection) error {

	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return ErrCollectorStopped
	}
	if !c.running || c.root == nil || c.root.Err() != nil {
		c.mu.Unlock()
		return ErrCollectorUnavailable
	}
	state := c.state[key]
	if state == nil {
		state = &providerState{}
		c.state[key] = state
	}
	if state.removed {
		c.mu.Unlock()
		return ErrNotReady
	}
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

	go c.runChain(chainCtx, key, source, manual)
	return nil
}

func (c *Collector) runChain(ctx context.Context, key hostKey, source triggerSource, manual *manualCollection) {
	defer c.wg.Done()
	result, err := c.collect(ctx, key, manual)
	if err != nil && source != triggerManual && !expectedChainError(err) {
		c.log.Warn("服务器流量采集失败", "provider", key.provider, "trigger", string(source), "err", err)
	}
	c.finishChain(key)
	if manual != nil {
		manual.outcome <- chainOutcome{result: result, err: err}
	}
}

func (c *Collector) finishChain(key hostKey) {
	c.mu.Lock()
	state := c.state[key]
	state.busy = false
	if state.removed {
		delete(c.state, key)
	}
	var root context.Context
	if state.pending && !state.removed && c.running && !c.stopped {
		state.pending = false
		state.busy = true
		c.wg.Add(1)
		root = c.root
	}
	c.mu.Unlock()

	if root != nil {
		go c.runChain(root, key, triggerConfig, nil)
	}
}

func (c *Collector) collect(ctx context.Context, key hostKey, manual *manualCollection) (Result, error) {
	slots := c.slots[providerIndex(key.provider)]
	select {
	case slots <- struct{}{}:
		defer func() { <-slots }()
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
	binding, accountRevision, err := c.resolveBinding(ctx, key)
	if err != nil {
		if manual != nil {
			return Result{}, discardOnIneligible(err)
		}
		return Result{}, err
	}
	revision := collectionRevision{binding: binding.Revision}
	var requestCtx context.Context
	if manual != nil {
		requestCtx = manual.requestCtx
		revision = manual.revision
		if !revision.matches(binding) {
			return Result{}, ErrDiscarded
		}
	}
	if err := chainContextError(ctx, requestCtx); err != nil {
		return Result{}, err
	}

	var sample TrafficSample
	if key.provider == ProviderKiwiVM {
		var apiKey string
		apiKey, err = c.svc.loadCredential(ctx, key, binding.Revision)
		if err != nil {
			return Result{}, discardOnIneligible(err)
		}
		if err := c.checkEligible(ctx, key, revision); err != nil {
			return Result{}, err
		}
		if err := chainContextError(ctx, requestCtx); err != nil {
			return Result{}, err
		}
		sample, err = c.kiwi.FetchRemaining(ctx, binding.InstanceID, apiKey)
	} else {
		var accessToken string
		accessToken, err = c.vpsAccess(ctx, requestCtx, key, revision, accountRevision)
		if err == nil {
			if err := c.checkEligible(ctx, key, revision); err != nil {
				return Result{}, err
			}
			if err := chainContextError(ctx, requestCtx); err != nil {
				return Result{}, err
			}
			sample, err = c.vps.FetchRemaining(ctx, binding.InstanceID, accessToken)
			var upstream *UpstreamError
			if errors.As(err, &upstream) && upstream.Category == UpstreamAccessTokenRejected {
				c.clearToken(accountRevision, accessToken)
			}
		}
	}
	if ctxErr := chainContextError(ctx, requestCtx); ctxErr != nil {
		return Result{}, ctxErr
	}

	var result Result
	switch {
	case err == nil:
		result = Result{Status: ResultOK, Remaining: sample.Remaining, Unit: sample.Unit, SampledAt: c.now().UTC()}
	case errors.Is(err, errRotationUnsaved):
		result = Result{Status: ResultError, Message: messageCollectionFailed, Hint: hintRotationUnsaved}
	default:
		var ok bool
		result, ok = upstreamErrorResult(key.provider, binding.Revision, err)
		if !ok {
			return Result{}, discardOnIneligible(err)
		}
	}
	return c.publishHost(ctx, requestCtx, key, revision, result)
}

var errRotationUnsaved = errors.New("V.PS 续期结果未保存")

// vpsAccess 串行化账号的令牌读取、续期和落库；实例资源查询在账号锁外进行。
func (c *Collector) vpsAccess(ctx, requestCtx context.Context, key hostKey, revision collectionRevision, accountRevision string) (string, error) {
	select {
	case c.tokenGate <- struct{}{}:
		defer func() { <-c.tokenGate }()
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if err := c.checkEligible(ctx, key, revision); err != nil {
		return "", err
	}
	if err := chainContextError(ctx, requestCtx); err != nil {
		return "", err
	}
	if access, ok := c.cachedToken(accountRevision); ok {
		return access, nil
	}
	refreshToken, err := c.svc.loadCredential(ctx, key, accountRevision)
	if err != nil {
		return "", err
	}
	if err := c.checkEligible(ctx, key, revision); err != nil {
		return "", err
	}
	if err := chainContextError(ctx, requestCtx); err != nil {
		return "", err
	}
	pair, err := c.vps.RefreshToken(ctx, refreshToken)
	if err != nil {
		return "", err
	}
	updated, err := c.updateRotatedToken(ctx, accountRevision, pair.Refresh)
	if err != nil {
		c.log.Warn("服务器流量续期结果未能入库", "provider", ProviderVPS, "err", err)
		return "", errRotationUnsaved
	}
	if !updated {
		return "", ErrDiscarded
	}
	// 续期结果归账号所有；调用方仍需复核主机资格和取消状态后才能查询资源。
	c.cacheToken(accountRevision, pair.Token)
	return pair.Token, nil
}

func (c *Collector) updateRotatedToken(ctx context.Context, revision, refresh string) (bool, error) {
	commitCtx, cancel := rotationCommitContext(ctx)
	defer cancel()
	return c.svc.UpdateCredentialByRevision(commitCtx, ProviderVPS, revision, refresh)
}

func (c *Collector) cachedToken(revision string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cached := c.token
	if cached.revision == "" {
		return "", false
	}
	if cached.revision != revision || !c.now().Before(cached.expiresAt) {
		c.token = cachedAccessToken{}
		return "", false
	}
	return cached.token, true
}

func (c *Collector) cacheToken(revision, token string) {
	expiresAt, ok := accessTokenExpiry(token)
	if !ok {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if existing := c.token; existing.revision != "" && existing.revision != revision {
		return
	}
	c.token = cachedAccessToken{revision: revision, token: token, expiresAt: expiresAt}
}

func (c *Collector) clearToken(revision, token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token.revision == revision && c.token.token == token {
		c.token = cachedAccessToken{}
	}
}

// publishHost 在提交期间持有结果锁，事务失败时恢复先前样本。
func (c *Collector) publishHost(ctx, requestCtx context.Context, key hostKey, revision collectionRevision, result Result) (Result, error) {
	var previous *Result
	var state *providerState
	var display Result
	defer func() {
		if state != nil {
			c.mu.Unlock()
		}
	}()
	err := c.svc.withPublicationSettings(ctx, func(settings Settings) error {
		binding, found := findHost(settings, key)
		if !settings.Enabled || !found || !binding.Enabled || !revision.matches(binding) {
			return ErrDiscarded
		}
		if err := chainContextError(ctx, requestCtx); err != nil {
			return err
		}
		c.mu.Lock()
		if c.stopped || !c.running {
			c.mu.Unlock()
			return ErrCollectorUnavailable
		}
		if err := chainContextError(ctx, requestCtx); err != nil {
			c.mu.Unlock()
			return err
		}
		state = c.state[key]
		if state == nil {
			state = &providerState{}
			c.state[key] = state
		}
		previous = state.result
		result.revision = revision.binding
		state.result = &result
		display = projectResult(binding, result)
		return nil
	})
	if err != nil {
		if state != nil {
			state.result = previous
		}
		return Result{}, err
	}
	return display, nil
}

func (c *Collector) checkEligible(ctx context.Context, key hostKey, revision collectionRevision) error {
	binding, _, err := c.resolveBinding(ctx, key)
	if err != nil {
		return discardOnIneligible(err)
	}
	if !revision.matches(binding) {
		return ErrDiscarded
	}
	return nil
}

func (c *Collector) resolveBinding(ctx context.Context, key hostKey) (ProviderBinding, string, error) {
	if providerIndex(key.provider) < 0 {
		return ProviderBinding{}, "", ErrUnknownProvider
	}
	if !c.isRunning() {
		return ProviderBinding{}, "", ErrCollectorUnavailable
	}
	configured, err := c.cfg.GetBoolStrict(ctx, config.KeyConfigured, false)
	if err != nil {
		return ProviderBinding{}, "", err
	}
	if !configured {
		return ProviderBinding{}, "", ErrNotReady
	}
	advanced, err := c.cfg.GetBoolStrict(ctx, config.KeyAdvancedMode, false)
	if err != nil {
		return ProviderBinding{}, "", err
	}
	if advanced {
		return ProviderBinding{}, "", ErrModeUnavailable
	}
	settings, err := c.svc.GetSettings(ctx)
	if err != nil {
		return ProviderBinding{}, "", err
	}
	binding, found := findHost(settings, key)
	if !settings.Enabled || !found || !binding.Enabled || !binding.CredentialConfigured {
		return ProviderBinding{}, "", ErrNotReady
	}
	return binding, settings.VPSRevision, nil
}

func findHost(settings Settings, key hostKey) (ProviderBinding, bool) {
	for _, binding := range settings.Hosts {
		if binding.key() == key {
			return binding, true
		}
	}
	return ProviderBinding{}, false
}

func (c *Collector) isRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running && !c.stopped
}

func discardOnIneligible(err error) error {
	if errors.Is(err, ErrNotReady) || errors.Is(err, ErrModeUnavailable) || errors.Is(err, ErrNotBound) {
		return ErrDiscarded
	}
	return err
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
