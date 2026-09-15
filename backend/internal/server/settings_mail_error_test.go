package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/mail"
)

// failingMailTemplates 注入存储/领域错误，验证 Handler 500 映射与 no-store。
type failingMailTemplates struct{}

func (failingMailTemplates) ListTemplates(context.Context) ([]mail.TemplateView, error) {
	return nil, errors.New("模拟模板存储读取失败")
}

func (failingMailTemplates) SaveTemplate(context.Context, mail.TemplateKind, mail.Template) (mail.TemplateView, error) {
	return mail.TemplateView{}, errors.New("模拟模板保存失败")
}

func (failingMailTemplates) RestoreTemplate(context.Context, mail.TemplateKind) (mail.TemplateView, error) {
	return mail.TemplateView{}, errors.New("模拟模板恢复失败")
}

func (failingMailTemplates) PreviewTemplate(context.Context, mail.TemplateKind, mail.Template) (mail.Rendered, error) {
	return mail.Rendered{}, errors.New("模拟模板预览失败")
}

func TestMailTemplateInjectedStorageError500(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	sessionMW := func(c *gin.Context) {
		c.Set(auth.CtxUserRole, "admin")
		c.Next()
	}
	adminMW := func(c *gin.Context) { c.Next() }
	RegisterSettingsRoutes(engine, &SettingsHandler{mailTemplates: failingMailTemplates{}}, sessionMW, adminMW)

	for _, tc := range []struct {
		name, method, path, body string
	}{
		{"list", http.MethodGet, "/api/admin/settings/mail-templates", ""},
		{"save", http.MethodPut, "/api/admin/settings/mail-templates/password_reset", `{"subject":"x","body":"{{reset_url}}"}`},
		{"restore", http.MethodDelete, "/api/admin/settings/mail-templates/password_reset", ""},
		{"preview", http.MethodPost, "/api/admin/settings/mail-templates/password_reset/preview", `{"subject":"x","body":"{{reset_url}}"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body io.Reader
			if tc.body != "" {
				body = strings.NewReader(tc.body)
			}
			req := httptest.NewRequest(tc.method, tc.path, body)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)
			if w.Code != http.StatusInternalServerError {
				t.Fatalf("注入错误应 500: %d %s", w.Code, w.Body.String())
			}
			if got := w.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("500 应带 no-store: %q", got)
			}
		})
	}
}
