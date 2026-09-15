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
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/config"
	"vpn-sub/internal/store"
	"vpn-sub/internal/urlguard"
	"vpn-sub/internal/user"
)

// --- 配置管理（Design1 §3.1/5.3）：OIDC 参数存 system_config；各提供商参数独立存储（切换类型保留已填字段）---

const (
	KeyProviderType = "oidc_provider_type" // keycloak/auth0/generic/mock
	KeyConfigured   = "oidc_configured"
	// 各提供商参数以 JSON 存于独立键（敏感字段在 JSON 内单独加密）：
	//   oidc_params_keycloak / oidc_params_auth0 / oidc_params_generic / oidc_params_mock
	// 结构：{ base_url, realm, client_id, client_secret(密文) }
	KeyOidcApproval = "oidc_approval"  // OIDC 新用户审批开关（默认关闭，Build3 面板接通）
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

	jwksMu    sync.Mutex
	jwksCache map[string]*jwkSet // JWKS 缓存（key = jwks_uri）
}

func NewService(st *store.Store, cfg *config.Service, authSvc *auth.Service, users *user.Service, mode string, lg *slog.Logger) *Service {
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	// 代理策略：D-F06-2 已确认保留 ProxyFromEnvironment；经代理时 DialContext 只能看到代理地址，
	// 目标 DNS/公网 IP 校验不生效，代理可信作为部署边界。需要直连校验时由部署层配置 NO_PROXY。
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			var dialIP string
			for _, ip := range ips {
				if isBlockedIP(ip.IP) {
					return nil, fmt.Errorf("禁止访问非公网地址: %s", ip.IP)
				}
				if dialIP == "" {
					dialIP = ip.IP.String()
				}
			}
			if dialIP == "" {
				return nil, errors.New("URL 主机无可用解析结果")
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(dialIP, port))
		},
		TLSHandshakeTimeout: 5 * time.Second,
	}
	return &Service{
		store: st, cfg: cfg, authSvc: authSvc, users: users, mode: mode, log: lg,
		httpCli: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return errors.New("重定向次数过多")
				}
				return validateOIDCURL(req.URL.String())
			},
		},
		discCache: map[string]*Discovery{},
		jwksCache: map[string]*jwkSet{},
	}
}

// IsConfigured OIDC 是否已配置
func (s *Service) IsConfigured(ctx context.Context) bool {
	return s.cfg.GetBool(ctx, KeyConfigured, false)
}

// currentParams 读取当前提供商参数；损坏的脱敏占位符不得进入授权/换 token 链路。
func (s *Service) currentParams(ctx context.Context) (*Params, error) {
	providerType, err := s.cfg.Get(ctx, KeyProviderType)
	if err != nil {
		return nil, err
	}
	if providerType == "" {
		return nil, errors.New("OIDC 未配置")
	}
	p, err := s.loadParams(ctx, providerType)
	if err != nil {
		return nil, err
	}
	if p.ClientSecret == config.MaskedSecret {
		return nil, errors.New("OIDC Client Secret 已被脱敏占位符覆盖，请管理员在设置页重新输入")
	}
	return p, nil
}

// LoadParams 读取指定提供商参数（client_secret 自动解密为明文；供面板回显/可用性判定/测试连接，Build3 Step 3）
func (s *Service) LoadParams(ctx context.Context, providerType string) (*Params, error) {
	return s.loadParams(ctx, providerType)
}

