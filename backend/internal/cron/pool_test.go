package cron

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"vpn-sub/internal/pool"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRunPoolAutoSyncTriggersDueOnce(t *testing.T) {
	state := &poolAutoSyncState{fired: map[int64]bool{}}
	var submitted []int64
	deps := poolAutoSyncDeps{
		queryDue: func(_ context.Context, hhmm string) ([]int64, error) {
			if hhmm != "04:00" {
				t.Fatalf("期望查询 04:00，实际 %s", hhmm)
			}
			return []int64{1, 2}, nil
		},
		submit: func(_ context.Context, id int64) error {
			submitted = append(submitted, id)
			return nil
		},
	}
	runPoolAutoSync(context.Background(), deps, time.Date(2026, 8, 20, 4, 0, 0, 0, time.UTC), state, testLogger())
	if len(submitted) != 2 || submitted[0] != 1 || submitted[1] != 2 {
		t.Fatalf("到期池应提交 1、2，实际 %v", submitted)
	}
}

func TestRunPoolAutoSyncSameMinuteDedup(t *testing.T) {
	state := &poolAutoSyncState{fired: map[int64]bool{}}
	var submitted []int64
	deps := poolAutoSyncDeps{
		queryDue: func(_ context.Context, _ string) ([]int64, error) {
			return []int64{1}, nil
		},
		submit: func(_ context.Context, id int64) error {
			submitted = append(submitted, id)
			return nil
		},
	}
	base := time.Date(2026, 8, 20, 4, 0, 0, 0, time.UTC)
	runPoolAutoSync(context.Background(), deps, base, state, testLogger())
	runPoolAutoSync(context.Background(), deps, base.Add(30*time.Second), state, testLogger())
	if len(submitted) != 1 {
		t.Fatalf("同一分钟内同一池不应重复提交，实际 %v", submitted)
	}
}

func TestRunPoolAutoSyncSkipsRunning(t *testing.T) {
	state := &poolAutoSyncState{fired: map[int64]bool{}}
	var submitted []int64
	deps := poolAutoSyncDeps{
		queryDue: func(_ context.Context, _ string) ([]int64, error) {
			return []int64{1, 2}, nil
		},
		submit: func(_ context.Context, id int64) error {
			submitted = append(submitted, id)
			if id == 1 {
				return pool.ErrSyncRunning
			}
			return nil
		},
	}
	runPoolAutoSync(context.Background(), deps, time.Date(2026, 8, 20, 4, 0, 0, 0, time.UTC), state, testLogger())
	if len(submitted) != 2 {
		t.Fatalf("running 跳过不应中断后续池，实际 %v", submitted)
	}
}

func TestRunPoolAutoSyncCatchUpMissed(t *testing.T) {
	state := &poolAutoSyncState{fired: map[int64]bool{}}
	var submitted []int64
	deps := poolAutoSyncDeps{
		queryDue: func(_ context.Context, _ string) ([]int64, error) {
			return nil, nil
		},
		queryMissed: func(_ context.Context, _ time.Time) ([]int64, error) {
			return []int64{3, 4}, nil
		},
		submit: func(_ context.Context, id int64) error {
			submitted = append(submitted, id)
			return nil
		},
	}
	runPoolAutoSync(context.Background(), deps, time.Date(2026, 8, 20, 4, 0, 0, 0, time.UTC), state, testLogger())
	if len(submitted) != 2 || submitted[0] != 3 || submitted[1] != 4 {
		t.Fatalf("错过池应提交 3、4，实际 %v", submitted)
	}
}

func TestRunPoolAutoSyncDueAndMissedDedup(t *testing.T) {
	state := &poolAutoSyncState{fired: map[int64]bool{}}
	var submitted []int64
	deps := poolAutoSyncDeps{
		queryDue: func(_ context.Context, _ string) ([]int64, error) {
			return []int64{1}, nil
		},
		queryMissed: func(_ context.Context, _ time.Time) ([]int64, error) {
			return []int64{1, 2}, nil
		},
		submit: func(_ context.Context, id int64) error {
			submitted = append(submitted, id)
			return nil
		},
	}
	runPoolAutoSync(context.Background(), deps, time.Date(2026, 8, 20, 4, 0, 0, 0, time.UTC), state, testLogger())
	if len(submitted) != 2 || submitted[0] != 1 || submitted[1] != 2 {
		t.Fatalf("当前分钟与补跑应去重后提交 1、2，实际 %v", submitted)
	}
}
