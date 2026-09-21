package oidc

import (
	"errors"
	"testing"
	"time"

	"vpn-sub/internal/config"
)

// R31-07 业务层隔离测试：流程代际/指纹、停用阻断、旧 state/ticket 不可恢复。

func setFlowEpochForTest(t *testing.T, svc *Service, epoch string) {
	t.Helper()
	if err := svc.cfg.Set(ctx, config.KeyOidcFlowEpoch, epoch); err != nil {
		t.Fatalf("设置流程代际失败: %v", err)
	}
}

func countUsersForTest(t *testing.T, svc *Service) int {
	t.Helper()
	var n int
	if err := svc.store.DB().QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		t.Fatalf("统计用户失败: %v", err)
	}
	return n
}

// TestR3107StartFlowRejectsDisabled StartFlow 在写入 state 前拒绝停用状态。
func TestR3107StartFlowRejectsDisabled(t *testing.T) {
	st, svc, _ := newTestOidcService(t)
	if err := svc.cfg.Set(ctx, KeyConfigured, "false"); err != nil {
		t.Fatalf("设置停用失败: %v", err)
	}
	for _, intent := range []string{"login", "bind"} {
		if _, _, err := svc.StartFlow(ctx, intent, 1); !errors.Is(err, ErrOidcDisabled) {
			t.Fatalf("停用后 StartFlow(%s) 应返回 ErrOidcDisabled，实际: %v", intent, err)
		}
	}
	var n int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM oidc_states`).Scan(&n); err != nil {
		t.Fatalf("统计 state 失败: %v", err)
	}
	if n != 0 {
		t.Fatalf("停用后不得新增 state，实际 %d", n)
	}
}

// TestR3107ConsumeStateRejectsDisabledAndPreservesRow 停用时回调不得消费/删除 state 原行。
func TestR3107ConsumeStateRejectsDisabledAndPreservesRow(t *testing.T) {
	st, svc, _ := newTestOidcService(t)
	_, state, err := svc.StartFlow(ctx, "login", 0)
	if err != nil {
		t.Fatalf("StartFlow 失败: %v", err)
	}
	if err := svc.cfg.Set(ctx, KeyConfigured, "false"); err != nil {
		t.Fatalf("设置停用失败: %v", err)
	}
	if _, err := svc.ConsumeState(ctx, state); err == nil {
		t.Fatal("停用时 ConsumeState 应拒绝")
	}
	var n int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM oidc_states WHERE state = ?`, state).Scan(&n); err != nil {
		t.Fatalf("统计 state 失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("停用拒绝应保留 state 原行，实际 %d", n)
	}
}

