package server

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestRequestAndPanicLoggerUseInstance 请求日志与 panic 日志均写入传入的实例 Logger。
func TestRequestAndPanicLoggerUseInstance(t *testing.T) {
	var buf bytes.Buffer
	lg := slog.New(slog.NewTextHandler(&buf, nil))
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(requestLogger(lg), panicRecovery(lg))
	engine.GET("/ok", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	engine.GET("/panic", func(c *gin.Context) { panic("boom") })

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if w.Code != http.StatusNoContent {
		t.Fatalf("正常请求状态异常: %d", w.Code)
	}
	if !strings.Contains(buf.String(), "http_request") {
		t.Fatalf("请求日志未写入实例 Logger: %q", buf.String())
	}

	buf.Reset()
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("panic 应转 500: %d", w.Code)
	}
	if !strings.Contains(buf.String(), "panic 恢复") {
		t.Fatalf("panic 日志未写入实例 Logger: %q", buf.String())
	}
}
