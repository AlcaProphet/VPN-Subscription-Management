package providertraffic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

// rotationCommitTimeout 是已收到的新 Refresh Token 的独立本地提交上限。
// 该提交不绑定手动请求的取消信号，避免浏览器取消后丢失已轮换的凭据。
const rotationCommitTimeout = 5 * time.Second

// cachedAccessToken 是 V.P.S Access Token 的唯一内存缓存。
// 缓存以账号修订为键；修订不匹配或已过期时不得复用。
type cachedAccessToken struct {
	revision  string
	token     string
	expiresAt time.Time
}

// rotationCommitContext 为已解析的续期结果保留独立、最长 5 秒的提交机会。
// WithoutCancel 同时解除调用方取消、服务停止与请求截止时间的绑定，只保留上下文值。
func rotationCommitContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), rotationCommitTimeout)
}

// accessTokenExpiry 只解析 JWT 载荷中的数值 exp，不校验签名、不记录 Token。
// 无法取得有效到期时间时返回 false，调用方只在本轮使用该 Access Token。
func accessTokenExpiry(tokenString string) (time.Time, bool) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp json.RawMessage `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return time.Time{}, false
	}
	seconds, ok := parseFiniteNumber(claims.Exp)
	if !ok || seconds <= 0 || seconds > 1e12 {
		return time.Time{}, false
	}
	whole := int64(seconds)
	nanos := int64((seconds - float64(whole)) * float64(time.Second))
	return time.Unix(whole, nanos), true
}
