// 本文件所有 VEID、API Key 与响应正文均为合成样本，不包含真实凭据。
package providertraffic

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

const (
	syntheticKiwiVMInstanceID = "1001"
	syntheticKiwiVMAPIKey     = "synthetic-kiwivm-api-key"
	syntheticKiwiVMSpecialKey = "synthetic-kiwivm-key-&=+ /"
)

func TestKiwiVMFetchRemainingRequestShape(t *testing.T) {
	const response = `{"error":0,"plan_monthly_data":3221225472,"data_counter":1073741824,"monthly_data_multiplier":1}`
	rt, captured := captureTransport(http.StatusOK, response)
	client, _ := newKiwiVMClientForTest(t, rt)

	sample, err := client.FetchRemaining(context.Background(), syntheticKiwiVMInstanceID, syntheticKiwiVMSpecialKey)
	if err != nil {
		t.Fatalf("KiwiVM 请求失败: %v", err)
	}
	requireSample(t, sample, 2, UnitGiB)
	assertRequestLine(t, captured, http.MethodPost, "https", "api.64clouds.com", "/v1/getServiceInfo")
	assertNoQueryOrSecret(t, captured, syntheticKiwiVMSpecialKey)
	assertFormExact(t, captured, map[string]string{
		"veid":    syntheticKiwiVMInstanceID,
		"api_key": syntheticKiwiVMSpecialKey,
	})
	if got := captured.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
		t.Fatalf("Content-Type 不一致: %q", got)
	}
	if got := captured.Header.Get("Authorization"); got != "" {
		t.Fatalf("KiwiVM 不应发送 Authorization: %q", got)
	}
}

func TestKiwiVMFetchRemainingValues(t *testing.T) {
	cases := []struct {
		name     string
		response string
		want     float64
	}{
		{
			name:     "正常余量",
			response: `{"error":0,"plan_monthly_data":107374182400,"data_counter":21474836480,"monthly_data_multiplier":1}`,
			want:     80,
		},
		{
			name:     "剩余合法零",
			response: `{"error":0,"plan_monthly_data":1073741824,"data_counter":1073741824,"monthly_data_multiplier":1}`,
			want:     0,
		},
		{
			name:     "已用量合法零",
			response: `{"error":0,"plan_monthly_data":1073741824,"data_counter":0,"monthly_data_multiplier":1}`,
			want:     1,
		},
		{
			name:     "已用量超过套餐时钳制为零",
			response: `{"error":0,"plan_monthly_data":1073741824,"data_counter":2147483648,"monthly_data_multiplier":1}`,
			want:     0,
		},
		{
			name:     "字符串数值",
			response: `{"error":"0","plan_monthly_data":"2147483648","data_counter":"1073741824","monthly_data_multiplier":"1"}`,
			want:     1,
		},
		{
			name:     "等值指数形式",
			response: `{"error":"-0e1000","plan_monthly_data":1073741824,"data_counter":0,"monthly_data_multiplier":"100.0e-2"}`,
			want:     1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rt, _ := captureTransport(http.StatusOK, tc.response)
			client, _ := newKiwiVMClientForTest(t, rt)
			sample, err := client.FetchRemaining(context.Background(), syntheticKiwiVMInstanceID, syntheticKiwiVMAPIKey)
			if err != nil {
				t.Fatalf("KiwiVM 请求失败: %v", err)
			}
			requireSample(t, sample, tc.want, UnitGiB)
		})
	}
}

