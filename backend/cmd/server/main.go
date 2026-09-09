// 程序入口：环境变量解析、日志初始化、数据库迁移、路由装配与优雅退出。
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"vpn-sub/internal/config"
	"vpn-sub/internal/cron"
	"vpn-sub/internal/dataclear"
	"vpn-sub/internal/emergency"
	"vpn-sub/internal/log"
	"vpn-sub/internal/pool"
	"vpn-sub/internal/proxytrust"
	"vpn-sub/internal/server"
	"vpn-sub/internal/store"
	"vpn-sub/internal/user"
	"vpn-sub/internal/version"
	"vpn-sub/migrations"
)

func main() {
	// 环境变量：APP_MODE(dev|prod 默认 prod)、LOG_LEVEL(默认 info)、LOG_FORMAT(默认 console)、
	// PORT(默认 8080)、TRUST_PROXY(默认 auto)、DATA_DIR(默认 ./data)、
	// RESET_ADMIN_PASSWORD（Build3 Step 6：设置后启动即进入应急模式）
	mode := envOr("APP_MODE", "prod")
	if mode != "dev" && mode != "prod" {
		fmt.Fprintln(os.Stderr, "APP_MODE 仅支持 dev|prod")
		os.Exit(1)
	}
	// 实时日志流：环形缓冲（最近 500 条）接入统一日志管道（stdout + 缓冲并存，Build3 Step 5）
	logBuf := log.NewRingBuffer()
	logger := log.New(envOr("LOG_LEVEL", "info"), envOr("LOG_FORMAT", "console"), logBuf)
	log.SetDefault(logger)
	streamSvc := log.NewStreamService(logBuf, logger)

	trustPolicy, err := proxytrust.Parse(envOr("TRUST_PROXY", "auto"), envOr("TRUST_PROXY_CIDRS", ""))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// 数据库文件按模式分离（Design1 §5.5）
	dbFile := map[string]string{"dev": "app-dev.db", "prod": "app-prod.db"}[mode]
	dataDir := envOr("DATA_DIR", "./data")
	st, err := store.Open(dataDir, dbFile)
	if err != nil {
		// 数据库无法打开（如文件完全损坏）→ 自动进入应急模式（Design1 §3.8），不再直接退出；
		// 传空配置 Service（store nil 时 Get 按未设置处理），保证 status/站点信息端点可用
		log.Error("打开数据库失败，自动进入应急模式", "err", err)
		runEmergencyMode(emergency.TriggerDBCorrupt, false, nil, config.NewService(nil, logger), dataDir, dbFile, mode, trustPolicy, logger)
		return
	}
	if err := st.Migrate(context.Background(), migrations.FS); err != nil {
		// 迁移失败（如中间页损坏）→ 自动进入应急模式，提供重新初始化救援（Design1 §3.8）
		log.Error("数据库迁移失败，自动进入应急模式", "err", err)
		cfg := config.NewService(st, logger)
		reason, dbReadable := emergency.Detect(context.Background(), st, cfg, logger)
		if reason == emergency.TriggerNone {
			reason = emergency.TriggerDBCorrupt // 防御性兜底：调用方已知迁移失败即视为数据库损坏
		}
		runEmergencyMode(reason, dbReadable, st, cfg, dataDir, dbFile, mode, trustPolicy, logger)
		return
	}

	cfg := config.NewService(st, logger)
	if err := cfg.Set(context.Background(), config.KeyAppMode, mode); err != nil {
		log.Error("记录运行模式失败", "err", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 规则素材池：启动时把服务重启前残留的 running 任务置 failed，并同步刷新池快照
	if _, err := st.DB().ExecContext(ctx,
		`UPDATE pool_sync_tasks SET status = 'failed', error = '服务重启，任务中断', finished_at = CURRENT_TIMESTAMP
		 WHERE status = 'running'`); err != nil {
		log.Error("重置素材池同步任务失败", "err", err)
		os.Exit(1)
	}
	if _, err := st.DB().ExecContext(ctx,
		`UPDATE rule_pools SET sync_status = 'failed', sync_error = '服务重启，任务中断'
		 WHERE id IN (SELECT DISTINCT pool_id FROM pool_sync_tasks WHERE status = 'failed' AND error = '服务重启，任务中断')`); err != nil {
		log.Error("刷新素材池同步快照失败", "err", err)
		os.Exit(1)
	}
	poolSvc := pool.NewService(st, logger)
	stopPoolSync := cron.StartPoolAutoSync(st, poolSvc, logger)
	defer stopPoolSync()

	// 同步历史 7 天全局自动清理（独立于单池同步完成副作用）
	stopSyncCleanup := cron.StartSyncHistoryCleanup(poolSvc, logger)
	defer stopSyncCleanup()

	// 应急模式触发判定（Build3 Step 6）：手动（RESET_ADMIN_PASSWORD）/自动（数据库损坏/关键配置损坏）
	reason, dbReadable := emergency.Detect(ctx, st, cfg, logger)
	if reason != emergency.TriggerNone {
		runEmergencyMode(reason, dbReadable, st, cfg, dataDir, dbFile, mode, trustPolicy, logger)
		return
	}

	users := user.NewService(st, cfg, logger)
	// 版本指针启动自检（Build2 Step 2）：DB「当前」与 symlink 不一致时以 DB 为准重建
	verSvc := version.NewService(st, dataDir, logger)
	if err := verSvc.StartupCheck(context.Background()); err != nil {
		log.Error("版本指针自检失败", "err", err)
		os.Exit(1)
	}
	srv, err := server.New(st, cfg, users, logger, mode, trustPolicy, envOr("PORT", "8080"), dataDir, streamSvc)
	if err != nil {
		log.Error("装配 HTTP 服务失败", "err", err)
		os.Exit(1)
	}
	// 访问日志 90 天自动清理（Build2 Step 4）
	stopCleanup := cron.StartAccessLogCleanup(st.DB(), logger)
	defer stopCleanup()
	// 密码重置令牌每日清理（低风险硬化 L08）
	stopResetCleanup := cron.StartResetTokenCleanup(st.DB(), logger)
	defer stopResetCleanup()

	// 信号驱动优雅退出
	if err := srv.Run(ctx); err != nil {
		log.Error("HTTP 服务退出异常", "err", err)
		os.Exit(1)
	}
	log.Info("服务已退出")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// runEmergencyMode 应急模式装配（DB 损坏/迁移失败/手动/关键配置损坏共用）：
// 仅注册 系统状态/站点信息/应急端点/静态资源；业务 API 与下载端点 503；
// Open 失败分支 st/cfg 为 nil，config.Get/user.IsTableEmpty 等已做 nil 守卫降级（Design1 §3.8）
func runEmergencyMode(reason emergency.TriggerReason, dbReadable bool, st *store.Store, cfg *config.Service, dataDir, dbFile, mode string, trust *proxytrust.Policy, logger *slog.Logger) {
	clearSvc := dataclear.NewService(st, dataDir, logger)
	emSvc := emergency.NewService(reason, dbReadable, st, cfg, clearSvc, dataDir, dbFile, logger)
	srv, err := server.NewEmergency(st, cfg, emSvc, logger, mode, trust, envOr("PORT", "8080"), dataDir)
	if err != nil {
		log.Error("装配应急服务失败", "err", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := srv.Run(ctx); err != nil {
		log.Error("应急服务退出异常", "err", err)
		os.Exit(1)
	}
	log.Info("应急服务已退出")
}
