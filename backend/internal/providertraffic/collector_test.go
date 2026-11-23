// 本文件所有实例 ID、凭据与 JWT 均为合成样本，不包含真实凭据。
package providertraffic

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"vpn-sub/internal/config"
	"vpn-sub/internal/store"
)

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type collectorHarness struct {
	st     *store.Store
	cfg    *config.Service
	svc    *Service
	c      *Collector
	log    *slog.Logger
	logBuf *bytes.Buffer
	clock  *testClock
	cycles chan struct{}
}

func newCollectorHarness(t *testing.T) *collectorHarness {
	t.Helper()
	st, cfg, svc := newTestTraffic(t)
	lg, buf := newTestLogger()
	clock := &testClock{now: time.Date(2026, 11, 24, 2, 0, 0, 0, time.UTC)}
	c := NewCollector(svc, cfg, lg)
	c.now = clock.Now
	c.testCycle = make(chan time.Time, 1)
	cycles := make(chan struct{}, 16)
	c.testOnCycle = func() {
		select {
		case cycles <- struct{}{}:
		default:
		}
	}
	h := &collectorHarness{st: st, cfg: cfg, svc: svc, c: c, log: lg, logBuf: buf, clock: clock, cycles: cycles}
	h.c.kiwi = &fakeKiwiVM{fetch: func(context.Context, string, string) (TrafficSample, error) {
		return TrafficSample{}, errors.New("未设置 KiwiVM 测试响应")
	}}
	h.c.vps = &fakeVPS{
		refresh: func(context.Context, string) (TokenPair, error) {
			return TokenPair{}, errors.New("未设置 V.P.S 续期测试响应")
		},
		fetch: func(context.Context, string, string) (TrafficSample, error) {
			return TrafficSample{}, errors.New("未设置 V.P.S 资源测试响应")
		},
	}
	return h
}

func (h *collectorHarness) start(t *testing.T) {
	t.Helper()
	if err := h.c.Start(context.Background()); err != nil {
		t.Fatalf("启动采集服务失败: %v", err)
	}
	t.Cleanup(h.c.Stop)
}

func (h *collectorHarness) waitCycle(t *testing.T) {
	t.Helper()
	select {
	case <-h.cycles:
	case <-time.After(5 * time.Second):
		t.Fatal("等待采集周期超时")
	}
}

func (h *collectorHarness) tick(t *testing.T) {
	t.Helper()
	select {
	case h.c.testCycle <- h.clock.Now():
	case <-time.After(5 * time.Second):
		t.Fatal("发送采集周期超时")
	}
	h.waitCycle(t)
}

func (h *collectorHarness) waitIdle(t *testing.T) {
	t.Helper()
	waitForCondition(t, 5*time.Second, func() bool {
		h.c.mu.Lock()
		defer h.c.mu.Unlock()
		return !h.c.state[0].busy && !h.c.state[1].busy
	}, "采集器空闲")
}

func (h *collectorHarness) busy(provider Provider) bool {
	h.c.mu.Lock()
	defer h.c.mu.Unlock()
	return h.c.state[providerIndex(provider)].busy
}

func (h *collectorHarness) pending(provider Provider) bool {
	h.c.mu.Lock()
	defer h.c.mu.Unlock()
	return h.c.state[providerIndex(provider)].pending
}

func (h *collectorHarness) setBoolConfig(t *testing.T, key string, value bool) {
	t.Helper()
	if err := h.cfg.Set(context.Background(), key, strconv.FormatBool(value)); err != nil {
		t.Fatalf("写入配置 %s 失败: %v", key, err)
	}
}

func (h *collectorHarness) setConfigured(t *testing.T, value bool) {
	t.Helper()
	h.setBoolConfig(t, config.KeyConfigured, value)
}

func (h *collectorHarness) setAdvanced(t *testing.T, value bool) {
	t.Helper()
	h.setBoolConfig(t, config.KeyAdvancedMode, value)
}

func (h *collectorHarness) setTotal(t *testing.T, enabled bool) {
	t.Helper()
	if _, err := h.svc.SetEnabled(context.Background(), enabled); err != nil {
		t.Fatalf("保存总开关失败: %v", err)
	}
}

func (h *collectorHarness) setTotalAndSchedule(t *testing.T, enabled bool) {
	t.Helper()
	if _, err := h.c.SetEnabled(context.Background(), enabled); err != nil {
		t.Fatalf("保存总开关并调度失败: %v", err)
	}
}

