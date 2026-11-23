// 本文件所有 Service ID、Refresh/Access Token 与响应正文均为合成样本，不包含真实凭据。
package providertraffic

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const (
	syntheticVPSInstanceID   = "1001"
	syntheticVPSRefreshToken = "synthetic-vps-refresh-token"
	syntheticVPSAccessToken  = "synthetic-vps-access-token"
)

func TestVPSRefreshTokenRequestShape(t *testing.T) {
	const response = `{"token":"synthetic-new-access-token","refresh":"synthetic-new-refresh-token"}`
	rt, captured := captureTransport(http.StatusOK, response)
	client, _ := newVPSClientForTest(t, rt)

	pair, err := client.RefreshToken(context.Background(), syntheticVPSRefreshToken)
	if err != nil {
		t.Fatalf("V.P.S 续期失败: %v", err)
	}
	if pair.Token != "synthetic-new-access-token" || pair.Refresh != "synthetic-new-refresh-token" {
		t.Fatalf("Token 对不一致: %+v", pair)
	}
	assertRequestLine(t, captured, http.MethodPost, "https", "vps.hosting", "/api/token")
	assertNoQueryOrSecret(t, captured, syntheticVPSRefreshToken)
	assertJSONExact(t, captured, map[string]string{"refresh_token": syntheticVPSRefreshToken})
	if got := captured.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type 不一致: %q", got)
	}
	if got := captured.Header.Get("Authorization"); got != "" {
		t.Fatalf("续期请求不应发送 Authorization: %q", got)
	}
}

func TestVPSRefreshTokenErrors(t *testing.T) {
	cases := []struct {
		name     string
		response string
		want     UpstreamErrorCategory
	}{
		{"缺 token", `{"refresh":"x"}`, UpstreamFailed},
		{"缺 refresh", `{"token":"x"}`, UpstreamFailed},
		{"token 为空", `{"token":"","refresh":"x"}`, UpstreamFailed},
		{"refresh 为空白", `{"token":"x","refresh":"   "}`, UpstreamFailed},
		{"token 为 null", `{"token":null,"refresh":"x"}`, UpstreamFailed},
		{"token 非字符串", `{"token":123,"refresh":"x"}`, UpstreamFailed},
		{"明确拒绝 Refresh Token", `{"error":["refresh_token_invalid"]}`, UpstreamCredentialRejected},
		{"明确拒绝字符串错误", `{"error":"refresh_token_invalid"}`, UpstreamCredentialRejected},
		{"明确拒绝 invalid_token", `{"error":["invalid_token"]}`, UpstreamCredentialRejected},
		{"业务错误优先于字段", `{"error":["unknown"],"token":"x","refresh":"y"}`, UpstreamFailed},
		{"未知业务错误", `{"error":["unknown"]}`, UpstreamFailed},
		{"error 形状非法", `{"error":123}`, UpstreamFailed},
		{"空对象", `{}`, UpstreamFailed},
		{"非对象", `[]`, UpstreamFailed},
		{"非 JSON", `not json`, UpstreamFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rt, _ := captureTransport(http.StatusOK, tc.response)
			client, _ := newVPSClientForTest(t, rt)
			_, err := client.RefreshToken(context.Background(), syntheticVPSRefreshToken)
			requireUpstreamCategory(t, err, tc.want)
		})
	}
}

