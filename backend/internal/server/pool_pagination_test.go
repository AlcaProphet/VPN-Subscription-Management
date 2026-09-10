package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"vpn-sub/internal/pool"
)

func TestSourceSnapshotsStrictPagination(t *testing.T) {
	engine, _, svc := newPoolRoutesEnv(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "分页严格池", []pool.SourceInput{{URL: "https://example.com/rules", SourceMode: pool.SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	base := "/api/admin/pools/" + strconv.FormatInt(p.ID, 10) + "/sources/" + strconv.FormatInt(p.Sources[0].ID, 10) + "/snapshots"
	for _, query := range []string{"?page=abc", "?page_size=xyz", "?page=0", "?page_size=-1", "?page=", "?page_size="} {
		req := httptest.NewRequest(http.MethodGet, base+query, nil)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s 应返回 400，实际 %d body=%s", query, w.Code, w.Body.String())
		}
	}
	for _, query := range []string{"", "?page=1&page_size=20"} {
		req := httptest.NewRequest(http.MethodGet, base+query, nil)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s 应返回 200，实际 %d body=%s", query, w.Code, w.Body.String())
		}
	}
}

func TestOtherPoolListEndpointsRemainTolerant(t *testing.T) {
	engine, _, svc := newPoolRoutesEnv(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "分页兼容池", nil, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	paths := []string{
		"/api/admin/pools/" + strconv.FormatInt(p.ID, 10) + "/entries?page=abc",
		"/api/admin/pools/" + strconv.FormatInt(p.ID, 10) + "/sync/tasks?page=abc",
	}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s 应保持原有默认回退行为，实际 %d body=%s", path, w.Code, w.Body.String())
		}
	}
}
