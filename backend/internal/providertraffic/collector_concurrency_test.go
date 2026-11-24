// 本文件所有实例 ID、凭据与 JWT 均为合成样本，不包含真实凭据。
package providertraffic

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCollectorPeriodicSkipsBusyProvider(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-vps-refresh")
	h.setTotal(t, true)

	var kiwiCalls, vpsResourceCalls atomic.Int32
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	h.c.kiwi = &fakeKiwiVM{fetch: func(ctx context.Context, _ string, _ string) (TrafficSample, error) {
		kiwiCalls.Add(1)
		started <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
			return TrafficSample{}, ctx.Err()
		}
		return TrafficSample{Remaining: 3, Unit: UnitGiB}, nil
	}}
	h.c.vps = &fakeVPS{
		refresh: func(context.Context, string) (TokenPair, error) {
			return TokenPair{
				Token:   syntheticJWTValue(t, map[string]any{"exp": h.clock.Now().Add(time.Hour).Unix()}),
				Refresh: "synthetic-rotated-refresh",
			}, nil
		},
		fetch: func(context.Context, string, string) (TrafficSample, error) {
			vpsResourceCalls.Add(1)
			return TrafficSample{Remaining: 6, Unit: UnitGB}, nil
		},
	}

	h.start(t)
	h.waitCycle(t)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("等待 KiwiVM 首轮开始超时")
	}
	waitForCondition(t, 5*time.Second, func() bool {
		return vpsResourceCalls.Load() == 1 && !h.busy(ProviderVPS)
	}, "V.P.S 首轮完成")

	h.tick(t)
	waitForCondition(t, 5*time.Second, func() bool { return vpsResourceCalls.Load() == 2 }, "V.P.S 周期采集完成")
	if got := kiwiCalls.Load(); got != 1 {
		t.Fatalf("定时遇忙不得追加 KiwiVM 请求，实际 %d 次", got)
	}
	if !h.busy(ProviderKiwiVM) {
		t.Fatal("KiwiVM 应仍处于在途状态")
	}
	if h.pending(ProviderKiwiVM) {
		t.Fatal("定时遇忙不得设置待执行信号")
	}

	close(release)
	h.waitIdle(t)
	if got := kiwiCalls.Load(); got != 1 {
		t.Fatalf("定时遇忙跳过不得在结束后补采，实际 %d 次", got)
	}
	result := h.mustCurrent(t, ProviderKiwiVM)
	assertOK(t, result, 3, UnitGiB)
}

func TestCollectorManualBusy(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	h.setTotal(t, true)

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	h.c.kiwi = &fakeKiwiVM{fetch: func(ctx context.Context, _ string, _ string) (TrafficSample, error) {
		started <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
			return TrafficSample{}, ctx.Err()
		}
		return TrafficSample{Remaining: 2, Unit: UnitGiB}, nil
	}}

	h.start(t)
	h.waitCycle(t)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("等待首轮开始超时")
	}

	result, err := h.manual(t, ProviderKiwiVM)
	if result.Status != "" {
		t.Fatalf("忙碌手动采集不应返回结果: %+v", result)
	}
	mustErrIs(t, err, ErrCollectorBusy)

	close(release)
	h.waitIdle(t)
	h.manualOK(t, ProviderKiwiVM)
}

