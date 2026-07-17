package middleware

import (
	"net/http"
	"strings"

	"vpn-sub/internal/repository"

	"github.com/gin-gonic/gin"
)

// authService interface allows the middleware to verify JWT tokens and fetch user info.
// Implemented by the auth package in block 2.
type AuthService interface {
	ValidateJWT(tokenString string) (string, error) // returns user_id
}

// defaultAuthService is set during app initialization (block 2).
// For block 1, it's nil and the middleware will reject all requests.
var DefaultAuthService AuthService

// SetAuthService sets the auth service used by AuthRequired middleware.
func SetAuthService(svc AuthService) {
	DefaultAuthService = svc
}

// AuthRequired is a middleware that verifies the JWT token from the
// Authorization: Bearer header and looks up the user from the database.
// It stores user_id in the Gin context for downstream handlers.
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Guard against misconfiguration where DefaultAuthService is nil
		// (e.g. configured=true but OIDC init failed at startup)
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

		tokenString := parts[1]

		// Validate JWT
		userID, err := DefaultAuthService.ValidateJWT(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		// Real-time DB lookup: verify the user still exists
		userRepo := repository.NewUserRepo()
		user, err := userRepo.FindByID(userID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			return
		}

		// Store user info in context for downstream handlers
		c.Set("user_id", user.UserID)
		c.Set("user_role", user.Role)
		c.Set("user_is_advanced", user.IsAdvanced)

		c.Next()
	}
}

// GetUserID extracts the user ID from the Gin context.
// Must be called after AuthRequired middleware.
func GetUserID(c *gin.Context) string {
	uid, _ := c.Get("user_id")
	if uid == nil {
		return ""
	}
	return uid.(string)
}

// GetUserRole extracts the user role from the Gin context.
func GetUserRole(c *gin.Context) string {
	role, _ := c.Get("user_role")
	if role == nil {
		return "user"
	}
	return role.(string)
}

// GetUserIsAdvanced extracts the user's is_advanced flag from the Gin context.
func GetUserIsAdvanced(c *gin.Context) bool {
	isAdv, _ := c.Get("user_is_advanced")
	if isAdv == nil {
		return false
	}
	return isAdv.(bool)
}
