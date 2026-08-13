package captcha

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
)

// newCaptchaEnv 创建临时库 + 验证码服务
func newCaptchaEnv(t *testing.T) (*Service, *config.Service) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	fsys := fstest.MapFS{
		"0001_init.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY, value TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
	}
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	return NewService(cfg, log.New("error", "console")), cfg
}

// TestEnforcedOff provider=off 时不强制
func TestEnforcedOff(t *testing.T) {
	svc, cfg := newCaptchaEnv(t)
	ctx := context.Background()
	_ = cfg.Set(ctx, KeyProvider, "off")
	if svc.Enforced(ctx, "login") {
		t.Error("provider=off 不应强制")
	}
	_ = cfg.Set(ctx, KeyProvider, "")
	if svc.Enforced(ctx, "login") {
		t.Error("provider 空不应强制")
	}
}

// TestEnforcedPageNotIncluded 页面不在 captcha_pages 时不强制
func TestEnforcedPageNotIncluded(t *testing.T) {
	svc, cfg := newCaptchaEnv(t)
	ctx := context.Background()
	_ = cfg.Set(ctx, KeyProvider, "recaptcha")
	_ = cfg.Set(ctx, KeyPages, `["register"]`)
	_ = cfg.Set(ctx, KeySecretKey, "secret-value")
	if svc.Enforced(ctx, "login") {
		t.Error("login 不在 captcha_pages 不应强制")
	}
	if !svc.Enforced(ctx, "register") {
		t.Error("register 在 captcha_pages 且密钥已配置应强制")
	}
}

// TestEnforcedSecretMissing 密钥缺失时跳过校验（兜底）
func TestEnforcedSecretMissing(t *testing.T) {
	svc, cfg := newCaptchaEnv(t)
	ctx := context.Background()
	_ = cfg.Set(ctx, KeyProvider, "turnstile")
	_ = cfg.Set(ctx, KeyPages, `["forgot"]`)
	// 密钥未配置 → Enforced 返回 false
	if svc.Enforced(ctx, "forgot") {
		t.Error("密钥缺失应跳过校验（兜底）")
	}
}

// TestVerifyEmptyToken 强制页面 + 空 token → 报错
func TestVerifyEmptyToken(t *testing.T) {
	svc, cfg := newCaptchaEnv(t)
	ctx := context.Background()
	_ = cfg.Set(ctx, KeyProvider, "recaptcha")
	_ = cfg.Set(ctx, KeyPages, `["login"]`)
	_ = cfg.Set(ctx, KeySecretKey, "secret-value")
	if err := svc.Verify(ctx, "login", ""); err == nil {
		t.Error("空 token 应报错")
	}
	// 非强制页面 → 直接放行
	if err := svc.Verify(ctx, "forgot", ""); err != nil {
		t.Errorf("非强制页面应放行: %v", err)
	}
}

// mockRoundTripper 拦截 siteverify 请求返回固定结果（避免真实网络调用）
type mockRoundTripper func(*http.Request) (*http.Response, error)

func (f mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// TestMiddlewareBodyReuse 验证码中间件读 body 后处理器仍可正常绑定：
// 中间件与处理器须统一 ShouldBindBodyWithJSON（gin 多次绑定唯一安全姿势，R11 修复）
func TestMiddlewareBodyReuse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, cfg := newCaptchaEnv(t)
	ctx := context.Background()
	_ = cfg.Set(ctx, KeyProvider, "recaptcha")
	_ = cfg.Set(ctx, KeyPages, `["login"]`)
	_ = cfg.Set(ctx, KeySecretKey, "secret-value")
	// mock siteverify：token 视为有效
	svc.httpCli = &http.Client{Transport: mockRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"success":true}`)), Header: make(http.Header)}, nil
	})}
	engine := gin.New()
	engine.POST("/api/auth/login", svc.Middleware("login"), func(c *gin.Context) {
		var req struct {
			Email        string `json:"email"`
			CaptchaToken string `json:"captcha_token"`
		}
		if err := c.ShouldBindBodyWithJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"email": req.Email, "token": req.CaptchaToken})
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/auth/login",
		strings.NewReader(`{"email":"a@b.com","captcha_token":"valid-token"}`))
	r.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("验证码通过后处理器应可绑定 body: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"email":"a@b.com"`) || !strings.Contains(w.Body.String(), `"token":"valid-token"`) {
		t.Errorf("处理器应读到表单与验证码字段: %s", w.Body.String())
	}
}
