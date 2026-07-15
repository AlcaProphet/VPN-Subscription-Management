package middleware

import (
	"github.com/gin-gonic/gin"
)

// CacheControlMiddleware sets Cache-Control: no-store for all responses by default.
// This can be overridden by specific handlers if needed.
func CacheControlMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set headers before processing the request so they take effect.
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}
