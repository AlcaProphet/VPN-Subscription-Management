// 本文件所有实例 ID、凭据与 JWT 均为合成样本，不包含真实凭据。
package providertraffic

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func publishLegacy(c *Collector, ctx, requestCtx context.Context, provider Provider, revision string, result Result) error {
	_, err := c.publishHost(ctx, requestCtx, legacyKey(provider), collectionRevision{binding: revision}, result)
	return err
}

func TestCollectorCancelWhilePublicationWaitsForResultLock(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	h.setTotal(t, true)
	h.c.kiwi = &fakeKiwiVM{fetch: func(context.Context, string, string) (TrafficSample, error) {
		return TrafficSample{Remaining: 3, Unit: UnitGiB}, nil
	}}
	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)
	previous := h.mustCurrent(t, ProviderKiwiVM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	db := h.st.DB()
	func() {
		h.c.mu.Lock()
		defer h.c.mu.Unlock()
		go func() {
			// 先取消手动请求，模拟链上下文的异步取消传播尚未到达。
			done <- publishLegacy(h.c, context.Background(), ctx, ProviderKiwiVM, previous.revision, Result{
				Provider: ProviderKiwiVM, Status: ResultError, Message: "服务商返回错误",
			})
		}()
		waitForCondition(t, 5*time.Second, func() bool { return db.Stats().InUse == 1 }, "发布事务取得连接")
		cancel()
	}()
	select {
	case err := <-done:
		mustErrIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("等待取消后的发布结束超时")
	}
	if got := h.mustCurrent(t, ProviderKiwiVM); got != previous {
		t.Fatalf("取消后应保留原结果及采集时间: %+v", got)
	}
}

func TestCollectorPublicationOrdersWithSettingsWrite(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	settings := h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)
	h.setTotal(t, true)
	revision := settings.Providers[0].Revision

	ctx := context.Background()
	result := Result{Provider: ProviderKiwiVM, Status: ResultOK, Remaining: 9, Unit: UnitGiB, SampledAt: h.clock.Now()}
	published, disabled := make(chan error, 1), make(chan error, 1)
	db := h.st.DB()
	func() {
		h.c.mu.Lock()
		defer h.c.mu.Unlock()
		go func() { published <- publishLegacy(h.c, ctx, nil, ProviderKiwiVM, revision, result) }()
		waitForCondition(t, 5*time.Second, func() bool { return db.Stats().InUse == 1 }, "发布事务取得连接")
		waitCount := db.Stats().WaitCount
		go func() {
			_, err := h.svc.SetEnabled(ctx, false)
			disabled <- err
		}()
		waitForCondition(t, 5*time.Second, func() bool { return db.Stats().WaitCount > waitCount }, "关闭事务等待发布结束")
	}()
	for _, done := range []chan error{published, disabled} {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("等待发布或关闭事务超时")
		}
	}
	assertOK(t, h.mustCurrent(t, ProviderKiwiVM), 9, UnitGiB)
	if got := h.results(t); len(got) != 0 {
		t.Fatalf("关闭后首页应隐藏原结果: %+v", got)
	}
	// 关闭已提交后，下一份在途结果必须丢弃。
	result.Remaining = 10
	mustErrIs(t, publishLegacy(h.c, ctx, nil, ProviderKiwiVM, revision, result), ErrDiscarded)
	assertOK(t, h.mustCurrent(t, ProviderKiwiVM), 9, UnitGiB)
}

func TestCollectorResultSuccessFailureRecovery(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-vps-refresh")
	h.setTotal(t, true)

	var kiwiCalls atomic.Int32
	var vpsFail atomic.Bool
	h.c.kiwi = &fakeKiwiVM{fetch: func(context.Context, string, string) (TrafficSample, error) {
		switch kiwiCalls.Add(1) {
		case 1:
			return TrafficSample{Remaining: 10, Unit: UnitGiB}, nil
		case 2:
			return TrafficSample{}, &UpstreamError{Category: UpstreamInvalidData}
		default:
			return TrafficSample{Remaining: 20, Unit: UnitGiB}, nil
		}
	}}
	h.c.vps = &fakeVPS{
		refresh: func(context.Context, string) (TokenPair, error) {
			return TokenPair{
				Token:   syntheticJWTValue(t, map[string]any{"exp": h.clock.Now().Add(time.Hour).Unix()}),
				Refresh: "synthetic-rotated-refresh",
			}, nil
		},
		fetch: func(context.Context, string, string) (TrafficSample, error) {
			if vpsFail.Load() {
				return TrafficSample{}, &UpstreamError{Category: UpstreamInvalidData}
			}
			return TrafficSample{Remaining: 7, Unit: UnitGB}, nil
		},
	}

	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)
	assertOK(t, h.mustCurrent(t, ProviderKiwiVM), 10, UnitGiB)
	assertOK(t, h.mustCurrent(t, ProviderVPS), 7, UnitGB)

	h.clock.Advance(time.Minute)
	kiwiError := h.manualOK(t, ProviderKiwiVM)
	assertErrorResult(t, kiwiError, "服务商返回的流量数据无法读取")
	if kiwiError.Hint == "" {
		t.Fatal("流量数据非法应带安全提示")
	}
	assertOK(t, h.mustCurrent(t, ProviderVPS), 7, UnitGB)

	h.clock.Advance(time.Minute)
	assertOK(t, h.manualOK(t, ProviderKiwiVM), 20, UnitGiB)
	if got := h.mustCurrent(t, ProviderKiwiVM); !got.SampledAt.Equal(h.clock.Now()) {
		t.Fatalf("恢复后的采集时间 = %v，期望 %v", got.SampledAt, h.clock.Now())
	}

	vpsFail.Store(true)
	assertErrorResult(t, h.manualOK(t, ProviderVPS), "服务商返回的流量数据无法读取")
	assertOK(t, h.mustCurrent(t, ProviderKiwiVM), 20, UnitGiB)
	assertErrorResult(t, h.mustCurrent(t, ProviderVPS), "服务商返回的流量数据无法读取")

	vpsFail.Store(false)
	assertOK(t, h.manualOK(t, ProviderVPS), 7, UnitGB)
	assertOK(t, h.mustCurrent(t, ProviderKiwiVM), 20, UnitGiB)
}

