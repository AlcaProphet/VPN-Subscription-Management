package providertraffic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

const (
	// vpsTokenURL 是 V.P.S 固定的续期地址。
	vpsTokenURL = "https://vps.hosting/api/token"
	// vpsResourcesURLPrefix 与 vpsResourcesURLSuffix 组成固定的资源查询地址。
	vpsResourcesURLPrefix = "https://vps.hosting/api/service/"
	vpsResourcesURLSuffix = "/resources"
)

// VPSClient 是固定 V.P.S 续期与资源查询协议的上游客户端。
// 不缓存 Token、不写数据库；轮换落库与采集编排由后续采集层负责。
type VPSClient struct {
	httpClient *http.Client
	log        *slog.Logger
}

// NewVPSClient 构造 V.P.S 客户端；不读写数据库，也不缓存 Token。
func NewVPSClient(lg *slog.Logger) *VPSClient {
	return &VPSClient{httpClient: newUpstreamHTTPClient(), log: lg}
}

// TokenPair 是 V.P.S 续期返回的新 Access Token 与 Refresh Token。
// 客户端不保存、不记录它们；调用方须先按绑定修订保存 Refresh Token 再使用。
type TokenPair struct {
	Token   string
	Refresh string
}

// RefreshToken 使用已保存的 Refresh Token 换取一对新 Token。
// 请求体只含 refresh_token；成功响应必须同时包含非空 token 与 refresh。
func (c *VPSClient) RefreshToken(ctx context.Context, refreshToken string) (TokenPair, error) {
	payload, err := json.Marshal(struct {
		RefreshToken string `json:"refresh_token"`
	}{RefreshToken: refreshToken})
	if err != nil {
		return TokenPair{}, newUpstreamError(c.log, ProviderVPS, stageVPSToken, UpstreamFailed, 0)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, vpsTokenURL, bytes.NewReader(payload))
	if err != nil {
		return TokenPair{}, newUpstreamError(c.log, ProviderVPS, stageVPSToken, UpstreamFailed, 0)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return TokenPair{}, newUpstreamError(c.log, ProviderVPS, stageVPSToken, UpstreamUnavailable, 0)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		category := vpsHTTPFailureCategory(resp, false)
		return TokenPair{}, newUpstreamError(c.log, ProviderVPS, stageVPSToken, category, resp.StatusCode)
	}

	body, err := readUpstreamBody(resp.Body)
	if err != nil {
		if errors.Is(err, errUpstreamBodyTooLarge) {
			return TokenPair{}, newUpstreamError(c.log, ProviderVPS, stageVPSToken, UpstreamFailed, resp.StatusCode)
		}
		return TokenPair{}, newUpstreamError(c.log, ProviderVPS, stageVPSToken, UpstreamUnavailable, resp.StatusCode)
	}

	root, ok := decodeJSONObject(body)
	if !ok {
		return TokenPair{}, newUpstreamError(c.log, ProviderVPS, stageVPSToken, UpstreamFailed, resp.StatusCode)
	}
	codes, present, ok := vpsErrorCodesFromObject(root)
	if !ok {
		return TokenPair{}, newUpstreamError(c.log, ProviderVPS, stageVPSToken, UpstreamFailed, resp.StatusCode)
	}
	if present {
		if category, auth := vpsAuthErrorCategory(codes, false); auth {
			return TokenPair{}, newUpstreamError(c.log, ProviderVPS, stageVPSToken, category, resp.StatusCode)
		}
		return TokenPair{}, newUpstreamError(c.log, ProviderVPS, stageVPSToken, UpstreamFailed, resp.StatusCode)
	}

	token, ok := jsonStringField(root, "token")
	if !ok || strings.TrimSpace(token) == "" {
		return TokenPair{}, newUpstreamError(c.log, ProviderVPS, stageVPSToken, UpstreamFailed, resp.StatusCode)
	}
	refresh, ok := jsonStringField(root, "refresh")
	if !ok || strings.TrimSpace(refresh) == "" {
		return TokenPair{}, newUpstreamError(c.log, ProviderVPS, stageVPSToken, UpstreamFailed, resp.StatusCode)
	}
	return TokenPair{Token: token, Refresh: refresh}, nil
}

