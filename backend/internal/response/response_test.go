package response

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	applog "vpn-sub/internal/log"
)

func failBody(t *testing.T, ctxDebug bool) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/fail", nil)
	if ctxDebug {
		req = req.WithContext(WithDebug(req.Context(), true))
	}
	c.Request = req
	Fail(c, http.StatusInternalServerError, "内部详情")
	return rec.Body.String()
}

// TestDebugFlagFromRequestContext 5xx 详情只由当前请求上下文决定，不使用包级可变回调。
func TestDebugFlagFromRequestContext(t *testing.T) {
	if got := failBody(t, true); !strings.Contains(got, "内部详情") {
		t.Fatalf("调试请求应返回内部详情: %s", got)
	}
	got := failBody(t, false)
	if strings.Contains(got, "内部详情") || !strings.Contains(got, "服务器内部错误") {
		t.Fatalf("非调试请求应脱敏: %s", got)
	}
	if DebugEnabled(context.Background()) {
		t.Fatal("缺失调试标志应默认 false")
	}
}

// TestFailUsesContextLogger 5xx 日志写入请求上下文中的实例 Logger。
func TestFailUsesContextLogger(t *testing.T) {
	var buf bytes.Buffer
	lg := slog.New(slog.NewTextHandler(&buf, nil))
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/fail-log", nil)
	req = req.WithContext(applog.WithLogger(req.Context(), lg))
	c.Request = req
	Fail(c, http.StatusInternalServerError, "内部详情")
	if !strings.Contains(buf.String(), "内部错误") || !strings.Contains(buf.String(), "fail-log") {
		t.Fatalf("5xx 日志未写入实例 Logger: %q", buf.String())
	}
}