func (h *collectorHarness) saveBinding(t *testing.T, provider Provider, enabled bool, instanceID, credential string) Settings {
	t.Helper()
	input := h.bindingInput(t, provider, enabled, instanceID, credential)
	out, err := h.svc.SaveBinding(context.Background(), input)
	if err != nil {
		t.Fatalf("保存 %s 绑定失败: %v", provider, err)
	}
	return out
}

func (h *collectorHarness) saveBindingAndSchedule(t *testing.T, provider Provider, enabled bool, instanceID, credential string) Settings {
	t.Helper()
	input := h.bindingInput(t, provider, enabled, instanceID, credential)
	out, err := h.c.SaveBinding(context.Background(), input)
	if err != nil {
		t.Fatalf("保存 %s 绑定并调度失败: %v", provider, err)
	}
	return out
}

func (h *collectorHarness) bindingInput(t *testing.T, provider Provider, enabled bool, instanceID, credential string) SaveBindingInput {
	t.Helper()
	ctx := context.Background()
	settings, err := h.svc.GetSettings(ctx)
	if err != nil {
		t.Fatalf("读取当前设置失败: %v", err)
	}
	var revision *string
	for _, binding := range settings.Providers {
		if binding.Provider == provider && binding.Revision != "" {
			value := binding.Revision
			revision = &value
		}
	}
	input := SaveBindingInput{
		Provider:   provider,
		Enabled:    enabled,
		InstanceID: instanceID,
		Revision:   revision,
	}
	if credential != "" {
		input.Credential = CredentialInput{Value: credential}
	} else {
		input.Credential = CredentialInput{Omitted: true}
	}
	return input
}

func (h *collectorHarness) clearBinding(t *testing.T, provider Provider) {
	t.Helper()
	settings, err := h.svc.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("读取当前设置失败: %v", err)
	}
	var revision string
	for _, binding := range settings.Providers {
		if binding.Provider == provider {
			revision = binding.Revision
		}
	}
	if revision == "" {
		t.Fatalf("%s 当前未绑定", provider)
	}
	if _, err := h.c.ClearBinding(context.Background(), provider, revision); err != nil {
		t.Fatalf("解除 %s 绑定失败: %v", provider, err)
	}
}

func (h *collectorHarness) results(t *testing.T) []Result {
	t.Helper()
	results, err := h.c.Results(context.Background())
	if err != nil {
		t.Fatalf("读取首页结果失败: %v", err)
	}
	return results
}

func (h *collectorHarness) current(t *testing.T, provider Provider) *Result {
	t.Helper()
	result, err := h.c.CurrentResult(context.Background(), provider)
	if err != nil {
		t.Fatalf("读取 %s 结果失败: %v", provider, err)
	}
	return result
}

func (h *collectorHarness) mustCurrent(t *testing.T, provider Provider) Result {
	t.Helper()
	result := h.current(t, provider)
	if result == nil {
		t.Fatalf("%s 应有结果", provider)
	}
	return *result
}

func (h *collectorHarness) manual(t *testing.T, provider Provider) (Result, error) {
	t.Helper()
	return h.c.ManualRefresh(context.Background(), provider)
}

func (h *collectorHarness) manualOK(t *testing.T, provider Provider) Result {
	t.Helper()
	result, err := h.manual(t, provider)
	if err != nil {
		t.Fatalf("手动采集 %s 失败: %v", provider, err)
	}
	return result
}

type manualOutcome struct {
	result Result
	err    error
}

func (h *collectorHarness) startManual(ctx context.Context, provider Provider) <-chan manualOutcome {
	out := make(chan manualOutcome, 1)
	go func() {
		result, err := h.c.ManualRefresh(ctx, provider)
		out <- manualOutcome{result: result, err: err}
	}()
	return out
}

func waitManual(t *testing.T, out <-chan manualOutcome) manualOutcome {
	t.Helper()
	select {
	case value := <-out:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("等待手动采集超时")
		return manualOutcome{}
	}
}

func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool, description string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("等待 %s 超时", description)
}

func mustErrIs(t *testing.T, err error, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("错误 = %v，期望 %v", err, target)
	}
}

