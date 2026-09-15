package oidc

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// r3102Recorder 记录隔离端点收到的路径与请求体，用于断言“目标端未收到 Secret”。
type r3102Recorder struct {
	mu     sync.Mutex
	hits   map[string]int
	bodies map[string][]string
}

func newR3102Recorder() *r3102Recorder {
	return &r3102Recorder{hits: map[string]int{}, bodies: map[string][]string{}}
}

func (r *r3102Recorder) record(path, body string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hits[path]++
	r.bodies[path] = append(r.bodies[path], body)
}

func (r *r3102Recorder) count(path string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.hits[path]
}

func (r *r3102Recorder) contains(path, needle string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, body := range r.bodies[path] {
		if strings.Contains(body, needle) {
			return true
		}
	}
	return false
}

// r3102ReadBody 读取测试端点请求体；测试 handler 运行在非 test goroutine，不能调用 t.Fatalf。
func r3102ReadBody(r *http.Request) string {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	return string(body)
}

func TestR3102HTTPInitialDiscoveryRejectedBeforeNetwork(t *testing.T) {
	var hits int32
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer httpSrv.Close()

	st, svc, _ := newTestOidcService(t)
	if err := svc.cfg.Set(ctx, KeyProviderType, "generic"); err != nil {
		t.Fatalf("设置真实提供商失败: %v", err)
	}
	raw, err := json.Marshal(Params{BaseURL: httpSrv.URL, ClientID: "client"})
	if err != nil {
		t.Fatalf("序列化测试参数失败: %v", err)
	}
	if _, err := st.DB().Exec(
		`INSERT INTO system_config(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		"oidc_params_generic", string(raw)); err != nil {
		t.Fatalf("写入 HTTP OIDC 参数失败: %v", err)
	}

	if _, err := svc.fetchDiscovery(ctx, &Params{BaseURL: httpSrv.URL, ClientID: "client"}); err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("HTTP 初始发现地址应被拒绝，实际: %v", err)
	}
	if _, _, err := svc.StartFlow(ctx, "login", 0); err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("HTTP 配置的 StartFlow 应被拒绝，实际: %v", err)
	}
	if _, err := svc.getJWKS(ctx, httpSrv.URL+"/jwks"); err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("HTTP JWKS 地址应被拒绝，实际: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 0 {
		t.Fatalf("HTTP 端点不应收到请求，实际命中 %d 次", got)
	}
}

func TestR3102HTTPDiscoveryEndpointsRejectedBeforeCache(t *testing.T) {
	_, svc, _ := newTestOidcService(t)
	if err := svc.cfg.Set(ctx, KeyProviderType, "generic"); err != nil {
		t.Fatalf("设置真实提供商失败: %v", err)
	}

	cases := []struct {
		name    string
		mutate  func(*Discovery)
		wantErr string
	}{
		{
			name: "authorization_endpoint 为 HTTP",
			mutate: func(d *Discovery) {
				d.AuthorizationEndpoint = "http://idp.example.com/authorize"
			},
			wantErr: "authorization_endpoint",
		},
		{
			name: "token_endpoint 为 HTTP",
			mutate: func(d *Discovery) {
				d.TokenEndpoint = "http://idp.example.com/token"
			},
			wantErr: "token_endpoint",
		},
		{
			name: "jwks_uri 为 HTTP",
			mutate: func(d *Discovery) {
				d.JWKSURI = "http://idp.example.com/jwks"
			},
			wantErr: "jwks_uri",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var hits int32
			var srv *httptest.Server
			srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&hits, 1)
				disc := Discovery{
					AuthorizationEndpoint: srv.URL + "/authorize",
					TokenEndpoint:         srv.URL + "/token",
					JWKSURI:               srv.URL + "/jwks",
				}
				tc.mutate(&disc)
				_ = json.NewEncoder(w).Encode(disc)
			}))
			defer srv.Close()

			svc.ClearDiscCache()
			svc.httpCli.Transport = srv.Client().Transport
			params := &Params{BaseURL: srv.URL, ClientID: "client"}
			if _, err := svc.fetchDiscovery(ctx, params); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("发现文档 %s 非法时第一次请求应失败且包含 %q，实际: %v", tc.name, tc.wantErr, err)
			}
			if got := atomic.LoadInt32(&hits); got != 1 {
				t.Fatalf("第一次请求应命中发现文档一次，实际 %d", got)
			}
			if len(svc.discCache) != 0 {
				t.Fatal("非法发现文档不得写入缓存")
			}
			if _, err := svc.fetchDiscovery(ctx, params); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("发现文档 %s 非法时第二次请求仍应失败，实际: %v", tc.name, err)
			}
			if got := atomic.LoadInt32(&hits); got != 2 {
				t.Fatalf("非法发现文档不得缓存，第二次应重新请求，实际命中 %d 次", got)
			}
		})
	}
}

func TestR3102ValidDiscoveryCacheHit(t *testing.T) {
	_, svc, _ := newTestOidcService(t)
	if err := svc.cfg.Set(ctx, KeyProviderType, "generic"); err != nil {
		t.Fatalf("设置真实提供商失败: %v", err)
	}
	var hits int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_ = json.NewEncoder(w).Encode(Discovery{
			AuthorizationEndpoint: "https://idp.example.com/authorize",
			TokenEndpoint:         "https://idp.example.com/token",
			JWKSURI:               "https://idp.example.com/jwks",
		})
	}))
	defer srv.Close()
	svc.ClearDiscCache()
	svc.httpCli.Transport = srv.Client().Transport
	params := &Params{BaseURL: srv.URL, ClientID: "client"}
	if _, err := svc.fetchDiscovery(ctx, params); err != nil {
		t.Fatalf("首次合法发现文档请求失败: %v", err)
	}
	if _, err := svc.fetchDiscovery(ctx, params); err != nil {
		t.Fatalf("缓存命中请求失败: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("合法发现文档应只请求一次，实际 %d 次", got)
	}
}

func TestR3102ActualUseGuardRejectsCachedHTTPEndpoints(t *testing.T) {
	_, svc, _ := newTestOidcService(t)
	if err := svc.cfg.Set(ctx, KeyProviderType, "generic"); err != nil {
		t.Fatalf("设置真实提供商失败: %v", err)
	}
	const base = "https://idp.example.com"
	if err := svc.SaveParams(ctx, "generic", Params{BaseURL: base, ClientID: "client", ClientSecret: "secret"}); err != nil {
		t.Fatalf("保存测试 OIDC 参数失败: %v", err)
	}
	wellKnown := base + "/.well-known/openid-configuration"

	// 缓存命中恶意授权端点：StartFlow 必须在交给浏览器前拒绝。
	svc.discCache[wellKnown] = &Discovery{
		AuthorizationEndpoint: "http://evil.example.com/authorize",
		TokenEndpoint:         base + "/token",
		JWKSURI:               base + "/jwks",
	}
	if _, _, err := svc.StartFlow(ctx, "login", 0); err == nil || !strings.Contains(err.Error(), "授权端点地址校验失败") {
		t.Fatalf("缓存中的 HTTP 授权端点应被实际使用点守卫拒绝，实际: %v", err)
	}

	// 缓存命中恶意 token 端点：Exchange 必须在 POST 前拒绝。
	svc.discCache[wellKnown] = &Discovery{
		AuthorizationEndpoint: base + "/authorize",
		TokenEndpoint:         "http://evil.example.com/token",
		JWKSURI:               base + "/jwks",
	}
	if _, err := svc.Exchange(ctx, pinnedStateRecord(t, svc, "generic"), "code"); err == nil || !strings.Contains(err.Error(), "token 端点地址校验失败") {
		t.Fatalf("缓存中的 HTTP token 端点应被实际使用点守卫拒绝，实际: %v", err)
	}

	// JWKS 初始地址绕过缓存直接调用也必须拒绝。
	if _, err := svc.getJWKS(ctx, "http://evil.example.com/jwks"); err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("HTTP JWKS 地址应被拒绝，实际: %v", err)
	}
}

func TestR3102CredentialRedirectsForbidden(t *testing.T) {
	const secret = "r3102-client-secret"
	targetRecorder := newR3102Recorder()
	tokenRecorder := newR3102Recorder()
	httpRecorder := newR3102Recorder()

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpRecorder.record(r.URL.Path, r3102ReadBody(r))
		w.WriteHeader(http.StatusOK)
	}))
	defer httpSrv.Close()

	var targetSrv *httptest.Server
	targetSrv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetRecorder.record(r.URL.Path, r3102ReadBody(r))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer targetSrv.Close()

	var tokenSrv *httptest.Server
	tokenSrv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(Discovery{
				AuthorizationEndpoint: tokenSrv.URL + "/authorize",
				TokenEndpoint:         tokenSrv.URL + "/redirect-cross-307",
				JWKSURI:               tokenSrv.URL + "/jwks",
			})
		case "/redirect-cross-307", "/redirect-cross-308":
			tokenRecorder.record(r.URL.Path, r3102ReadBody(r))
			status := http.StatusTemporaryRedirect
			if r.URL.Path == "/redirect-cross-308" {
				status = http.StatusPermanentRedirect
			}
			w.Header().Set("Location", targetSrv.URL+"/target")
			w.WriteHeader(status)
		case "/redirect-same":
			tokenRecorder.record(r.URL.Path, r3102ReadBody(r))
			w.Header().Set("Location", tokenSrv.URL+"/token2")
			w.WriteHeader(http.StatusTemporaryRedirect)
		case "/redirect-http":
			tokenRecorder.record(r.URL.Path, r3102ReadBody(r))
			w.Header().Set("Location", httpSrv.URL+"/target")
			w.WriteHeader(http.StatusTemporaryRedirect)
		case "/token2":
			tokenRecorder.record(r.URL.Path, r3102ReadBody(r))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case "/authorize", "/jwks":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer tokenSrv.Close()

	_, svc, _ := newTestOidcService(t)
	if err := svc.cfg.Set(ctx, KeyProviderType, "generic"); err != nil {
		t.Fatalf("设置真实提供商失败: %v", err)
	}
	if err := svc.SaveParams(ctx, "generic", Params{BaseURL: tokenSrv.URL, ClientID: "client", ClientSecret: secret}); err != nil {
		t.Fatalf("保存测试 OIDC 参数失败: %v", err)
	}
	svc.ClearDiscCache()
	svc.httpCli.Transport = tokenSrv.Client().Transport

	t.Run("Exchange 跨源 HTTPS 307", func(t *testing.T) {
		_, err := svc.Exchange(ctx, pinnedStateRecord(t, svc, "generic"), "code")
		if err == nil || !strings.Contains(err.Error(), "禁止重定向") {
			t.Fatalf("Exchange 应拒绝跨源 307，实际: %v", err)
		}
		if got := targetRecorder.count("/target"); got != 0 {
			t.Fatalf("跨源目标不应收到请求，实际 %d 次", got)
		}
		if targetRecorder.contains("/target", secret) {
			t.Fatal("跨源目标不应收到 Client Secret")
		}
		// 初始 token 端点正常收到 Secret，证明被阻止的是重放路径。
		if !tokenRecorder.contains("/redirect-cross-307", secret) {
			t.Fatal("初始 token 端点应收到 Client Secret")
		}
	})

	t.Run("测试连接跨源 HTTPS 308", func(t *testing.T) {
		err := svc.verifyClientCredentials(ctx, tokenSrv.URL+"/redirect-cross-308", &Params{ClientID: "client", ClientSecret: secret})
		if err == nil || !strings.Contains(err.Error(), "禁止重定向") {
			t.Fatalf("测试连接应拒绝跨源 308，实际: %v", err)
		}
		if got := targetRecorder.count("/target"); got != 0 {
			t.Fatalf("跨源目标不应收到请求，实际 %d 次", got)
		}
		if targetRecorder.contains("/target", secret) {
			t.Fatal("跨源目标不应收到 Client Secret")
		}
	})

	t.Run("同源 HTTPS 307 同样禁止", func(t *testing.T) {
		err := svc.verifyClientCredentials(ctx, tokenSrv.URL+"/redirect-same", &Params{ClientID: "client", ClientSecret: secret})
		if err == nil || !strings.Contains(err.Error(), "禁止重定向") {
			t.Fatalf("同源重定向也应按策略禁止，实际: %v", err)
		}
		if got := tokenRecorder.count("/token2"); got != 0 {
			t.Fatalf("同源第二跳不应收到请求，实际 %d 次", got)
		}
	})

	t.Run("HTTPS 降级 HTTP 被拒绝", func(t *testing.T) {
		err := svc.verifyClientCredentials(ctx, tokenSrv.URL+"/redirect-http", &Params{ClientID: "client", ClientSecret: secret})
		if err == nil || !strings.Contains(err.Error(), "HTTPS") {
			t.Fatalf("HTTPS 降级应被拒绝，实际: %v", err)
		}
		if got := httpRecorder.count("/target"); got != 0 {
			t.Fatalf("HTTP 降级目标不应收到请求，实际 %d 次", got)
		}
		if httpRecorder.contains("/target", secret) {
			t.Fatal("HTTP 降级目标不应收到 Client Secret")
		}
	})
}

func TestR3102DialContextRejectsNonPublicTargets(t *testing.T) {
	_, svc, _ := newTestOidcService(t)
	transport, ok := svc.httpCli.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("OIDC transport 类型异常: %T", svc.httpCli.Transport)
	}
	for _, addr := range []string{
		"127.0.0.1:443",
		"10.0.0.1:443",
		"169.254.1.1:443",
		"0.0.0.0:443",
		"[::1]:443",
		"[fe80::1]:443",
		"224.0.0.1:443",
		"localhost:443",
	} {
		t.Run(addr, func(t *testing.T) {
			_, err := transport.DialContext(ctx, "tcp", addr)
			if err == nil || !strings.Contains(err.Error(), "禁止访问非公网地址") {
				t.Fatalf("拨号 %s 应被非公网 IP 守卫拒绝，实际: %v", addr, err)
			}
		})
	}
}

// TestR3102TestConnectionRejectsHTTPDiscoveryEndpoint 测试连接与真实流程同口径：发现文档含 HTTP 端点时直接失败。
func TestR3102TestConnectionRejectsHTTPDiscoveryEndpoint(t *testing.T) {
	_, svc, _ := newTestOidcService(t)
	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Discovery{
			AuthorizationEndpoint: srv.URL + "/authorize",
			TokenEndpoint:         "http://idp.example.com/token",
			JWKSURI:               srv.URL + "/jwks",
		})
	}))
	defer srv.Close()
	svc.httpCli.Transport = srv.Client().Transport

	res, err := svc.TestConnection(ctx, "generic", Params{BaseURL: srv.URL, ClientID: "client", ClientSecret: "secret"})
	if err != nil {
		t.Fatalf("测试连接不应返回内部错误: %v", err)
	}
	if res == nil || res.OK || !strings.Contains(res.Message, "token_endpoint") {
		t.Fatalf("HTTP token_endpoint 应使测试连接失败并给出明确提示，实际: %+v", res)
	}
}
