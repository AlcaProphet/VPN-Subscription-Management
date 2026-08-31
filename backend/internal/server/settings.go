// server/settings.go：面板配置端点（接入层，Build3 Step 3）——会话 + 管理员双中间件；按分区独立 GET/PUT。
// 敏感字段 GET 返回脱敏值；PUT 空串字段不修改；站点信息公开端点无需鉴权。
package server

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/config"
	"vpn-sub/internal/oidc"
	"vpn-sub/internal/proxytrust"
)

// SettingsHandler 面板配置处理器（结构体 Handler + 依赖注入）
type SettingsHandler struct {
	adminCfg   *config.AdminService
	oidcSvc    *oidc.Service
	trustProxy *proxytrust.Policy // TRUST_PROXY 策略（速率限制分区展示生效值）
}

// oidcOpsAdapter 将 oidc.Service 适配为 config.OidcOps 接口（config 包避免循环依赖）
type oidcOpsAdapter struct {
	svc *oidc.Service
}

func (a oidcOpsAdapter) SaveParams(ctx context.Context, providerType, baseURL, realm, clientID, clientSecret string) error {
	return a.svc.SaveParams(ctx, providerType, oidc.Params{
		BaseURL: baseURL, Realm: realm, ClientID: clientID, ClientSecret: clientSecret,
	})
}

func (a oidcOpsAdapter) LoadParams(ctx context.Context, providerType string) (string, string, string, string, error) {
	p, err := a.svc.LoadParams(ctx, providerType)
	if err != nil {
		return "", "", "", "", err
	}
	return p.BaseURL, p.Realm, p.ClientID, p.ClientSecret, nil
}

func (a oidcOpsAdapter) IsConfigured(ctx context.Context) bool {
	return a.svc.IsConfigured(ctx)
}

func (a oidcOpsAdapter) ClearDiscCache() {
	a.svc.ClearDiscCache()
}

// RegisterSettingsRoutes 注册面板配置端点；按分区独立 GET/PUT（不聚合）
func RegisterSettingsRoutes(engine *gin.Engine, h *SettingsHandler, sessionMW, adminMW gin.HandlerFunc) {
	g := engine.Group("/api/admin/settings", sessionMW, adminMW)
	g.GET("/oidc", h.getOidc)
	g.PUT("/oidc", h.saveOidc)
	g.DELETE("/oidc", h.clearOidc) // 清空 OIDC 配置（二次确认前端负责）
	g.GET("/oidc-rules", h.getOidcRules)
	g.PUT("/oidc-rules", h.saveOidcRules)
	g.GET("/local-auth", h.getLocalAuth)
	g.PUT("/local-auth", h.saveLocalAuth)
	g.GET("/captcha", h.getCaptcha)
	g.PUT("/captcha", h.saveCaptcha)
	g.GET("/smtp", h.getSMTP)
	g.PUT("/smtp", h.saveSMTP)
	g.GET("/site", h.getSite)
	g.PUT("/site", h.saveSite) // multipart（名称 + ICON 文件可选）
	g.DELETE("/site/icon", h.deleteSiteIcon)
	g.GET("/ratelimit", h.getRateLimit)
	g.PUT("/ratelimit", h.saveRateLimit)
	g.GET("/log-level", h.getLogLevel)
	g.PUT("/log-level", h.saveLogLevel)
	g.GET("/announcement", h.getAnnouncement)
	g.PUT("/announcement", h.saveAnnouncement)
	g.GET("/debug", h.getDebug)
	g.PUT("/debug", h.saveDebug)
	g.GET("/advanced", h.getAdvanced)
	g.PUT("/advanced", h.saveAdvanced)
	// OIDC 测试连接（管理员专用，复用 Build1 Step 6 TestConnection）
	g.POST("/oidc/test", h.testOidc)

	// 站点信息公开端点（无需鉴权，供登录页/Setup/首页渲染）
	engine.GET("/api/site/info", h.siteInfo)
}

// --- OIDC 配置分区 ---

func (h *SettingsHandler) getOidc(c *gin.Context) {
	out, err := h.adminCfg.GetOidc(c.Request.Context())
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, out)
}

func (h *SettingsHandler) saveOidc(c *gin.Context) {
	var in config.OidcSettings
	if err := c.ShouldBindJSON(&in); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	if err := h.adminCfg.SaveOidc(c.Request.Context(), in); err != nil {
		mapSettingsErr(c, err)
		return
	}
	// 前端地址/回调地址修改需重启容器生效（启动缓存语义）
	needRestart := in.FrontendURL != "" || in.CallbackURL != ""
	OK(c, gin.H{"need_restart": needRestart})
}

