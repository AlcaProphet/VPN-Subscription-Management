package dataclear

import (
	"context"
	"errors"
	"testing"
)

func TestClearAllHooksSuccess(t *testing.T) {
	st, svc, _ := newTestClear(t)
	ctx := context.Background()
	if _, err := st.DB().Exec(`INSERT INTO users (username, email) VALUES ('u1','u1@example.com')`); err != nil {
		t.Fatalf("插入用户失败: %v", err)
	}
	var beforeCalled, afterCalled bool
	var afterCleared bool
	svc.SetClearHooks(func(context.Context) error {
		beforeCalled = true
		return nil
	}, func(_ context.Context, cleared bool) {
		afterCalled = true
		afterCleared = cleared
	})
	if err := svc.ClearAll(ctx, ConfirmWordReset); err != nil {
		t.Fatalf("清空失败: %v", err)
	}
	if !beforeCalled || !afterCalled || !afterCleared {
		t.Fatalf("hook 调用异常 before=%v after=%v cleared=%v", beforeCalled, afterCalled, afterCleared)
	}
	var n int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if n != 0 {
		t.Fatalf("清空后 users 应为 0，实际 %d", n)
	}
}

func TestClearAllHookFailureDoesNotClearDB(t *testing.T) {
	st, svc, _ := newTestClear(t)
	ctx := context.Background()
	if _, err := st.DB().Exec(`INSERT INTO users (username, email) VALUES ('u1','u1@example.com')`); err != nil {
		t.Fatalf("插入用户失败: %v", err)
	}
	hookErr := errors.New("dispatcher pause failed")
	var afterCalled, afterCleared bool
	svc.SetClearHooks(func(context.Context) error {
		return hookErr
	}, func(_ context.Context, cleared bool) {
		afterCalled = true
		afterCleared = cleared
	})
	err := svc.ClearAll(ctx, ConfirmWordReset)
	if !errors.Is(err, ErrClearLifecycle) {
		t.Fatalf("应返回 ErrClearLifecycle: %v", err)
	}
	if !afterCalled || afterCleared {
		t.Fatalf("前置失败后应执行恢复 hook 且 cleared=false: %+v", afterCleared)
	}
	var n int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("前置失败不得清库，users 应仍为 1，实际 %d", n)
	}
}
