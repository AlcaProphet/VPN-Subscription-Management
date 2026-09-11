// Package response 提供统一响应结构助手（错误码约定 AGENTS §4.8）。
// 独立成包以避免 server 与业务层（auth 等）之间的循环依赖。
package response

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/log"
)

// Response 统一响应结构
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// ListData 列表包裹结构：{ "code":0, "data": { "list": [...], "total": N } }
type ListData struct {
	List  any   `json:"list"`
	Total int64 `json:"total"`
}

// debugContextKey 请求上下文中的调试模式标志键（类型化私有键，避免与其他包 context 值冲突）。
type debugContextKey struct{}

// WithDebug 把当前请求的调试模式标志写入 context；server 中间件按各自 cfg 注入，不使用包级可变回调。
func WithDebug(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, debugContextKey{}, enabled)
}

// DebugEnabled 读取请求上下文调试标志；缺失时默认 false（5xx 对外脱敏）。
func DebugEnabled(ctx context.Context) bool {
	enabled, _ := ctx.Value(debugContextKey{}).(bool)
	return enabled
}

// OK 成功响应
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{Code: 0, Data: data})
}

// Fail httpStatus 与业务码同步取值（400/401/403/409/429/500）
func Fail(c *gin.Context, httpStatus int, msg string) {
	if httpStatus >= 500 {
		log.FromContext(c.Request.Context()).Error("内部错误", "path", c.Request.URL.Path, "msg", msg) // 经脱敏 Handler 输出
		// 5xx 对外脱敏：仅当前请求上下文显式开启调试时返回真实详情
		if !DebugEnabled(c.Request.Context()) {
			msg = "服务器内部错误"
		}
	}
	c.JSON(httpStatus, Response{Code: httpStatus, Message: msg})
}
