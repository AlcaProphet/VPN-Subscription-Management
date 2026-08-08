// server/approval.go：审批中心与 SMTP 测试邮件端点（接入层）——会话 + 管理员双中间件。
package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/approval"
	"vpn-sub/internal/auth"
	"vpn-sub/internal/mail"
	"vpn-sub/internal/user"
)

// ApprovalHandler 审批中心处理器（结构体 Handler + 依赖注入）
type ApprovalHandler struct {
	approvalSvc *approval.Service
	mailSvc     *mail.Service
	users       *user.Service
}

// RegisterApprovalRoutes 注册审批端点；全部叠加会话 + 管理员双中间件；
// SMTP 测试邮件端点（Step 3 面板复用，本 Step 建立）
func RegisterApprovalRoutes(engine *gin.Engine, h *ApprovalHandler, sessionMW, adminMW gin.HandlerFunc) {
	g := engine.Group("/api/admin/approvals", sessionMW, adminMW)
	g.GET("", h.list)                        // ?page=&size=
	g.POST("/:id/approve", h.approve)
	g.POST("/:id/reject", h.reject)
	g.POST("/batch_approve", h.batchApprove) // body: { ids: [] }
	engine.POST("/api/admin/settings/smtp/test", sessionMW, adminMW, h.smtpTest)
}

// list 待审批列表（后端分页）
func (h *ApprovalHandler) list(c *gin.Context) {
	list, total, err := h.approvalSvc.List(c.Request.Context(),
		atoiDefault(c.Query("page"), 1), atoiDefault(c.Query("size"), 20))
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, ListData{List: list, Total: total}) // 分页列表保留统一包裹结构（R02-01）
}

func (h *ApprovalHandler) approve(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.approvalSvc.Approve(c.Request.Context(), id); err != nil {
		if errors.Is(err, approval.ErrNotFound) {
			Fail(c, http.StatusNotFound, "待审批记录不存在")
			return
		}
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

func (h *ApprovalHandler) reject(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.approvalSvc.Reject(c.Request.Context(), id); err != nil {
		if errors.Is(err, approval.ErrNotFound) {
			Fail(c, http.StatusNotFound, "待审批记录不存在")
			return
		}
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

func (h *ApprovalHandler) batchApprove(c *gin.Context) {
	var req struct {
		IDs []int64 `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	succeeded, failed, err := h.approvalSvc.BatchApprove(c.Request.Context(), req.IDs)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"succeeded": succeeded, "failed": failed})
}

// smtpTest 发送测试邮件到当前操作管理员邮箱，失败返回具体错误（供面板展示）
func (h *ApprovalHandler) smtpTest(c *gin.Context) {
	userID := c.GetInt64(auth.CtxUserID)
	u, err := h.users.GetByID(c.Request.Context(), userID)
	if err != nil || u == nil || u.Email == "" {
		Fail(c, http.StatusBadRequest, "当前账号无邮箱，无法发送测试邮件")
		return
	}
	if err := h.mailSvc.SendTest(c.Request.Context(), u.Email); err != nil {
		Fail(c, http.StatusBadRequest, "发送失败："+err.Error()) // 具体错误供面板展示
		return
	}
	OK(c, gin.H{"message": "测试邮件已发送"})
}
