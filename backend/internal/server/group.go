// Package server 用户组端点（接入层）：会话 + 管理员双中间件。
package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/config"
	"vpn-sub/internal/group"
)

// GroupHandler 用户组处理器（结构体 Handler + 依赖注入）
type GroupHandler struct {
	groupSvc *group.Service
	cfg      *config.Service
}

// RegisterGroupRoutes 注册用户组端点；全部叠加会话 + 管理员双中间件。
// 基础 CRUD 不纳入高级模式屏蔽；节点分配/默认配额写入端点由 Build6 追加。
func RegisterGroupRoutes(engine *gin.Engine, h *GroupHandler, sessionMW, adminMW, advancedMW gin.HandlerFunc) {
	admin := engine.Group("/api/admin/groups", sessionMW, adminMW)
	admin.GET("", h.list)    // 组名、默认组标签、默认配额、节点数、组内用户数
	admin.GET("/:id", h.get) // 组详情（advanced_mode=off 时 default_quota 为 nil，JSON 省略）
	admin.POST("", h.create)
	admin.PUT("/:id", h.update)    // 仅改名
	admin.DELETE("/:id", h.delete) // 组内用户迁默认组
	// 高级端点：节点分配与默认配额写入，受 advancedMode 保护
	advanced := admin.Group("", advancedMW)
	advanced.PUT("/:id/nodes", h.setNodes)
	advanced.PUT("/:id/quota", h.setQuota)
}

func (h *GroupHandler) list(c *gin.Context) {
	list, err := h.groupSvc.List(c.Request.Context())
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, ListData{List: list, Total: int64(len(list))})
}

// get 组详情：advanced_mode=on 时返回节点分配与候选集；off 时仅返回基础组信息。
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
	if !h.cfg.GetBool(ctx, config.KeyAdvancedMode, false) {
		OK(c, gin.H{"group": g})
		return
	}
	nodes, err := h.groupSvc.GroupNodes(ctx, id)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	candidates, err := h.groupSvc.CandidateSet(ctx)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{
		"group":           g,
		"nodes":           nodes,
		"candidate_nodes": candidates,
	})
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

func (h *GroupHandler) update(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name" binding:"required,min=1,max=64"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	err := h.groupSvc.Update(c.Request.Context(), id, req.Name)
	if errors.Is(err, group.ErrNameConflict) {
		Fail(c, http.StatusConflict, err.Error())
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

func (h *GroupHandler) setNodes(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		NodeIDs []int64 `json:"node_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	err := h.groupSvc.SetNodes(c.Request.Context(), id, req.NodeIDs)
	if errors.Is(err, group.ErrNotFound) {
		Fail(c, http.StatusNotFound, "用户组不存在")
		return
	}
	if errors.Is(err, group.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

func (h *GroupHandler) setQuota(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		DefaultQuota *float64 `json:"default_quota"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	err := h.groupSvc.SetDefaultQuota(c.Request.Context(), id, req.DefaultQuota)
	if errors.Is(err, group.ErrNotFound) {
		Fail(c, http.StatusNotFound, "用户组不存在")
		return
	}
	if errors.Is(err, group.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}
