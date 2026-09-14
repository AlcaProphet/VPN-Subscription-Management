// server/approval.go：审批中心与 SMTP 测试邮件端点（接入层）——会话 + 管理员双中间件。
package server

import (
	"errors"
	"net/http"
	stdmail "net/mail"
	"strings"

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
	g.GET("", h.list) // ?page=&size=
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

// smtpTest 可将测试邮件发送至指定邮箱；留空时使用当前管理员邮箱。
func (h *ApprovalHandler) smtpTest(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
	var req struct {
		To string `json:"to"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		var sizeErr *http.MaxBytesError
		if errors.As(err, &sizeErr) {
			Fail(c, http.StatusRequestEntityTooLarge, "测试邮件请求体过大")
			return
		}
		Fail(c, http.StatusBadRequest, "测试收件人参数无效")
		return
	}
	userID := c.GetInt64(auth.CtxUserID)
	u, err := h.users.GetByID(c.Request.Context(), userID)
	if err != nil || u == nil {
		Fail(c, http.StatusBadRequest, "当前管理员账号不可用")
		return
	}
	to, source, err := resolveSMTPTestRecipient(req.To, u.Email)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.mailSvc.SendTest(c.Request.Context(), to); err != nil {
		Fail(c, http.StatusBadRequest, "发送失败："+err.Error()) // 具体错误供面板展示
		return
	}
	OK(c, gin.H{"message": "测试邮件已发送", "to": to, "recipient_source": source})
}

func resolveSMTPTestRecipient(input, adminEmail string) (string, string, error) {
	to := strings.TrimSpace(input)
	source := "specified"
	if to == "" {
		to = adminEmail
		source = "default"
	}
	if to == "" {
		return "", "", errors.New("当前账号无邮箱，请填写测试收件人")
	}
	parsed, err := stdmail.ParseAddress(to)
	if err != nil || parsed.Address != to || strings.ContainsAny(to, "\r\n,;") {
		return "", "", errors.New("请输入单个有效的收件邮箱")
	}
	return to, source, nil
}
