package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"vpn-sub/internal/mail"
)

// TestRegisterDoesNotWaitForSMTP R32-02：欢迎邮件走派发器后，注册响应不得等待 SMTP 网络阶段。
func TestRegisterDoesNotWaitForSMTP(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("启动 SMTP stub 失败: %v", err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.Copy(io.Discard, conn)
	}()
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	for k, v := range map[string]string{
		mail.KeyHost: host, mail.KeyPort: port, mail.KeyFrom: "sender@example.com",
		mail.KeySecurity: "plain", mail.KeyAuth: "false", mail.KeyScopes: `["welcome"]`,
		"site_name": "测试站点", "frontend_url": "https://app.example.com",
	} {
		if err := srv.cfg.Set(ctx, k, v); err != nil {
			t.Fatalf("配置 %s 失败: %v", k, err)
		}
	}
	t.Cleanup(srv.mailDispatcher.Stop)

	body, _ := json.Marshal(map[string]string{
		"username": "async-user", "email": "async@example.com", "password": "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	start := time.Now()
	srv.Engine().ServeHTTP(w, req)
	elapsed := time.Since(start)
	if w.Code != http.StatusOK {
		t.Fatalf("注册应成功: %d %s", w.Code, w.Body.String())
	}
	if elapsed > time.Second {
		t.Fatalf("注册响应等待 SMTP 超时: %v", elapsed)
	}
}