func TestCollectorToggleRetainsResult(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	h.setTotal(t, true)

	var calls atomic.Int32
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	h.c.kiwi = &fakeKiwiVM{fetch: func(ctx context.Context, _ string, _ string) (TrafficSample, error) {
		n := calls.Add(1)
		started <- struct{}{}
		if n == 2 {
			select {
			case <-release:
			case <-ctx.Done():
				return TrafficSample{}, ctx.Err()
			}
		}
		return TrafficSample{Remaining: float64(n), Unit: UnitGiB}, nil
	}}

	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)
	first := h.mustCurrent(t, ProviderKiwiVM)
	assertOK(t, first, 1, UnitGiB)

	h.saveBindingAndSchedule(t, ProviderKiwiVM, false, "1001", "")
	h.waitIdle(t)
	if results := h.results(t); len(results) != 0 {
		t.Fatalf("停用后首页应隐藏该行: %+v", results)
	}
	if retained := h.mustCurrent(t, ProviderKiwiVM); !retained.SampledAt.Equal(first.SampledAt) {
		t.Fatalf("停用应保留管理端原结果与时间: %+v", retained)
	}

	h.clock.Advance(10 * time.Minute)
	h.saveBindingAndSchedule(t, ProviderKiwiVM, true, "1001", "")
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("等待重新启用后的采集开始超时")
	}
	results := h.results(t)
	if len(results) != 1 || results[0].Status != ResultOK || results[0].Remaining != 1 {
		t.Fatalf("重新启用后、新结果前应显示原结果: %+v", results)
	}
	if !results[0].SampledAt.Equal(first.SampledAt) {
		t.Fatalf("重新启用后应保留原采集时间: got=%v want=%v", results[0].SampledAt, first.SampledAt)
	}

	close(release)
	h.waitIdle(t)
	updated := h.mustCurrent(t, ProviderKiwiVM)
	assertOK(t, updated, 2, UnitGiB)
	if !updated.SampledAt.Equal(h.clock.Now()) {
		t.Fatalf("新结果采集时间 = %v，期望 %v", updated.SampledAt, h.clock.Now())
	}
}

func TestCollectorUpstreamErrorMessages(t *testing.T) {
	cases := []struct {
		name     string
		category UpstreamErrorCategory
		message  string
	}{
		{"无法连接", UpstreamUnavailable, "无法连接服务商"},
		{"限流", UpstreamRateLimited, "服务商限流"},
		{"倍率待核对", UpstreamMultiplierCheck, "计费倍率待核对"},
		{"流量数据非法", UpstreamInvalidData, "服务商返回的流量数据无法读取"},
		{"凭据被拒绝", UpstreamCredentialRejected, "服务商凭据被拒绝"},
		{"Access Token 被拒绝", UpstreamAccessTokenRejected, "服务商鉴权失败"},
		{"其他上游错误", UpstreamFailed, "服务商返回错误"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newCollectorHarness(t)
			h.setConfigured(t, true)
			h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
			h.setTotal(t, true)
			h.c.kiwi = &fakeKiwiVM{fetch: func(context.Context, string, string) (TrafficSample, error) {
				return TrafficSample{}, &UpstreamError{Category: tc.category}
			}}

			h.start(t)
			h.waitCycle(t)
			h.waitIdle(t)
			result := h.mustCurrent(t, ProviderKiwiVM)
			assertErrorResult(t, result, tc.message)
			if result.Hint == "" {
				t.Fatalf("%s 应带安全提示", tc.category)
			}
		})
	}
}

