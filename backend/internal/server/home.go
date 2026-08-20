// Package server 用户端数据端点（接入层）：平台卡片、Token 刷新、更新时间戳。
package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/home"
	"vpn-sub/internal/token"
)

// HomeHandler 用户端数据处理器（结构体 Handler + 依赖注入；R14-16：查询已下沉 internal/home）
type HomeHandler struct {
	homeSvc *home.Service
}

// RegisterHomeRoutes 注册用户端数据端点；全部需会话
func RegisterHomeRoutes(engine *gin.Engine, h *HomeHandler, sessionMW gin.HandlerFunc) {
	g := engine.Group("/api/home", sessionMW)
	g.GET("/platforms", h.platforms)
	g.GET("/summary", h.summary)
	g.POST("/token/refresh", h.refreshToken)
	g.GET("/updated_at", h.updatedAt)
}

// platforms 当前用户可见平台卡片数据，直接携带可用下载 Token，无 Token 时按需生成
func (h *HomeHandler) platforms(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.GetInt64(auth.CtxUserID)
	role := c.GetString(auth.CtxUserRole)
	cards, err := h.homeSvc.ListPlatforms(ctx, userID, role)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, ListData{List: cards, Total: int64(len(cards))}) // 列表统一包裹结构（AGENTS §4.8）
}

// summary 首页独立汇总端点（Design2Report11 决策）：traffic + home_rule。
func (h *HomeHandler) summary(c *gin.Context) {
	resp, err := h.homeSvc.Summary(c.Request.Context())
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, resp)
}

// refreshToken 刷新指定平台下载 Token（旧失效）——业务逻辑在 internal/home
func (h *HomeHandler) refreshToken(c *gin.Context) {
	var req struct {
		PlatformID int64 `json:"platform_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	userID := c.GetInt64(auth.CtxUserID)
	t, err := h.homeSvc.RefreshToken(c.Request.Context(), userID, req.PlatformID)
	if errors.Is(err, token.ErrTokenNotFound) {
		Fail(c, http.StatusNotFound, "该平台无可用 Token")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"token": t})
}

// updatedAt 订阅更新时间戳：普通用户=自定义订阅 + 平台唯一订阅的最大版本更新时间；管理员=全池最大值
func (h *HomeHandler) updatedAt(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.GetInt64(auth.CtxUserID)
	role := c.GetString(auth.CtxUserRole)
	ts, err := h.homeSvc.UpdatedAt(ctx, userID, role)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"updated_at": ts})
}
