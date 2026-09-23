// openvpn_parse_test.go：Build32 Step 19 `.ovpn` 解析端点的 HTTP 合同测试。
// 覆盖 no-store 必须先于 session/admin 生效、固定状态码、Content-Type、有界输入、
// 零落库，以及日志不含原文或凭据。
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/node"
	"vpn-sub/internal/store"
	"vpn-sub/migrations"
)

// newOpenVPNParseEnv 构建只注册节点路由的测试引擎；lg 非空时注入请求上下文 Logger。
func newOpenVPNParseEnv(t *testing.T, lg *slog.Logger, sessionMW, adminMW gin.HandlerFunc) (*gin.Engine, *store.Store) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), migrations.FS); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	baseLogger := log.New("error", "console")
	cfg := config.NewService(st, baseLogger)
	if err := cfg.Set(context.Background(), config.KeySigningKey, "test-signing-key-0123456789abcdef"); err != nil {
		t.Fatalf("写入签名密钥失败: %v", err)
	}
	nodeSvc := node.NewService(st, cfg, baseLogger)
	if sessionMW == nil {
		sessionMW = func(c *gin.Context) { c.Next() }
	}
	if adminMW == nil {
		adminMW = func(c *gin.Context) { c.Next() }
	}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	if lg != nil {
		engine.Use(loggerContextMiddleware(lg))
	}
	RegisterNodeRoutes(engine, &NodeHandler{nodeSvc: nodeSvc}, sessionMW, adminMW)
	return engine, st
}

// postOpenVPNParse 发送解析请求。
func postOpenVPNParse(engine *gin.Engine, contentType, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/nodes/openvpn/parse", strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func parseRequestBody(t *testing.T, text string) string {
	t.Helper()
	encoded, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		t.Fatalf("序列化解析请求失败: %v", err)
	}
	return string(encoded)
}

func ovpnRouteSample() string {
	return strings.Join([]string{
		"client",
		"dev tun",
		"proto udp",
		"remote vpn.example.com 1194",
		"cipher AES-256-GCM",
		"auth SHA256",
		"auth-user-pass",
		"<ca>",
		"-----BEGIN CERTIFICATE-----",
		"route-ca-body",
		"-----END CERTIFICATE-----",
		"</ca>",
	}, "\n")
}

// TestOpenVPNParseRouteNoStoreBeforeAuth 覆盖 no-store 必须先于 session/admin 生效。
func TestOpenVPNParseRouteNoStoreBeforeAuth(t *testing.T) {
	cases := []struct {
		name     string
		session  gin.HandlerFunc
		admin    gin.HandlerFunc
		wantCode int
	}{
		{
			name:     "anonymous session rejected",
			session:  func(c *gin.Context) { c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401}) },
			admin:    func(c *gin.Context) { c.Next() },
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "non admin rejected",
			session:  func(c *gin.Context) { c.Next() },
			admin:    func(c *gin.Context) { c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403}) },
			wantCode: http.StatusForbidden,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine, _ := newOpenVPNParseEnv(t, nil, tc.session, tc.admin)
			w := postOpenVPNParse(engine, "application/json", parseRequestBody(t, ovpnRouteSample()))
			if w.Code != tc.wantCode {
				t.Fatalf("期望状态码 %d，实际 %d", tc.wantCode, w.Code)
			}
			if got := w.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("%s 的 401／403 响应必须带 no-store，实际 %q", tc.name, got)
			}
		})
	}
}

