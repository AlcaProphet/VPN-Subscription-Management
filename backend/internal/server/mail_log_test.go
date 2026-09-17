package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vpn-sub/internal/mail"
)

func TestMailActivityLogAPI(t *testing.T) {
	srv := newTestServer(t)
	adminToken := regUser(t, srv, "log-admin", "log-admin@example.com", "password123")

	uid := int64(1)
	okID := srv.activityLog.BeginQueued(mail.TemplateWelcomeLocal, "selfreg", &uid, "full@example.com")
	srv.activityLog.MarkSending(okID)
	srv.activityLog.MarkAccepted(okID)
	failID := srv.activityLog.BeginQueued(mail.TemplatePasswordReset, mail.SourcePublicForgot, &uid, "other@example.com")
	srv.activityLog.MarkQueuedFailed(failID, mail.FailureQueueFull)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/logs/mail", nil)
	w := httptest.NewRecorder()
	srv.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("未鉴权应 401: %d", w.Code)
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("401 也应带 no-store: %q", got)
	}

	w = profileReq(t, srv, http.MethodGet, "/api/admin/logs/mail?page=1&size=20&kind=password_reset&status=failed", adminToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("查询应 200: %d %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("成功响应应带 no-store: %q", got)
	}
	var resp struct {
		Data struct {
			List  []mail.ActivityRecord `json:"list"`
			Total int64                 `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Data.Total != 1 || len(resp.Data.List) != 1 {
		t.Fatalf("过滤结果异常: %+v", resp.Data)
	}
	if resp.Data.List[0].RecipientMasked != "o***@example.com" {
		t.Fatalf("收件人必须掩码: %+v", resp.Data.List[0])
	}
	if resp.Data.List[0].FailureStage == nil || *resp.Data.List[0].FailureStage != mail.FailureQueueFull {
		t.Fatalf("失败阶段异常: %+v", resp.Data.List[0])
	}

	for _, path := range []string{
		"/api/admin/logs/mail?page=0",
		"/api/admin/logs/mail?size=101",
		"/api/admin/logs/mail?kind=unknown",
		"/api/admin/logs/mail?status=unknown",
	} {
		w = profileReq(t, srv, http.MethodGet, path, adminToken, nil)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s 应 400，实际 %d", path, w.Code)
		}
	}

	for i := 0; i < 2; i++ {
		w = profileReq(t, srv, http.MethodPost, "/api/admin/logs/mail/clear", adminToken, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("清空应 200: %d %s", w.Code, w.Body.String())
		}
	}
	w = profileReq(t, srv, http.MethodGet, "/api/admin/logs/mail", adminToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("清空后查询应 200: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"list":[]`) {
		t.Fatalf("空列表必须为 []: %s", w.Body.String())
	}
}
