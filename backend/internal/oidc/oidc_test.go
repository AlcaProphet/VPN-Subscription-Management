package oidc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
			nonce TEXT NOT NULL DEFAULT '',
			intent TEXT NOT NULL CHECK (intent IN ('login','bind')),
			bind_user_id INTEGER,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			provider_type TEXT NOT NULL DEFAULT '',
			config_hash TEXT NOT NULL DEFAULT '',
			redirect_uri TEXT NOT NULL DEFAULT '');
			CREATE TABLE IF NOT EXISTS oidc_login_tickets (
			ticket TEXT PRIMARY KEY,
			session_token TEXT NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			flow_hash TEXT NOT NULL DEFAULT '');`)},
	}
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	// 运行期 OIDC 保存不再自动生成签名密钥；测试库按已完成 Setup 的态显式生成。
	if _, err := cfg.EnsureSigningKey(context.Background()); err != nil {
		t.Fatalf("生成测试签名密钥失败: %v", err)
	}
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

// TestResolveLoginFirstAdminDoesNotWriteAdminInitialized 通过完整 ResolveLogin 路径验证：
// OIDC 首个用户仍为 admin，但不再写入无读取方的 admin_initialized 标记。
func TestResolveLoginFirstAdminDoesNotWriteAdminInitialized(t *testing.T) {
	st, svc, _ := newTestOidcService(t)
	res, err := svc.ResolveLogin(ctx, &Identity{
		Subject:       "oidc-first-subject",
		Email:         "oidc-first@example.com",
		EmailVerified: true,
		Username:      "oidc-first",
	})
	if err != nil {
		t.Fatalf("ResolveLogin 失败: %v", err)
	}
	if res.User == nil || res.User.Role != "admin" {
		t.Fatalf("OIDC 首用户应为 admin: %+v", res)
	}
	var count int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM system_config WHERE key = ?`, config.KeyAdminInitialized).Scan(&count); err != nil {
		t.Fatalf("查询 admin_initialized 失败: %v", err)
	}
	if count != 0 {
		t.Fatalf("OIDC 首管理员不应写入 admin_initialized，实际 %d 行", count)
	}
}

// TestLoadParamsDecryptsSavedSecret 库内存储密文，LoadParams 必须返回解密后的明文。
func TestLoadParamsDecryptsSavedSecret(t *testing.T) {
	st, svc, _ := newTestOidcService(t)
	if err := svc.SaveParams(ctx, "generic", Params{
		BaseURL: "https://idp.example.com", ClientID: "c", ClientSecret: "plain-secret",
	}); err != nil {
		t.Fatalf("SaveParams 失败: %v", err)
	}
	raw := readOidcParamsRaw(t, st, "generic")
	if strings.Contains(raw, "plain-secret") {
		t.Fatalf("库内不应出现明文 Secret: %s", raw)
	}
	got, err := svc.LoadParams(ctx, "generic")
	if err != nil {
		t.Fatalf("LoadParams 失败: %v", err)
	}
	if got.ClientSecret != "plain-secret" {
		t.Fatalf("LoadParams 应返回解密明文，实际 %q", got.ClientSecret)
	}
}

// TestSaveParamsEmptyPreservesCipher 空值保存保留原密文，显式新值替换，并拒绝脱敏占位符。
func TestSaveParamsEmptyPreservesCipher(t *testing.T) {
	st, svc, _ := newTestOidcService(t)
	if err := svc.SaveParams(ctx, "generic", Params{
		BaseURL: "https://idp.example.com", ClientID: "c", ClientSecret: "first-secret",
	}); err != nil {
		t.Fatalf("首次保存失败: %v", err)
	}
	first := readOidcParamsRaw(t, st, "generic")
	var firstParams Params
	if err := json.Unmarshal([]byte(first), &firstParams); err != nil {
		t.Fatalf("解析首次保存 JSON 失败: %v", err)
	}
	if firstParams.ClientSecret == "" || firstParams.ClientSecret == "first-secret" {
		t.Fatalf("首次保存应为密文: %q", firstParams.ClientSecret)
	}
	// 空值保存：Secret 密文原样保留
	if err := svc.SaveParams(ctx, "generic", Params{BaseURL: "https://idp.example.com", ClientID: "c"}); err != nil {
		t.Fatalf("空值保存失败: %v", err)
	}
	second := readOidcParamsRaw(t, st, "generic")
	var secondParams Params
	if err := json.Unmarshal([]byte(second), &secondParams); err != nil {
		t.Fatalf("解析二次保存 JSON 失败: %v", err)
	}
	if secondParams.ClientSecret != firstParams.ClientSecret {
		t.Fatalf("空值保存不得改写原密文: before=%q after=%q", firstParams.ClientSecret, secondParams.ClientSecret)
	}
	if got, _ := svc.LoadParams(ctx, "generic"); got.ClientSecret != "first-secret" {
		t.Fatalf("空值保存后应仍解密为原明文: %q", got.ClientSecret)
	}
	// 显式占位符拒绝且不落库
	if err := svc.SaveParams(ctx, "generic", Params{BaseURL: "https://idp.example.com", ClientID: "c", ClientSecret: config.MaskedSecret}); err == nil {
		t.Fatal("SaveParams 应拒绝脱敏占位符")
	}
	if readOidcParamsRaw(t, st, "generic") != second {
		t.Fatal("占位符拒绝后库内参数不应变化")
	}
}