func TestCollectorConfigTriggerCoalesces(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-key-1")
	h.setTotal(t, true)

	var calls atomic.Int32
	started := make(chan struct{}, 4)
	release := make(chan struct{})
	var mu sync.Mutex
	var credentials []string
	h.c.kiwi = &fakeKiwiVM{fetch: func(ctx context.Context, _ string, credential string) (TrafficSample, error) {
		n := calls.Add(1)
		mu.Lock()
		credentials = append(credentials, credential)
		mu.Unlock()
		started <- struct{}{}
		if n == 1 {
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

	second := h.saveBindingAndSchedule(t, ProviderKiwiVM, true, "1001", "synthetic-key-2")
	if !h.pending(ProviderKiwiVM) {
		t.Fatal("配置变更遇忙应设置待执行信号")
	}

	third := h.saveBindingAndSchedule(t, ProviderKiwiVM, true, "1001", "synthetic-key-3")
	if !h.pending(ProviderKiwiVM) {
		t.Fatal("第二次配置变更应继续合并为同一待执行信号")
	}
	if second.Providers[0].Revision == third.Providers[0].Revision {
		t.Fatal("两次换证应产生不同修订")
	}

	close(release)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("等待合并后的补采开始超时")
	}
	h.waitIdle(t)

	if got := calls.Load(); got != 2 {
		t.Fatalf("配置变更遇忙应只补采一次，实际 %d 次", got)
	}
	mu.Lock()
	if len(credentials) != 2 || credentials[0] != "synthetic-key-1" || credentials[1] != "synthetic-key-3" {
		t.Fatalf("补采应使用最新凭据: %v", credentials)
	}
	mu.Unlock()

	result := h.mustCurrent(t, ProviderKiwiVM)
	assertOK(t, result, 2, UnitGiB)
	if result.revision != third.Providers[0].Revision {
		t.Fatalf("结果修订 = %q，期望最新修订 %q", result.revision, third.Providers[0].Revision)
	}
	if h.pending(ProviderKiwiVM) {
		t.Fatal("补采完成后不得残留待执行信号")
	}
}

func TestCollectorIndependentProviders(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-vps-refresh")
	h.setTotal(t, true)

	kiwiStarted := make(chan struct{}, 1)
	kiwiRelease := make(chan struct{})
	vpsStarted := make(chan struct{}, 1)
	vpsRelease := make(chan struct{})
	h.c.kiwi = &fakeKiwiVM{fetch: func(ctx context.Context, _ string, _ string) (TrafficSample, error) {
		kiwiStarted <- struct{}{}
		select {
		case <-kiwiRelease:
		case <-ctx.Done():
			return TrafficSample{}, ctx.Err()
		}
		return TrafficSample{Remaining: 11, Unit: UnitGiB}, nil
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
			return TrafficSample{Remaining: 12, Unit: UnitGB}, nil
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

	close(kiwiRelease)
	waitForCondition(t, 5*time.Second, func() bool { return h.current(t, ProviderKiwiVM) != nil }, "KiwiVM 结果发布")
	if h.current(t, ProviderVPS) != nil {
		t.Fatal("一家完成不得提前发布另一家结果")
	}
	if !h.busy(ProviderVPS) {
		t.Fatal("V.P.S 应仍在途")
	}

	close(vpsRelease)
	h.waitIdle(t)
	assertOK(t, h.mustCurrent(t, ProviderKiwiVM), 11, UnitGiB)
	assertOK(t, h.mustCurrent(t, ProviderVPS), 12, UnitGB)
}

func TestCollectorUnbindInFlight(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	h.setTotal(t, true)

	var calls atomic.Int32
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	h.c.kiwi = &fakeKiwiVM{fetch: func(ctx context.Context, _ string, _ string) (TrafficSample, error) {
		calls.Add(1)
		started <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
			return TrafficSample{}, ctx.Err()
		}
		return TrafficSample{Remaining: 1, Unit: UnitGiB}, nil
	}}

	h.start(t)
	h.waitCycle(t)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("等待首轮开始超时")
	}

	h.clearBinding(t, ProviderKiwiVM)
	close(release)
	h.waitIdle(t)

	if got := calls.Load(); got != 1 {
		t.Fatalf("解绑不得触发补采，实际 %d 次", got)
	}
	if result := h.current(t, ProviderKiwiVM); result != nil {
		t.Fatalf("解绑后不得发布旧结果: %+v", result)
	}
	if results := h.results(t); len(results) != 0 {
		t.Fatalf("解绑后首页应为空: %+v", results)
	}
}

func TestCollectorClearTableInFlight(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-original-refresh")
	h.setTotal(t, true)

	var resourceCalls atomic.Int32
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	h.c.vps = &fakeVPS{
		refresh: func(ctx context.Context, _ string) (TokenPair, error) {
			started <- struct{}{}
			select {
			case <-release:
			case <-ctx.Done():
				return TokenPair{}, ctx.Err()
			}
			return TokenPair{
				Token:   syntheticJWTValue(t, map[string]any{"exp": h.clock.Now().Add(time.Hour).Unix()}),
				Refresh: "synthetic-rotated-refresh",
			}, nil
		},
		fetch: func(context.Context, string, string) (TrafficSample, error) {
			resourceCalls.Add(1)
			return TrafficSample{Remaining: 1, Unit: UnitGB}, nil
		},
	}

	h.start(t)
	h.waitCycle(t)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("等待续期开始超时")
	}

	if _, err := h.st.DB().ExecContext(context.Background(), `DELETE FROM provider_traffic_settings`); err != nil {
		t.Fatalf("模拟导入/清空删除功能行失败: %v", err)
	}
	close(release)
	h.waitIdle(t)

	if got := resourceCalls.Load(); got != 0 {
		t.Fatalf("清空后不得查询资源，实际 %d 次", got)
	}
	if _, _, _, err := h.svc.LoadCredential(context.Background(), ProviderVPS); !errors.Is(err, ErrNotBound) {
		t.Fatalf("清空后绑定应不存在，实际 %v", err)
	}
	if result := h.current(t, ProviderVPS); result != nil {
		t.Fatalf("清空后不得发布结果: %+v", result)
	}
	if results := h.results(t); len(results) != 0 {
		t.Fatalf("清空后首页应为空: %+v", results)
	}
}

func TestCollectorStopNoPublishAndCannotRestart(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	h.setTotal(t, true)

	started := make(chan struct{}, 1)
	h.c.kiwi = &fakeKiwiVM{fetch: func(ctx context.Context, _ string, _ string) (TrafficSample, error) {
		started <- struct{}{}
		<-ctx.Done()
		return TrafficSample{}, ctx.Err()
	}}

	h.start(t)
	h.waitCycle(t)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("等待首轮开始超时")
	}

	h.c.Stop()
	if result := h.current(t, ProviderKiwiVM); result != nil {
		t.Fatalf("停止后不得发布在途结果: %+v", result)
	}
	if err := h.c.Start(context.Background()); !errors.Is(err, ErrCollectorStopped) {
		t.Fatalf("停止后重新启动 = %v，期望 ErrCollectorStopped", err)
	}
}

