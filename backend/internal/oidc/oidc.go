// Package oidc 提供 OIDC 认证业务层：配置管理、PKCE 授权流、用户查建/合并、绑定、模拟 OIDC 与测试连接。
package oidc

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/config"
	"vpn-sub/internal/store"
	"vpn-sub/internal/user"
)

// --- 配置管理（Design1 §3.1/5.3）：OIDC 参数存 system_config；各提供商参数独立存储（切换类型保留已填字段）---

const (
	KeyProviderType = "oidc_provider_type" // keycloak/auth0/generic/mock
	KeyConfigured   = "oidc_configured"
	// 各提供商参数以 JSON 存于独立键（敏感字段在 JSON 内单独加密）：
	//   oidc_params_keycloak / oidc_params_auth0 / oidc_params_generic / oidc_params_mock
	// 结构：{ base_url, realm, client_id, client_secret(密文) }
	KeyOidcApproval = "oidc_approval" // OIDC 新用户审批开关（默认关闭，Build3 面板接通）
	KeyWhitelist    = "oidc_whitelist" // OIDC 白名单 JSON（Build3 面板接通）：{role_claim_path, role_values, group_claim_path, group_values}
)

const stateTTL = 10 * time.Minute // OIDC state TTL（关键设计参数，Design1 §3.2）

// Params OIDC 提供商参数
type Params struct {
	BaseURL      string `json:"base_url"`
	Realm        string `json:"realm"` // keycloak 专用
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"` // 落库前经 config.Encrypt 加密
}

// Discovery OIDC 发现文档（最小字段集）
type Discovery struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
	Issuer                string `json:"issuer"`
}

// Service OIDC 服务
type Service struct {
	store   *store.Store
	cfg     *config.Service
	authSvc *auth.Service
	users   *user.Service
	mode    string // APP_MODE：模拟 OIDC 仅 dev 可用
	log     *slog.Logger
	httpCli *http.Client

	mu        sync.Mutex
	discCache map[string]*Discovery // 发现文档缓存（key = base_url）
}

func NewService(st *store.Store, cfg *config.Service, authSvc *auth.Service, users *user.Service, mode string, lg *slog.Logger) *Service {
	return &Service{
		store: st, cfg: cfg, authSvc: authSvc, users: users, mode: mode, log: lg,
		httpCli:   &http.Client{Timeout: 10 * time.Second},
		discCache: map[string]*Discovery{},
	}
}

// IsConfigured OIDC 是否已配置
func (s *Service) IsConfigured(ctx context.Context) bool {
	return s.cfg.GetBool(ctx, KeyConfigured, false)
}

// currentParams 读取当前提供商参数
func (s *Service) currentParams(ctx context.Context) (*Params, error) {
	providerType, err := s.cfg.Get(ctx, KeyProviderType)
	if err != nil {
		return nil, err
	}
	if providerType == "" {
		return nil, errors.New("OIDC 未配置")
	}
	return s.loadParams(ctx, providerType)
}

// LoadParams 读取指定提供商参数（client_secret 自动解密；供面板配置回显/可用性判定，Build3 Step 3）
func (s *Service) LoadParams(ctx context.Context, providerType string) (*Params, error) {
	return s.loadParams(ctx, providerType)
}

// loadParams 读取指定提供商参数（client_secret 自动解密）
func (s *Service) loadParams(ctx context.Context, providerType string) (*Params, error) {
	raw, err := s.cfg.Get(ctx, "oidc_params_"+providerType)
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, errors.New("OIDC 参数未配置")
	}
	var p Params
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil, fmt.Errorf("解析 OIDC 参数失败: %w", err)
	}
	return &p, nil
}

// SaveParams 保存提供商参数（client_secret 加密落库；空值保留原密文）
func (s *Service) SaveParams(ctx context.Context, providerType string, p Params) error {
	secretCipher := ""
	if p.ClientSecret != "" {
		enc, err := s.cfg.EncryptSensitive(ctx, p.ClientSecret)
		if err != nil {
			return err
		}
		secretCipher = enc
	} else {
		// 未提供新 Secret：保留库内既有密文（面板回显脱敏场景）
		if existing, err := s.loadParams(ctx, providerType); err == nil {
			secretCipher = existing.ClientSecret
		}
	}
	params := Params{BaseURL: p.BaseURL, Realm: p.Realm, ClientID: p.ClientID, ClientSecret: secretCipher}
	raw, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("序列化 OIDC 参数失败: %w", err)
	}
	return s.cfg.Set(ctx, "oidc_params_"+providerType, string(raw))
}

