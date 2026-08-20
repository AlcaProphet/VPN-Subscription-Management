package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/xray"
)

// XrayHandler Xray 实例与节点检测接入层。
type XrayHandler struct {
	instanceSvc *xray.InstanceService
	syncSvc     *xray.SyncService
}

// RegisterXrayRoutes 注册 Xray 管理端点；全部受 advancedMode 保护。
func RegisterXrayRoutes(engine *gin.Engine, h *XrayHandler, sessionMW, adminMW, advancedMW gin.HandlerFunc) {
	admin := engine.Group("/api/admin/xray", sessionMW, adminMW, advancedMW)
	admin.GET("/instances", h.list)
	admin.POST("/instances", h.create)
	admin.GET("/instances/:id", h.get)
	admin.PUT("/instances/:id", h.update)
	admin.DELETE("/instances/:id", h.delete)
	admin.POST("/instances/test", h.test)
	admin.POST("/instances/:id/detect", h.detect)
	admin.POST("/init", h.init)
	admin.GET("/users/:id/sync", h.userSync)
	admin.POST("/users/:id/retry", h.userRetry)
	admin.GET("/instances/:id/stats", h.instanceStats)
	admin.POST("/users/:id/reset-quota", h.resetQuota)
}

func (h *XrayHandler) list(c *gin.Context) {
	list, err := h.instanceSvc.List(c.Request.Context())
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, ListData{List: list, Total: int64(len(list))})
}

func (h *XrayHandler) get(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	inst, err := h.instanceSvc.Get(c.Request.Context(), id)
	if errors.Is(err, xray.ErrInstanceNotFound) {
		Fail(c, http.StatusNotFound, "Xray 实例不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, inst)
}

func (h *XrayHandler) create(c *gin.Context) {
	var req struct {
		Name    string `json:"name" binding:"required"`
		APIAddr string `json:"api_addr" binding:"required"`
		APITag  string `json:"api_tag"`
		Enabled *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	inst, err := h.instanceSvc.Create(c.Request.Context(), req.Name, req.APIAddr, req.APITag, enabled)
	if errors.Is(err, xray.ErrInstanceConflict) {
		Fail(c, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, xray.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, inst)
}

func (h *XrayHandler) update(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Name    string `json:"name" binding:"required"`
		APIAddr string `json:"api_addr" binding:"required"`
		APITag  string `json:"api_tag"`
		Enabled *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	inst, err := h.instanceSvc.Update(c.Request.Context(), id, req.Name, req.APIAddr, req.APITag, enabled)
	if errors.Is(err, xray.ErrInstanceNotFound) {
		Fail(c, http.StatusNotFound, "Xray 实例不存在")
		return
	}
	if errors.Is(err, xray.ErrInstanceConflict) {
		Fail(c, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, xray.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, inst)
}

func (h *XrayHandler) delete(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	taskID, err := h.instanceSvc.DeleteAsync(c.Request.Context(), id)
	if errors.Is(err, xray.ErrInstanceNotFound) {
		Fail(c, http.StatusNotFound, "Xray 实例不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"task_id": taskID})
}

func (h *XrayHandler) test(c *gin.Context) {
	var req struct {
		APIAddr string `json:"api_addr" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	err := h.instanceSvc.TestConnection(c.Request.Context(), req.APIAddr)
	if err != nil {
		Fail(c, http.StatusBadRequest, "连接失败: "+err.Error())
		return
	}
	OK(c, gin.H{"ok": true})
}

func (h *XrayHandler) detect(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	res, err := h.instanceSvc.DetectNodes(c.Request.Context(), id)
	if errors.Is(err, xray.ErrInstanceNotFound) {
		Fail(c, http.StatusNotFound, "Xray 实例不存在")
		return
	}
	if errors.Is(err, xray.ErrDisabled) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, res)
}

func (h *XrayHandler) init(c *gin.Context) {
	taskID, err := h.syncSvc.StartInit(c.Request.Context())
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"task_id": taskID})
}

func (h *XrayHandler) userSync(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	list, err := h.syncSvc.UserSyncStatus(c.Request.Context(), id)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"list": list})
}

func (h *XrayHandler) userRetry(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	result, err := h.syncSvc.RetryUser(c.Request.Context(), id)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, result)
}

func (h *XrayHandler) instanceStats(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	inst, err := h.instanceSvc.Get(c.Request.Context(), id)
	if errors.Is(err, xray.ErrInstanceNotFound) {
		Fail(c, http.StatusNotFound, "Xray 实例不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, inst)
}

func (h *XrayHandler) resetQuota(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.syncSvc.ResetQuota(c.Request.Context(), id); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, nil)
}
