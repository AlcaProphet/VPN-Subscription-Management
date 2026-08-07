// Package server 分享订阅端点（接入层）：会话 + 管理员双中间件；版本端点复用通用模式。
package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/share"
	"vpn-sub/internal/version"
)

// ShareHandler 分享订阅处理器（结构体 Handler + 依赖注入）
type ShareHandler struct {
	shareSvc *share.Service
	verSvc   *version.Service
}

// RegisterShareRoutes 注册分享订阅端点；全部叠加会话 + 管理员双中间件；
// 分享下载走 Step 4 已注册的 GET /share/{slug}/download?token=（公开，无需登录）
func RegisterShareRoutes(engine *gin.Engine, h *ShareHandler, sessionMW, adminMW gin.HandlerFunc) {
	admin := engine.Group("/api/admin/shares", sessionMW, adminMW)
	admin.GET("", h.list)                       // 含 token_status；Token 值仅 active 时返回
	admin.POST("", h.create)                    // 名称 + 首版本（文件/文本）
	admin.PUT("/:id", h.rename)                 // 仅改名
	admin.DELETE("/:id", h.delete)
	admin.POST("/:id/token/refresh", h.refresh) // 轮替（含 revoked 恢复）
	admin.POST("/:id/token/revoke", h.revoke)
	// 版本端点：/api/admin/shares/:id/versions/... 同 custom 模式
	admin.GET("/:id/versions", h.listVersions)
	admin.POST("/:id/versions", h.createVersion)
	admin.PUT("/:id/versions/current", h.switchVersion)
	admin.GET("/:id/versions/:ver/preview", h.previewVersion)
	admin.DELETE("/:id/versions/:ver", h.deleteVersion)
}

func (h *ShareHandler) list(c *gin.Context) {
	list, err := h.shareSvc.List(c.Request.Context())
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, ListData{List: list, Total: int64(len(list))})
}

// create 名称 + 首版本（mode=upload 文件 / mode=text 文本）
func (h *ShareHandler) create(c *gin.Context) {
	ctx := c.Request.Context()
	var name string
	var src version.ContentProvider
	if c.Query("mode") == "text" {
		var req struct {
			Name string `json:"name" binding:"required,min=1,max=100"`
			Text string `json:"text" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, "参数校验失败")
			return
		}
		name = req.Name
		src = version.BytesContent([]byte(req.Text))
	} else {
		name = c.PostForm("name")
		file, _, err := c.Request.FormFile("file")
		if err != nil {
			Fail(c, http.StatusBadRequest, "未接收到文件")
			return
		}
		defer file.Close()
		src = version.ReaderContent{R: file, Max: version.MaxContentSize}
	}
	if name == "" {
		Fail(c, http.StatusBadRequest, "名称必填")
		return
	}
	sh, err := h.shareSvc.Create(ctx, name, src)
	if errors.Is(err, share.ErrBadRequest) {
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
	OK(c, sh)
}

func (h *ShareHandler) rename(c *gin.Context) {
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
	err := h.shareSvc.Rename(c.Request.Context(), id, req.Name)
	if errors.Is(err, share.ErrNotFound) {
		Fail(c, http.StatusNotFound, "分享订阅不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

func (h *ShareHandler) delete(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	err := h.shareSvc.Delete(c.Request.Context(), id)
	if errors.Is(err, share.ErrNotFound) {
		Fail(c, http.StatusNotFound, "分享订阅不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

// refresh 轮替 Token（吊销状态恢复：清标记并新建）
func (h *ShareHandler) refresh(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	tk, err := h.shareSvc.RefreshToken(c.Request.Context(), id)
	if errors.Is(err, share.ErrNotFound) {
		Fail(c, http.StatusNotFound, "分享订阅不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"token": tk})
}

// revoke 吊销 Token：物理删除记录 + 置 revoked，链接立即失效
func (h *ShareHandler) revoke(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	err := h.shareSvc.RevokeToken(c.Request.Context(), id)
	if errors.Is(err, share.ErrNotFound) {
		Fail(c, http.StatusNotFound, "分享订阅不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

// --- 版本端点（复用通用模式，owner_type=share） ---

func (h *ShareHandler) listVersions(c *gin.Context) {
	versionList(c, h.verSvc, version.OwnerShare)
}

func (h *ShareHandler) createVersion(c *gin.Context) {
	versionCreate(c, h.verSvc, version.OwnerShare)
}

func (h *ShareHandler) switchVersion(c *gin.Context) {
	versionSwitch(c, h.verSvc, version.OwnerShare)
}

func (h *ShareHandler) previewVersion(c *gin.Context) {
	versionPreview(c, h.verSvc, version.OwnerShare)
}

func (h *ShareHandler) deleteVersion(c *gin.Context) {
	versionDelete(c, h.verSvc, version.OwnerShare)
}
