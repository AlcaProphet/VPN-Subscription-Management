package server

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/config"
	"vpn-sub/internal/oidc"
	"vpn-sub/internal/proxytrust"
	"vpn-sub/internal/ratelimit"
	"vpn-sub/internal/setup"
	"vpn-sub/internal/store"
)

// OidcHandler OIDC 端点处理器（接入层）
type OidcHandler struct {
	oidcSvc  *oidc.Service
	authSvc  *auth.Service
	setupSvc *setup.Service
	store    *store.Store
	cfg      *config.Service
	trust    *proxytrust.Policy
}

// RegisterOidcRoutes 注册 OIDC 路由
func RegisterOidcRoutes(engine *gin.Engine, h *OidcHandler, sessionMW gin.HandlerFunc, limiter *ratelimit.Limiter) {
	g := engine.Group("/api/auth/oidc")
	g.GET("/login", h.login)           // 发起授权（302），不限流
	g.GET("/callback", h.callback)     // 回调，不限流（state 一次性 + 三重校验已防重放）
	g.POST("/mock/login", h.mockLogin) // 模拟登录（仅 Dev + mock）
	g.POST("/bind", sessionMW, h.bind) // 发起绑定（需会话）
	g.POST("/exchange", h.exchange)    // HttpOnly ticket 一次性换会话（L01）
	// Setup 匿名测试：仅系统未配置时可用；系统已配置后不再匿名开放（管理员专用端点保留）。
	engine.POST("/api/setup/oidc/test", limiter.Middleware("oidc_test", "ratelimit_oidc_test", 10), func(c *gin.Context) {
		configured, err := h.setupSvc.IsConfigured(c.Request.Context())
		if err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		if configured {
			Fail(c, http.StatusNotFound, "接口不存在")
			return
		}
		h.test(c)
	})
}

const stateCookie = "oidc_state"

// requestIsSecure 判定当前请求是否视为 HTTPS：TLS 直连 > 可信 X-Forwarded-Proto > frontend_url 兜底。
func (h *OidcHandler) requestIsSecure(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	if h.trust != nil && h.trust.Trusted(c.Request.RemoteAddr) {
		xfp := strings.TrimSpace(strings.Split(c.GetHeader("X-Forwarded-Proto"), ",")[0])
		if strings.EqualFold(xfp, "https") {
			return true
		}
	}
	if h.cfg != nil {
		furl, _ := h.cfg.Get(c.Request.Context(), config.KeyFrontendURL)
		if strings.HasPrefix(strings.ToLower(furl), "https://") {
			return true
		}
	}
	return false
}

func (h *OidcHandler) login(c *gin.Context) {
	authURL, state, err := h.oidcSvc.StartFlow(c.Request.Context(), "login", 0)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(stateCookie, state, 600, "/", "", h.requestIsSecure(c), true) // HttpOnly，按 TLS/可信反代/frontend_url 决定 Secure
	c.Redirect(http.StatusFound, authURL)
}

func (h *OidcHandler) callback(c *gin.Context) {
	ctx := c.Request.Context()
	cookieState, err := c.Cookie(stateCookie)
	paramState := c.Query("state")
	// 三重校验前两层：Cookie state == 回调参数 state（第三层：存储记录存在）
	if err != nil || cookieState == "" || cookieState != paramState {
		c.Redirect(http.StatusFound, "/login?oidc_error=state_mismatch")
		return
	}
	rec, err := h.oidcSvc.ConsumeState(ctx, paramState) // 用后即删
	if err != nil {
		c.Redirect(http.StatusFound, "/login?oidc_error=state_expired")
		return
	}
	id, err := h.oidcSvc.Exchange(ctx, rec, c.Query("code"))
	if err != nil {
		c.Redirect(http.StatusFound, "/login?oidc_error=exchange_failed")
		return
	}
	switch rec.Intent {
	case "bind":
		if err := h.oidcSvc.ResolveBind(ctx, rec, id); err != nil {
			c.Redirect(http.StatusFound, "/profile?oidc_bind_error="+url.QueryEscape(err.Error()))
			return
		}
		c.Redirect(http.StatusFound, "/profile?oidc_bound=1") // 不签发会话
	case "login":
		res, err := h.oidcSvc.ResolveLogin(ctx, id)
		if err != nil {
			c.Redirect(http.StatusFound, "/login?oidc_error=resolve_failed")
			return
		}
		if res.Pending {
			c.Redirect(http.StatusFound, "/pending") // 不签发会话，不经凭据中转页
			return
		}
		if res.User == nil { // 冲突（已绑定其他 OIDC/已禁用/邮箱未验证）
			c.Redirect(http.StatusFound, "/login?oidc_error="+url.QueryEscape(res.Message))
			return
		}
		// OIDC 会话固定 7 天，无记住我（Design1 §3.2）
		token, _, err := h.authSvc.Issue(ctx, res.User.ID, res.User.CredentialVersion, auth.OidcSession)
		if err != nil {
			c.Redirect(http.StatusFound, "/login?oidc_error=issue_failed")
			return
		}
		ticket, err := h.issueLoginTicket(ctx, token)
		if err != nil {
			c.Redirect(http.StatusFound, "/login?oidc_error=exchange_failed")
			return
		}
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie("oidc_login_ticket", ticket, 60, "/api/auth/oidc/exchange", "", h.requestIsSecure(c), true)
		c.Redirect(http.StatusFound, "/login/callback") // 不再携带任何查询参数
	}
}

