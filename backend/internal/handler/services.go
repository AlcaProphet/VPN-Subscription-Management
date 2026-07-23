package handler

import (
	"net/http"

	"vpn-sub/internal/service"
	"vpn-sub/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// Service instances — initialized once when the server starts.
var (
	PlatformSvc  *service.PlatformService
	UserSvc      *service.UserService
	SubSvc       *service.SubscriptionService
	RuleSvc      *service.RuleService
	CustomSubSvc *service.CustomSubscriptionService
	ShareSvc     *service.ShareSubscriptionService
	SystemSvc    *service.SystemService
)

// DebugMode controls whether 5xx errors return detailed internal messages
// (true) or generic "Internal server error" (false). Managed via admin
// panel and persisted in system_config.debug_mode. Default false (safe).
var DebugMode bool

// SetDebugMode is called at startup to initialize from system_config.
func SetDebugMode(v bool) { DebugMode = v }

// internalError logs the real error and returns a safe HTTP 500 response.
// When DebugMode is true (admin enabled it), the real error message is
// included in the response for troubleshooting.
func internalError(c *gin.Context, err error, msg string) {
	log.Error().Err(err).Str("context", msg).Msg("internal server error")
	if DebugMode {
		c.JSON(http.StatusInternalServerError, gin.H{"error": msg + ": " + err.Error()})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
	}
}

// InitServices initializes all service instances.
// Must be called before any handler that uses services.
func InitServices() {
	dataDir := utils.GetEnv("DATA_DIR", "./data")
	versionSvc := service.NewVersionService(dataDir)

	PlatformSvc = service.NewPlatformService(versionSvc)
	UserSvc = service.NewUserService(versionSvc)
	SubSvc = service.NewSubscriptionService(versionSvc)
	RuleSvc = service.NewRuleService(versionSvc)
	CustomSubSvc = service.NewCustomSubscriptionService(versionSvc)
	ShareSvc = service.NewShareSubscriptionService(versionSvc)
	SystemSvc = service.NewSystemService()
}