func TestKiwiVMFetchRemainingErrors(t *testing.T) {
	cases := []struct {
		name     string
		response string
		want     UpstreamErrorCategory
	}{
		{"凭据被拒绝", `{"error":700005,"message":"Authentication failure"}`, UpstreamCredentialRejected},
		{"等值错误码", `{"error":"70000500e-2"}`, UpstreamCredentialRejected},
		{"近似认证错误码", `{"error":700005.00000000001}`, UpstreamFailed},
		{"其他业务错误", `{"error":1,"message":"boom"}`, UpstreamFailed},
		{"非零错误码下溢", `{"error":1e-400,"plan_monthly_data":1073741824,"data_counter":0,"monthly_data_multiplier":1}`, UpstreamFailed},
		{"error 缺失", `{"plan_monthly_data":1073741824,"data_counter":0,"monthly_data_multiplier":1}`, UpstreamFailed},
		{"error 为 null", `{"error":null,"plan_monthly_data":1073741824,"data_counter":0,"monthly_data_multiplier":1}`, UpstreamFailed},
		{"error 非法", `{"error":"abc","plan_monthly_data":1073741824,"data_counter":0,"monthly_data_multiplier":1}`, UpstreamFailed},
		{"倍率缺失", `{"error":0,"plan_monthly_data":1073741824,"data_counter":0}`, UpstreamMultiplierCheck},
		{"倍率为二", `{"error":0,"plan_monthly_data":1073741824,"data_counter":0,"monthly_data_multiplier":2}`, UpstreamMultiplierCheck},
		{"倍率接近但不等于一", `{"error":0,"plan_monthly_data":1073741824,"data_counter":0,"monthly_data_multiplier":1.0000000000000001}`, UpstreamMultiplierCheck},
		{"倍率为零", `{"error":0,"plan_monthly_data":1073741824,"data_counter":0,"monthly_data_multiplier":0}`, UpstreamMultiplierCheck},
		{"倍率为负", `{"error":0,"plan_monthly_data":1073741824,"data_counter":0,"monthly_data_multiplier":-1}`, UpstreamMultiplierCheck},
		{"倍率为空值", `{"error":0,"plan_monthly_data":1073741824,"data_counter":0,"monthly_data_multiplier":null}`, UpstreamMultiplierCheck},
		{"倍率为非数", `{"error":0,"plan_monthly_data":1073741824,"data_counter":0,"monthly_data_multiplier":"abc"}`, UpstreamMultiplierCheck},
		{"套餐缺失", `{"error":0,"data_counter":0,"monthly_data_multiplier":1}`, UpstreamInvalidData},
		{"套餐为零", `{"error":0,"plan_monthly_data":0,"data_counter":0,"monthly_data_multiplier":1}`, UpstreamInvalidData},
		{"套餐为负", `{"error":0,"plan_monthly_data":-1,"data_counter":0,"monthly_data_multiplier":1}`, UpstreamInvalidData},
		{"已用量缺失", `{"error":0,"plan_monthly_data":1073741824,"monthly_data_multiplier":1}`, UpstreamInvalidData},
		{"已用量为负", `{"error":0,"plan_monthly_data":1073741824,"data_counter":-1,"monthly_data_multiplier":1}`, UpstreamInvalidData},
		{"负已用量下溢", `{"error":0,"plan_monthly_data":1073741824,"data_counter":-1e-400,"monthly_data_multiplier":1}`, UpstreamInvalidData},
		{"带单位的数值", `{"error":0,"plan_monthly_data":"1 GB","data_counter":0,"monthly_data_multiplier":1}`, UpstreamInvalidData},
		{"非 JSON", `not json`, UpstreamInvalidData},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rt, _ := captureTransport(http.StatusOK, tc.response)
			client, _ := newKiwiVMClientForTest(t, rt)
			_, err := client.FetchRemaining(context.Background(), syntheticKiwiVMInstanceID, syntheticKiwiVMAPIKey)
			requireUpstreamCategory(t, err, tc.want)
		})
	}
}

func TestKiwiVMFetchRemainingHTTPFailures(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   UpstreamErrorCategory
	}{
		{"HTTP 401", http.StatusUnauthorized, `{"error":700005}`, UpstreamCredentialRejected},
		{"HTTP 403 网关", http.StatusForbidden, `<html>cloudflare blocked synthetic-kiwivm-api-key</html>`, UpstreamUnavailable},
		{"HTTP 429", http.StatusTooManyRequests, ``, UpstreamRateLimited},
		{"HTTP 500", http.StatusInternalServerError, ``, UpstreamFailed},
		{"拒绝重定向", http.StatusTemporaryRedirect, ``, UpstreamUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			rt := roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				resp := testResponse(tc.status, tc.body)
				if tc.status == http.StatusTemporaryRedirect {
					resp.Header.Set("Location", "https://example.invalid/redirect")
				}
				return resp, nil
			})
			client, _ := newKiwiVMClientForTest(t, rt)
			_, err := client.FetchRemaining(context.Background(), syntheticKiwiVMInstanceID, syntheticKiwiVMAPIKey)
			requireUpstreamCategory(t, err, tc.want)
			if calls != 1 {
				t.Fatalf("上游请求次数应为 1，实际 %d", calls)
			}
		})
	}
}

