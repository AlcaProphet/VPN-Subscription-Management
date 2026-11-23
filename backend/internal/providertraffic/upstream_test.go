package providertraffic

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"vpn-sub/internal/log"
)

// newTestLogger 返回带生产脱敏包装且写入内存缓冲的测试日志器。
func newTestLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	handler := log.NewRedactHandler(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return slog.New(handler), &buf
}

// roundTripFunc 让测试以函数形式实现 http.RoundTripper。
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// testResponse 构造带可控状态码和正文的测试响应。
func testResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// capturedRequest 保存 mock transport 实际收到的请求副本。
type capturedRequest struct {
	Method string
	URL    *url.URL
	Header http.Header
	Body   []byte
}

// captureTransport 返回固定响应并捕获第一个请求。
func captureTransport(status int, responseBody string) (http.RoundTripper, *capturedRequest) {
	captured := &capturedRequest{}
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body []byte
		if req.Body != nil {
			var err error
			body, err = io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
		}
		captured.Method = req.Method
		captured.URL = req.URL
		captured.Header = req.Header.Clone()
		captured.Body = body
		return testResponse(status, responseBody), nil
	})
	return rt, captured
}

// newKiwiVMClientForTest 复用生产构造器，仅替换 Transport，保留超时和重定向策略。
func newKiwiVMClientForTest(t *testing.T, rt http.RoundTripper) (*KiwiVMClient, *bytes.Buffer) {
	t.Helper()
	lg, buf := newTestLogger()
	client := NewKiwiVMClient(lg)
	client.httpClient.Transport = rt
	return client, buf
}

// newVPSClientForTest 复用生产构造器，仅替换 Transport，保留超时和重定向策略。
func newVPSClientForTest(t *testing.T, rt http.RoundTripper) (*VPSClient, *bytes.Buffer) {
	t.Helper()
	lg, buf := newTestLogger()
	client := NewVPSClient(lg)
	client.httpClient.Transport = rt
	return client, buf
}

// requireUpstreamCategory 断言错误是受控 UpstreamError、类别与文案正确且不携带原始错误链。
func requireUpstreamCategory(t *testing.T, err error, want UpstreamErrorCategory) {
	t.Helper()
	if err == nil {
		t.Fatalf("期望上游错误 %s，实际为 nil", want)
	}
	var upstreamErr *UpstreamError
	if !errors.As(err, &upstreamErr) {
		t.Fatalf("期望 *UpstreamError，实际为 %T: %v", err, err)
	}
	if upstreamErr.Category != want {
		t.Fatalf("错误类别不一致: got %s want %s", upstreamErr.Category, want)
	}
	if got := err.Error(); got != want.Message() {
		t.Fatalf("错误文案不一致: got %q want %q", got, want.Message())
	}
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		t.Fatalf("上游错误不得包装原始错误链: %v", unwrapped)
	}
}

// requireSample 断言余量与单位。
func requireSample(t *testing.T, got TrafficSample, wantRemaining float64, wantUnit string) {
	t.Helper()
	if got.Unit != wantUnit {
		t.Fatalf("单位不一致: got %q want %q", got.Unit, wantUnit)
	}
	if math.Abs(got.Remaining-wantRemaining) > 1e-9 {
		t.Fatalf("余量不一致: got %v want %v", got.Remaining, wantRemaining)
	}
}

// assertRequestLine 断言方法、协议、主机和路径。
func assertRequestLine(t *testing.T, got *capturedRequest, method, scheme, host, path string) {
	t.Helper()
	if got.Method != method {
		t.Fatalf("方法不一致: got %s want %s", got.Method, method)
	}
	if got.URL == nil {
		t.Fatal("未捕获请求 URL")
	}
	if got.URL.Scheme != scheme {
		t.Fatalf("协议不一致: got %s want %s", got.URL.Scheme, scheme)
	}
	if got.URL.Host != host {
		t.Fatalf("主机不一致: got %s want %s", got.URL.Host, host)
	}
	if got.URL.Path != path {
		t.Fatalf("路径不一致: got %s want %s", got.URL.Path, path)
	}
}

