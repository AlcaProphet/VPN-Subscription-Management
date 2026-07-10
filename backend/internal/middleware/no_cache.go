package middleware

import (
	"github.com/gin-gonic/gin"
)

// NoCacheForDownloads sets aggressive no-cache headers for download endpoints.
// This ensures VPN clients and intermediate proxies always fetch the latest
// current version and never cache stale configurations.
// Apply this middleware to all download routes.
func NoCacheForDownloads() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Next()
	}
}
