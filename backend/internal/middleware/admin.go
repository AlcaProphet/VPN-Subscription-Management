package middleware

import (
	"net/http"
	"strings"

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

		// System is configured — require JWT + admin role
		// Inline AuthRequired logic (we can't chain middleware dynamically in Gin easily,
		// so we replicate the essential checks here).
		if DefaultAuthService == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "Auth service not initialized"})
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing authorization header"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header"})
			return
		}

		userID, err := DefaultAuthService.ValidateJWT(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		userRepo := repository.NewUserRepo()
		user, err := userRepo.FindByID(userID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			return
		}

		if user.Role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}

		// Store user info for downstream handlers
		c.Set("user_id", user.UserID)
		c.Set("user_role", user.Role)
		c.Set("user_is_advanced", user.IsAdvanced)

		c.Next()
	}
}
