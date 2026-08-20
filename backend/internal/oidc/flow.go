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
)

// StateRecord oidc_states 记录
type StateRecord struct {
	State        string
	CodeVerifier string
	Intent       string
	BindUserID   int64
	CreatedAt    time.Time
}

// StartFlow 生成 state（≥128 位）与 code_verifier（PKCE S256）→ 持久化 → 返回授权页 URL
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
	// TTL 10 分钟：写入前顺带清理过期记录（代替独立定时器，简单可靠）
	if err := s.saveState(ctx, state, verifier, intent, bindUserID); err != nil {
		return "", "", err
	}
	p, err := s.currentParams(ctx)
	if err != nil {
		return "", "", err
	}
	challenge := pkceChallenge(verifier)
	disc, err := s.fetchDiscovery(ctx, p)
	if err != nil {
		return "", "", err
	}
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {p.ClientID},
		"redirect_uri":          {s.CallbackURL(ctx)},
		"scope":                 {"openid email profile"},
		"state":                 {state},
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

// saveState 持久化 state（写入前清理过期记录；BEGIN IMMEDIATE 先读后写防并发）
func (s *Service) saveState(ctx context.Context, state, verifier, intent string, bindUserID int64) error {
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		// 顺带清理过期记录（代替独立定时器，简单可靠）
		if _, err := tx.ExecContext(ctx, `DELETE FROM oidc_states WHERE created_at < ?`,
			time.Now().Add(-stateTTL)); err != nil {
			return fmt.Errorf("清理过期 state 失败: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO oidc_states (state, code_verifier, intent, bind_user_id) VALUES (?,?,?,?)`,
			state, verifier, intent, nullIf0(bindUserID)); err != nil {
			return fmt.Errorf("写入 state 失败: %w", err)
		}
		return nil
	})
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
			`SELECT state, code_verifier, intent, COALESCE(bind_user_id,0), created_at FROM oidc_states WHERE state = ?`, state).
			Scan(&rec.State, &rec.CodeVerifier, &rec.Intent, &rec.BindUserID, &rec.CreatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err != nil {
			return err
		}
		if time.Since(rec.CreatedAt) > stateTTL {
			return sql.ErrNoRows // 过期视同不存在
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
	providerType, _ := s.cfg.Get(ctx, KeyProviderType)
	if providerType == "mock" {
		return s.mockExchange(rec, code) // 模拟模式：code 即携带身份信息的 base64 JSON
	}
	p, err := s.currentParams(ctx)
	if err != nil {
		return nil, err
	}
	disc, err := s.fetchDiscovery(ctx, p)
	if err != nil {
		return nil, err
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
	resp, err := s.httpCli.Do(req)
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
	// 解析 id_token payload（本 Build 简化：不验签，Build3 面板提供 jwks 验签增强；生产请配置可信提供商）
	payload, err := decodeJWTPayload(tok.IDToken)
	if err != nil {
		return nil, err
	}
	var claims struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified *bool  `json:"email_verified"`
		PreferredName string `json:"preferred_username"`
		Name          string `json:"name"`
		RealmAccess   struct {
			Roles []string `json:"roles"`
		} `json:"realm_access"`
		Groups []string `json:"groups"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
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
		RawClaims:     string(payload),
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