// mockLogin 仅 Dev + mock；入参 email/username/roles/groups/email_verified → 成功签发 7 天会话
func (h *OidcHandler) mockLogin(c *gin.Context) {
	var req struct {
		Email         string   `json:"email" binding:"required,max=254"`
		Username      string   `json:"username" binding:"max=64"`
		EmailVerified bool     `json:"email_verified"`
		Roles         []string `json:"roles"`
		Groups        []string `json:"groups"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	ctx := c.Request.Context()
	res, err := h.oidcSvc.MockLogin(ctx, req.Email, req.Username, req.EmailVerified, req.Roles, req.Groups)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if res.Pending {
		OK(c, gin.H{"status": "pending", "message": res.Message})
		return
	}
	if res.User == nil {
		Fail(c, http.StatusConflict, res.Message)
		return
	}
	token, exp, err := h.authSvc.Issue(ctx, res.User.ID, res.User.CredentialVersion, auth.OidcSession)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"token": token, "expires_at": exp.Unix(), "status": res.User.Status})
}

// issueLoginTicket 生成一次性换票记录（60 秒），用于 OIDC 回调后通过 HttpOnly Cookie 换取会话。
func (h *OidcHandler) issueLoginTicket(ctx context.Context, sessionToken string) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	ticket := base64.RawURLEncoding.EncodeToString(buf)
	err := h.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM oidc_login_tickets WHERE expires_at < ?`, time.Now()); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO oidc_login_tickets (ticket, session_token, expires_at) VALUES (?,?,?)`,
			ticket, sessionToken, time.Now().Add(60*time.Second))
		return err
	})
	if err != nil {
		return "", err
	}
	return ticket, nil
}

// exchange 读取一次性 HttpOnly ticket，返回会话 token；严格一次性，且不留查询参数痕迹。
func (h *OidcHandler) exchange(c *gin.Context) {
	ctx := c.Request.Context()
	ticket, err := c.Cookie("oidc_login_ticket")
	if err != nil || ticket == "" {
		Fail(c, http.StatusUnauthorized, "换票凭据缺失或已过期")
		return
	}
	var sessionToken string
	var expiresAt time.Time
	err = h.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx,
			`SELECT session_token, expires_at FROM oidc_login_tickets WHERE ticket = ?`, ticket).
			Scan(&sessionToken, &expiresAt)
		if err != nil {
			return err
		}
		if time.Now().After(expiresAt) {
			_, _ = tx.ExecContext(ctx, `DELETE FROM oidc_login_tickets WHERE ticket = ?`, ticket)
			return sql.ErrNoRows
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM oidc_login_tickets WHERE ticket = ?`, ticket)
		return err
	})
	c.SetCookie("oidc_login_ticket", "", -1, "/api/auth/oidc/exchange", "", h.requestIsSecure(c), true)
	if err != nil {
		Fail(c, http.StatusUnauthorized, "换票凭据无效或已过期")
		return
	}
	OK(c, gin.H{"token": sessionToken, "expires_at": time.Now().Add(7 * 24 * time.Hour).Unix()})
}

// bind 会话内发起绑定授权（StartFlow("bind", userID) → Cookie + 返回授权 URL 供前端跳转）
// 返回 JSON 而非 302：前端需携带 Bearer 会话凭据调用本端点，浏览器导航无法附加请求头
func (h *OidcHandler) bind(c *gin.Context) {
	userID := c.GetInt64(auth.CtxUserID)
	authURL, state, err := h.oidcSvc.StartFlow(c.Request.Context(), "bind", userID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(stateCookie, state, 600, "/", "", h.requestIsSecure(c), true)
	OK(c, gin.H{"auth_url": authURL})
}

// test 测试连接（不落库）；入参 provider_type + 参数（Setup 与面板共用）
func (h *OidcHandler) test(c *gin.Context) {
	var req struct {
		ProviderType string `json:"provider_type" binding:"required"`
		BaseURL      string `json:"base_url"`
		Realm        string `json:"realm"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	result, err := h.oidcSvc.TestConnection(c.Request.Context(), req.ProviderType, oidc.Params{
		BaseURL: req.BaseURL, Realm: req.Realm, ClientID: req.ClientID, ClientSecret: req.ClientSecret,
	})
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, result)
}