func TestCollectorRestartLosesMemory(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-vps-refresh")
	h.setTotal(t, true)

	h.c.kiwi = &fakeKiwiVM{fetch: func(context.Context, string, string) (TrafficSample, error) {
		return TrafficSample{Remaining: 1, Unit: UnitGiB}, nil
	}}
	h.c.vps = &fakeVPS{
		refresh: func(context.Context, string) (TokenPair, error) {
			return TokenPair{
				Token:   syntheticJWTValue(t, map[string]any{"exp": h.clock.Now().Add(time.Hour).Unix()}),
				Refresh: "synthetic-rotated-refresh",
			}, nil
		},
		fetch: func(context.Context, string, string) (TrafficSample, error) {
			return TrafficSample{Remaining: 2, Unit: UnitGB}, nil
		},
	}

	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)
	first := h.mustCurrent(t, ProviderKiwiVM)
	h.c.Stop()

	h.clock.Advance(time.Hour)
	var kiwiCalls, vpsRefreshCalls atomic.Int32
	var refreshInput string
	c2 := NewCollector(h.svc, h.cfg, h.log)
	c2.now = h.clock.Now
	c2.testCycle = make(chan time.Time, 1)
	cycles2 := make(chan struct{}, 8)
	c2.testOnCycle = func() {
		select {
		case cycles2 <- struct{}{}:
		default:
		}
	}
	c2.kiwi = &fakeKiwiVM{fetch: func(context.Context, string, string) (TrafficSample, error) {
		kiwiCalls.Add(1)
		return TrafficSample{Remaining: 3, Unit: UnitGiB}, nil
	}}
	c2.vps = &fakeVPS{
		refresh: func(_ context.Context, refreshToken string) (TokenPair, error) {
			refreshInput = refreshToken
			vpsRefreshCalls.Add(1)
			return TokenPair{
				Token:   syntheticJWTValue(t, map[string]any{"exp": h.clock.Now().Add(time.Hour).Unix()}),
				Refresh: "synthetic-rotated-refresh-2",
			}, nil
		},
		fetch: func(context.Context, string, string) (TrafficSample, error) {
			return TrafficSample{Remaining: 4, Unit: UnitGB}, nil
		},
	}

	if result := c2.ResultForRevision(ProviderKiwiVM, first.revision); result != nil {
		t.Fatalf("新进程不应持有旧结果: %+v", result)
	}
	waiting, err := c2.Results(context.Background())
	if err != nil {
		t.Fatalf("新进程读取首页失败: %v", err)
	}
	if len(waiting) != 2 || waiting[0].Status != ResultWaiting || waiting[1].Status != ResultWaiting {
		t.Fatalf("重启后应回到待采集: %+v", waiting)
	}

	if err := c2.Start(context.Background()); err != nil {
		t.Fatalf("重启采集服务失败: %v", err)
	}
	t.Cleanup(c2.Stop)
	select {
	case <-cycles2:
	case <-time.After(5 * time.Second):
		t.Fatal("等待重启首轮超时")
	}
	waitForCondition(t, 5*time.Second, func() bool {
		c2.mu.Lock()
		defer c2.mu.Unlock()
		return !hostState(c2, ProviderKiwiVM).busy && !hostState(c2, ProviderVPS).busy
	}, "重启采集器空闲")

	if got := kiwiCalls.Load(); got != 1 {
		t.Fatalf("重启首轮 KiwiVM 调用 = %d，期望 1", got)
	}
	if got := vpsRefreshCalls.Load(); got != 1 {
		t.Fatalf("重启不得复用旧 Access Token，续期 = %d 次", got)
	}
	if refreshInput != "synthetic-rotated-refresh" {
		t.Fatalf("重启应使用库内已轮换的 Refresh Token，实际 %q", refreshInput)
	}
	current := h.mustCurrent(t, ProviderKiwiVM)
	if !current.SampledAt.Equal(first.SampledAt) {
		t.Fatalf("停止后内存结果应保留原采集时间: got=%v want=%v", current.SampledAt, first.SampledAt)
	}
	c2Result := c2.ResultForRevision(ProviderKiwiVM, first.revision)
	if c2Result == nil {
		t.Fatal("重启首轮应生成新结果")
	}
	assertOK(t, *c2Result, 3, UnitGiB)
	if c2Result.SampledAt.Equal(first.SampledAt) {
		t.Fatal("重启结果应使用新的采集时间")
	}
}