// FetchRemaining 使用 Access Token 查询固定资源地址并按 GB 返回余量。
// 只读取 limit.data_combined；不推算字节、重置日期或其他资源字段。
func (c *VPSClient) FetchRemaining(ctx context.Context, instanceID, accessToken string) (TrafficSample, error) {
	if !validInstanceID(instanceID) {
		return TrafficSample{}, ErrInvalidInstanceID
	}
	endpoint := vpsResourcesURLPrefix + instanceID + vpsResourcesURLSuffix
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return TrafficSample{}, newUpstreamError(c.log, ProviderVPS, stageVPSResources, UpstreamFailed, 0)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return TrafficSample{}, newUpstreamError(c.log, ProviderVPS, stageVPSResources, UpstreamUnavailable, 0)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		category := vpsHTTPFailureCategory(resp, true)
		return TrafficSample{}, newUpstreamError(c.log, ProviderVPS, stageVPSResources, category, resp.StatusCode)
	}

	body, err := readUpstreamBody(resp.Body)
	if err != nil {
		if errors.Is(err, errUpstreamBodyTooLarge) {
			return TrafficSample{}, newUpstreamError(c.log, ProviderVPS, stageVPSResources, UpstreamInvalidData, resp.StatusCode)
		}
		return TrafficSample{}, newUpstreamError(c.log, ProviderVPS, stageVPSResources, UpstreamUnavailable, resp.StatusCode)
	}

	root, ok := decodeJSONObject(body)
	if !ok {
		return TrafficSample{}, newUpstreamError(c.log, ProviderVPS, stageVPSResources, UpstreamInvalidData, resp.StatusCode)
	}
	codes, present, ok := vpsErrorCodesFromObject(root)
	if !ok {
		return TrafficSample{}, newUpstreamError(c.log, ProviderVPS, stageVPSResources, UpstreamFailed, resp.StatusCode)
	}
	if present {
		if category, auth := vpsAuthErrorCategory(codes, true); auth {
			return TrafficSample{}, newUpstreamError(c.log, ProviderVPS, stageVPSResources, category, resp.StatusCode)
		}
		return TrafficSample{}, newUpstreamError(c.log, ProviderVPS, stageVPSResources, UpstreamFailed, resp.StatusCode)
	}

	limitRaw, ok := root["limit"]
	if !ok {
		return TrafficSample{}, newUpstreamError(c.log, ProviderVPS, stageVPSResources, UpstreamInvalidData, resp.StatusCode)
	}
	var limit struct {
		DataCombined json.RawMessage `json:"data_combined"`
	}
	if err := json.Unmarshal(limitRaw, &limit); err != nil {
		return TrafficSample{}, newUpstreamError(c.log, ProviderVPS, stageVPSResources, UpstreamInvalidData, resp.StatusCode)
	}
	remaining, ok := parseFiniteNumber(limit.DataCombined)
	if !ok || remaining < 0 {
		return TrafficSample{}, newUpstreamError(c.log, ProviderVPS, stageVPSResources, UpstreamInvalidData, resp.StatusCode)
	}
	return TrafficSample{Remaining: remaining, Unit: UnitGB}, nil
}

// vpsHTTPFailureCategory 优先处理可由状态确定的错误，再限读其余 4xx 正文以识别凭据失效。
func vpsHTTPFailureCategory(resp *http.Response, accessToken bool) UpstreamErrorCategory {
	status := resp.StatusCode
	switch {
	case status == http.StatusTooManyRequests:
		return UpstreamRateLimited
	case status == http.StatusUnauthorized:
		if accessToken {
			return UpstreamAccessTokenRejected
		}
		return UpstreamCredentialRejected
	case isUpstreamRedirect(resp):
		return UpstreamUnavailable
	case status >= 400 && status < 500:
		body, err := readUpstreamBody(resp.Body)
		if err != nil && !errors.Is(err, errUpstreamBodyTooLarge) {
			return UpstreamUnavailable
		}
		if root, ok := decodeJSONObject(body); ok {
			if codes, present, valid := vpsErrorCodesFromObject(root); present && valid {
				if category, auth := vpsAuthErrorCategory(codes, accessToken); auth {
					return category
				}
			}
		}
	}
	if status == http.StatusForbidden {
		return UpstreamUnavailable
	}
	return UpstreamFailed
}
