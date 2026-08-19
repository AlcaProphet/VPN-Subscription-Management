package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/proxygroup"
)

// ProxyGroupHandler 代理组处理器。
type ProxyGroupHandler struct {
	groupSvc *proxygroup.Service
}

// RegisterProxyGroupRoutes 注册代理组管理路由（会话 + 管理员双中间件）。
func RegisterProxyGroupRoutes(engine *gin.Engine, h *ProxyGroupHandler, sessionMW, adminMW gin.HandlerFunc) {
	admin := engine.Group("/api/admin/proxy-groups", sessionMW, adminMW)
	admin.GET("", h.list)
	admin.POST("", h.create)
	admin.PUT("/:id", h.update)
	admin.DELETE("/:id", h.delete)
	admin.PUT("/:id/preset-toggle", h.presetToggle)
}

func (h *ProxyGroupHandler) list(c *gin.Context) {
	list, err := h.groupSvc.List(c.Request.Context())
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, ListData{List: list, Total: int64(len(list))})
}

func (h *ProxyGroupHandler) create(c *gin.Context) {
	var req struct {
		Name      string                   `json:"name" binding:"required"`
		GroupType string                   `json:"group_type" binding:"required"`
		Definition proxygroup.Definition   `json:"definition"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	g, err := h.groupSvc.CreateCustom(c.Request.Context(), req.Name, req.GroupType, req.Definition)
	if errors.Is(err, proxygroup.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, proxygroup.ErrConflict) {
		Fail(c, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, g)
}

func (h *ProxyGroupHandler) update(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		GroupType  string                 `json:"group_type" binding:"required"`
		Definition proxygroup.Definition  `json:"definition"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	g, err := h.groupSvc.Update(c.Request.Context(), id, req.GroupType, req.Definition)
	if errors.Is(err, proxygroup.ErrNotFound) {
		Fail(c, http.StatusNotFound, "代理组不存在")
		return
	}
	if errors.Is(err, proxygroup.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, g)
}

func (h *ProxyGroupHandler) delete(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	err := h.groupSvc.Delete(c.Request.Context(), id)
	if errors.Is(err, proxygroup.ErrNotFound) {
		Fail(c, http.StatusNotFound, "代理组不存在")
		return
	}
	if errors.Is(err, proxygroup.ErrForbidden) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

func (h *ProxyGroupHandler) presetToggle(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	g, err := h.groupSvc.SetPresetEnabled(c.Request.Context(), id, req.Enabled)
	if errors.Is(err, proxygroup.ErrNotFound) {
		Fail(c, http.StatusNotFound, "代理组不存在")
		return
	}
	if errors.Is(err, proxygroup.ErrForbidden) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, g)
}
