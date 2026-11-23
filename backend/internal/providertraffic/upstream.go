package providertraffic

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// upstreamTimeout 是单次上游请求的总超时，覆盖连接、TLS、请求和响应读取。
	upstreamTimeout = 8 * time.Second
	// maxUpstreamResponseBytes 是单次上游响应体读取上限，256 KiB。
	maxUpstreamResponseBytes = 256 * 1024

	// UnitGiB 是 KiwiVM 余量的固定单位。
	UnitGiB = "GiB"
	// UnitGB 是 V.P.S 余量的固定单位。
	UnitGB = "GB"
)

// 日志阶段固定为三种上游动作，不携带实例 ID、URL 或凭据。
const (
	stageKiwiVMServiceInfo = "service_info"
	stageVPSToken          = "token"
	stageVPSResources      = "resources"
)

// TrafficSample 是一次上游读取得到的余量和固定单位。
// 采集时间、平台标识和错误状态由后续采集层组装，本层不持久化。
type TrafficSample struct {
	Remaining float64
	Unit      string
}

// UpstreamErrorCategory 是上游失败的安全分类；对外文案固定，不包含上游原始正文。
type UpstreamErrorCategory string

const (
	// UpstreamUnavailable 表示网络、TLS、超时、被拒绝的重定向或未明确凭据失效的 403。
	UpstreamUnavailable UpstreamErrorCategory = "unavailable"
	// UpstreamRateLimited 表示上游返回 429。
	UpstreamRateLimited UpstreamErrorCategory = "rate_limited"
	// UpstreamMultiplierCheck 表示 KiwiVM 计费倍率缺失、异常或非 1。
	UpstreamMultiplierCheck UpstreamErrorCategory = "multiplier_check"
	// UpstreamInvalidData 表示流量字段缺失、非法或无法解析。
	UpstreamInvalidData UpstreamErrorCategory = "invalid_data"
	// UpstreamCredentialRejected 表示上游明确拒绝 API Key 或 Refresh Token。
	UpstreamCredentialRejected UpstreamErrorCategory = "credential_rejected"
	// UpstreamAccessTokenRejected 表示 V.P.S 明确拒绝 Access Token。
	UpstreamAccessTokenRejected UpstreamErrorCategory = "access_token_rejected"
	// UpstreamFailed 表示其他非成功 HTTP 或业务结果。
	UpstreamFailed UpstreamErrorCategory = "upstream_failed"
)

// Message 返回 Build41 §3.3 冻结的短文案。
func (c UpstreamErrorCategory) Message() string {
	switch c {
	case UpstreamUnavailable:
		return "无法连接服务商"
	case UpstreamRateLimited:
		return "服务商限流"
	case UpstreamMultiplierCheck:
		return "计费倍率待核对"
	case UpstreamInvalidData:
		return "服务商返回的流量数据无法读取"
	case UpstreamCredentialRejected:
		return "服务商凭据被拒绝"
	case UpstreamAccessTokenRejected:
		return "服务商鉴权失败"
	default:
		return "服务商返回错误"
	}
}

// UpstreamError 只携带受控分类；不包装原始网络或解析错误，
// 避免调用方通过错误链取得上游正文、凭据或底层地址。
type UpstreamError struct {
	Category UpstreamErrorCategory
}

// Error 返回与冻结合同一致的安全提示。
func (e *UpstreamError) Error() string {
	return e.Category.Message()
}

// newUpstreamHTTPClient 构造固定出站客户端：8 秒总超时并拒绝任何重定向。
// 不设置 Transport，沿用默认 Transport 的代理与连接池行为。
func newUpstreamHTTPClient() *http.Client {
	return &http.Client{
		Timeout: upstreamTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// isUpstreamRedirect 识别 CheckRedirect 拒绝跟随后交回的首跳响应。
func isUpstreamRedirect(resp *http.Response) bool {
	switch resp.StatusCode {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return resp.Header.Get("Location") != ""
	default:
		return false
	}
}

// logUpstreamFailure 只记录平台、阶段、受控分类和 HTTP 状态码；
// 禁止记录 URL、实例 ID、请求体、响应体、请求头或凭据。
func logUpstreamFailure(lg *slog.Logger, provider Provider, stage string, category UpstreamErrorCategory, status int) {
	lg.Warn("服务商上游请求失败",
		"provider", string(provider),
		"stage", stage,
		"category", string(category),
		"status", status)
}

// newUpstreamError 统一记录安全字段并返回受控错误。
func newUpstreamError(lg *slog.Logger, provider Provider, stage string, category UpstreamErrorCategory, status int) *UpstreamError {
	logUpstreamFailure(lg, provider, stage, category, status)
	return &UpstreamError{Category: category}
}

// errUpstreamBodyTooLarge 是内部限读哨兵，不会返回给调用方。
var errUpstreamBodyTooLarge = errors.New("上游响应体超过上限")

// readUpstreamBody 最多读取 maxUpstreamResponseBytes+1 字节；
// 超出上限时返回内部哨兵，由调用方转换为受控类别。
func readUpstreamBody(body io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxUpstreamResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxUpstreamResponseBytes {
		return nil, errUpstreamBodyTooLarge
	}
	return data, nil
}

// decimalNumberPattern 匹配十进制数值字符串；拒绝单位、千分位、下划线、
// 十六进制和 NaN/Inf 等非十进制形式。
var decimalNumberPattern = regexp.MustCompile(`^[+-]?(?:\d+(?:\.\d+)?|\.\d+)(?:[eE][+-]?\d+)?$`)

// decimalNumber 以规范化科学计数法保存十进制值。digits 为空表示零。
type decimalNumber struct {
	digits   string
	exponent int
	negative bool
}

// parseDecimalNumber 规范化 JSON 数值或十进制字符串，保留可用于精确比较的位数。
func parseDecimalNumber(raw json.RawMessage) (decimalNumber, bool) {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return decimalNumber{}, false
	}
	if text[0] == '"' {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return decimalNumber{}, false
		}
		text = value
	}
	if !decimalNumberPattern.MatchString(text) {
		return decimalNumber{}, false
	}
	negative := text[0] == '-'
	if negative || text[0] == '+' {
		text = text[1:]
	}
	mantissa, exponentText := text, "0"
	if i := strings.IndexAny(text, "eE"); i >= 0 {
		mantissa, exponentText = text[:i], text[i+1:]
	}
	whole, fraction, _ := strings.Cut(mantissa, ".")
	digits := strings.TrimLeft(whole+fraction, "0")
	if digits == "" {
		return decimalNumber{}, true
	}
	exponent, err := strconv.Atoi(exponentText)
	offset := len(digits) - len(fraction) - 1
	// 先按 float64 可用的十进制量级检查，再调整指数以避免整数溢出。
	if err != nil || exponent < -324-offset || exponent > 308-offset {
		return decimalNumber{}, false
	}
	return decimalNumber{digits: strings.TrimRight(digits, "0"), exponent: exponent + offset, negative: negative}, true
}

