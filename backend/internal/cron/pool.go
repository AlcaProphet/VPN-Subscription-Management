// pool.go：规则素材池可选每日定时同步（UTC，停机错过不补跑，Design2 §2.4）
package cron

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"vpn-sub/internal/pool"
	"vpn-sub/internal/store"
)

// poolAutoSyncState 保存同一分钟内每池判重状态。
type poolAutoSyncState struct {
	mu         sync.Mutex
	lastMinute string
	fired      map[int64]bool
}

// poolAutoSyncDeps 抽象定时同步依赖，便于单元测试。
type poolAutoSyncDeps struct {
	queryDue func(ctx context.Context, hhmm string) ([]int64, error)
	submit   func(ctx context.Context, id int64) error
}

// runPoolAutoSync 执行一次定时同步检查：按当前分钟查到期池，同一分钟内每池最多提交一次。
func runPoolAutoSync(ctx context.Context, deps poolAutoSyncDeps, now time.Time, state *poolAutoSyncState, lg *slog.Logger) {
	slot := now.UTC().Format("2006-01-02 15:04")
	hhmm := now.UTC().Format("15:04")
	state.mu.Lock()
	if slot != state.lastMinute {
		state.lastMinute = slot
		state.fired = map[int64]bool{} // 新分钟清空判重集合
	}
	state.mu.Unlock()

	ids, err := deps.queryDue(ctx, hhmm)
	if err != nil {
		lg.Error("查询定时同步素材池失败", "err", err)
		return
	}
	for _, id := range ids {
		state.mu.Lock()
		if state.fired[id] {
			state.mu.Unlock()
			continue
		}
		state.fired[id] = true
		state.mu.Unlock()
		if err := deps.submit(ctx, id); err != nil {
			if errors.Is(err, pool.ErrSyncRunning) {
				continue // 手动/定时同步不并发，下一周期再试
			}
			lg.Error("定时同步提交失败", "pool_id", id, "err", err)
		}
	}
}

// StartPoolAutoSync 每分钟检查到期池并触发同步；同一分钟内每池最多触发一次；
// 返回 stop 函数供优雅退出时调用
func StartPoolAutoSync(st *store.Store, poolSvc *pool.Service, lg *slog.Logger) (stop func()) {
	ticker := time.NewTicker(time.Minute)
	done := make(chan struct{})
	state := &poolAutoSyncState{fired: map[int64]bool{}}
	deps := poolAutoSyncDeps{
		queryDue: func(ctx context.Context, hhmm string) ([]int64, error) {
			rows, err := st.DB().QueryContext(ctx,
				`SELECT id FROM rule_pools WHERE auto_sync = 1 AND sync_time = ?`, hhmm)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			var ids []int64
			for rows.Next() {
				var id int64
				if err := rows.Scan(&id); err != nil {
					return nil, err
				}
				ids = append(ids, id)
			}
			return ids, rows.Err()
		},
		submit: func(ctx context.Context, id int64) error {
			_, err := poolSvc.SubmitSync(ctx, id)
			return err
		},
	}

	run := func() {
		runPoolAutoSync(context.Background(), deps, time.Now().UTC(), state, lg)
	}

	go func() {
		run() // 启动即检查一次当前分钟（进程启动于整点前后也不遗漏，未到点无匹配）
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