func (h *SettingsHandler) clearOidc(c *gin.Context) {
	if err := h.adminCfg.ClearOidc(c.Request.Context()); err != nil {
		mapSettingsErr(c, err)
		return
	}
	OK(c, nil)
}

// testOidc 测试连接（复用 Build1 Step 6 TestConnection，加管理员校验）
func (h *SettingsHandler) testOidc(c *gin.Context) {
	var req struct {
		ProviderType string `json:"provider_type"`
		BaseURL      string `json:"base_url"`
		Realm        string `json:"realm"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	p := oidc.Params{BaseURL: req.BaseURL, Realm: req.Realm, ClientID: req.ClientID, ClientSecret: req.ClientSecret}
	res, err := h.oidcSvc.TestConnection(c.Request.Context(), req.ProviderType, p)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, res)
}

// --- OIDC 启用规则分区 ---

func (h *SettingsHandler) getOidcRules(c *gin.Context) {
	approvalOn, wl, err := h.adminCfg.GetOidcRules(c.Request.Context())
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"approval_on": approvalOn, "whitelist": wl})
}

func (h *SettingsHandler) saveOidcRules(c *gin.Context) {
	var req struct {
		ApprovalOn bool                   `json:"approval_on"`
		Whitelist  config.WhitelistConfig `json:"whitelist"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	warning, err := h.adminCfg.SaveOidcRules(c.Request.Context(), req.ApprovalOn, req.Whitelist)
	if err != nil {
		mapSettingsErr(c, err)
		return
	}
	OK(c, gin.H{"warning": warning}) // 白名单为空且审批开启 → warning 标记（前端显著警告）
}

// --- 本地认证分区 ---

func (h *SettingsHandler) getLocalAuth(c *gin.Context) {
	OK(c, h.adminCfg.GetLocalAuth(c.Request.Context()))
}

func (h *SettingsHandler) saveLocalAuth(c *gin.Context) {
	var in config.LocalAuthSettings
	if err := c.ShouldBindJSON(&in); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	if err := h.adminCfg.SaveLocalAuth(c.Request.Context(), in); err != nil {
		mapSettingsErr(c, err)
		return
	}
	OK(c, nil)
}

// --- 验证码分区 ---

func (h *SettingsHandler) getCaptcha(c *gin.Context) {
	OK(c, h.adminCfg.GetCaptcha(c.Request.Context()))
}

func (h *SettingsHandler) saveCaptcha(c *gin.Context) {
	var in config.CaptchaSettings
	if err := c.ShouldBindJSON(&in); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	if err := h.adminCfg.SaveCaptcha(c.Request.Context(), in); err != nil {
		mapSettingsErr(c, err)
		return
	}
	OK(c, nil)
}

// --- SMTP 分区 ---

func (h *SettingsHandler) getSMTP(c *gin.Context) {
	OK(c, h.adminCfg.GetSMTP(c.Request.Context()))
}

func (h *SettingsHandler) saveSMTP(c *gin.Context) {
	var in config.SMTPSettings
	if err := c.ShouldBindJSON(&in); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	if err := h.adminCfg.SaveSMTP(c.Request.Context(), in); err != nil {
		mapSettingsErr(c, err)
		return
	}
	OK(c, nil)
}

// --- 站点信息分区 ---

func (h *SettingsHandler) getSite(c *gin.Context) {
	OK(c, h.adminCfg.GetSiteInfo(c.Request.Context()))
}

// saveSite multipart：名称 + ICON 文件可选（≤2MB，png/jpeg/webp/ico）
func (h *SettingsHandler) saveSite(c *gin.Context) {
	name := c.PostForm("site_name")
	if name == "" {
		Fail(c, http.StatusBadRequest, "站点名称必填")
		return
	}
	var icon io.Reader
	var iconName string
	if file, header, err := c.Request.FormFile("icon"); err == nil {
		defer file.Close()
		icon = file
		iconName = header.Filename
	}
	if err := h.adminCfg.SaveSiteInfo(c.Request.Context(), name, icon, iconName); err != nil {
		mapSettingsErr(c, err)
		return
	}
	OK(c, h.adminCfg.GetSiteInfo(c.Request.Context()))
}

func (h *SettingsHandler) deleteSiteIcon(c *gin.Context) {
	if err := h.adminCfg.DeleteSiteIcon(c.Request.Context()); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

// siteInfo 站点信息公开端点（无需鉴权；无敏感信息）
func (h *SettingsHandler) siteInfo(c *gin.Context) {
	OK(c, h.adminCfg.GetSiteInfo(c.Request.Context()))
}

// --- 速率限制分区 ---

func (h *SettingsHandler) getRateLimit(c *gin.Context) {
	// 返回当前 TRUST_PROXY 生效值与 CIDR 摘要供前端展示（Design1 §3.4.8）
	OK(c, gin.H{
		"settings":          h.adminCfg.GetRateLimit(c.Request.Context()),
		"trust_proxy":       string(h.trustProxy.Mode()),
		"trust_proxy_cidrs": h.trustProxy.RawCIDRs(),
	})
}

func (h *SettingsHandler) saveRateLimit(c *gin.Context) {
	var in config.RateLimitSettings
	if err := c.ShouldBindJSON(&in); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	if err := h.adminCfg.SaveRateLimit(c.Request.Context(), in); err != nil {
		mapSettingsErr(c, err)
		return
	}
	OK(c, nil)
}

// --- 日志级别分区 ---

func (h *SettingsHandler) getLogLevel(c *gin.Context) {
	OK(c, gin.H{"level": h.adminCfg.GetLogLevel(c.Request.Context())})
}

func (h *SettingsHandler) saveLogLevel(c *gin.Context) {
	var req struct {
		Level string `json:"level" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	if err := h.adminCfg.SetLogLevel(c.Request.Context(), req.Level); err != nil {
		mapSettingsErr(c, err)
		return
	}
	OK(c, nil)
}

// --- 公告与页脚分区（R10-07：首页公告 / 登录页公告 / 登录页页脚三份独立配置）---

func (h *SettingsHandler) getAnnouncement(c *gin.Context) {
	OK(c, gin.H{
		"home_announcement":  h.adminCfg.GetAnnouncement(c.Request.Context()),
		"login_announcement": h.adminCfg.GetLoginAnnouncement(c.Request.Context()),
		"login_footer":       h.adminCfg.GetLoginFooter(c.Request.Context()),
	})
}

func (h *SettingsHandler) saveAnnouncement(c *gin.Context) {
	var req struct {
		HomeAnnouncement  string `json:"home_announcement"`
		LoginAnnouncement string `json:"login_announcement"`
		LoginFooter       string `json:"login_footer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	if err := h.adminCfg.SaveAnnouncement(c.Request.Context(), req.HomeAnnouncement); err != nil {
		mapSettingsErr(c, err)
		return
	}
	if err := h.adminCfg.SaveLoginAnnouncement(c.Request.Context(), req.LoginAnnouncement); err != nil {
		mapSettingsErr(c, err)
		return
	}
	if err := h.adminCfg.SaveLoginFooter(c.Request.Context(), req.LoginFooter); err != nil {
		mapSettingsErr(c, err)
		return
	}
	OK(c, nil)
}

// --- 调试模式分区 ---

func (h *SettingsHandler) getDebug(c *gin.Context) {
	OK(c, gin.H{"on": h.adminCfg.GetDebug(c.Request.Context())})
}

func (h *SettingsHandler) saveDebug(c *gin.Context) {
	var req struct {
		On bool `json:"on"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	if err := h.adminCfg.SetDebug(c.Request.Context(), req.On); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

// --- 高级模式分区 ---

func (h *SettingsHandler) getAdvanced(c *gin.Context) {
	OK(c, h.adminCfg.GetAdvancedSettings(c.Request.Context()))
}

func (h *SettingsHandler) saveAdvanced(c *gin.Context) {
	var req struct {
		config.AdvancedSettings
		ConfirmWord string `json:"confirm_word"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	taskID, err := h.adminCfg.SaveAdvancedSettings(c.Request.Context(), req.AdvancedSettings, req.ConfirmWord)
	if err != nil {
		mapSettingsErr(c, err)
		return
	}
	if taskID != "" {
		OK(c, gin.H{"task_id": taskID})
		return
	}
	OK(c, gin.H{"message": "高级模式设置已保存"})
}

// mapSettingsErr 面板配置错误映射：参数类 → 400（含死锁/验证码密钥缺失提示）
func mapSettingsErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, config.ErrBadRequest), errors.Is(err, config.ErrAuthDeadlock),
		errors.Is(err, config.ErrCaptchaKeyMissing):
		Fail(c, http.StatusBadRequest, err.Error())
	default:
		Fail(c, http.StatusInternalServerError, err.Error())
	}
}