// equalsPositiveInteger 精确比较正整数。digits 为不含符号和前导零的十进制整数。
func (n decimalNumber) equalsPositiveInteger(digits string) bool {
	return !n.negative && n.exponent == len(digits)-1 && n.digits == strings.TrimRight(digits, "0")
}

// parseFiniteNumber 转换流量字段。非零值下溢和溢出均视为非法值。
func parseFiniteNumber(raw json.RawMessage) (float64, bool) {
	number, ok := parseDecimalNumber(raw)
	if !ok || number.digits == "" {
		return 0, ok
	}
	mantissa := number.digits
	if len(mantissa) > 1 {
		mantissa = mantissa[:1] + "." + mantissa[1:]
	}
	if number.negative {
		mantissa = "-" + mantissa
	}
	value, err := strconv.ParseFloat(mantissa+"e"+strconv.Itoa(number.exponent), 64)
	if err != nil || value == 0 {
		return 0, false
	}
	return value, true
}

// decodeJSONObject 解析顶层 JSON 对象；非对象返回 false。
// JSON null 解码为 nil map 且不报错，由调用方按字段缺失处理。
func decodeJSONObject(body []byte) (map[string]json.RawMessage, bool) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, false
	}
	return root, true
}

// decodeVPSErrorValue 解析 V.P.S 的 error 字段，支持字符串或字符串数组。
// 空值视为没有业务错误；形状无法识别时返回 false。
func decodeVPSErrorValue(raw json.RawMessage) ([]string, bool) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, true
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		if strings.TrimSpace(single) == "" {
			return nil, true
		}
		return []string{single}, true
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		codes := make([]string, 0, len(list))
		for _, code := range list {
			if strings.TrimSpace(code) != "" {
				codes = append(codes, code)
			}
		}
		return codes, true
	}
	return nil, false
}

// vpsErrorCodesFromObject 读取顶层 error 字段。
// present 表示存在非空业务错误；ok 表示字段形状可识别。
func vpsErrorCodesFromObject(root map[string]json.RawMessage) (codes []string, present, ok bool) {
	if root == nil {
		return nil, false, true
	}
	raw, exists := root["error"]
	if !exists {
		return nil, false, true
	}
	decoded, valid := decodeVPSErrorValue(raw)
	if !valid {
		return nil, true, false
	}
	if len(decoded) == 0 {
		return nil, false, true
	}
	return decoded, true, true
}

// jsonStringField 读取 JSON 对象中的字符串字段；缺失或非字符串返回 false。
// JSON null 会解码为空字符串，调用方还必须检查去空白后是否为空。
func jsonStringField(root map[string]json.RawMessage, key string) (string, bool) {
	if root == nil {
		return "", false
	}
	raw, exists := root[key]
	if !exists {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

// vpsAuthErrorCategory 将明确的 V.P.S 凭据错误码映射为受控类别。
func vpsAuthErrorCategory(codes []string, accessToken bool) (UpstreamErrorCategory, bool) {
	for _, code := range codes {
		switch strings.ToLower(strings.TrimSpace(code)) {
		case "invalid_token", "unauthorized":
			if accessToken {
				return UpstreamAccessTokenRejected, true
			}
			return UpstreamCredentialRejected, true
		case "refresh_token_invalid":
			if !accessToken {
				return UpstreamCredentialRejected, true
			}
		}
	}
	return "", false
}

// kiwiVMHTTPFailureCategory 归类 KiwiVM 非 200 HTTP 响应；
// 业务失败由 HTTP 200 响应内的 error 字段判定。
func kiwiVMHTTPFailureCategory(resp *http.Response) UpstreamErrorCategory {
	if isUpstreamRedirect(resp) {
		return UpstreamUnavailable
	}
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return UpstreamRateLimited
	case http.StatusUnauthorized:
		return UpstreamCredentialRejected
	case http.StatusForbidden:
		return UpstreamUnavailable
	default:
		return UpstreamFailed
	}
}
