package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// regUser 注册并登录一个用户，返回会话 token（首管理员）
func regUser(t *testing.T, srv *Server, username, email, password string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": username, "email": email, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("注册失败: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析注册响应失败: %v", err)
	}
	return resp.Data.Token
}

// profileReq 带会话请求
func profileReq(t *testing.T, srv *Server, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	srv.Engine().ServeHTTP(w, req)
	return w
}

// TestProfileEmailBumpsCredentialVersion 改邮箱递增 credential_version：旧会话立即失效 401；新邮箱冲突 409
func TestProfileEmailBumpsCredentialVersion(t *testing.T) {
	srv := newDownloadTestServer(t)
	token := regUser(t, srv, "u1", "u1@x.com", "password123")
	// 改邮箱
	w := profileReq(t, srv, http.MethodPut, "/api/profile/email", token, map[string]string{"email": "u1-new@x.com"})
	if w.Code != http.StatusOK {
		t.Fatalf("改邮箱失败: %d %s", w.Code, w.Body.String())
	}
	// 旧会话失效（credential_version 递增）
	w = profileReq(t, srv, http.MethodGet, "/api/auth/me", token, nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("改邮箱后旧会话应 401: %d", w.Code)
	}
	// 登录拿新会话
	loginBody, _ := json.Marshal(map[string]string{"email": "u1-new@x.com", "password": "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	srv.Engine().ServeHTTP(w2, req)
	if w2.Code != http.StatusOK {
		t.Fatalf("新邮箱登录失败: %d %s", w2.Code, w2.Body.String())
	}
	var loginResp struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &loginResp)
	// 再建用户占用旧邮箱（邮箱已释放）→ 改回旧邮箱冲突 409（直接 SQL 插入，绕开自注册开关）
	if _, err := srv.store.DB().Exec(
		`INSERT INTO users (username, email, role, user_source, status) VALUES ('u2','u1@x.com','user','local','active')`); err != nil {
		t.Fatalf("创建占用用户失败: %v", err)
	}
	w3 := profileReq(t, srv, http.MethodPut, "/api/profile/email", loginResp.Data.Token, map[string]string{"email": "u1@x.com"})
	if w3.Code != http.StatusConflict {
		t.Errorf("占用邮箱应 409: %d", w3.Code)
	}
}

// TestProfilePasswordFlow 修改密码：已设密码需验证当前密码；成功后递增 credential_version（旧会话失效）
func TestProfilePasswordFlow(t *testing.T) {
	srv := newDownloadTestServer(t)
	token := regUser(t, srv, "u1", "u1@x.com", "password123")
	// 错误当前密码 → 400
	w := profileReq(t, srv, http.MethodPut, "/api/profile/password", token,
		map[string]string{"current_password": "wrong-old", "new_password": "newpass456"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("错误当前密码应 400: %d %s", w.Code, w.Body.String())
	}
	// 正确当前密码 → 成功
	w = profileReq(t, srv, http.MethodPut, "/api/profile/password", token,
		map[string]string{"current_password": "password123", "new_password": "newpass456"})
	if w.Code != http.StatusOK {
		t.Fatalf("修改密码失败: %d %s", w.Code, w.Body.String())
	}
	// 旧会话失效
	w = profileReq(t, srv, http.MethodGet, "/api/auth/me", token, nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("改密码后旧会话应 401: %d", w.Code)
	}
	// 新密码可登录
	loginBody, _ := json.Marshal(map[string]string{"email": "u1@x.com", "password": "newpass456"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	srv.Engine().ServeHTTP(w2, req)
	if w2.Code != http.StatusOK {
		t.Errorf("新密码应可登录: %d", w2.Code)
	}
}

// TestProfilePasswordOidcFirstSet OIDC 用户（无密码）首次设置免旧密码
func TestProfilePasswordOidcFirstSet(t *testing.T) {
	srv := newDownloadTestServer(t)
	// 注册用户后清空 password_hash 模拟 OIDC-only 用户
	token := regUser(t, srv, "u1", "u1@x.com", "password123")
	if _, err := srv.store.DB().Exec(`UPDATE users SET password_hash = NULL, user_source = 'oidc', oidc_subject = 'sub-x' WHERE email = 'u1@x.com'`); err != nil {
		t.Fatalf("模拟 OIDC 用户失败: %v", err)
	}
	// 不传 current_password → 应成功（OIDC 首设免旧密码）
	w := profileReq(t, srv, http.MethodPut, "/api/profile/password", token,
		map[string]string{"current_password": "", "new_password": "firstpass123"})
	if w.Code != http.StatusOK {
		t.Fatalf("OIDC 首设密码应免旧密码: %d %s", w.Code, w.Body.String())
	}
}

// TestProfileUsername 改用户名即时生效
func TestProfileUsername(t *testing.T) {
	srv := newDownloadTestServer(t)
	token := regUser(t, srv, "old-name", "u1@x.com", "password123")
	w := profileReq(t, srv, http.MethodPut, "/api/profile/username", token, map[string]string{"username": "new-name"})
	if w.Code != http.StatusOK {
		t.Fatalf("改用户名失败: %d %s", w.Code, w.Body.String())
	}
	w = profileReq(t, srv, http.MethodGet, "/api/auth/me", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("me 失败: %d", w.Code)
	}
	var resp struct {
		Data struct {
			Username string `json:"username"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.Username != "new-name" {
		t.Errorf("用户名应即时生效: %s", resp.Data.Username)
	}
}
