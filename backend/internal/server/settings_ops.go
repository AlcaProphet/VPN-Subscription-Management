// server/settings_ops.go：运维端点（接入层，Build3 Step 4）——一键清空/配置导入导出/备份下载；
// Setup 导入端点仅在未配置状态暴露（无会话保护，依赖导出密码 + 按 IP 限流防在线爆破）。
package server

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/backup"
	"vpn-sub/internal/config"
	"vpn-sub/internal/dataclear"
	"vpn-sub/internal/ratelimit"
	"vpn-sub/internal/setup"
)

// SettingsOpsHandler 运维端点处理器（结构体 Handler + 依赖注入）
type SettingsOpsHandler struct {
	clearSvc  *dataclear.Service
	exportSvc *config.ExportService
	backupSvc *backup.Service
	setupSvc  *setup.Service
	limiter   *ratelimit.Limiter
}

// RegisterSettingsOpsRoutes 注册运维端点；面板端点叠加会话 + 管理员双中间件
func RegisterSettingsOpsRoutes(engine *gin.Engine, h *SettingsOpsHandler, sessionMW, adminMW gin.HandlerFunc) {
	g := engine.Group("/api/admin/settings", sessionMW, adminMW)
	g.POST("/clear_all", h.clearAll) // body: { confirm_word: "RESET" }
	g.POST("/export", h.export)      // body: { password } → 返回加密文件下载
	g.POST("/import", h.importPanel) // multipart 文件 + password + confirm_word=IMPORT
	g.GET("/backup", h.backup)       // tar.gz 流式下载
	// Setup 导入端点：未配置状态暴露，无会话保护——依赖导出密码（Argon2id 高成本）+ 按 IP 限流（同注册口径 5/min）
	engine.POST("/api/setup/import",
		h.limiter.Middleware("setup_import", ratelimit.KeyRegister, 5), h.importSetup)
}

// clearAll 一键清空所有数据（确认词 RESET + 二次确认）
func (h *SettingsOpsHandler) clearAll(c *gin.Context) {
	var req struct {
		ConfirmWord string `json:"confirm_word" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	if err := h.clearSvc.ClearAll(c.Request.Context(), req.ConfirmWord); err != nil {
		Fail(c, http.StatusBadRequest, err.Error()) // 确认词不正确等
		return
	}
	OK(c, gin.H{"message": "系统已重置，即将进入首次配置"})
}

// export 导出加密配置（仅 Production；body 含导出密码 ≥8）
func (h *SettingsOpsHandler) export(c *gin.Context) {
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	data, err := h.exportSvc.Export(c.Request.Context(), req.Password)
	if err != nil {
		if errors.Is(err, config.ErrModeRestricted) {
			Fail(c, http.StatusForbidden, err.Error()) // 仅 Production 提供（R07-06 哨兵映射）
			return
		}
		if errors.Is(err, config.ErrBadRequest) {
			Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		Fail(c, http.StatusForbidden, err.Error())
		return
	}
	c.Header("Content-Disposition", `attachment; filename="vpn-sub-config-`+time.Now().Format("20060102")+`.enc"`)
	c.Data(http.StatusOK, "application/octet-stream", data)
}

// importPanel 面板导入（multipart 文件不设大小上限 + password + confirm_word=IMPORT）
func (h *SettingsOpsHandler) importPanel(c *gin.Context) {
	h.importCommon(c, false)
}

// importSetup Setup 导入（未配置状态暴露；限流已在路由层叠加）
func (h *SettingsOpsHandler) importSetup(c *gin.Context) {
	configured, err := h.setupSvc.IsConfigured(c.Request.Context())
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if configured {
		Fail(c, http.StatusConflict, "系统已完成配置，请使用面板导入")
		return
	}
	h.importCommon(c, true)
}

// importCommon 导入公共处理（multipart：file + password + confirm_word）
func (h *SettingsOpsHandler) importCommon(c *gin.Context, setupMode bool) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		Fail(c, http.StatusBadRequest, "未接收到导入文件")
		return
	}
	defer file.Close()
	// 导入上传文件不设大小上限（设计取舍：正常导出文件 ≤3MB，边界由部署层反代控制，Design1 §3.4.8）
	data := make([]byte, 0)
	buf := make([]byte, 64*1024)
	for {
		n, rerr := file.Read(buf)
		data = append(data, buf[:n]...)
		if rerr != nil {
			break
		}
	}
	password := c.PostForm("password")
	confirmWord := c.PostForm("confirm_word")
	if password == "" || confirmWord == "" {
		Fail(c, http.StatusBadRequest, "导出密码与确认词必填")
		return
	}
	taskID, err := h.exportSvc.ImportV2(c.Request.Context(), data, password, confirmWord, setupMode)
	if err != nil {
		if errors.Is(err, config.ErrModeRestricted) {
			Fail(c, http.StatusForbidden, err.Error()) // 仅 Production 提供（R07-06 哨兵映射，Setup 导入同路径）
			return
		}
		if errors.Is(err, config.ErrBadRequest) {
			Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		Fail(c, http.StatusBadRequest, err.Error()) // 确认词错误/密码错误或文件损坏
		return
	}
	// v2 导入异步返回 task_id；v1 保持同步完成提示。
	if taskID != "" {
		OK(c, gin.H{"task_id": taskID})
		return
	}
	// 导入后效果：签名密钥替换 → 全部会话立即失效（含执行导入的管理员）；含前端地址/回调地址时需重启生效
	OK(c, gin.H{"message": "配置已导入，请立即重启容器后再重新登录"})
}

// backup 备份下载（tar.gz 流式；打包前预检失败时仍返回 500）
func (h *SettingsOpsHandler) backup(c *gin.Context) {
	c.Header("Content-Disposition", `attachment; filename="vpn-sub-backup-`+time.Now().Format("20060102-150405")+`.tar.gz"`)
	c.Header("Content-Type", "application/gzip")
	if err := h.backupSvc.CreateBackup(c.Request.Context(), c.Writer); err != nil {
		// 流式写出后无法再改状态码；快照/打包失败时此处仍返回 500
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
}
