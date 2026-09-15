package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/mail"
)

// maxMailTemplateBodyBytes 保存/预览接口的请求体硬上限（64 KiB）。
const maxMailTemplateBodyBytes = 64 << 10

// mailTemplateService 管理 API 所需的邮件模板领域服务；由 mail.Service 实现，测试可注入 mock。
type mailTemplateService interface {
	ListTemplates(ctx context.Context) ([]mail.TemplateView, error)
	SaveTemplate(ctx context.Context, kind mail.TemplateKind, t mail.Template) (mail.TemplateView, error)
	RestoreTemplate(ctx context.Context, kind mail.TemplateKind) (mail.TemplateView, error)
	PreviewTemplate(ctx context.Context, kind mail.TemplateKind, t mail.Template) (mail.Rendered, error)
}

// registerMailTemplateRoutes 注册四类模板路由；no-store 必须在 session/admin 之前执行，
// 保证 401/403 等中间件响应也带 no-store。
func registerMailTemplateRoutes(engine *gin.Engine, h *SettingsHandler, sessionMW, adminMW gin.HandlerFunc) {
	g := engine.Group("/api/admin/settings", noStoreMiddleware(), sessionMW, adminMW)
	g.GET("/mail-templates", h.getMailTemplates)
	g.PUT("/mail-templates/:kind", h.putMailTemplate)
	g.DELETE("/mail-templates/:kind", h.deleteMailTemplate)
	g.POST("/mail-templates/:kind/preview", h.previewMailTemplate)
}

func noStoreMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}

// getMailTemplates 返回五个有效模板、limits 元数据与固定合成预览值。
func (h *SettingsHandler) getMailTemplates(c *gin.Context) {
	if h.mailTemplates == nil {
		FailSanitized(c, http.StatusInternalServerError, "邮件模板服务不可用", errors.New("mailTemplates 未注入"))
		return
	}
	templates, err := h.mailTemplates.ListTemplates(c.Request.Context())
	if err != nil {
		FailSanitized(c, http.StatusInternalServerError, "读取邮件模板失败", err)
		return
	}
	preview := mail.PreviewValues()
	OK(c, gin.H{
		"templates": templates,
		"limits": gin.H{
			"subject": mail.MaxSubjectRunes,
			"body":    mail.MaxBodyRunes,
		},
		"preview_values": gin.H{
			"site_name": preview.SiteName,
			"login_url": preview.LoginURL,
			"reset_url": preview.ResetURL,
		},
	})
}

// putMailTemplate 保存单个模板；请求体 64 KiB，未知 kind/非法 JSON/领域校验失败均 400。
func (h *SettingsHandler) putMailTemplate(c *gin.Context) {
	if h.mailTemplates == nil {
		FailSanitized(c, http.StatusInternalServerError, "邮件模板服务不可用", errors.New("mailTemplates 未注入"))
		return
	}
	tpl, err := decodeMailTemplateRequest(c)
	if err != nil {
		mapMailTemplateRequestErr(c, err)
		return
	}
	view, err := h.mailTemplates.SaveTemplate(c.Request.Context(), mail.TemplateKind(c.Param("kind")), tpl)
	if err != nil {
		mapMailTemplateErr(c, err)
		return
	}
	OK(c, view)
}

// deleteMailTemplate 恢复单分支默认（删除目标键）；未知 kind 400，删除失败 500，重复调用幂等。
func (h *SettingsHandler) deleteMailTemplate(c *gin.Context) {
	if h.mailTemplates == nil {
		FailSanitized(c, http.StatusInternalServerError, "邮件模板服务不可用", errors.New("mailTemplates 未注入"))
		return
	}
	view, err := h.mailTemplates.RestoreTemplate(c.Request.Context(), mail.TemplateKind(c.Param("kind")))
	if err != nil {
		mapMailTemplateErr(c, err)
		return
	}
	OK(c, view)
}

// previewMailTemplate 使用固定合成值渲染草稿；绝不写数据库、绝不发送邮件。
func (h *SettingsHandler) previewMailTemplate(c *gin.Context) {
	if h.mailTemplates == nil {
		FailSanitized(c, http.StatusInternalServerError, "邮件模板服务不可用", errors.New("mailTemplates 未注入"))
		return
	}
	tpl, err := decodeMailTemplateRequest(c)
	if err != nil {
		mapMailTemplateRequestErr(c, err)
		return
	}
	rendered, err := h.mailTemplates.PreviewTemplate(c.Request.Context(), mail.TemplateKind(c.Param("kind")), tpl)
	if err != nil {
		mapMailTemplateErr(c, err)
		return
	}
	OK(c, rendered)
}

// decodeMailTemplateRequest 严格解析 {subject, body}，并执行 64 KiB MaxBytesReader 限制。
// 请求体读取失败（含超限）原样返回，便于上层映射 413；解析统一复用 mail.ParseTemplateJSON。
func decodeMailTemplateRequest(c *gin.Context) (mail.Template, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxMailTemplateBodyBytes)
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return mail.Template{}, maxErr
		}
		return mail.Template{}, fmt.Errorf("%w：请求读取失败", mail.ErrInvalidTemplate)
	}
	return mail.ParseTemplateJSON(string(data))
}

func mapMailTemplateRequestErr(c *gin.Context, err error) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		Fail(c, http.StatusRequestEntityTooLarge, "请求体过大")
		return
	}
	Fail(c, http.StatusBadRequest, err.Error())
}

func mapMailTemplateErr(c *gin.Context, err error) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		Fail(c, http.StatusRequestEntityTooLarge, "请求体过大")
		return
	}
	if errors.Is(err, mail.ErrUnknownTemplate) || errors.Is(err, mail.ErrInvalidTemplate) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	FailSanitized(c, http.StatusInternalServerError, "邮件模板操作失败", err)
}