func assertOK(t *testing.T, result Result, wantRemaining float64, wantUnit string) {
	t.Helper()
	if result.Status != ResultOK {
		t.Fatalf("状态 = %q，期望 ok；result=%+v", result.Status, result)
	}
	if result.Remaining != wantRemaining || result.Unit != wantUnit {
		t.Fatalf("余量 = %v %q，期望 %v %q", result.Remaining, result.Unit, wantRemaining, wantUnit)
	}
	if result.SampledAt.IsZero() {
		t.Fatal("成功结果必须带采集时间")
	}
	if result.Message != "" || result.Hint != "" {
		t.Fatalf("成功结果不应带错误文案: %+v", result)
	}
}

func assertErrorResult(t *testing.T, result Result, wantMessage string) {
	t.Helper()
	if result.Status != ResultError {
		t.Fatalf("状态 = %q，期望 error；result=%+v", result.Status, result)
	}
	if result.Message != wantMessage {
		t.Fatalf("文案 = %q，期望 %q", result.Message, wantMessage)
	}
	if result.Remaining != 0 || result.Unit != "" || !result.SampledAt.IsZero() {
		t.Fatalf("错误结果不得保留余量、单位或采集时间: %+v", result)
	}
}

type fakeKiwiVM struct {
	fetch func(ctx context.Context, instanceID, apiKey string) (TrafficSample, error)
}

func (f *fakeKiwiVM) FetchRemaining(ctx context.Context, instanceID, apiKey string) (TrafficSample, error) {
	return f.fetch(ctx, instanceID, apiKey)
}

type fakeVPS struct {
	refresh func(ctx context.Context, refreshToken string) (TokenPair, error)
	fetch   func(ctx context.Context, instanceID, accessToken string) (TrafficSample, error)
}

func (f *fakeVPS) RefreshToken(ctx context.Context, refreshToken string) (TokenPair, error) {
	return f.refresh(ctx, refreshToken)
}

func (f *fakeVPS) FetchRemaining(ctx context.Context, instanceID, accessToken string) (TrafficSample, error) {
	return f.fetch(ctx, instanceID, accessToken)
}

func TestCollectorStartupAndFixedCycle(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-vps-refresh")
	h.setTotal(t, true)

	var kiwiCalls, vpsRefreshCalls, vpsResourceCalls atomic.Int32
	var tokenMu sync.Mutex
	var usedTokens []string
	h.c.kiwi = &fakeKiwiVM{fetch: func(context.Context, string, string) (TrafficSample, error) {
		kiwiCalls.Add(1)
		return TrafficSample{Remaining: 5.5, Unit: UnitGiB}, nil
	}}
	h.c.vps = &fakeVPS{
		refresh: func(context.Context, string) (TokenPair, error) {
			vpsRefreshCalls.Add(1)
			token := syntheticJWTValue(t, map[string]any{"exp": h.clock.Now().Add(time.Hour).Unix()})
			return TokenPair{Token: token, Refresh: "synthetic-rotated-refresh"}, nil
		},
		fetch: func(_ context.Context, _ string, accessToken string) (TrafficSample, error) {
			vpsResourceCalls.Add(1)
			tokenMu.Lock()
			usedTokens = append(usedTokens, accessToken)
			tokenMu.Unlock()
			return TrafficSample{Remaining: 7.25, Unit: UnitGB}, nil
		},
	}

	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)

	if got := kiwiCalls.Load(); got != 1 {
		t.Fatalf("首轮 KiwiVM 调用 = %d，期望 1", got)
	}
	if got := vpsRefreshCalls.Load(); got != 1 {
		t.Fatalf("首轮 V.P.S 续期 = %d，期望 1", got)
	}
	if got := vpsResourceCalls.Load(); got != 1 {
		t.Fatalf("首轮 V.P.S 资源 = %d，期望 1", got)
	}
	results := h.results(t)
	if len(results) != 2 {
		t.Fatalf("首轮结果数 = %d，期望 2", len(results))
	}
	if results[0].Provider != ProviderKiwiVM || results[1].Provider != ProviderVPS {
		t.Fatalf("结果顺序错误: %+v", results)
	}
	assertOK(t, results[0], 5.5, UnitGiB)
	assertOK(t, results[1], 7.25, UnitGB)
	if !results[0].SampledAt.Equal(h.clock.Now()) || !results[1].SampledAt.Equal(h.clock.Now()) {
		t.Fatalf("首轮采集时间不一致: %v / %v", results[0].SampledAt, results[1].SampledAt)
	}

	h.clock.Advance(time.Minute)
	h.tick(t)
	h.waitIdle(t)

	if got := kiwiCalls.Load(); got != 2 {
		t.Fatalf("固定周期 KiwiVM 调用 = %d，期望 2", got)
	}
	if got := vpsRefreshCalls.Load(); got != 1 {
		t.Fatalf("固定周期内未过期 Token 不应续期，实际续期 %d 次", got)
	}
	if got := vpsResourceCalls.Load(); got != 2 {
		t.Fatalf("固定周期 V.P.S 资源 = %d，期望 2", got)
	}
	tokenMu.Lock()
	if len(usedTokens) != 2 || usedTokens[0] != usedTokens[1] {
		t.Fatalf("固定周期应复用同一 Access Token: %v", usedTokens)
	}
	tokenMu.Unlock()
	results = h.results(t)
	if !results[0].SampledAt.Equal(h.clock.Now()) || !results[1].SampledAt.Equal(h.clock.Now()) {
		t.Fatalf("固定周期采集时间未更新: %v / %v", results[0].SampledAt, results[1].SampledAt)
	}
}