func TestCollectorConcurrentReads(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	settings := h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	kiwi, _ := bindingFromSettings(settings, ProviderKiwiVM)
	h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-vps-refresh")
	h.setTotal(t, true)

	var kiwiCalls atomic.Int32
	h.c.kiwi = &fakeKiwiVM{fetch: func(context.Context, string, string) (TrafficSample, error) {
		n := kiwiCalls.Add(1)
		return TrafficSample{Remaining: float64(n), Unit: UnitGiB}, nil
	}}
	h.c.vps = &fakeVPS{
		refresh: func(context.Context, string) (TokenPair, error) {
			return TokenPair{
				Token:   syntheticJWTValue(t, map[string]any{"exp": h.clock.Now().Add(time.Hour).Unix()}),
				Refresh: "synthetic-rotated-refresh",
			}, nil
		},
		fetch: func(context.Context, string, string) (TrafficSample, error) {
			return TrafficSample{Remaining: 2, Unit: UnitGB}, nil
		},
	}

	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				results, err := h.c.Results(context.Background())
				if err != nil {
					t.Errorf("并发读取首页结果失败: %v", err)
					return
				}
				for _, result := range results {
					switch result.Status {
					case ResultOK:
						if result.Unit == "" || result.SampledAt.IsZero() {
							t.Errorf("成功结果字段不完整: %+v", result)
						}
					case ResultWaiting:
						if result.Remaining != 0 || result.Unit != "" || !result.SampledAt.IsZero() || result.Message != "" {
							t.Errorf("等待结果不应带数据: %+v", result)
						}
					case ResultError:
						if result.Message == "" || result.Remaining != 0 || result.Unit != "" || !result.SampledAt.IsZero() {
							t.Errorf("错误结果字段异常: %+v", result)
						}
					default:
						t.Errorf("未知结果状态: %q", result.Status)
					}
				}
				if result := h.c.ResultForRevision(ProviderKiwiVM, kiwi.revision); result == nil || result.Status != ResultOK {
					t.Errorf("并发读取单家结果失败: %+v", result)
					return
				}
			}
		}()
	}
	h.manualOK(t, ProviderKiwiVM)
	wg.Wait()
	h.waitIdle(t)
}

