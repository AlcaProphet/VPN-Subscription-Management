package xray

import (
	"context"
	"sync"
	"testing"
	"time"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/internal/tasks"
	"vpn-sub/migrations"
)

func TestSubmitAdvancedModeConcurrentOffCreatesSingleTask(t *testing.T) {
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Migrate(context.Background(), migrations.FS); err != nil {
		t.Fatal(err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	ctx := context.Background()
	if err := cfg.Set(ctx, config.KeySigningKey, "0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Set(ctx, config.KeyAdvancedMode, "true"); err != nil {
		t.Fatal(err)
	}
	reg := tasks.NewRegistry()
	svc := NewOffClearService(st, cfg, reg, log.New("error", "console"))

	const n = 8
	var wg sync.WaitGroup
	ids := make([]string, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ids[i], errs[i] = svc.SubmitAdvancedMode(ctx, false, ConfirmWordDisable)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d error: %v", i, err)
		}
	}
	var nonEmpty int
	for _, id := range ids {
		if id != "" {
			nonEmpty++
		}
	}
	if nonEmpty != 1 {
		t.Fatalf("应只创建一个 OFF 任务，实际 %d 个: %v", nonEmpty, ids)
	}
	// 等待任务完成（避免测试退出时后台 goroutine 仍在跑）
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		done := true
		for _, id := range ids {
			if id == "" {
				continue
			}
			if reg.Get(id).Status == tasks.StatusRunning {
				done = false
			}
		}
		if done {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("OFF 任务未在超时内完成")
}
