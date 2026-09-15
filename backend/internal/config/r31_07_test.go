package config

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// R31-07 配置层隔离测试：停用/清空事务、保留语义、防死锁与故障回滚。

func seedR3107Enabled(t *testing.T, svc *AdminService) {
	t.Helper()
	ctx := context.Background()
	for k, v := range map[string]string{
		KeyAllowLocalLogin:    "true",
		oidcKeyConfigured:     "true",
		oidcKeyProviderType:   "generic",
		"oidc_params_generic": `{"base_url":"https://idp.example.com","realm":"","client_id":"c","client_secret":"cipher"}`,
		KeyFrontendURL:        "https://app.example.com",
		KeyCallbackURL:        "https://callback.example.com" + OidcCallbackPath,
	} {
		if err := svc.cfg.Set(ctx, k, v); err != nil {
			t.Fatalf("写入测试配置 %s 失败: %v", k, err)
		}
	}
}

func seedR3107TransientRows(t *testing.T, svc *AdminService) {
	t.Helper()
	ctx := context.Background()
	if _, err := svc.store.DB().ExecContext(ctx,
		`INSERT INTO oidc_states (state, code_verifier, nonce, intent, provider_type, config_hash, redirect_uri)
		 VALUES ('r3107-state','v','n','login','generic','hash','https://app.example.com/api/auth/oidc/callback')`); err != nil {
		t.Fatalf("写入 state 失败: %v", err)
	}
	if _, err := svc.store.DB().ExecContext(ctx,
		`INSERT INTO oidc_login_tickets (ticket, session_token, expires_at, flow_hash)
		 VALUES ('r3107-ticket','session',datetime('now','+1 minute'),'flow-hash')`); err != nil {
		t.Fatalf("写入 ticket 失败: %v", err)
	}
	if _, err := svc.store.DB().ExecContext(ctx,
		`INSERT INTO users (id, oidc_subject) VALUES (1, 'r3107-subject')`); err != nil {
		t.Fatalf("写入用户绑定失败: %v", err)
	}
}

func countR3107Rows(t *testing.T, svc *AdminService, table string) int {
	t.Helper()
	var n int
	if err := svc.store.DB().QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatalf("统计 %s 失败: %v", table, err)
	}
	return n
}

// TestR3107DisableOidcPersistsAndPreserves 停用只写 enabled=false 与流程代际；
// provider/参数/地址/已有绑定全部保留，进行中 state 与未兑换 ticket 被同事务清理。
func TestR3107DisableOidcPersistsAndPreserves(t *testing.T) {
	_, svc := newTestAdmin(t, &mockOidcOps{configured: true, secret: "cipher"})
	ctx := context.Background()
	seedR3107Enabled(t, svc)
	seedR3107TransientRows(t, svc)

	if err := svc.DisableOidc(ctx); err != nil {
		t.Fatalf("DisableOidc 失败: %v", err)
	}
	if got := svc.cfg.GetOr(ctx, oidcKeyConfigured); got != "false" {
		t.Fatalf("oidc_configured 应为 false，实际 %q", got)
	}
	if got := svc.cfg.GetOr(ctx, oidcKeyProviderType); got != "generic" {
		t.Fatalf("停用后应保留 provider，实际 %q", got)
	}
	if got := svc.cfg.GetOr(ctx, "oidc_params_generic"); got == "" {
		t.Fatalf("停用后应保留提供商参数")
	}
	if got := svc.cfg.GetOr(ctx, KeyFrontendURL); got != "https://app.example.com" {
		t.Fatalf("停用后应保留前端地址，实际 %q", got)
	}
	if got := svc.cfg.GetOr(ctx, KeyCallbackURL); got != "https://callback.example.com"+OidcCallbackPath {
		t.Fatalf("停用后应保留独立回调，实际 %q", got)
	}
	if got := svc.cfg.GetOr(ctx, KeyAllowLocalLogin); got != "true" {
		t.Fatalf("停用不应改变本地登录开关，实际 %q", got)
	}
	if got := countR3107Rows(t, svc, "oidc_states"); got != 0 {
		t.Fatalf("停用后 state 应清空，实际 %d", got)
	}
	if got := countR3107Rows(t, svc, "oidc_login_tickets"); got != 0 {
		t.Fatalf("停用后 ticket 应清空，实际 %d", got)
	}
	var subject string
	if err := svc.store.DB().QueryRow(`SELECT COALESCE(oidc_subject,'') FROM users WHERE id=1`).Scan(&subject); err != nil || subject != "r3107-subject" {
		t.Fatalf("停用必须保留已有绑定: subject=%q err=%v", subject, err)
	}
	if epoch := svc.cfg.GetOr(ctx, KeyOidcFlowEpoch); epoch == "" {
		t.Fatalf("停用应写入新的流程代际")
	}
}

