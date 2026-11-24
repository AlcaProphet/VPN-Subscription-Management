package providertraffic

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCollectorMultipleHostsAndHalfRemaining(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	ctx := context.Background()
	var kiwiQueries, tokenRefreshes, vpsQueries atomic.Int32
	h.c.kiwi = &fakeKiwiVM{fetch: func(_ context.Context, instance, _ string) (TrafficSample, error) {
		kiwiQueries.Add(1)
		if instance == "1002" {
			return TrafficSample{}, &UpstreamError{Category: UpstreamUnavailable}
		}
		return TrafficSample{Remaining: 101.5, Unit: UnitGiB}, nil
	}}
	access := syntheticJWTValue(t, map[string]any{"exp": h.clock.Now().Add(time.Hour).Unix()})
	h.c.vps = &fakeVPS{
		refresh: func(_ context.Context, credential string) (TokenPair, error) {
			tokenRefreshes.Add(1)
			if credential != "synthetic-refresh" {
				t.Error("首次续期应使用已配置的账号凭据")
			}
			return TokenPair{Token: access, Refresh: "synthetic-rotated"}, nil
		},
		fetch: func(ctx context.Context, instance, token string) (TrafficSample, error) {
			vpsQueries.Add(1)
			settings, err := h.svc.GetSettings(ctx)
			if err != nil {
				t.Errorf("读取账号设置失败: %v", err)
				return TrafficSample{}, err
			}
			credential, err := h.svc.loadCredential(ctx, legacyKey(ProviderVPS), settings.VPSRevision)
			if err != nil || credential != "synthetic-rotated" || token != access {
				t.Errorf("两台主机均应使用同一轮续期且先落库: %v", err)
			}
			remaining := 200.0
			if instance == "2002" {
				remaining = 0
			}
			return TrafficSample{Remaining: remaining, Unit: UnitGB}, nil
		},
	}
	_, err := h.c.SaveVPSAccount(ctx, "", "synthetic-refresh")
	if err != nil {
		t.Fatal(err)
	}
	kiwiCredential := CredentialInput{Value: "synthetic-kiwi-key"}
	omitted := CredentialInput{Omitted: true}
	var settings Settings
	for _, in := range []SaveHostInput{
		{Provider: ProviderKiwiVM, Name: "JP01", InstanceID: "1001", Enabled: true, Credential: kiwiCredential},
		{Provider: ProviderKiwiVM, Name: "JP02", InstanceID: "1002", Enabled: true, Credential: kiwiCredential},
		{Provider: ProviderVPS, Name: "VPS01", InstanceID: "2001", Enabled: true, HalfRemaining: true, Credential: omitted},
		{Provider: ProviderVPS, Name: "VPS02", InstanceID: "2002", Enabled: true, HalfRemaining: true, Credential: omitted},
	} {
		settings, err = h.c.SaveHost(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(settings.Hosts) != 4 {
		t.Fatalf("应保存四台主机: %+v", settings.Hosts)
	}
	h.setTotal(t, true)
	h.start(t)
	h.waitCycle(t)
	waitForCondition(t, 5*time.Second, func() bool {
		results, err := h.c.Results(ctx)
		return err == nil && len(results) == 4 && results[0].Status == ResultOK && results[1].Status == ResultError &&
			results[2].Status == ResultOK && results[3].Status == ResultOK
	}, "四台主机独立完成采集")
	results, err := h.c.Results(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertOK(t, results[0], 101.5, UnitGiB)
	assertOK(t, results[2], 100, UnitGB)
	assertOK(t, results[3], 0, UnitGB)
	if tokenRefreshes.Load() != 1 || kiwiQueries.Load() != 2 || vpsQueries.Load() != 2 {
		t.Fatal("每台主机应查询一次，同一 V.PS 账号应续期一次")
	}

	first := settings.Hosts[0]
	changed, err := h.c.SaveHost(ctx, SaveHostInput{
		Provider: first.Provider, ID: first.ID, Name: "东京 JP01", InstanceID: first.InstanceID,
		Enabled: true, HalfRemaining: true, Revision: &first.EditRevision, Credential: omitted,
	})
	if err != nil {
		t.Fatal(err)
	}
	display := h.c.ResultForBinding(changed.Hosts[0])
	if display == nil || display.Name != "东京 JP01" || !display.HalfRemaining || display.Remaining != 50.75 ||
		!display.SampledAt.Equal(results[0].SampledAt) || kiwiQueries.Load() != 2 {
		t.Fatalf("修改展示设置应直接复用原始采集样本: %+v", display)
	}
	_, err = h.c.RefreshHost(ctx, first.Provider, first.ID, first.EditRevision)
	mustErrIs(t, err, ErrRevisionConflict)
	result, err := h.c.RefreshHost(ctx, first.Provider, first.ID, changed.Hosts[0].EditRevision)
	if err != nil {
		t.Fatal(err)
	}
	assertOK(t, result, 50.75, UnitGiB)
	_, err = h.c.SaveHost(ctx, SaveHostInput{
		Provider: ProviderKiwiVM, Name: "重复主机", InstanceID: first.InstanceID, Credential: kiwiCredential,
	})
	mustErrIs(t, err, ErrDuplicateInstance)
	_, err = h.c.SaveVPSAccount(ctx, settings.VPSRevision, "")
	mustErrIs(t, err, ErrVPSHostsExist)
}

func TestCollectorManualHostEditDiscardsQueuedAndInFlightResults(t *testing.T) {
	for _, phase := range []string{"等待平台槽位", "查询资源中"} {
		t.Run(phase, func(t *testing.T) {
			h := newCollectorHarness(t)
			h.setConfigured(t, true)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			host := h.saveBinding(t, ProviderKiwiVM, true, "1001", "synthetic-key").Hosts[0]
			var calls atomic.Int32
			queryStarted := make(chan struct{})
			releaseQuery := make(chan struct{})
			unblock := sync.OnceFunc(func() { close(releaseQuery) })
			h.c.kiwi = &fakeKiwiVM{fetch: func(ctx context.Context, _, _ string) (TrafficSample, error) {
				switch calls.Add(1) {
				case 1:
					return TrafficSample{Remaining: 100, Unit: UnitGiB}, nil
				case 2:
					if phase == "查询资源中" {
						close(queryStarted)
						select {
						case <-releaseQuery:
						case <-ctx.Done():
							return TrafficSample{}, ctx.Err()
						}
					}
				}
				return TrafficSample{Remaining: 200, Unit: UnitGiB}, nil
			}}
			h.start(t)
			defer unblock()
			h.waitCycle(t)
			h.setTotal(t, true)
			previous := h.manualOK(t, ProviderKiwiVM)
			h.clock.Advance(time.Minute)

			slots := h.c.slots[providerIndex(ProviderKiwiVM)]
			releaseSlots := sync.OnceFunc(func() {
				for range cap(slots) {
					<-slots
				}
			})
			if phase == "等待平台槽位" {
				for range cap(slots) {
					slots <- struct{}{}
				}
				defer releaseSlots()
			}
			out := make(chan manualOutcome, 1)
			go func() {
				result, err := h.c.RefreshHost(ctx, host.Provider, host.ID, host.EditRevision)
				out <- manualOutcome{result: result, err: err}
			}()
			if phase == "等待平台槽位" {
				waitForCondition(t, time.Second, func() bool { return h.busy(ProviderKiwiVM) }, "手动采集已排队")
			} else {
				select {
				case <-queryStarted:
				case <-ctx.Done():
					t.Fatal("等待手动资源查询超时")
				}
			}
			changed, err := h.c.SaveHost(ctx, SaveHostInput{
				Provider: host.Provider, ID: host.ID, Name: "JP01", Enabled: true, InstanceID: host.InstanceID,
				HalfRemaining: true, Revision: &host.EditRevision, Credential: CredentialInput{Omitted: true},
			})
			if err != nil {
				t.Fatal(err)
			}
			if phase == "等待平台槽位" {
				releaseSlots()
			}
			unblock()
			mustErrIs(t, waitManual(t, out).err, ErrDiscarded)
			_, err = h.c.ManualRefresh(ctx, host.Provider, host.EditRevision)
			mustErrIs(t, err, ErrRevisionConflict)
			wantCalls := int32(1)
			if phase == "查询资源中" {
				wantCalls++
			}
			if got := calls.Load(); got != wantCalls {
				t.Fatalf("资源请求 = %d，期望 %d", got, wantCalls)
			}

			current := changed.Hosts[0]
			display := h.c.ResultForBinding(current)
			if display == nil || display.Remaining != 50 || !display.SampledAt.Equal(previous.SampledAt) {
				t.Fatalf("应按新设置显示上一份样本及原采集时间: %+v", display)
			}
			result, err := h.c.ManualRefresh(ctx, host.Provider, current.EditRevision)
			if err != nil {
				t.Fatal(err)
			}
			assertOK(t, result, 100, UnitGiB)
		})
	}
}

func TestCollectorVPSReusesRotationAfterInitiatingHostDisabled(t *testing.T) {
	h := newCollectorHarness(t)
	h.setConfigured(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := h.c.SaveVPSAccount(ctx, "", "synthetic-initial"); err != nil {
		t.Fatal(err)
	}
	omitted := CredentialInput{Omitted: true}
	settings, err := h.c.SaveHost(ctx, SaveHostInput{
		Provider: ProviderVPS, Name: "VPS01", InstanceID: "2001", Enabled: true, Credential: omitted,
	})
	if err != nil {
		t.Fatal(err)
	}
	first := settings.Hosts[0]
	settings, err = h.c.SaveHost(ctx, SaveHostInput{
		Provider: ProviderVPS, Name: "VPS02", InstanceID: "2002", Enabled: true, Credential: omitted,
	})
	if err != nil {
		t.Fatal(err)
	}
	second := settings.Hosts[1]
	access := syntheticJWTValue(t, map[string]any{"exp": h.clock.Now().Add(time.Hour).Unix()})
	rotationStarted := make(chan struct{})
	releaseRotation := make(chan struct{})
	unblock := sync.OnceFunc(func() { close(releaseRotation) })
	var refreshes, queries atomic.Int32
	h.c.vps = &fakeVPS{
		refresh: func(ctx context.Context, _ string) (TokenPair, error) {
			if refreshes.Add(1) == 1 {
				close(rotationStarted)
				select {
				case <-releaseRotation:
				case <-ctx.Done():
					return TokenPair{}, ctx.Err()
				}
			}
			return TokenPair{Token: access, Refresh: "synthetic-rotated"}, nil
		},
		fetch: func(ctx context.Context, instance, token string) (TrafficSample, error) {
			queries.Add(1)
			saved, err := h.svc.loadCredential(ctx, second.key(), settings.VPSRevision)
			if err != nil {
				return TrafficSample{}, err
			}
			if instance != second.InstanceID || token != access || saved != "synthetic-rotated" {
				t.Error("应只查询第二台主机，并使用已落库的轮换结果")
			}
			return TrafficSample{Remaining: 100, Unit: UnitGB}, nil
		},
	}
	h.start(t)
	defer unblock()
	h.waitCycle(t)
	h.setTotal(t, true)
	refresh := func(host ProviderBinding) <-chan manualOutcome {
		out := make(chan manualOutcome, 1)
		go func() {
			result, err := h.c.RefreshHost(ctx, host.Provider, host.ID, host.EditRevision)
			out <- manualOutcome{result: result, err: err}
		}()
		return out
	}
	firstRequest := refresh(first)
	select {
	case <-rotationStarted:
	case <-ctx.Done():
		t.Fatal("等待续期开启超时")
	}
	secondRequest := refresh(second)
	_, err = h.c.SaveHost(ctx, SaveHostInput{
		Provider: first.Provider, ID: first.ID, Name: first.Name, InstanceID: first.InstanceID,
		Enabled: false, Revision: &first.EditRevision, Credential: omitted,
	})
	if err != nil {
		t.Fatal(err)
	}
	unblock()
	mustErrIs(t, waitManual(t, firstRequest).err, ErrDiscarded)
	outcome := waitManual(t, secondRequest)
	if outcome.err != nil {
		t.Fatal(outcome.err)
	}
	assertOK(t, outcome.result, 100, UnitGB)
	if refreshes.Load() != 1 || queries.Load() != 1 {
		t.Fatalf("应只续期一次并只查询第二台主机: 续期 %d，查询 %d", refreshes.Load(), queries.Load())
	}
}
