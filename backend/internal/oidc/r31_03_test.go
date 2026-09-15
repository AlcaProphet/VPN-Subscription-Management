package oidc

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"vpn-sub/internal/config"
)

// setRawOidcParams 写入指定提供商的原始参数 JSON（测试辅助；允许写入损坏 JSON）。
func setRawOidcParams(t *testing.T, svc *Service, providerType, raw string) {
	t.Helper()
	if err := svc.cfg.Set(ctx, "oidc_params_"+providerType, raw); err != nil {
		t.Fatalf("写入 %s 原始参数失败: %v", providerType, err)
	}
}

// TestR3103DescribeParamsStates 目标读取状态判定：可用、尚缺 Secret、Secret 损坏、JSON 损坏、签名密钥故障。
func TestR3103DescribeParamsStates(t *testing.T) {
	_, svc, _ := newTestOidcService(t)
	if err := svc.SaveParams(ctx, "generic", Params{BaseURL: "https://idp.example.com", ClientID: "c", ClientSecret: "secret"}); err != nil {
		t.Fatalf("保存显式 Secret 失败: %v", err)
	}
	st, err := svc.DescribeParams(ctx, "generic")
	if err != nil {
		t.Fatalf("DescribeParams 可用状态失败: %v", err)
	}
	if !st.Present || !st.SecretUsable || st.SecretDamaged || st.JSONDamaged || st.BaseURL != "https://idp.example.com" || st.ClientID != "c" {
		t.Fatalf("可用状态异常: %+v", st)
	}

	// 空 Secret 保存：字段一致时保留原密文，仍为可用。
	if err := svc.SaveParams(ctx, "generic", Params{BaseURL: "https://idp.example.com", ClientID: "c"}); err != nil {
		t.Fatalf("空 Secret 字段一致保存失败: %v", err)
	}
	st, _ = svc.DescribeParams(ctx, "generic")
	if !st.SecretUsable {
		t.Fatalf("空 Secret 保留后应仍为可用: %+v", st)
	}

	// 尚缺可用 Secret。
	setRawOidcParams(t, svc, "generic", `{"base_url":"https://idp.example.com","client_id":"c","client_secret":""}`)
	st, err = svc.DescribeParams(ctx, "generic")
	if err != nil || !st.Present || st.SecretUsable || st.SecretDamaged {
		t.Fatalf("空 Secret 应为尚缺状态: %+v %v", st, err)
	}

	// 字面 ***（非密文）按 Secret 损坏处理，但仍保留可解析字段。
	setRawOidcParams(t, svc, "generic", `{"base_url":"https://idp.example.com","client_id":"c","client_secret":"***"}`)
	st, err = svc.DescribeParams(ctx, "generic")
	if err != nil || !st.SecretDamaged || st.SecretUsable || st.BaseURL != "https://idp.example.com" {
		t.Fatalf("字面 *** 应为 Secret 损坏并保留字段: %+v %v", st, err)
	}

	// 加密后解密为 *** 同样按 Secret 损坏处理。
	writeMaskedOidcParams(t, svc.store, svc, "generic", "https://idp.example.com", "", "c")
	st, err = svc.DescribeParams(ctx, "generic")
	if err != nil || !st.SecretDamaged {
		t.Fatalf("解密后 *** 应标 Secret 损坏: %+v %v", st, err)
	}

	// 非法 JSON 标 JSON 损坏，不猜测字段。
	setRawOidcParams(t, svc, "auth0", `{`)
	st, err = svc.DescribeParams(ctx, "auth0")
	if err != nil || !st.Present || !st.JSONDamaged || st.BaseURL != "" {
		t.Fatalf("非法 JSON 应标 JSON 损坏: %+v %v", st, err)
	}

	// 签名密钥缺失为独立错误，不伪装成 Secret 损坏。
	_, svc2, _ := newTestOidcService(t)
	setRawOidcParams(t, svc2, "generic", `{"base_url":"https://idp.example.com","client_id":"c","client_secret":"not-a-cipher"}`)
	if _, err := svc2.DescribeParams(ctx, "generic"); err == nil || !strings.Contains(err.Error(), "签名密钥") {
		t.Fatalf("签名密钥缺失应返回独立错误: %v", err)
	}
}

