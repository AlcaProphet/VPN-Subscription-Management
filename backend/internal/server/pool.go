// Package server 规则素材池端点（接入层）：会话 + 管理员双中间件；同步异步任务 + 轮询。
package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/pool"
)

// PoolHandler 规则素材池处理器
type PoolHandler struct {
	poolSvc *pool.Service
}

// RegisterPoolRoutes 注册素材池管理端点
func RegisterPoolRoutes(engine *gin.Engine, h *PoolHandler, sessionMW, adminMW gin.HandlerFunc) {
	admin := engine.Group("/api/admin/pools", sessionMW, adminMW)
	admin.GET("", h.list)
	admin.POST("", h.create)
	admin.PUT("/:id", h.update)
	admin.DELETE("/:id", h.delete)

	admin.GET("/:id/entries", h.listEntries)
	admin.POST("/:id/entries", h.createEntry)
	admin.PUT("/:id/entries/:entryId", h.updateEntry)
	admin.DELETE("/:id/entries/:entryId", h.deleteEntry)

	admin.POST("/:id/sync", h.submitSync)
	admin.GET("/:id/sync/status", h.syncStatus)
	admin.GET("/:id/sync/tasks", h.listSyncTasks)
	admin.POST("/:id/sync/tasks/:taskId/cancel", h.cancelSync)
	admin.DELETE("/:id/sync/tasks/completed", h.clearSyncTasks)

	admin.POST("/:id/sources/:sourceId/pending/:snapshotId/activate", h.activatePending)
	admin.DELETE("/:id/sources/:sourceId/pending/:snapshotId", h.discardPending)
}

type poolReq struct {
	Name     string             `json:"name" binding:"required,min=1,max=100"`
	Sources  []pool.SourceInput `json:"sources"`
	URLs     []string           `json:"urls,omitempty"`
	AutoSync bool               `json:"auto_sync"`
	SyncTime string             `json:"sync_time"`
}

func toSources(req poolReq) []pool.SourceInput {
	if len(req.Sources) > 0 {
		return req.Sources
	}
	// 兼容旧 urls 字段
	out := make([]pool.SourceInput, 0, len(req.URLs))
	for _, u := range req.URLs {
		out = append(out, pool.SourceInput{URL: u, SourceMode: pool.SourceModeAuto})
	}
	return out
}

func (h *PoolHandler) list(c *gin.Context) {
	list, err := h.poolSvc.List(c.Request.Context())
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, ListData{List: list, Total: int64(len(list))})
}

