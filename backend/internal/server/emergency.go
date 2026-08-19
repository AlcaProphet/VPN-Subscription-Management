// server/emergency.go：应急端点（接入层，Build3 Step 6）——仅在应急模式下注册路由（正常运行时不注册）；
// 应急模式为极低频救援场景且正常服务已暂停，不额外加限流/验证码（Design1 §3.8）。
package server

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/emergency"
)

// EmergencyHandler 应急端点处理器（结构体 Handler + 依赖注入）
type EmergencyHandler struct {
	emSvc *emergency.Service
}

// RegisterEmergencyRoutes 仅在应急模式下调用（main 按 DetectTrigger 结果分支装配）
func RegisterEmergencyRoutes(engine *gin.Engine, h *EmergencyHandler) {
	g := engine.Group("/api/emergency")
	g.POST("/verify", h.verify)                // 校验操作码，通过返回管理员名单（若可重置）
	g.POST("/reset_password", h.resetPassword) // 操作码 + user_id + 新密码
	g.POST("/reinitialize", h.reinitialize)    // 操作码 + 二次确认
}

// verify 校验操作码；通过后返回可用能力与管理员名单（验码前不暴露名单）
func (h *EmergencyHandler) verify(c *gin.Context) {
	var req struct {
		OpCode string `json:"op_code" binding:"required,len=8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	if !h.emSvc.VerifyOpCode(req.OpCode) {
		Fail(c, http.StatusUnauthorized, "操作码已失效，请重新从运行日志获取")
		return
	}
	resp := gin.H{"can_reset_password": h.emSvc.CanResetPassword(c.Request.Context())}
	if resp["can_reset_password"] == true {
		admins, err := h.emSvc.ListAdmins(c.Request.Context()) // 验码通过后才返回名单
		if err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		resp["admins"] = admins
	}
	OK(c, resp)
}

// resetPassword 重置管理员密码；成功后进程退出（exit），由 compose restart 拉起
func (h *EmergencyHandler) resetPassword(c *gin.Context) {
	var req struct {
		OpCode      string `json:"op_code" binding:"required,len=8"`
		UserID      int64  `json:"user_id" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,max=128"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	if !h.emSvc.VerifyOpCode(req.OpCode) {
		Fail(c, http.StatusUnauthorized, "操作码已失效，请重新从运行日志获取")
		return
	}
	if !h.emSvc.CanResetPassword(c.Request.Context()) {
		Fail(c, http.StatusForbidden, "当前环境不支持重置密码")
		return
	}
	if err := h.emSvc.ResetAdminPassword(c.Request.Context(), req.UserID, req.NewPassword); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, gin.H{"message": "密码已重置，进程即将退出重启，请移除 RESET_ADMIN_PASSWORD 环境变量后重启容器"})
	go func() { time.Sleep(500 * time.Millisecond); os.Exit(0) }() // 响应发出后进程退出，compose restart 拉起
}

// reinitialize 重新初始化（应急全清）；成功后进程退出（exit）
func (h *EmergencyHandler) reinitialize(c *gin.Context) {
	var req struct {
		OpCode      string `json:"op_code" binding:"required,len=8"`
		ConfirmText string `json:"confirm" binding:"required"` // 二次确认非空校验（操作码已兼确认词职能，本字段仅防误触）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	if !h.emSvc.VerifyOpCode(req.OpCode) {
		Fail(c, http.StatusUnauthorized, "操作码已失效，请重新从运行日志获取")
		return
	}
	if err := h.emSvc.Reinitialize(c.Request.Context()); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"message": "系统已重新初始化，进程即将退出重启"})
	go func() { time.Sleep(500 * time.Millisecond); os.Exit(0) }()
}