// TestR3103SaveParamsEmptySecretAtomicGuard 空 Secret 原子守卫：目标缺失/字段变化/损坏时拒绝且 raw JSON 不变。
func TestR3103SaveParamsEmptySecretAtomicGuard(t *testing.T) {
	st, svc, _ := newTestOidcService(t)
	// 目标无参数时拒绝。
	if err := svc.SaveParams(ctx, "generic", Params{BaseURL: "https://idp.example.com", ClientID: "c"}); !errors.Is(err, config.ErrBadRequest) {
		t.Fatalf("目标无旧 Secret 应返回 ErrBadRequest: %v", err)
	}
	// 写入可用 Secret 后，字段变化必须拒绝且 raw 不变。
	if err := svc.SaveParams(ctx, "generic", Params{BaseURL: "https://idp.example.com", ClientID: "c", ClientSecret: "secret"}); err != nil {
		t.Fatalf("首次保存失败: %v", err)
	}
	before := readOidcParamsRaw(t, st, "generic")
	if err := svc.SaveParams(ctx, "generic", Params{BaseURL: "https://evil.example.com", ClientID: "c"}); !errors.Is(err, config.ErrBadRequest) {
		t.Fatalf("字段变化时空 Secret 应返回 ErrBadRequest: %v", err)
	}
	if got := readOidcParamsRaw(t, st, "generic"); got != before {
		t.Fatalf("拒绝后 raw JSON 不应变化: before=%s after=%s", before, got)
	}
	// 字段一致时空 Secret 保留原密文。
	if err := svc.SaveParams(ctx, "generic", Params{BaseURL: "https://idp.example.com", ClientID: "c"}); err != nil {
		t.Fatalf("字段一致时空 Secret 应成功: %v", err)
	}
	var saved Params
	if err := json.Unmarshal([]byte(readOidcParamsRaw(t, st, "generic")), &saved); err != nil {
		t.Fatalf("解析保存结果失败: %v", err)
	}
	if saved.ClientSecret == "" || saved.ClientSecret == "secret" {
		t.Fatalf("空 Secret 保存应保留原密文: %+v", saved)
	}
	loaded, err := svc.LoadParams(ctx, "generic")
	if err != nil || loaded.ClientSecret != "secret" {
		t.Fatalf("保留后应可解密出原明文: %+v %v", loaded, err)
	}

	// JSON 损坏：空 Secret 拒绝且 raw 不变；显式新 Secret + 必要字段可修复。
	setRawOidcParams(t, svc, "generic", `{`)
	damagedRaw := readOidcParamsRaw(t, st, "generic")
	if err := svc.SaveParams(ctx, "generic", Params{BaseURL: "https://idp.example.com", ClientID: "c"}); !errors.Is(err, config.ErrBadRequest) {
		t.Fatalf("JSON 损坏时空 Secret 应返回 ErrBadRequest: %v", err)
	}
	if got := readOidcParamsRaw(t, st, "generic"); got != damagedRaw {
		t.Fatalf("JSON 损坏拒绝后 raw 不应变化: %s", got)
	}
	if err := svc.SaveParams(ctx, "generic", Params{BaseURL: "https://idp.example.com", ClientID: "c", ClientSecret: "repaired-secret"}); err != nil {
		t.Fatalf("显式新 Secret 应可修复损坏 JSON: %v", err)
	}
	stAfter, err := svc.DescribeParams(ctx, "generic")
	if err != nil || !stAfter.Present || !stAfter.SecretUsable || stAfter.JSONDamaged || stAfter.SecretDamaged {
		t.Fatalf("修复后应为可用状态: %+v %v", stAfter, err)
	}

	// 字面 ***：同样拒绝空 Secret，显式新 Secret 可恢复。
	setRawOidcParams(t, svc, "generic", `{"base_url":"https://idp.example.com","client_id":"c","client_secret":"***"}`)
	if err := svc.SaveParams(ctx, "generic", Params{BaseURL: "https://idp.example.com", ClientID: "c"}); !errors.Is(err, config.ErrBadRequest) {
		t.Fatalf("字面 *** 空 Secret 应返回 ErrBadRequest: %v", err)
	}
	if err := svc.SaveParams(ctx, "generic", Params{BaseURL: "https://idp.example.com", ClientID: "c", ClientSecret: "fresh-secret"}); err != nil {
		t.Fatalf("显式新 Secret 应可修复字面 ***: %v", err)
	}
	stAfter, err = svc.DescribeParams(ctx, "generic")
	if err != nil || !stAfter.SecretUsable || stAfter.SecretDamaged {
		t.Fatalf("字面 *** 修复后应为可用状态: %+v %v", stAfter, err)
	}
}

