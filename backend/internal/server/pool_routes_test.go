package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/log"
	"vpn-sub/internal/pool"
	"vpn-sub/internal/store"
	"vpn-sub/migrations"
)

func newPoolRoutesEnv(t *testing.T) (*gin.Engine, *store.Store, *pool.Service) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), migrations.FS); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	poolSvc := pool.NewService(st, log.New("error", "console"))
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	noop := func(c *gin.Context) { c.Next() }
	RegisterPoolRoutes(engine, &PoolHandler{poolSvc: poolSvc}, noop, noop)
	return engine, st, poolSvc
}

func TestPoolUpdateEntryConflictReturnsHTTP409(t *testing.T) {
	engine, _, poolSvc := newPoolRoutesEnv(t)
	ctx := context.Background()
	p, err := poolSvc.Create(ctx, "HTTP冲突池", nil, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	m1, err := poolSvc.CreateEntry(ctx, p.ID, "DOMAIN", "a.com")
	if err != nil {
		t.Fatalf("创建 m1 失败: %v", err)
	}
	m2, err := poolSvc.CreateEntry(ctx, p.ID, "DOMAIN", "b.com")
	if err != nil {
		t.Fatalf("创建 m2 失败: %v", err)
	}
	body := map[string]any{"rule_type": "DOMAIN", "match_value": "b.com"}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/api/admin/pools/"+jsonInt(p.ID)+"/entries/"+jsonInt(m1.ID), bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("manual→manual 冲突应返回 409，实际 %d body=%s", w.Code, w.Body.String())
	}
	list, _, err := poolSvc.ListEntries(ctx, p.ID, 1, 20, "manual")
	if err != nil || len(list) != 2 {
		t.Fatalf("409 后两条记录应保留: %+v err=%v", list, err)
	}
	if list[0].MatchValue != "a.com" || list[1].MatchValue != "b.com" {
		t.Fatalf("409 后内容不应变化: %+v", list)
	}
	_ = m2
}

func TestPoolSourceStatusAPIDoesNotLeakOriginalURL(t *testing.T) {
	engine, _, poolSvc := newPoolRoutesEnv(t)
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("DOMAIN,a.com\n"))
	}))
	defer srv.Close()
	raw := srv.URL + "?token=SECRET"
	p, err := poolSvc.Create(ctx, "状态API池", []pool.SourceInput{{URL: raw, SourceMode: pool.SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := poolSvc.SubmitSync(ctx, p.ID); err != nil {
		t.Fatalf("提交同步失败: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		task, err := poolSvc.GetStatus(ctx, p.ID)
		if err != nil {
			t.Fatalf("查询任务失败: %v", err)
		}
		if task != nil && task.Status == "succeeded" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	path := "/api/admin/pools/" + jsonInt(p.ID) + "/sources/status"
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("sources/status 状态码异常: %d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "SECRET") {
		t.Fatalf("sources/status 不应泄漏原始 URL 凭据: %s", body)
	}
	if !strings.Contains(body, `"display_url":"`+srv.URL+`?token=***"`) {
		t.Fatalf("sources/status 应使用脱敏 display_url: %s", body)
	}
}
