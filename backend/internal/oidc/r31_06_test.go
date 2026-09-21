package oidc

import (
	"context"
	"errors"
	"testing"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
)

// newProdOidcServiceR3106 复用已写入 mock 参数的测试库，构造 Production 启动 mode 的 OIDC 服务。
// mock 参数通过 Dev 服务准备，避免 SaveParams 在 Production 下被 R31-06 守卫提前拒绝。
func newProdOidcServiceR3106(t *testing.T) *Service {
	t.Helper()
	_, devSvc, users := newTestOidcService(t)
	cfg := devSvc.cfg
	authSvc := auth.NewService(cfg, users, log.New("error", "console"))
	return NewService(devSvc.store, cfg, authSvc, users, "prod", log.New("error", "console"))
}

// TestR3106ProductionStartFlowMockRejected Production 下 mock 发起登录/绑定时不得写入 state，
// 且不得顺带清理过期 state（守卫在清理之前生效）。
func TestR3106ProductionStartFlowMockRejected(t *testing.T) {
	svc := newProdOidcServiceR3106(t)
	ctx := context.Background()
	if _, err := svc.store.DB().ExecContext(ctx,
		`INSERT INTO oidc_states (state, code_verifier, nonce, intent, created_at, provider_type, config_hash, redirect_uri)
		 VALUES ('r3106-expired', 'v', 'n', 'login', datetime('now', '-1 hour'), 'mock', 'hash', 'https://app.example.com/api/auth/oidc/callback')`); err != nil {
		t.Fatalf("写入过期 state 失败: %v", err)
	}
	for _, intent := range []string{"login", "bind"} {
		if _, _, err := svc.StartFlow(ctx, intent, 1); !errors.Is(err, config.ErrMockModeRestricted) {
			t.Fatalf("Production StartFlow(%s) 应返回 ErrMockModeRestricted，实际: %v", intent, err)
		}
	}
	var count int
	if err := svc.store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM oidc_states`).Scan(&count); err != nil {
		t.Fatalf("统计 oidc_states 失败: %v", err)
	}
	if count != 1 {
		t.Fatalf("StartFlow 拒绝不得新增/清理 state，实际行数 %d", count)
	}
}

// TestR3106ProductionExchangeMockRejected Production 回调换身份时再次拒绝 mock，不得解析 code。
func TestR3106ProductionExchangeMockRejected(t *testing.T) {
	svc := newProdOidcServiceR3106(t)
	rec := &StateRecord{
		State:        "r3106-state",
		CodeVerifier: "verifier",
		Nonce:        "nonce",
		Intent:       "login",
		ProviderType: "mock",
		ConfigHash:   "hash",
		RedirectURI:  "https://app.example.com/api/auth/oidc/callback",
	}
	if _, err := svc.Exchange(context.Background(), rec, "not-a-real-code"); !errors.Is(err, config.ErrMockModeRestricted) {
		t.Fatalf("Production Exchange(mock) 应返回 ErrMockModeRestricted，实际: %v", err)
	}
}

// TestR3106ProductionWritesAndTestConnectionRejectMock 底层写入与测试连接统一拒绝 Production mock。
func TestR3106ProductionWritesAndTestConnectionRejectMock(t *testing.T) {
	svc := newProdOidcServiceR3106(t)
	ctx := context.Background()
	if err := svc.SaveParams(ctx, "mock", Params{}); !errors.Is(err, config.ErrMockModeRestricted) {
		t.Fatalf("Production SaveParams(mock) 应拒绝，实际: %v", err)
	}
	res, err := svc.TestConnectionWithSavedSecret(ctx, "mock", Params{})
	if err != nil {
		t.Fatalf("Production 测试连接不应返回内部错误: %v", err)
	}
	if res.OK {
		t.Fatalf("Production mock 测试连接不得报告通过: %+v", res)
	}
}

// TestR3106DevMockTestConnectionStillWorks Dev mock 测试连接原有路径保持可用。
func TestR3106DevMockTestConnectionStillWorks(t *testing.T) {
	_, svc, _ := newTestOidcService(t)
	res, err := svc.TestConnection(context.Background(), "mock", Params{})
	if err != nil {
		t.Fatalf("Dev mock 测试连接失败: %v", err)
	}
	if !res.OK {
		t.Fatalf("Dev mock 测试连接应通过: %+v", res)
	}
}