// TestR3103StartFlowPinsProviderAndConfig StartFlow 必须在 state 中固定 provider 与配置指纹。
func TestR3103StartFlowPinsProviderAndConfig(t *testing.T) {
	st, svc, _ := newTestOidcService(t)
	if _, _, err := svc.StartFlow(ctx, "login", 0); err != nil {
		t.Fatalf("StartFlow 失败: %v", err)
	}
	raw, err := svc.cfg.Get(ctx, "oidc_params_mock")
	if err != nil {
		t.Fatalf("读取 mock 参数失败: %v", err)
	}
	var providerType, configHash string
	if err := st.DB().QueryRow(`SELECT provider_type, config_hash FROM oidc_states ORDER BY created_at DESC LIMIT 1`).Scan(&providerType, &configHash); err != nil {
		t.Fatalf("查询 state 固定标识失败: %v", err)
	}
	if providerType != "mock" || configHash != providerConfigHash("mock", raw) {
		t.Fatalf("state 固定标识异常: provider=%q hash=%q want=%q", providerType, configHash, providerConfigHash("mock", raw))
	}
}

// TestR3103ConsumeStateRejectsLegacyState 旧 state 行保留，但回调消费必须拒绝。
func TestR3103ConsumeStateRejectsLegacyState(t *testing.T) {
	st, svc, _ := newTestOidcService(t)
	if _, err := st.DB().Exec(
		`INSERT INTO oidc_states (state, code_verifier, nonce, intent) VALUES ('legacy-state','v','n','login')`); err != nil {
		t.Fatalf("写入旧 state 失败: %v", err)
	}
	if _, err := svc.ConsumeState(ctx, "legacy-state"); err == nil {
		t.Fatal("未固定 provider/config 的旧 state 必须拒绝")
	}
	var count int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM oidc_states WHERE state = 'legacy-state'`).Scan(&count); err != nil {
		t.Fatalf("查询旧 state 失败: %v", err)
	}
	if count != 1 {
		t.Fatalf("旧 state 行应按约定保留，实际 count=%d", count)
	}
}

// TestR3103ExchangeRejectsProviderOrConfigChange 授权发起后 provider 或参数变化，Exchange 必须直接拒绝。
func TestR3103ExchangeRejectsProviderOrConfigChange(t *testing.T) {
	t.Run("切换提供商", func(t *testing.T) {
		_, svc, _ := newTestOidcService(t)
		_, state, err := svc.StartFlow(ctx, "login", 0)
		if err != nil {
			t.Fatalf("StartFlow 失败: %v", err)
		}
		rec, err := svc.ConsumeState(ctx, state)
		if err != nil {
			t.Fatalf("ConsumeState 失败: %v", err)
		}
		if err := svc.SaveParams(ctx, "generic", Params{BaseURL: "https://idp.example.com", ClientID: "g", ClientSecret: "g-secret"}); err != nil {
			t.Fatalf("保存 generic 失败: %v", err)
		}
		if err := svc.cfg.Set(ctx, KeyProviderType, "generic"); err != nil {
			t.Fatalf("切换 provider 失败: %v", err)
		}
		if _, err := svc.Exchange(ctx, rec, "old-code"); err == nil || !strings.Contains(err.Error(), "提供商已变更") {
			t.Fatalf("切换提供商后旧 state 应拒绝: %v", err)
		}
	})

	t.Run("同提供商参数变化", func(t *testing.T) {
		_, svc, _ := newTestOidcService(t)
		_, state, err := svc.StartFlow(ctx, "login", 0)
		if err != nil {
			t.Fatalf("StartFlow 失败: %v", err)
		}
		rec, err := svc.ConsumeState(ctx, state)
		if err != nil {
			t.Fatalf("ConsumeState 失败: %v", err)
		}
		if err := svc.SaveParams(ctx, "mock", Params{BaseURL: "https://changed.example.com", ClientID: "changed", ClientSecret: "new-secret"}); err != nil {
			t.Fatalf("修改 mock 参数失败: %v", err)
		}
		if _, err := svc.Exchange(ctx, rec, "code"); err == nil || !strings.Contains(err.Error(), "配置已变更") {
			t.Fatalf("同提供商参数变化后旧 state 应拒绝: %v", err)
		}
	})
}

// TestR3103ExchangeKeepsSwitchBackBoundary 切走再切回且原始参数完全未变时，旧 state 仍属于同一发起边界。
func TestR3103ExchangeKeepsSwitchBackBoundary(t *testing.T) {
	_, svc, _ := newTestOidcService(t)
	originalRaw, err := svc.cfg.Get(ctx, "oidc_params_mock")
	if err != nil {
		t.Fatalf("读取 mock 原始参数失败: %v", err)
	}
	_, state, err := svc.StartFlow(ctx, "login", 0)
	if err != nil {
		t.Fatalf("StartFlow 失败: %v", err)
	}
	rec, err := svc.ConsumeState(ctx, state)
	if err != nil {
		t.Fatalf("ConsumeState 失败: %v", err)
	}
	// 切到其他提供商。
	if err := svc.SaveParams(ctx, "generic", Params{BaseURL: "https://idp.example.com", ClientID: "g", ClientSecret: "g-secret"}); err != nil {
		t.Fatalf("保存 generic 失败: %v", err)
	}
	if err := svc.cfg.Set(ctx, KeyProviderType, "generic"); err != nil {
		t.Fatalf("切换 provider 失败: %v", err)
	}
	// 原样切回 mock：直接恢复原始 raw JSON 与 provider。
	if err := svc.cfg.Set(ctx, "oidc_params_mock", originalRaw); err != nil {
		t.Fatalf("恢复 mock 参数失败: %v", err)
	}
	if err := svc.cfg.Set(ctx, KeyProviderType, "mock"); err != nil {
		t.Fatalf("切回 mock 失败: %v", err)
	}
	code, err := svc.MockCode("switchback@example.com", "switchback", true, nil, nil)
	if err != nil {
		t.Fatalf("生成 mock code 失败: %v", err)
	}
	id, err := svc.Exchange(ctx, rec, code)
	if err != nil {
		t.Fatalf("切回且参数未变时旧 state 应可继续: %v", err)
	}
	if id.Subject != "switchback@example.com" {
		t.Fatalf("身份还原异常: %+v", id)
	}
}

// TestR3103RealProviderPinnedBoundary 隔离 TLS 端点验证：切换/改参后旧授权不会到达新地址或收到新 Secret。
func TestR3103RealProviderPinnedBoundary(t *testing.T) {
	var mu sync.Mutex
	hits := map[string]int{}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits[r.URL.Path]++
		mu.Unlock()
		if strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration") {
			base := strings.TrimSuffix(r.URL.Path, "/.well-known/openid-configuration")
			_ = json.NewEncoder(w).Encode(Discovery{
				AuthorizationEndpoint: srvURL(r) + base + "/authorize",
				TokenEndpoint:         srvURL(r) + base + "/token",
			})
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id_token":"unused"}`))
	}))
	defer srv.Close()

	_, svc, _ := newTestOidcService(t)
	svc.httpCli.Transport = srv.Client().Transport
	svc.ClearDiscCache()
	if err := svc.SaveParams(ctx, "keycloak", Params{BaseURL: srv.URL + "/a", ClientID: "client-a", ClientSecret: "secret-a"}); err != nil {
		t.Fatalf("保存 provider A 失败: %v", err)
	}
	if err := svc.cfg.Set(ctx, KeyProviderType, "keycloak"); err != nil {
		t.Fatalf("设置 provider A 失败: %v", err)
	}
	_, state, err := svc.StartFlow(ctx, "login", 0)
	if err != nil {
		t.Fatalf("provider A StartFlow 失败: %v", err)
	}
	rec, err := svc.ConsumeState(ctx, state)
	if err != nil {
		t.Fatalf("ConsumeState 失败: %v", err)
	}
	// 切换到 provider B。
	if err := svc.SaveParams(ctx, "generic", Params{BaseURL: srv.URL + "/b", ClientID: "client-b", ClientSecret: "secret-b"}); err != nil {
		t.Fatalf("保存 provider B 失败: %v", err)
	}
	if err := svc.cfg.Set(ctx, KeyProviderType, "generic"); err != nil {
		t.Fatalf("切换 provider B 失败: %v", err)
	}
	if _, err := svc.Exchange(ctx, rec, "old-code"); err == nil || !strings.Contains(err.Error(), "提供商已变更") {
		t.Fatalf("provider 切换后旧 state 应在网络前拒绝: %v", err)
	}
	mu.Lock()
	for path := range hits {
		if strings.HasPrefix(path, "/b") {
			t.Fatalf("旧授权不应触达新 provider 路径 %s: %+v", path, hits)
		}
	}
	mu.Unlock()

	// 同 provider 改地址/Secret：旧 state 同样不得触达新地址。
	if err := svc.SaveParams(ctx, "keycloak", Params{BaseURL: srv.URL + "/a", ClientID: "client-a", ClientSecret: "secret-a"}); err != nil {
		t.Fatalf("恢复 provider A 失败: %v", err)
	}
	if err := svc.cfg.Set(ctx, KeyProviderType, "keycloak"); err != nil {
		t.Fatalf("恢复 provider A 失败: %v", err)
	}
	_, state2, err := svc.StartFlow(ctx, "login", 0)
	if err != nil {
		t.Fatalf("provider A 第二次 StartFlow 失败: %v", err)
	}
	rec2, err := svc.ConsumeState(ctx, state2)
	if err != nil {
		t.Fatalf("第二次 ConsumeState 失败: %v", err)
	}
	if err := svc.SaveParams(ctx, "keycloak", Params{BaseURL: srv.URL + "/a2", ClientID: "client-a", ClientSecret: "new-secret"}); err != nil {
		t.Fatalf("修改 provider A 失败: %v", err)
	}
	if _, err := svc.Exchange(ctx, rec2, "old-code-2"); err == nil || !strings.Contains(err.Error(), "配置已变更") {
		t.Fatalf("同 provider 改参后旧 state 应在网络前拒绝: %v", err)
	}
	mu.Lock()
	if hits["/a2/token"] != 0 {
		t.Fatalf("旧授权不应触达新地址 /a2/token: %+v", hits)
	}
	mu.Unlock()
}

