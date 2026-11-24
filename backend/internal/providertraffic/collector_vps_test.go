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

func TestCollectorVPSTokenExpiresDuringEligibilityRead(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	settings := h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-original-refresh")
	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)
	h.setTotal(t, true)
	binding, _ := bindingFromSettings(settings, ProviderVPS)
	expiry := h.clock.Now().Add(time.Minute)
	oldAccess := syntheticJWTValue(t, map[string]any{"exp": expiry.Unix()})
	newAccess := syntheticJWTValue(t, map[string]any{"exp": expiry.Add(time.Hour).Unix()})
	h.c.cacheToken(binding.revision, oldAccess)

	var refreshes, queries atomic.Int32
	h.c.vps = &fakeVPS{
		refresh: func(_ context.Context, refresh string) (TokenPair, error) {
			refreshes.Add(1)
			if refresh != "synthetic-original-refresh" {
				t.Error("续期应使用已保存的 Refresh Token")
			}
			return TokenPair{Token: newAccess, Refresh: "synthetic-rotated-refresh"}, nil
		},
		fetch: func(ctx context.Context, _, access string) (TrafficSample, error) {
			queries.Add(1)
			if access != newAccess {
				return TrafficSample{}, &UpstreamError{Category: UpstreamAccessTokenRejected}
			}
			_, _, refresh, err := h.svc.LoadCredential(ctx, ProviderVPS)
			if err != nil || refresh != "synthetic-rotated-refresh" {
				t.Errorf("查询资源前应已提交轮换结果: %v", err)
			}
			return TrafficSample{Remaining: 8, Unit: UnitGB}, nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	db := h.st.DB()
	out := make(chan manualOutcome, 1)
	func() {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := conn.Close(); err != nil {
				t.Error(err)
			}
		}()
		waitCount := db.Stats().WaitCount
		go func() {
			result, err := h.c.collectVPS(ctx, ctx, binding, "synthetic-original-refresh")
			out <- manualOutcome{result: result, err: err}
		}()
		waitForCondition(t, 5*time.Second, func() bool { return db.Stats().WaitCount > waitCount }, "出站资格复核等待数据库连接")
		h.clock.Advance(time.Minute)
	}()
	outcome := waitManual(t, out)
	if outcome.err != nil {
		t.Fatal(outcome.err)
	}
	assertOK(t, outcome.result, 8, UnitGB)
	if refreshes.Load() != 1 || queries.Load() != 1 {
		t.Fatalf("到期后应只续期和查询各一次: refresh=%d resources=%d", refreshes.Load(), queries.Load())
	}
}

func TestCollectorVPSTokenCacheReuseExpiryAndRebind(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	settings := h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-refresh-1")
	h.setTotal(t, true)
	firstRevision := settings.Providers[1].Revision

	var refreshCalls, resourceCalls atomic.Int32
	var mu sync.Mutex
	var usedTokens []string
	h.c.vps = &fakeVPS{
		refresh: func(context.Context, string) (TokenPair, error) {
			n := refreshCalls.Add(1)
			token := syntheticJWTValue(t, map[string]any{
				"exp": h.clock.Now().Add(time.Hour).Unix(),
				"jti": fmt.Sprintf("synthetic-token-%d", n),
			})
			return TokenPair{Token: token, Refresh: fmt.Sprintf("synthetic-refresh-%d", n+1)}, nil
		},
		fetch: func(_ context.Context, _ string, accessToken string) (TrafficSample, error) {
			resourceCalls.Add(1)
			mu.Lock()
			usedTokens = append(usedTokens, accessToken)
			mu.Unlock()
			return TrafficSample{Remaining: 3.5, Unit: UnitGB}, nil
		},
	}

	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)
	h.manualOK(t, ProviderVPS)
	h.waitIdle(t)
	if got := refreshCalls.Load(); got != 1 {
		t.Fatalf("未过期 Token 不应续期，实际续期 %d 次", got)
	}
	if got := resourceCalls.Load(); got != 2 {
		t.Fatalf("V.P.S 资源调用 = %d，期望 2", got)
	}
	mu.Lock()
	if len(usedTokens) != 2 || usedTokens[0] != usedTokens[1] {
		t.Fatalf("未过期 Token 应复用: %v", usedTokens)
	}
	mu.Unlock()

	h.clock.Advance(2 * time.Hour)
	h.manualOK(t, ProviderVPS)
	h.waitIdle(t)
	if got := refreshCalls.Load(); got != 2 {
		t.Fatalf("过期 Token 应重新续期，实际续期 %d 次", got)
	}
	mu.Lock()
	if len(usedTokens) != 3 || usedTokens[2] == usedTokens[0] {
		t.Fatalf("过期后应使用新 Access Token: %v", usedTokens)
	}
	mu.Unlock()

	settings = h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-refresh-new")
	if settings.Providers[1].Revision == firstRevision {
		t.Fatalf("换证后修订不应复用: %q", firstRevision)
	}
	h.manualOK(t, ProviderVPS)
	h.waitIdle(t)
	if got := refreshCalls.Load(); got != 3 {
		t.Fatalf("换证后不得复用旧修订 Token，实际续期 %d 次", got)
	}
}

