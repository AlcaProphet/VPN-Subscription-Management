// xray.go：高级模式流量采集定时任务（Build6 Step5）。
package cron

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"vpn-sub/internal/config"
	"vpn-sub/internal/store"
	"vpn-sub/internal/xray"
)

// runXrayCollect 执行一轮采集与配额检查；返回是否真正执行（受间隔与高级模式开关约束）。
// lastRun 由调用方持有并在执行成功后更新，便于测试与定时任务共用。
func runXrayCollect(st *store.Store, instSvc *xray.InstanceService, syncSvc *xray.SyncService, extSvc *xray.ExtService, cfg *config.Service, lg *slog.Logger, lastRun *time.Time) bool {
	interval := cfg.GetInt(context.Background(), "xray_collect_interval_minutes", 10)
	if interval < 1 {
		interval = 1
	}
	if !lastRun.IsZero() && time.Since(*lastRun) < time.Duration(interval)*time.Minute {
		return false
	}
	if !cfg.GetBool(context.Background(), config.KeyAdvancedMode, false) {
		return false
	}
	*lastRun = time.Now()
	ctx := context.Background()
	instances, err := instSvc.List(ctx)
	if err != nil {
		lg.Error("读取 Xray 实例列表失败", "err", err)
		return true
	}
	for _, inst := range instances {
		if !inst.Enabled {
			continue
		}
		if err := instSvc.CollectInstance(ctx, inst); err != nil {
			lg.Warn("Xray 实例采集失败", "instance", inst.ID, "err", err)
			continue
		}
		if err := extSvc.CollectExtTraffic(ctx, inst); err != nil {
			lg.Warn("Xray 独立账号采集失败", "instance", inst.ID, "err", err)
		}
	}
	// 配额检查（面板用户 + 独立账号）
	ids, err := syncSvc.ActiveUserIDs(ctx)
	if err != nil {
		lg.Error("读取 active 用户失败", "err", err)
		return true
	}
	for _, id := range ids {
		if err := syncSvc.CheckQuota(ctx, id); err != nil {
			lg.Warn("配额检查失败", "user_id", id, "err", err)
		}
	}
	if err := extSvc.CheckAllExtQuota(ctx); err != nil {
		lg.Warn("独立账号配额检查失败", "err", err)
	}
	return true
}

// StartXrayCollect 每分钟检查，按 xray_collect_interval_minutes 间隔执行采集与配额检查。
// 返回 stop 函数供优雅退出时调用。
func StartXrayCollect(st *store.Store, instSvc *xray.InstanceService, syncSvc *xray.SyncService, extSvc *xray.ExtService, cfg *config.Service, lg *slog.Logger) (stop func()) {
	ticker := time.NewTicker(time.Minute)
	done := make(chan struct{})
	var mu sync.Mutex
	var lastRun time.Time

	run := func() {
		mu.Lock()
		defer mu.Unlock()
		runXrayCollect(st, instSvc, syncSvc, extSvc, cfg, lg, &lastRun)
	}

	go func() {
		for {
			select {
			case <-ticker.C:
				run()
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()
	return func() { close(done) }
}
