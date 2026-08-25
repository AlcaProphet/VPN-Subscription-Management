// Package server 订阅池端点（接入层）：会话 + 管理员双中间件；版本端点通用模式（四类资源复用）。
package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/subscription"
	"vpn-sub/internal/version"
)

// SubscriptionHandler 订阅处理器（结构体 Handler + 依赖注入）
type SubscriptionHandler struct {
	subSvc *subscription.Service
	verSvc *version.Service
	// onVersionSwitched 订阅版本激活切换后的候选集重算回调（Build6 Step2）
	onVersionSwitched func(ctx context.Context, ot version.OwnerType, ownerID int64)
}

// RegisterSubscriptionRoutes 注册订阅与版本端点；全部叠加会话 + 管理员双中间件
func RegisterSubscriptionRoutes(engine *gin.Engine, h *SubscriptionHandler, sessionMW, adminMW gin.HandlerFunc) {
	admin := engine.Group("/api/admin/subscriptions", sessionMW, adminMW)
	admin.GET("", h.list) // 平铺列表（每平台一份订阅条目）
	admin.POST("", h.create)
	admin.GET("/:id", h.get) // 单条订阅（版本管理页装配入口预填等场景）
	admin.PUT("/:id", h.update)
	admin.DELETE("/:id", h.delete)

	// 版本端点（四类资源通用模式）
	admin.GET("/:id/versions", h.listVersions)
	admin.POST("/:id/versions", h.createVersion)              // 文件上传/文本编辑双模式（multipart 字段 mode=upload|text）
	admin.PUT("/:id/versions/current", h.switchVersion)       // body: { version_no }
	admin.GET("/:id/versions/:ver/preview", h.previewVersion) // text/plain + no-store，仅管理员
	admin.DELETE("/:id/versions/:ver", h.deleteVersion)

	// 标识唯一性即时校验（供前端输入时提示）
	engine.GET("/api/admin/slug/check", sessionMW, adminMW, h.checkSlug) // ?slug=&type=&id=
}

func parseID(c *gin.Context, key string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil || id <= 0 {
		Fail(c, http.StatusBadRequest, "参数错误")
		return 0, false
	}
	return id, true
}

type subCreateReq struct {
	PlatformID int64  `json:"platform_id" binding:"required"`
	Name       string `json:"name" binding:"required,min=1,max=100"`
	Slug       string `json:"slug"` // 可选：为空时后端自动生成（subscription- 前缀）
}

func (h *SubscriptionHandler) list(c *gin.Context) {
	list, err := h.subSvc.List(c.Request.Context())
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, ListData{List: list, Total: int64(len(list))}) // 列表统一包裹结构（AGENTS §4.8）
}