func TestVPSRefreshTokenHTTPFailures(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   UpstreamErrorCategory
	}{
		{"HTTP 401", http.StatusUnauthorized, ``, UpstreamCredentialRejected},
		{"HTTP 403 网关", http.StatusForbidden, `<html>blocked</html>`, UpstreamUnavailable},
		{"HTTP 403 明确拒绝", http.StatusForbidden, `{"error":["invalid_token"]}`, UpstreamCredentialRejected},
		{"HTTP 429", http.StatusTooManyRequests, ``, UpstreamRateLimited},
		{"HTTP 500", http.StatusInternalServerError, ``, UpstreamFailed},
		{"拒绝重定向", http.StatusFound, ``, UpstreamUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			rt := roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				resp := testResponse(tc.status, tc.body)
				if tc.status == http.StatusFound {
					resp.Header.Set("Location", "https://example.invalid/redirect")
				}
				return resp, nil
			})
			client, _ := newVPSClientForTest(t, rt)
			_, err := client.RefreshToken(context.Background(), syntheticVPSRefreshToken)
			requireUpstreamCategory(t, err, tc.want)
			if calls != 1 {
				t.Fatalf("上游请求次数应为 1，实际 %d", calls)
			}
		})
	}
}

func TestVPSRefreshTokenTimeout(t *testing.T) {
	client, _ := newVPSClientForTest(t, blockingTransport())
	client.httpClient.Timeout = 50 * time.Millisecond
	start := time.Now()
	_, err := client.RefreshToken(context.Background(), syntheticVPSRefreshToken)
	elapsed := time.Since(start)
	requireUpstreamCategory(t, err, UpstreamUnavailable)
	if elapsed > time.Second {
		t.Fatalf("超时未被及时映射: elapsed=%s", elapsed)
	}
}

func TestVPSRefreshTokenBodyLimit(t *testing.T) {
	prefix := `{"token":"synthetic-access","refresh":"synthetic-refresh","padding":"`
	suffix := `"}`
	t.Run("恰好 256 KiB", func(t *testing.T) {
		rt, _ := captureTransport(http.StatusOK, jsonBodyOfSize(t, prefix, suffix, maxUpstreamResponseBytes))
		client, _ := newVPSClientForTest(t, rt)
		pair, err := client.RefreshToken(context.Background(), syntheticVPSRefreshToken)
		if err != nil {
			t.Fatalf("256 KiB 响应应可解析: %v", err)
		}
		if pair.Token != "synthetic-access" || pair.Refresh != "synthetic-refresh" {
			t.Fatalf("Token 对不一致: %+v", pair)
		}
	})
	t.Run("超过 256 KiB", func(t *testing.T) {
		rt, _ := captureTransport(http.StatusOK, jsonBodyOfSize(t, prefix, suffix, maxUpstreamResponseBytes+1))
		client, _ := newVPSClientForTest(t, rt)
		_, err := client.RefreshToken(context.Background(), syntheticVPSRefreshToken)
		requireUpstreamCategory(t, err, UpstreamFailed)
	})
}

func TestVPSRefreshTokenRedaction(t *testing.T) {
	const secret = "synthetic-vps-refresh-token-redaction"
	response := `{"error":["refresh_token_invalid"],"detail":"` + secret + `"}`
	rt, captured := captureTransport(http.StatusOK, response)
	client, logs := newVPSClientForTest(t, rt)
	_, err := client.RefreshToken(context.Background(), secret)
	requireUpstreamCategory(t, err, UpstreamCredentialRejected)
	requireNoSecret(t, secret, err.Error(), logs.String(), captured.URL.String())
	assertLogContains(t, logs.String(), "provider=vps", "stage=token", "category=credential_rejected", "status=200")
}

func TestVPSFetchRemainingRequestShape(t *testing.T) {
	const response = `{"limit":{"data_combined":12.5}}`
	rt, captured := captureTransport(http.StatusOK, response)
	client, _ := newVPSClientForTest(t, rt)

	sample, err := client.FetchRemaining(context.Background(), syntheticVPSInstanceID, syntheticVPSAccessToken)
	if err != nil {
		t.Fatalf("V.P.S 资源查询失败: %v", err)
	}
	requireSample(t, sample, 12.5, UnitGB)
	assertRequestLine(t, captured, http.MethodGet, "https", "vps.hosting", "/api/service/1001/resources")
	assertNoQueryOrSecret(t, captured, syntheticVPSAccessToken)
	if len(captured.Body) != 0 {
		t.Fatalf("资源查询不应携带请求体: %q", string(captured.Body))
	}
	if got := captured.Header.Get("Authorization"); got != "Bearer "+syntheticVPSAccessToken {
		t.Fatalf("Authorization 不一致: %q", got)
	}
	if got := captured.Header.Get("Content-Type"); got != "" {
		t.Fatalf("资源查询不应发送 Content-Type: %q", got)
	}
}

