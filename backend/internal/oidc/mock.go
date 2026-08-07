package oidc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"vpn-sub/internal/auth"
)

// MockLogin 模拟 OIDC 登录（仅 Dev 模式且 provider=mock）：
// subject 固定为输入邮箱，走与真实 OIDC 一致的查建/合并逻辑（可复现合并/冲突测试）。
// 返回结构：登录成功返回 User + 已签发凭据由接入层处理；pending/冲突返回 ResolveResult。
func (s *Service) MockLogin(ctx context.Context, email, username string, emailVerified bool, roles, groups []string) (*ResolveResult, error) {
	providerType, _ := s.cfg.Get(ctx, KeyProviderType)
	if s.mode != "dev" || providerType != "mock" {
		return nil, errors.New("模拟登录仅 Dev 模式且选择模拟 OIDC 时可用")
	}
	normalized, err := auth.NormalizeEmail(email)
	if err != nil {
		return nil, err
	}
	if username == "" {
		username = strings.SplitN(normalized, "@", 2)[0] // 留空取邮箱 @ 前缀
	}
	id := &Identity{
		Subject:       normalized,
		Email:         normalized,
		EmailVerified: emailVerified,
		Username:      username,
		RoleClaims:    roles,
		GroupClaims:   groups,
	}
	raw, err := json.Marshal(struct {
		Sub           string   `json:"sub"`
		Email         string   `json:"email"`
		EmailVerified bool     `json:"email_verified"`
		Username      string   `json:"username"`
		Roles         []string `json:"roles"`
		Groups        []string `json:"groups"`
	}{normalized, normalized, emailVerified, username, roles, groups})
	if err != nil {
		return nil, err
	}
	id.RawClaims = string(raw)
	return s.ResolveLogin(ctx, id)
}

// MockCode 生成模拟授权 code（携带身份信息，供 mockExchange 还原）
func (s *Service) MockCode(email, username string, emailVerified bool, roles, groups []string) (string, error) {
	payload := struct {
		Sub           string   `json:"sub"`
		Email         string   `json:"email"`
		EmailVerified bool     `json:"email_verified"`
		Username      string   `json:"username"`
		Roles         []string `json:"roles"`
		Groups        []string `json:"groups"`
	}{email, email, emailVerified, username, roles, groups}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// --- 测试连接（Design1 §3.1）---

// TestResult 测试连接结果
type TestResult struct {
	OK       bool     `json:"ok"`
	Message  string   `json:"message"`
	Warnings []string `json:"warnings"`
}

// TestConnection 验证发现文档可达性与配置完整性；以 client_credentials 换 token 验证 Client ID/Secret；
// 不支持该授权类型时降级为警告不阻断；模拟模式始终通过
func (s *Service) TestConnection(ctx context.Context, providerType string, p Params) (*TestResult, error) {
	if providerType == "mock" {
		return &TestResult{OK: true, Message: "模拟模式始终通过"}, nil
	}
	// ① 发现文档可达性 + 配置完整性（base_url/client_id/回调地址）
	if p.BaseURL == "" || p.ClientID == "" {
		return &TestResult{OK: false, Message: "Base URL 与 Client ID 为必填项"}, nil
	}
	disc, err := s.fetchDiscoveryWithParams(ctx, providerType, &p)
	if err != nil {
		return &TestResult{OK: false, Message: "发现文档不可达：" + err.Error()}, nil
	}
	res := &TestResult{OK: true, Message: "配置有效"}
	// ② client_credentials 换 token 验证 Client ID/Secret；不支持该授权类型时降级为警告不阻断
	if p.ClientSecret != "" {
		if err := s.verifyClientCredentials(ctx, disc.TokenEndpoint, &p); err != nil {
			if isGrantUnsupported(err) {
				res.Warnings = append(res.Warnings, "提供商不支持 client_credentials，未验证 Client Secret："+err.Error())
			} else {
				return &TestResult{OK: false, Message: "Client ID/Secret 验证失败：" + err.Error()}, nil
			}
		}
	}
	return res, nil
}

// fetchDiscoveryWithParams 按显式参数获取发现文档（测试连接用，不依赖已保存配置）
func (s *Service) fetchDiscoveryWithParams(ctx context.Context, providerType string, p *Params) (*Discovery, error) {
	base := strings.TrimSuffix(p.BaseURL, "/")
	wellKnown := base + "/.well-known/openid-configuration"
	if p.Realm != "" {
		wellKnown = base + "/realms/" + p.Realm + "/.well-known/openid-configuration"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnown, nil)
	if err != nil {
		return nil, fmt.Errorf("构造发现文档请求失败: %w", err)
	}
	resp, err := s.httpCli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("发现文档返回 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var disc Discovery
	if err := json.Unmarshal(body, &disc); err != nil {
		return nil, fmt.Errorf("解析发现文档失败: %w", err)
	}
	if disc.AuthorizationEndpoint == "" || disc.TokenEndpoint == "" {
		return nil, errors.New("发现文档缺少必要端点")
	}
	return &disc, nil
}

// verifyClientCredentials 以 client_credentials 换 token 验证 Client ID/Secret
func (s *Service) verifyClientCredentials(ctx context.Context, tokenEndpoint string, p *Params) error {
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {p.ClientID},
		"client_secret": {p.ClientSecret},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, stringsNewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.httpCli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

// isGrantUnsupported 判定错误是否为「不支持该授权类型」
func isGrantUnsupported(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "unsupported_grant_type") ||
		strings.Contains(msg, "invalid_grant") ||
		strings.Contains(msg, "unauthorized_client") ||
		strings.Contains(msg, "405")
}
