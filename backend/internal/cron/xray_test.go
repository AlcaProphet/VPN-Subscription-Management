package cron

import (
	"context"
	"testing"
	"time"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/internal/tasks"
	"vpn-sub/internal/xray"
	"vpn-sub/migrations"
)

func newXrayCronEnv(t *testing.T) (*store.Store, *config.Service, *xray.InstanceService, *xray.SyncService, *xray.ExtService) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), migrations.FS); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	if err := cfg.Set(context.Background(), config.KeySigningKey, "0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Set(context.Background(), config.KeyAdvancedMode, "true"); err != nil {
		t.Fatal(err)
	}
	instSvc := xray.NewInstanceService(st, log.New("error", "console"), tasks.NewRegistry())
	creds := xray.NewCredentialService(st, cfg)
	syncSvc := xray.NewSyncService(st, cfg, creds, instSvc, tasks.NewRegistry(), log.New("error", "console"))
	extSvc := xray.NewExtService(st, cfg, instSvc, log.New("error", "console"))
	return st, cfg, instSvc, syncSvc, extSvc
}

func TestRunXrayCollectAdvancedOffSkips(t *testing.T) {
	st, cfg, instSvc, syncSvc, extSvc := newXrayCronEnv(t)
	if err := cfg.Set(context.Background(), config.KeyAdvancedMode, "false"); err != nil {
		t.Fatal(err)
	}
	var lastRun time.Time
	if ran := runXrayCollect(st, instSvc, syncSvc, extSvc, cfg, log.New("error", "console"), &lastRun); ran {
		t.Fatal("高级模式关闭时不应执行采集")
	}
	if !lastRun.IsZero() {
		t.Fatal("高级模式关闭时不应更新 lastRun")
	}
}

func TestRunXrayCollectIntervalAndReentry(t *testing.T) {
	st, cfg, instSvc, syncSvc, extSvc := newXrayCronEnv(t)
	lg := log.New("error", "console")
	var lastRun time.Time
	if ran := runXrayCollect(st, instSvc, syncSvc, extSvc, cfg, lg, &lastRun); !ran {
		t.Fatal("高级模式开启且无历史运行时应当执行")
	}
	if lastRun.IsZero() {
		t.Fatal("执行后应更新 lastRun")
	}
	if ran := runXrayCollect(st, instSvc, syncSvc, extSvc, cfg, lg, &lastRun); ran {
		t.Fatal("间隔未到不应重复执行")
	}
	// 配置间隔下限为 1 分钟，间隔 0 时按 1 分钟判断。
	if err := cfg.Set(context.Background(), "xray_collect_interval_minutes", "0"); err != nil {
		t.Fatal(err)
	}
	lastRun = time.Now().Add(-2 * time.Minute)
	if ran := runXrayCollect(st, instSvc, syncSvc, extSvc, cfg, lg, &lastRun); !ran {
		t.Fatal("间隔下限 1 分钟已满足时应执行")
	}
}
