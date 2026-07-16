package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"vpn-sub/internal/repository"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// In-memory sliding-window rate limiter.
//
// Each IP address gets a sliding window of request timestamps. On each request,
// timestamps older than 1 minute are pruned. If the count of remaining
// timestamps exceeds the limit, the request is rejected with HTTP 429.
//
// Limits are read from system_config on every request so that admin changes
// take effect immediately (no caching). A background goroutine periodically
// removes IP entries that have had no activity for >2 minutes to bound memory.
// ============================================================================

type ipWindow struct {
	mu      sync.Mutex
	windows map[string][]time.Time
}

var (
	loginWindow    = &ipWindow{windows: make(map[string][]time.Time)}
	downloadWindow = &ipWindow{windows: make(map[string][]time.Time)}
)

func init() {
	go loginWindow.periodicCleanup()
	go downloadWindow.periodicCleanup()
}

// allow returns (allowed, retryAfterSeconds).
func (w *ipWindow) allow(ip string, limit int) (bool, int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Minute)

	timestamps := w.windows[ip]

	// Prune entries older than 1 minute
	valid := timestamps[:0]
	for _, t := range timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= limit {
		// Over limit — compute Retry-After from the oldest timestamp in the window
		oldest := valid[0]
		retryAfter := int(oldest.Add(time.Minute).Sub(now).Seconds())
		if retryAfter < 1 {
			retryAfter = 1
		}
		w.windows[ip] = valid
		return false, retryAfter
	}

	// Allow — record this request
	valid = append(valid, now)
	w.windows[ip] = valid
	return true, 0
}

// periodicCleanup removes IP entries with no activity in the last 2 minutes.
// This prevents unbounded memory growth from one-off request sources.
func (w *ipWindow) periodicCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		w.mu.Lock()
		cutoff := time.Now().Add(-2 * time.Minute)
		for ip, timestamps := range w.windows {
			// If the newest timestamp is older than cutoff, remove the IP
			hasRecent := false
			for _, t := range timestamps {
				if t.After(cutoff) {
					hasRecent = true
					break
				}
			}
			if !hasRecent {
				delete(w.windows, ip)
			}
		}
		w.mu.Unlock()
	}
}

// ============================================================================
// Rate limit configuration (read from system_config on every request)
// ============================================================================

func getLoginRateLimit() int {
	cfgRepo := repository.NewSystemConfigRepo()
	if val, err := cfgRepo.Get("rate_limit_login"); err == nil && val != "" {
		if n, parseErr := strconv.Atoi(val); parseErr == nil && n > 0 {
			return n
		}
	}
	return 10
}

func getDownloadRateLimit() int {
	cfgRepo := repository.NewSystemConfigRepo()
	if val, err := cfgRepo.Get("rate_limit_download"); err == nil && val != "" {
		if n, parseErr := strconv.Atoi(val); parseErr == nil && n > 0 {
			return n
		}
	}
	return 20
}

// ============================================================================
// Middleware constructors
// ============================================================================

// RateLimitLogin returns a rate limit middleware for login/auth endpoints.
// Default: 10 requests per minute per IP (configurable via system_config).
func RateLimitLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		loginLimit := getLoginRateLimit()
		ip := c.ClientIP()
		if allowed, retryAfter := loginWindow.allow(ip, loginLimit); !allowed {
			writeRateLimitResponse(c, retryAfter, true)
			return
		}
		c.Next()
	}
}

// RateLimitDownload returns a rate limit middleware for download endpoints.
// Default: 20 requests per minute per IP (configurable via system_config).
func RateLimitDownload() gin.HandlerFunc {
	return func(c *gin.Context) {
		downloadLimit := getDownloadRateLimit()
		ip := c.ClientIP()
		if allowed, retryAfter := downloadWindow.allow(ip, downloadLimit); !allowed {
			writeRateLimitResponse(c, retryAfter, false)
			return
		}
		c.Next()
	}
}

// ============================================================================
// Response helpers
// ============================================================================

// writeRateLimitResponse writes a 429 Too Many Requests response.
// Login endpoints get a JSON error body; download endpoints get plain text.
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
