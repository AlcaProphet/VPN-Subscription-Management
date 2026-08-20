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
	extSvc      *xray.ExtService
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
	// 独立账号
	admin.GET("/ext", h.listExt)
	admin.POST("/ext", h.createExt)
	admin.GET("/ext/:id", h.getExt)
	admin.PUT("/ext/:id", h.updateExt)
	admin.DELETE("/ext/:id", h.deleteExt)
	admin.GET("/ext/:id/credentials", h.extCredentials)
	admin.POST("/ext/:id/retry", h.extRetry)
	admin.POST("/ext/:id/reset-quota", h.extResetQuota)
	// 实例级对账
	admin.GET("/instances/:id/reconcile", h.reconcile)
	admin.POST("/instances/:id/reconcile/push", h.reconcilePush)
	admin.POST("/instances/:id/reconcile/clean", h.reconcileClean)
	admin.POST("/instances/:id/reconcile/credentials", h.reconcileCredentials)
	admin.POST("/instances/:id/reconcile/push-one", h.reconcilePushOne)
	admin.POST("/instances/:id/reconcile/credentials-one", h.reconcileCredentialsOne)
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


// --- 独立账号端点 ---

func (h *XrayHandler) listExt(c *gin.Context) {
	list, err := h.extSvc.ListExt(c.Request.Context())
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, ListData{List: list, Total: int64(len(list))})
}

func (h *XrayHandler) getExt(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	acc, err := h.extSvc.GetExt(c.Request.Context(), id)
	if errors.Is(err, xray.ErrExtNotFound) {
		Fail(c, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, acc)
}

func (h *XrayHandler) createExt(c *gin.Context) {
	var req struct {
		Name           string               `json:"name" binding:"required"`
		CredentialMode string               `json:"credential_mode" binding:"required,oneof=generate manual"`
		UUID           string               `json:"uuid"`
		ProxySecret    string               `json:"proxy_secret"`
		Quota          *float64             `json:"quota"`
		PushTargets    []xray.ExtPushTarget `json:"push_targets"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	acc, creds, err := h.extSvc.CreateExt(c.Request.Context(), req.Name, req.CredentialMode, req.UUID, req.ProxySecret, req.Quota, req.PushTargets)
	if errors.Is(err, xray.ErrExtConflict) {
		Fail(c, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, xray.ErrBadRequest) || errors.Is(err, xray.ErrAdvancedOff) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if creds != nil {
		OK(c, gin.H{"account": acc, "credentials": creds})
		return
	}
	OK(c, gin.H{"account": acc})
}

func (h *XrayHandler) updateExt(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Name        string               `json:"name" binding:"required"`
		UUID        string               `json:"uuid"`
		ProxySecret string               `json:"proxy_secret"`
		Quota       *float64             `json:"quota"`
		PushTargets []xray.ExtPushTarget `json:"push_targets"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	acc, err := h.extSvc.UpdateExt(c.Request.Context(), id, req.Name, req.UUID, req.ProxySecret, req.Quota, req.PushTargets)
	if errors.Is(err, xray.ErrExtNotFound) {
		Fail(c, http.StatusNotFound, err.Error())
		return
	}
	if errors.Is(err, xray.ErrExtConflict) {
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
	OK(c, acc)
}

func (h *XrayHandler) deleteExt(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	err := h.extSvc.DeleteExt(c.Request.Context(), id)
	if errors.Is(err, xray.ErrExtNotFound) {
		Fail(c, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

func (h *XrayHandler) extCredentials(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	creds, err := h.extSvc.GetExtCredentials(c.Request.Context(), id)
	if errors.Is(err, xray.ErrExtNotFound) {
		Fail(c, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.Header("Cache-Control", "no-store")
	OK(c, creds)
}

func (h *XrayHandler) extRetry(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	result, err := h.extSvc.RetryExt(c.Request.Context(), id)
	if errors.Is(err, xray.ErrExtNotFound) {
		Fail(c, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, result)
}

func (h *XrayHandler) extResetQuota(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	err := h.extSvc.ResetExtQuota(c.Request.Context(), id)
	if errors.Is(err, xray.ErrExtNotFound) {
		Fail(c, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

// --- 实例级对账端点 ---

func (h *XrayHandler) reconcile(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	res, err := h.syncSvc.Reconcile(c.Request.Context(), id)
	if errors.Is(err, xray.ErrInstanceNotFound) {
		Fail(c, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, res)
}

func (h *XrayHandler) reconcilePush(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	taskID, err := h.syncSvc.RepairPushAsync(c.Request.Context(), id)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"task_id": taskID})
}

func (h *XrayHandler) reconcileClean(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Emails []string `json:"emails"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	taskID, err := h.syncSvc.CleanOrphansAsync(c.Request.Context(), id, req.Emails)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"task_id": taskID})
}

func (h *XrayHandler) reconcileCredentials(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Items []xray.ReconcileItem `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	taskID, err := h.syncSvc.RepairCredentialsAsync(c.Request.Context(), id, req.Items)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"task_id": taskID})
}

func (h *XrayHandler) reconcilePushOne(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req xray.ReconcileItem
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	req.InstanceID = id
	if err := h.syncSvc.PushOne(c.Request.Context(), req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, gin.H{"message": "已补推"})
}

func (h *XrayHandler) reconcileCredentialsOne(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req xray.ReconcileItem
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	req.InstanceID = id
	if err := h.syncSvc.CredentialsOne(c.Request.Context(), req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, gin.H{"message": "凭据已修复"})
}
