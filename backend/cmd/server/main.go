// 程序入口：环境变量解析、日志初始化、数据库迁移、路由装配与优雅退出。
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/server"
	"vpn-sub/internal/store"
	"vpn-sub/internal/user"
	"vpn-sub/migrations"
)

func main() {
	// 环境变量：APP_MODE(dev|prod 默认 prod)、LOG_LEVEL(默认 info)、LOG_FORMAT(默认 console)、
	// PORT(默认 8080)、TRUST_PROXY(默认 auto)、DATA_DIR(默认 ./data)、
	// RESET_ADMIN_PASSWORD（本 Build 仅读取留存，应急逻辑在 Build3 实现）
	mode := envOr("APP_MODE", "prod")
	if mode != "dev" && mode != "prod" {
		fmt.Fprintln(os.Stderr, "APP_MODE 仅支持 dev|prod")
		os.Exit(1)
	}
	logger := log.New(envOr("LOG_LEVEL", "info"), envOr("LOG_FORMAT", "console"))
	log.SetDefault(logger)
	_ = os.Getenv("RESET_ADMIN_PASSWORD") // 留存读取点，Build3 接通

	// 数据库文件按模式分离（Design1 §5.5）
	dbFile := map[string]string{"dev": "app-dev.db", "prod": "app-prod.db"}[mode]
	dataDir := envOr("DATA_DIR", "./data")
	st, err := store.Open(dataDir, dbFile)
	if err != nil {
		log.Error("打开数据库失败", "err", err)
		os.Exit(1)
	}
	if err := st.Migrate(context.Background(), migrations.FS); err != nil {
		log.Error("数据库迁移失败，拒绝启动", "err", err)
		os.Exit(1)
	}

	cfg := config.NewService(st, logger)
	if err := cfg.Set(context.Background(), config.KeyAppMode, mode); err != nil {
		log.Error("记录运行模式失败", "err", err)
		os.Exit(1)
	}
	users := user.NewService(st, cfg, logger)
	srv, err := server.New(st, cfg, users, logger, mode, envOr("TRUST_PROXY", "auto"), envOr("PORT", "8080"), dataDir)
	if err != nil {
		log.Error("装配 HTTP 服务失败", "err", err)
		os.Exit(1)
	}

	// 信号驱动优雅退出
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
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
