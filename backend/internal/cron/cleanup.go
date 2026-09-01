// Package cron 提供后台定时任务：访问日志定期清理。
package cron

import (
	"database/sql"
	"log/slog"
	"time"
)

// StartAccessLogCleanup 访问日志 90 天自动清理（Design1 §3.4.9）：每日巡检一次；
// 返回 stop 函数供优雅退出时调用
func StartAccessLogCleanup(db *sql.DB, lg *slog.Logger) (stop func()) {
	ticker := time.NewTicker(24 * time.Hour)
	done := make(chan struct{})
	go func() {
		// 启动即先清理一次（历史遗留日志）
		cleanupOnce(db, lg)
		for {
			select {
			case <-ticker.C:
				cleanupOnce(db, lg)
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()
	return func() { close(done) }
}

// cleanupOnce 删除 90 天前的访问日志
func cleanupOnce(db *sql.DB, lg *slog.Logger) {
	cutoff := time.Now().AddDate(0, 0, -90).Format("2006-01-02 15:04:05")
	if _, err := db.Exec(`DELETE FROM access_logs WHERE created_at < ?`, cutoff); err != nil {
		lg.Error("清理访问日志失败", "err", err)
	}
}

// StartResetTokenCleanup 每日清理过期/已使用的密码重置令牌。
func StartResetTokenCleanup(db *sql.DB, lg *slog.Logger) (stop func()) {
	ticker := time.NewTicker(24 * time.Hour)
	done := make(chan struct{})
	go func() {
		cleanupResetTokensOnce(db, lg)
		for {
			select {
			case <-ticker.C:
				cleanupResetTokensOnce(db, lg)
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()
	return func() { close(done) }
}

// cleanupResetTokensOnce 删除过期或已使用的密码重置令牌。
func cleanupResetTokensOnce(db *sql.DB, lg *slog.Logger) {
	if _, err := db.Exec(`DELETE FROM password_reset_tokens WHERE expires_at < ? OR used = 1`, time.Now().Format("2006-01-02 15:04:05")); err != nil {
		lg.Error("清理密码重置令牌失败", "err", err)
	}
}
