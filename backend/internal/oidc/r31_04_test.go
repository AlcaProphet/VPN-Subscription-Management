package oidc

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"vpn-sub/internal/config"
)

// rawParams 构造测试用参数 JSON；clientSecret 原样写入（可为密文、字面占位符或损坏值）。
func rawParams(baseURL, realm, clientID, clientSecret string) string {
	b, _ := json.Marshal(Params{BaseURL: baseURL, Realm: realm, ClientID: clientID, ClientSecret: clientSecret})
	return string(b)
}

// encryptForTest 用当前测试库签名密钥加密明文，供“正常密文/解密后 ***/空明文”用例使用。
func encryptForTest(t *testing.T, svc *Service, plain string) string {
	t.Helper()
	key, err := svc.cfg.GetSigningKey(ctx)
	if err != nil {
		t.Fatalf("读取测试签名密钥失败: %v", err)
	}
	enc, err := config.Encrypt([]byte(plain), key)
	if err != nil {
		t.Fatalf("加密测试值失败: %v", err)
	}
	return enc
}

// TestR3104DescribeParamsStateMatrix 六态状态机与字段保留/不猜测规则。
func TestR3104DescribeParamsStateMatrix(t *testing.T) {
	st, svc, _ := newTestOidcService(t)

	assertState := func(name, raw string, want config.OidcParamsStateCode, wantFields bool) {
		t.Helper()
		if err := svc.cfg.Set(ctx, "oidc_params_generic", raw); err != nil {
			t.Fatalf("%s 写入 raw 失败: %v", name, err)
		}
		got, err := svc.DescribeParams(ctx, "generic")
		if err != nil {
			t.Fatalf("%s 不应返回错误: %v", name, err)
		}
		if config.EffectiveOidcParamsState(got) != want {
			t.Fatalf("%s 状态错误: got=%s want=%s state=%+v", name, got.State, want, got)
		}
		if wantFields {
			if got.BaseURL != "https://idp.example.com" || got.ClientID != "c" {
				t.Fatalf("%s 应保留可解析非 Secret 字段: %+v", name, got)
			}
		} else if got.BaseURL != "" || got.ClientID != "" {
			t.Fatalf("%s 不应猜测非 Secret 字段: %+v", name, got)
		}
	}

	assertState("空 JSON", "", config.OidcParamsNotConfigured, false)
	assertState("空对象", "{}", config.OidcParamsMissingSecret, false)
	assertState("坏 JSON", `{`, config.OidcParamsJSONDamaged, false)
	assertState("空 Secret", rawParams("https://idp.example.com", "", "c", ""), config.OidcParamsMissingSecret, true)
	assertState("字面 ***", rawParams("https://idp.example.com", "", "c", config.MaskedSecret), config.OidcParamsSecretDamaged, true)
	assertState("非法密文", rawParams("https://idp.example.com", "", "c", "not-a-cipher"), config.OidcParamsSecretDamaged, true)
	// 合法 base64/GCM 结构但由其他签名密钥加密：GCM 校验失败，属“无法解密密文”。
	wrongKeyCipher, err := config.Encrypt([]byte("wrong-key-secret"), []byte("wrong-signing-key-0123456789abcdef"))
	if err != nil {
		t.Fatalf("构造错误密钥密文失败: %v", err)
	}
	assertState("错误密钥密文", rawParams("https://idp.example.com", "", "c", wrongKeyCipher), config.OidcParamsSecretDamaged, true)

	// 加密后解密为 ***：仍标 Secret 损坏，保留字段。
	writeMaskedOidcParams(t, st, svc, "generic", "https://idp.example.com", "", "c")
	got, err := svc.DescribeParams(ctx, "generic")
	if err != nil || config.EffectiveOidcParamsState(got) != config.OidcParamsSecretDamaged ||
		got.BaseURL != "https://idp.example.com" || got.ClientID != "c" {
		t.Fatalf("解密后 *** 应标 Secret 损坏并保留字段: %+v %v", got, err)
	}

	// 加密后的空明文按“缺可用 Secret”处理，不标损坏。
	encEmpty := encryptForTest(t, svc, "")
	assertState("加密空值", rawParams("https://idp.example.com", "", "c", encEmpty), config.OidcParamsMissingSecret, true)

	// 正常密文：usable。
	encNormal := encryptForTest(t, svc, "normal-secret")
	assertState("正常密文", rawParams("https://idp.example.com", "", "c", encNormal), config.OidcParamsUsable, true)

	// signing_key 缺失：独立 signing_key_fault，JSON 可解析字段保留，且错误可类型化识别。
	if err := svc.cfg.Set(ctx, config.KeySigningKey, ""); err != nil {
		t.Fatalf("清空签名密钥失败: %v", err)
	}
	if err := svc.cfg.Set(ctx, "oidc_params_generic", rawParams("https://idp.example.com", "", "c", "not-a-cipher")); err != nil {
		t.Fatalf("写入 raw 失败: %v", err)
	}
	got, err = svc.DescribeParams(ctx, "generic")
	if !errors.Is(err, config.ErrSigningKeyUnavailable) || config.EffectiveOidcParamsState(got) != config.OidcParamsSigningKeyFault {
		t.Fatalf("签名密钥缺失应为独立 signing_key_fault: %+v %v", got, err)
	}
	if got.BaseURL != "https://idp.example.com" || got.ClientID != "c" {
		t.Fatalf("签名密钥故障时 JSON 可解析字段应保留: %+v", got)
	}

	// signing_key 行值为 NULL 的读取失败路径同样独立归类，不伪装成 Secret 损坏。
	if err := svc.cfg.Set(ctx, config.KeySigningKey, "temporary-key"); err != nil {
		t.Fatalf("写入临时签名密钥失败: %v", err)
	}
	if _, err := svc.store.DB().ExecContext(ctx, `UPDATE system_config SET value = NULL WHERE key = ?`, config.KeySigningKey); err != nil {
		t.Fatalf("写入 NULL 签名密钥失败: %v", err)
	}
	got, err = svc.DescribeParams(ctx, "generic")
	if !errors.Is(err, config.ErrSigningKeyUnavailable) || config.EffectiveOidcParamsState(got) != config.OidcParamsSigningKeyFault {
		t.Fatalf("签名密钥读取失败应为独立 signing_key_fault: %+v %v", got, err)
	}

	// 空 JSON 优先按未配置处理，不因全站签名密钥故障伪装成提供商损坏。
	if err := svc.cfg.Set(ctx, "oidc_params_generic", ""); err != nil {
		t.Fatalf("写入空 JSON 失败: %v", err)
	}
	got, err = svc.DescribeParams(ctx, "generic")
	if err != nil || config.EffectiveOidcParamsState(got) != config.OidcParamsNotConfigured {
		t.Fatalf("空 JSON 应为 not_configured: %+v %v", got, err)
	}
}

