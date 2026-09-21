package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/config"
	"vpn-sub/internal/dataclear"
	"vpn-sub/internal/log"
	"vpn-sub/internal/mail"
	"vpn-sub/internal/store"
	"vpn-sub/internal/user"
	"vpn-sub/migrations"
)

// newFullMigrationServer 为生命周期 smoke 使用真实全量迁移，避免最小测试 schema 缺表。
func newFullMigrationServer(t *testing.T) *Server {
	t.Helper()
	dataDir := t.TempDir()
	st, err := store.Open(dataDir, "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), migrations.FS); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	lg := log.New("error", "console")
	cfg := config.NewService(st, lg)
	if err := cfg.Set(context.Background(), "debug_mode", "true"); err != nil {
		t.Fatalf("设置调试模式失败: %v", err)
	}
	users := user.NewService(st, cfg, lg)
	streamSvc := log.NewStreamService(log.NewRingBuffer(), lg)
	srv, err := New(st, cfg, users, log.NewRuntime("error", "console"), "dev", mustPolicy(t, "off"), "0", dataDir, streamSvc)
	if err != nil {
		t.Fatalf("装配 server 失败: %v", err)
	}
	t.Cleanup(srv.mailDispatcher.Stop)
	return srv
}

// TestClearAllPausesAndResumesDispatcher 清空成功：短期日志清空、派发器恢复并保持可用。
func TestClearAllPausesAndResumesDispatcher(t *testing.T) {
	srv := newFullMigrationServer(t)
	adminToken := regUser(t, srv, "clear-admin", "clear-admin@example.com", "password123")
	uid := int64(1)
	srv.activityLog.BeginQueued(mail.TemplateWelcomeLocal, "selfreg", &uid, "old@example.com")

	w := profileReq(t, srv, http.MethodPost, "/api/admin/settings/clear_all", adminToken,
		map[string]string{"confirm_word": "RESET"})
	if w.Code != http.StatusOK {
		t.Fatalf("清空应 200: %d %s", w.Code, w.Body.String())
	}
	if got := len(srv.activityLog.Snapshot()); got != 0 {
		t.Fatalf("清空成功后短期日志应为空，实际 %d", got)
	}
	// 系统已回未配置状态：scope 未启用返回 skipped，但派发器必须已恢复可调用而非 rejected/dispatcher_unavailable。
	res := srv.mailDispatcher.DispatchWelcome(context.Background(), 1, "new@example.com", "selfreg")
	if res.Status != mail.DispatchSkipped {
		t.Fatalf("清空恢复后派发器应可再次调用并返回 skipped: %+v", res)
	}
}

// TestClearAllPreHookFailureReturns503 前置暂停失败：503，且原数据库不被清空。
func TestClearAllPreHookFailureReturns503(t *testing.T) {
	srv := newFullMigrationServer(t)
	if _, err := srv.store.DB().Exec(`INSERT INTO users (username, email, role, user_source, status)
VALUES ('keep','keep@example.com','user','local','active')`); err != nil {
		t.Fatalf("插入保留用户失败: %v", err)
	}
	clearSvc := dataclear.NewService(srv.store, t.TempDir(), log.New("error", "console"))
	clearSvc.SetClearHooks(func(context.Context) error {
		return context.DeadlineExceeded
	}, func(context.Context, bool) {})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/clear", (&SettingsOpsHandler{clearSvc: clearSvc}).clearAll)
	req := httptest.NewRequest(http.MethodPost, "/clear", bytes.NewReader([]byte(`{"confirm_word":"RESET"}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("前置暂停失败应 503: %d %s", w.Code, w.Body.String())
	}
	var n int
	if err := srv.store.DB().QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		t.Fatalf("查询用户失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("前置失败不得清库，users 应仍为 1，实际 %d", n)
	}
}
