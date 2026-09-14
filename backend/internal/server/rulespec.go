// rulespec.go：中央能力注册表只读元数据端点。
package server

import (
	"github.com/gin-gonic/gin"

	"vpn-sub/internal/rulespec"
)

// RegisterRulespecRoutes 注册能力元数据端点（管理端）。
func RegisterRulespecRoutes(engine *gin.Engine, sessionMW, adminMW gin.HandlerFunc) {
	engine.GET("/api/admin/rulespec/meta", sessionMW, adminMW, func(c *gin.Context) {
		OK(c, gin.H{
			"legacy":       rulespec.LegacyMetadata(),
			"capabilities": rulespec.Metadata(),
		})
	})
}