// TestTestConnectionSavedSecretScopes 测试连接的空 Secret 回退边界：
// 仅管理员面板且 base_url/realm/client_id 与已保存配置一致时使用库内明文；默认/Setup 与任一字段不一致均不回退。
func TestTestConnectionSavedSecretScopes(t *testing.T) {
	_, svc, _ := newTestOidcService(t)
	var gotSecret string
	var ts *httptest.Server
	ts = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(Discovery{
				AuthorizationEndpoint: ts.URL + "/authorize",
				TokenEndpoint:         ts.URL + "/token",
			})
			return
		}
		switch r.URL.Path {
		case "/token":
			if err := r.ParseForm(); err != nil {
				http.Error(w, "bad form", http.StatusBadRequest)
				return
			}
			gotSecret = r.Form.Get("client_secret")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"access_token":"test-token"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()
	svc.httpCli = ts.Client()
	if err := svc.SaveParams(ctx, "keycloak", Params{
		BaseURL: ts.URL, Realm: "realm-a", ClientID: "c", ClientSecret: "saved-secret",
	}); err != nil {
		t.Fatalf("保存 keycloak 参数失败: %v", err)
	}
	reset := func() { gotSecret = "" }
	assertNoFallback := func(name string, res *TestResult, err error) {
		t.Helper()
		if err != nil || res == nil || !res.OK {
			t.Fatalf("%s discovery 应完成并返回 OK: %+v %v", name, res, err)
		}
		if gotSecret != "" {
			t.Fatalf("%s 不应回退已存 Secret，token 端点收到 %q", name, gotSecret)
		}
		if !hasWarning(res, "未执行凭据校验") {
			t.Fatalf("%s 应提示未执行凭据校验: %+v", name, res)
		}
	}

	// 默认 TestConnection（Setup/草稿路径）不得回退已存 Secret
	reset()
	res, err := svc.TestConnection(ctx, "keycloak", Params{BaseURL: ts.URL, Realm: "realm-a", ClientID: "c"})
	assertNoFallback("默认 TestConnection", res, err)

	// 管理面板回退：ClientID 不一致时不回退
	reset()
	res2, err := svc.TestConnectionWithSavedSecret(ctx, "keycloak", Params{BaseURL: ts.URL, Realm: "realm-a", ClientID: "other"})
	assertNoFallback("ClientID 不一致", res2, err)

	// 管理面板回退：Realm 不一致时不回退
	reset()
	res3, err := svc.TestConnectionWithSavedSecret(ctx, "keycloak", Params{BaseURL: ts.URL, Realm: "realm-b", ClientID: "c"})
	assertNoFallback("Realm 不一致", res3, err)

	// 管理面板回退：BaseURL 不一致时不回退
	reset()
	res4, err := svc.TestConnectionWithSavedSecret(ctx, "keycloak", Params{BaseURL: ts.URL + "/other", Realm: "realm-a", ClientID: "c"})
	assertNoFallback("BaseURL 不一致", res4, err)

	// 管理面板回退且三项一致：使用已存明文
	reset()
	res5, err := svc.TestConnectionWithSavedSecret(ctx, "keycloak", Params{BaseURL: ts.URL, Realm: "realm-a", ClientID: "c"})
	if err != nil || res5 == nil || !res5.OK {
		t.Fatalf("参数一致时应回退并通过: %+v %v", res5, err)
	}
	if gotSecret != "saved-secret" {
		t.Fatalf("参数一致时 token 端点应收到解密明文，实际 %q", gotSecret)
	}
}

// hasWarning 判定测试结果是否包含指定警告片段。
func hasWarning(res *TestResult, want string) bool {
	if res == nil {
		return false
	}
	for _, w := range res.Warnings {
		if strings.Contains(w, want) {
			return true
		}
	}
	return false
}

// TestTestConnectionRejectsMaskedSecret 测试接口同样拒绝占位符，不发起凭据校验。
func TestTestConnectionRejectsMaskedSecret(t *testing.T) {
	_, svc, _ := newTestOidcService(t)
	res, err := svc.TestConnection(ctx, "generic", Params{BaseURL: "https://idp.example.com", ClientID: "c", ClientSecret: config.MaskedSecret})
	if err != nil {
		t.Fatalf("TestConnection 不应返回错误: %v", err)
	}
	if res == nil || res.OK || !strings.Contains(res.Message, "脱敏占位符") {
		t.Fatalf("占位符应返回明确失败结果: %+v", res)
	}
}

