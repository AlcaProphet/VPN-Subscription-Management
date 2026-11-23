package providertraffic

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

const (
	// kiwiVMServiceInfoURL 是 KiwiVM 固定的服务信息地址。
	kiwiVMServiceInfoURL = "https://api.64clouds.com/v1/getServiceInfo"
	// kiwiVMAuthFailureCode 是 KiwiVM 明确拒绝 VEID/API Key 的业务错误码。
	kiwiVMAuthFailureCode = "700005"
	// bytesPerGiB 将字节换算为 GiB。
	bytesPerGiB = 1 << 30
)

// KiwiVMClient 是固定 KiwiVM getServiceInfo 协议的上游客户端。
type KiwiVMClient struct {
	httpClient *http.Client
	log        *slog.Logger
}

// NewKiwiVMClient 构造 KiwiVM 客户端；不读写数据库，也不缓存结果。
func NewKiwiVMClient(lg *slog.Logger) *KiwiVMClient {
	return &KiwiVMClient{httpClient: newUpstreamHTTPClient(), log: lg}
}

// kiwiVMServiceInfoResponse 只读取计费相关字段，其余字段忽略。
type kiwiVMServiceInfoResponse struct {
	Error                 json.RawMessage `json:"error"`
	PlanMonthlyData       json.RawMessage `json:"plan_monthly_data"`
	DataCounter           json.RawMessage `json:"data_counter"`
	MonthlyDataMultiplier json.RawMessage `json:"monthly_data_multiplier"`
}

// FetchRemaining 调用 getServiceInfo 并按冻结合同返回剩余流量。
// 仅当 HTTP 200、业务 error=0 且计费倍率为 1 时计算；倍率异常单独提示待核对。
func (c *KiwiVMClient) FetchRemaining(ctx context.Context, instanceID, apiKey string) (TrafficSample, error) {
	if !validInstanceID(instanceID) {
		return TrafficSample{}, ErrInvalidInstanceID
	}
	form := url.Values{"veid": {instanceID}, "api_key": {apiKey}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, kiwiVMServiceInfoURL, strings.NewReader(form.Encode()))
	if err != nil {
		return TrafficSample{}, newUpstreamError(c.log, ProviderKiwiVM, stageKiwiVMServiceInfo, UpstreamFailed, 0)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return TrafficSample{}, newUpstreamError(c.log, ProviderKiwiVM, stageKiwiVMServiceInfo, UpstreamUnavailable, 0)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return TrafficSample{}, newUpstreamError(c.log, ProviderKiwiVM, stageKiwiVMServiceInfo,
			kiwiVMHTTPFailureCategory(resp), resp.StatusCode)
	}

	body, err := readUpstreamBody(resp.Body)
	if err != nil {
		if errors.Is(err, errUpstreamBodyTooLarge) {
			return TrafficSample{}, newUpstreamError(c.log, ProviderKiwiVM, stageKiwiVMServiceInfo, UpstreamInvalidData, resp.StatusCode)
		}
		return TrafficSample{}, newUpstreamError(c.log, ProviderKiwiVM, stageKiwiVMServiceInfo, UpstreamUnavailable, resp.StatusCode)
	}

	var payload kiwiVMServiceInfoResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return TrafficSample{}, newUpstreamError(c.log, ProviderKiwiVM, stageKiwiVMServiceInfo, UpstreamInvalidData, resp.StatusCode)
	}

	errorCode, ok := parseDecimalNumber(payload.Error)
	if !ok {
		return TrafficSample{}, newUpstreamError(c.log, ProviderKiwiVM, stageKiwiVMServiceInfo, UpstreamFailed, resp.StatusCode)
	}
	if errorCode.digits != "" {
		category := UpstreamFailed
		if errorCode.equalsPositiveInteger(kiwiVMAuthFailureCode) {
			category = UpstreamCredentialRejected
		}
		return TrafficSample{}, newUpstreamError(c.log, ProviderKiwiVM, stageKiwiVMServiceInfo, category, resp.StatusCode)
	}

	multiplier, ok := parseDecimalNumber(payload.MonthlyDataMultiplier)
	if !ok || !multiplier.equalsPositiveInteger("1") {
		return TrafficSample{}, newUpstreamError(c.log, ProviderKiwiVM, stageKiwiVMServiceInfo, UpstreamMultiplierCheck, resp.StatusCode)
	}

	plan, ok := parseFiniteNumber(payload.PlanMonthlyData)
	if !ok || plan <= 0 {
		return TrafficSample{}, newUpstreamError(c.log, ProviderKiwiVM, stageKiwiVMServiceInfo, UpstreamInvalidData, resp.StatusCode)
	}
	used, ok := parseFiniteNumber(payload.DataCounter)
	if !ok || used < 0 {
		return TrafficSample{}, newUpstreamError(c.log, ProviderKiwiVM, stageKiwiVMServiceInfo, UpstreamInvalidData, resp.StatusCode)
	}
	remaining := plan - used
	if remaining < 0 {
		remaining = 0
	}
	return TrafficSample{Remaining: remaining / bytesPerGiB, Unit: UnitGiB}, nil
}
