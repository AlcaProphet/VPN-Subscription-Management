// Package server 用户组端点（接入层）：会话 + 管理员双中间件。
package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/group"
)

// GroupHandler 用户组处理器（结构体 Handler + 依赖注入）
type GroupHandler struct {
	groupSvc *group.Service
}

// RegisterGroupRoutes 注册用户组端点；全部叠加会话 + 管理员双中间件
func RegisterGroupRoutes(engine *gin.Engine, h *GroupHandler, sessionMW, adminMW gin.HandlerFunc) {
	admin := engine.Group("/api/admin/groups", sessionMW, adminMW)
	admin.GET("", h.list) // 组名、关联订阅数、组内用户数、needs_reselect
	admin.GET("/:id", h.get) // 组详情（编辑回显：含当前每平台选定）
	admin.POST("", h.create)
	admin.PUT("/:id", h.update)                   // 改名 + 关联订阅 + 每平台选定（整体提交）
	admin.DELETE("/:id", h.delete)                // 迁入默认组
	admin.PUT("/:id/selections", h.setSelections) // 入参 [{platform_id, subscription_id}]
}

func (h *GroupHandler) list(c *gin.Context) {
	list, err := h.groupSvc.List(c.Request.Context())
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, ListData{List: list, Total: int64(len(list))})
}

// get 组详情（编辑回显：组基础信息 + 当前每平台选定）
func (h *GroupHandler) get(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	g, err := h.groupSvc.Get(ctx, id)
	if errors.Is(err, group.ErrNotFound) {
		Fail(c, http.StatusNotFound, "用户组不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	selections, err := h.groupSvc.Selections(ctx, id)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"group": g, "selections": selections})
}

func (h *GroupHandler) create(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required,min=1,max=64"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	g, err := h.groupSvc.Create(c.Request.Context(), req.Name)
	if errors.Is(err, group.ErrNameConflict) {
		Fail(c, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, g)
}

// updateReq 改名 + 关联订阅多选 + 每平台选定（整体提交）
type updateReq struct {
	Name       string           `json:"name" binding:"required,min=1,max=64"`
	SubIDs     []int64          `json:"sub_ids"`
	Selections []group.Selection `json:"selections"`
}

func (h *GroupHandler) update(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req updateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	err := h.groupSvc.Update(c.Request.Context(), id, req.Name, req.SubIDs, req.Selections)
	if errors.Is(err, group.ErrNameConflict) {
		Fail(c, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, group.ErrSubInSelection) || errors.Is(err, group.ErrSubNotLinked) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, group.ErrNotFound) {
		Fail(c, http.StatusNotFound, "用户组不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

func (h *GroupHandler) delete(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	err := h.groupSvc.Delete(c.Request.Context(), id)
	if errors.Is(err, group.ErrDefaultGroup) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, group.ErrNotFound) {
		Fail(c, http.StatusNotFound, "用户组不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

// setSelections 每平台选定：入参 [{platform_id, subscription_id}]（subscription_id=0 取消选定）
func (h *GroupHandler) setSelections(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Selections []group.Selection `json:"selections" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	err := h.groupSvc.SetSelections(c.Request.Context(), id, req.Selections)
	if errors.Is(err, group.ErrSubNotLinked) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, group.ErrNotFound) {
		Fail(c, http.StatusNotFound, "用户组不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}
