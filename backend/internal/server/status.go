package server

import (
	"github.com/gin-gonic/gin"

	"vpn-sub/internal/captcha"
	"vpn-sub/internal/config"
	"vpn-sub/internal/emergency"
	"vpn-sub/internal/oidc"
	"vpn-sub/internal/user"
)

// StatusHandler 系统状态处理器（结构体 Handler + 依赖注入）
type StatusHandler struct {
	cfg        *config.Service
	users      *user.Service
	oidcSvc    *oidc.Service
	captchaSvc *captcha.Service
	emSvc      *emergency.Service // 应急模式下非 nil（Build3 Step 6 接通 emergency 标记）
}

// registerStatus 注册系统状态端点（公开端点，无需鉴权）
func registerStatus(engine *gin.Engine, cfg *config.Service, users *user.Service, oidcSvc *oidc.Service, captchaSvc *captcha.Service, mode string, emSvc *emergency.Service) {
	h := &StatusHandler{cfg: cfg, users: users, oidcSvc: oidcSvc, captchaSvc: captchaSvc, emSvc: emSvc}
	engine.GET("/api/system/status", h.handle(mode))
	// 公告公开端点（Design1 §3.3/§5.2：数据接口公开，未登录可获取；UI 仅登录后首页展示由前端控制）
	// 不注册到 NewEmergency（应急页不依赖，保持最小面，§3.8）
	engine.GET("/api/public/announcement", h.announcement)
}

// announcement 公告/页脚公开端点：返回首页公告 / 登录页公告 / 登录页页脚（仅管理员面板可写，无敏感信息；R07-02/R10-07）
func (h *StatusHandler) announcement(c *gin.Context) {
	home, _ := h.cfg.Get(c.Request.Context(), "announcement")
	login, _ := h.cfg.Get(c.Request.Context(), "login_announcement")
	footer, _ := h.cfg.Get(c.Request.Context(), "login_footer")
	OK(c, gin.H{"home_announcement": home, "login_announcement": login, "login_footer": footer})
}

// handle 返回系统状态：configured / app_mode / emergency / 本地认证与注册入口字段 / OIDC 字段
func (h *StatusHandler) handle(mode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		configured := h.cfg.GetBool(ctx, config.KeyConfigured, false)
		empty, err := h.users.IsTableEmpty(ctx)
		if err != nil {
			Fail(c, 500, err.Error())
			return
		}
		providerType, _ := h.cfg.Get(ctx, oidc.KeyProviderType)
		captchaProvider, _ := h.cfg.Get(ctx, captcha.KeyProvider)
		siteKey, _ := h.cfg.Get(ctx, captcha.KeySiteKey)
		// 应急标记（Build3 Step 6）：应急模式下 true + 触发原因 + 可用能力
		emergencyOn := false
		emergencyReason := ""
		canResetPassword := false
		if h.emSvc != nil {
			emergencyOn = true
			emergencyReason = string(h.emSvc.Reason())
			canResetPassword = h.emSvc.CanResetPassword(ctx)
		}
		OK(c, gin.H{
			"configured":         configured,
			"app_mode":           mode,
			"emergency":          emergencyOn,
			"emergency_reason":   emergencyReason,
			"can_reset_password": canResetPassword,
			"allow_local_login":  h.cfg.GetBool(ctx, config.KeyAllowLocalLogin, true),
			"allow_selfreg":      h.cfg.GetBool(ctx, config.KeyAllowSelfreg, false),
			"user_table_empty":   empty, // 注册入口可见性所需，有意公开（Design1 §5.2）
			"oidc_configured":    h.oidcSvc.IsConfigured(ctx),
			"oidc_provider_type": providerType, // 未配置时为空串
			// 验证码字段（供前端渲染验证码组件；secret_key 禁止返回）
			"captcha_provider": captchaProvider,
			"captcha_site_key": siteKey,
			"captcha_pages":    h.cfg.GetJSONStringSlice(ctx, captcha.KeyPages),
		})
	}
}