func (h *PoolHandler) create(c *gin.Context) {
	var req poolReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	p, err := h.poolSvc.Create(c.Request.Context(), req.Name, toSources(req), req.AutoSync, req.SyncTime)
	if errors.Is(err, pool.ErrNameConflict) {
		Fail(c, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, pool.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, p)
}

func (h *PoolHandler) update(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req poolReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	err := h.poolSvc.Update(c.Request.Context(), id, req.Name, toSources(req), req.AutoSync, req.SyncTime)
	if errors.Is(err, pool.ErrNotFound) {
		Fail(c, http.StatusNotFound, "素材池不存在")
		return
	}
	if errors.Is(err, pool.ErrNameConflict) {
		Fail(c, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, pool.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

func (h *PoolHandler) delete(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	err := h.poolSvc.Delete(c.Request.Context(), id)
	if errors.Is(err, pool.ErrNotFound) {
		Fail(c, http.StatusNotFound, "素材池不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

func pagination(c *gin.Context) (int64, int64) {
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 64)
	size, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 64)
	return page, size
}

func (h *PoolHandler) listEntries(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	page, size := pagination(c)
	list, total, err := h.poolSvc.ListEntries(c.Request.Context(), id, page, size, c.Query("source"))
	if errors.Is(err, pool.ErrNotFound) {
		Fail(c, http.StatusNotFound, "素材池不存在")
		return
	}
	if errors.Is(err, pool.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, ListData{List: list, Total: total})
}

func (h *PoolHandler) createEntry(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		RuleType   string `json:"rule_type" binding:"required"`
		MatchValue string `json:"match_value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	e, err := h.poolSvc.CreateEntry(c.Request.Context(), id, req.RuleType, req.MatchValue)
	if errors.Is(err, pool.ErrEntryConflict) {
		Fail(c, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, pool.ErrNotFound) {
		Fail(c, http.StatusNotFound, "素材池不存在")
		return
	}
	if errors.Is(err, pool.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, e)
}

func (h *PoolHandler) updateEntry(c *gin.Context) {
	entryID, ok := parseID(c, "entryId")
	if !ok {
		return
	}
	var req struct {
		RuleType   string `json:"rule_type" binding:"required"`
		MatchValue string `json:"match_value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	err := h.poolSvc.UpdateEntry(c.Request.Context(), entryID, req.RuleType, req.MatchValue)
	if errors.Is(err, pool.ErrEntryConflict) {
		Fail(c, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, pool.ErrNotFound) {
		Fail(c, http.StatusNotFound, "条目不存在")
		return
	}
	if errors.Is(err, pool.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

func (h *PoolHandler) deleteEntry(c *gin.Context) {
	entryID, ok := parseID(c, "entryId")
	if !ok {
		return
	}
	err := h.poolSvc.DeleteEntry(c.Request.Context(), entryID)
	if errors.Is(err, pool.ErrNotFound) {
		Fail(c, http.StatusNotFound, "条目不存在")
		return
	}
	if errors.Is(err, pool.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

func (h *PoolHandler) submitSync(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	taskID, err := h.poolSvc.SubmitSync(c.Request.Context(), id)
	if errors.Is(err, pool.ErrSyncRunning) {
		Fail(c, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, pool.ErrNotFound) {
		Fail(c, http.StatusNotFound, "素材池不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"task_id": taskID})
}

func (h *PoolHandler) cancelSync(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	taskID, ok := parseID(c, "taskId")
	if !ok {
		return
	}
	err := h.poolSvc.CancelSync(c.Request.Context(), id, taskID)
	if errors.Is(err, pool.ErrNotFound) {
		Fail(c, http.StatusNotFound, "同步任务不存在")
		return
	}
	if errors.Is(err, pool.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"task_id": taskID, "status": "cancel_requested"})
}

func (h *PoolHandler) clearSyncTasks(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	n, err := h.poolSvc.ClearFinishedTasks(c.Request.Context(), id)
	if errors.Is(err, pool.ErrNotFound) {
		Fail(c, http.StatusNotFound, "素材池不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"cleared": n})
}

func (h *PoolHandler) syncStatus(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	t, err := h.poolSvc.GetStatus(c.Request.Context(), id)
	if errors.Is(err, pool.ErrNotFound) {
		Fail(c, http.StatusNotFound, "素材池不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if t == nil {
		OK(c, gin.H{"task_id": 0, "status": "", "per_url": []pool.PerURLResult{}, "error": ""})
		return
	}
	OK(c, t)
}

func (h *PoolHandler) listSyncTasks(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	page, size := pagination(c)
	list, total, err := h.poolSvc.ListTasks(c.Request.Context(), id, page, size)
	if errors.Is(err, pool.ErrNotFound) {
		Fail(c, http.StatusNotFound, "素材池不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, ListData{List: list, Total: total})
}

func (h *PoolHandler) activatePending(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	sourceID, ok := parseID(c, "sourceId")
	if !ok {
		return
	}
	snapshotID, ok := parseID(c, "snapshotId")
	if !ok {
		return
	}
	err := h.poolSvc.ActivatePending(c.Request.Context(), id, sourceID, snapshotID)
	if errors.Is(err, pool.ErrNotFound) {
		Fail(c, http.StatusNotFound, "pending 快照不存在")
		return
	}
	if errors.Is(err, pool.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

func (h *PoolHandler) discardPending(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	sourceID, ok := parseID(c, "sourceId")
	if !ok {
		return
	}
	snapshotID, ok := parseID(c, "snapshotId")
	if !ok {
		return
	}
	err := h.poolSvc.DiscardPending(c.Request.Context(), id, sourceID, snapshotID)
	if errors.Is(err, pool.ErrNotFound) {
		Fail(c, http.StatusNotFound, "pending 快照不存在")
		return
	}
	if errors.Is(err, pool.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}