func TestCollectorVPSTokenWithoutExpiryOnlyThisRound(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-refresh-1")
	h.setTotal(t, true)

	var refreshCalls, resourceCalls atomic.Int32
	h.c.vps = &fakeVPS{
		refresh: func(context.Context, string) (TokenPair, error) {
			n := refreshCalls.Add(1)
			token := syntheticJWTValue(t, map[string]any{"sub": "synthetic-without-exp"})
			return TokenPair{Token: token, Refresh: fmt.Sprintf("synthetic-refresh-%d", n+1)}, nil
		},
		fetch: func(context.Context, string, string) (TrafficSample, error) {
			resourceCalls.Add(1)
			return TrafficSample{Remaining: 4, Unit: UnitGB}, nil
		},
	}

	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)
	h.manualOK(t, ProviderVPS)
	h.waitIdle(t)
	if got := refreshCalls.Load(); got != 2 {
		t.Fatalf("无有效到期时间的 Token 只供本轮，实际续期 %d 次", got)
	}
	if got := resourceCalls.Load(); got != 2 {
		t.Fatalf("V.P.S 资源调用 = %d，期望 2", got)
	}
}

func TestCollectorVPSAccessTokenRejectedClearsCache(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-refresh-1")
	h.setTotal(t, true)

	var refreshCalls, resourceCalls atomic.Int32
	h.c.vps = &fakeVPS{
		refresh: func(context.Context, string) (TokenPair, error) {
			n := refreshCalls.Add(1)
			token := syntheticJWTValue(t, map[string]any{
				"exp": h.clock.Now().Add(time.Hour).Unix(),
				"jti": fmt.Sprintf("synthetic-token-%d", n),
			})
			return TokenPair{Token: token, Refresh: fmt.Sprintf("synthetic-refresh-%d", n+1)}, nil
		},
		fetch: func(context.Context, string, string) (TrafficSample, error) {
			switch resourceCalls.Add(1) {
			case 1:
				return TrafficSample{Remaining: 5, Unit: UnitGB}, nil
			case 2:
				return TrafficSample{}, &UpstreamError{Category: UpstreamAccessTokenRejected}
			default:
				return TrafficSample{Remaining: 6, Unit: UnitGB}, nil
			}
		},
	}

	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)

	result, err := h.manual(t, ProviderVPS)
	if err != nil {
		t.Fatalf("Access Token 被拒绝应作为平台结果返回: %v", err)
	}
	assertErrorResult(t, result, "服务商鉴权失败")
	if result.Hint == "" {
		t.Fatal("Access Token 被拒绝应带安全提示")
	}
	if got := refreshCalls.Load(); got != 1 {
		t.Fatalf("Access Token 被拒绝后本轮不得重试续期，实际续期 %d 次", got)
	}

	h.manualOK(t, ProviderVPS)
	h.waitIdle(t)
	if got := refreshCalls.Load(); got != 2 {
		t.Fatalf("缓存清除后下一轮应重新续期，实际续期 %d 次", got)
	}
	if got := resourceCalls.Load(); got != 3 {
		t.Fatalf("V.P.S 资源调用 = %d，期望 3", got)
	}
}

