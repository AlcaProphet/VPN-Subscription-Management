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
	"vpn-sub/internal/config"
)

// MockLogin 模拟 OIDC 登录（仅 Dev 模式且 provider=mock）：
// subject 固定为输入邮箱，走与真实 OIDC 一致的查建/合并逻辑（可复现合并/冲突测试）。
// 返回结构：登录成功返回 User + 已签发凭据由接入层处理；pending/冲突返回 ResolveResult。
func (s *Service) MockLogin(ctx context.Context, email, username string, emailVerified bool, roles, groups []string) (*ResolveResult, error) {
	if s.mode != "dev" {
		return nil, fmt.Errorf("%w: 模拟登录仅 Dev 模式可用", config.ErrMockModeRestricted)
	}
	providerType := s.cfg.GetOr(ctx, KeyProviderType)
	if providerType != "mock" {
		return nil, errors.New("当前提供商不是模拟 OIDC")
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

// TestConnection 使用显式表单参数验证连接，不回退库内已保存 Secret（Setup/草稿测试路径）。
func (s *Service) TestConnection(ctx context.Context, providerType string, p Params) (*TestResult, error) {
	return s.testConnection(ctx, providerType, p, false)
}

// TestConnectionWithSavedSecret 管理员面板测试：Secret 留空时，仅当请求的 base_url/realm/client_id
// 与库内已保存配置完全一致，才回退已存明文，避免把已存 Secret 发往调用者指定的其他地址。
func (s *Service) TestConnectionWithSavedSecret(ctx context.Context, providerType string, p Params) (*TestResult, error) {
	return s.testConnection(ctx, providerType, p, true)
}

// testConnection 统一实现；allowSavedSecret 控制空 Secret 是否可回退已保存明文。
// K1：管理端测试连接在真实提供商路径先确认 signing_key 可用，故障时任何输入都在网络前阻断。
// TC-A：Secret 留空且目标已存 JSON/Secret 损坏时直接返回专门失败，不继续以“未提供 Secret”警告代替。
func (s *Service) testConnection(ctx context.Context, providerType string, p Params, allowSavedSecret bool) (*TestResult, error) {
	if providerType == "mock" {
		if s.mode != "dev" {
			return &TestResult{OK: false, Message: "生产模式不支持模拟 OIDC 测试连接，请切换到真实提供商"}, nil
		}
		return &TestResult{OK: true, Message: "模拟模式始终通过"}, nil
	}
	if p.ClientSecret == config.MaskedSecret {
		return &TestResult{OK: false, Message: "Client Secret 不能使用脱敏占位符，请重新输入"}, nil
	}
	if allowSavedSecret {
		// 管理员面板路径：K1 阻断 signing_key 缺失/读取失败；Setup 的显式参数测试不受影响。
		if _, err := s.cfg.GetSigningKey(ctx); err != nil {
			return &TestResult{OK: false, Message: config.OidcTestSigningKeyFaultMessage}, nil
		}
	}
	// ① 留空 Secret 时先检查已存状态：损坏直接专门失败；仅完全一致且状态可用才回退明文。
	if p.ClientSecret == "" && allowSavedSecret {
		insp, err := s.inspectParams(ctx, providerType)
		if err != nil {
			if errors.Is(err, config.ErrSigningKeyUnavailable) {
				return &TestResult{OK: false, Message: config.OidcTestSigningKeyFaultMessage}, nil
			}
			return &TestResult{OK: false, Message: "读取已存 OIDC 配置失败，请稍后重试"}, nil
		}
		if insp != nil {
			switch config.EffectiveOidcParamsState(insp.State) {
			case config.OidcParamsJSONDamaged, config.OidcParamsSecretDamaged, config.OidcParamsSigningKeyFault:
				return &TestResult{OK: false, Message: config.OidcTestStoredDamagedMessage}, nil
			case config.OidcParamsUsable:
				if insp.Raw != nil && insp.Plain != nil &&
					insp.Raw.BaseURL == p.BaseURL && insp.Raw.Realm == p.Realm && insp.Raw.ClientID == p.ClientID {
					p.ClientSecret = insp.Plain.ClientSecret
				}
			}
		}
	}
	// ② 发现文档可达性 + 配置完整性（base_url/client_id）
	if p.BaseURL == "" || p.ClientID == "" {
		return &TestResult{OK: false, Message: "Base URL 与 Client ID 为必填项"}, nil
	}
	disc, err := s.fetchDiscoveryWithParams(ctx, providerType, &p)
	if err != nil {
		return &TestResult{OK: false, Message: "发现文档不可达：" + err.Error()}, nil
	}
	res := &TestResult{OK: true, Message: "配置有效"}
	// ③ client_credentials 换 token 验证 Client ID/Secret；不支持该授权类型时降级为警告不阻断
	if p.ClientSecret != "" {
		if err := s.verifyClientCredentials(ctx, disc.TokenEndpoint, &p); err != nil {
			if isGrantUnsupported(err) {
				res.Warnings = append(res.Warnings, "提供商不支持 client_credentials，未验证 Client Secret："+err.Error())
			} else {
				return &TestResult{OK: false, Message: "Client ID/Secret 验证失败：" + err.Error()}, nil
			}
		}
	} else {
		res.Warnings = append(res.Warnings, "未提供 Client Secret，未执行凭据校验")
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
	if err := validateOIDCURL(wellKnown); err != nil {
		return nil, fmt.Errorf("发现文档地址校验失败: %w", err)
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
	if err := validateDiscoveryEndpoints(&disc); err != nil {
		return nil, err
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
	if err := validateOIDCURL(tokenEndpoint); err != nil {
		return fmt.Errorf("Token 端点地址校验失败: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, stringsNewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.doCredentialRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	return fmt.Errorf("HTTP %d", resp.StatusCode)
}

// isGrantUnsupported 判定错误是否为「不支持该授权类型」
func isGrantUnsupported(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "unsupported_grant_type") ||
		strings.Contains(msg, "invalid_grant") ||
		strings.Contains(msg, "unauthorized_client") ||
		strings.Contains(msg, "405")
}
