package oidc

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"vpn-sub/internal/config"
)

// StateRecord oidc_states 记录
type StateRecord struct {
	State        string
	CodeVerifier string
	Nonce        string
	Intent       string
	BindUserID   int64
	CreatedAt    time.Time
	ProviderType string // 发起授权时的生效提供商
	ConfigHash   string // 发起授权时该提供商参数原始 JSON 的带版本哈希
}

// StartFlow 生成 state（≥128 位）、nonce 与 code_verifier（PKCE S256）→ 持久化 → 返回授权页 URL
func (s *Service) StartFlow(ctx context.Context, intent string, bindUserID int64) (authURL, state string, err error) {
	stateBytes := make([]byte, 32) // 256 位 ≥ 128 位要求
	if _, err := randRead(stateBytes); err != nil {
		return "", "", fmt.Errorf("生成 state 失败: %w", err)
	}
	state = base64.RawURLEncoding.EncodeToString(stateBytes)
	verifierBytes := make([]byte, 32)
	if _, err := randRead(verifierBytes); err != nil {
		return "", "", fmt.Errorf("生成 code_verifier 失败: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)
	nonceBytes := make([]byte, 32)
	if _, err := randRead(nonceBytes); err != nil {
		return "", "", fmt.Errorf("生成 nonce 失败: %w", err)
	}
	nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)
	// 在同一 BEGIN IMMEDIATE 事务内读取当前 provider/raw 参数并固定到 state，避免发起后配置切换造成混用。
	providerType, p, err := s.saveState(ctx, state, verifier, nonce, intent, bindUserID)
	if err != nil {
		return "", "", err
	}
	challenge := pkceChallenge(verifier)
	disc, err := s.fetchDiscoveryForProvider(ctx, providerType, p)
	if err != nil {
		return "", "", err
	}
	// 实际使用点再守卫：mock 无真实网络请求，真实提供商必须使用 HTTPS 授权端点。
	if providerType != "mock" {
		if err := validateOIDCURL(disc.AuthorizationEndpoint); err != nil {
			return "", "", fmt.Errorf("授权端点地址校验失败: %w", err)
		}
	}
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {p.ClientID},
		"redirect_uri":          {s.CallbackURL(ctx)},
		"scope":                 {"openid email profile"},
		"state":                 {state},
		"nonce":                 {nonce},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	return disc.AuthorizationEndpoint + "?" + q.Encode(), state, nil // endpoint 取自发现文档（带缓存）
}

// pkceChallenge PKCE S256 challenge = BASE64URL(SHA256(verifier))
func pkceChallenge(verifier string) string {
	sum := sha256Sum([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// providerConfigHash 计算发起时配置指纹：带版本前缀，包含提供商类型与参数原始 JSON（含密文）。
func providerConfigHash(providerType, rawJSON string) string {
	sum := sha256Sum([]byte("v1\x00" + providerType + "\x00" + rawJSON))
	return fmt.Sprintf("%x", sum)
}

// parseParamsWithTx 在事务内解析参数 JSON 并解密 Secret，保持与 loadParams/currentParams 相同的损坏拒绝语义。
func (s *Service) parseParamsWithTx(ctx context.Context, tx *sql.Tx, rawJSON string) (*Params, error) {
	var p Params
	if err := json.Unmarshal([]byte(rawJSON), &p); err != nil {
		return nil, fmt.Errorf("解析 OIDC 参数失败: %w", err)
	}
	if p.ClientSecret != "" {
		key, err := s.cfg.GetSigningKeyTx(ctx, tx)
		if err != nil {
			return nil, fmt.Errorf("解密 OIDC Client Secret 失败: %w", err)
		}
		plain, err := config.Decrypt(p.ClientSecret, key)
		if err != nil {
			return nil, fmt.Errorf("解密 OIDC Client Secret 失败: %w", err)
		}
		p.ClientSecret = string(plain)
	}
	if p.ClientSecret == config.MaskedSecret {
		return nil, errors.New("OIDC Client Secret 已被脱敏占位符覆盖，请管理员在设置页重新输入")
	}
	return &p, nil
}

// saveState 在单个 BEGIN IMMEDIATE 事务内清理过期 state、固定发起时 provider/raw 参数并写入 state。
// 返回的 Params 已解密，供 StartFlow 以同一配置快照获取 discovery 与构造授权 URL。
func (s *Service) saveState(ctx context.Context, state, verifier, nonce, intent string, bindUserID int64) (providerType string, p *Params, err error) {
	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		// 顺带清理过期记录（代替独立定时器，简单可靠）
		if _, err := tx.ExecContext(ctx, `DELETE FROM oidc_states WHERE created_at < ?`,
			time.Now().Add(-stateTTL)); err != nil {
			return fmt.Errorf("清理过期 state 失败: %w", err)
		}
		providerType, err = s.cfg.GetTx(ctx, tx, KeyProviderType)
		if err != nil {
			return fmt.Errorf("读取 OIDC 提供商失败: %w", err)
		}
		if providerType == "" {
			return errors.New("OIDC 未配置")
		}
		raw, err := s.cfg.GetTx(ctx, tx, "oidc_params_"+providerType)
		if err != nil {
			return fmt.Errorf("读取 OIDC 参数失败: %w", err)
		}
		if raw == "" {
			return errors.New("OIDC 参数未配置")
		}
		p, err = s.parseParamsWithTx(ctx, tx, raw)
		if err != nil {
			return err
		}
		configHash := providerConfigHash(providerType, raw)
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO oidc_states (state, code_verifier, nonce, intent, bind_user_id, provider_type, config_hash) VALUES (?,?,?,?,?,?,?)`,
			state, verifier, nonce, intent, nullIf0(bindUserID), providerType, configHash); err != nil {
			return fmt.Errorf("写入 state 失败: %w", err)
		}
		return nil
	})
	return providerType, p, err
}

// loadPinnedParams 回调换取身份前校验 state 固定的 provider/config 指纹仍与当前生效配置一致。
// 不一致时在发出任何 discovery/token 请求前拒绝，确保旧授权码绝不接触新提供商地址或 Secret。
func (s *Service) loadPinnedParams(ctx context.Context, rec *StateRecord) (*Params, error) {
	if rec.ProviderType == "" || rec.ConfigHash == "" {
		return nil, errors.New("授权 state 缺少发起时提供商配置标识，请重新发起登录")
	}
	var p *Params
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		currentProvider, err := s.cfg.GetTx(ctx, tx, KeyProviderType)
		if err != nil {
			return fmt.Errorf("读取 OIDC 提供商失败: %w", err)
		}
		if currentProvider != rec.ProviderType {
			return errors.New("授权发起后 OIDC 提供商已变更，请重新发起登录")
		}
		raw, err := s.cfg.GetTx(ctx, tx, "oidc_params_"+currentProvider)
		if err != nil {
			return fmt.Errorf("读取 OIDC 参数失败: %w", err)
		}
		if raw == "" {
			return errors.New("OIDC 参数未配置")
		}
		if providerConfigHash(currentProvider, raw) != rec.ConfigHash {
			return errors.New("授权发起后 OIDC 配置已变更，请重新发起登录")
		}
		p, err = s.parseParamsWithTx(ctx, tx, raw)
		return err
	})
	if err != nil {
		return nil, err
	}
	return p, nil
}

// nullIf0 0 转 NULL
func nullIf0(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

// ConsumeState 回调时校验存储记录存在并用后即删（防重放）；
// 三重校验（Cookie state == 回调参数 state == 存储记录）由接入层比对 Cookie 后调用本方法
func (s *Service) ConsumeState(ctx context.Context, state string) (*StateRecord, error) {
	var rec StateRecord
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx,
			`SELECT state, code_verifier, nonce, intent, COALESCE(bind_user_id,0), created_at, provider_type, config_hash FROM oidc_states WHERE state = ?`, state).
			Scan(&rec.State, &rec.CodeVerifier, &rec.Nonce, &rec.Intent, &rec.BindUserID, &rec.CreatedAt, &rec.ProviderType, &rec.ConfigHash)
		if errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err != nil {
			return err
		}
		if time.Since(rec.CreatedAt) > stateTTL {
			return sql.ErrNoRows // 过期视同不存在
		}
		// 旧记录没有发起时提供商/配置标识：保留原行，回调视为无效 state 拒绝。
		if rec.ProviderType == "" || rec.ConfigHash == "" {
			return errors.New("state 缺少发起时提供商配置标识")
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM oidc_states WHERE state = ?`, state) // 用后即删
		return err
	})
	if err != nil {
		return nil, errors.New("state 无效或已过期")
	}
	return &rec, nil
}

// Identity OIDC 身份信息
type Identity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Username      string
	RoleClaims    []string
	GroupClaims   []string
	RawClaims     string // JSON 快照（待审批用户存 oidc_claims 列）
}