func TestCollectorResultForRevisionAfterRebind(t *testing.T) {
	h := newCollectorHarness(t)
	previous := h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-initial")
	oldBinding, _ := bindingFromSettings(previous, ProviderKiwiVM)
	h.setTotal(t, true)
	h.c.kiwi = &fakeKiwiVM{fetch: func(_ context.Context, id, _ string) (TrafficSample, error) {
		if id == "1002" {
			return TrafficSample{Remaining: 23, Unit: UnitGiB}, nil
		}
		return TrafficSample{Remaining: 12, Unit: UnitGiB}, nil
	}}
	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)
	assertOK(t, h.mustCurrent(t, ProviderKiwiVM), 12, UnitGiB)

	replaced := h.saveBindingAndSchedule(t, ProviderKiwiVM, true, "1002", "synthetic-replacement")
	current, _ := bindingFromSettings(replaced, ProviderKiwiVM)
	h.waitIdle(t)
	if result := h.c.ResultForRevision(ProviderKiwiVM, oldBinding.revision); result != nil {
		t.Fatalf("旧绑定不得读取新绑定的结果: %+v", result)
	}
	result := h.c.ResultForRevision(ProviderKiwiVM, current.revision)
	if result == nil {
		t.Fatal("新绑定应能读取自己的结果")
	}
	assertOK(t, *result, 23, UnitGiB)
}

func TestCollectorNoWriteTransactionDuringOutbound(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-original-refresh")
	h.setTotal(t, true)

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	h.c.vps = &fakeVPS{
		refresh: func(ctx context.Context, _ string) (TokenPair, error) {
			started <- struct{}{}
			select {
			case <-release:
			case <-ctx.Done():
				return TokenPair{}, ctx.Err()
			}
			return TokenPair{
				Token:   syntheticJWTValue(t, map[string]any{"exp": h.clock.Now().Add(time.Hour).Unix()}),
				Refresh: "synthetic-rotated-refresh",
			}, nil
		},
		fetch: func(context.Context, string, string) (TrafficSample, error) {
			return TrafficSample{Remaining: 5, Unit: UnitGB}, nil
		},
	}

	h.start(t)
	h.waitCycle(t)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("等待续期开始超时")
	}

	writeDone := make(chan error, 1)
	go func() {
		writeDone <- h.cfg.Set(context.Background(), "synthetic_outbound_probe", "1")
	}()
	select {
	case err := <-writeDone:
		if err != nil {
			t.Fatalf("出站期间本地写入失败: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("出站期间本地写入被阻塞，疑似持有写事务")
	}

	close(release)
	h.waitIdle(t)
}

func TestCollectorImportResetsTokenCache(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-original-refresh")
	h.setTotal(t, true)

	var refreshCalls, resourceCalls atomic.Int32
	h.c.vps = &fakeVPS{
		refresh: func(context.Context, string) (TokenPair, error) {
			n := refreshCalls.Add(1)
			return TokenPair{
				Token: syntheticJWTValue(t, map[string]any{
					"exp": h.clock.Now().Add(time.Hour).Unix(),
					"jti": fmt.Sprintf("synthetic-token-%d", n),
				}),
				Refresh: fmt.Sprintf("synthetic-rotated-refresh-%d", n),
			}, nil
		},
		fetch: func(context.Context, string, string) (TrafficSample, error) {
			resourceCalls.Add(1)
			return TrafficSample{Remaining: 7, Unit: UnitGB}, nil
		},
	}

	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)
	if got := refreshCalls.Load(); got != 1 {
		t.Fatalf("首轮续期 = %d，期望 1", got)
	}

	if _, err := h.st.DB().ExecContext(context.Background(), `DELETE FROM provider_traffic_settings`); err != nil {
		t.Fatalf("模拟导入清空功能行失败: %v", err)
	}
	h.setTotal(t, true)
	h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-new-refresh")

	if result := h.current(t, ProviderVPS); result != nil {
		t.Fatalf("导入后旧修订结果不得可见: %+v", result)
	}
	results := h.results(t)
	if len(results) != 1 || results[0].Status != ResultWaiting {
		t.Fatalf("导入后应等待新采集: %+v", results)
	}

	h.manualOK(t, ProviderVPS)
	h.waitIdle(t)
	if got := refreshCalls.Load(); got != 2 {
		t.Fatalf("导入后不得复用旧修订 Access Token，实际续期 %d 次", got)
	}
	if got := resourceCalls.Load(); got != 2 {
		t.Fatalf("导入后资源请求 = %d，期望 2", got)
	}
}