// srvURL 从测试请求还原 TLS server 的绝对地址（httptest 请求自身可用于发现文档）。
func srvURL(r *http.Request) string {
	return "https://" + r.Host
}

// TestR3103TestConnectionUsesOnlyTargetSavedSecret 保存前后测试连接不得回退源提供商 Secret，也不得把任何 Secret 发往非目标地址。
func TestR3103TestConnectionUsesOnlyTargetSavedSecret(t *testing.T) {
	var mu sync.Mutex
	bodies := map[string][]string{}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		mu.Lock()
		bodies[r.URL.Path] = append(bodies[r.URL.Path], string(body))
		mu.Unlock()
		if strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration") {
			base := strings.TrimSuffix(r.URL.Path, "/.well-known/openid-configuration")
			_ = json.NewEncoder(w).Encode(Discovery{
				AuthorizationEndpoint: srvURL(r) + base + "/authorize",
				TokenEndpoint:         srvURL(r) + base + "/token",
			})
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token":"test-token"}`))
	}))
	defer srv.Close()

	_, svc, _ := newTestOidcService(t)
	svc.httpCli.Transport = srv.Client().Transport
	svc.ClearDiscCache()
	if err := svc.SaveParams(ctx, "keycloak", Params{BaseURL: srv.URL + "/source", ClientID: "client-source", ClientSecret: "source-secret"}); err != nil {
		t.Fatalf("保存 source 失败: %v", err)
	}
	if err := svc.SaveParams(ctx, "generic", Params{BaseURL: srv.URL + "/target", ClientID: "client-target", ClientSecret: "target-secret"}); err != nil {
		t.Fatalf("保存 target 失败: %v", err)
	}
	if err := svc.cfg.Set(ctx, KeyProviderType, "generic"); err != nil {
		t.Fatalf("设置当前 provider 失败: %v", err)
	}

	countPath := func(path string) int {
		mu.Lock()
		defer mu.Unlock()
		return len(bodies[path])
	}
	contains := func(path, needle string) bool {
		mu.Lock()
		defer mu.Unlock()
		for _, body := range bodies[path] {
			if strings.Contains(body, needle) {
				return true
			}
		}
		return false
	}
	lastBody := func(path string) string {
		mu.Lock()
		defer mu.Unlock()
		if len(bodies[path]) == 0 {
			return ""
		}
		return bodies[path][len(bodies[path])-1]
	}

	// 保存前：目标字段一致 + 空 Secret → 只回退目标已存 Secret。
	res, err := svc.TestConnectionWithSavedSecret(ctx, "generic", Params{BaseURL: srv.URL + "/target", ClientID: "client-target"})
	if err != nil || res == nil || !res.OK {
		t.Fatalf("目标字段一致时测试连接应回退目标 Secret: %+v %v", res, err)
	}
	if !contains("/target/token", "client_secret=target-secret") || contains("/target/token", "client_secret=source-secret") {
		t.Fatalf("目标 token 端点应只收到目标 Secret: %+v", bodies)
	}
	if contains("/source/token", "client_secret=target-secret") {
		t.Fatalf("目标 Secret 不应发往源提供商地址: %+v", bodies)
	}

	// 目标字段变化 + 空 Secret → 不得回退任何旧 Secret，也不得向变化后地址发凭据。
	res, err = svc.TestConnectionWithSavedSecret(ctx, "generic", Params{BaseURL: srv.URL + "/target-evil", ClientID: "client-target"})
	if err != nil || res == nil || !res.OK {
		t.Fatalf("字段变化时测试连接应完成 discovery 但跳过凭据校验: %+v %v", res, err)
	}
	if !hasWarning(res, "未执行凭据校验") {
		t.Fatalf("字段变化 + 空 Secret 应提示未执行凭据校验: %+v", res)
	}
	if countPath("/target-evil/token") != 0 {
		t.Fatalf("字段变化后不得向新地址发送 Secret: %+v", bodies)
	}

	// 保存后：显式新 Secret 替换，空 Secret 测试回退到新目标 Secret。
	if err := svc.SaveParams(ctx, "generic", Params{BaseURL: srv.URL + "/target", ClientID: "client-target", ClientSecret: "new-target-secret"}); err != nil {
		t.Fatalf("替换目标 Secret 失败: %v", err)
	}
	res, err = svc.TestConnectionWithSavedSecret(ctx, "generic", Params{BaseURL: srv.URL + "/target", ClientID: "client-target"})
	if err != nil || res == nil || !res.OK {
		t.Fatalf("替换后测试连接应回退新目标 Secret: %+v %v", res, err)
	}
	lastTargetBody := lastBody("/target/token")
	if !strings.Contains(lastTargetBody, "client_secret=new-target-secret") || strings.Contains(lastTargetBody, "client_secret=target-secret") || strings.Contains(lastTargetBody, "client_secret=source-secret") {
		t.Fatalf("保存后目标 token 端点应使用新 Secret 且不混入旧/源 Secret: last=%q all=%+v", lastTargetBody, bodies)
	}

	// 源提供商测试：回退源 Secret，目标 Secret 不进入源地址。
	res, err = svc.TestConnectionWithSavedSecret(ctx, "keycloak", Params{BaseURL: srv.URL + "/source", ClientID: "client-source"})
	if err != nil || res == nil || !res.OK {
		t.Fatalf("源提供商测试连接应回退源 Secret: %+v %v", res, err)
	}
	if !contains("/source/token", "client_secret=source-secret") {
		t.Fatalf("源 token 端点应收到源 Secret: %+v", bodies)
	}
	if contains("/source/token", "client_secret=target-secret") || contains("/source/token", "client_secret=new-target-secret") {
		t.Fatalf("目标 Secret 不应发往源提供商地址: %+v", bodies)
	}
}
