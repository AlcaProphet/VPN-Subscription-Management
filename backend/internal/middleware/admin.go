package middleware

import (
	"net/http"

	"vpn-sub/internal/repository"

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

// ConditionalSetupAuth conditionally applies AuthRequired + AdminRequired.
// During initial setup (configured=false), the request passes through
// without authentication. After setup (configured=true), only admins
// can access the endpoint.
//
// This middleware is used for endpoints that serve dual purposes:
//   - /admin/system/configure  (initial setup + reconfiguration)
//   - /admin/system/switch-provider
//   - /admin/test-oidc
//   - /admin/oidc-config
func ConditionalSetupAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfgRepo := repository.NewSystemConfigRepo()
		configured := false
		if val, err := cfgRepo.Get("configured"); err == nil && val == "true" {
			configured = true
		}

		if !configured {
			// System not yet configured — allow without auth
			c.Next()
			return
		}

		// System is configured — require JWT + admin role.
		// Reuses ValidateJWTAndSetContext to avoid duplicating auth logic.
		_, role, err := ValidateJWTAndSetContext(c)
		if err != nil {
			if ae, ok := err.(*authError); ok {
				c.AbortWithStatusJSON(ae.HTTPStatus(), gin.H{"error": ae.Error()})
			} else {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			}
			return
		}

		if role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}

		c.Next()
	}
}
