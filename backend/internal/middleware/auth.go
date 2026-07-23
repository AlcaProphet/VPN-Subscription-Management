package middleware

import (
	"net/http"
	"strings"

	"vpn-sub/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
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

// ValidateJWTAndSetContext validates the Authorization: Bearer header, looks up
// the user from the database, and stores user_id, user_role, user_is_advanced
// in the Gin context. Returns the user's ID and role on success.
//
// This is a shared helper used by both AuthRequired and ConditionalSetupAuth
// to avoid code duplication.
func ValidateJWTAndSetContext(c *gin.Context) (userID, role string, err error) {
	if DefaultAuthService == nil {
		return "", "", ErrAuthServiceNotInit
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return "", "", ErrMissingAuthHeader
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", "", ErrInvalidAuthHeader
	}

	userID, err = DefaultAuthService.ValidateJWT(parts[1])
	if err != nil {
		log.Debug().Err(err).Msg("JWT validation failed")
		return "", "", ErrInvalidToken
	}

	// Real-time DB lookup: verify the user still exists
	userRepo := repository.NewUserRepo()
	user, err := userRepo.FindByID(userID)
	if err != nil {
		log.Debug().Str("user_id", userID).Err(err).Msg("User not found in DB after JWT validation")
		return "", "", ErrUserNotFound
	}

	// Store user info in context for downstream handlers
	c.Set("user_id", user.UserID)
	c.Set("user_role", user.Role)
	c.Set("user_is_advanced", user.IsAdvanced)

	log.Debug().Str("user_id", user.UserID).Str("role", user.Role).Msg("Auth OK — context set")
	return user.UserID, user.Role, nil
}

// Sentinel errors for ValidateJWTAndSetContext so callers can distinguish
// error types and map them to the correct HTTP status codes.
var (
	ErrAuthServiceNotInit = &authError{"Auth service not initialized", http.StatusServiceUnavailable}
	ErrMissingAuthHeader  = &authError{"Missing authorization header", http.StatusUnauthorized}
	ErrInvalidAuthHeader  = &authError{"Invalid authorization header", http.StatusUnauthorized}
	ErrInvalidToken       = &authError{"Invalid or expired token", http.StatusUnauthorized}
	ErrUserNotFound       = &authError{"User not found", http.StatusUnauthorized}
)

type authError struct {
	msg        string
	httpStatus int
}

func (e *authError) Error() string   { return e.msg }
func (e *authError) HTTPStatus() int { return e.httpStatus }

// AuthRequired is a middleware that verifies the JWT token from the
// Authorization: Bearer header and looks up the user from the database.
// It stores user_id in the Gin context for downstream handlers.
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, _, err := ValidateJWTAndSetContext(c)
		if err != nil {
			if ae, ok := err.(*authError); ok {
				c.AbortWithStatusJSON(ae.HTTPStatus(), gin.H{"error": ae.Error()})
			} else {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			}
			return
		}
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
	v, ok := isAdv.(bool)
	if !ok {
		return false
	}
	return v
}
