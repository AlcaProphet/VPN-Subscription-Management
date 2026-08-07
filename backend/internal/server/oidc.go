package server

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/oidc"
)

// OidcHandler OIDC 端点处理器（接入层）
type OidcHandler struct {
	oidcSvc *oidc.Service
	authSvc *auth.Service
}

// RegisterOidcRoutes 注册 OIDC 路由
func RegisterOidcRoutes(engine *gin.Engine, h *OidcHandler, sessionMW gin.HandlerFunc) {
	g := engine.Group("/api/auth/oidc")
	g.GET("/login", h.login)                       // 发起授权（302），不限流
	g.GET("/callback", h.callback)                 // 回调，不限流（state 一次性 + 三重校验已防重放）
	g.POST("/mock/login", h.mockLogin)             // 模拟登录（仅 Dev + mock）
	g.POST("/bind", sessionMW, h.bind)             // 发起绑定（需会话）
	engine.POST("/api/oidc/test", h.test)          // 本 Step 不加鉴权；Build3 新增管理员专用测试端点
}

const stateCookie = "oidc_state"

func (h *OidcHandler) login(c *gin.Context) {
	authURL, state, err := h.oidcSvc.StartFlow(c.Request.Context(), "login", 0)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(stateCookie, state, 600, "/", "", c.Request.TLS != nil, true) // HttpOnly，HTTPS 下 Secure
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
		c.Redirect(http.StatusFound, "/login/callback?token="+url.QueryEscape(token))
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

// bind 会话内发起绑定授权（StartFlow("bind", userID) → Cookie + 302）
func (h *OidcHandler) bind(c *gin.Context) {
	userID := c.GetInt64(auth.CtxUserID)
	authURL, state, err := h.oidcSvc.StartFlow(c.Request.Context(), "bind", userID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(stateCookie, state, 600, "/", "", c.Request.TLS != nil, true)
	c.Redirect(http.StatusFound, authURL)
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