func (h *SubscriptionHandler) get(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	sub, err := h.subSvc.Get(c.Request.Context(), id)
	if errors.Is(err, subscription.ErrNotFound) {
		Fail(c, http.StatusNotFound, "订阅不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, sub)
}

func (h *SubscriptionHandler) create(c *gin.Context) {
	var req subCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	sub, err := h.subSvc.Create(c.Request.Context(), subscription.CreateInput{
		PlatformID: req.PlatformID,
		Name:       req.Name,
		Slug:       req.Slug,
	})
	if errors.Is(err, subscription.ErrPlatformOccupied) {
		Fail(c, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, subscription.ErrSlugConflict) {
		Fail(c, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, subscription.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, sub)
}

func (h *SubscriptionHandler) update(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name" binding:"required,min=1,max=100"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	err := h.subSvc.Update(c.Request.Context(), id, req.Name)
	if errors.Is(err, subscription.ErrNotFound) {
		Fail(c, http.StatusNotFound, "订阅不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

func (h *SubscriptionHandler) delete(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	err := h.subSvc.Delete(c.Request.Context(), id)
	if errors.Is(err, subscription.ErrNotFound) {
		Fail(c, http.StatusNotFound, "订阅不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

// checkSlug 标识唯一性即时校验（?slug=&type=&id=，type/id 供编辑时排除自身）
func (h *SubscriptionHandler) checkSlug(c *gin.Context) {
	slugVal := c.Query("slug")
	ownerType := c.Query("type")
	var ownerID int64
	if v := c.Query("id"); v != "" {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			Fail(c, http.StatusBadRequest, "id 参数必须是数字")
			return
		}
		ownerID = parsed
	}
	available, err := h.subSvc.CheckSlug(c.Request.Context(), slugVal, ownerType, ownerID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"available": available})
}

// --- 版本端点（四类资源通用模式） ---

func (h *SubscriptionHandler) listVersions(c *gin.Context) {
	versionList(c, h.verSvc, version.OwnerSubscription)
}

func (h *SubscriptionHandler) createVersion(c *gin.Context) {
	// 订阅地址池版本：activate=false（仅入池）；文本模式按 product_type 补默认文件名
	versionCreateWith(c, h.verSvc, version.OwnerSubscription, false, func(ctx context.Context, id int64) (string, error) {
		sub, err := h.subSvc.Get(ctx, id)
		if err != nil {
			return "", err
		}
		return subscriptionTextFileName(sub.ProductType), nil
	})
}

func (h *SubscriptionHandler) switchVersion(c *gin.Context) {
	versionSwitch(c, h.verSvc, version.OwnerSubscription, h.onVersionSwitched)
}

func (h *SubscriptionHandler) previewVersion(c *gin.Context) {
	versionPreview(c, h.verSvc, version.OwnerSubscription)
}

func (h *SubscriptionHandler) deleteVersion(c *gin.Context) {
	versionDelete(c, h.verSvc, version.OwnerSubscription)
}

// --- 以下为四类资源共用的版本端点处理器（custom/share/rule 复用，禁止复制粘贴） ---

// versionList 版本列表（当前激活标记以 DB 为准）
func versionList(c *gin.Context, verSvc *version.Service, ot version.OwnerType) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	current, err := verSvc.CurrentNo(ctx, ot, id)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	list, err := verSvc.ListVersions(ctx, ot, id, current)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, ListData{List: list, Total: int64(len(list))}) // 列表统一包裹结构（AGENTS §4.8）
}

// versionCreate 双模式——mode=upload 取 multipart 文件流（ReaderContent，≤50MB，Design1 §6.3）；
// mode=text 取文本体（BytesContent）。rule/share/custom 手动上传/文本编辑保持创建即激活。
func versionCreate(c *gin.Context, verSvc *version.Service, ot version.OwnerType) {
	versionCreateWith(c, verSvc, ot, ot != version.OwnerSubscription, nil)
}

// versionCreateWith 通用版本创建：activate 显式传参；textName 用于订阅文本模式按 product_type 补默认文件名
func versionCreateWith(c *gin.Context, verSvc *version.Service, ot version.OwnerType, activate bool, textName func(ctx context.Context, id int64) (string, error)) {
	clearReadDeadline(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	var src version.ContentProvider
	if c.Query("mode") == "text" {
		var req struct {
			Text string `json:"text" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, "参数校验失败")
			return
		}
		var name string
		if textName != nil {
			n, err := textName(ctx, id)
			if err != nil {
				Fail(c, http.StatusInternalServerError, err.Error())
				return
			}
			name = n
		}
		src = version.TextContent{Text: []byte(req.Text), Name: name}
	} else {
		file, fileHeader, err := c.Request.FormFile("file")
		if err != nil {
			Fail(c, http.StatusBadRequest, "未接收到文件")
			return
		}
		defer file.Close()
		src = version.ReaderContent{R: file, Max: version.MaxContentSize, Name: fileHeader.Filename}
	}
	v, activated, err := verSvc.CreateVersion(ctx, ot, id, src, version.CreateOptions{Activate: activate})
	if errors.Is(err, version.ErrContentTooLarge) {
		Fail(c, http.StatusBadRequest, "内容超过 50MB 限制")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"version_no": v.No, "auto_activated": activated})
}

// subscriptionTextFileName 订阅文本模式默认文件名（按 product_type 区分下发扩展名，Design2 §5.7）
func subscriptionTextFileName(productType string) string {
	switch productType {
	case "subs", "generic-subs":
		return "subscription.txt"
	default:
		return "subscription.yaml"
	}
}

// versionSwitch 切换当前版本（原子切换）
func versionSwitch(c *gin.Context, verSvc *version.Service, ot version.OwnerType, onSwitched func(ctx context.Context, ot version.OwnerType, ownerID int64)) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		VersionNo int64 `json:"version_no" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	ctx := c.Request.Context()
	err := verSvc.SwitchVersion(ctx, ot, id, req.VersionNo)
	if errors.Is(err, version.ErrVersionNotFound) {
		Fail(c, http.StatusNotFound, "版本不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if onSwitched != nil {
		onSwitched(ctx, ot, id)
	}
	OK(c, nil)
}

// versionPreview 禁缓存 + text/plain（AGENTS §4.5 下载类分级）
func versionPreview(c *gin.Context, verSvc *version.Service, ot version.OwnerType) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	ver, ok := parseID(c, "ver")
	if !ok {
		return
	}
	content, err := verSvc.PreviewVersion(c.Request.Context(), ot, id, ver)
	if errors.Is(err, version.ErrVersionNotFound) {
		Fail(c, http.StatusNotFound, "版本不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/plain; charset=utf-8", content)
}

// versionDelete 删除版本（不可删最后/当前）
func versionDelete(c *gin.Context, verSvc *version.Service, ot version.OwnerType) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	ver, ok := parseID(c, "ver")
	if !ok {
		return
	}
	err := verSvc.DeleteVersion(c.Request.Context(), ot, id, ver)
	if errors.Is(err, version.ErrLastVersion) || errors.Is(err, version.ErrCurrentVersion) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, version.ErrVersionNotFound) {
		Fail(c, http.StatusNotFound, "版本不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}