// TestR3107DisableRejectedWhenLocalLoginOff 本地登录已关时停用必须拒绝且不写任何配置/临时记录。
func TestR3107DisableRejectedWhenLocalLoginOff(t *testing.T) {
	_, svc := newTestAdmin(t, &mockOidcOps{configured: true, secret: "cipher"})
	ctx := context.Background()
	seedR3107Enabled(t, svc)
	seedR3107TransientRows(t, svc)
	if err := svc.cfg.Set(ctx, KeyAllowLocalLogin, "false"); err != nil {
		t.Fatalf("设置本地登录关闭失败: %v", err)
	}
	before := readSystemConfigSnapshot(t, svc.store)
	beforeStates := countR3107Rows(t, svc, "oidc_states")
	beforeTickets := countR3107Rows(t, svc, "oidc_login_tickets")

	if err := svc.DisableOidc(ctx); !errors.Is(err, ErrAuthDeadlock) {
		t.Fatalf("本地登录关闭时停用应返回 ErrAuthDeadlock，实际: %v", err)
	}
	if after := readSystemConfigSnapshot(t, svc.store); !equalStringMap(before, after) {
		t.Fatalf("停用拒绝后配置不得变化:\nbefore=%v\nafter=%v", before, after)
	}
	if got := countR3107Rows(t, svc, "oidc_states"); got != beforeStates {
		t.Fatalf("停用拒绝后 state 不得变化: before=%d after=%d", beforeStates, got)
	}
	if got := countR3107Rows(t, svc, "oidc_login_tickets"); got != beforeTickets {
		t.Fatalf("停用拒绝后 ticket 不得变化: before=%d after=%d", beforeTickets, got)
	}
}

// TestR3107ClearOidcTransactionAndEpoch 清空删除 provider/参数并轮换代际，地址与绑定不由清空动作改写。
func TestR3107ClearOidcTransactionAndEpoch(t *testing.T) {
	_, svc := newTestAdmin(t, &mockOidcOps{configured: true, secret: "cipher"})
	ctx := context.Background()
	seedR3107Enabled(t, svc)
	seedR3107TransientRows(t, svc)

	if err := svc.ClearOidc(ctx); err != nil {
		t.Fatalf("ClearOidc 失败: %v", err)
	}
	if got := svc.cfg.GetOr(ctx, oidcKeyConfigured); got != "false" {
		t.Fatalf("清空后 oidc_configured 应为 false，实际 %q", got)
	}
	if got := svc.cfg.GetOr(ctx, oidcKeyProviderType); got != "" {
		t.Fatalf("清空后 provider 应为空，实际 %q", got)
	}
	for _, p := range validProviders {
		if got := svc.cfg.GetOr(ctx, "oidc_params_"+p); got != "" {
			t.Fatalf("清空后参数 %s 应为空，实际 %q", p, got)
		}
	}
	if got := svc.cfg.GetOr(ctx, KeyFrontendURL); got != "https://app.example.com" {
		t.Fatalf("清空不应删除站点级前端地址，实际 %q", got)
	}
	if got := countR3107Rows(t, svc, "oidc_states"); got != 0 {
		t.Fatalf("清空后 state 应为 0，实际 %d", got)
	}
	if got := countR3107Rows(t, svc, "oidc_login_tickets"); got != 0 {
		t.Fatalf("清空后 ticket 应为 0，实际 %d", got)
	}
	if epoch := svc.cfg.GetOr(ctx, KeyOidcFlowEpoch); epoch == "" {
		t.Fatalf("清空应写入新的流程代际")
	}
}