// TestR3104ErrorMessagesDoNotEchoCredentialMaterial 错误与测试连接回执不得包含密文/明文/密钥值。
func TestR3104ErrorMessagesDoNotEchoCredentialMaterial(t *testing.T) {
	_, svc, _ := newTestOidcService(t)
	const cipherMarker = "CIPHER_MARKER_9f8a7c6d5e4f"
	const plainMarker = "PLAIN_MARKER_1a2b3c4d5e6f"

	setRawOidcParams(t, svc, "generic", rawParams("https://idp.example.com", "", "c", cipherMarker))
	if err := svc.SaveParams(ctx, "generic", Params{BaseURL: "https://idp.example.com", ClientID: "c"}); err == nil || strings.Contains(err.Error(), cipherMarker) {
		t.Fatalf("空 Secret 拒绝错误不得包含原始密文: %v", err)
	}
	res, _ := svc.TestConnectionWithSavedSecret(ctx, "generic", Params{BaseURL: "https://idp.example.com", ClientID: "c"})
	if res == nil || strings.Contains(res.Message, cipherMarker) {
		t.Fatalf("测试连接专门失败不得包含原始密文: %+v", res)
	}

	if err := svc.cfg.Set(ctx, config.KeySigningKey, ""); err != nil {
		t.Fatalf("清空签名密钥失败: %v", err)
	}
	setRawOidcParams(t, svc, "generic", rawParams("https://idp.example.com", "", "c", cipherMarker))
	err := svc.SaveParams(ctx, "generic", Params{BaseURL: "https://idp.example.com", ClientID: "c", ClientSecret: plainMarker})
	if !errors.Is(err, config.ErrSigningKeyUnavailable) || strings.Contains(err.Error(), plainMarker) || strings.Contains(err.Error(), cipherMarker) {
		t.Fatalf("签名密钥故障错误不得包含明文/密文: %v", err)
	}
}