func TestVPSFetchRemainingValues(t *testing.T) {
	cases := []struct {
		name     string
		response string
		want     float64
	}{
		{"正常余量", `{"limit":{"data_combined":12.5}}`, 12.5},
		{"合法零", `{"limit":{"data_combined":0}}`, 0},
		{"字符串零", `{"limit":{"data_combined":"0"}}`, 0},
		{"字符串小数", `{"limit":{"data_combined":"12.5"}}`, 12.5},
		{"科学计数法", `{"limit":{"data_combined":"1e3"}}`, 1000},
		{"空错误数组不阻断", `{"error":[],"limit":{"data_combined":1}}`, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rt, _ := captureTransport(http.StatusOK, tc.response)
			client, _ := newVPSClientForTest(t, rt)
			sample, err := client.FetchRemaining(context.Background(), syntheticVPSInstanceID, syntheticVPSAccessToken)
			if err != nil {
				t.Fatalf("V.P.S 资源查询失败: %v", err)
			}
			requireSample(t, sample, tc.want, UnitGB)
		})
	}
}

func TestVPSFetchRemainingErrors(t *testing.T) {
	cases := []struct {
		name     string
		response string
		want     UpstreamErrorCategory
	}{
		{"limit 缺失", `{}`, UpstreamInvalidData},
		{"limit 为空", `{"limit":null}`, UpstreamInvalidData},
		{"data_combined 缺失", `{"limit":{}}`, UpstreamInvalidData},
		{"data_combined 为空", `{"limit":{"data_combined":null}}`, UpstreamInvalidData},
		{"data_combined 为负", `{"limit":{"data_combined":-1}}`, UpstreamInvalidData},
		{"负余量下溢", `{"limit":{"data_combined":-1e-400}}`, UpstreamInvalidData},
		{"data_combined 带单位", `{"limit":{"data_combined":"100 GB"}}`, UpstreamInvalidData},
		{"data_combined 千分位", `{"limit":{"data_combined":"1,024"}}`, UpstreamInvalidData},
		{"data_combined 布尔", `{"limit":{"data_combined":true}}`, UpstreamInvalidData},
		{"data_combined 溢出", `{"limit":{"data_combined":1e400}}`, UpstreamInvalidData},
		{"非 JSON", `not json`, UpstreamInvalidData},
		{"业务错误明确拒绝 Access Token", `{"error":["invalid_token"]}`, UpstreamAccessTokenRejected},
		{"业务错误字符串明确拒绝", `{"error":"invalid_token"}`, UpstreamAccessTokenRejected},
		{"业务错误优先于余量", `{"error":["unknown"],"limit":{"data_combined":1}}`, UpstreamFailed},
		{"业务错误未知", `{"error":["unknown"]}`, UpstreamFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rt, _ := captureTransport(http.StatusOK, tc.response)
			client, _ := newVPSClientForTest(t, rt)
			_, err := client.FetchRemaining(context.Background(), syntheticVPSInstanceID, syntheticVPSAccessToken)
			requireUpstreamCategory(t, err, tc.want)
		})
	}
}