// assertNoQueryOrSecret 断言 URL 没有查询参数、fragment 或凭据明文。
func assertNoQueryOrSecret(t *testing.T, got *capturedRequest, secrets ...string) {
	t.Helper()
	if got.URL == nil {
		t.Fatal("未捕获请求 URL")
	}
	if got.URL.RawQuery != "" {
		t.Fatalf("请求不得携带查询参数: %s", got.URL.RawQuery)
	}
	if got.URL.Fragment != "" {
		t.Fatalf("请求不得携带 fragment: %s", got.URL.Fragment)
	}
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		if strings.Contains(got.URL.String(), secret) {
			t.Fatalf("URL 包含凭据明文: %s", got.URL.String())
		}
	}
}

// assertFormExact 断言表单字段数量与取值完全一致。
func assertFormExact(t *testing.T, got *capturedRequest, want map[string]string) {
	t.Helper()
	values, err := url.ParseQuery(string(got.Body))
	if err != nil {
		t.Fatalf("解析表单失败: %v", err)
	}
	if len(values) != len(want) {
		t.Fatalf("表单字段数量不一致: got %d want %d (%v)", len(values), len(want), values)
	}
	for key, expected := range want {
		if actual := values.Get(key); actual != expected {
			t.Fatalf("表单字段 %s 不一致: got %q want %q", key, actual, expected)
		}
	}
}

// assertJSONExact 断言 JSON 请求体字段数量与字符串取值完全一致。
func assertJSONExact(t *testing.T, got *capturedRequest, want map[string]string) {
	t.Helper()
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(got.Body, &payload); err != nil {
		t.Fatalf("解析 JSON 请求体失败: %v", err)
	}
	if len(payload) != len(want) {
		t.Fatalf("JSON 字段数量不一致: got %d want %d", len(payload), len(want))
	}
	for key, expected := range want {
		raw, ok := payload[key]
		if !ok {
			t.Fatalf("缺少 JSON 字段 %s", key)
		}
		var actual string
		if err := json.Unmarshal(raw, &actual); err != nil {
			t.Fatalf("JSON 字段 %s 不是字符串: %v", key, err)
		}
		if actual != expected {
			t.Fatalf("JSON 字段 %s 不一致: got %q want %q", key, actual, expected)
		}
	}
}

// requireNoSecret 断言输出中不含凭据明文。
func requireNoSecret(t *testing.T, secret string, outputs ...string) {
	t.Helper()
	if secret == "" {
		return
	}
	for _, output := range outputs {
		if strings.Contains(output, secret) {
			t.Fatalf("输出包含凭据明文: %q", output)
		}
	}
}

// assertLogContains 断言日志包含安全字段。
func assertLogContains(t *testing.T, logOutput string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(logOutput, part) {
			t.Fatalf("日志缺少安全字段 %q: %s", part, logOutput)
		}
	}
}

// jsonBodyOfSize 构造指定字节数的合法 JSON 对象，用于响应体上限边界测试。
func jsonBodyOfSize(t *testing.T, prefix, suffix string, size int) string {
	t.Helper()
	padding := size - len(prefix) - len(suffix)
	if padding < 0 {
		t.Fatalf("JSON 前缀与后缀超过目标大小: prefix=%d suffix=%d size=%d", len(prefix), len(suffix), size)
	}
	body := prefix + strings.Repeat("a", padding) + suffix
	if len(body) != size {
		t.Fatalf("生成的 JSON 大小不一致: got %d want %d", len(body), size)
	}
	return body
}

// blockingTransport 阻塞到请求上下文结束，用于验证客户端超时映射。
func blockingTransport() http.RoundTripper {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-timer.C:
			return nil, errors.New("测试传输层等待超时")
		}
	})
}