func TestCollectorSettingsNoEffectiveChange(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	h.setTotal(t, true)

	var calls atomic.Int32
	h.c.kiwi = &fakeKiwiVM{fetch: func(context.Context, string, string) (TrafficSample, error) {
		calls.Add(1)
		return TrafficSample{Remaining: 1, Unit: UnitGiB}, nil
	}}

	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)
	if got := calls.Load(); got != 1 {
		t.Fatalf("首轮调用 = %d，期望 1", got)
	}

	input := h.bindingInput(t, ProviderKiwiVM, true, "1001", "synthetic-other-key")
	input.Revision = nil
	_, err := h.c.SaveBinding(context.Background(), input)
	mustErrIs(t, err, ErrRevisionConflict)
	h.waitIdle(t)
	if got := calls.Load(); got != 1 {
		t.Fatalf("修订冲突不得触发采集，实际 %d 次", got)
	}

	h.saveBindingAndSchedule(t, ProviderKiwiVM, true, "1001", "")
	h.waitIdle(t)
	if got := calls.Load(); got != 1 {
		t.Fatalf("同值保存不得触发采集，实际 %d 次", got)
	}

	h.setTotalAndSchedule(t, true)
	h.waitIdle(t)
	if got := calls.Load(); got != 1 {
		t.Fatalf("同值总开关不得触发采集，实际 %d 次", got)
	}

	h.saveBindingAndSchedule(t, ProviderKiwiVM, false, "1001", "")
	h.waitIdle(t)
	if got := calls.Load(); got != 1 {
		t.Fatalf("停用不得触发采集，实际 %d 次", got)
	}
	if results := h.results(t); len(results) != 0 {
		t.Fatalf("停用后首页应为空: %+v", results)
	}
	if result := h.mustCurrent(t, ProviderKiwiVM); result.Remaining != 1 {
		t.Fatalf("停用应保留原结果: %+v", result)
	}

	h.saveBindingAndSchedule(t, ProviderKiwiVM, true, "1001", "")
	h.waitIdle(t)
	if got := calls.Load(); got != 2 {
		t.Fatalf("重新启用应触发一次采集，实际 %d 次", got)
	}
}

func TestCollectorSettingsReopenUsesCommittedTransition(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	h.setTotal(t, true)
	var calls atomic.Int32
	h.c.kiwi = &fakeKiwiVM{fetch: func(context.Context, string, string) (TrafficSample, error) {
		calls.Add(1)
		return TrafficSample{Remaining: 1, Unit: UnitGiB}, nil
	}}
	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)

	// 模拟关闭已提交、但采集器没有观察到关闭快照的情况。
	h.setTotal(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	settings, err := h.c.applySettingsChange(func() (settingsChange, error) {
		change, err := h.svc.setEnabled(ctx, true)
		// 本地写入已结束，管理请求取消不得吞掉已提交的变化。
		cancel()
		return change, err
	})
	if err != nil || !settings.Enabled {
		t.Fatalf("重开设置应已提交: enabled=%v err=%v", settings.Enabled, err)
	}
	h.waitIdle(t)
	if got := calls.Load(); got != 2 {
		t.Fatalf("重开应在定时周期前发出一次采集，实际合计 %d 次", got)
	}
}

