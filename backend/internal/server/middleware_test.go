package server

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

func TestAdvancedModeMiddleware(t *testing.T) {
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	defer st.Close()
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
	router := gin.New()
	router.Use(AdvancedMode(cfg))
	router.GET("/ping", func(c *gin.Context) { OK(c, gin.H{"ok": true}) })

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("advanced_mode=off 应 403，实际 %d", w.Code)
	}

	if err := cfg.Set(context.Background(), config.KeyAdvancedMode, "true"); err != nil {
		t.Fatalf("写入 advanced_mode 失败: %v", err)
	}
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/ping", nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("advanced_mode=on 应 200，实际 %d", w2.Code)
	}
}
