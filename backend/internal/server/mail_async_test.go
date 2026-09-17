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

// TestRegisterDoesNotWaitForSMTP R32-02：欢迎邮件走派发器后，注册响应必须在 SMTP 仍阻塞时返回。
// 使用“HTTP 响应完成 + SMTP stub 已接受连接但仍未应答”的双通道屏障，避免墙钟阈值在 race/高负载下抖动。
func TestRegisterDoesNotWaitForSMTP(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("启动 SMTP stub 失败: %v", err)
	}
	defer ln.Close()
	acceptedCh := make(chan struct{}, 1)
	connCh := make(chan net.Conn, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		connCh <- conn
		acceptedCh <- struct{}{}
		_, _ = io.Copy(io.Discard, conn) // 不发送 SMTP greeting，保持会话阻塞
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
	respCh := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		srv.Engine().ServeHTTP(w, req)
		respCh <- w
	}()

	var resp *httptest.ResponseRecorder
	accepted := false
	deadline := time.After(10 * time.Second)
	for resp == nil || !accepted {
		select {
		case <-acceptedCh:
			accepted = true
			acceptedCh = nil // 避免关闭后忙轮询
		case w := <-respCh:
			resp = w
		case <-deadline:
			t.Fatal("注册响应未在 SMTP 会话仍阻塞时返回（疑似同步等待 SMTP）")
		}
	}
	if conn := <-connCh; conn != nil {
		_ = conn.Close()
	}
	if resp.Code != http.StatusOK {
		t.Fatalf("注册应成功: %d %s", resp.Code, resp.Body.String())
	}
}
