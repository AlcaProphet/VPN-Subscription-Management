package server

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"

	"vpn-sub/internal/config"
)

// sealR3106ImportFile 生成 R31-06 导入拒绝测试用的 v1/v2 加密文件。
func sealR3106ImportFile(t *testing.T, version int, cfg map[string]string) []byte {
	t.Helper()
	plain, err := json.Marshal(config.ExportPayload{FormatVersion: version, Config: cfg})
	if err != nil {
		t.Fatalf("序列化导入文件失败: %v", err)
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		t.Fatalf("生成 salt 失败: %v", err)
	}
	key := argon2.IDKey([]byte("export-pass-123"), salt, 1, 64*1024, 4, 32)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("创建 AES cipher 失败: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("创建 GCM 失败: %v", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		t.Fatalf("生成 nonce 失败: %v", err)
	}
	out := append(salt, nonce...)
	out = gcm.Seal(out, nonce, plain, nil)
	return out
}

// seedR3106ProductionMock 在隔离 Production 库中直接造 mock 历史配置与可被导入覆盖的 app_mode。
func seedR3106ProductionMock(t *testing.T, srv *Server) {
	t.Helper()
	for k, v := range map[string]string{
		config.KeyConfigured:      "true",
		config.KeyAllowLocalLogin: "true",
		config.KeyAppMode:         "dev", // 模拟导入覆盖：Production 运行判断不得信任该值
		"oidc_configured":         "true",
		"oidc_provider_type":      "mock",
		"oidc_params_mock":        "{}",
		"frontend_url":            "https://app.example.com",
	} {
		if _, err := srv.store.DB().Exec(
			`INSERT INTO system_config(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, k, v); err != nil {
			t.Fatalf("写入测试配置 %s 失败: %v", k, err)
		}
	}
}

// TestR3106ProductionStatusAndAdminReadOnly Production 公开状态不得暴露 mock，管理端仍可只读查看并提示。
func TestR3106ProductionStatusAndAdminReadOnly(t *testing.T) {
	srv := newImportTestServer(t)
	seedR3106ProductionMock(t, srv)

	w := doReq(t, srv, http.MethodGet, "/api/system/status")
	if w.Code != http.StatusOK {
		t.Fatalf("系统状态应 200: %d %s", w.Code, w.Body.String())
	}
	var statusResp struct {
		Code int `json:"code"`
		Data struct {
			AppMode          string `json:"app_mode"`
			OIDCConfigured   bool   `json:"oidc_configured"`
			OIDCProviderType string `json:"oidc_provider_type"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("解析系统状态失败: %v", err)
	}
	if statusResp.Data.AppMode != "prod" || statusResp.Data.OIDCConfigured || statusResp.Data.OIDCProviderType != "" {
		t.Fatalf("Production mock 不得公开为可用登录方式: %+v", statusResp.Data)
	}

	// 公开状态拒绝展示不等于删除数据：管理端仍可只读查看历史 mock 并收到切换提示。
	token := regUser(t, srv, "r3106-status-admin", "r3106-status-admin@example.com", "password123")
	w = profileReq(t, srv, http.MethodGet, "/api/admin/settings/oidc", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("管理端 GET OIDC 应 200: %d %s", w.Code, w.Body.String())
	}
	var adminResp struct {
		Data struct {
			ProviderType  string `json:"provider_type"`
			ParamsWarning string `json:"params_warning"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &adminResp); err != nil {
		t.Fatalf("解析管理端 OIDC 响应失败: %v", err)
	}
	if adminResp.Data.ProviderType != "mock" || !strings.Contains(adminResp.Data.ParamsWarning, "生产模式不支持模拟 OIDC") {
		t.Fatalf("管理端应只读保留历史 mock 并提示切换: %+v", adminResp.Data)
	}
}

// TestR3106ProductionSaveSetupAndTestConnectionRejectMock 两个保存入口拒绝 mock；两个测试连接都不得报告通过。
func TestR3106ProductionSaveSetupAndTestConnectionRejectMock(t *testing.T) {
	t.Run("setup save", func(t *testing.T) {
		srv := newImportTestServer(t)
		before := serverSystemConfigSnapshot(t, srv.store)
		body, _ := json.Marshal(map[string]string{"provider_type": "mock"})
		req := httptest.NewRequest(http.MethodPost, "/api/setup/oidc", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.Engine().ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "模拟 OIDC") {
			t.Fatalf("Setup 保存 mock 应 400 并提示不支持: %d %s", w.Code, w.Body.String())
		}
		after := serverSystemConfigSnapshot(t, srv.store)
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("Setup 拒绝后不得写配置:\nbefore=%v\nafter=%v", before, after)
		}
	})

	t.Run("setup test connection", func(t *testing.T) {
		srv := newImportTestServer(t)
		body, _ := json.Marshal(map[string]string{"provider_type": "mock"})
		req := httptest.NewRequest(http.MethodPost, "/api/setup/oidc/test", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.Engine().ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Setup 测试连接应返回 200 失败结果: %d %s", w.Code, w.Body.String())
		}
		var resp struct {
			Data struct {
				OK bool `json:"ok"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("解析 Setup 测试连接响应失败: %v", err)
		}
		if resp.Data.OK {
			t.Fatalf("Production mock 测试连接不得报告通过: %+v", resp.Data)
		}
	})

	t.Run("admin save", func(t *testing.T) {
		srv := newImportTestServer(t)
		token := regUser(t, srv, "r3106-save-admin", "r3106-save-admin@example.com", "password123")
		before := serverSystemConfigSnapshot(t, srv.store)
		w := profileReq(t, srv, http.MethodPut, "/api/admin/settings/oidc", token, map[string]any{
			"provider_type": "mock", "base_url": "", "realm": "", "client_id": "",
			"client_secret": "", "frontend_url": "https://app.example.com", "callback_url": "",
		})
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "模拟 OIDC") {
			t.Fatalf("管理端保存 mock 应 400 并提示不支持: %d %s", w.Code, w.Body.String())
		}
		after := serverSystemConfigSnapshot(t, srv.store)
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("管理端拒绝后不得写配置:\nbefore=%v\nafter=%v", before, after)
		}
	})

	t.Run("admin test connection", func(t *testing.T) {
		srv := newImportTestServer(t)
		token := regUser(t, srv, "r3106-test-admin", "r3106-test-admin@example.com", "password123")
		w := profileReq(t, srv, http.MethodPost, "/api/admin/settings/oidc/test", token, map[string]any{
			"provider_type": "mock", "base_url": "", "realm": "", "client_id": "", "client_secret": "",
		})
		if w.Code != http.StatusOK {
			t.Fatalf("管理端测试连接应返回 200 失败结果: %d %s", w.Code, w.Body.String())
		}
		var resp struct {
			Data struct {
				OK bool `json:"ok"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("解析管理端测试连接响应失败: %v", err)
		}
		if resp.Data.OK {
			t.Fatalf("Production mock 测试连接不得报告通过: %+v", resp.Data)
		}
	})
}

// TestR3106ProductionExistingMockCannotStartOrCompleteLogin 旧库/导入残留 mock 配置在 Production 下不能发起或完成登录。
func TestR3106ProductionExistingMockCannotStartOrCompleteLogin(t *testing.T) {
	t.Run("login initiation", func(t *testing.T) {
		srv := newImportTestServer(t)
		seedR3106ProductionMock(t, srv)
		w := doReq(t, srv, http.MethodGet, "/api/auth/oidc/login")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("Production mock 登录发起应 400，不得 302: %d %s", w.Code, w.Body.String())
		}
		var count int
		if err := srv.store.DB().QueryRow(`SELECT COUNT(*) FROM oidc_states`).Scan(&count); err != nil {
			t.Fatalf("统计 oidc_states 失败: %v", err)
		}
		if count != 0 {
			t.Fatalf("Production mock 拒绝后不得新增 state，实际 %d", count)
		}
	})

	t.Run("bind initiation", func(t *testing.T) {
		srv := newImportTestServer(t)
		seedR3106ProductionMock(t, srv)
		token := regUser(t, srv, "r3106-bind-admin", "r3106-bind-admin@example.com", "password123")
		w := profileReq(t, srv, http.MethodPost, "/api/auth/oidc/bind", token, nil)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("Production mock 绑定发起应 400: %d %s", w.Code, w.Body.String())
		}
		var count int
		if err := srv.store.DB().QueryRow(`SELECT COUNT(*) FROM oidc_states`).Scan(&count); err != nil {
			t.Fatalf("统计 oidc_states 失败: %v", err)
		}
		if count != 0 {
			t.Fatalf("Production mock 拒绝后不得新增 state，实际 %d", count)
		}
	})

	t.Run("mock login", func(t *testing.T) {
		srv := newImportTestServer(t)
		seedR3106ProductionMock(t, srv)
		body, _ := json.Marshal(map[string]any{"email": "r3106@example.com", "email_verified": true})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/oidc/mock/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.Engine().ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("Production mock 登录接口应 400: %d %s", w.Code, w.Body.String())
		}
		var ticketCount int
		if err := srv.store.DB().QueryRow(`SELECT COUNT(*) FROM oidc_login_tickets`).Scan(&ticketCount); err != nil {
			t.Fatalf("统计 ticket 失败: %v", err)
		}
		if ticketCount != 0 {
			t.Fatalf("Production mock 登录不得签发 ticket，实际 %d", ticketCount)
		}
	})

	t.Run("callback exchange", func(t *testing.T) {
		srv := newImportTestServer(t)
		seedR3106ProductionMock(t, srv)
		if _, err := srv.store.DB().Exec(
			`INSERT INTO oidc_states (state, code_verifier, nonce, intent, provider_type, config_hash, redirect_uri)
			 VALUES ('r3106-callback', 'v', 'n', 'login', 'mock', 'hash', 'https://app.example.com/api/auth/oidc/callback')`); err != nil {
			t.Fatalf("写入回调 state 失败: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?state=r3106-callback&code=code", nil)
		req.AddCookie(&http.Cookie{Name: stateCookieName(), Value: "r3106-callback"})
		w := httptest.NewRecorder()
		srv.Engine().ServeHTTP(w, req)
		if w.Code != http.StatusFound || !strings.Contains(w.Header().Get("Location"), "oidc_error=exchange_failed") {
			t.Fatalf("Production mock 回调应 exchange_failed 拒绝: code=%d location=%s", w.Code, w.Header().Get("Location"))
		}
		var ticketCount int
		if err := srv.store.DB().QueryRow(`SELECT COUNT(*) FROM oidc_login_tickets`).Scan(&ticketCount); err != nil {
			t.Fatalf("统计 ticket 失败: %v", err)
		}
		if ticketCount != 0 {
			t.Fatalf("Production mock 回调不得签发 ticket，实际 %d", ticketCount)
		}
	})
}

// TestR3106ProductionLocalAuthCannotLockOutWithMock DB app_mode 被覆盖成 dev 也不能用 mock 关闭本地登录。
func TestR3106ProductionLocalAuthCannotLockOutWithMock(t *testing.T) {
	srv := newImportTestServer(t)
	seedR3106ProductionMock(t, srv)
	token := regUser(t, srv, "r3106-local-admin", "r3106-local-admin@example.com", "password123")
	w := profileReq(t, srv, http.MethodPut, "/api/admin/settings/local-auth", token, map[string]any{
		"allow_local_login": false, "allow_selfreg": false, "selfreg_approval": false,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("本地登录关闭 + Production mock 应 400 防锁死: %d %s", w.Code, w.Body.String())
	}
	var allow string
	if err := srv.store.DB().QueryRow(`SELECT value FROM system_config WHERE key = ?`, config.KeyAllowLocalLogin).Scan(&allow); err != nil {
		t.Fatalf("读取 allow_local_login 失败: %v", err)
	}
	if allow != "true" {
		t.Fatalf("拒绝保存后 allow_local_login 不得变化，实际 %q", allow)
	}
}

// TestR3106HTTPImportsRejectMock v1/v2 的 Setup/管理端导入遇到 mock 类型或 oidc_params_mock 键时整体拒绝且不写库。
func TestR3106HTTPImportsRejectMock(t *testing.T) {
	cases := []struct {
		name    string
		version int
		cfg     map[string]string
	}{
		{
			name:    "生效类型 mock",
			version: 1,
			cfg: map[string]string{
				config.KeyConfigured:      "true",
				config.KeyAllowLocalLogin: "true",
				"oidc_configured":         "true",
				"oidc_provider_type":      "mock",
			},
		},
		{
			name:    "仅空 oidc_params_mock",
			version: config.FormatVersion,
			cfg: map[string]string{
				config.KeyConfigured:      "true",
				config.KeyAllowLocalLogin: "true",
				"oidc_params_mock":        "",
			},
		},
		{
			name:    "生效类型 mock 且本地登录关闭",
			version: config.FormatVersion,
			cfg: map[string]string{
				config.KeyConfigured:      "true",
				config.KeyAllowLocalLogin: "false",
				"oidc_configured":         "false",
				"oidc_provider_type":      "mock",
			},
		},
		{
			name:    "非空 oidc_params_mock 且本地登录关闭",
			version: config.FormatVersion,
			cfg: map[string]string{
				config.KeyConfigured:      "true",
				config.KeyAllowLocalLogin: "false",
				"oidc_configured":         "false",
				"oidc_params_mock":        `{"base_url":"","client_id":""}`,
			},
		},
	}
	for _, tc := range cases {
		data := sealR3106ImportFile(t, tc.version, tc.cfg)
		for _, entry := range []struct {
			name        string
			path        string
			needToken   bool
			disableWord string
		}{
			{name: "setup", path: "/api/setup/import", needToken: false},
			{name: "admin", path: "/api/admin/settings/import", needToken: true, disableWord: config.ConfirmWordDisable},
		} {
			t.Run(tc.name+"/"+entry.name, func(t *testing.T) {
				srv := newImportTestServer(t)
				token := ""
				if entry.needToken {
					token = regUser(t, srv, "r3106-http-import-admin", "r3106-http-import-admin@example.com", "password123")
				}
				before := serverSystemConfigSnapshot(t, srv.store)
				body, ct := multipartR3105ImportBody(t, data, config.ConfirmWordImport, entry.disableWord)
				w := sendImportRequest(t, srv, entry.path, token, body, ct, nil)
				if w.Code != http.StatusBadRequest {
					t.Fatalf("含 mock 配置的导入应 400: %d %s", w.Code, w.Body.String())
				}
				if !strings.Contains(w.Body.String(), "模拟 OIDC") && !strings.Contains(w.Body.String(), "mock") {
					t.Fatalf("拒绝原因应明确指向 mock 配置: %s", w.Body.String())
				}
				if strings.Contains(w.Body.String(), "task_id") {
					t.Fatalf("拒绝导入不得创建异步任务: %s", w.Body.String())
				}
				after := serverSystemConfigSnapshot(t, srv.store)
				if !reflect.DeepEqual(before, after) {
					t.Fatalf("拒绝导入后不得写库:\nbefore=%v\nafter=%v", before, after)
				}
			})
		}
	}
}
