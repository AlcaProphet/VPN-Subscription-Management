package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
)

// newLimiterEnv 创建临时库 + 限流器
func newLimiterEnv(t *testing.T) (*Limiter, *config.Service) {
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
	gin.SetMode(gin.TestMode)
	return New(cfg, log.New("error", "console")), cfg
}

// doLimited 通过限流中间件发起请求
func doLimited(t *testing.T, l *Limiter, scope, cfgKey string, limit int) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	r.POST("/test", l.Middleware(scope, cfgKey, limit), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestLimiterFixedWindow 固定窗口计数：第 N+1 次 429 + Retry-After 头存在
func TestLimiterFixedWindow(t *testing.T) {
	l, _ := newLimiterEnv(t)
	for i := 0; i < 5; i++ {
		w := doLimited(t, l, "login", KeyLogin, 5)
		if w.Code != http.StatusOK {
			t.Fatalf("第 %d 次应放行: %d", i+1, w.Code)
		}
	}
	// 第 6 次 → 429 + Retry-After
	w := doLimited(t, l, "login", KeyLogin, 5)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("第 6 次应 429: %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("429 应携带 Retry-After 头")
	}
}

// TestLimiterThresholdFromConfig 阈值从配置实时读取（改配置立即生效）
func TestLimiterThresholdFromConfig(t *testing.T) {
	l, cfg := newLimiterEnv(t)
	ctx := context.Background()
	// 默认 5：第 6 次 429
	for i := 0; i < 5; i++ {
		doLimited(t, l, "forgot", KeyForgot, 5)
	}
	if w := doLimited(t, l, "forgot", KeyForgot, 5); w.Code != http.StatusTooManyRequests {
		t.Error("默认阈值下第 6 次应 429")
	}
	// 修改配置为 10 → 立即生效（新窗口继续计数）
	if err := cfg.Set(ctx, KeyForgot, "10"); err != nil {
		t.Fatalf("写配置失败: %v", err)
	}
	for i := 0; i < 10; i++ {
		if w := doLimited(t, l, "forgot2", KeyForgot, 5); w.Code != http.StatusOK {
			t.Fatalf("配置 10 后第 %d 次应放行: %d", i+1, w.Code)
		}
	}
}

// TestLimiterScopeIsolation 不同作用域独立计数
func TestLimiterScopeIsolation(t *testing.T) {
	l, _ := newLimiterEnv(t)
	// login 打满
	for i := 0; i < 5; i++ {
		doLimited(t, l, "login", KeyLogin, 5)
	}
	if w := doLimited(t, l, "login", KeyLogin, 5); w.Code != http.StatusTooManyRequests {
		t.Error("login 作用域应 429")
	}
	// register 作用域不受影响
	if w := doLimited(t, l, "register", KeyRegister, 5); w.Code != http.StatusOK {
		t.Error("register 作用域应独立计数")
	}
}