func TestCollectorManualChainKeepsAdmittedRevision(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	initial := h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	var calls atomic.Int32
	h.c.kiwi = &fakeKiwiVM{fetch: func(context.Context, string, string) (TrafficSample, error) {
		calls.Add(1)
		return TrafficSample{Remaining: 3, Unit: UnitGiB}, nil
	}}
	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)
	h.setTotal(t, true)
	ctx := context.Background()
	host := initial.Providers[0]
	manual := &manualCollection{
		requestCtx: ctx,
		revision:   collectionRevision{binding: host.Revision, edit: host.EditRevision},
		outcome:    make(chan chainOutcome, 1),
	}
	// 手动入口确认原修订后、采集链读取绑定前，有另一笔改绑提交。
	h.saveBinding(t, ProviderKiwiVM, true, "1002", "synthetic-other-kiwivm-key")
	if err := h.c.startHostChain(ctx, legacyKey(ProviderKiwiVM), triggerManual, manual); err != nil {
		t.Fatal(err)
	}
	select {
	case out := <-manual.outcome:
		mustErrIs(t, out.err, ErrDiscarded)
	case <-time.After(5 * time.Second):
		t.Fatal("等待手动采集结束超时")
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("原手动请求不得查询新绑定，实际 %d 次", got)
	}
}

// 两家都已换证或改绑时，旧页面的请求不得触发新凭据的查询或续期。
func TestCollectorManualRejectsStaleRequestedRevision(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-old-key")
	h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-old-refresh")
	kiwiRevision := h.editRevision(t, ProviderKiwiVM)
	vpsRevision := h.editRevision(t, ProviderVPS)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-new-key")
	h.saveBinding(t, ProviderVPS, true, "2003", "synthetic-new-refresh")

	var kiwiCalls, refreshCalls, resourceCalls atomic.Int32
	h.c.kiwi = &fakeKiwiVM{fetch: func(context.Context, string, string) (TrafficSample, error) {
		kiwiCalls.Add(1)
		return TrafficSample{Remaining: 1, Unit: UnitGiB}, nil
	}}
	h.c.vps = &fakeVPS{
		refresh: func(context.Context, string) (TokenPair, error) {
			refreshCalls.Add(1)
			return TokenPair{Token: "synthetic-access", Refresh: "synthetic-rotated"}, nil
		},
		fetch: func(context.Context, string, string) (TrafficSample, error) {
			resourceCalls.Add(1)
			return TrafficSample{Remaining: 1, Unit: UnitGB}, nil
		},
	}
	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)
	// 直接写总开关，让计数只包含待测的手动请求。
	h.setTotal(t, true)
	ctx := context.Background()
	_, err := h.c.ManualRefresh(ctx, ProviderKiwiVM, kiwiRevision)
	mustErrIs(t, err, ErrRevisionConflict)
	_, err = h.c.ManualRefresh(ctx, ProviderVPS, vpsRevision)
	mustErrIs(t, err, ErrRevisionConflict)
	h.waitIdle(t)
	if kiwiCalls.Load() != 0 || refreshCalls.Load() != 0 || resourceCalls.Load() != 0 {
		t.Fatalf("旧修订触发了上游请求: kiwi=%d, refresh=%d, resource=%d",
			kiwiCalls.Load(), refreshCalls.Load(), resourceCalls.Load())
	}
	if h.current(t, ProviderKiwiVM) != nil || h.current(t, ProviderVPS) != nil {
		t.Fatal("旧修订手动请求发布了采集结果")
	}
}

func TestCollectorTotalOnOnlyReadyProviders(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	h.saveBinding(t, ProviderVPS, false, "2002", "synthetic-vps-refresh")
	h.setTotal(t, false)

	var kiwiCalls, vpsRefreshCalls atomic.Int32
	h.c.kiwi = &fakeKiwiVM{fetch: func(context.Context, string, string) (TrafficSample, error) {
		kiwiCalls.Add(1)
		return TrafficSample{Remaining: 1, Unit: UnitGiB}, nil
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
			return TrafficSample{Remaining: 2, Unit: UnitGB}, nil
		},
	}

	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)
	if kiwiCalls.Load() != 0 || vpsRefreshCalls.Load() != 0 {
		t.Fatalf("总开关关闭时不得出站")
	}

	h.setTotalAndSchedule(t, true)
	h.waitIdle(t)
	if got := kiwiCalls.Load(); got != 1 {
		t.Fatalf("总开关开启应只触发已就绪的 KiwiVM，实际 %d 次", got)
	}
	if got := vpsRefreshCalls.Load(); got != 0 {
		t.Fatalf("总开关开启不得触发停用的 V.P.S，实际 %d 次", got)
	}

	h.saveBindingAndSchedule(t, ProviderVPS, true, "2002", "")
	h.waitIdle(t)
	if got := vpsRefreshCalls.Load(); got != 1 {
		t.Fatalf("V.P.S 重新启用后应采集一次，实际 %d 次", got)
	}
	if got := kiwiCalls.Load(); got != 1 {
		t.Fatalf("单独启用 V.P.S 不应重采 KiwiVM，实际 %d 次", got)
	}
}