// loadRawParams 读取指定提供商参数（client_secret 保持库内密文，仅用于保存空输入时保留原值）
func (s *Service) loadRawParams(ctx context.Context, providerType string) (*Params, error) {
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

// loadParams 读取指定提供商参数（client_secret 自动解密为明文）
func (s *Service) loadParams(ctx context.Context, providerType string) (*Params, error) {
	p, err := s.loadRawParams(ctx, providerType)
	if err != nil {
		return nil, err
	}
	if p.ClientSecret != "" {
		plain, err := s.cfg.DecryptWithKey(ctx, p.ClientSecret)
		if err != nil {
			return nil, fmt.Errorf("解密 OIDC Client Secret 失败: %w", err)
		}
		p.ClientSecret = string(plain)
	}
	return p, nil
}

// DescribeParams 读取指定提供商参数并给出只读状态（不含 Secret 明文/密文）。
// Secret 留空视为尚未配置可用 Secret；可解析但解密失败/为脱敏占位符视为 Secret 损坏；
// JSON 非空但无法解析视为 JSON 损坏；签名密钥读取失败作为独立系统错误返回。
func (s *Service) DescribeParams(ctx context.Context, providerType string) (config.OidcParamsState, error) {
	raw, err := s.cfg.Get(ctx, "oidc_params_"+providerType)
	if err != nil {
		return config.OidcParamsState{}, fmt.Errorf("读取 OIDC 参数失败: %w", err)
	}
	if strings.TrimSpace(raw) == "" {
		return config.OidcParamsState{}, nil
	}
	var out config.OidcParamsState
	out.Present = true
	var p Params
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		out.JSONDamaged = true
		return out, nil
	}
	out.BaseURL, out.Realm, out.ClientID = p.BaseURL, p.Realm, p.ClientID
	if p.ClientSecret == "" {
		return out, nil
	}
	key, err := s.cfg.GetSigningKey(ctx)
	if err != nil {
		return config.OidcParamsState{}, fmt.Errorf("读取签名密钥失败: %w", err)
	}
	plain, err := config.Decrypt(p.ClientSecret, key)
	if err != nil {
		out.SecretDamaged = true
		return out, nil
	}
	switch string(plain) {
	case "":
		// 空密文按尚未配置可用 Secret 处理，不标损坏。
	case config.MaskedSecret:
		out.SecretDamaged = true
	default:
		out.SecretUsable = true
	}
	return out, nil
}

// validateOIDCBaseURL 校验真实提供商写入的 base_url；mock 与空值不在参数写入层拦截。
func validateOIDCBaseURL(providerType, baseURL string) error {
	if providerType == "mock" || baseURL == "" {
		return nil
	}
	if err := urlguard.ValidateHTTPS(baseURL); err != nil {
		return fmt.Errorf("%w: OIDC Base URL 必须是 HTTPS 地址: %v", config.ErrBadRequest, err)
	}
	return nil
}

// saveParamsKeepSecret 空 Secret 保存的原子守卫：字段组校验、旧 Secret 可用性校验与保留密文写回必须在同一写事务内完成。
func (s *Service) saveParamsKeepSecret(ctx context.Context, providerType string, p Params) error {
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		raw, err := s.cfg.GetTx(ctx, tx, "oidc_params_"+providerType)
		if err != nil {
			return fmt.Errorf("读取 OIDC 参数失败: %w", err)
		}
		if strings.TrimSpace(raw) == "" {
			return fmt.Errorf("%w: 目标提供商尚无可用 Client Secret，请输入新的 Client Secret", config.ErrBadRequest)
		}
		var existing Params
		if err := json.Unmarshal([]byte(raw), &existing); err != nil {
			return fmt.Errorf("%w: 目标提供商已存 OIDC 参数 JSON 损坏，请重新填写必要参数并输入新的 Client Secret", config.ErrBadRequest)
		}
		if existing.ClientSecret == "" {
			return fmt.Errorf("%w: 目标提供商尚无可用 Client Secret，请输入新的 Client Secret", config.ErrBadRequest)
		}
		key, err := s.cfg.GetSigningKeyTx(ctx, tx)
		if err != nil {
			return fmt.Errorf("读取签名密钥失败: %w", err)
		}
		plain, err := config.Decrypt(existing.ClientSecret, key)
		if err != nil {
			return fmt.Errorf("%w: 目标提供商已存 Client Secret 损坏，请输入新的 Client Secret", config.ErrBadRequest)
		}
		if string(plain) == "" || string(plain) == config.MaskedSecret {
			return fmt.Errorf("%w: 目标提供商已存 Client Secret 损坏或为空，请输入新的 Client Secret", config.ErrBadRequest)
		}
		if existing.BaseURL != p.BaseURL || existing.Realm != p.Realm || existing.ClientID != p.ClientID {
			return fmt.Errorf("%w: 目标提供商的 Base URL/Realm/Client ID 与已存配置不一致，不能留空复用旧 Client Secret，请重新输入", config.ErrBadRequest)
		}
		params := Params{BaseURL: p.BaseURL, Realm: p.Realm, ClientID: p.ClientID, ClientSecret: existing.ClientSecret}
		rawNew, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("序列化 OIDC 参数失败: %w", err)
		}
		return s.cfg.SetTx(ctx, tx, "oidc_params_"+providerType, string(rawNew))
	})
}

