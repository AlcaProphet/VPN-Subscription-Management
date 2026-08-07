package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// registerHealth 注册健康检查端点。
// GET /health 公开端点；应急模式返回 503 的判定在 Build3 Step 6 接入（预留注释）
func registerHealth(engine *gin.Engine) {
	engine.GET("/health", func(c *gin.Context) {
		// TODO(Build3 Step 6)：应急模式下返回 503 {"status":"emergency"}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}