// TestOpenVPNParseRouteSuccessReturnsDraftWithNoStore 覆盖成功响应的草稿、来源行号与禁止缓存头。
func TestOpenVPNParseRouteSuccessReturnsDraftWithNoStore(t *testing.T) {
	engine, st := newOpenVPNParseEnv(t, nil, nil, nil)
	w := postOpenVPNParse(engine, "application/json; charset=utf-8", parseRequestBody(t, ovpnRouteSample()))
	if w.Code != http.StatusOK {
		t.Fatalf("解析成功响应状态码异常: %d body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("成功响应必须带 no-store，实际 %q", got)
	}
	var response struct {
		Code int                     `json:"code"`
		Data node.OpenVPNParseResult `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("解析响应失败: %v body=%s", err, w.Body.String())
	}
	if response.Code != 0 || response.Data.Host != "vpn.example.com" || response.Data.Port != 1194 {
		t.Fatalf("结构化草稿异常: %+v", response)
	}
	if response.Data.Selectors["auth_mode"] != "userpass" {
		t.Fatalf("认证模式推导异常: %+v", response.Data.Selectors)
	}
	if _, ok := response.Data.FieldSources["remote"]; !ok {
		t.Fatalf("响应必须提供来源行号: %+v", response.Data.FieldSources)
	}
	if strings.Contains(w.Body.String(), `"diagnostics":null`) {
		t.Fatalf("诊断必须序列化为数组: %s", w.Body.String())
	}
	// 解析不得落库。
	var count int
	if err := st.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM nodes`).Scan(&count); err != nil {
		t.Fatalf("读取节点数量失败: %v", err)
	}
	if count != 0 {
		t.Fatalf("解析端点不得写入数据库: count=%d", count)
	}
}

// TestOpenVPNParseRouteBlockedAndOversize 覆盖阻断 400（含稳定 error_code 与诊断）与超限 413。
func TestOpenVPNParseRouteBlockedAndOversize(t *testing.T) {
	engine, _ := newOpenVPNParseEnv(t, nil, nil, nil)

	t.Run("dangerous directive", func(t *testing.T) {
		text := ovpnRouteSample() + "\nup /etc/openvpn/up.sh\n"
		w := postOpenVPNParse(engine, "application/json", parseRequestBody(t, text))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("危险指令必须返回 400，实际 %d body=%s", w.Code, w.Body.String())
		}
		if got := w.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("失败响应必须带 no-store，实际 %q", got)
		}
		var body struct {
			ErrorCode   string                        `json:"error_code"`
			Diagnostics []node.OpenVPNParseDiagnostic `json:"diagnostics"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("解析失败响应失败: %v", err)
		}
		if body.ErrorCode != node.OpenVPNDiagDangerousDirective || len(body.Diagnostics) == 0 {
			t.Fatalf("失败响应必须带稳定 error_code 与诊断: %+v", body)
		}
	})

	t.Run("oversize text", func(t *testing.T) {
		text := strings.Repeat("# padding\n", (node.MaxOpenVPNParseBytes/10)+64)
		w := postOpenVPNParse(engine, "application/json", parseRequestBody(t, text))
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("超过 256 KiB 必须返回 413，实际 %d body=%s", w.Code, w.Body.String())
		}
		if got := w.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("超限响应必须带 no-store，实际 %q", got)
		}
	})

	t.Run("oversize request body", func(t *testing.T) {
		text := strings.Repeat("a", node.MaxOpenVPNParseBytes+(64<<10))
		w := postOpenVPNParse(engine, "application/json", parseRequestBody(t, text))
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("超过请求体上限必须返回 413，实际 %d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("non json content type", func(t *testing.T) {
		w := postOpenVPNParse(engine, "text/plain", parseRequestBody(t, ovpnRouteSample()))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("非 JSON 请求必须返回 400，实际 %d", w.Code)
		}
		if got := w.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("非 JSON 失败响应必须带 no-store，实际 %q", got)
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		w := postOpenVPNParse(engine, "application/json", "{not-json")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("非法 JSON 必须返回 400，实际 %d", w.Code)
		}
	})
}

// TestOpenVPNParseRouteLogsOnlyCountsAndCode 覆盖日志与响应都不含原文凭据。
func TestOpenVPNParseRouteLogsOnlyCountsAndCode(t *testing.T) {
	var buffer bytes.Buffer
	lg := slog.New(slog.NewJSONHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelDebug}))
	engine, _ := newOpenVPNParseEnv(t, lg, nil, nil)

	const caMarker = "route-log-ca-marker"
	const keyMarker = "route-log-tls-key-marker"
	text := strings.Join([]string{
		"client", "dev tun", "remote vpn.example.com 1194",
		"auth-user-pass",
		"<ca>", "-----BEGIN CERTIFICATE-----", caMarker, "-----END CERTIFICATE-----", "</ca>",
		"<tls-auth>", keyMarker, "</tls-auth>",
	}, "\n")
	w := postOpenVPNParse(engine, "application/json", parseRequestBody(t, text))
	if w.Code != http.StatusOK {
		t.Fatalf("解析应成功: %d body=%s", w.Code, w.Body.String())
	}
	logged := buffer.String()
	if logged == "" {
		t.Fatal("解析端点必须记录长度、结果数与 code 级别的结构化日志")
	}
	for _, secret := range []string{caMarker, keyMarker, "auth-user-pass", "vpn.example.com"} {
		if strings.Contains(logged, secret) {
			t.Fatalf("日志不得记录原文或凭据，发现 %q: %s", secret, logged)
		}
	}
	if !strings.Contains(logged, "bytes") || !strings.Contains(logged, "mapped_fields") {
		t.Fatalf("日志必须只记录长度与结果数: %s", logged)
	}

	// 阻断路径同样只记录 code。
	buffer.Reset()
	blocked := ovpnRouteSample() + "\nup /etc/openvpn/route-log-up.sh\n"
	if w := postOpenVPNParse(engine, "application/json", parseRequestBody(t, blocked)); w.Code != http.StatusBadRequest {
		t.Fatalf("危险指令必须 400: %d", w.Code)
	}
	if logged := buffer.String(); strings.Contains(logged, "route-log-up.sh") {
		t.Fatalf("阻断日志不得记录原文: %s", logged)
	}
	if !strings.Contains(buffer.String(), "error_code") {
		t.Fatalf("阻断日志必须记录错误 code: %s", buffer.String())
	}
}
