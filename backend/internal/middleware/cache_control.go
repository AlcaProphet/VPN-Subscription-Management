package middleware

import (
	"github.com/gin-gonic/gin"
)

// CacheControlMiddleware sets Cache-Control: no-store for all responses by default.
// This can be overridden by specific handlers if needed.
func CacheControlMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		// NoCacheForDownloads middleware handles download-specific cache headers.
		// This middleware is a generic no-cache for API responses.
		c.Header("Cache-Control", "no-store")
	}
}