// TestR3104SaveParamsAtomicAndRepair 损坏状态空 Secret 拒绝且不写库；显式新 Secret 可恢复；密钥故障不生成新密钥。
func TestR3104SaveParamsAtomicAndRepair(t *testing.T) {
	_, svc, _ := newTestOidcService(t)
	fields := Params{BaseURL: "https://idp.example.com", Realm: "", ClientID: "c"}

	assertEmptySecretRejected := func(name, raw string, wantErr error) {
		t.Helper()
		setRawOidcParams(t, svc, "generic", raw)
		before := readOidcParamsRaw(t, svc.store, "generic")
		err := svc.SaveParams(ctx, "generic", fields)
		if err == nil {
			t.Fatalf("%s 空 Secret 应被拒绝", name)
		}
		if wantErr != nil && !errors.Is(err, wantErr) {
			t.Fatalf("%s 错误类型不符: %v", name, err)
		}
		if got := readOidcParamsRaw(t, svc.store, "generic"); got != before {
			t.Fatalf("%s 拒绝后 raw 不应变化: before=%s after=%s", name, before, got)
		}
	}

	assertEmptySecretRejected("坏 JSON", `{`, config.ErrBadRequest)
	assertEmptySecretRejected("字面 ***", rawParams("https://idp.example.com", "", "c", config.MaskedSecret), config.ErrBadRequest)
	assertEmptySecretRejected("非法密文", rawParams("https://idp.example.com", "", "c", "not-a-cipher"), config.ErrBadRequest)
	assertEmptySecretRejected("加密空值", rawParams("https://idp.example.com", "", "c", encryptForTest(t, svc, "")), config.ErrBadRequest)

	// 显式新 Secret 恢复损坏值；每次恢复后必须回到 usable。
	wrongKeyCipher, err := config.Encrypt([]byte("wrong-key-secret"), []byte("wrong-signing-key-0123456789abcdef"))
	if err != nil {
		t.Fatalf("构造错误密钥密文失败: %v", err)
	}
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{"字面 ***", rawParams("https://idp.example.com", "", "c", config.MaskedSecret)},
		{"非法密文", rawParams("https://idp.example.com", "", "c", "not-a-cipher")},
		{"错误密钥密文", rawParams("https://idp.example.com", "", "c", wrongKeyCipher)},
		{"加密后 ***", rawParams("https://idp.example.com", "", "c", encryptForTest(t, svc, config.MaskedSecret))},
		{"坏 JSON", `{`},
	} {
		t.Run("恢复/"+tc.name, func(t *testing.T) {
			setRawOidcParams(t, svc, "generic", tc.raw)
			if err := svc.SaveParams(ctx, "generic", Params{BaseURL: "https://idp.example.com", ClientID: "c", ClientSecret: "repaired-secret"}); err != nil {
				t.Fatalf("显式新 Secret 修复失败: %v", err)
			}
			got, err := svc.DescribeParams(ctx, "generic")
			if err != nil || config.EffectiveOidcParamsState(got) != config.OidcParamsUsable {
				t.Fatalf("修复后应为 usable: %+v %v", got, err)
			}
			loaded, err := svc.LoadParams(ctx, "generic")
			if err != nil || loaded.ClientSecret != "repaired-secret" {
				t.Fatalf("修复后应解密出新 Secret: %+v %v", loaded, err)
			}
		})
	}

	// JSON 整体损坏且未重填必要非 Secret 字段：显式新 Secret 也必须拒绝且不写库。
	setRawOidcParams(t, svc, "generic", `{`)
	beforeBroken := readOidcParamsRaw(t, svc.store, "generic")
	if err := svc.SaveParams(ctx, "generic", Params{BaseURL: "", ClientID: "", ClientSecret: "new-secret"}); !errors.Is(err, config.ErrBadRequest) {
		t.Fatalf("坏 JSON 缺少字段时显式保存应返回 ErrBadRequest: %v", err)
	}
	if got := readOidcParamsRaw(t, svc.store, "generic"); got != beforeBroken {
		t.Fatalf("坏 JSON 缺少字段拒绝后 raw 不应变化: %s", got)
	}

	// 签名密钥故障：即使提交新 Secret 也不得写入、不得生成新密钥。
	if err := svc.cfg.Set(ctx, config.KeySigningKey, ""); err != nil {
		t.Fatalf("清空签名密钥失败: %v", err)
	}
	setRawOidcParams(t, svc, "generic", rawParams("https://idp.example.com", "", "c", "not-a-cipher"))
	beforeKeyFault := readOidcParamsRaw(t, svc.store, "generic")
	if err := svc.SaveParams(ctx, "generic", Params{BaseURL: "https://idp.example.com", ClientID: "c", ClientSecret: "new-secret"}); !errors.Is(err, config.ErrSigningKeyUnavailable) {
		t.Fatalf("签名密钥故障应返回 ErrSigningKeyUnavailable: %v", err)
	}
	if got := readOidcParamsRaw(t, svc.store, "generic"); got != beforeKeyFault {
		t.Fatalf("签名密钥故障拒绝后 raw 不应变化: %s", got)
	}
	if key, _ := svc.cfg.Get(ctx, config.KeySigningKey); key != "" {
		t.Fatalf("签名密钥故障路径不得自动生成新密钥: %q", key)
	}
}