// TestR3107DisableRollbackOnWriteFailure 停用过程中任一写失败，参数、state/ticket、代际全部回滚。
func TestR3107DisableRollbackOnWriteFailure(t *testing.T) {
	_, svc := newTestAdmin(t, &mockOidcOps{configured: true, secret: "cipher"})
	ctx := context.Background()
	seedR3107Enabled(t, svc)
	seedR3107TransientRows(t, svc)
	before := readSystemConfigSnapshot(t, svc.store)
	if _, err := svc.store.DB().Exec(`
		CREATE TRIGGER r3107_fail_configured BEFORE UPDATE ON system_config
		WHEN NEW.key = 'oidc_configured'
		BEGIN
			SELECT RAISE(ABORT, 'r3107 injected failure');
		END;`); err != nil {
		t.Fatalf("创建失败注入触发器失败: %v", err)
	}
	t.Cleanup(func() { _, _ = svc.store.DB().Exec(`DROP TRIGGER IF EXISTS r3107_fail_configured`) })

	if err := svc.DisableOidc(ctx); err == nil {
		t.Fatal("注入写失败后 DisableOidc 应返回错误")
	}
	if after := readSystemConfigSnapshot(t, svc.store); !equalStringMap(before, after) {
		t.Fatalf("停用失败后配置应整体回滚:\nbefore=%v\nafter=%v", before, after)
	}
	if got := countR3107Rows(t, svc, "oidc_states"); got != 1 {
		t.Fatalf("停用失败后 state 应回滚保留，实际 %d", got)
	}
	if got := countR3107Rows(t, svc, "oidc_login_tickets"); got != 1 {
		t.Fatalf("停用失败后 ticket 应回滚保留，实际 %d", got)
	}
}

// TestR3107SaveLocalAuthRollbackOnWriteFailure 本地登录三开关的检查与写入同事务；任一写失败整体回滚。
func TestR3107SaveLocalAuthRollbackOnWriteFailure(t *testing.T) {
	_, svc := newTestAdmin(t, &mockOidcOps{configured: true, secret: "cipher"})
	ctx := context.Background()
	for _, k := range []string{KeyAllowLocalLogin, KeyAllowSelfreg, KeySelfRegApproval} {
		if err := svc.cfg.Set(ctx, k, "false"); err != nil {
			t.Fatalf("初始化 %s 失败: %v", k, err)
		}
	}
	before := readSystemConfigSnapshot(t, svc.store)
	if _, err := svc.store.DB().Exec(`
		CREATE TRIGGER r3107_fail_selfreg BEFORE UPDATE ON system_config
		WHEN NEW.key = 'selfreg_approval'
		BEGIN
			SELECT RAISE(ABORT, 'r3107 injected failure');
		END;`); err != nil {
		t.Fatalf("创建失败注入触发器失败: %v", err)
	}
	t.Cleanup(func() { _, _ = svc.store.DB().Exec(`DROP TRIGGER IF EXISTS r3107_fail_selfreg`) })

	err := svc.SaveLocalAuth(ctx, LocalAuthSettings{AllowLocalLogin: true, AllowSelfReg: true, SelfRegApproval: true})
	if err == nil {
		t.Fatal("注入写失败后 SaveLocalAuth 应返回错误")
	}
	if after := readSystemConfigSnapshot(t, svc.store); !equalStringMap(before, after) {
		t.Fatalf("本地登录保存失败后开关应整体回滚:\nbefore=%v\nafter=%v", before, after)
	}
}

