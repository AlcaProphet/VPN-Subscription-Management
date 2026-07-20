package router

import (
	"vpn-sub/internal/handler"
	"vpn-sub/internal/middleware"

	"github.com/gin-gonic/gin"
)

// SetupRouter creates and configures the Gin router.
// All routes are always registered so that the server can transition from
// setup mode to normal mode without a restart. Setup-specific endpoints
// use ConditionalSetupAuth to allow unauthenticated access during initial
// setup and require admin JWT afterwards.
func SetupRouter() *gin.Engine {
	r := gin.New()

	// Global middleware (applied to all routes)
	r.Use(middleware.RecoveryMiddleware())
	r.Use(middleware.LoggerMiddleware())
	r.Use(middleware.CORSMiddleware())

	// Health check (no prefix, for container health checks)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")
	{
		// Public endpoints — always available
		api.GET("/system/status", handler.GetSystemStatus)
		api.GET("/platforms", handler.GetPlatforms)
		api.GET("/rules", handler.GetRules)
		api.GET("/rules/:id/download", middleware.NoCacheForDownloads(), middleware.RateLimitDownload(), handler.GetRuleDownload)

		// Setup/reconfigure admin endpoints — always registered.
		// ConditionalSetupAuth allows unauthenticated access during initial
		// setup (configured=false) and requires admin JWT after setup.
		setupAdmin := api.Group("/admin")
		setupAdmin.Use(middleware.ConditionalSetupAuth())
		{
			setupAdmin.POST("/system/configure", handler.PostConfigure)
			setupAdmin.POST("/system/switch-provider", handler.PostSwitchProvider)
			setupAdmin.POST("/test-oidc", handler.PostTestOIDC)
			setupAdmin.GET("/oidc-config", handler.GetOIDCConfig)
		}

		// All normal-mode routes are always registered so that after the
		// initial setup completes the server does not need a restart.
		registerAuthRoutes(api)
		registerUserRoutes(api)
		registerDownloadRoutes(api)
		registerShareDownloadRoutes(api)
		registerAdminRoutes(api)
	}

	return r
}

func registerAuthRoutes(api *gin.RouterGroup) {
	auth := api.Group("/auth")
	auth.Use(middleware.RateLimitLogin())
	{
		auth.GET("/login", handler.AuthLogin)
		auth.GET("/callback", handler.AuthCallback)
		auth.GET("/me", middleware.AuthRequired(), handler.AuthMe)
	}
}

func registerUserRoutes(api *gin.RouterGroup) {
	user := api.Group("/user")
	user.Use(middleware.AuthRequired())
	{
		user.GET("/platforms", handler.UserPlatforms)
		user.GET("/update-time", handler.UserUpdateTime)
		user.POST("/refresh-token", handler.UserRefreshToken)
	}
}

func registerDownloadRoutes(api *gin.RouterGroup) {
	subs := api.Group("/subscriptions/:platform")
	subs.Use(middleware.NoCacheForDownloads())
	subs.Use(middleware.RateLimitDownload())
	{
		subs.GET("/download", middleware.AuthRequired(), handler.SubDownload)
		subs.GET("/download/preview", middleware.AuthRequired(), handler.SubDownloadPreview)
		subs.GET("/download-token", handler.SubDownloadToken)
	}
}

func registerShareDownloadRoutes(api *gin.RouterGroup) {
	api.GET("/share/:id/download", middleware.NoCacheForDownloads(), middleware.RateLimitDownload(), handler.ShareDownload)
}

