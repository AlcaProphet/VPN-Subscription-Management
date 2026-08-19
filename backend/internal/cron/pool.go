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

// StartPoolAutoSync 每分钟检查到期池并触发同步；同一分钟内每池最多触发一次；
// 返回 stop 函数供优雅退出时调用
func StartPoolAutoSync(st *store.Store, poolSvc *pool.Service, lg *slog.Logger) (stop func()) {
	ticker := time.NewTicker(time.Minute)
	done := make(chan struct{})
	var mu sync.Mutex
	lastMinute := ""
	fired := map[int64]bool{}

	run := func() {
		now := time.Now().UTC()
		slot := now.Format("2006-01-02 15:04")
		hhmm := now.Format("15:04")
		mu.Lock()
		if slot != lastMinute {
			lastMinute = slot
			fired = map[int64]bool{} // 新分钟清空判重集合
		}
		mu.Unlock()

		rows, err := st.DB().QueryContext(context.Background(),
			`SELECT id FROM rule_pools WHERE auto_sync = 1 AND sync_time = ?`, hhmm)
		if err != nil {
			lg.Error("查询定时同步素材池失败", "err", err)
			return
		}
		var ids []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return
			}
			ids = append(ids, id)
		}
		_ = rows.Close()

		for _, id := range ids {
			mu.Lock()
			if fired[id] {
				mu.Unlock()
				continue
			}
			fired[id] = true
			mu.Unlock()
			if _, err := poolSvc.SubmitSync(context.Background(), id); err != nil {
				if errors.Is(err, pool.ErrSyncRunning) {
					continue // 手动/定时同步不并发，下一周期再试
				}
				lg.Error("定时同步提交失败", "pool_id", id, "err", err)
			}
		}
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