// SaveParams 保存提供商参数（入参 client_secret 为明文；空值保留库内原密文，显式新值才加密替换）。
// 真实提供商空 Secret 必须走 saveParamsKeepSecret 原子守卫；mock 无 Secret 语义，保持独立存储路径。
func (s *Service) SaveParams(ctx context.Context, providerType string, p Params) error {
	if err := validateOIDCBaseURL(providerType, p.BaseURL); err != nil {
		return err
	}
	if p.ClientSecret == config.MaskedSecret {
		return errors.New("Client Secret 不能使用脱敏占位符")
	}
	if providerType != "mock" && p.ClientSecret == "" {
		return s.saveParamsKeepSecret(ctx, providerType, p)
	}
	secretCipher := ""
	if p.ClientSecret != "" {
		enc, err := s.cfg.EncryptSensitive(ctx, p.ClientSecret)
		if err != nil {
			return err
		}
		secretCipher = enc
	} else {
		// mock 空 Secret：保留库内既有密文，避免已存在值被无意义清空。
		if existing, err := s.loadRawParams(ctx, providerType); err == nil {
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

// SaveParamsTx 事务内写入提供商参数 JSON（Setup OIDC 分支使用）；写入前校验 base_url。
func (s *Service) SaveParamsTx(ctx context.Context, tx *sql.Tx, providerType, rawJSON string) error {
	var p Params
	if err := json.Unmarshal([]byte(rawJSON), &p); err != nil {
		return fmt.Errorf("解析 OIDC 参数失败: %w", err)
	}
	if err := validateOIDCBaseURL(providerType, p.BaseURL); err != nil {
		return err
	}
	return s.cfg.SetTx(ctx, tx, "oidc_params_"+providerType, rawJSON)
}

// SetProviderTx 事务内写入提供商类型
func (s *Service) SetProviderTx(ctx context.Context, tx *sql.Tx, providerType string) error {
	return s.cfg.SetTx(ctx, tx, KeyProviderType, providerType)
}

// CallbackURL 回调地址（frontend_url + /api/auth/oidc/callback）
func (s *Service) CallbackURL(ctx context.Context) string {
	furl := s.cfg.GetOr(ctx, config.KeyFrontendURL)
	return furl + "/api/auth/oidc/callback"
}

// --- 发现文档获取（带缓存）---

// validateDiscoveryEndpoints 校验发现文档声明的必要端点；JWKS 缺失留给验签阶段报错，存在时必须为 HTTPS。
func validateDiscoveryEndpoints(d *Discovery) error {
	if d.AuthorizationEndpoint == "" || d.TokenEndpoint == "" {
		return errors.New("发现文档缺少必要端点")
	}
	if err := validateOIDCURL(d.AuthorizationEndpoint); err != nil {
		return fmt.Errorf("authorization_endpoint 校验失败: %w", err)
	}
	if err := validateOIDCURL(d.TokenEndpoint); err != nil {
		return fmt.Errorf("token_endpoint 校验失败: %w", err)
	}
	if d.JWKSURI != "" {
		if err := validateOIDCURL(d.JWKSURI); err != nil {
			return fmt.Errorf("jwks_uri 校验失败: %w", err)
		}
	}
	return nil
}

// fetchDiscovery 获取发现文档（带缓存，缓存键 = base_url + realm）；使用当前生效提供商判定 mock。
// StartFlow/Exchange 必须使用 fetchDiscoveryForProvider 传入固定 provider，不能经本包装读取当前配置。
func (s *Service) fetchDiscovery(ctx context.Context, p *Params) (*Discovery, error) {
	return s.fetchDiscoveryForProvider(ctx, s.cfg.GetOr(ctx, KeyProviderType), p)
}

// fetchDiscoveryForProvider 按显式 provider 获取发现文档；mock 不依赖真实网络。
func (s *Service) fetchDiscoveryForProvider(ctx context.Context, providerType string, p *Params) (*Discovery, error) {
	// 模拟模式：不依赖真实提供商
	if providerType == "mock" {
		return &Discovery{AuthorizationEndpoint: "mock://authorize", TokenEndpoint: "mock://token"}, nil
	}
	base := strings.TrimSuffix(p.BaseURL, "/")
	wellKnown := base + "/.well-known/openid-configuration"
	if p.Realm != "" {
		wellKnown = base + "/realms/" + p.Realm + "/.well-known/openid-configuration"
	}
	if err := validateOIDCURL(wellKnown); err != nil {
		return nil, fmt.Errorf("发现文档地址校验失败: %w", err)
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
	if err := validateDiscoveryEndpoints(&disc); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.discCache[cacheKey] = &disc
	s.mu.Unlock()
	return &disc, nil
}

// ClearDiscCache 清空发现文档与 JWKS 缓存（配置变更后调用）
func (s *Service) ClearDiscCache() {
	s.mu.Lock()
	s.discCache = map[string]*Discovery{}
	s.mu.Unlock()
	s.jwksMu.Lock()
	s.jwksCache = map[string]*jwkSet{}
	s.jwksMu.Unlock()
}

// getJWKS 获取并缓存 OIDC 提供商 JWKS；使用当前 httpCli 以沿用 SSRF/代理策略。
func (s *Service) getJWKS(ctx context.Context, jwksURI string) (*jwkSet, error) {
	if err := validateOIDCURL(jwksURI); err != nil {
		return nil, fmt.Errorf("JWKS 地址校验失败: %w", err)
	}
	s.jwksMu.Lock()
	if set, ok := s.jwksCache[jwksURI]; ok {
		s.jwksMu.Unlock()
		return set, nil
	}
	s.jwksMu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURI, nil)
	if err != nil {
		return nil, fmt.Errorf("构造 JWKS 请求失败: %w", err)
	}
	resp, err := s.httpCli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取 JWKS 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS 返回 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取 JWKS 失败: %w", err)
	}
	var set jwkSet
	if err := json.Unmarshal(body, &set); err != nil {
		return nil, fmt.Errorf("解析 JWKS 失败: %w", err)
	}
	s.jwksMu.Lock()
	s.jwksCache[jwksURI] = &set
	s.jwksMu.Unlock()
	return &set, nil
}

// doCredentialRequest 执行携带 Client Secret 的 token 请求，禁止任何重定向。
// Go 对 307/308 会保留 method 与 body；沿用通用 HTTPS 重定向策略可能把 Secret 重放到其他主机。
func (s *Service) doCredentialRequest(req *http.Request) (*http.Response, error) {
	if err := validateOIDCURL(req.URL.String()); err != nil {
		return nil, err
	}
	cli := *s.httpCli
	cli.CheckRedirect = func(next *http.Request, _ []*http.Request) error {
		if err := validateOIDCURL(next.URL.String()); err != nil {
			return err
		}
		return errors.New("token 凭据请求禁止重定向，已阻止 Client Secret 转发")
	}
	return cli.Do(req)
}

// refreshJWKS 删除指定 JWKS 缓存，下次 getJWKS 会重新拉取（用于密钥轮换后的重试）。
func (s *Service) refreshJWKS(jwksURI string) {
	s.jwksMu.Lock()
	delete(s.jwksCache, jwksURI)
	s.jwksMu.Unlock()
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