func TestCollectorVPSCommitBeforeResource(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	settings := h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-original-refresh")
	h.setTotal(t, true)
	revision := settings.Providers[1].Revision
	accessToken := syntheticJWTValue(t, map[string]any{"exp": h.clock.Now().Add(time.Hour).Unix()})

	var mu sync.Mutex
	var refreshInput, usedAccessToken, storedCredential, storedRevision, storedInstance string
	var storedErr error
	h.c.vps = &fakeVPS{
		refresh: func(_ context.Context, refreshToken string) (TokenPair, error) {
			mu.Lock()
			refreshInput = refreshToken
			mu.Unlock()
			return TokenPair{Token: accessToken, Refresh: "synthetic-rotated-refresh"}, nil
		},
		fetch: func(ctx context.Context, instanceID, token string) (TrafficSample, error) {
			instance, currentRevision, credential, err := h.svc.LoadCredential(ctx, ProviderVPS)
			mu.Lock()
			usedAccessToken = token
			storedInstance = instance
			storedRevision = currentRevision
			storedCredential = credential
			storedErr = err
			mu.Unlock()
			return TrafficSample{Remaining: 8, Unit: UnitGB}, nil
		},
	}

	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)

	mu.Lock()
	defer mu.Unlock()
	if storedErr != nil {
		t.Fatalf("资源查询前读取绑定失败: %v", storedErr)
	}
	if refreshInput != "synthetic-original-refresh" {
		t.Fatalf("续期输入 = %q，期望原 Refresh Token", refreshInput)
	}
	if storedCredential != "synthetic-rotated-refresh" {
		t.Fatalf("资源查询前应先提交新 Refresh Token，实际库内为 %q", storedCredential)
	}
	if storedRevision != revision || storedInstance != "2002" {
		t.Fatalf("提交后修订或实例错误: revision=%q instance=%q", storedRevision, storedInstance)
	}
	if usedAccessToken != accessToken {
		t.Fatalf("资源查询应使用新 Access Token")
	}
}

func TestCollectorVPSCommitFailureNoResource(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-original-refresh")
	h.setTotal(t, true)

	const triggerName = "synthetic_vps_credential_failure"
	if _, err := h.st.DB().ExecContext(context.Background(), `
		CREATE TRIGGER `+triggerName+`
		BEFORE UPDATE OF vps_credential_ciphertext ON provider_traffic_settings
		BEGIN
			SELECT RAISE(ABORT, 'synthetic credential update failure');
		END;`); err != nil {
		t.Fatalf("创建故障注入触发器失败: %v", err)
	}

	var refreshCalls, resourceCalls atomic.Int32
	h.c.vps = &fakeVPS{
		refresh: func(context.Context, string) (TokenPair, error) {
			n := refreshCalls.Add(1)
			return TokenPair{
				Token:   syntheticJWTValue(t, map[string]any{"exp": h.clock.Now().Add(time.Hour).Unix()}),
				Refresh: fmt.Sprintf("synthetic-rotated-refresh-%d", n),
			}, nil
		},
		fetch: func(context.Context, string, string) (TrafficSample, error) {
			resourceCalls.Add(1)
			return TrafficSample{Remaining: 9, Unit: UnitGB}, nil
		},
	}

	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)

	result := h.mustCurrent(t, ProviderVPS)
	assertErrorResult(t, result, "服务器流量采集失败")
	if result.Hint == "" {
		t.Fatal("续期入库失败应带安全提示")
	}
	if got := refreshCalls.Load(); got != 1 {
		t.Fatalf("续期调用 = %d，期望 1", got)
	}
	if got := resourceCalls.Load(); got != 0 {
		t.Fatalf("续期入库失败后资源请求必须为 0，实际 %d", got)
	}
	if _, _, credential, err := h.svc.LoadCredential(context.Background(), ProviderVPS); err != nil {
		t.Fatalf("读取故障后的绑定失败: %v", err)
	} else if credential != "synthetic-original-refresh" {
		t.Fatalf("入库失败不得替换原 Refresh Token，实际 %q", credential)
	}

	if _, err := h.st.DB().ExecContext(context.Background(), `DROP TRIGGER `+triggerName); err != nil {
		t.Fatalf("删除故障注入触发器失败: %v", err)
	}
	h.manualOK(t, ProviderVPS)
	h.waitIdle(t)
	if got := refreshCalls.Load(); got != 2 {
		t.Fatalf("故障恢复后应重新续期，实际续期 %d 次", got)
	}
	if got := resourceCalls.Load(); got != 1 {
		t.Fatalf("故障恢复后资源请求 = %d，期望 1", got)
	}
}

