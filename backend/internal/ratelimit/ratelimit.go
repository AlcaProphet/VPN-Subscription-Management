// Package ratelimit 提供按 IP 固定窗口速率限制中间件。
package ratelimit

import (
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/config"
	"vpn-sub/internal/response"
)

// 阈值配置键（存 system_config，每次请求读当前配置 → 修改立即生效）
const (
	KeyLogin    = "ratelimit_login"    // 默认 10/min
	KeyRegister = "ratelimit_register" // 默认 5/min
	KeyForgot   = "ratelimit_forgot"   // 默认 5/min
	KeyDownload = "ratelimit_download" // Build2 追加，默认 20/min
)

type bucket struct{ count int }

// Limiter 按 IP 固定窗口计数（分钟槽）
type Limiter struct {
	cfg     *config.Service
	log     *slog.Logger
	mu      sync.Mutex
	buckets map[string]bucket // key = 作用域+IP+分钟槽
}

func New(cfg *config.Service, lg *slog.Logger) *Limiter {
	return &Limiter{cfg: cfg, log: lg, buckets: map[string]bucket{}}
}

// Middleware 作用于登录/注册/找回密码端点（OIDC 回调与当前用户端点不限流，Design1 §5.2）
func (l *Limiter) Middleware(scope, configKey string, defaultLimit int) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP() // 真实客户端 IP：gin 已按 TRUST_PROXY 策略解析转发头
		slot := time.Now().UTC().Truncate(time.Minute)
		limit := l.cfg.GetInt(c.Request.Context(), configKey, defaultLimit)
		l.mu.Lock()
		key := scope + "|" + ip + "|" + slot.Format("200601021504")
		b := l.buckets[key]
		b.count++
		l.buckets[key] = b
		l.gc(slot) // 顺带清理过期槽，防内存泄漏
		l.mu.Unlock()
		if b.count > limit {
			c.Header("Retry-After", strconv.Itoa(int(time.Until(slot.Add(time.Minute)).Seconds())+1))
			response.Fail(c, http.StatusTooManyRequests, "操作过于频繁，请稍后再试")
			c.Abort()
			return
		}
		c.Next()
	}
}

// gc 清理早于当前分钟槽的过期桶
func (l *Limiter) gc(current time.Time) {
	cutoff := current.Add(-time.Minute).Format("200601021504")
	for k := range l.buckets {
		// key 格式：scope|ip|slot，取最后一个 | 后的槽值比较
		if idx := lastSep(k); idx >= 0 && k[idx+1:] < cutoff {
			delete(l.buckets, k)
		}
	}
}

// lastSep 返回最后一个分隔符位置
func lastSep(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '|' {
			return i
		}
	}
	return -1
}

// Reset 清空全部计数（一键清空数据时内存态复位回调，Build3 Step 4 使用）
func (l *Limiter) Reset() {
	l.mu.Lock()
	l.buckets = map[string]bucket{}
	l.mu.Unlock()
}
