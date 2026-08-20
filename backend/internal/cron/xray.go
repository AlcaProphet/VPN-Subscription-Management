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

// StartXrayCollect 每分钟检查，按 xray_collect_interval_minutes 间隔执行采集与配额检查。
// 返回 stop 函数供优雅退出时调用。
func StartXrayCollect(st *store.Store, instSvc *xray.InstanceService, syncSvc *xray.SyncService, cfg *config.Service, lg *slog.Logger) (stop func()) {
	ticker := time.NewTicker(time.Minute)
	done := make(chan struct{})
	var mu sync.Mutex
	var lastRun time.Time

	run := func() {
		mu.Lock()
		defer mu.Unlock()
		interval := cfg.GetInt(context.Background(), "xray_collect_interval_minutes", 10)
		if interval < 1 {
			interval = 1
		}
		if !lastRun.IsZero() && time.Since(lastRun) < time.Duration(interval)*time.Minute {
			return
		}
		if !cfg.GetBool(context.Background(), config.KeyAdvancedMode, false) {
			return
		}
		lastRun = time.Now()
		ctx := context.Background()
		instances, err := instSvc.List(ctx)
		if err != nil {
			lg.Error("读取 Xray 实例列表失败", "err", err)
			return
		}
		for _, inst := range instances {
			if !inst.Enabled {
				continue
			}
			if err := instSvc.CollectInstance(ctx, inst); err != nil {
				lg.Warn("Xray 实例采集失败", "instance", inst.ID, "err", err)
				continue
			}
		}
		// 配额检查
		ids, err := syncSvc.ActiveUserIDs(ctx)
		if err != nil {
			lg.Error("读取 active 用户失败", "err", err)
			return
		}
		for _, id := range ids {
			if err := syncSvc.CheckQuota(ctx, id); err != nil {
				lg.Warn("配额检查失败", "user_id", id, "err", err)
			}
		}
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
