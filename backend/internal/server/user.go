// server/user.go：管理员用户管理端点（接入层）——会话 + 管理员双中间件；全部操作执行前校验五重管理员保护。
package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/user"
)

// UserAdminHandler 用户管理处理器（结构体 Handler + 依赖注入）
type UserAdminHandler struct {
	adminSvc *user.AdminService
}

// RegisterUserAdminRoutes 注册用户管理端点；全部叠加会话 + 管理员双中间件
func RegisterUserAdminRoutes(engine *gin.Engine, h *UserAdminHandler, sessionMW, adminMW gin.HandlerFunc) {
	g := engine.Group("/api/admin/users", sessionMW, adminMW)
	g.GET("", h.list)                                // ?page=&size=&keyword=
	g.POST("", h.create)                             // 新建用户
	g.PUT("/:id", h.update)                          // 编辑/换组（body: group_id；email 非空时补填）
	g.PUT("/:id/role", h.changeRole)                 // body: { role: admin|user }
	g.POST("/:id/tokens/revoke", h.revokeTokens)     // 吊销所有下载 Token
	g.POST("/:id/password/reset", h.resetPassword)   // body: { mode: "send_email"|"direct" }
	g.DELETE("/:id/oidc", h.clearOidc)               // 清除 OIDC 绑定
	g.PUT("/:id/status", h.setStatus)                // body: { disabled: bool }
	g.DELETE("/:id", h.delete)                       // 删除用户
	g.POST("/send_password_links", h.batchSendLinks) // 批量发密码设置链接
}

// mapProtectErr 统一保护错误映射：SelfOperation/PendingNotAllowed/参数类 → 400；LastAdmin → 403；
// 邮箱冲突 → 409；用户不存在 → 404；命中返回 true（已响应），未命中返回 false
func mapProtectErr(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, user.ErrSelfOperation), errors.Is(err, user.ErrPendingNotAllowed),
		errors.Is(err, user.ErrNoEmail), errors.Is(err, user.ErrSMTPNotConfigured),
		errors.Is(err, user.ErrBadRequest):
		Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, user.ErrLastAdmin):
		Fail(c, http.StatusForbidden, err.Error())
	case errors.Is(err, user.ErrEmailConflict):
		Fail(c, http.StatusConflict, "该邮箱已被使用")
	case errors.Is(err, user.ErrUserNotFound):
		Fail(c, http.StatusNotFound, err.Error())
	default:
		return false
	}
	return true
}

// list 用户列表（后端分页 + 用户名/邮箱模糊搜索）
func (h *UserAdminHandler) list(c *gin.Context) {
	q := user.ListQuery{
		Page:    atoiDefault(c.Query("page"), 1),
		Size:    atoiDefault(c.Query("size"), 20),
		Keyword: c.Query("keyword"),
	}
	list, total, err := h.adminSvc.List(c.Request.Context(), q)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, ListData{List: list, Total: total}) // 分页列表保留统一包裹结构（R02-01）
}

// create 新建用户（用户名 + 邮箱 + 密码）
func (h *UserAdminHandler) create(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required,min=1,max=64"`
		Email    string `json:"email" binding:"required,max=254"`
		Password string `json:"password" binding:"required,max=128"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	u, err := h.adminSvc.Create(c.Request.Context(), req.Username, req.Email, req.Password)
	if mapProtectErr(c, err) {
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"id": u.ID, "username": u.Username, "email": u.Email})
}

// update 编辑/换组（body: group_id；email 非空时视为补填）
func (h *UserAdminHandler) update(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		GroupID int64  `json:"group_id"`
		Email   string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	if err := h.adminSvc.UpdateGroup(c.Request.Context(), id, req.GroupID); mapProtectErr(c, err) {
		return
	} else if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	// 无邮箱用户可在此补填邮箱（补填后获得设置密码/重置能力）
	if req.Email != "" {
		if err := h.adminSvc.FillEmail(c.Request.Context(), id, req.Email); mapProtectErr(c, err) {
			return
		} else if err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
	}
	OK(c, nil)
}

// changeRole 角色变更（admin↔user；仅可由其他管理员执行；降级级联清显式 Token）
func (h *UserAdminHandler) changeRole(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Role string `json:"role" binding:"required,oneof=admin user"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	operatorID := c.GetInt64(auth.CtxUserID)
	err := h.adminSvc.ChangeRole(c.Request.Context(), operatorID, id, req.Role)
	if mapProtectErr(c, err) {
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

// revokeTokens 吊销所有下载 Token（物理删除，无标记态）
func (h *UserAdminHandler) revokeTokens(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.adminSvc.RevokeAllTokens(c.Request.Context(), id); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

// resetPassword 重置密码：mode=direct → 返回 { password }（仅一次展示）；
// mode=send_email → SMTP 未配置返 400 提示
func (h *UserAdminHandler) resetPassword(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Mode string `json:"mode" binding:"required,oneof=send_email direct"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	operatorID := c.GetInt64(auth.CtxUserID)
	ctx := c.Request.Context()
	if req.Mode == "direct" {
		pwd, err := h.adminSvc.ResetPasswordDirect(ctx, operatorID, id)
		if mapProtectErr(c, err) {
			return
		}
		if err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		OK(c, gin.H{"password": pwd})
		return
	}
	if err := h.adminSvc.ResetPasswordByEmail(ctx, operatorID, id); mapProtectErr(c, err) {
		return
	} else if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"message": "重置邮件已发送"})
}

// clearOidc 清除 OIDC 绑定；返回 has_password 标记供前端显著警告
func (h *UserAdminHandler) clearOidc(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	hasPassword, err := h.adminSvc.ClearOidcBinding(c.Request.Context(), id)
	if mapProtectErr(c, err) {
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"has_password": hasPassword})
}

// setStatus 禁用/启用（body: { disabled: bool }；禁用 = 同事务递增凭据版本号 + 物理删全部 Token）
func (h *UserAdminHandler) setStatus(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Disabled bool `json:"disabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	operatorID := c.GetInt64(auth.CtxUserID)
	err := h.adminSvc.SetStatus(c.Request.Context(), operatorID, id, req.Disabled)
	if mapProtectErr(c, err) {
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

// delete 删除用户（级联 + 五重保护；待审批账号与「拒绝」同效果）
func (h *UserAdminHandler) delete(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	operatorID := c.GetInt64(auth.CtxUserID)
	err := h.adminSvc.Delete(c.Request.Context(), operatorID, id)
	if mapProtectErr(c, err) {
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

// batchSendLinks 为所有无密码用户发送密码设置链接（回执排除范围计数）
func (h *UserAdminHandler) batchSendLinks(c *gin.Context) {
	sent, skippedPending, skippedDisabled, skippedNoEmail, err := h.adminSvc.BatchSendPasswordLinks(c.Request.Context())
	if mapProtectErr(c, err) {
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{
		"sent":             sent,
		"skipped_pending":  skippedPending,
		"skipped_disabled": skippedDisabled,
		"skipped_no_email": skippedNoEmail,
	})
}

// atoiDefault 查询参数整数解析（非法或空 → 默认值）
func atoiDefault(raw string, def int) int {
	if raw == "" {
		return def
	}
	n := 0
	for _, r := range raw {
		if r < '0' || r > '9' {
			return def
		}
		n = n*10 + int(r-'0')
	}
	if n <= 0 {
		return def
	}
	return n
}