// TestR3107ConcurrentDisableAndLocalAuthSerialized 停用与关闭本地登录必须只有一个先成功，
// 不允许并发交错后出现 allow_local_login=false + oidc_configured=false 的双不可用状态。
func TestR3107ConcurrentDisableAndLocalAuthSerialized(t *testing.T) {
	for i := 0; i < 10; i++ {
		_, svc := newTestAdmin(t, &mockOidcOps{configured: true, secret: "cipher"})
		seedR3107Enabled(t, svc)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		var wg sync.WaitGroup
		start := make(chan struct{})
		var localErr, disableErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			localErr = svc.SaveLocalAuth(ctx, LocalAuthSettings{AllowLocalLogin: false, AllowSelfReg: false, SelfRegApproval: false})
		}()
		go func() {
			defer wg.Done()
			<-start
			disableErr = svc.DisableOidc(ctx)
		}()
		close(start)
		wg.Wait()
		cancel()

		readCtx := context.Background()
		allowLocal := svc.cfg.GetOr(readCtx, KeyAllowLocalLogin)
		configured := svc.cfg.GetOr(readCtx, oidcKeyConfigured)
		if allowLocal == "false" && configured == "false" {
			t.Fatalf("第 %d 轮并发后出现本地登录与 OIDC 双关", i)
		}
		switch {
		case localErr == nil && errors.Is(disableErr, ErrAuthDeadlock):
			if allowLocal != "false" || configured != "true" {
				t.Fatalf("第 %d 轮 SaveLocalAuth 先成功，状态异常: allow=%q oidc=%q", i, allowLocal, configured)
			}
		case disableErr == nil && errors.Is(localErr, ErrAuthDeadlock):
			if allowLocal != "true" || configured != "false" {
				t.Fatalf("第 %d 轮 DisableOidc 先成功，状态异常: allow=%q oidc=%q", i, allowLocal, configured)
			}
		default:
			t.Fatalf("第 %d 轮并发结果不符合串行化预期: localErr=%v disableErr=%v", i, localErr, disableErr)
		}
	}
}

// TestR3107ConcurrentClearAndLocalAuthSerialized 清空与关闭本地登录同样必须串行：清空后若 OIDC 不再可用，
// 并发请求不得再成功关闭本地登录形成双不可用。
func TestR3107ConcurrentClearAndLocalAuthSerialized(t *testing.T) {
	for i := 0; i < 10; i++ {
		_, svc := newTestAdmin(t, &mockOidcOps{configured: true, secret: "cipher"})
		seedR3107Enabled(t, svc)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		var wg sync.WaitGroup
		start := make(chan struct{})
		var localErr, clearErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			localErr = svc.SaveLocalAuth(ctx, LocalAuthSettings{AllowLocalLogin: false, AllowSelfReg: false, SelfRegApproval: false})
		}()
		go func() {
			defer wg.Done()
			<-start
			clearErr = svc.ClearOidc(ctx)
		}()
		close(start)
		wg.Wait()
		cancel()

		readCtx := context.Background()
		allowLocal := svc.cfg.GetOr(readCtx, KeyAllowLocalLogin)
		configured := svc.cfg.GetOr(readCtx, oidcKeyConfigured)
		if allowLocal == "false" && configured == "false" {
			t.Fatalf("第 %d 轮清空/本地登录并发后出现双关", i)
		}
		switch {
		case localErr == nil && errors.Is(clearErr, ErrAuthDeadlock):
			if allowLocal != "false" || configured != "true" {
				t.Fatalf("第 %d 轮 SaveLocalAuth 先成功，状态异常: allow=%q oidc=%q", i, allowLocal, configured)
			}
		case clearErr == nil && errors.Is(localErr, ErrAuthDeadlock):
			if allowLocal != "true" || configured != "false" {
				t.Fatalf("第 %d 轮 ClearOidc 先成功，状态异常: allow=%q oidc=%q", i, allowLocal, configured)
			}
		default:
			t.Fatalf("第 %d 轮清空/本地登录并发结果不符合串行化预期: localErr=%v clearErr=%v", i, localErr, clearErr)
		}
	}
}

// TestR3107GetOidcEnabledFlag 管理端 GET 区分生效状态与保留 provider。
func TestR3107GetOidcEnabledFlag(t *testing.T) {
	_, svc := newTestAdmin(t, &mockOidcOps{configured: true, secret: "cipher"})
	ctx := context.Background()
	seedR3107Enabled(t, svc)

	got, err := svc.GetOidc(ctx)
	if err != nil || !got.Enabled || got.ProviderType != "generic" {
		t.Fatalf("启用态 GET 异常: %+v err=%v", got, err)
	}
	if err := svc.DisableOidc(ctx); err != nil {
		t.Fatalf("停用失败: %v", err)
	}
	got, err = svc.GetOidc(ctx)
	if err != nil {
		t.Fatalf("停用后 GET 失败: %v", err)
	}
	if got.Enabled || got.ProviderType != "generic" {
		t.Fatalf("停用后 GET 应 enabled=false 且保留 provider: %+v", got)
	}
}
