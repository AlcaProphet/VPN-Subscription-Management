package handler

import (
	"context"
	"net/http"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/repository"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// Public Handlers
// ============================================================================

func GetSystemStatus(c *gin.Context) {
	cfgRepo := repository.NewSystemConfigRepo()
	configured := false
	if val, err := cfgRepo.Get("configured"); err == nil && val == "true" {
		configured = true
	}
	c.JSON(http.StatusOK, gin.H{"configured": configured})
}

func GetPlatforms(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"platforms": []interface{}{}})
}

func GetRules(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"rules": []interface{}{}})
}

func GetRuleDownload(c *gin.Context) {
	c.String(http.StatusOK, "# rule download stub")
}

// ============================================================================
// Auth Handlers
// ============================================================================

func AuthLogin(c *gin.Context) {
	svc := auth.DefaultService
	if svc == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Auth service not initialized"})
		return
	}

	result, err := svc.InitiateLogin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initiate login: " + err.Error()})
		return
	}

	// Set state in HttpOnly cookie for CSRF triple verification
	c.SetCookie(
		"oidc_state", result.State, 600, "/", "", true, true, // 10 min, secure, httpOnly
	)

	c.Redirect(http.StatusFound, result.RedirectURL)
}

func AuthCallback(c *gin.Context) {
	svc := auth.DefaultService
	if svc == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Auth service not initialized"})
		return
	}

	// Read state from query param and cookie
	queryState := c.Query("state")
	cookieState, _ := c.Cookie("oidc_state")
	code := c.Query("code")

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing authorization code"})
		return
	}

	result, err := svc.HandleCallback(context.Background(), queryState, cookieState, code)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication failed: " + err.Error()})
		return
	}

	// Clear the state cookie
	c.SetCookie("oidc_state", "", -1, "/", "", true, true)

	// Redirect to frontend callback page with JWT in query
	frontendCallback := result.FrontendURL + "/auth/callback?token=" + result.JWT
	c.Redirect(http.StatusFound, frontendCallback)
}

func AuthMe(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	user, err := auth.GetCurrentUser(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":     user.UserID,
		"username":    user.Username,
		"email":       user.Email,
		"role":        user.Role,
		"is_advanced": user.IsAdvanced,
		"groups":      user.Groups,
	})
}

// ============================================================================
// User Handlers
// ============================================================================

func UserPlatforms(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"platforms": []interface{}{}})
}

func UserUpdateTime(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"update_time": ""})
}

