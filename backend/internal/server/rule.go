// Package server 规则端点（接入层）：管理端（会话 + 管理员）+ 用户端（仅会话）。
package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/rule"
	"vpn-sub/internal/subscription"
	"vpn-sub/internal/version"
)

// RuleHandler 规则处理器（结构体 Handler + 依赖注入）
type RuleHandler struct {
	ruleSvc *rule.Service
	verSvc  *version.Service
}

// RegisterRuleRoutes 注册规则端点；管理端叠加会话 + 管理员双中间件，用户端仅会话
func RegisterRuleRoutes(engine *gin.Engine, h *RuleHandler, sessionMW, adminMW gin.HandlerFunc) {
	admin := engine.Group("/api/admin/rules", sessionMW, adminMW)
	admin.GET("", h.list)
	admin.POST("", h.create)
	admin.PUT("/:id", h.rename)
	admin.PUT("/:id/home-default", h.setHomeDefault)
	admin.DELETE("/:id", h.delete)
	admin.POST("/:id/token/refresh", h.refresh)
	// 版本端点：/api/admin/rules/:id/versions/... 同通用模式
	admin.GET("/:id/versions", h.listVersions)
	admin.POST("/:id/versions", h.createVersion)
	admin.PUT("/:id/versions/current", h.switchVersion)
	admin.GET("/:id/versions/:ver/preview", h.previewVersion)
	admin.DELETE("/:id/versions/:ver", h.deleteVersion)

	user := engine.Group("/api/rules", sessionMW) // 用户端：仅会话
	user.GET("", h.userList)                      // 规则卡片列表（登录用户，含全局 Token 供一键导入）
	user.GET("/:id/preview", h.preview)           // 会话凭据预览当前版本（需登录；text/plain + no-store）
}

func (h *RuleHandler) list(c *gin.Context) {
	list, err := h.ruleSvc.List(c.Request.Context())
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, ListData{List: list, Total: int64(len(list))})
}

// create 创建规则：mode=upload 文件 / mode=text 文本（名称+标识+客户端类型+scheme+可选首版本内容）。
// 首版本内容可选：两者都缺省时创建空规则实体（供 SR 分流规则装配目标，Design2 §3.4）。
func (h *RuleHandler) create(c *gin.Context) {
	ctx := c.Request.Context()
	var name, slugVal, clientType string
	var schemes []string
	var src version.ContentProvider
	if c.Query("mode") == "text" {
		var req struct {
			Name       string   `json:"name" binding:"required,min=1,max=100"`
			Slug       string   `json:"slug"` // 可选：为空时后端自动生成（rule- 前缀）
			ClientType string   `json:"client_type" binding:"required"`
			Schemes    []string `json:"schemes"`
			Text       string   `json:"text"` // 可选：留空=创建空规则实体
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, "参数校验失败")
			return
		}
		name, slugVal, clientType, schemes = req.Name, req.Slug, req.ClientType, req.Schemes
		if req.Text != "" {
			src = version.BytesContent([]byte(req.Text))
		}
	} else {
		name = c.PostForm("name")
		slugVal = c.PostForm("slug")
		clientType = c.PostForm("client_type")
		if err := parseFormJSON(c, "schemes", &schemes); err != nil {
			Fail(c, http.StatusBadRequest, "schemes 格式错误")
			return
		}
		if file, fileHeader, err := c.Request.FormFile("file"); err == nil {
			defer file.Close()
			src = version.ReaderContent{R: file, Max: version.MaxContentSize, Name: fileHeader.Filename}
		}
	}
	r, err := h.ruleSvc.Create(ctx, name, slugVal, clientType, schemes, src)
	if errors.Is(err, subscription.ErrSlugConflict) {
		Fail(c, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, rule.ErrBadRequest) {
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
	OK(c, r)
}

func (h *RuleHandler) rename(c *gin.Context) {
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
	err := h.ruleSvc.Rename(c.Request.Context(), id, req.Name)
	if errors.Is(err, rule.ErrNotFound) {
		Fail(c, http.StatusNotFound, "规则不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

// setHomeDefault 设置/取消首页默认规则（至多一条 =1；切换时事务内清旧置新）
func (h *RuleHandler) setHomeDefault(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		IsDefault bool `json:"is_default"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	err := h.ruleSvc.SetHomeDefault(c.Request.Context(), id, req.IsDefault)
	if errors.Is(err, rule.ErrNotFound) {
		Fail(c, http.StatusNotFound, "规则不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

func (h *RuleHandler) delete(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	err := h.ruleSvc.Delete(c.Request.Context(), id)
	if errors.Is(err, rule.ErrNotFound) {
		Fail(c, http.StatusNotFound, "规则不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

func (h *RuleHandler) refresh(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	tk, err := h.ruleSvc.RefreshToken(c.Request.Context(), id)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"token": tk})
}

// userList 用户端规则列表（含全局 Token 供一键导入）
func (h *RuleHandler) userList(c *gin.Context) {
	list, err := h.ruleSvc.List(c.Request.Context())
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, ListData{List: list, Total: int64(len(list))}) // 列表统一包裹结构（AGENTS §4.8）
}

// preview 会话凭据预览当前版本（text/plain + no-store）
func (h *RuleHandler) preview(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	content, err := h.verSvc.ReadCurrent(c.Request.Context(), version.OwnerRule, id)
	if errors.Is(err, version.ErrVersionNotFound) {
		// 空规则实体/无激活版本：HTTP 200 纯文本注释块（Design2 §4.4/§5.10）
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("# error: no active version\n"))
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/plain; charset=utf-8", content)
}

// --- 版本端点（复用通用模式，owner_type=rule） ---

func (h *RuleHandler) listVersions(c *gin.Context) {
	versionList(c, h.verSvc, version.OwnerRule)
}

func (h *RuleHandler) createVersion(c *gin.Context) {
	versionCreate(c, h.verSvc, version.OwnerRule)
}

func (h *RuleHandler) switchVersion(c *gin.Context) {
	versionSwitch(c, h.verSvc, version.OwnerRule)
}

func (h *RuleHandler) previewVersion(c *gin.Context) {
	versionPreview(c, h.verSvc, version.OwnerRule)
}

func (h *RuleHandler) deleteVersion(c *gin.Context) {
	versionDelete(c, h.verSvc, version.OwnerRule)
}

// parseFormJSON 解析 multipart 表单中的 JSON 字段（如 schemes 数组）
func parseFormJSON(c *gin.Context, key string, out any) error {
	raw := c.PostForm(key)
	if raw == "" {
		return nil
	}
	return json.Unmarshal([]byte(raw), out)
}