// TestR3104StartFlowRejectsDamageBeforeNetwork 真实登录入口对损坏/密钥故障在 discovery 前拒绝。
func TestR3104StartFlowRejectsDamageBeforeNetwork(t *testing.T) {
	var hits int32
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		http.NotFound(w, r)
	}))
	defer ts.Close()

	_, svc, _ := newTestOidcService(t)
	svc.httpCli.Transport = ts.Client().Transport
	svc.ClearDiscCache()
	if err := svc.cfg.Set(ctx, KeyProviderType, "generic"); err != nil {
		t.Fatalf("设置 provider 失败: %v", err)
	}

	cases := []struct {
		name string
		raw  string
	}{
		{"坏 JSON", `{`},
		{"字面 ***", rawParams("https://idp.example.com", "", "c", config.MaskedSecret)},
		{"非法密文", rawParams("https://idp.example.com", "", "c", "not-a-cipher")},
		{"加密后 ***", rawParams("https://idp.example.com", "", "c", encryptForTest(t, svc, config.MaskedSecret))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setRawOidcParams(t, svc, "generic", tc.raw)
			before := atomic.LoadInt32(&hits)
			if _, _, err := svc.StartFlow(ctx, "login", 0); err == nil {
				t.Fatalf("%s StartFlow 应被拒绝", tc.name)
			}
			if got := atomic.LoadInt32(&hits); got != before {
				t.Fatalf("%s StartFlow 不应发出网络请求: before=%d after=%d", tc.name, before, got)
			}
		})
	}

	// 签名密钥故障：即使参数 JSON 可解析且密文非空，也必须在网络前拒绝。
	if err := svc.cfg.Set(ctx, config.KeySigningKey, ""); err != nil {
		t.Fatalf("清空签名密钥失败: %v", err)
	}
	setRawOidcParams(t, svc, "generic", rawParams("https://idp.example.com", "", "c", "any-cipher"))
	before := atomic.LoadInt32(&hits)
	if _, _, err := svc.StartFlow(ctx, "login", 0); err == nil || !strings.Contains(err.Error(), "签名密钥") {
		t.Fatalf("签名密钥故障 StartFlow 应拒绝且提示密钥: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != before {
		t.Fatalf("签名密钥故障 StartFlow 不应发出网络请求: before=%d after=%d", before, got)
	}
	// Exchange 同样必须在 discovery/token 前拒绝。
	rec := &StateRecord{ProviderType: "generic", ConfigHash: providerConfigHash("generic", rawParams("https://idp.example.com", "", "c", "any-cipher")), CodeVerifier: "verifier", Nonce: "nonce", RedirectURI: svc.CallbackURL(ctx)}
	before = atomic.LoadInt32(&hits)
	if _, err := svc.Exchange(ctx, rec, "code"); err == nil || !strings.Contains(err.Error(), "签名密钥") {
		t.Fatalf("签名密钥故障 Exchange 应拒绝且提示密钥: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != before {
		t.Fatalf("签名密钥故障 Exchange 不应发出网络请求: before=%d after=%d", before, got)
	}
}

// TestR3104TestConnectionDamageAndKeyFault 管理端测试连接：已存损坏专门失败；密钥故障也阻断且 0 网络请求。
func TestR3104TestConnectionDamageAndKeyFault(t *testing.T) {
	var hits int32
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		http.NotFound(w, r)
	}))
	defer ts.Close()

	_, svc, _ := newTestOidcService(t)
	svc.httpCli.Transport = ts.Client().Transport
	svc.ClearDiscCache()
	baseURL := ts.URL
	wrongKeyCipher, err := config.Encrypt([]byte("wrong-key-secret"), []byte("wrong-signing-key-0123456789abcdef"))
	if err != nil {
		t.Fatalf("构造错误密钥密文失败: %v", err)
	}

	cases := []struct {
		name string
		raw  string
	}{
		{"坏 JSON", `{`},
		{"字面 ***", rawParams(baseURL, "", "c", config.MaskedSecret)},
		{"非法密文", rawParams(baseURL, "", "c", "not-a-cipher")},
		{"错误密钥密文", rawParams(baseURL, "", "c", wrongKeyCipher)},
		{"加密后 ***", rawParams(baseURL, "", "c", encryptForTest(t, svc, config.MaskedSecret))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setRawOidcParams(t, svc, "generic", tc.raw)
			before := atomic.LoadInt32(&hits)
			res, err := svc.TestConnectionWithSavedSecret(ctx, "generic", Params{BaseURL: baseURL, ClientID: "c"})
			if err != nil || res == nil || res.OK || res.Message != config.OidcTestStoredDamagedMessage {
				t.Fatalf("%s 应返回专门损坏失败且不误报: res=%+v err=%v", tc.name, res, err)
			}
			if got := atomic.LoadInt32(&hits); got != before {
				t.Fatalf("%s 不应发起 discovery/凭据请求: before=%d after=%d", tc.name, before, got)
			}
		})
	}

	// 显式新 Secret 草稿绕过已存损坏检查，仍可发起实际测试连接。
	before := atomic.LoadInt32(&hits)
	res, err := svc.TestConnectionWithSavedSecret(ctx, "generic", Params{BaseURL: baseURL, ClientID: "c", ClientSecret: "draft-secret"})
	if err != nil || res == nil || res.Message == config.OidcTestStoredDamagedMessage {
		t.Fatalf("显式新 Secret 草稿不应被已存损坏阻断: res=%+v err=%v", res, err)
	}
	if got := atomic.LoadInt32(&hits); got <= before {
		t.Fatalf("显式新 Secret 草稿应实际发起测试连接: before=%d after=%d", before, got)
	}

	// 签名密钥故障：显式新 Secret 也阻断，避免出现“配置有效”但真实登录仍不可能成功的误报。
	if err := svc.cfg.Set(ctx, config.KeySigningKey, ""); err != nil {
		t.Fatalf("清空签名密钥失败: %v", err)
	}
	before = atomic.LoadInt32(&hits)
	res, err = svc.TestConnectionWithSavedSecret(ctx, "generic", Params{BaseURL: baseURL, ClientID: "c", ClientSecret: "new-secret"})
	if err != nil || res == nil || res.OK || res.Message != config.OidcTestSigningKeyFaultMessage {
		t.Fatalf("签名密钥故障应阻断管理端测试连接: res=%+v err=%v", res, err)
	}
	if got := atomic.LoadInt32(&hits); got != before {
		t.Fatalf("签名密钥故障测试连接不应发出网络请求: before=%d after=%d", before, got)
	}
	// Setup 显式参数路径（allowSavedSecret=false）不应用 K1；密钥故障不影响显式测试连接发起。
	before = atomic.LoadInt32(&hits)
	res, err = svc.TestConnection(ctx, "generic", Params{BaseURL: baseURL, ClientID: "c", ClientSecret: "new-secret"})
	if err != nil || res == nil || res.Message == config.OidcTestSigningKeyFaultMessage {
		t.Fatalf("Setup 显式参数测试连接不应被 K1 阻断: res=%+v err=%v", res, err)
	}
	if got := atomic.LoadInt32(&hits); got <= before {
		t.Fatalf("Setup 显式参数测试连接应实际发起网络请求: before=%d after=%d", before, got)
	}
}