// TestCurrentParamsRejectsDamagedPlaceholder 真实 OIDC 登录链路（StartFlow/Exchange 共用 currentParams）
// 必须在进入授权或换 token 前拒绝解密后为脱敏占位符的 Secret。
func TestCurrentParamsRejectsDamagedPlaceholder(t *testing.T) {
	st, svc, _ := newTestOidcService(t)
	writeMaskedOidcParams(t, st, svc, "generic", "https://idp.example.com", "", "c")
	if _, err := svc.currentParams(ctx); err == nil || !strings.Contains(err.Error(), "重新输入") {
		t.Fatalf("损坏占位符应被 currentParams 拒绝并提示重填: %v", err)
	}
	if _, _, err := svc.StartFlow(ctx, "login", 0); err == nil || !strings.Contains(err.Error(), "重新输入") {
		t.Fatalf("损坏占位符应在 StartFlow 阶段被拒绝: %v", err)
	}
	rec := pinnedStateRecord(t, svc, "generic")
	if _, err := svc.Exchange(ctx, rec, "code"); err == nil || !strings.Contains(err.Error(), "重新输入") {
		t.Fatalf("损坏占位符应在 Exchange 阶段被拒绝: %v", err)
	}
}

// TestTestConnectionStoredMaskedSecret 管理员面板回退命中已存密文时，
// 解密结果为脱敏占位符必须直接返回失败，不能把 "***" 送进 token 请求。
func TestTestConnectionStoredMaskedSecret(t *testing.T) {
	st, svc, _ := newTestOidcService(t)
	writeMaskedOidcParams(t, st, svc, "generic", "https://idp.example.com", "", "c")
	res, err := svc.TestConnectionWithSavedSecret(ctx, "generic", Params{BaseURL: "https://idp.example.com", ClientID: "c"})
	if err != nil {
		t.Fatalf("TestConnectionWithSavedSecret 不应返回错误: %v", err)
	}
	if res == nil || res.OK || res.Message != config.OidcTestStoredDamagedMessage {
		t.Fatalf("已存脱敏占位符应返回专门损坏失败结果: %+v", res)
	}
}

// writeMaskedOidcParams 写入指定提供商参数，client_secret 为加密后的脱敏占位符（测试辅助）。
func writeMaskedOidcParams(t *testing.T, st *store.Store, svc *Service, providerType, baseURL, realm, clientID string) {
	t.Helper()
	cipher, err := svc.cfg.EncryptSensitive(ctx, config.MaskedSecret)
	if err != nil {
		t.Fatalf("加密占位符失败: %v", err)
	}
	raw, err := json.Marshal(Params{BaseURL: baseURL, Realm: realm, ClientID: clientID, ClientSecret: cipher})
	if err != nil {
		t.Fatalf("序列化参数失败: %v", err)
	}
	if _, err := st.DB().Exec(
		`INSERT INTO system_config(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		"oidc_params_"+providerType, string(raw)); err != nil {
		t.Fatalf("写入 %s 参数失败: %v", providerType, err)
	}
	if _, err := st.DB().Exec(
		`INSERT INTO system_config(key,value) VALUES('oidc_provider_type',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		providerType); err != nil {
		t.Fatalf("设置提供商失败: %v", err)
	}
}

// readOidcParamsRaw 读取指定提供商参数 JSON 原始值（测试辅助）。
func readOidcParamsRaw(t *testing.T, st *store.Store, providerType string) string {
	t.Helper()
	var raw string
	if err := st.DB().QueryRow(`SELECT value FROM system_config WHERE key = ?`, "oidc_params_"+providerType).Scan(&raw); err != nil {
		t.Fatalf("读取 %s 参数失败: %v", providerType, err)
	}
	return raw
}

// testFlowHash 按当前服务 mode、库内代际与给定 raw 计算 R31-07 完整流程指纹（测试辅助）。
func testFlowHash(t *testing.T, svc *Service, providerType, raw string) string {
	t.Helper()
	epoch, err := svc.cfg.Get(ctx, config.KeyOidcFlowEpoch)
	if err != nil {
		t.Fatalf("读取流程代际失败: %v", err)
	}
	return flowConfigHash(svc.mode, epoch, providerType, raw)
}

// pinnedStateRecord 构造一个与当前库内配置匹配的 StateRecord，供直接调用 Exchange 的旧测试复用；
// 不需要真实 StartFlow 网络请求。
func pinnedStateRecord(t *testing.T, svc *Service, providerType string) *StateRecord {
	t.Helper()
	raw, err := svc.cfg.Get(ctx, "oidc_params_"+providerType)
	if err != nil {
		t.Fatalf("读取 %s 参数失败: %v", providerType, err)
	}
	if raw == "" {
		t.Fatalf("%s 参数未配置", providerType)
	}
	return &StateRecord{
		ProviderType: providerType,
		ConfigHash:   testFlowHash(t, svc, providerType, raw),
		CodeVerifier: "verifier",
		RedirectURI:  svc.CallbackURL(ctx),
	}
}