// EncryptWithTx 事务内加密（Setup/OIDC Setup 事务内使用：同一事务读签名密钥）
func (s *Service) EncryptWithTx(ctx context.Context, tx *sql.Tx, plain string) (string, error) {
	return s.cfg.EncryptWithTx(ctx, tx, plain)
}

// SaveParamsTx 事务内写入提供商参数 JSON（Setup OIDC 分支使用）
func (s *Service) SaveParamsTx(ctx context.Context, tx *sql.Tx, providerType, rawJSON string) error {
	return s.cfg.SetTx(ctx, tx, "oidc_params_"+providerType, rawJSON)
}

// SetProviderTx 事务内写入提供商类型
func (s *Service) SetProviderTx(ctx context.Context, tx *sql.Tx, providerType string) error {
	return s.cfg.SetTx(ctx, tx, KeyProviderType, providerType)
}

// CallbackURL 回调地址（frontend_url + /api/auth/oidc/callback）
func (s *Service) CallbackURL(ctx context.Context) string {
	furl, _ := s.cfg.Get(ctx, config.KeyFrontendURL)
	return furl + "/api/auth/oidc/callback"
}

// --- 发现文档获取（带缓存）---

// fetchDiscovery 获取发现文档（带缓存，缓存键 = base_url + realm）
func (s *Service) fetchDiscovery(ctx context.Context, p *Params) (*Discovery, error) {
	// 模拟模式：不依赖真实提供商
	if providerType, _ := s.cfg.Get(ctx, KeyProviderType); providerType == "mock" {
		return &Discovery{AuthorizationEndpoint: "mock://authorize", TokenEndpoint: "mock://token"}, nil
	}
	base := strings.TrimSuffix(p.BaseURL, "/")
	wellKnown := base + "/.well-known/openid-configuration"
	if p.Realm != "" {
		wellKnown = base + "/realms/" + p.Realm + "/.well-known/openid-configuration"
	}
	cacheKey := wellKnown
	s.mu.Lock()
	if d, ok := s.discCache[cacheKey]; ok {
		s.mu.Unlock()
		return d, nil
	}
	s.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnown, nil)
	if err != nil {
		return nil, fmt.Errorf("构造发现文档请求失败: %w", err)
	}
	resp, err := s.httpCli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发现文档不可达: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("发现文档返回 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取发现文档失败: %w", err)
	}
	var disc Discovery
	if err := json.Unmarshal(body, &disc); err != nil {
		return nil, fmt.Errorf("解析发现文档失败: %w", err)
	}
	if disc.AuthorizationEndpoint == "" || disc.TokenEndpoint == "" {
		return nil, errors.New("发现文档缺少必要端点")
	}
	s.mu.Lock()
	s.discCache[cacheKey] = &disc
	s.mu.Unlock()
	return &disc, nil
}

// ClearDiscCache 清空发现文档缓存（配置变更后调用）
func (s *Service) ClearDiscCache() {
	s.mu.Lock()
	s.discCache = map[string]*Discovery{}
	s.mu.Unlock()
}

// matchWhitelist 白名单匹配（Build3 Step 3 接通配置）：
// 读取 oidc_whitelist（JSON：{role_claim_path, role_values, group_claim_path, group_values}）；
// 未配置/解析失败/白名单为空 → 跳过校验直接激活（Design1 §2.6）；Role 或 Group 任一命中 → 激活
func (s *Service) matchWhitelist(ctx context.Context, id *Identity) bool {
	raw, err := s.cfg.Get(ctx, KeyWhitelist)
	if err != nil || raw == "" {
		return true
	}
	var wl config.WhitelistConfig
	if err := json.Unmarshal([]byte(raw), &wl); err != nil {
		s.log.Warn("解析 OIDC 白名单配置失败，按空白名单处理", "err", err)
		return true
	}
	roleHit := false
	if wl.RoleValues != nil && len(wl.RoleValues) > 0 && wl.RoleClaimPath != "" {
		vals, ok := claimPathValues(id.RawClaims, wl.RoleClaimPath)
		roleHit = ok && intersectAny(vals, wl.RoleValues)
	}
	groupHit := false
	if wl.GroupValues != nil && len(wl.GroupValues) > 0 && wl.GroupClaimPath != "" {
		vals, ok := claimPathValues(id.RawClaims, wl.GroupClaimPath)
		groupHit = ok && intersectAny(vals, wl.GroupValues)
	}
	return roleHit || groupHit // 任一命中即激活
}
