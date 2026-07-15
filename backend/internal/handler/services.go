package handler

import (
	"vpn-sub/internal/service"
	"vpn-sub/internal/utils"
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
