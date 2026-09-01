// Package server 平台端点（接入层）：会话 + 管理员双中间件；公开安装包下载由 static.go 的 /public 路径承载。
package server

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/config"
	"vpn-sub/internal/platform"
)

// PlatformHandler 平台处理器（结构体 Handler + 依赖注入）
type PlatformHandler struct {
	platformSvc *platform.Service
	cfg         *config.Service
}

// RegisterPlatformRoutes 注册平台管理端点；全部叠加会话 + 管理员双中间件
func RegisterPlatformRoutes(engine *gin.Engine, h *PlatformHandler, sessionMW, adminMW gin.HandlerFunc) {
	admin := engine.Group("/api/admin/platforms", sessionMW, adminMW)
	admin.GET("", h.list)
	admin.POST("", h.create)
	admin.GET("/:id", h.get)
	admin.PUT("/:id", h.update)                                  // slug 只读：不接收 slug 字段
	admin.DELETE("/:id", h.delete)                               // 级联删除，二次确认由前端 ConfirmModal 负责
	admin.POST("/:id/installers", h.uploadInstaller)             // 追加上传一个安装包
	admin.DELETE("/:id/installers/:file", h.deleteInstallerFile) // 按磁盘文件名删一个安装包
	// 公开下载端点：GET /public/installers/<file> 已由 Build1 static.go 承载（可缓存、无需鉴权、不限流、不记访问日志）
}

// parsePlatformID 解析路径中的平台 ID
func parsePlatformID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		Fail(c, http.StatusBadRequest, "参数错误")
		return 0, false
	}
	return id, true
}

// headerValueCI 按 HTTP 头名大小写不敏感方式读取。
func headerValueCI(headers map[string]string, key string) (string, bool) {
	for k, v := range headers {
		if strings.EqualFold(k, key) {
			return v, true
		}
	}
	return "", false
}

// platformReq 创建/编辑入参；slug 一律不接收（创建后不可修改）；外部下载链接列表随平台保存
// （本地安装包由独立上传端点追加，不经本结构）
type platformReq struct {
	Name          string                      `json:"name" binding:"required,min=1,max=100"`
	Description   string                      `json:"description" binding:"max=500"`
	ProductType   string                      `json:"product_type"` // 默认 yaml（空值由业务层补默认）
	Schemes       []string                    `json:"schemes"`
	ExtraHeaders  map[string]string           `json:"extra_headers"`
	InstallerURLs []platform.InstallerURLItem `json:"installer_urls"`
}

func (h *PlatformHandler) list(c *gin.Context) {
	list, err := h.platformSvc.List(c.Request.Context())
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, ListData{List: list, Total: int64(len(list))})
}

func (h *PlatformHandler) get(c *gin.Context) {
	id, ok := parsePlatformID(c)
	if !ok {
		return
	}
	p, err := h.platformSvc.Get(c.Request.Context(), id)
	if errors.Is(err, platform.ErrNotFound) {
		Fail(c, http.StatusNotFound, "平台不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, p)
}

func (h *PlatformHandler) create(c *gin.Context) {
	var req platformReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	ctx := c.Request.Context()
	if h.cfg.GetBool(ctx, config.KeyAdvancedMode, false) {
		for k := range req.ExtraHeaders {
			if platform.IsSystemManagedHeader(k) {
				Fail(c, http.StatusBadRequest, fmt.Sprintf("高级模式下系统接管字段 %q 不可写入", k))
				return
			}
		}
	}
	p, err := h.platformSvc.Create(ctx, req.Name, req.Description, req.ProductType, req.Schemes, req.ExtraHeaders, req.InstallerURLs)
	if errors.Is(err, platform.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, p)
}

func (h *PlatformHandler) update(c *gin.Context) {
	id, ok := parsePlatformID(c)
	if !ok {
		return
	}
	var req platformReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	ctx := c.Request.Context()
	headers := req.ExtraHeaders
	if h.cfg.GetBool(ctx, config.KeyAdvancedMode, false) {
		existing, err := h.platformSvc.Get(ctx, id)
		if err != nil {
			if errors.Is(err, platform.ErrNotFound) {
				Fail(c, http.StatusNotFound, "平台不存在")
			} else {
				Fail(c, http.StatusInternalServerError, err.Error())
			}
			return
		}
		merged := map[string]string{}
		// 高级模式下三个系统接管字段由系统维护：先保留原值，再校验请求中出现的同名字段是否与现有一致。
		for k, v := range existing.ExtraHeaders {
			if platform.IsSystemManagedHeader(k) {
				merged[k] = v
			}
		}
		for k, v := range req.ExtraHeaders {
			if platform.IsSystemManagedHeader(k) {
				cur, found := headerValueCI(existing.ExtraHeaders, k)
				if !found || cur != v {
					Fail(c, http.StatusBadRequest, fmt.Sprintf("高级模式下系统接管字段 %q 不可修改", k))
					return
				}
				// 相同值仅确认，不覆盖原键，避免大小写变体造成重复语义头。
				continue
			}
			merged[k] = v
		}
		headers = merged
	}
	err := h.platformSvc.Update(ctx, id, req.Name, req.Description, req.ProductType, req.Schemes, headers, req.InstallerURLs)
	if errors.Is(err, platform.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, platform.ErrProductTypeInUse) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, platform.ErrNotFound) {
		Fail(c, http.StatusNotFound, "平台不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

func (h *PlatformHandler) delete(c *gin.Context) {
	id, ok := parsePlatformID(c)
	if !ok {
		return
	}
	err := h.platformSvc.Delete(c.Request.Context(), id)
	if errors.Is(err, platform.ErrNotFound) {
		Fail(c, http.StatusNotFound, "平台不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

// uploadInstaller 流式透传 c.Request.Body（限流在业务层 LimitReader，禁止整读内存）；追加上传，返回更新后列表
func (h *PlatformHandler) uploadInstaller(c *gin.Context) {
	clearReadDeadline(c)
	id, ok := parsePlatformID(c)
	if !ok {
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		Fail(c, http.StatusBadRequest, "未接收到文件")
		return
	}
	defer file.Close()
	list, err := h.platformSvc.UploadInstaller(c.Request.Context(), id, file, header.Filename)
	if errors.Is(err, platform.ErrInstallerTooLarge) {
		Fail(c, http.StatusBadRequest, "安装包超过 300MB 限制")
		return
	}
	if errors.Is(err, platform.ErrUnsafeInstallerExt) {
		Fail(c, http.StatusBadRequest, "安装包扩展名不安全，仅允许可下载安装包格式")
		return
	}
	if errors.Is(err, platform.ErrNotFound) {
		Fail(c, http.StatusNotFound, "平台不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, ListData{List: list, Total: int64(len(list))})
}

// deleteInstallerFile 按磁盘文件名删除单个安装包（文件名校验在业务层：必须为基本文件名）
func (h *PlatformHandler) deleteInstallerFile(c *gin.Context) {
	id, ok := parsePlatformID(c)
	if !ok {
		return
	}
	err := h.platformSvc.DeleteInstallerFile(c.Request.Context(), id, c.Param("file"))
	if errors.Is(err, platform.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	if errors.Is(err, platform.ErrNotFound) {
		Fail(c, http.StatusNotFound, "平台不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}
