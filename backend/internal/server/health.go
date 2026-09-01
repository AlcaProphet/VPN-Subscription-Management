package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// registerHealth 注册健康检查端点。
// GET /health 公开端点；应急模式返回 503 由 NewEmergency 的独立 /health 路由处理。
func registerHealth(engine *gin.Engine) {
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}
