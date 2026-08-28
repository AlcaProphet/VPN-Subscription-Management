package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestResetValidateEndpoint(t *testing.T) {
	srv := newTestServer(t)
	_ = regUser(t, srv, "u1", "u1@example.com", "password123")
	ctx := context.Background()
	var userID int64
	if err := srv.store.DB().QueryRowContext(ctx, `SELECT id FROM users WHERE email='u1@example.com'`).Scan(&userID); err != nil {
		t.Fatalf("查询用户失败: %v", err)
	}
	// 插入一个有效令牌
	if _, err := srv.store.DB().ExecContext(ctx,
		`INSERT INTO password_reset_tokens (token, user_id, expires_at, used) VALUES ('valid-route-token', ?, ?, 0)`,
		userID, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("插入令牌失败: %v", err)
	}

	cases := []struct {
		token string
		want  string
	}{
		{"valid-route-token", "valid"},
		{"missing-route-token", "missing"},
		{"used-route-token", "used"},
		{"expired-route-token", "expired"},
	}
	// 补充 used/expired
	if _, err := srv.store.DB().ExecContext(ctx,
		`INSERT INTO password_reset_tokens (token, user_id, expires_at, used) VALUES ('used-route-token', ?, ?, 1)`,
		userID, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("插入已用令牌失败: %v", err)
	}
	if _, err := srv.store.DB().ExecContext(ctx,
		`INSERT INTO password_reset_tokens (token, user_id, expires_at, used) VALUES ('expired-route-token', ?, ?, 0)`,
		userID, time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("插入过期令牌失败: %v", err)
	}

	for _, tc := range cases {
		w := profileReq(t, srv, http.MethodPost, "/api/auth/reset/validate", "", map[string]string{"token": tc.token})
		if w.Code != http.StatusOK {
			t.Fatalf("%s 状态码异常: %d %s", tc.token, w.Code, w.Body.String())
		}
		var resp struct {
			Code int `json:"code"`
			Data struct {
				Status string `json:"status"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}
		if resp.Code != 0 || resp.Data.Status != tc.want {
			t.Errorf("%s 期望 %s，实际 code=%d status=%s", tc.token, tc.want, resp.Code, resp.Data.Status)
		}
	}
}

func TestResetValidateRateLimit(t *testing.T) {
	srv := newTestServer(t)
	// 默认 10/min；第 11 次应 429。
	for i := 0; i < 10; i++ {
		w := profileReq(t, srv, http.MethodPost, "/api/auth/reset/validate", "", map[string]string{"token": "limit-token"})
		if w.Code != http.StatusOK {
			t.Fatalf("第 %d 次应通过: %d %s", i+1, w.Code, w.Body.String())
		}
	}
	w := profileReq(t, srv, http.MethodPost, "/api/auth/reset/validate", "", map[string]string{"token": "limit-token"})
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("第 11 次应 429: %d %s", w.Code, w.Body.String())
	}
}