func TestCollectorAdvancedModeDuringFetch(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	h.setTotal(t, true)

	var calls atomic.Int32
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	h.c.kiwi = &fakeKiwiVM{fetch: func(ctx context.Context, _ string, _ string) (TrafficSample, error) {
		n := calls.Add(1)
		if n == 1 {
			started <- struct{}{}
			select {
			case <-release:
			case <-ctx.Done():
				return TrafficSample{}, ctx.Err()
			}
		}
		return TrafficSample{Remaining: float64(n), Unit: UnitGiB}, nil
	}}

	h.start(t)
	h.waitCycle(t)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("等待首轮开始超时")
	}
	h.setAdvanced(t, true)
	if results := h.results(t); len(results) != 0 {
		t.Fatalf("高级模式首页应为空: %+v", results)
	}
	close(release)
	h.waitIdle(t)
	if result := h.current(t, ProviderKiwiVM); result != nil {
		t.Fatalf("高级模式切换后不得发布在途结果: %+v", result)
	}

	h.setAdvanced(t, false)
	results := h.results(t)
	if len(results) != 1 || results[0].Status != ResultWaiting {
		t.Fatalf("切回基础模式后应回到待采集: %+v", results)
	}
	h.tick(t)
	h.waitIdle(t)
	assertOK(t, h.mustCurrent(t, ProviderKiwiVM), 2, UnitGiB)
}

func TestCollectorTotalOffWhileBothInFlight(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-original-refresh")
	h.setTotal(t, true)

	kiwiStarted := make(chan struct{}, 1)
	kiwiRelease := make(chan struct{})
	vpsStarted := make(chan struct{}, 1)
	vpsRelease := make(chan struct{})
	var vpsResourceCalls atomic.Int32
	h.c.kiwi = &fakeKiwiVM{fetch: func(ctx context.Context, _ string, _ string) (TrafficSample, error) {
		kiwiStarted <- struct{}{}
		select {
		case <-kiwiRelease:
		case <-ctx.Done():
			return TrafficSample{}, ctx.Err()
		}
		return TrafficSample{Remaining: 1, Unit: UnitGiB}, nil
	}}
	h.c.vps = &fakeVPS{
		refresh: func(ctx context.Context, _ string) (TokenPair, error) {
			vpsStarted <- struct{}{}
			select {
			case <-vpsRelease:
			case <-ctx.Done():
				return TokenPair{}, ctx.Err()
			}
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
	select {
	case <-kiwiStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("等待 KiwiVM 开始超时")
	}
	select {
	case <-vpsStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("等待 V.P.S 开始超时")
	}

	h.setTotalAndSchedule(t, false)
	close(kiwiRelease)
	close(vpsRelease)
	h.waitIdle(t)

	if h.current(t, ProviderKiwiVM) != nil || h.current(t, ProviderVPS) != nil {
		t.Fatal("关闭功能后不得发布任何在途结果")
	}
	if results := h.results(t); len(results) != 0 {
		t.Fatalf("关闭功能后首页应为空: %+v", results)
	}
	if got := vpsResourceCalls.Load(); got != 0 {
		t.Fatalf("关闭功能后不得查询资源，实际 %d 次", got)
	}
	if _, _, credential, err := h.svc.LoadCredential(context.Background(), ProviderVPS); err != nil {
		t.Fatalf("读取关闭后的绑定失败: %v", err)
	} else if credential != "synthetic-rotated-refresh" {
		t.Fatalf("同一绑定关闭功能后仍应提交续期结果，实际 %q", credential)
	}
}

func TestCollectorTotalToggleRetainsResult(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	h.setTotal(t, true)

	var calls atomic.Int32
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	h.c.kiwi = &fakeKiwiVM{fetch: func(ctx context.Context, _ string, _ string) (TrafficSample, error) {
		n := calls.Add(1)
		started <- struct{}{}
		if n == 2 {
			select {
			case <-release:
			case <-ctx.Done():
				return TrafficSample{}, ctx.Err()
			}
		}
		return TrafficSample{Remaining: float64(n), Unit: UnitGiB}, nil
	}}

	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)
	first := h.mustCurrent(t, ProviderKiwiVM)

	h.setTotalAndSchedule(t, false)
	h.waitIdle(t)
	if results := h.results(t); len(results) != 0 {
		t.Fatalf("关闭总开关后首页应为空: %+v", results)
	}
	if retained := h.mustCurrent(t, ProviderKiwiVM); !retained.SampledAt.Equal(first.SampledAt) {
		t.Fatalf("关闭总开关应保留最近结果: %+v", retained)
	}

	h.clock.Advance(5 * time.Minute)
	h.setTotalAndSchedule(t, true)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("等待重新开启后的采集开始超时")
	}
	results := h.results(t)
	if len(results) != 1 || results[0].Remaining != 1 || !results[0].SampledAt.Equal(first.SampledAt) {
		t.Fatalf("重新开启后、新结果前应显示原结果及时间: %+v", results)
	}

	close(release)
	h.waitIdle(t)
	updated := h.mustCurrent(t, ProviderKiwiVM)
	assertOK(t, updated, 2, UnitGiB)
	if !updated.SampledAt.Equal(h.clock.Now()) {
		t.Fatalf("新结果时间 = %v，期望 %v", updated.SampledAt, h.clock.Now())
	}
}
