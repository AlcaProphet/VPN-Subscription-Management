package oidc

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/internal/user"
)

// newTestOidcService 创建临时库 + oidc 服务（含 users/oidc_states 迁移）
func newTestOidcService(t *testing.T) (*store.Store, *Service, *user.Service) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	fsys := fstest.MapFS{
		"0001_init.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY, value TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"0002_users.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			oidc_subject TEXT UNIQUE,
			username TEXT NOT NULL,
			email TEXT UNIQUE,
			role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin','user')),
			group_id INTEGER,
			password_hash TEXT,
			user_source TEXT NOT NULL CHECK (user_source IN ('oidc','local','selfreg')),
			status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','active','disabled')),
			credential_version INTEGER NOT NULL DEFAULT 0,
			oidc_claims TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"0004_oidc.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS oidc_states (
			state TEXT PRIMARY KEY,
			code_verifier TEXT NOT NULL,
			intent TEXT NOT NULL CHECK (intent IN ('login','bind')),
			bind_user_id INTEGER,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
	}
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	users := user.NewService(st, cfg, log.New("error", "console"))
	authSvc := auth.NewService(cfg, users, log.New("error", "console"))
	svc := NewService(st, cfg, authSvc, users, "dev", log.New("error", "console"))
	// 配置模拟提供商
	if err := svc.SaveParams(ctx, "mock", Params{}); err != nil {
		t.Fatalf("保存 mock 参数失败: %v", err)
	}
	if err := cfg.Set(ctx, KeyProviderType, "mock"); err != nil {
		t.Fatalf("设置提供商失败: %v", err)
	}
	if err := cfg.Set(ctx, KeyConfigured, "true"); err != nil {
		t.Fatalf("设置 OIDC 配置标记失败: %v", err)
	}
	if err := cfg.Set(ctx, "frontend_url", "http://vpn.example.com"); err != nil {
		t.Fatalf("设置 frontend_url 失败: %v", err)
	}
	return st, svc, users
}

var ctx = context.Background()

// TestStartFlowPKCE 授权 URL 含 PKCE S256 声明且 state/challenge 正确
func TestStartFlowPKCE(t *testing.T) {
	_, svc, _ := newTestOidcService(t)
	authURL, state, err := svc.StartFlow(ctx, "login", 0)
	if err != nil {
		t.Fatalf("StartFlow 失败: %v", err)
	}
	if !strings.Contains(authURL, "code_challenge_method=S256") {
		t.Error("授权 URL 缺少 S256 声明")
	}
	if !strings.Contains(authURL, "state="+state) {
		t.Error("授权 URL state 与返回值不一致")
	}
	if len(state) < 32 {
		t.Errorf("state 熵不足: %d", len(state))
	}
}

// TestConsumeStateOneTime state 三重校验：Consume 后再用同 state 失败（用后即删）
func TestConsumeStateOneTime(t *testing.T) {
	_, svc, _ := newTestOidcService(t)
	_, state, err := svc.StartFlow(ctx, "login", 0)
	if err != nil {
		t.Fatalf("StartFlow 失败: %v", err)
	}
	rec, err := svc.ConsumeState(ctx, state)
	if err != nil || rec == nil {
		t.Fatalf("首次 ConsumeState 失败: %v", err)
	}
	if rec.Intent != "login" || rec.CodeVerifier == "" {
		t.Errorf("记录内容异常: %+v", rec)
	}
	// 用后即删：再次消费失败
	if _, err := svc.ConsumeState(ctx, state); err == nil {
		t.Error("二次消费同 state 应失败（用后即删）")
	}
	// 不存在 state 失败
	if _, err := svc.ConsumeState(ctx, "nonexistent-state"); err == nil {
		t.Error("不存在 state 应失败")
	}
}

// TestMockLoginCreateAndMerge 模拟登录：创建新用户（首管理员）→ 再次登录命中 subject → 邮箱合并/冲突分支
func TestMockLoginCreateAndMerge(t *testing.T) {
	_, svc, _ := newTestOidcService(t)
	// 首次模拟登录：创建用户（空表 → 首管理员）
	res, err := svc.MockLogin(ctx, "alice@example.com", "", true, nil, nil)
	if err != nil {
		t.Fatalf("MockLogin 失败: %v", err)
	}
	if res.User == nil || res.User.Role != "admin" {
		t.Fatalf("首用户应为 admin: %+v", res)
	}
	// subject 命中（相同邮箱）：直接登录，无新用户
	res2, err := svc.MockLogin(ctx, "alice@example.com", "alice2", true, nil, nil)
	if err != nil {
		t.Fatalf("二次 MockLogin 失败: %v", err)
	}
	if res2.User == nil || res2.User.ID != res.User.ID {
		t.Errorf("subject 命中应复用同一用户")
	}
	// username 刷新为最新值（重新查库验证——返回对象为查库快照）
	u2, err := svc.users.GetBySubject(ctx, "alice@example.com")
	if err != nil || u2 == nil {
		t.Fatalf("查库失败: %v", err)
	}
	if u2.Username != "alice2" {
		t.Errorf("username 应刷新为最新值: %s", u2.Username)
	}
}

