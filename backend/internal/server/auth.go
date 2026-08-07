package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/captcha"
	"vpn-sub/internal/config"
	"vpn-sub/internal/ratelimit"
	"vpn-sub/internal/user"
)

// AuthHandler 认证端点处理器（接入层）
type AuthHandler struct {
	authSvc *auth.Service
	userSvc *user.Service
	cfg     *config.Service
	resetSvc *auth.ResetService
}

// RegisterAuthRoutes 注册认证路由；Step 7 在 register/login/forgot 上叠加限流与验证码中间件
func RegisterAuthRoutes(engine *gin.Engine, h *AuthHandler, limiter *ratelimit.Limiter, captchaSvc *captcha.Service) {
	g := engine.Group("/api/auth")
	// 中间件链顺序：限流 → 验证码 → 处理器
	g.POST("/register", limiter.Middleware("register", ratelimit.KeyRegister, 5), captchaSvc.Middleware("register"), h.register)
	g.POST("/login", limiter.Middleware("login", ratelimit.KeyLogin, 10), captchaSvc.Middleware("login"), h.login)
	g.POST("/forgot", limiter.Middleware("forgot", ratelimit.KeyForgot, 5), captchaSvc.Middleware("forgot"), h.forgot)
	g.POST("/reset", h.reset) // 重置凭令牌保护，不额外限流
	g.GET("/me", h.authSvc.SessionMiddleware(), h.me)
	g.POST("/logout", h.authSvc.SessionMiddleware(), h.logout)
}

// 表单入参统一长度限制（AGENTS §八-6）
type registerReq struct {
	Username string `json:"username" binding:"required,min=1,max=64"`
	Email    string `json:"email" binding:"required,max=254"`
	Password string `json:"password" binding:"required,max=128"`
}

func (h *AuthHandler) register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	ctx := c.Request.Context()
	// 注册入口可见性：依赖本地登录开启（Design1 §3.2：本地登录关闭时注册产物无法本地登录，无意义）
	if !h.cfg.GetBool(ctx, config.KeyAllowLocalLogin, true) {
		Fail(c, http.StatusForbidden, "本地登录已关闭")
		return
	}
	// 注册入口可见性：allow_selfreg 开启，或用户表为空（例外，Design1 §5.2）
	allowSelf := h.cfg.GetBool(ctx, config.KeyAllowSelfreg, false)
	empty, err := h.userSvc.IsTableEmpty(ctx)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !allowSelf && !empty {
		Fail(c, http.StatusForbidden, "未开放注册")
		return
	}
	u, err := h.userSvc.Register(ctx, req.Username, req.Email, req.Password)
	if errors.Is(err, user.ErrEmailConflict) {
		Fail(c, http.StatusConflict, "该邮箱已被注册")
		return
	}
	if errors.Is(err, user.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if u.Status == "active" {
		// 直接激活：签发会话（注册无记住我选项，按 24 小时）
		token, exp, err := h.authSvc.Issue(ctx, u.ID, u.CredentialVersion, auth.SessionNoRemember)
		if err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		OK(c, gin.H{"token": token, "expires_at": exp.Unix(), "status": u.Status, "is_admin": u.Role == "admin"})
		return
	}
	OK(c, gin.H{"status": "pending", "message": "账号已提交，等待管理员审批"})
}

type loginReq struct {
	Email    string `json:"email" binding:"required,max=254"`
	Password string `json:"password" binding:"required,max=128"`
	Remember bool   `json:"remember"`
}

func (h *AuthHandler) login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	ctx := c.Request.Context()
	// 本地登录开关（Design1 §3.2：本地登录为基底，可关闭；关闭后本端点不可用，注册入口同步隐藏）
	if !h.cfg.GetBool(ctx, config.KeyAllowLocalLogin, true) {
		Fail(c, http.StatusForbidden, "本地登录已关闭")
		return
	}
	u, err := h.userSvc.Login(ctx, req.Email, req.Password)
	if errors.Is(err, user.ErrAuthFailed) {
		Fail(c, http.StatusUnauthorized, "邮箱或密码错误") // 统一措辞
		return
	}
	if errors.Is(err, user.ErrAccountInactive) {
		Fail(c, http.StatusUnauthorized, "账号未激活或已被禁用")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	dur := auth.SessionNoRemember
	if req.Remember {
		dur = auth.SessionRemember // 7 天 / 24 小时
	}
	token, exp, err := h.authSvc.Issue(c.Request.Context(), u.ID, u.CredentialVersion, dur)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"token": token, "expires_at": exp.Unix(), "user": userInfo(u)})
}

// me 返回当前用户信息（username/email/role/group/status/user_source + group_name 供顶栏展示）
func (h *AuthHandler) me(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.GetInt64(auth.CtxUserID)
	u, err := h.userSvc.GetByID(ctx, userID)
	if err != nil || u == nil {
		Fail(c, http.StatusUnauthorized, "会话凭据无效或已过期")
		return
	}
	info := userInfo(u)
	if u.GroupID != 0 {
		if name, err := h.userSvc.GroupNameByID(ctx, u.GroupID); err == nil {
			info["group_name"] = name
		}
	}
	OK(c, info)
}

// logout 退出为客户端语义（Design1 §5.4：无服务端会话存储），仅返回成功，前端清除本地 token
func (h *AuthHandler) logout(c *gin.Context) {
	OK(c, nil)
}

// forgot 忘记密码：统一防枚举响应
func (h *AuthHandler) forgot(c *gin.Context) {
	var req struct{ Email string `json:"email" binding:"required,max=254"` }
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	if err := h.resetSvc.Request(c.Request.Context(), req.Email); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"message": "若该邮箱已注册，重置链接已发送"}) // 统一防枚举响应
}

// reset 密码重置：校验令牌设置新密码
func (h *AuthHandler) reset(c *gin.Context) {
	var req struct {
		Token    string `json:"token" binding:"required,max=256"`
		Password string `json:"password" binding:"required,max=128"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	if err := h.resetSvc.Complete(c.Request.Context(), req.Token, req.Password); err != nil {
		if errors.Is(err, auth.ErrTokenInvalid) {
			Fail(c, http.StatusBadRequest, "重置链接无效或已过期")
			return
		}
		if errors.Is(err, auth.ErrBadRequest) {
			Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"message": "密码已重置，请使用新密码登录"})
}

// userInfo 组装对外用户信息（group 字段在 Step 5 前可返回空）
func userInfo(u *user.User) gin.H {
	groupID := any(nil)
	if u.GroupID != 0 {
		groupID = u.GroupID
	}
	return gin.H{
		"id":          u.ID,
		"username":    u.Username,
		"email":       u.Email,
		"role":        u.Role,
		"group_id":    groupID,
		"status":      u.Status,
		"user_source": u.Source,
	}
}