func TestKiwiVMFetchRemainingTimeout(t *testing.T) {
	client, _ := newKiwiVMClientForTest(t, blockingTransport())
	client.httpClient.Timeout = 50 * time.Millisecond
	start := time.Now()
	_, err := client.FetchRemaining(context.Background(), syntheticKiwiVMInstanceID, syntheticKiwiVMAPIKey)
	elapsed := time.Since(start)
	requireUpstreamCategory(t, err, UpstreamUnavailable)
	if elapsed > time.Second {
		t.Fatalf("超时未被及时映射: elapsed=%s", elapsed)
	}
}

func TestKiwiVMFetchRemainingBodyLimit(t *testing.T) {
	prefix := `{"error":0,"plan_monthly_data":1073741824,"data_counter":0,"monthly_data_multiplier":1,"padding":"`
	suffix := `"}`
	t.Run("恰好 256 KiB", func(t *testing.T) {
		rt, _ := captureTransport(http.StatusOK, jsonBodyOfSize(t, prefix, suffix, maxUpstreamResponseBytes))
		client, _ := newKiwiVMClientForTest(t, rt)
		sample, err := client.FetchRemaining(context.Background(), syntheticKiwiVMInstanceID, syntheticKiwiVMAPIKey)
		if err != nil {
			t.Fatalf("256 KiB 响应应可解析: %v", err)
		}
		requireSample(t, sample, 1, UnitGiB)
	})
	t.Run("超过 256 KiB", func(t *testing.T) {
		rt, _ := captureTransport(http.StatusOK, jsonBodyOfSize(t, prefix, suffix, maxUpstreamResponseBytes+1))
		client, _ := newKiwiVMClientForTest(t, rt)
		_, err := client.FetchRemaining(context.Background(), syntheticKiwiVMInstanceID, syntheticKiwiVMAPIKey)
		requireUpstreamCategory(t, err, UpstreamInvalidData)
	})
}

func TestKiwiVMFetchRemainingInvalidInstanceID(t *testing.T) {
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
			client, _ := newKiwiVMClientForTest(t, rt)
			_, err := client.FetchRemaining(context.Background(), tc.id, syntheticKiwiVMAPIKey)
			if !errors.Is(err, ErrInvalidInstanceID) {
				t.Fatalf("期望 ErrInvalidInstanceID，实际 %v", err)
			}
			if called {
				t.Fatal("非法实例 ID 不应发起上游请求")
			}
		})
	}
}

func TestKiwiVMFetchRemainingRedaction(t *testing.T) {
	const secret = "synthetic-kiwivm-api-key-redaction"
	response := `{"error":700005,"message":"Authentication failure for ` + secret + `"}`
	rt, captured := captureTransport(http.StatusOK, response)
	client, logs := newKiwiVMClientForTest(t, rt)
	_, err := client.FetchRemaining(context.Background(), syntheticKiwiVMInstanceID, secret)
	requireUpstreamCategory(t, err, UpstreamCredentialRejected)
	requireNoSecret(t, secret, err.Error(), logs.String(), captured.URL.String())
	assertLogContains(t, logs.String(), "provider=kiwivm", "stage=service_info", "category=credential_rejected", "status=200")
}

func TestKiwiVMGateway403DoesNotLeak(t *testing.T) {
	const secret = "synthetic-kiwivm-api-key-403"
	rt, _ := captureTransport(http.StatusForbidden, `<html>blocked `+secret+`</html>`)
	client, logs := newKiwiVMClientForTest(t, rt)
	_, err := client.FetchRemaining(context.Background(), syntheticKiwiVMInstanceID, secret)
	requireUpstreamCategory(t, err, UpstreamUnavailable)
	requireNoSecret(t, secret, err.Error(), logs.String())
}