// TestR3107ConsumedStateCannotResumeAfterReenable state 已消费后进行中回调仍受固定流程指纹约束；
// 停用后快速重新启用也不能恢复旧流程、旧绑定或签发会话。
func TestR3107ConsumedStateCannotResumeAfterReenable(t *testing.T) {
	st, svc, users := newTestOidcService(t)
	_, state, err := svc.StartFlow(ctx, "login", 0)
	if err != nil {
		t.Fatalf("StartFlow 失败: %v", err)
	}
	rec, err := svc.ConsumeState(ctx, state)
	if err != nil {
		t.Fatalf("ConsumeState 失败: %v", err)
	}
	wantHash := rec.ConfigHash
	if wantHash == "" {
		t.Fatal("state 应固定流程指纹")
	}

	u, err := users.Register(ctx, "r3107-flow-user", "r3107-flow@example.com", "password123")
	if err != nil {
		t.Fatalf("创建绑定目标用户失败: %v", err)
	}
	rec.Intent = "bind"
	rec.BindUserID = u.ID

	// 模拟停用并快速重新启用：配置 provider/参数不变，仅轮换代际。
	if err := svc.cfg.Set(ctx, KeyConfigured, "false"); err != nil {
		t.Fatalf("停用失败: %v", err)
	}
	setFlowEpochForTest(t, svc, "r3107-epoch-2")
	if err := svc.cfg.Set(ctx, KeyConfigured, "true"); err != nil {
		t.Fatalf("重新启用失败: %v", err)
	}

	if _, err := svc.Exchange(ctx, rec, "code"); !errors.Is(err, ErrOidcFlowInvalid) {
		t.Fatalf("已消费 state 在重新启用后 Exchange 应返回 ErrOidcFlowInvalid，实际: %v", err)
	}
	if err := svc.ResolveBind(ctx, rec, &Identity{Subject: "r3107-sub-new", Email: "r3107-sub-new@example.com", EmailVerified: true}); !errors.Is(err, ErrOidcFlowInvalid) {
		t.Fatalf("旧绑定流程应返回 ErrOidcFlowInvalid，实际: %v", err)
	}
	var subject string
	if err := st.DB().QueryRow(`SELECT COALESCE(oidc_subject,'') FROM users WHERE id = ?`, u.ID).Scan(&subject); err != nil {
		t.Fatalf("读取绑定失败: %v", err)
	}
	if subject != "" {
		t.Fatalf("停用后快速重新启用不得恢复/产生新绑定: %q", subject)
	}

	beforeUsers := countUsersForTest(t, svc)
	if _, err := svc.ResolveLoginForFlow(ctx, rec, &Identity{Subject: "r3107-login-new", Email: "r3107-login-new@example.com", EmailVerified: true}); !errors.Is(err, ErrOidcFlowInvalid) {
		t.Fatalf("旧登录流程应返回 ErrOidcFlowInvalid，实际: %v", err)
	}
	if afterUsers := countUsersForTest(t, svc); afterUsers != beforeUsers {
		t.Fatalf("旧登录流程不得新建用户: before=%d after=%d", beforeUsers, afterUsers)
	}

	if _, _, err := svc.IssueLoginSessionForFlow(ctx, u.ID, u.CredentialVersion, rec.ConfigHash); !errors.Is(err, ErrOidcFlowInvalid) {
		t.Fatalf("旧流程不得签发会话 ticket，实际: %v", err)
	}
	var tickets int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM oidc_login_tickets`).Scan(&tickets); err != nil {
		t.Fatalf("统计 ticket 失败: %v", err)
	}
	if tickets != 0 {
		t.Fatalf("旧流程不得留下 ticket，实际 %d", tickets)
	}
}

// TestR3107TicketFlowHashRejectsDisableAndLegacy 新 ticket 固定流程指纹；停用/重启用与历史空指纹均不可兑换。
func TestR3107TicketFlowHashRejectsDisableAndLegacy(t *testing.T) {
	st, svc, _ := newTestOidcService(t)
	flowHash, err := svc.currentFlowHash(ctx)
	if err != nil {
		t.Fatalf("读取当前流程指纹失败: %v", err)
	}
	ticket, err := svc.IssueLoginTicketForFlow(ctx, "session-old", flowHash)
	if err != nil {
		t.Fatalf("签发旧流程 ticket 失败: %v", err)
	}

	// 停用 + 快速重新启用，代际变化。
	if err := svc.cfg.Set(ctx, KeyConfigured, "false"); err != nil {
		t.Fatalf("停用失败: %v", err)
	}
	setFlowEpochForTest(t, svc, "r3107-epoch-3")
	if err := svc.cfg.Set(ctx, KeyConfigured, "true"); err != nil {
		t.Fatalf("重新启用失败: %v", err)
	}
	if _, err := svc.ConsumeLoginTicket(ctx, ticket); !errors.Is(err, ErrLoginTicketInvalid) {
		t.Fatalf("重新启用后旧 ticket 应无效，实际: %v", err)
	}
	var n int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM oidc_login_tickets WHERE ticket = ?`, ticket).Scan(&n); err != nil {
		t.Fatalf("统计旧 ticket 失败: %v", err)
	}
	if n != 0 {
		t.Fatalf("失效 ticket 应被清理，实际 %d", n)
	}

	// 历史空指纹 ticket（含迁移前/R31-06 旧 Dev 数据）永久无效。
	if _, err := st.DB().Exec(
		`INSERT INTO oidc_login_tickets (ticket, session_token, expires_at, flow_hash) VALUES ('legacy-r3107','legacy-session',?, '')`,
		time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("写入历史 ticket 失败: %v", err)
	}
	if _, err := svc.ConsumeLoginTicket(ctx, "legacy-r3107"); !errors.Is(err, ErrLoginTicketInvalid) {
		t.Fatalf("历史空指纹 ticket 应无效，实际: %v", err)
	}
}

