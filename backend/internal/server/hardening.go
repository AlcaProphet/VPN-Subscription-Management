package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/config"
)

// newHTTPServer 构造带超时配置的 http.Server；超时值从 system_config 启动时读取。
func newHTTPServer(addr string, handler http.Handler, cfg *config.Service) *http.Server {
	ctx := context.Background()
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: time.Duration(cfg.GetInt(ctx, "http_read_header_timeout_sec", 5)) * time.Second,
		ReadTimeout:       time.Duration(cfg.GetInt(ctx, "http_read_timeout_sec", 60)) * time.Second,
		WriteTimeout:      time.Duration(cfg.GetInt(ctx, "http_write_timeout_sec", 300)) * time.Second,
		IdleTimeout:       time.Duration(cfg.GetInt(ctx, "http_idle_timeout_sec", 120)) * time.Second,
	}
}

// bodyLimitMiddleware 按路由分级限制请求体。导入路由豁免；版本/安装包上传保留较大余量。
func bodyLimitMiddleware(cfg *config.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.FullPath()
		ctx := c.Request.Context()
		// 导入路由继续由部署层反代控制，维持 F02 边界口径。
		if path == "/api/setup/import" || path == "/api/admin/settings/import" {
			c.Next()
			return
		}
		var limit int64
		switch {
		case strings.HasPrefix(path, "/api/admin/platforms/:id/installers"):
			limit = 320 << 20 // 300MB 文件 + multipart 余量
		case strings.Contains(path, "/versions") || strings.Contains(path, "/custom") ||
			strings.Contains(path, "/api/admin/shares") || strings.Contains(path, "/api/admin/rules"):
			limit = 55 << 20 // 50MB 内容 + multipart/文本余量
		default:
			limit = int64(cfg.GetInt(ctx, "http_max_body_mb", 4)) << 20
		}
		if c.Request.ContentLength > limit {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"code": 413, "message": "请求体过大"})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}

// clearWriteDeadline 供 SSE、备份/导出/公开大文件等长传输端点清除 WriteTimeout。
func clearWriteDeadline(c *gin.Context) {
	_ = http.NewResponseController(c.Writer).SetWriteDeadline(time.Time{})
}

// clearReadDeadline 供大文件上传端点清除 ReadTimeout。
func clearReadDeadline(c *gin.Context) {
	_ = http.NewResponseController(c.Writer).SetReadDeadline(time.Time{})
}
