package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/user"
)

// TestLogStreamAdminRoute 流端点必须走会话+管理员中间件：401/403/管理员可建立。
func TestLogStreamAdminRoute(t *testing.T) {
	srv := newDownloadTestServer(t)
	// 未登录 401
	req := httptest.NewRequest(http.MethodGet, "/api/admin/logs/stream", nil)
	w := httptest.NewRecorder()
	srv.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("未登录流端点应 401: %d", w.Code)
	}

	// 普通用户 403：直接创建 active/user，并由同签名密钥签发测试会话。
	adminToken := regUser(t, srv, "admin", "admin@x.com", "password123")
	hash, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatalf("生成测试密码失败: %v", err)
	}
	res, err := srv.store.DB().Exec(
		`INSERT INTO users (username,email,password_hash,role,user_source,status,credential_version) VALUES ('user','user@x.com',?,'user','local','active',0)`, hash)
	if err != nil {
		t.Fatalf("创建普通用户失败: %v", err)
	}
	userID, _ := res.LastInsertId()
	userSvc := user.NewService(srv.store, srv.cfg, srv.log)
	userToken, _, err := auth.NewService(srv.cfg, userSvc, srv.log).Issue(context.Background(), userID, 0, time.Hour)
	if err != nil {
		t.Fatalf("签发测试会话失败: %v", err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/admin/logs/stream", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	w = httptest.NewRecorder()
	srv.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("普通用户流端点应 403: %d %s", w.Code, w.Body.String())
	}

	// 管理员：用真实 httptest.Server 建立 SSE 后立即取消，避免竞态读取 ResponseRecorder。
	ts := httptest.NewServer(srv.Engine())
	defer ts.Close()
	ctx, cancel := context.WithCancel(context.Background())
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/admin/logs/stream", nil)
	if err != nil {
		t.Fatalf("构造管理员流请求失败: %v", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("管理员 SSE 建立失败: %v", err)
	}
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/event-stream" {
		_ = resp.Body.Close()
		cancel()
		t.Fatalf("管理员应能建立 SSE: code=%d content-type=%s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	cancel()
	_ = resp.Body.Close()
}

// TestLogStreamTokenEndpointRemoved 旧一次性查询 Token 端点必须不存在（404）。
func TestLogStreamTokenEndpointRemoved(t *testing.T) {
	srv := newDownloadTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/logs/stream/token", nil)
	w := httptest.NewRecorder()
	srv.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("旧 stream/token 端点应 404: %d %s", w.Code, w.Body.String())
	}
}