func TestCollectorVPSZeroRowsDiscardsRotatedToken(t *testing.T) {
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
				Refresh: "synthetic-rotated-old-revision",
			}, nil
		},
		fetch: func(context.Context, string, string) (TrafficSample, error) {
			t.Errorf("旧修订不得发起资源请求")
			return TrafficSample{}, errors.New("旧修订不得发起资源请求")
		},
	}

	h.start(t)
	h.waitCycle(t)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("等待续期开始超时")
	}

	settings := h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-new-refresh")
	close(release)
	h.waitIdle(t)

	if h.current(t, ProviderVPS) != nil {
		t.Fatal("旧修订不得发布结果")
	}
	results := h.results(t)
	if len(results) != 1 || results[0].Provider != ProviderVPS || results[0].Status != ResultWaiting {
		t.Fatalf("新绑定应回到待采集: %+v", results)
	}
	if _, _, credential, err := h.svc.LoadCredential(context.Background(), ProviderVPS); err != nil {
		t.Fatalf("读取新绑定失败: %v", err)
	} else if credential != "synthetic-new-refresh" {
		t.Fatalf("旧修订轮换结果写入了新绑定: %q", credential)
	}
	if settings.Providers[1].Revision == "" {
		t.Fatal("新绑定修订为空")
	}
}

func TestCollectorVPSCommitSurvivesCancellation(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-original-refresh")
	h.setTotal(t, false)
	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)

	h.setTotal(t, true)
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseFn := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseFn()

	h.c.vps = &fakeVPS{
		refresh: func(context.Context, string) (TokenPair, error) {
			started <- struct{}{}
			<-release // 故意忽略 ctx：模拟已收到并解析完成的新 Token 对
			return TokenPair{
				Token:   syntheticJWTValue(t, map[string]any{"exp": h.clock.Now().Add(time.Hour).Unix()}),
				Refresh: "synthetic-rotated-refresh",
			}, nil
		},
		fetch: func(context.Context, string, string) (TrafficSample, error) {
			t.Errorf("请求取消后不得查询资源")
			return TrafficSample{}, errors.New("请求取消后不得查询资源")
		},
	}

	callerCtx, cancel := context.WithCancel(context.Background())
	out := h.startManual(t, callerCtx, ProviderVPS)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("等待续期开始超时")
	}
	cancel()
	releaseFn()
	outcome := waitManual(t, out)
	if !errors.Is(outcome.err, context.Canceled) {
		t.Fatalf("取消后的手动请求错误 = %v，期望 context.Canceled", outcome.err)
	}
	h.waitIdle(t)

	if _, _, credential, err := h.svc.LoadCredential(context.Background(), ProviderVPS); err != nil {
		t.Fatalf("读取取消后的绑定失败: %v", err)
	} else if credential != "synthetic-rotated-refresh" {
		t.Fatalf("已收到的续期结果仍应提交，实际 %q", credential)
	}
	if h.current(t, ProviderVPS) != nil {
		t.Fatal("请求取消后不得发布结果")
	}
}

func TestCollectorVPSTotalOffAfterRotationCommit(t *testing.T) {
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
			t.Errorf("关闭功能后不得查询资源")
			return TrafficSample{}, errors.New("关闭功能后不得查询资源")
		},
	}

	h.start(t)
	h.waitCycle(t)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("等待续期开始超时")
	}
	h.setTotal(t, false)
	close(release)
	h.waitIdle(t)

	if _, _, credential, err := h.svc.LoadCredential(context.Background(), ProviderVPS); err != nil {
		t.Fatalf("读取关闭后的绑定失败: %v", err)
	} else if credential != "synthetic-rotated-refresh" {
		t.Fatalf("同一绑定关闭功能后仍应提交新 Refresh Token，实际 %q", credential)
	}
	if h.current(t, ProviderVPS) != nil {
		t.Fatal("关闭功能后不得发布结果")
	}
	if results := h.results(t); len(results) != 0 {
		t.Fatalf("总开关关闭时首页应为空: %+v", results)
	}
}

func TestCollectorVPSRefreshRejectedNoResource(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-refresh")
	h.setTotal(t, true)

	var resourceCalls atomic.Int32
	h.c.vps = &fakeVPS{
		refresh: func(context.Context, string) (TokenPair, error) {
			return TokenPair{}, &UpstreamError{Category: UpstreamCredentialRejected}
		},
		fetch: func(context.Context, string, string) (TrafficSample, error) {
			resourceCalls.Add(1)
			return TrafficSample{Remaining: 1, Unit: UnitGB}, nil
		},
	}

	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)

	result := h.mustCurrent(t, ProviderVPS)
	assertErrorResult(t, result, "服务商凭据被拒绝")
	if result.Hint == "" {
		t.Fatal("凭据被拒绝应带安全提示")
	}
	if got := resourceCalls.Load(); got != 0 {
		t.Fatalf("续期失败后资源请求必须为 0，实际 %d", got)
	}
}