func TestVPSFetchRemainingHTTPFailures(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   UpstreamErrorCategory
	}{
		{"HTTP 401", http.StatusUnauthorized, ``, UpstreamAccessTokenRejected},
		{"HTTP 403 网关", http.StatusForbidden, `<html>blocked</html>`, UpstreamUnavailable},
		{"HTTP 403 明确拒绝", http.StatusForbidden, `{"error":["invalid_token"]}`, UpstreamAccessTokenRejected},
		{"HTTP 404 明确拒绝", http.StatusNotFound, `{"error":["invalid_token","invalid method"]}`, UpstreamAccessTokenRejected},
		{"HTTP 404 路由错误", http.StatusNotFound, `{"error":["invalid method"]}`, UpstreamFailed},
		{"HTTP 429", http.StatusTooManyRequests, ``, UpstreamRateLimited},
		{"HTTP 500", http.StatusInternalServerError, ``, UpstreamFailed},
		{"拒绝重定向", http.StatusPermanentRedirect, ``, UpstreamUnavailable},
		{"无跳转目标的 302", http.StatusFound, ``, UpstreamFailed},
		{"未修改", http.StatusNotModified, ``, UpstreamFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			rt := roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				resp := testResponse(tc.status, tc.body)
				if tc.status == http.StatusPermanentRedirect {
					resp.Header.Set("Location", "https://example.invalid/redirect")
				}
				return resp, nil
			})
			client, _ := newVPSClientForTest(t, rt)
			_, err := client.FetchRemaining(context.Background(), syntheticVPSInstanceID, syntheticVPSAccessToken)
			requireUpstreamCategory(t, err, tc.want)
			if calls != 1 {
				t.Fatalf("上游请求次数应为 1，实际 %d", calls)
			}
		})
	}
}

func TestVPSFetchRemainingTimeout(t *testing.T) {
	client, _ := newVPSClientForTest(t, blockingTransport())
	client.httpClient.Timeout = 50 * time.Millisecond
	start := time.Now()
	_, err := client.FetchRemaining(context.Background(), syntheticVPSInstanceID, syntheticVPSAccessToken)
	elapsed := time.Since(start)
	requireUpstreamCategory(t, err, UpstreamUnavailable)
	if elapsed > time.Second {
		t.Fatalf("超时未被及时映射: elapsed=%s", elapsed)
	}
}

// failureBodyReader 用请求上下文模拟正文开始发送后的读取失败。
type failureBodyReader func([]byte) (int, error)

func (read failureBodyReader) Read(p []byte) (int, error) { return read(p) }

func TestVPSHTTPFailureBodyTimeout(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		refresh  bool
		wantRead bool
		want     UpstreamErrorCategory
	}{
		{"续期 404 正文超时", http.StatusNotFound, true, true, UpstreamUnavailable},
		{"资源 404 正文超时", http.StatusNotFound, false, true, UpstreamUnavailable},
		{"续期 401 无需读正文", http.StatusUnauthorized, true, false, UpstreamCredentialRejected},
		{"资源 401 无需读正文", http.StatusUnauthorized, false, false, UpstreamAccessTokenRejected},
		{"429 无需读正文", http.StatusTooManyRequests, false, false, UpstreamRateLimited},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			const syntheticDetail = "synthetic-failure-body-detail"
			read := false
			rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				resp := testResponse(tc.status, "")
				wait := failureBodyReader(func([]byte) (int, error) {
					read = true
					<-req.Context().Done()
					return 0, req.Context().Err()
				})
				resp.Body = io.NopCloser(io.MultiReader(strings.NewReader(`{"detail":"`+syntheticDetail), wait))
				return resp, nil
			})
			client, logs := newVPSClientForTest(t, rt)
			client.httpClient.Timeout = 50 * time.Millisecond
			var err error
			if tc.refresh {
				_, err = client.RefreshToken(context.Background(), syntheticVPSRefreshToken)
			} else {
				_, err = client.FetchRemaining(context.Background(), syntheticVPSInstanceID, syntheticVPSAccessToken)
			}
			requireUpstreamCategory(t, err, tc.want)
			if read != tc.wantRead {
				t.Fatalf("等待正文=%t，期望 %t", read, tc.wantRead)
			}
			requireNoSecret(t, syntheticDetail, err.Error(), logs.String())
		})
	}
}