// TestR3107MockDirectSessionGated MockLogin 固定请求开始时的流程；停用/快速重启用后旧流程不得签发直连会话。
func TestR3107MockDirectSessionGated(t *testing.T) {
	st, svc, _ := newTestOidcService(t)
	res, err := svc.MockLogin(ctx, "r3107-mock@example.com", "", true, nil, nil)
	if err != nil {
		t.Fatalf("MockLogin 失败: %v", err)
	}
	if res.User == nil || res.FlowHash == "" {
		t.Fatalf("MockLogin 应返回用户与固定流程: %+v", res)
	}

	// 仍处于同一流程时可直接签发，直连会话 JWT 可解析。
	token, exp, err := svc.IssueDirectSessionForFlow(ctx, res.User.ID, res.User.CredentialVersion, res.FlowHash)
	if err != nil || token == "" || time.Until(exp) <= 0 {
		t.Fatalf("同流程直连会话应签发成功: token=%q exp=%v err=%v", token, exp, err)
	}
	if _, err := svc.authSvc.Parse(ctx, token); err != nil {
		t.Fatalf("直连会话 JWT 应可解析: %v", err)
	}

	// 停用 + 快速重新启用，代际变化后旧流程不可再签发。
	if err := svc.cfg.Set(ctx, KeyConfigured, "false"); err != nil {
		t.Fatalf("停用失败: %v", err)
	}
	setFlowEpochForTest(t, svc, "r3107-epoch-4")
	if err := svc.cfg.Set(ctx, KeyConfigured, "true"); err != nil {
		t.Fatalf("重新启用失败: %v", err)
	}
	if _, _, err := svc.IssueDirectSessionForFlow(ctx, res.User.ID, res.User.CredentialVersion, res.FlowHash); !errors.Is(err, ErrOidcFlowInvalid) {
		t.Fatalf("重新启用后旧流程不得签发直连会话，实际: %v", err)
	}
	_ = st
}

// TestR3107FlowHashVersionBoundaries 流程指纹必须随 mode/epoch/provider/raw 变化；地址不属于指纹。
func TestR3107FlowHashVersionBoundaries(t *testing.T) {
	_, svc, _ := newTestOidcService(t)
	raw, err := svc.cfg.Get(ctx, "oidc_params_mock")
	if err != nil {
		t.Fatalf("读取 mock 参数失败: %v", err)
	}
	base := flowConfigHash("dev", "epoch-a", "mock", raw)
	if base == flowConfigHash("prod", "epoch-a", "mock", raw) {
		t.Fatal("流程指纹必须包含运行模式")
	}
	if base == flowConfigHash("dev", "epoch-b", "mock", raw) {
		t.Fatal("流程指纹必须包含流程代际")
	}
	otherRaw, err := svc.cfg.Get(ctx, "oidc_params_mock")
	if err != nil {
		t.Fatalf("读取参数失败: %v", err)
	}
	if base == flowConfigHash("dev", "epoch-a", "generic", otherRaw) {
		t.Fatal("流程指纹必须包含提供商类型")
	}
	if base == flowConfigHash("dev", "epoch-a", "mock", otherRaw+"x") {
		t.Fatal("流程指纹必须包含原始参数 JSON")
	}
}

// TestR3107LegacyStateHashRejected 旧版本 config_hash（不含流程代际/模式）在 Exchange 前拒绝，避免旧 state 重新有效。
func TestR3107LegacyStateHashRejected(t *testing.T) {
	st, svc, _ := newTestOidcService(t)
	if _, err := st.DB().Exec(
		`INSERT INTO oidc_states (state, code_verifier, nonce, intent, provider_type, config_hash, redirect_uri)
		 VALUES ('r3107-legacy-hash','v','n','login','mock','legacy-v1-hash','http://vpn.example.com/api/auth/oidc/callback')`); err != nil {
		t.Fatalf("写入旧 hash state 失败: %v", err)
	}
	rec, err := svc.ConsumeState(ctx, "r3107-legacy-hash")
	if err != nil {
		t.Fatalf("ConsumeState 失败: %v", err)
	}
	if _, err := svc.Exchange(ctx, rec, "code"); !errors.Is(err, ErrOidcFlowInvalid) {
		t.Fatalf("旧版本 state hash 必须在 Exchange 前拒绝，实际: %v", err)
	}
}
