package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestPasswordResetHTTPUnavailableSemantics(t *testing.T) {
	srv := newTestServer(t)
	adminToken := regUser(t, srv, "reset-admin", "reset-admin@example.com", "password123")
	res, err := srv.store.DB().Exec(`INSERT INTO users (username, email, role, user_source, status, password_hash)
		VALUES ('reset-target','reset-target@example.com','user','local','active','hash')`)
	if err != nil {
		t.Fatalf("插入目标用户失败: %v", err)
	}
	targetID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.store.DB().Exec(`INSERT INTO users (username, email, role, user_source, status, password_hash)
		VALUES ('batch-target','batch-target@example.com','user','oidc','active',NULL)`); err != nil {
		t.Fatalf("插入批量目标失败: %v", err)
	}

	w := profileReq(t, srv, http.MethodPost, "/api/admin/users/"+strconv.FormatInt(targetID, 10)+"/password/reset",
		adminToken, map[string]string{"mode": "send_email"})
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("邮件不可用应 503: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "密码重置邮件当前不可用") {
		t.Fatalf("不可用文案不符: %s", w.Body.String())
	}
	var tokenCount int
	if err := srv.store.DB().QueryRow(`SELECT COUNT(*) FROM password_reset_tokens WHERE user_id = ?`, targetID).Scan(&tokenCount); err != nil {
		t.Fatalf("统计 token 失败: %v", err)
	}
	if tokenCount != 0 {
		t.Fatalf("不可用不得创建 token: %d", tokenCount)
	}

	w = profileReq(t, srv, http.MethodPost, "/api/admin/users/send_password_links", adminToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("批量不可用应 200 + 计数: %d %s", w.Code, w.Body.String())
	}
	var batch struct {
		Data struct {
			Queued             int `json:"queued"`
			SkippedUnavailable int `json:"skipped_unavailable"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &batch); err != nil {
		t.Fatalf("解析批量响应失败: %v", err)
	}
	if batch.Data.Queued != 0 || batch.Data.SkippedUnavailable != 1 {
		t.Fatalf("批量不可用计数异常: %+v", batch.Data)
	}
	if strings.Contains(w.Body.String(), `"sent"`) {
		t.Fatalf("批量响应不得再包含 sent: %s", w.Body.String())
	}
}

func TestForgotHTTPMessage(t *testing.T) {
	srv := newTestServer(t)
	w := profileReq(t, srv, http.MethodPost, "/api/auth/forgot", "", map[string]string{"email": "ghost@example.com"})
	if w.Code != http.StatusOK {
		t.Fatalf("公共忘记密码应固定 200: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "若该邮箱已注册，重置邮件将发送") {
		t.Fatalf("防枚举文案不符: %s", w.Body.String())
	}
}
