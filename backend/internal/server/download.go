// Package server 统一下载端点与会话预览（接入层）。
package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/download"
	"vpn-sub/internal/ratelimit"
	"vpn-sub/internal/version"
)

// DownloadHandler 下载处理器（结构体 Handler + 依赖注入）
type DownloadHandler struct {
	dlSvc     *download.Service
	limiter   *ratelimit.Limiter
	sessionMW gin.HandlerFunc
}

// RegisterDownloadRoutes 注册下载端点；全部下载端点：限流（20/min 按 IP）+ 禁缓存 + text/plain + 访问日志
func RegisterDownloadRoutes(engine *gin.Engine, h *DownloadHandler) {
	dl := engine.Group("", h.limiter.Middleware("download", ratelimit.KeyDownload, 20))
	dl.GET("/subscriptions/:platform/download", h.userDownload)
	dl.GET("/share/:slug/download", h.shareDownload)                 // Step 5 接通解析
	dl.GET("/rules/:slug/download", h.ruleDownload)                  // Step 6 接通解析
	engine.GET("/api/subscriptions/preview", h.sessionMW, h.preview) // 会话凭据预览（独立鉴权）
}

// setNoCache 禁缓存头（AGENTS §4.5 下载类端点）
func setNoCache(c *gin.Context) {
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
	c.Header("Pragma", "no-cache")
}

func (h *DownloadHandler) userDownload(c *gin.Context) {
	ctx := c.Request.Context()
	res, entry, err := h.dlSvc.ResolveUserDownload(ctx, c.Query("token"), c.Param("platform"))
	ip := c.ClientIP()
	switch {
	case errors.Is(err, download.ErrTokenInvalid):
		h.dlSvc.WriteAccessLog(ctx, ip, entry, false)
		setNoCache(c)
		Fail(c, http.StatusNotFound, "资源不存在") // 统一 404，不泄露资源存在性
	case errors.Is(err, version.ErrVersionNotFound):
		h.dlSvc.WriteAccessLog(ctx, ip, entry, false) // 无激活版本：HTTP 200 纯文本注释块
		setNoCache(c)
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("# error: no active version\n"))
	case errors.Is(err, download.ErrUnassigned):
		h.dlSvc.WriteAccessLog(ctx, ip, entry, false)
		setNoCache(c)
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("# error: unassigned\n")) // HTTP 200 纯文本注释块
	case err != nil:
		Fail(c, http.StatusInternalServerError, err.Error())
	default:
		h.dlSvc.WriteAccessLog(ctx, ip, entry, true)
		setNoCache(c)
		// 下载文件名（资源名 + 原始扩展名）；平台附加头优先（如 clash-verge 预置的 Content-Disposition）
		if res.Filename != "" {
			c.Header("Content-Disposition", `attachment; filename="`+download.SanitizeFilename(res.Filename)+`"`)
		}
		for k, v := range res.ExtraHeaders {
			c.Header(k, v)
		}
		c.Data(http.StatusOK, "text/plain; charset=utf-8", res.Content)
	}
}

// shareDownload 分享下载（Step 5 接通 ResolveShare；附 Content-Disposition）
func (h *DownloadHandler) shareDownload(c *gin.Context) {
	ctx := c.Request.Context()
	slug := c.Param("slug")
	res, entry, err := h.dlSvc.ResolveShare(ctx, c.Query("token"), slug)
	ip := c.ClientIP()
	switch {
	case errors.Is(err, download.ErrTokenInvalid):
		h.dlSvc.WriteAccessLog(ctx, ip, entry, false)
		setNoCache(c)
		Fail(c, http.StatusNotFound, "资源不存在")
	case errors.Is(err, version.ErrVersionNotFound):
		h.dlSvc.WriteAccessLog(ctx, ip, entry, false) // 无激活版本：HTTP 200 纯文本注释块
		setNoCache(c)
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("# error: no active version\n"))
	case err != nil:
		Fail(c, http.StatusInternalServerError, err.Error())
	default:
		h.dlSvc.WriteAccessLog(ctx, ip, entry, true)
		setNoCache(c)
		filename := res.Filename
		if filename == "" {
			filename = slug
		}
		c.Header("Content-Disposition", `attachment; filename="`+download.SanitizeFilename(filename)+`"`)
		c.Data(http.StatusOK, "text/plain; charset=utf-8", res.Content)
	}
}

// ruleDownload 规则下载（Step 6 接通 ResolveRule；附 Content-Disposition）
func (h *DownloadHandler) ruleDownload(c *gin.Context) {
	ctx := c.Request.Context()
	slug := c.Param("slug")
	res, entry, err := h.dlSvc.ResolveRule(ctx, c.Query("token"), slug)
	ip := c.ClientIP()
	switch {
	case errors.Is(err, download.ErrTokenInvalid):
		h.dlSvc.WriteAccessLog(ctx, ip, entry, false)
		setNoCache(c)
		Fail(c, http.StatusNotFound, "资源不存在")
	case errors.Is(err, version.ErrVersionNotFound):
		h.dlSvc.WriteAccessLog(ctx, ip, entry, false) // 无激活版本：HTTP 200 纯文本注释块
		setNoCache(c)
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("# error: no active version\n"))
	case err != nil:
		Fail(c, http.StatusInternalServerError, err.Error())
	default:
		h.dlSvc.WriteAccessLog(ctx, ip, entry, true)
		setNoCache(c)
		filename := res.Filename
		if filename == "" {
			filename = slug
		}
		c.Header("Content-Disposition", `attachment; filename="`+download.SanitizeFilename(filename)+`"`)
		c.Data(http.StatusOK, "text/plain; charset=utf-8", res.Content)
	}
}

// preview 会话凭据预览（Design2 §4.4）：管理员与普通用户统一按「平台 → 唯一订阅 → 当前版本」解析；
// 普通用户有自定义订阅时优先返回自定义内容；无激活版本返回 HTTP 200 注释块。
func (h *DownloadHandler) preview(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.GetInt64(auth.CtxUserID)
	platformSlug := c.Query("platform")
	content, err := h.dlSvc.PreviewForUser(ctx, userID, platformSlug)
	switch {
	case errors.Is(err, download.ErrTokenInvalid):
		Fail(c, http.StatusNotFound, "资源不存在")
	case errors.Is(err, download.ErrUnassigned):
		setNoCache(c)
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("# error: unassigned\n"))
	case errors.Is(err, version.ErrVersionNotFound):
		h.dlSvc.WriteAccessLog(ctx, c.ClientIP(), &download.AccessEntry{
			UserID: userID, Platform: platformSlug, Type: "subscription", FailReason: "no_active_version",
		}, false)
		setNoCache(c)
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("# error: no active version\n"))
	case err != nil:
		Fail(c, http.StatusInternalServerError, err.Error())
	default:
		setNoCache(c)
		c.Data(http.StatusOK, "text/plain; charset=utf-8", content)
	}
}
