package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestDebugContextMiddlewareServerIsolation 两个 Server 实例 debug_mode 不同且互不污染：
// 调试实例的 5xx 返回详情，非调试实例仍返回通用脱敏信息。
func TestDebugContextMiddlewareServerIsolation(t *testing.T) {
	debugSrv := newDownloadTestServer(t)
	normalSrv := newDownloadTestServer(t)
	if err := debugSrv.cfg.Set(context.Background(), "debug_mode", "true"); err != nil {
		t.Fatalf("开启调试模式失败: %v", err)
	}
	handler := func(c *gin.Context) { Fail(c, http.StatusInternalServerError, "内部详情") }
	debugSrv.Engine().GET("/__debug_fail", handler)
	normalSrv.Engine().GET("/__debug_fail", handler)

	request := func(srv *Server) string {
		req := httptest.NewRequest(http.MethodGet, "/__debug_fail", nil)
		w := httptest.NewRecorder()
		srv.Engine().ServeHTTP(w, req)
		return w.Body.String()
	}
	if got := request(debugSrv); !strings.Contains(got, "内部详情") {
		t.Fatalf("调试实例应返回详情: %s", got)
	}
	got := request(normalSrv)
	if strings.Contains(got, "内部详情") || !strings.Contains(got, "服务器内部错误") {
		t.Fatalf("普通实例应脱敏: %s", got)
	}
}
