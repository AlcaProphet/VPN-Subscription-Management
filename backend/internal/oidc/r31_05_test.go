package oidc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"vpn-sub/internal/config"
)

// TestR3105StartFlowPinsRedirectURI StartFlow 必须在 state 中固定当次 redirect_uri；
// 独立回调优先，清除后回退 frontend_url 推导；仅改地址不改变 R31-03 的 provider/config 指纹。
func TestR3105StartFlowPinsRedirectURI(t *testing.T) {
	st, svc, _ := newTestOidcService(t)
	fallback := "http://vpn.example.com" + config.OidcCallbackPath
	authURL, state, err := svc.StartFlow(ctx, "login", 0)
	if err != nil {
		t.Fatalf("StartFlow 失败: %v", err)
	}
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("解析授权 URL 失败: %v", err)
	}
	if got := u.Query().Get("redirect_uri"); got != fallback {
		t.Fatalf("未设置独立回调时应使用推导值 %q，实际 %q", fallback, got)
	}
	var pinned, providerType, configHash string
	if err := st.DB().QueryRow(
		`SELECT redirect_uri, provider_type, config_hash FROM oidc_states WHERE state = ?`, state).
		Scan(&pinned, &providerType, &configHash); err != nil {
		t.Fatalf("查询 state 固定值失败: %v", err)
	}
	if pinned != fallback {
		t.Fatalf("state 固定 redirect_uri 异常: %q", pinned)
	}
	firstHash := configHash

	// 保存独立回调后，新 flow 使用独立值。
	independent := "https://callback.example.com" + config.OidcCallbackPath
	if err := svc.cfg.Set(ctx, config.KeyCallbackURL, independent); err != nil {
		t.Fatalf("设置独立回调失败: %v", err)
	}
	authURL, state, err = svc.StartFlow(ctx, "login", 0)
	if err != nil {
		t.Fatalf("独立回调 StartFlow 失败: %v", err)
	}
	u, _ = url.Parse(authURL)
	if got := u.Query().Get("redirect_uri"); got != independent {
		t.Fatalf("独立回调应优先: got=%q want=%q", got, independent)
	}
	if err := st.DB().QueryRow(
		`SELECT redirect_uri, provider_type, config_hash FROM oidc_states WHERE state = ?`, state).
		Scan(&pinned, &providerType, &configHash); err != nil {
		t.Fatalf("查询独立回调 state 失败: %v", err)
	}
	if pinned != independent {
		t.Fatalf("state 应固定独立回调: %q", pinned)
	}
	if configHash != firstHash {
		t.Fatalf("仅改地址不得改变 R31-03 配置指纹: before=%s after=%s", firstHash, configHash)
	}

	// 显式清除后新 flow 回退推导。
	if err := svc.cfg.Set(ctx, config.KeyCallbackURL, ""); err != nil {
		t.Fatalf("清除独立回调失败: %v", err)
	}
	authURL, state, err = svc.StartFlow(ctx, "login", 0)
	if err != nil {
		t.Fatalf("清除后 StartFlow 失败: %v", err)
	}
	u, _ = url.Parse(authURL)
	if got := u.Query().Get("redirect_uri"); got != fallback {
		t.Fatalf("清除后应回退推导 %q，实际 %q", fallback, got)
	}

	// frontend_url 带反代前缀时，推导值路径可不同于根路径；Exchange 不得按显式独立回调的精确路径二次拒绝。
	if err := svc.cfg.Set(ctx, config.KeyFrontendURL, "http://vpn.example.com/prefix"); err != nil {
		t.Fatalf("设置带前缀前端地址失败: %v", err)
	}
	prefixedFallback := "http://vpn.example.com/prefix" + config.OidcCallbackPath
	authURL, state, err = svc.StartFlow(ctx, "login", 0)
	if err != nil {
		t.Fatalf("带前缀 StartFlow 失败: %v", err)
	}
	if u, _ = url.Parse(authURL); u.Query().Get("redirect_uri") != prefixedFallback {
		t.Fatalf("带前缀推导异常: %s", authURL)
	}
	rec, err := svc.ConsumeState(ctx, state)
	if err != nil {
		t.Fatalf("ConsumeState 失败: %v", err)
	}
	if _, err := svc.Exchange(ctx, rec, "invalid-code"); err == nil || strings.Contains(err.Error(), "回调地址无效") {
		t.Fatalf("带前缀推导值应通过 redirect_uri 校验并进入 mock 解析: %v", err)
	}
}

