package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AdminRequired is a middleware that ensures the current user has the "admin" role.
// Must be used after AuthRequired middleware.
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := GetUserRole(c)
		if role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Admin access required",
			})
			return
		}
		c.Next()
	}
}
