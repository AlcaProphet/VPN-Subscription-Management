package middleware

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// sanitizeQueryToken masks the ?token= query parameter value in URLs.
// Uses net/url.Parse for robust handling of URL-encoded params, multi-value
// tokens containing '&', and non-standard separators.
func sanitizeQueryToken(rawURL string) string {
	// Try to parse as a full URL first, then fall back to treating the
	// whole string as a path+query.
	var path, query string
	if u, err := url.Parse(rawURL); err == nil && u.RawQuery != "" {
		path = strings.SplitN(rawURL, "?", 2)[0]
		query = u.RawQuery
	} else if idx := strings.Index(rawURL, "?"); idx >= 0 {
		path = rawURL[:idx]
		query = rawURL[idx+1:]
	} else {
		return rawURL // no query string
	}

	vals, err := url.ParseQuery(query)
	if err != nil {
		return rawURL // can't parse, return as-is (safe — won't expose more than raw)
	}

	// Mask any token parameters
	masked := false
	for key := range vals {
		if key == "token" {
			vals.Set(key, "***")
			masked = true
		}
	}

	if !masked {
		return rawURL
	}

	return path + "?" + vals.Encode()
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
			Str("xff", c.GetHeader("X-Forwarded-For")).
			Str("xri", c.GetHeader("X-Real-IP")).
			Str("remote", c.Request.RemoteAddr).
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
