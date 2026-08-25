// Package server 自定义订阅端点（接入层）：会话 + 管理员双中间件；版本端点复用通用模式。
package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/custom"
	"vpn-sub/internal/version"
)

// CustomHandler 自定义订阅处理器（结构体 Handler + 依赖注入）
type CustomHandler struct {
	customSvc *custom.Service
	verSvc    *version.Service
}

// RegisterCustomRoutes 注册自定义订阅端点；全部叠加会话 + 管理员双中间件
func RegisterCustomRoutes(engine *gin.Engine, h *CustomHandler, sessionMW, adminMW gin.HandlerFunc) {
	admin := engine.Group("/api/admin", sessionMW, adminMW)
	admin.POST("/users/:id/custom", h.upsert)             // 上传/覆盖：multipart 文件/文本双模式（同版本创建模式）
	admin.DELETE("/users/:id/custom/:platform", h.delete) // 按用户+平台删除
	// 版本端点复用通用路由模式（用户管理界面在 Build3，本 Step 预留路由）
	admin.GET("/customs/:id/versions", h.listVersions)
	admin.POST("/customs/:id/versions", h.createVersion)
	admin.PUT("/customs/:id/versions/current", h.switchVersion)
	admin.GET("/customs/:id/versions/:ver/preview", h.previewVersion)
	admin.DELETE("/customs/:id/versions/:ver", h.deleteVersion)
}

// upsert 上传/覆盖自定义订阅：mode=upload 取 multipart 文件流（platform_id 在表单字段）；mode=text 取 JSON 文本体
func (h *CustomHandler) upsert(c *gin.Context) {
	clearReadDeadline(c)
	userID, ok := parseID(c, "id")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	var platformID int64
	var src version.ContentProvider
	if c.Query("mode") == "text" {
		var req struct {
			PlatformID int64  `json:"platform_id" binding:"required"`
			Text       string `json:"text" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, "参数校验失败")
			return
		}
		platformID = req.PlatformID
		src = version.BytesContent([]byte(req.Text))
	} else {
		parsedPlatformID, err := strconv.ParseInt(c.PostForm("platform_id"), 10, 64)
		if err != nil || parsedPlatformID <= 0 {
			Fail(c, http.StatusBadRequest, "platform_id 必填")
			return
		}
		platformID = parsedPlatformID
		file, fileHeader, err := c.Request.FormFile("file")
		if err != nil {
			Fail(c, http.StatusBadRequest, "未接收到文件")
			return
		}
		defer file.Close()
		src = version.ReaderContent{R: file, Max: version.MaxContentSize, Name: fileHeader.Filename}
	}
	sub, err := h.customSvc.Upsert(ctx, userID, platformID, src)
	if errors.Is(err, custom.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, version.ErrContentTooLarge) {
		Fail(c, http.StatusBadRequest, "内容超过 50MB 限制")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, sub)
}

func (h *CustomHandler) delete(c *gin.Context) {
	userID, ok := parseID(c, "id")
	if !ok {
		return
	}
	platformID, ok := parseID(c, "platform")
	if !ok {
		return
	}
	err := h.customSvc.Delete(c.Request.Context(), userID, platformID)
	if errors.Is(err, custom.ErrNotFound) {
		Fail(c, http.StatusNotFound, "自定义订阅不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

// --- 版本端点（复用通用模式，owner_type=custom） ---

func (h *CustomHandler) listVersions(c *gin.Context) {
	versionList(c, h.verSvc, version.OwnerCustom)
}

func (h *CustomHandler) createVersion(c *gin.Context) {
	versionCreate(c, h.verSvc, version.OwnerCustom)
}

func (h *CustomHandler) switchVersion(c *gin.Context) {
	versionSwitch(c, h.verSvc, version.OwnerCustom, nil)
}

func (h *CustomHandler) previewVersion(c *gin.Context) {
	versionPreview(c, h.verSvc, version.OwnerCustom)
}

func (h *CustomHandler) deleteVersion(c *gin.Context) {
	versionDelete(c, h.verSvc, version.OwnerCustom)
}