func TestCollectorDefaultOffNoOutbound(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)

	var kiwiCalls, vpsRefreshCalls, vpsResourceCalls atomic.Int32
	h.c.kiwi = &fakeKiwiVM{fetch: func(context.Context, string, string) (TrafficSample, error) {
		kiwiCalls.Add(1)
		return TrafficSample{Remaining: 1, Unit: UnitGiB}, nil
	}}
	h.c.vps = &fakeVPS{
		refresh: func(context.Context, string) (TokenPair, error) {
			vpsRefreshCalls.Add(1)
			return TokenPair{Token: "synthetic", Refresh: "synthetic"}, nil
		},
		fetch: func(context.Context, string, string) (TrafficSample, error) {
			vpsResourceCalls.Add(1)
			return TrafficSample{Remaining: 1, Unit: UnitGB}, nil
		},
	}

	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)
	if kiwiCalls.Load() != 0 || vpsRefreshCalls.Load() != 0 || vpsResourceCalls.Load() != 0 {
		t.Fatalf("默认关闭时不得出站: kiwi=%d refresh=%d resource=%d",
			kiwiCalls.Load(), vpsRefreshCalls.Load(), vpsResourceCalls.Load())
	}
	if results := h.results(t); len(results) != 0 {
		t.Fatalf("默认关闭时首页结果应为空: %+v", results)
	}

	h.tick(t)
	h.waitIdle(t)
	if kiwiCalls.Load() != 0 || vpsRefreshCalls.Load() != 0 || vpsResourceCalls.Load() != 0 {
		t.Fatalf("默认关闭的固定周期不得出站: kiwi=%d refresh=%d resource=%d",
			kiwiCalls.Load(), vpsRefreshCalls.Load(), vpsResourceCalls.Load())
	}

	h.setTotalAndSchedule(t, true)
	h.waitIdle(t)
	if kiwiCalls.Load() != 0 || vpsRefreshCalls.Load() != 0 || vpsResourceCalls.Load() != 0 {
		t.Fatalf("两家均未绑定时不得出站: kiwi=%d refresh=%d resource=%d",
			kiwiCalls.Load(), vpsRefreshCalls.Load(), vpsResourceCalls.Load())
	}
}