// Exchange 用 code + code_verifier 换 token，解析 id_token/userinfo 提取身份（含 role/group claims）
// 实现说明：真实提供商场景需验签 id_token（jwks）；为保持本 Build 可自测，mock 提供商走本地解析。
// 真实解析：POST token_endpoint 换 token → 解析 id_token（JWT payload 提取 subject/email/email_verified/username）。
func (s *Service) Exchange(ctx context.Context, rec *StateRecord, code string) (*Identity, error) {
	p, err := s.loadPinnedParams(ctx, rec)
	if err != nil {
		return nil, err
	}
	if rec.ProviderType == "mock" {
		return s.mockExchange(rec, code) // 模拟模式：code 即携带身份信息的 base64 JSON
	}
	disc, err := s.fetchDiscoveryForProvider(ctx, rec.ProviderType, p)
	if err != nil {
		return nil, err
	}
	if err := validateOIDCURL(disc.TokenEndpoint); err != nil {
		return nil, fmt.Errorf("token 端点地址校验失败: %w", err)
	}
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {s.CallbackURL(ctx)},
		"client_id":     {p.ClientID},
		"code_verifier": {rec.CodeVerifier},
	}
	if p.ClientSecret != "" {
		form.Set("client_secret", p.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, disc.TokenEndpoint, stringsNewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("构造 token 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.doCredentialRequest(req)
	if err != nil {
		return nil, fmt.Errorf("token 端点不可达: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取 token 响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token 交换失败（HTTP %d）: %s", resp.StatusCode, string(body))
	}
	var tok struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("解析 token 响应失败: %w", err)
	}
	if tok.IDToken == "" {
		return nil, errors.New("token 响应缺少 id_token")
	}
	// 验签 id_token：JWKS 验签 + iss/aud/exp/nonce/azp 校验。
	_, rawClaims, err := s.verifyIDToken(ctx, p, disc, tok.IDToken, rec.Nonce)
	if err != nil {
		return nil, err
	}
	var claims struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified *bool  `json:"email_verified"`
		PreferredName string `json:"preferred_username"`
		Name          string `json:"name"`
		Azp           string `json:"azp"`
		RealmAccess   struct {
			Roles []string `json:"roles"`
		} `json:"realm_access"`
		Groups []string `json:"groups"`
	}
	if err := json.Unmarshal([]byte(rawClaims), &claims); err != nil {
		return nil, fmt.Errorf("解析 id_token payload 失败: %w", err)
	}
	if claims.Sub == "" {
		return nil, errors.New("id_token 缺少 sub")
	}
	verified := false
	if claims.EmailVerified != nil {
		verified = *claims.EmailVerified
	}
	username := claims.PreferredName
	if username == "" {
		username = claims.Name
	}
	if username == "" && claims.Email != "" {
		username = stringsSplitN(claims.Email, "@", 2)[0]
	}
	id := &Identity{
		Subject:       claims.Sub,
		Email:         claims.Email,
		EmailVerified: verified,
		Username:      username,
		RoleClaims:    claims.RealmAccess.Roles,
		GroupClaims:   claims.Groups,
		RawClaims:     rawClaims,
	}
	return id, nil
}

// mockExchange 模拟模式：code 即携带身份信息的 base64 JSON（由模拟登录入口生成，见 MockLogin）
func (s *Service) mockExchange(rec *StateRecord, code string) (*Identity, error) {
	raw, err := base64.RawURLEncoding.DecodeString(code)
	if err != nil {
		return nil, errors.New("模拟 code 无效")
	}
	var claims struct {
		Sub           string   `json:"sub"`
		Email         string   `json:"email"`
		EmailVerified bool     `json:"email_verified"`
		Username      string   `json:"username"`
		Roles         []string `json:"roles"`
		Groups        []string `json:"groups"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		return nil, errors.New("模拟 code 内容无效")
	}
	return &Identity{
		Subject:       claims.Sub,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		Username:      claims.Username,
		RoleClaims:    claims.Roles,
		GroupClaims:   claims.Groups,
		RawClaims:     string(raw),
	}, nil
}