func TestUpstreamErrorMessages(t *testing.T) {
	cases := []struct {
		category UpstreamErrorCategory
		want     string
	}{
		{UpstreamUnavailable, "无法连接服务商"},
		{UpstreamRateLimited, "服务商限流"},
		{UpstreamMultiplierCheck, "计费倍率待核对"},
		{UpstreamInvalidData, "服务商返回的流量数据无法读取"},
		{UpstreamCredentialRejected, "服务商凭据被拒绝"},
		{UpstreamAccessTokenRejected, "服务商鉴权失败"},
		{UpstreamFailed, "服务商返回错误"},
		{UpstreamErrorCategory("unknown"), "服务商返回错误"},
	}
	for _, tc := range cases {
		t.Run(string(tc.category), func(t *testing.T) {
			if got := tc.category.Message(); got != tc.want {
				t.Fatalf("文案不一致: got %q want %q", got, tc.want)
			}
			if got := (&UpstreamError{Category: tc.category}).Error(); got != tc.want {
				t.Fatalf("错误文案不一致: got %q want %q", got, tc.want)
			}
		})
	}
}

func TestParseFiniteNumber(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want float64
		ok   bool
	}{
		{"JSON 零", `0`, 0, true},
		{"JSON 正整数", `1073741824`, 1073741824, true},
		{"JSON 小数", `12.5`, 12.5, true},
		{"JSON 指数", `1e3`, 1000, true},
		{"JSON 负零", `-0`, 0, true},
		{"字符串零", `"0"`, 0, true},
		{"字符串小数", `"12.5"`, 12.5, true},
		{"字符串指数", `"1e3"`, 1000, true},
		{"长尾零与指数抵消", "1" + strings.Repeat("0", 1000) + "e-1000", 1, true},
		{"负数可解析", `-1`, -1, true},
		{"缺失", ``, 0, false},
		{"null", `null`, 0, false},
		{"布尔", `true`, 0, false},
		{"对象", `{}`, 0, false},
		{"数组", `[]`, 0, false},
		{"带单位", `"100 GB"`, 0, false},
		{"千分位", `"1,024"`, 0, false},
		{"十六进制", `"0x10"`, 0, false},
		{"NaN 字符串", `"NaN"`, 0, false},
		{"Inf 字符串", `"Inf"`, 0, false},
		{"空字符串", `""`, 0, false},
		{"空白字符串", `" 1 "`, 0, false},
		{"溢出", `1e400`, 0, false},
		{"下溢为零", `1e-400`, 0, false},
		{"可表示的极小正数", `5e-324`, math.SmallestNonzeroFloat64, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseFiniteNumber(json.RawMessage(tc.raw))
			if ok != tc.ok {
				t.Fatalf("可解析性不一致: got %t want %t", ok, tc.ok)
			}
			if ok && (got != tc.want || got == 0 && math.Signbit(got)) {
				t.Fatalf("数值不一致: got %v want %v", got, tc.want)
			}
		})
	}
}

func TestUpstreamClientConfiguration(t *testing.T) {
	if upstreamTimeout != 8*time.Second {
		t.Fatalf("上游超时必须为 8 秒，实际 %s", upstreamTimeout)
	}
	if maxUpstreamResponseBytes != 256*1024 {
		t.Fatalf("上游响应体上限必须为 256 KiB，实际 %d", maxUpstreamResponseBytes)
	}
	lg, _ := newTestLogger()
	for name, client := range map[string]*http.Client{
		"kiwivm": NewKiwiVMClient(lg).httpClient,
		"vps":    NewVPSClient(lg).httpClient,
	} {
		if client.Timeout != upstreamTimeout {
			t.Fatalf("%s 客户端超时不一致: got %s want %s", name, client.Timeout, upstreamTimeout)
		}
		if client.CheckRedirect == nil {
			t.Fatalf("%s 客户端必须配置拒绝重定向", name)
		}
	}
}

func TestReadUpstreamBodyLimit(t *testing.T) {
	exact := strings.Repeat("a", maxUpstreamResponseBytes)
	body, err := readUpstreamBody(strings.NewReader(exact))
	if err != nil {
		t.Fatalf("恰好 256 KiB 应可读取: %v", err)
	}
	if len(body) != maxUpstreamResponseBytes {
		t.Fatalf("读取长度不一致: got %d want %d", len(body), maxUpstreamResponseBytes)
	}
	tooLarge := strings.Repeat("a", maxUpstreamResponseBytes+1)
	if _, err := readUpstreamBody(strings.NewReader(tooLarge)); !errors.Is(err, errUpstreamBodyTooLarge) {
		t.Fatalf("超过 256 KiB 应返回内部限读哨兵，实际 %v", err)
	}
}