func TestCollectorGates(t *testing.T) {
	configureCountingKiwi := func(t *testing.T, h *collectorHarness, counter *atomic.Int32) {
		t.Helper()
		h.c.kiwi = &fakeKiwiVM{fetch: func(context.Context, string, string) (TrafficSample, error) {
			counter.Add(1)
			return TrafficSample{Remaining: 9, Unit: UnitGiB}, nil
		}}
	}

	t.Run("Setup 未完成", func(t *testing.T) {
		h := newCollectorHarness(t)
		h.setConfigured(t, false)
		h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
		h.setTotal(t, true)
		var calls atomic.Int32
		configureCountingKiwi(t, h, &calls)

		h.start(t)
		h.waitCycle(t)
		h.waitIdle(t)
		if calls.Load() != 0 {
			t.Fatalf("Setup 未完成不得出站，实际 %d 次", calls.Load())
		}
		if results := h.results(t); len(results) != 0 {
			t.Fatalf("Setup 未完成首页应为空: %+v", results)
		}
		h.tick(t)
		h.waitIdle(t)
		if calls.Load() != 0 {
			t.Fatalf("Setup 未完成的固定周期不得出站，实际 %d 次", calls.Load())
		}

		h.setConfigured(t, true)
		h.tick(t)
		h.waitIdle(t)
		if calls.Load() != 1 {
			t.Fatalf("Setup 完成后应采集 1 次，实际 %d", calls.Load())
		}
	})

	t.Run("高级模式", func(t *testing.T) {
		h := newCollectorHarness(t)
		h.setConfigured(t, true)
		h.setAdvanced(t, true)
		h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
		h.setTotal(t, true)
		var calls atomic.Int32
		configureCountingKiwi(t, h, &calls)

		h.start(t)
		h.waitCycle(t)
		h.waitIdle(t)
		if calls.Load() != 0 {
			t.Fatalf("高级模式不得出站，实际 %d 次", calls.Load())
		}
		if results := h.results(t); len(results) != 0 {
			t.Fatalf("高级模式首页应为空: %+v", results)
		}

		h.setAdvanced(t, false)
		h.tick(t)
		h.waitIdle(t)
		if calls.Load() != 1 {
			t.Fatalf("切回基础模式后下一周期应采集 1 次，实际 %d", calls.Load())
		}
	})

	t.Run("总开关关闭", func(t *testing.T) {
		h := newCollectorHarness(t)
		h.setConfigured(t, true)
		h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
		h.setTotal(t, false)
		var calls atomic.Int32
		configureCountingKiwi(t, h, &calls)

		h.start(t)
		h.waitCycle(t)
		h.waitIdle(t)
		if calls.Load() != 0 {
			t.Fatalf("总开关关闭不得出站，实际 %d 次", calls.Load())
		}
		if results := h.results(t); len(results) != 0 {
			t.Fatalf("总开关关闭首页应为空: %+v", results)
		}

		h.setTotalAndSchedule(t, true)
		h.waitIdle(t)
		if calls.Load() != 1 {
			t.Fatalf("总开关开启后应采集 1 次，实际 %d", calls.Load())
		}
	})

	t.Run("单家开关关闭", func(t *testing.T) {
		h := newCollectorHarness(t)
		h.setConfigured(t, true)
		h.saveBinding(t, ProviderKiwiVM, false, "1001", "synthetic-kiwivm-key")
		h.setTotal(t, true)
		var calls atomic.Int32
		configureCountingKiwi(t, h, &calls)

		h.start(t)
		h.waitCycle(t)
		h.waitIdle(t)
		if calls.Load() != 0 {
			t.Fatalf("单家开关关闭不得出站，实际 %d 次", calls.Load())
		}
		if results := h.results(t); len(results) != 0 {
			t.Fatalf("单家开关关闭首页应为空: %+v", results)
		}

		h.saveBindingAndSchedule(t, ProviderKiwiVM, true, "1001", "")
		h.waitIdle(t)
		if calls.Load() != 1 {
			t.Fatalf("单家重新启用后应采集 1 次，实际 %d", calls.Load())
		}
	})
}