func registerAdminRoutes(api *gin.RouterGroup) {
	admin := api.Group("/admin")
	admin.Use(middleware.AuthRequired())
	admin.Use(middleware.AdminRequired())
	{
		// Users
		admin.GET("/users", handler.ListUsers)
		admin.GET("/users/:id", handler.GetUser)
		admin.PUT("/users/:id", handler.UpdateUser)
		admin.DELETE("/users/:id", handler.DeleteUser)
		admin.POST("/users/:id/revoke-tokens", handler.RevokeUserTokens)
		admin.POST("/users/:id/custom-subscription", handler.UploadCustomSubscription)
		admin.POST("/users/:id/custom-subscription/versions", handler.UploadCustomSubscriptionVersion)
		admin.DELETE("/users/:id/custom-subscription", handler.DeleteCustomSubscription)
		admin.GET("/users/:id/custom-subscription/versions/:versionId", handler.GetCustomVersion)
		admin.PUT("/users/:id/custom-subscription/versions/:versionId/current", handler.SwitchCustomVersion)
		admin.DELETE("/users/:id/custom-subscription/versions/:versionId", handler.DeleteCustomVersion)
		admin.POST("/users/:id/custom-subscription/refresh-token", handler.RefreshCustomSubToken)

		// Subscriptions
		admin.GET("/subscriptions", handler.ListSubscriptions)
		admin.POST("/subscriptions", handler.CreateSubscription)
		admin.GET("/subscriptions/:id", handler.GetSubscription)
		admin.PUT("/subscriptions/:id", handler.UpdateSubscription)
		admin.DELETE("/subscriptions/:id", handler.DeleteSubscription)
		admin.POST("/subscriptions/:id/versions", handler.UploadSubscriptionVersion)
		admin.PUT("/subscriptions/:id/versions/:versionId/current", handler.SwitchSubscriptionVersion)
		admin.GET("/subscriptions/:id/versions/:versionId", handler.GetSubscriptionVersion)
		admin.DELETE("/subscriptions/:id/versions/:versionId", handler.DeleteSubscriptionVersion)

		// Share Subscriptions
		admin.GET("/shares", handler.ListShares)
		admin.POST("/shares", handler.CreateShare)
		admin.GET("/shares/:id", handler.GetShare)
		admin.PUT("/shares/:id", handler.UpdateShare)
		admin.DELETE("/shares/:id", handler.DeleteShare)
		admin.POST("/shares/:id/versions", handler.UploadShareVersion)
		admin.PUT("/shares/:id/versions/:versionId/current", handler.SwitchShareVersion)
		admin.GET("/shares/:id/versions/:versionId", handler.GetShareVersion)
		admin.DELETE("/shares/:id/versions/:versionId", handler.DeleteShareVersion)
		admin.POST("/shares/:id/refresh-token", handler.RefreshShareToken)
		admin.DELETE("/shares/:id/token", handler.RevokeShareToken)

		// Platforms
		admin.GET("/platforms", handler.ListPlatforms)
		admin.POST("/platforms", handler.CreatePlatform)
		admin.GET("/platforms/:id", handler.GetPlatform)
		admin.PUT("/platforms/:id", handler.UpdatePlatform)
		admin.DELETE("/platforms/:id", handler.DeletePlatform)

		// Rules
		admin.GET("/rules", handler.ListAdminRules)
		admin.POST("/rules", handler.CreateRule)
		admin.GET("/rules/:id", handler.GetAdminRule)
		admin.PUT("/rules/:id", handler.UpdateAdminRule)
		admin.DELETE("/rules/:id", handler.DeleteAdminRule)
		admin.POST("/rules/:id/versions", handler.UploadRuleVersion)
		admin.PUT("/rules/:id/versions/:versionId/current", handler.SwitchRuleVersion)
		admin.GET("/rules/:id/versions/:versionId", handler.GetRuleVersion)
		admin.DELETE("/rules/:id/versions/:versionId", handler.DeleteRuleVersion)
		admin.POST("/rules/:id/refresh-token", handler.RefreshRuleToken)

		// System Config (oidc-config, test-oidc, configure, switch-provider
		// are handled by the ConditionalSetupAuth group above)
		admin.GET("/system/rate-limit", handler.GetRateLimit)
		admin.PUT("/system/rate-limit", handler.UpdateRateLimit)

		// Logs
		admin.GET("/logs", handler.GetLogs)
	}
}