// TestR3105ConsumeStateRejectsMissingRedirectURI 迁移前已存在的、无固定 redirect_uri 的 state
// 必须在消费阶段拒绝并保留原行（由 TTL 清理）。
func TestR3105ConsumeStateRejectsMissingRedirectURI(t *testing.T) {
	st, svc, _ := newTestOidcService(t)
	if _, err := st.DB().Exec(
		`INSERT INTO oidc_states (state, code_verifier, nonce, intent, provider_type, config_hash) VALUES ('legacy-no-redirect','v','n','login','mock','hash')`); err != nil {
		t.Fatalf("写入旧 state 失败: %v", err)
	}
	if _, err := svc.ConsumeState(ctx, "legacy-no-redirect"); err == nil {
		t.Fatal("缺少 redirect_uri 的 state 必须拒绝")
	}
	var count int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM oidc_states WHERE state='legacy-no-redirect'`).Scan(&count); err != nil {
		t.Fatalf("查询旧 state 失败: %v", err)
	}
	if count != 1 {
		t.Fatalf("旧 state 行应保留，实际 %d", count)
	}
}

// TestR3105ExchangeUsesPinnedRedirectURI 授权期间改址后，token 交换仍携带发起时固定的 redirect_uri。
func TestR3105ExchangeUsesPinnedRedirectURI(t *testing.T) {
	var mu sync.Mutex
	var tokenForms []url.Values
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration") {
			base := strings.TrimSuffix(r.URL.Path, "/.well-known/openid-configuration")
			_ = json.NewEncoder(w).Encode(Discovery{
				AuthorizationEndpoint: srvURL(r) + base + "/authorize",
				TokenEndpoint:         srvURL(r) + base + "/token",
			})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/token") {
			_ = r.ParseForm()
			mu.Lock()
			tokenForms = append(tokenForms, r.PostForm)
			mu.Unlock()
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	_, svc, _ := newTestOidcService(t)
	svc.httpCli.Transport = srv.Client().Transport
	svc.ClearDiscCache()
	baseURL := srv.URL + "/a"
	if err := svc.SaveParams(ctx, "generic", Params{BaseURL: baseURL, ClientID: "client", ClientSecret: "secret"}); err != nil {
		t.Fatalf("保存 generic 参数失败: %v", err)
	}
	if err := svc.cfg.Set(ctx, KeyProviderType, "generic"); err != nil {
		t.Fatalf("设置 provider 失败: %v", err)
	}
	oldCallback := "https://old-callback.example.com" + config.OidcCallbackPath
	newCallback := "https://new-callback.example.com" + config.OidcCallbackPath
	if err := svc.cfg.Set(ctx, config.KeyCallbackURL, oldCallback); err != nil {
		t.Fatalf("设置旧回调失败: %v", err)
	}
	authURL, state, err := svc.StartFlow(ctx, "login", 0)
	if err != nil {
		t.Fatalf("StartFlow 失败: %v", err)
	}
	if u, _ := url.Parse(authURL); u.Query().Get("redirect_uri") != oldCallback {
		t.Fatalf("授权 URL 应使用旧回调: %s", authURL)
	}
	// 授权发起后改址；进行中 state 不应因地址变化失效。
	if err := svc.cfg.Set(ctx, config.KeyCallbackURL, newCallback); err != nil {
		t.Fatalf("更新回调失败: %v", err)
	}
	rec, err := svc.ConsumeState(ctx, state)
	if err != nil {
		t.Fatalf("ConsumeState 失败: %v", err)
	}
	if _, err := svc.Exchange(ctx, rec, "old-code"); err == nil || !strings.Contains(err.Error(), "token 交换失败") {
		t.Fatalf("token 端点应被触达并返回失败，实际: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(tokenForms) != 1 {
		t.Fatalf("应捕获 1 次 token 表单，实际 %d", len(tokenForms))
	}
	if got := tokenForms[0].Get("redirect_uri"); got != oldCallback {
		t.Fatalf("token 交换必须复用固定旧 redirect_uri: got=%q want=%q", got, oldCallback)
	}
	// 新发起的授权使用新地址。
	newAuthURL, _, err := svc.StartFlow(ctx, "login", 0)
	if err != nil {
		t.Fatalf("新 StartFlow 失败: %v", err)
	}
	if u, _ := url.Parse(newAuthURL); u.Query().Get("redirect_uri") != newCallback {
		t.Fatalf("新授权应使用新回调: %s", newAuthURL)
	}
}
