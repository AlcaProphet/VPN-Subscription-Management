package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// sanitizeQueryToken masks the ?token= query parameter value in URLs.
func sanitizeQueryToken(rawURL string) string {
	// Simple approach: find ?token= and replace value with ***
	idx := strings.Index(rawURL, "?token=")
	if idx == -1 {
		idx = strings.Index(rawURL, "&token=")
		if idx == -1 {
			return rawURL
		}
	}
	// Find the end of the token value (next & or end of string)
	start := idx
	for start < len(rawURL) && rawURL[start] != '=' {
		start++
	}
	start++ // skip '='
	end := start
	for end < len(rawURL) && rawURL[end] != '&' && rawURL[end] != ' ' {
		end++
	}
	return rawURL[:start] + "***" + rawURL[end:]
}

// LoggerMiddleware returns a Gin middleware that logs requests using zerolog.
// The ?token= query parameter value is masked as *** in log output.
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Log after request completes
		duration := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		path := sanitizeQueryToken(c.Request.URL.RequestURI())
		clientIP := c.ClientIP()

		var ev *zerolog.Event
		if status >= 500 {
			ev = log.Error()
		} else if status >= 400 {
			ev = log.Warn()
		} else {
			ev = log.Info()
		}

		ev.
			Str("method", method).
			Str("path", path).
			Int("status", status).
			Dur("duration", duration).
			Str("ip", clientIP).
			Msg("HTTP request")
	}
}

// RecoveryMiddleware returns a Gin recovery middleware using zerolog for panic logging.
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Error().
					Str("method", c.Request.Method).
					Str("path", sanitizeQueryToken(c.Request.URL.RequestURI())).
					Str("ip", c.ClientIP()).
					Interface("panic", err).
					Msg("Panic recovered")

				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "Internal server error",
				})
			}
		}()
		c.Next()
	}
}
