// Package response 提供统一响应结构助手（错误码约定 AGENTS §4.8）。
// 独立成包以避免 server 与业务层（auth 等）之间的循环依赖。
package response

import (
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

// OK 成功响应
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{Code: 0, Data: data})
}

// Fail httpStatus 与业务码同步取值（400/401/403/409/429/500）
func Fail(c *gin.Context, httpStatus int, msg string) {
	if httpStatus >= 500 {
		log.Error("内部错误", "path", c.Request.URL.Path, "msg", msg) // 经脱敏 Handler 输出
		msg = "服务器内部错误"                                          // 5xx 对外脱敏（debug_mode 开启时返回详情，Build3 Step 3 接通）
	}
	c.JSON(httpStatus, Response{Code: httpStatus, Message: msg})
}