func TestCollectorManualDiscardedOnRebind(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-original-refresh")
	h.setTotal(t, false)
	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)
	h.setTotal(t, true)

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseFn := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseFn()

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
				Refresh: "synthetic-rotated-old-revision",
			}, nil
		},
		fetch: func(context.Context, string, string) (TrafficSample, error) {
			t.Errorf("手动采集改绑后不得查询资源")
			return TrafficSample{}, errors.New("手动采集改绑后不得查询资源")
		},
	}

	out := h.startManual(t, context.Background(), ProviderVPS)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("等待手动续期开始超时")
	}
	settings := h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-new-refresh")
	releaseFn()
	outcome := waitManual(t, out)
	if !errors.Is(outcome.err, ErrDiscarded) {
		t.Fatalf("改绑后的手动结果 = %v，期望 ErrDiscarded", outcome.err)
	}
	h.waitIdle(t)
	if outcome.result.Status != "" {
		t.Fatalf("被丢弃的手动轮次不应返回结果: %+v", outcome.result)
	}
	if result := h.current(t, ProviderVPS); result != nil {
		t.Fatalf("改绑后不得发布旧结果: %+v", result)
	}
	if _, _, credential, err := h.svc.LoadCredential(context.Background(), ProviderVPS); err != nil {
		t.Fatalf("读取改绑后的凭据失败: %v", err)
	} else if credential != "synthetic-new-refresh" {
		t.Fatalf("旧修订不得写回凭据，实际 %q", credential)
	}
	if settings.Providers[1].Revision == "" {
		t.Fatal("改绑后修订不应为空")
	}
}

func TestCollectorVPSAdvancedModeAfterRotationCommit(t *testing.T) {
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
			t.Errorf("切换高级模式后不得查询资源")
			return TrafficSample{}, errors.New("切换高级模式后不得查询资源")
		},
	}

	h.start(t)
	h.waitCycle(t)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("等待续期开始超时")
	}
	h.setAdvanced(t, true)
	close(release)
	h.waitIdle(t)

	if _, _, credential, err := h.svc.LoadCredential(context.Background(), ProviderVPS); err != nil {
		t.Fatalf("读取切换模式后的绑定失败: %v", err)
	} else if credential != "synthetic-rotated-refresh" {
		t.Fatalf("同一绑定切换模式后仍应提交新 Refresh Token，实际 %q", credential)
	}
	if result := h.current(t, ProviderVPS); result != nil {
		t.Fatalf("切换高级模式后不得发布结果: %+v", result)
	}
	if results := h.results(t); len(results) != 0 {
		t.Fatalf("高级模式首页应为空: %+v", results)
	}
}

func TestCollectorVPSRebindClearsCachedState(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	first := h.saveBinding(t, ProviderVPS, true, "2002", "synthetic-refresh-1")
	h.setTotal(t, true)

	var refreshCalls, resourceCalls atomic.Int32
	var mu sync.Mutex
	var refreshInputs []string
	h.c.vps = &fakeVPS{
		refresh: func(_ context.Context, refreshToken string) (TokenPair, error) {
			n := refreshCalls.Add(1)
			mu.Lock()
			refreshInputs = append(refreshInputs, refreshToken)
			mu.Unlock()
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
			return TrafficSample{Remaining: 4, Unit: UnitGB}, nil
		},
	}

	h.start(t)
	h.waitCycle(t)
	h.waitIdle(t)
	if got := refreshCalls.Load(); got != 1 {
		t.Fatalf("首轮续期 = %d，期望 1", got)
	}
	if result := h.mustCurrent(t, ProviderVPS); result.revision != first.Providers[1].Revision {
		t.Fatalf("首轮结果修订 = %q，期望 %q", result.revision, first.Providers[1].Revision)
	}

	second := h.saveBindingAndSchedule(t, ProviderVPS, true, "2002", "synthetic-refresh-2")
	h.waitIdle(t)
	if got := refreshCalls.Load(); got != 2 {
		t.Fatalf("换证后应重新续期，实际 %d 次", got)
	}
	if got := resourceCalls.Load(); got != 2 {
		t.Fatalf("换证后资源请求 = %d，期望 2", got)
	}
	mu.Lock()
	if len(refreshInputs) != 2 || refreshInputs[1] != "synthetic-refresh-2" {
		t.Fatalf("换证后应使用新 Refresh Token: %v", refreshInputs)
	}
	mu.Unlock()
	result := h.mustCurrent(t, ProviderVPS)
	assertOK(t, result, 4, UnitGB)
	if result.revision != second.Providers[1].Revision {
		t.Fatalf("换证后结果修订 = %q，期望 %q", result.revision, second.Providers[1].Revision)
	}
}