func UserRefreshToken(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ============================================================================
// Download Handlers
// ============================================================================

func SubDownload(c *gin.Context) {
	c.String(http.StatusOK, "# subscription download stub")
}

func SubDownloadPreview(c *gin.Context) {
	c.String(http.StatusOK, "# subscription preview stub")
}

func SubDownloadToken(c *gin.Context) {
	c.String(http.StatusOK, "# subscription token download stub")
}

func ShareDownload(c *gin.Context) {
	c.String(http.StatusOK, "# share download stub")
}

// ============================================================================
// Admin: Setup
// ============================================================================

func PostConfigure(c *gin.Context) {
	var req struct {
		ProviderType string `json:"provider_type" binding:"required"`
		// Keycloak fields
		KeycloakBaseURL string `json:"keycloak_base_url"`
		KeycloakRealm   string `json:"keycloak_realm"`
		// Auth0 fields
		Auth0Domain string `json:"auth0_domain"`
		// Generic OIDC fields
		GenericIssuer string `json:"generic_issuer"`
		// Common fields
		ClientID     string `json:"client_id" binding:"required"`
		ClientSecret string `json:"client_secret" binding:"required"`
		RedirectURI  string `json:"redirect_uri" binding:"required"`
		FrontendURL  string `json:"frontend_url" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Validate provider type
	pt := auth.ProviderType(req.ProviderType)
	if pt != auth.ProviderKeycloak && pt != auth.ProviderAuth0 && pt != auth.ProviderGeneric {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider_type, must be keycloak, auth0, or generic"})
		return
	}

	cfg := &auth.OIDCConfig{
		ProviderType: pt,
		ClientID:     req.ClientID,
		RedirectURI:  req.RedirectURI,
		FrontendURL:  req.FrontendURL,
	}

	switch pt {
	case auth.ProviderKeycloak:
		if req.KeycloakBaseURL == "" || req.KeycloakRealm == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "keycloak_base_url and keycloak_realm are required for Keycloak"})
			return
		}
		cfg.KeycloakBaseURL = req.KeycloakBaseURL
		cfg.KeycloakRealm = req.KeycloakRealm
	case auth.ProviderAuth0:
		if req.Auth0Domain == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "auth0_domain is required for Auth0"})
			return
		}
		cfg.Auth0Domain = req.Auth0Domain
	case auth.ProviderGeneric:
		if req.GenericIssuer == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "generic_issuer is required for Generic OIDC"})
			return
		}
		cfg.GenericIssuer = req.GenericIssuer
	}

	cfgRepo := repository.NewSystemConfigRepo()
	_, err := auth.ConfigureSystem(cfgRepo, cfg, req.ClientSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Configuration failed: " + err.Error()})
		return
	}

	// Re-initialize the auth service with new config
	svc, err := auth.NewServiceFromDB(cfgRepo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize auth service: " + err.Error()})
		return
	}
	auth.DefaultService = svc

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func PostSwitchProvider(c *gin.Context) {
	var req struct {
		ProviderType string `json:"provider_type" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	pt := auth.ProviderType(req.ProviderType)
	if pt != auth.ProviderKeycloak && pt != auth.ProviderAuth0 && pt != auth.ProviderGeneric {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider_type"})
		return
	}

	cfgRepo := repository.NewSystemConfigRepo()
	if err := auth.SwitchProvider(cfgRepo, pt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to switch provider: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func PostTestOIDC(c *gin.Context) {
	var req struct {
		ProviderType string `json:"provider_type" binding:"required"`
		// Keycloak fields
		KeycloakBaseURL string `json:"keycloak_base_url"`
		KeycloakRealm   string `json:"keycloak_realm"`
		// Auth0 fields
		Auth0Domain string `json:"auth0_domain"`
		// Generic OIDC fields
		GenericIssuer string `json:"generic_issuer"`
		// Common fields
		ClientID     string `json:"client_id" binding:"required"`
		ClientSecret string `json:"client_secret" binding:"required"`
		RedirectURI  string `json:"redirect_uri" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	pt := auth.ProviderType(req.ProviderType)
	if pt != auth.ProviderKeycloak && pt != auth.ProviderAuth0 && pt != auth.ProviderGeneric {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider_type"})
		return
	}

	// Build issuer URL
	var issuerURL string
	switch pt {
	case auth.ProviderKeycloak:
		if req.KeycloakBaseURL == "" || req.KeycloakRealm == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "keycloak_base_url and keycloak_realm are required"})
			return
		}
		issuerURL = req.KeycloakBaseURL + "/realms/" + req.KeycloakRealm
	case auth.ProviderAuth0:
		if req.Auth0Domain == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "auth0_domain is required"})
			return
		}
		issuerURL = "https://" + req.Auth0Domain
	case auth.ProviderGeneric:
		if req.GenericIssuer == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "generic_issuer is required"})
			return
		}
		issuerURL = req.GenericIssuer
	}

	if err := auth.TestConnection(pt, issuerURL, req.ClientID, req.ClientSecret, req.RedirectURI); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OIDC connection test failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "OIDC connection successful"})
}

func GetOIDCConfig(c *gin.Context) {
	cfgRepo := repository.NewSystemConfigRepo()
	cfg, err := auth.GetMaskedOIDCConfig(cfgRepo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read OIDC config"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"config": cfg})
}

// ============================================================================
// Admin: Users
// ============================================================================

func ListUsers(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"users": []interface{}{}})
}

func GetUser(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"user": gin.H{}})
}

func UpdateUser(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteUser(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RevokeUserTokens(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UploadCustomSubscription(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UploadCustomSubscriptionVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteCustomSubscription(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetCustomVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"version": gin.H{}})
}

func SwitchCustomVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteCustomVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RefreshCustomSubToken(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ============================================================================
// Admin: Subscriptions
// ============================================================================

func ListSubscriptions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"subscriptions": []interface{}{}})
}

func CreateSubscription(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetSubscription(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"subscription": gin.H{}})
}

func UpdateSubscription(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteSubscription(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UploadSubscriptionVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func SwitchSubscriptionVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetSubscriptionVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"version": gin.H{}})
}

func DeleteSubscriptionVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ============================================================================
// Admin: Share Subscriptions
// ============================================================================

func ListShares(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"shares": []interface{}{}})
}

func CreateShare(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetShare(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"share": gin.H{}})
}

func UpdateShare(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteShare(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UploadShareVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func SwitchShareVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetShareVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"version": gin.H{}})
}

func DeleteShareVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RefreshShareToken(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RevokeShareToken(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ============================================================================
// Admin: Platforms
// ============================================================================

func ListPlatforms(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"platforms": []interface{}{}})
}

func CreatePlatform(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetPlatform(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"platform": gin.H{}})
}

func UpdatePlatform(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeletePlatform(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ============================================================================
// Admin: Rules
// ============================================================================

func ListAdminRules(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"rules": []interface{}{}})
}

func CreateRule(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetAdminRule(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"rule": gin.H{}})
}

func UpdateAdminRule(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteAdminRule(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UploadRuleVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func SwitchRuleVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetRuleVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"version": gin.H{}})
}

func DeleteRuleVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RefreshRuleToken(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ============================================================================
// Admin: System Config
// ============================================================================

func GetRateLimit(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"rate_limit_login": 10, "rate_limit_download": 20})
}

func UpdateRateLimit(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ============================================================================
// Admin: Logs
// ============================================================================

func GetLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"logs": []interface{}{}})
}