func TestVPSFetchRemainingBodyLimit(t *testing.T) {
	prefix := `{"limit":{"data_combined":12.5},"padding":"`
	suffix := `"}`
	t.Run("恰好 256 KiB", func(t *testing.T) {
		rt, _ := captureTransport(http.StatusOK, jsonBodyOfSize(t, prefix, suffix, maxUpstreamResponseBytes))
		client, _ := newVPSClientForTest(t, rt)
		sample, err := client.FetchRemaining(context.Background(), syntheticVPSInstanceID, syntheticVPSAccessToken)
		if err != nil {
			t.Fatalf("256 KiB 响应应可解析: %v", err)
		}
		requireSample(t, sample, 12.5, UnitGB)
	})
	t.Run("超过 256 KiB", func(t *testing.T) {
		rt, _ := captureTransport(http.StatusOK, jsonBodyOfSize(t, prefix, suffix, maxUpstreamResponseBytes+1))
		client, _ := newVPSClientForTest(t, rt)
		_, err := client.FetchRemaining(context.Background(), syntheticVPSInstanceID, syntheticVPSAccessToken)
		requireUpstreamCategory(t, err, UpstreamInvalidData)
	})
	t.Run("404 正文超限沿用状态分类", func(t *testing.T) {
		body := jsonBodyOfSize(t, `{"error":["invalid_token"],"padding":"`, suffix, maxUpstreamResponseBytes+1)
		rt, _ := captureTransport(http.StatusNotFound, body)
		client, _ := newVPSClientForTest(t, rt)
		_, err := client.FetchRemaining(context.Background(), syntheticVPSInstanceID, syntheticVPSAccessToken)
		requireUpstreamCategory(t, err, UpstreamFailed)
	})
}

func TestVPSFetchRemainingInvalidInstanceID(t *testing.T) {
	cases := []struct {
		name string
		id   string
	}{
		{"空", ""},
		{"零", "0"},
		{"前导零", "01"},
		{"非数字", "abc"},
		{"前导空白", " 1"},
		{"尾随空白", "1 "},
		{"负数", "-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			rt := roundTripFunc(func(*http.Request) (*http.Response, error) {
				called = true
				return testResponse(http.StatusOK, `{}`), nil
			})
			client, _ := newVPSClientForTest(t, rt)
			_, err := client.FetchRemaining(context.Background(), tc.id, syntheticVPSAccessToken)
			if !errors.Is(err, ErrInvalidInstanceID) {
				t.Fatalf("期望 ErrInvalidInstanceID，实际 %v", err)
			}
			if called {
				t.Fatal("非法实例 ID 不应发起上游请求")
			}
		})
	}
}

func TestVPSFetchRemainingRedaction(t *testing.T) {
	const secret = "synthetic-vps-access-token-redaction"
	response := `{"error":["invalid_token"],"detail":"` + secret + `"}`
	rt, captured := captureTransport(http.StatusUnauthorized, response)
	client, logs := newVPSClientForTest(t, rt)
	_, err := client.FetchRemaining(context.Background(), syntheticVPSInstanceID, secret)
	requireUpstreamCategory(t, err, UpstreamAccessTokenRejected)
	requireNoSecret(t, secret, err.Error(), logs.String(), captured.URL.String())
	assertLogContains(t, logs.String(), "provider=vps", "stage=resources", "category=access_token_rejected", "status=401")
}

func TestVPSGateway403DoesNotLeak(t *testing.T) {
	const secret = "synthetic-vps-access-token-403"
	rt, _ := captureTransport(http.StatusForbidden, `<html>blocked `+secret+`</html>`)
	client, logs := newVPSClientForTest(t, rt)
	_, err := client.FetchRemaining(context.Background(), syntheticVPSInstanceID, secret)
	requireUpstreamCategory(t, err, UpstreamUnavailable)
	requireNoSecret(t, secret, err.Error(), logs.String())
}
