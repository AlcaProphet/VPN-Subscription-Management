package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/node"
)

// NodeHandler 节点处理器。
type NodeHandler struct {
	nodeSvc *node.Service
}

// RegisterNodeRoutes 注册节点管理路由（会话 + 管理员双中间件）。
func RegisterNodeRoutes(engine *gin.Engine, h *NodeHandler, sessionMW, adminMW gin.HandlerFunc) {
	admin := engine.Group("/api/admin/nodes", sessionMW, adminMW)
	admin.GET("", h.list)
	admin.POST("", h.create)
	admin.POST("/import", h.importNodes)
	admin.POST("/check", h.check)
	admin.PUT("/:id", h.update)
	admin.DELETE("/:id", h.delete)
	admin.PUT("/:id/toggle", h.toggle)
	admin.PUT("/:id/display-name", h.setDisplayName)
	admin.GET("/protocols", h.protocols)
	admin.GET("/:id", h.get)
}

func (h *NodeHandler) list(c *gin.Context) {
	source := c.Query("source")
	list, err := h.nodeSvc.List(c.Request.Context(), source)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, ListData{List: list, Total: int64(len(list))})
}

func (h *NodeHandler) protocols(c *gin.Context) {
	OK(c, gin.H{"list": h.nodeSvc.GetProtocols()})
}

func (h *NodeHandler) get(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	n, err := h.nodeSvc.Get(c.Request.Context(), id)
	if errors.Is(err, node.ErrNotFound) {
		Fail(c, http.StatusNotFound, "节点不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, n)
}

func (h *NodeHandler) importNodes(c *gin.Context) {
	var req struct {
		Text string `json:"text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	list, err := h.nodeSvc.ImportURIs(c.Request.Context(), req.Text)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, ListData{List: list, Total: int64(len(list))})
}

func (h *NodeHandler) create(c *gin.Context) {
	var req node.CreateManualInput
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	n, err := h.nodeSvc.CreateManual(c.Request.Context(), req)
	if errors.Is(err, node.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, node.ErrConflict) {
		Fail(c, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, n)
}

func (h *NodeHandler) check(c *gin.Context) {
	var req node.CheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	resp, err := h.nodeSvc.Check(c.Request.Context(), req)
	if errors.Is(err, node.ErrNotFound) {
		Fail(c, http.StatusNotFound, "节点不存在")
		return
	}
	if errors.Is(err, node.ErrRevisionConflict) {
		current, _ := node.CurrentRevisionFromError(err)
		c.JSON(http.StatusConflict, gin.H{
			"error":            node.ErrRevisionConflict.Error(),
			"code":             "revision_conflict",
			"current_revision": current,
		})
		return
	}
	if errors.Is(err, node.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, node.ErrForbidden) {
		Fail(c, http.StatusForbidden, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, resp)
}

func (h *NodeHandler) update(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req node.UpdateManualInput
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	n, err := h.nodeSvc.UpdateManual(c.Request.Context(), id, req)
	if errors.Is(err, node.ErrNotFound) {
		Fail(c, http.StatusNotFound, "节点不存在")
		return
	}
	if errors.Is(err, node.ErrRevisionConflict) {
		current, _ := node.CurrentRevisionFromError(err)
		c.JSON(http.StatusConflict, gin.H{
			"error":            node.ErrRevisionConflict.Error(),
			"code":             "revision_conflict",
			"current_revision": current,
		})
		return
	}
	if errors.Is(err, node.ErrBadRequest) || errors.Is(err, node.ErrForbidden) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, n)
}

func (h *NodeHandler) delete(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	err := h.nodeSvc.Delete(c.Request.Context(), id)
	if errors.Is(err, node.ErrNotFound) {
		Fail(c, http.StatusNotFound, "节点不存在")
		return
	}
	if errors.Is(err, node.ErrForbidden) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

func (h *NodeHandler) toggle(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Enabled  *bool `json:"enabled"`
		IsPublic *bool `json:"is_public"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	if req.Enabled == nil && req.IsPublic == nil {
		Fail(c, http.StatusBadRequest, "缺少切换字段")
		return
	}
	var n *node.Node
	var err error
	if req.Enabled != nil {
		n, err = h.nodeSvc.SetEnabled(c.Request.Context(), id, *req.Enabled)
	} else {
		n, err = h.nodeSvc.SetPublic(c.Request.Context(), id, *req.IsPublic)
	}
	if errors.Is(err, node.ErrNotFound) {
		Fail(c, http.StatusNotFound, "节点不存在")
		return
	}
	if errors.Is(err, node.ErrBadRequest) || errors.Is(err, node.ErrForbidden) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, n)
}

func (h *NodeHandler) setDisplayName(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		DisplayName string `json:"display_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	n, err := h.nodeSvc.SetDisplayName(c.Request.Context(), id, req.DisplayName)
	if errors.Is(err, node.ErrNotFound) {
		Fail(c, http.StatusNotFound, "节点不存在")
		return
	}
	if errors.Is(err, node.ErrConflict) {
		Fail(c, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, node.ErrBadRequest) || errors.Is(err, node.ErrForbidden) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, n)
}
