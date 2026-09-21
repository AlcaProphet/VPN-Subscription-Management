package server

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"vpn-sub/internal/mail"
)

func TestMailActivityLogAPI(t *testing.T) {
	srv := newTestServer(t)
	adminToken := regUser(t, srv, "log-admin", "log-admin@example.com", "password123")

	uid := int64(1)
	activeQueuedID := srv.activityLog.BeginQueued(mail.TemplateWelcomeLocal, "selfreg", &uid, "queue@example.com")
	_ = activeQueuedID
	srv.activityLog.BeginSending(mail.TemplateWelcomeOIDC, "oidc", &uid, "sending@example.com")
	okID := srv.activityLog.BeginQueued(mail.TemplateWelcomeLocal, "selfreg", &uid, "full@example.com")
	srv.activityLog.MarkSending(okID)
	srv.activityLog.MarkAccepted(okID)
	failID := srv.activityLog.BeginQueued(mail.TemplatePasswordReset, mail.SourcePublicForgot, &uid, "other@example.com")
	srv.activityLog.MarkQueuedFailed(failID, mail.FailureQueueFull)
	if err := srv.mailResultLog.Flush(t.Context()); err != nil {
		t.Fatalf("等待终态结果写入失败: %v", err)
	}

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
			List  []mail.ResultRecord `json:"list"`
			Total int64               `json:"total"`
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
	if resp.Data.List[0].Result != mail.ActivityFailed {
		t.Fatalf("历史结果只能返回终态: %+v", resp.Data.List[0])
	}

	w = profileReq(t, srv, http.MethodGet, "/api/admin/logs/mail/active", adminToken, nil)
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("当前队列查询异常: %d %s", w.Code, w.Body.String())
	}
	var activeResp struct {
		Data struct {
			List    []mail.ActivityRecord `json:"list"`
			Queued  int                   `json:"queued"`
			Sending int                   `json:"sending"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &activeResp); err != nil {
		t.Fatalf("解析当前队列失败: %v", err)
	}
	if activeResp.Data.Queued != 1 || activeResp.Data.Sending != 1 || len(activeResp.Data.List) != 2 {
		t.Fatalf("active 只能返回 queued/sending: %+v", activeResp.Data)
	}
	if resp.Data.List[0].FailureStage == nil || *resp.Data.List[0].FailureStage != mail.FailureQueueFull {
		t.Fatalf("失败阶段异常: %+v", resp.Data.List[0])
	}

	for _, path := range []string{
		"/api/admin/logs/mail?page=0",
		"/api/admin/logs/mail?size=101",
		"/api/admin/logs/mail?kind=unknown",
		"/api/admin/logs/mail?status=unknown",
		"/api/admin/logs/mail?status=queued",
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
	// 清空历史结果不得影响当前发送队列。
	w = profileReq(t, srv, http.MethodGet, "/api/admin/logs/mail/active", adminToken, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"queued":1`) || !strings.Contains(w.Body.String(), `"sending":1`) {
		t.Fatalf("清空历史结果不得清空当前队列: %d %s", w.Code, w.Body.String())
	}
}

// TestMailActivityLogPaginationBounds 分页边界：大页码不得整数溢出 panic；溢出页码按 400，合法越界页返回空列表。
func TestMailActivityLogPaginationBounds(t *testing.T) {
	srv := newTestServer(t)
	adminToken := regUser(t, srv, "log-bounds-admin", "log-bounds-admin@example.com", "password123")

	for _, path := range []string{
		"/api/admin/logs/mail?page=0&size=20",
		"/api/admin/logs/mail?page=-1&size=20",
		"/api/admin/logs/mail?page=abc&size=20",
		"/api/admin/logs/mail?page=" + "9223372036854775807" + "&size=20",
		"/api/admin/logs/mail?page=" + "9223372036854775807" + "&size=100",
	} {
		w := profileReq(t, srv, http.MethodGet, path, adminToken, nil)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s 应 400，实际 %d %s", path, w.Code, w.Body.String())
		}
	}

	// 该 maxPage 精确等于 MaxInt/size 的边界值；start 不会溢出，应返回空列表而非 400。
	maxPage := math.MaxInt / 100
	w := profileReq(t, srv, http.MethodGet, "/api/admin/logs/mail?page="+strconv.Itoa(maxPage)+"&size=100", adminToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("合法大页码应 200，实际 %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"list":[]`) {
		t.Fatalf("合法大页码应返回空列表: %s", w.Body.String())
	}

	// 普通越界页仍返回空列表（不因“超出总数”误报 400）。
	w = profileReq(t, srv, http.MethodGet, "/api/admin/logs/mail?page=2&size=20", adminToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("普通越界页应 200，实际 %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"list":[]`) {
		t.Fatalf("普通越界页应返回空列表: %s", w.Body.String())
	}
}
