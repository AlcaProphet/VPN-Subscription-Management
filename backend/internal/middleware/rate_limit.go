package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"vpn-sub/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
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
		log.Debug().Str("ip", ip).Int("count", len(valid)).Int("limit", limit).Int("retry_after", retryAfter).Msg("Rate limit triggered")
		w.windows[ip] = valid
		return false, retryAfter
	}

	// Allow — record this request
	valid = append(valid, now)
	w.windows[ip] = valid
	return true, 0
}

// periodicCleanup removes IP entries with no activity in the last minute.
// This prevents unbounded memory growth from one-off request sources.
// Runs every 2 minutes to match the 1-minute sliding window.
func (w *ipWindow) periodicCleanup() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		w.mu.Lock()
		cutoff := time.Now().Add(-1 * time.Minute)
		for ip, timestamps := range w.windows {
			// If all timestamps are older than the window, remove the IP
			allExpired := true
			for _, t := range timestamps {
				if t.After(cutoff) {
					allExpired = false
					break
				}
			}
			if allExpired {
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
			logRateLimitedDownload(c, ip)
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
// Uses c.String() for download responses so that Content-Type: text/plain is
// properly set (AbortWithStatus + WriteString skips content-type sniffing).
func writeRateLimitResponse(c *gin.Context, retryAfter int, isLoginEndpoint bool) {
	c.Header("Retry-After", strconv.Itoa(retryAfter))
	if isLoginEndpoint {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"error": "请求过于频繁，请稍后再试",
		})
	} else {
		c.String(http.StatusTooManyRequests, "rate limit exceeded, retry after %d seconds", retryAfter)
		c.Abort()
	}
}

// logRateLimitedDownload infers the download_type and relevant IDs from the
// request URL path and writes an access_log entry with status=failed,
// error_reason=rate_limited.
func logRateLimitedDownload(c *gin.Context, ip string) {
	path := c.Request.URL.Path
	record := &repository.AccessLogRecord{
		IP:          ip,
		Status:      "failed",
		ErrorReason: "rate_limited",
	}

	// Match URL patterns to determine download_type and extract IDs
	switch {
	case strings.Contains(path, "/subscriptions/"):
		record.DownloadType = "subscription"
		// Extract platform from path: /api/v1/subscriptions/:platform/download...
		if parts := strings.Split(path, "/"); len(parts) >= 5 && parts[3] == "subscriptions" {
			record.Platform = parts[4]
		}
	case strings.Contains(path, "/share/") && strings.Contains(path, "/download"):
		record.DownloadType = "share"
		// Extract share ID from path: /api/v1/share/:id/download
		if parts := strings.Split(path, "/"); len(parts) >= 5 && parts[3] == "share" {
			record.ShareSubscriptionID = parts[4]
		}
	case strings.Contains(path, "/rules/") && strings.Contains(path, "/download"):
		record.DownloadType = "rule"
		// Extract rule ID from path: /api/v1/rules/:id/download
		if parts := strings.Split(path, "/"); len(parts) >= 5 && parts[3] == "rules" {
			record.RuleID = parts[4]
		}
	default:
		record.DownloadType = "subscription" // fallback
	}

	repository.InsertAccessLog(record)
}
