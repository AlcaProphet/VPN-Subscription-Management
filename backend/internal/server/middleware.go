package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/config"
)

// AdvancedMode 高级模式中间件：每次请求实时查 DB，禁止缓存布尔值。
func AdvancedMode(cfg *config.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cfg.GetBool(c.Request.Context(), config.KeyAdvancedMode, false) {
			Fail(c, http.StatusForbidden, "高级功能未开启")
			c.Abort()
			return
		}
		c.Next()
	}
}
