package main

import (
	"fmt"
	"log"
	"path/filepath"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/handler"
	"vpn-sub/internal/middleware"
	"vpn-sub/internal/repository"
	"vpn-sub/internal/router"
	"vpn-sub/internal/utils"

	"github.com/gin-gonic/gin"
)

func main() {
	// Determine environment
	port := utils.GetEnv("PORT", "8080")
	dataDir := utils.GetEnv("DATA_DIR", "./data")

	// Resolve database path
	dbPath := filepath.Join(dataDir, "vpn.db")

	// Initialize database
	log.Printf("Initializing database at %s", dbPath)
	if err := repository.InitDB(dbPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
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
			log.Printf("Warning: Failed to initialize auth service: %v", err)
		} else {
			auth.DefaultService = svc
			middleware.SetAuthService(svc)
			log.Println("Auth service initialized successfully")
		}
	}

	// Initialize business services (block 3)
	handler.InitServices()
	log.Println("Business services initialized")

	// Setup Gin
	gin.SetMode(gin.ReleaseMode)
	r := router.SetupRouter()

	// Configure trusted proxies (for X-Forwarded-For/X-Real-IP behind reverse proxy)
	if err := r.SetTrustedProxies([]string{"127.0.0.1"}); err != nil {
		log.Printf("Warning: Failed to set trusted proxies: %v", err)
	}

	// Start server
	addr := fmt.Sprintf(":%s", port)
	log.Printf("Starting server on %s (configured=%v)", addr, configured)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
