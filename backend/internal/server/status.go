package server

import (
	"github.com/gin-gonic/gin"

	"vpn-sub/internal/captcha"
	"vpn-sub/internal/config"
	"vpn-sub/internal/oidc"
	"vpn-sub/internal/user"
)

// StatusHandler 系统状态处理器（结构体 Handler + 依赖注入）
type StatusHandler struct {
	cfg         *config.Service
	users       *user.Service
	oidcSvc     *oidc.Service
	captchaSvc  *captcha.Service
}

// registerStatus 注册系统状态端点（公开端点，无需鉴权）
func registerStatus(engine *gin.Engine, cfg *config.Service, users *user.Service, oidcSvc *oidc.Service, captchaSvc *captcha.Service, mode string) {
	h := &StatusHandler{cfg: cfg, users: users, oidcSvc: oidcSvc, captchaSvc: captchaSvc}
	engine.GET("/api/system/status", h.handle(mode))
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
		OK(c, gin.H{
			"configured":         configured,
			"app_mode":           mode,
			"emergency":          false, // TODO(Build3 Step 6)：替换为应急服务真实判定
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
