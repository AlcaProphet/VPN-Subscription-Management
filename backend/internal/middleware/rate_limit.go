package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// RateLimiter interface for rate limiting. The actual implementation
// uses system_config for configurable limits and will be built in block 4.
type RateLimiter interface {
	Allow(key string) (bool, int) // returns (allowed, remaining_seconds)
}

// rateLimiterConfig holds the current rate limit configuration.
type rateLimiterConfig struct {
	LoginLimit    int // requests per minute for login endpoints
	DownloadLimit int // requests per minute for download endpoints
}

var defaultRateLimiterConfig = rateLimiterConfig{
	LoginLimit:    10,
	DownloadLimit: 20,
}

// RateLimitLogin returns a rate limit middleware for login/auth endpoints.
// Default: 10 requests per minute per IP.
func RateLimitLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Stub implementation for block 1 — always allow.
		// Actual implementation in block 4.
		c.Next()
	}
}

// RateLimitDownload returns a rate limit middleware for download endpoints.
// Default: 20 requests per minute per IP.
func RateLimitDownload() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Stub implementation for block 1 — always allow.
		// Actual implementation in block 4.
		c.Next()
	}
}

// writeRateLimitResponse writes a 429 Too Many Requests response.
func writeRateLimitResponse(c *gin.Context, retryAfter int, isLoginEndpoint bool) {
	c.Header("Retry-After", strconv.Itoa(retryAfter))
	if isLoginEndpoint {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"error": "请求过于频繁，请稍后再试",
		})
	} else {
		c.AbortWithStatus(http.StatusTooManyRequests)
		c.Writer.WriteString("rate limit exceeded, retry after " + strconv.Itoa(retryAfter) + " seconds")
	}
}