func TestCollectorSettingsLifecycle(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	var calls atomic.Int32
	h.c.kiwi = &fakeKiwiVM{fetch: func(context.Context, string, string) (TrafficSample, error) {
		calls.Add(1)
		return TrafficSample{Remaining: 1, Unit: UnitGiB}, nil
	}}

	h.setTotalAndSchedule(t, true)
	if h.busy(ProviderKiwiVM) {
		t.Fatal("启动前写入不得安排在途采集")
	}
	h.start(t)
	if err := h.c.Start(context.Background()); err != nil {
		t.Fatalf("重复启动应幂等: %v", err)
	}
	h.waitCycle(t)
	h.waitIdle(t)
	if got := calls.Load(); got != 1 {
		t.Fatalf("启动应采集保存的设置一次，实际 %d 次", got)
	}
	h.c.Stop()
	h.setTotalAndSchedule(t, false)
	h.setTotalAndSchedule(t, true)
	if h.busy(ProviderKiwiVM) || calls.Load() != 1 {
		t.Fatal("停止后不得因设置写入重新启动采集")
	}
}

func TestCollectorNewEnabledBindingOnlyThatProvider(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
	h.setTotal(t, true)

	var kiwiCalls, vpsRefreshCalls atomic.Int32
	h.c.kiwi = &fakeKiwiVM{fetch: func(context.Context, string, string) (TrafficSample, error) {
		kiwiCalls.Add(1)
		return TrafficSample{Remaining: 1, Unit: UnitGiB}, nil
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
			return TrafficSample{Remaining: 2, Unit: UnitGB}, nil
		},
	}

	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)
	if got := kiwiCalls.Load(); got != 1 {
		t.Fatalf("首轮 KiwiVM 调用 = %d，期望 1", got)
	}

	h.saveBindingAndSchedule(t, ProviderVPS, true, "2002", "synthetic-vps-refresh")
	h.waitIdle(t)
	if got := vpsRefreshCalls.Load(); got != 1 {
		t.Fatalf("新建已启用绑定应只触发该家，V.P.S 续期 = %d", got)
	}
	if got := kiwiCalls.Load(); got != 1 {
		t.Fatalf("新建 V.P.S 绑定不应重采 KiwiVM，实际 %d 次", got)
	}
}

func TestCollectorStopDuringManual(t *testing.T) {
	for _, cancelRoot := range []bool{false, true} {
		name := "stop collector"
		if cancelRoot {
			name = "cancel server context"
		}
		t.Run(name, func(t *testing.T) {
			h := newCollectorHarness(t)
			h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-kiwivm-key")
			started := make(chan struct{}, 1)
			h.c.kiwi = &fakeKiwiVM{fetch: func(ctx context.Context, _ string, _ string) (TrafficSample, error) {
				started <- struct{}{}
				<-ctx.Done()
				return TrafficSample{}, ctx.Err()
			}}
			root, cancel := context.WithCancel(context.Background())
			defer cancel()
			if err := h.c.Start(root); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(h.c.Stop)
			h.waitCycle(t)
			h.waitIdle(t)
			h.setTotal(t, true)

			out := h.startManual(t, context.Background(), ProviderKiwiVM)
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("等待手动采集开始超时")
			}
			if cancelRoot {
				cancel()
			} else {
				h.c.Stop()
			}
			outcome := waitManual(t, out)
			mustErrIs(t, outcome.err, ErrDiscarded)
			if outcome.result.Status != "" {
				t.Fatalf("停止时不得返回结果: %+v", outcome.result)
			}
			if result := h.current(t, ProviderKiwiVM); result != nil {
				t.Fatalf("停止后不得发布结果: %+v", result)
			}
		})
	}
}