func TestCollectorResultsOrderAndReadOnly(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-vps-refresh")
	h.setTotal(t, true)

	var kiwiCalls, vpsRefreshCalls, vpsResourceCalls atomic.Int32
	h.c.kiwi = &fakeKiwiVM{fetch: func(context.Context, string, string) (TrafficSample, error) {
		kiwiCalls.Add(1)
		return TrafficSample{Remaining: 0, Unit: UnitGiB}, nil
	}}
	h.c.vps = &fakeVPS{
		refresh: func(context.Context, string) (TokenPair, error) {
			vpsRefreshCalls.Add(1)
			return TokenPair{
				Token:   syntheticJWTValue(t, map[string]any{"exp": h.clock.Now().Add(time.Hour).Unix()}),
				Refresh: "synthetic-rotated-refresh",
			}, nil
		},
		fetch: func(context.Context, string, string) (TrafficSample, error) {
			vpsResourceCalls.Add(1)
			return TrafficSample{Remaining: 2, Unit: UnitGB}, nil
		},
	}

	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)
	results := h.results(t)
	if len(results) != 2 || results[0].Provider != ProviderKiwiVM || results[1].Provider != ProviderVPS {
		t.Fatalf("结果顺序或数量错误: %+v", results)
	}
	assertOK(t, results[0], 0, UnitGiB)

	kiwiBefore, refreshBefore, resourceBefore := kiwiCalls.Load(), vpsRefreshCalls.Load(), vpsResourceCalls.Load()
	for i := 0; i < 50; i++ {
		if _, err := h.c.Results(context.Background()); err != nil {
			t.Fatalf("读取结果失败: %v", err)
		}
		if _, err := h.c.CurrentResult(context.Background(), ProviderKiwiVM); err != nil {
			t.Fatalf("读取单家结果失败: %v", err)
		}
		if _, err := h.c.CurrentResult(context.Background(), ProviderVPS); err != nil {
			t.Fatalf("读取单家结果失败: %v", err)
		}
	}
	if kiwiCalls.Load() != kiwiBefore || vpsRefreshCalls.Load() != refreshBefore || vpsResourceCalls.Load() != resourceBefore {
		t.Fatalf("结果读取触发了出站: kiwi=%d->%d refresh=%d->%d resource=%d->%d",
			kiwiBefore, kiwiCalls.Load(), refreshBefore, vpsRefreshCalls.Load(), resourceBefore, vpsResourceCalls.Load())
	}
}

func TestCollectorSetupIncompleteManualNotReady(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, false)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	h.setTotal(t, true)
	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)

	result, err := h.manual(t, ProviderKiwiVM)
	if result.Status != "" {
		t.Fatalf("未就绪手动采集不应返回结果: %+v", result)
	}
	mustErrIs(t, err, ErrNotReady)
}

func TestCollectorAdvancedModeManualUnavailable(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.setAdvanced(t, true)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	h.setTotal(t, true)
	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)

	_, err := h.manual(t, ProviderKiwiVM)
	mustErrIs(t, err, ErrModeUnavailable)
}

func TestCollectorManualBeforeStartAndUnknownProvider(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	h.setTotal(t, true)

	if _, err := h.manual(t, ProviderKiwiVM); !errors.Is(err, ErrCollectorUnavailable) {
		t.Fatalf("未启动手动采集 = %v，期望 ErrCollectorUnavailable", err)
	}
	if _, err := h.manual(t, Provider("unknown")); !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("未知平台 = %v，期望 ErrUnknownProvider", err)
	}
}

func TestCollectorNoCredentialInLogs(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	const (
		kiwiCredential = "synthetic-kiwivm-secret-key"
		vpsRefresh     = "synthetic-vps-refresh-secret"
	)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", kiwiCredential)
	h.saveBinding(t, ProviderVPS, true, "2002", vpsRefresh)
	h.setTotal(t, true)
	accessToken := syntheticJWTValue(t, map[string]any{"exp": h.clock.Now().Add(time.Hour).Unix()})
	const rotatedRefresh = "synthetic-rotated-refresh-secret"

	h.c.kiwi = &fakeKiwiVM{fetch: func(context.Context, string, string) (TrafficSample, error) {
		return TrafficSample{Remaining: 4, Unit: UnitGiB}, nil
	}}
	h.c.vps = &fakeVPS{
		refresh: func(context.Context, string) (TokenPair, error) {
			return TokenPair{Token: accessToken, Refresh: rotatedRefresh}, nil
		},
		fetch: func(context.Context, string, string) (TrafficSample, error) {
			return TrafficSample{Remaining: 6, Unit: UnitGB}, nil
		},
	}

	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)

	logs := h.logBuf.String()
	for _, secret := range []string{kiwiCredential, vpsRefresh, accessToken, rotatedRefresh} {
		if secret != "" && bytes.Contains(h.logBuf.Bytes(), []byte(secret)) {
			t.Fatalf("日志不得包含凭据或 Token: %q\nlogs=%s", secret, logs)
		}
	}
	results := h.results(t)
	for _, result := range results {
		rendered := fmt.Sprintf("%+v", result)
		for _, secret := range []string{kiwiCredential, vpsRefresh, accessToken, rotatedRefresh} {
			if secret != "" && containsString(rendered, secret) {
				t.Fatalf("结果不得包含凭据或 Token: %+v", result)
			}
		}
	}
}

func containsString(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && bytes.Contains([]byte(haystack), []byte(needle))
}
