// server/emergency_gate.go：应急模式 503 拦截中间件（Build3 Step 6）——
// 应急模式下业务 API 与下载端点返回 503，系统状态/站点信息/应急端点/静态资源/SPA 回退正常服务（Design1 §3.8）。
package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// emergencyGate 应急模式网关：非白名单路径一律 503（应急模式下注册在全局中间件链）
func emergencyGate() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if isEmergencyAllowed(c.Request.Method, path) {
			c.Next()
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": "系统处于应急恢复模式"})
		c.Abort()
	}
}

// isEmergencyAllowed 白名单判定：
// 系统状态/站点信息/应急端点/静态资源（/assets、/public）/健康检查；
// SPA 前端路由（history 模式直达）显式列出——业务下载端点（/subscriptions/ 等）不在此列，一律 503
func isEmergencyAllowed(method, path string) bool {
	if strings.HasPrefix(path, "/api/system/status") ||
		strings.HasPrefix(path, "/api/site/info") ||
		strings.HasPrefix(path, "/api/emergency/") ||
		strings.HasPrefix(path, "/assets/") ||
		strings.HasPrefix(path, "/public/") ||
		path == "/health" {
		return true
	}
	if method == http.MethodGet && isSPAPath(path) {
		return true // 前端路由（前端据 emergency 标记强制跳 /emergency）
	}
	return false
}

// isSPAPath 前端路由集合（与 router/index.ts 路由表对齐）：
// 公开页（setup/login/register/forgot/reset/pending/callback/emergency）、用户端（/、/rules、/profile）、管理端（/admin/*）
func isSPAPath(path string) bool {
	switch {
	case path == "/" || path == "/emergency" || path == "/setup" || path == "/login" ||
		path == "/register" || path == "/forgot" || path == "/reset" || path == "/pending" ||
		path == "/login/callback" || path == "/rules" || path == "/profile":
		return true
	case path == "/admin" || strings.HasPrefix(path, "/admin/"):
		return true
	}
	return false
}