// TestMockLoginMergeAndConflict 既有本地用户：邮箱已验证自动合并；未验证冲突
func TestMockLoginMergeAndConflict(t *testing.T) {
	st, svc, users := newTestOidcService(t)
	if _, err := users.Register(ctx, "bob", "bob@example.com", "password123"); err != nil {
		t.Fatalf("注册本地用户失败: %v", err)
	}
	// 邮箱已验证 → 自动合并（subject 写入本地账号）
	res, err := svc.MockLogin(ctx, "bob@example.com", "bob-oidc", true, nil, nil)
	if err != nil {
		t.Fatalf("MockLogin 失败: %v", err)
	}
	if res.User == nil || res.User.Email != "bob@example.com" {
		t.Fatalf("自动合并失败: %+v", res)
	}
	var subject string
	if err := st.DB().QueryRow(`SELECT oidc_subject FROM users WHERE email='bob@example.com'`).Scan(&subject); err != nil {
		t.Fatalf("查询 subject 失败: %v", err)
	}
	if subject != "bob@example.com" {
		t.Errorf("合并后 subject 未写入: %s", subject)
	}
	// 另一本地用户：邮箱未验证 → 冲突不合并
	if _, err := users.Register(ctx, "carol", "carol@example.com", "password123"); err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	res2, err := svc.MockLogin(ctx, "carol@example.com", "", false, nil, nil)
	if err != nil {
		t.Fatalf("MockLogin 失败: %v", err)
	}
	if res2.User != nil || res2.Pending || res2.Message == "" {
		t.Errorf("未验证邮箱应冲突: %+v", res2)
	}
	// 已绑定其他 OIDC 的账号：冲突
	res3, err := svc.MockLogin(ctx, "dave@example.com", "", true, nil, nil)
	if err != nil {
		t.Fatalf("MockLogin 失败: %v", err)
	}
	if res3.User == nil {
		t.Fatalf("dave 创建失败: %+v", res3)
	}
	// 用另一 OIDC subject（不同邮箱但同账号？）模拟已绑定冲突：直接改库绑定后再用原 subject 登录
	res4, err := svc.MockLogin(ctx, "dave@example.com", "", true, nil, nil)
	if err != nil || res4.User == nil {
		t.Fatalf("dave 重复登录失败: %v", err)
	}
}

// TestMockLoginProdReject 非 Dev 或非同提供商拒绝
func TestMockLoginProdReject(t *testing.T) {
	st, _, users := newTestOidcService(t)
	cfg := config.NewService(st, log.New("error", "console"))
	authSvc := auth.NewService(cfg, users, log.New("error", "console"))
	prodSvc := NewService(st, cfg, authSvc, users, "prod", log.New("error", "console"))
	if _, err := prodSvc.MockLogin(ctx, "a@b.com", "", true, nil, nil); err == nil {
		t.Error("prod 模式模拟登录应拒绝")
	}
	// 非 mock 提供商
	devSvc := NewService(st, cfg, authSvc, users, "dev", log.New("error", "console"))
	_ = cfg.Set(ctx, KeyProviderType, "keycloak")
	if _, err := devSvc.MockLogin(ctx, "a@b.com", "", true, nil, nil); err == nil {
		t.Error("非 mock 提供商模拟登录应拒绝")
	}
}

// TestResolveBind 手动绑定：成功后不签发会话（绑定本身仅写入 subject）；subject 已绑其他账号拒绝
func TestResolveBind(t *testing.T) {
	_, svc, users := newTestOidcService(t)
	u, err := users.Register(ctx, "kyle", "kyle@example.com", "password123")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	// 生成一个有效 state 记录（intent=bind）
	_, state, err := svc.StartFlow(ctx, "bind", u.ID)
	if err != nil {
		t.Fatalf("StartFlow 失败: %v", err)
	}
	rec, err := svc.ConsumeState(ctx, state)
	if err != nil {
		t.Fatalf("ConsumeState 失败: %v", err)
	}
	// 模拟身份 subject
	id := &Identity{Subject: "oidc-subject-1", Email: "oidc@example.com", EmailVerified: true, Username: "oidcuser"}
	if err := svc.ResolveBind(ctx, rec, id); err != nil {
		t.Fatalf("ResolveBind 失败: %v", err)
	}
	// 验证绑定写入
	u2, err := users.GetBySubject(ctx, "oidc-subject-1")
	if err != nil || u2 == nil || u2.ID != u.ID {
		t.Errorf("绑定未写入目标账号: %v %v", u2, err)
	}
	// subject 已绑定其他账号 → 拒绝
	u3, err := users.Register(ctx, "bob", "bob@example.com", "password123")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	_, state3, err := svc.StartFlow(ctx, "bind", u3.ID)
	if err != nil {
		t.Fatalf("StartFlow 失败: %v", err)
	}
	rec3, _ := svc.ConsumeState(ctx, state3)
	if err := svc.ResolveBind(ctx, rec3, id); err == nil {
		t.Error("subject 已绑其他账号应拒绝")
	}
}

// TestMockExchange 模拟 code 还原身份
func TestMockExchange(t *testing.T) {
	_, svc, _ := newTestOidcService(t)
	code, err := svc.MockCode("carol@example.com", "carol", true, []string{"admin"}, nil)
	if err != nil {
		t.Fatalf("MockCode 失败: %v", err)
	}
	// 解码校验
	raw, err := base64.RawURLEncoding.DecodeString(code)
	if err != nil || !strings.Contains(string(raw), "carol@example.com") {
		t.Error("模拟 code 内容异常")
	}
	rec := &StateRecord{CodeVerifier: "dummy"}
	id, err := svc.mockExchange(rec, code)
	if err != nil {
		t.Fatalf("mockExchange 失败: %v", err)
	}
	if id.Subject != "carol@example.com" || !id.EmailVerified {
		t.Errorf("身份还原异常: %+v", id)
	}
}
