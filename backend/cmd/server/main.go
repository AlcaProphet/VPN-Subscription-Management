package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/handler"
	"vpn-sub/internal/middleware"
	"vpn-sub/internal/repository"
	"vpn-sub/internal/router"
	"vpn-sub/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Setup zerolog: console writer in development, JSON in production.
	// Set LOG_FORMAT=json in docker-compose.yml for structured logging.
	if utils.GetEnv("LOG_FORMAT", "") == "json" {
		log.Logger = zerolog.New(os.Stderr).With().Timestamp().Logger()
	} else {
		log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).With().Timestamp().Logger()
	}

	// Determine environment
	port := utils.GetEnv("PORT", "8080")
	dataDir := utils.GetEnv("DATA_DIR", "./data")

	// Resolve database path
	dbPath := filepath.Join(dataDir, "vpn.db")

	// Initialize database
	log.Info().Str("path", dbPath).Msg("Initializing database")
	if err := repository.InitDB(dbPath); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer repository.CloseDB()

	// Check if system is configured
	cfgRepo := repository.NewSystemConfigRepo()
	configured := false
	if val, err := cfgRepo.Get("configured"); err == nil && val == "true" {
		configured = true
	}

	// Initialize auth service if configured
	if configured {
		svc, err := auth.NewServiceFromDB(cfgRepo)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to initialize auth service")
		} else {
			auth.DefaultService = svc
			middleware.SetAuthService(svc)
			log.Info().Msg("Auth service initialized successfully")
		}
	}

	// Initialize business services (block 3)
	handler.InitServices()
	log.Info().Msg("Business services initialized")

	// Restore debug mode from system_config (may have been set in admin panel)
	if handler.SystemSvc != nil {
		handler.SetDebugMode(handler.SystemSvc.GetDebugMode())
	}

	// Setup Gin
	gin.SetMode(gin.ReleaseMode)
	r := router.SetupRouter()

	// Configure trusted proxies (for X-Forwarded-For/X-Real-IP behind reverse proxy).
	// In Docker, the external NGINX connects through the Docker gateway (e.g. 10.0.28.x),
	// not 127.0.0.1. We use 0.0.0.0/0 to trust all IPv4 proxies because:
	//   - Ports are bound to 127.0.0.1, so direct external access is impossible
	//   - Gin v1.10 SetTrustedProxies(nil) results in trustedCIDRs=nil which
	//     causes ClientIP() to skip X-Forwarded-For entirely
	if err := r.SetTrustedProxies([]string{"0.0.0.0/0"}); err != nil {
		log.Warn().Err(err).Msg("Failed to set trusted proxies")
	}

	// Start server
	addr := fmt.Sprintf(":%s", port)
	log.Info().Str("addr", addr).Bool("configured", configured).Msg("Starting server")
	if err := r.Run(addr); err != nil {
		log.Fatal().Err(err).Msg("Failed to start server")
	}
}
