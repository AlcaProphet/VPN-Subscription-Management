package handler

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/middleware"
	"vpn-sub/internal/models"
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
	platforms, err := PlatformSvc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"platforms": platforms})
}

func GetRules(c *gin.Context) {
	rules, err := RuleSvc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Enrich with token so the frontend can build download URLs
	type ruleWithToken struct {
		models.Rule
		Token string `json:"token,omitempty"`
	}
	result := make([]ruleWithToken, 0, len(rules))
	for _, r := range rules {
		tok, _ := RuleSvc.GetToken(r.ID)
		result = append(result, ruleWithToken{Rule: r, Token: tok})
	}
	c.JSON(http.StatusOK, gin.H{"rules": result})
}

func GetRuleDownload(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.String(http.StatusBadRequest, "missing token")
		return
	}
	ruleID, err := RuleSvc.ValidateToken(token)
	if err != nil {
		logAccess("", c.ClientIP(), "rule", "", "", ruleID, "failed", "token_invalid")
		c.String(http.StatusUnauthorized, "invalid token")
		return
	}
	content, err := RuleSvc.GetCurrentContent(ruleID)
	if err != nil {
		errorReason := errorReasonFromErr(err)
		logAccess("", c.ClientIP(), "rule", "", "", ruleID, "failed", errorReason)
		c.String(http.StatusNotFound, "rule not found")
		return
	}
	logAccess("", c.ClientIP(), "rule", "", "", ruleID, "success", "")
	setDownloadHeaders(c)
	c.String(http.StatusOK, content)
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

	// Optional ?prompt=login forces the OIDC provider to re-authenticate,
	// allowing the user to switch accounts even if they have an existing session.
	prompt := c.Query("prompt")

	result, err := svc.InitiateLogin(prompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initiate login: " + err.Error()})
		return
	}

	// Determine if the cookie should be Secure based on the request protocol.
	// In production behind a reverse proxy, X-Forwarded-Proto is set to "https".
	// In local development (HTTP), Secure must be false or the browser won't send it.
	isSecure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"

	// Set state in HttpOnly cookie for CSRF triple verification
	c.SetCookie(
		"oidc_state", result.State, 600, "/", "", isSecure, true,
	)

	c.Redirect(http.StatusFound, result.RedirectURL)
}

func AuthCallback(c *gin.Context) {
	svc := auth.DefaultService
	if svc == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Auth service not initialized"})
		return
	}

	// Determine if the cookie should be Secure based on the request protocol.
	// Computed once at the top so both success and error paths reuse it.
	isSecure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"

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
		// Clear the state cookie even on failure to avoid stale state interfering
		// with subsequent login attempts.
		c.SetCookie("oidc_state", "", -1, "/", "", isSecure, true)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication failed: " + err.Error()})
		return
	}

	// Clear the state cookie on success
	c.SetCookie("oidc_state", "", -1, "/", "", isSecure, true)

	// Redirect to frontend callback page with JWT in query
	frontendCallback := strings.TrimRight(result.FrontendURL, "/") + "/auth/callback?token=" + result.JWT
	c.Redirect(http.StatusFound, frontendCallback)
}

func AuthMe(c *gin.Context) {
	userID := middleware.GetUserID(c)
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

// UserPlatforms returns the platform list enriched with the current user's
// download tokens and custom subscription status. This is the primary endpoint
// used by the Home page.
func UserPlatforms(c *gin.Context) {
	userID := middleware.GetUserID(c)
	isAdvanced := middleware.GetUserIsAdvanced(c)
	isAdmin := middleware.GetUserRole(c) == "admin"

	platforms, err := PlatformSvc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Determine the user's primary subscription type
	primaryType := "default"
	if isAdvanced {
		primaryType = "advanced"
	}

	result := make([]models.UserPlatformInfo, 0, len(platforms))
	for _, p := range platforms {
		info := models.UserPlatformInfo{
			ID:                 p.ID,
			Name:               p.Name,
			Description:        p.Description,
			ClientSchemes:      p.ClientSchemes,
			DownloadURL:        p.DownloadURL,
			DefaultConfigured:  SubSvc.SubscriptionExists(p.ID, "default"),
			AdvancedConfigured: SubSvc.SubscriptionExists(p.ID, "advanced"),
		}

		// Check for custom subscription (overrides regular type for this platform)
		customSub, customErr := CustomSubSvc.GetByUserAndPlatform(userID, p.ID)
		if customErr == nil && customSub != nil {
			info.HasCustomSub = true
			info.CustomSubID = customSub.ID
			// Generate custom token for this platform
			tok, tokErr := SubSvc.GetOrCreateCustomToken(userID, p.ID, customSub.ID)
			if tokErr == nil {
				info.DownloadToken = tok
			}

			// Admin users also get default + advanced preview tokens even when
			// a custom subscription exists (AGENTS.md §2.4).
			if isAdmin {
				info.SubType = primaryType
				if SubSvc.SubscriptionExists(p.ID, "default") {
					prevTok, prevErr := SubSvc.GetOrCreateToken(userID, p.ID, "default")
					if prevErr == nil {
						info.PreviewToken = prevTok
						info.PreviewSubType = "default"
					}
				}
				if SubSvc.SubscriptionExists(p.ID, "advanced") {
					prevTok, prevErr := SubSvc.GetOrCreateToken(userID, p.ID, "advanced")
					if prevErr == nil {
						info.PreviewToken2 = prevTok
						info.PreviewSubType2 = "advanced"
					}
				}
			}
		} else {
			// No custom subscription — use primary type
			info.SubType = primaryType
			if SubSvc.SubscriptionExists(p.ID, primaryType) {
				tok, tokErr := SubSvc.GetOrCreateToken(userID, p.ID, primaryType)
				if tokErr == nil {
					info.DownloadToken = tok
				}
			}

			// Admin users also get a token for the other type (preview)
			if isAdmin {
				otherType := "default"
				if primaryType == "default" {
					otherType = "advanced"
				}
				if SubSvc.SubscriptionExists(p.ID, otherType) {
					tok, tokErr := SubSvc.GetOrCreateToken(userID, p.ID, otherType)
					if tokErr == nil {
						info.PreviewToken = tok
						info.PreviewSubType = otherType
					}
				}
			}
		}

		result = append(result, info)
	}

	c.JSON(http.StatusOK, gin.H{"platforms": result})
}

// UserUpdateTime returns the most recent updated_at timestamp across all
// subscription current versions. Used by the Home page header.
func UserUpdateTime(c *gin.Context) {
	updateTime, err := SubSvc.GetUpdateTime()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if updateTime.IsZero() {
		c.JSON(http.StatusOK, gin.H{"update_time": ""})
		return
	}
	c.JSON(http.StatusOK, gin.H{"update_time": updateTime.Format(time.RFC3339)})
}

// UserRefreshToken rotates the download token for a given platform+type.
// If the user has a custom subscription for that platform, the custom token
// is rotated instead.
func UserRefreshToken(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req struct {
		Platform string `json:"platform"`
		Type     string `json:"type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	if req.Platform == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "platform is required"})
		return
	}

	// Check if user has a custom subscription for this platform
	customSub, customErr := CustomSubSvc.GetByUserAndPlatform(userID, req.Platform)
	if customErr == nil && customSub != nil {
		// Rotate custom subscription token
		if err := CustomSubSvc.RefreshToken(customSub.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// Generate a fresh token
		newToken, err := SubSvc.GetOrCreateCustomToken(userID, req.Platform, customSub.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "token": newToken, "type": "custom"})
		return
	}

	// Rotate regular subscription token
	if req.Type != "default" && req.Type != "advanced" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be 'default' or 'advanced'"})
		return
	}
	newToken, err := SubSvc.RefreshToken(userID, req.Platform, req.Type)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "token": newToken})
}

// ============================================================================
// Download Handlers
// ============================================================================

// handleJWTDownload returns subscription content via JWT auth (Web UI preview).
// Admin users may override the subscription type via ?type=default|advanced.
func handleJWTDownload(c *gin.Context) {
	platform := c.Param("platform")
	userID := middleware.GetUserID(c)

	// Determine subscription type
	isAdvanced := middleware.GetUserIsAdvanced(c)
	subType := "default"
	if isAdvanced {
		subType = "advanced"
	}

	// Admin may override via ?type= query param
	if middleware.GetUserRole(c) == "admin" {
		if t := c.Query("type"); t == "default" || t == "advanced" {
			subType = t
		}
	}

	content, err := SubSvc.GetCurrentContent(platform, subType)
	if err != nil {
		errorReason := errorReasonFromErr(err)
		logAccess(userID, c.ClientIP(), "subscription", platform, "", "", "failed", errorReason)
		c.String(http.StatusNotFound, err.Error())
		return
	}

	logAccess(userID, c.ClientIP(), "subscription", platform, "", "", "success", "")
	c.String(http.StatusOK, content)
}

// SubDownload returns the subscription content via JWT auth (Web UI preview).
func SubDownload(c *gin.Context) {
	handleJWTDownload(c)
}

// SubDownloadPreview is identical to SubDownload but routed under /download/preview.
func SubDownloadPreview(c *gin.Context) {
	handleJWTDownload(c)
}

// SubDownloadToken returns the subscription content via download token
// (used by VPN clients). Token may be for a regular or custom subscription.
func SubDownloadToken(c *gin.Context) {
	platform := c.Param("platform")
	token := c.Query("token")
	if token == "" {
		c.String(http.StatusBadRequest, "missing token")
		return
	}

	userID, plat, tokType, customSubID, err := SubSvc.FindToken(token)
	if err != nil {
		logAccess("", c.ClientIP(), "subscription", platform, "", "", "failed", "token_invalid")
		c.String(http.StatusUnauthorized, "invalid token")
		return
	}

	// Ensure URL platform matches the token's bound platform
	if platform != plat {
		logAccess(userID, c.ClientIP(), "subscription", platform, "", "", "failed", "token_invalid")
		c.String(http.StatusUnauthorized, "invalid token")
		return
	}

	var content string
	var downloadType string

	if customSubID != "" {
		// Custom subscription
		content, err = CustomSubSvc.GetCurrentContent(customSubID)
		downloadType = "custom"
	} else {
		// Regular subscription
		content, err = SubSvc.GetCurrentContent(plat, tokType)
		downloadType = "subscription"
	}

	if err != nil {
		errorReason := errorReasonFromErr(err)
		logAccess(userID, c.ClientIP(), downloadType, plat, "", "", "failed", errorReason)
		c.String(http.StatusNotFound, err.Error())
		return
	}

	logAccess(userID, c.ClientIP(), downloadType, plat, "", "", "success", "")
	setDownloadHeaders(c)
	c.String(http.StatusOK, content)
}

// ShareDownload returns a share subscription's content via share token.
// No authentication required — only the ?token= query param is verified.
func ShareDownload(c *gin.Context) {
	id := c.Param("id")
	token := c.Query("token")
	if token == "" {
		c.String(http.StatusBadRequest, "missing token")
		return
	}

	shareID, err := ShareSvc.ValidateToken(token)
	if err != nil || shareID != id {
		logAccess("", c.ClientIP(), "share", "", id, "", "failed", "token_invalid")
		c.String(http.StatusUnauthorized, "invalid token")
		return
	}

	content, err := ShareSvc.GetCurrentContent(id)
	if err != nil {
		errorReason := errorReasonFromErr(err)
		logAccess("", c.ClientIP(), "share", "", id, "", "failed", errorReason)
		c.String(http.StatusNotFound, err.Error())
		return
	}

	logAccess("", c.ClientIP(), "share", "", id, "", "success", "")
	setDownloadHeaders(c)
	c.String(http.StatusOK, content)
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
		ClientSecret string `json:"client_secret"` // optional in Normal mode
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

	// In Normal mode (already configured), an empty or masked client_secret
	// means the admin chose not to modify it. Pass empty to ConfigureSystem
	// so it preserves the existing encrypted value.
	// In Setup mode (not yet configured), client_secret is still required.
	clientSecret := req.ClientSecret
	if clientSecret == "" || clientSecret == "••••••" || clientSecret == "***" {
		isConfigured := false
		if val, err := cfgRepo.Get("configured"); err == nil && val == "true" {
			isConfigured = true
		}
		if !isConfigured {
			c.JSON(http.StatusBadRequest, gin.H{"error": "client_secret is required for initial setup"})
			return
		}
		clientSecret = "" // signal ConfigureSystem to preserve existing secret
	}

	_, err := auth.ConfigureSystem(cfgRepo, cfg, clientSecret)
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
	middleware.SetAuthService(svc)

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

	// Build issuer URL (consistent with getIssuerURL in oidc_service.go)
	var issuerURL string
	switch pt {
	case auth.ProviderKeycloak:
		if req.KeycloakBaseURL == "" || req.KeycloakRealm == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "keycloak_base_url and keycloak_realm are required"})
			return
		}
		issuerURL = strings.TrimRight(req.KeycloakBaseURL, "/") + "/realms/" + req.KeycloakRealm
	case auth.ProviderAuth0:
		if req.Auth0Domain == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "auth0_domain is required"})
			return
		}
		domain := req.Auth0Domain
		domain = strings.TrimPrefix(domain, "https://")
		domain = strings.TrimPrefix(domain, "http://")
		issuerURL = "https://" + domain
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
	users, err := UserSvc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Enrich with custom subscription info per user
	type userWithCustom struct {
		models.User
		HasCustomSub       bool     `json:"has_custom_sub"`
		CustomSubPlatforms []string `json:"custom_sub_platforms"`
	}
	result := make([]userWithCustom, 0, len(users))
	for _, u := range users {
		uwc := userWithCustom{User: u, CustomSubPlatforms: []string{}}
		customs, _ := CustomSubSvc.ListByUser(u.UserID)
		if len(customs) > 0 {
			uwc.HasCustomSub = true
			platforms := make([]string, 0, len(customs))
			for _, cs := range customs {
				platforms = append(platforms, cs.Platform)
			}
			uwc.CustomSubPlatforms = platforms
		}
		result = append(result, uwc)
	}
	c.JSON(http.StatusOK, gin.H{"users": result})
}

func GetUser(c *gin.Context) {
	user, err := UserSvc.Get(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user})
}

func UpdateUser(c *gin.Context) {
	var req struct {
		Username   string   `json:"username"`
		Email      string   `json:"email"`
		IsAdvanced *bool    `json:"is_advanced"` // pointer to distinguish "not provided" from false
		Groups     []string `json:"groups"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	operatorID := middleware.GetUserID(c)
	// Fetch existing user to preserve fields not provided in request
	existing, err := UserSvc.Get(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	target := &models.User{
		UserID:     c.Param("id"),
		Username:   existing.Username,
		Email:      existing.Email,
		IsAdvanced: existing.IsAdvanced, // preserve if not provided
		Groups:     existing.Groups,     // preserve if not provided
		Role:       "",                  // role must not be changed via this endpoint
	}
	if req.Username != "" {
		target.Username = req.Username
	}
	if req.Email != "" {
		target.Email = req.Email
	}
	if req.IsAdvanced != nil {
		target.IsAdvanced = *req.IsAdvanced
	}
	if req.Groups != nil {
		target.Groups = req.Groups
	}
	if err := UserSvc.Update(operatorID, target); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteUser(c *gin.Context) {
	operatorID := middleware.GetUserID(c)
	if err := UserSvc.Delete(operatorID, c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RevokeUserTokens(c *gin.Context) {
	if err := UserSvc.RevokeTokens(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UploadCustomSubscription(c *gin.Context) {
	userID := c.Param("id")
	platform := c.Query("platform")
	if platform == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "platform query parameter is required"})
		return
	}
	content, err := readUploadContent(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cs, err := CustomSubSvc.Upload(userID, platform, content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "custom_subscription": cs})
}

func UploadCustomSubscriptionVersion(c *gin.Context) {
	userID := c.Param("id")
	platform := c.Query("platform")
	if platform == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "platform query parameter is required"})
		return
	}
	cs, err := CustomSubSvc.GetByUserAndPlatform(userID, platform)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "custom subscription not found"})
		return
	}
	content, err := readUploadContent(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cs, err = CustomSubSvc.UploadVersion(cs.ID, content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "custom_subscription": cs})
}

func DeleteCustomSubscription(c *gin.Context) {
	userID := c.Param("id")
	platform := c.Query("platform")
	if platform == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "platform query parameter is required"})
		return
	}
	cs, err := CustomSubSvc.GetByUserAndPlatform(userID, platform)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "custom subscription not found"})
		return
	}
	if err := CustomSubSvc.Delete(cs.ID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetCustomVersion(c *gin.Context) {
	userID := c.Param("id")
	platform := c.Query("platform")
	if platform == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "platform query parameter is required"})
		return
	}
	cs, err := CustomSubSvc.GetByUserAndPlatform(userID, platform)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "custom subscription not found"})
		return
	}
	versionNum, err := parseVersionParam(c.Param("versionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid versionId"})
		return
	}
	content, ver, err := CustomSubSvc.GetVersionContent(cs.ID, versionNum)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"version": ver, "content": content})
}

func SwitchCustomVersion(c *gin.Context) {
	userID := c.Param("id")
	platform := c.Query("platform")
	if platform == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "platform query parameter is required"})
		return
	}
	cs, err := CustomSubSvc.GetByUserAndPlatform(userID, platform)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "custom subscription not found"})
		return
	}
	versionNum, err := parseVersionParam(c.Param("versionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid versionId"})
		return
	}
	cs, err = CustomSubSvc.SwitchVersion(cs.ID, versionNum)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "custom_subscription": cs})
}

func DeleteCustomVersion(c *gin.Context) {
	userID := c.Param("id")
	platform := c.Query("platform")
	if platform == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "platform query parameter is required"})
		return
	}
	cs, err := CustomSubSvc.GetByUserAndPlatform(userID, platform)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "custom subscription not found"})
		return
	}
	versionNum, err := parseVersionParam(c.Param("versionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid versionId"})
		return
	}
	cs, err = CustomSubSvc.DeleteVersion(cs.ID, versionNum)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "custom_subscription": cs})
}

func RefreshCustomSubToken(c *gin.Context) {
	// Find the custom subscription for the user+platform
	userID := c.Param("id")
	platform := c.Query("platform")
	if platform == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "platform query parameter is required"})
		return
	}
	cs, err := CustomSubSvc.GetByUserAndPlatform(userID, platform)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "custom subscription not found"})
		return
	}
	if err := CustomSubSvc.RefreshToken(cs.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ============================================================================
// Admin: Subscriptions
// ============================================================================

func ListSubscriptions(c *gin.Context) {
	subs, err := SubSvc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"subscriptions": subs})
}

func CreateSubscription(c *gin.Context) {
	var sub models.Subscription
	if err := c.ShouldBindJSON(&sub); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	if err := SubSvc.Create(&sub); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "subscription": sub})
}

func GetSubscription(c *gin.Context) {
	sub, err := SubSvc.Get(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Subscription not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"subscription": sub})
}

func UpdateSubscription(c *gin.Context) {
	var sub models.Subscription
	if err := c.ShouldBindJSON(&sub); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	sub.ID = c.Param("id")
	if err := SubSvc.Update(&sub); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteSubscription(c *gin.Context) {
	if err := SubSvc.Delete(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UploadSubscriptionVersion(c *gin.Context) {
	content, err := readUploadContent(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sub, err := SubSvc.UploadVersion(c.Param("id"), content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "subscription": sub})
}

func SwitchSubscriptionVersion(c *gin.Context) {
	versionNum, err := parseVersionParam(c.Param("versionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid versionId"})
		return
	}
	sub, err := SubSvc.SwitchVersion(c.Param("id"), versionNum)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "subscription": sub})
}

func GetSubscriptionVersion(c *gin.Context) {
	versionNum, err := parseVersionParam(c.Param("versionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid versionId"})
		return
	}
	content, ver, err := SubSvc.GetVersionContent(c.Param("id"), versionNum)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"version": ver, "content": content})
}

func DeleteSubscriptionVersion(c *gin.Context) {
	versionNum, err := parseVersionParam(c.Param("versionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid versionId"})
		return
	}
	sub, err := SubSvc.DeleteVersion(c.Param("id"), versionNum)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "subscription": sub})
}

// ============================================================================
// Admin: Share Subscriptions
// ============================================================================

func ListShares(c *gin.Context) {
	shares, err := ShareSvc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Enrich with token status and value
	type shareWithToken struct {
		models.ShareSubscription
		HasToken bool   `json:"has_token"`
		Token    string `json:"token,omitempty"`
	}
	result := make([]shareWithToken, 0, len(shares))
	for _, s := range shares {
		tok, tokErr := ShareSvc.GetToken(s.ID)
		result = append(result, shareWithToken{ShareSubscription: s, HasToken: tokErr == nil, Token: tok})
	}
	c.JSON(http.StatusOK, gin.H{"shares": result})
}

func CreateShare(c *gin.Context) {
	// Try JSON body first (name + content in one request)
	if strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		var req struct {
			Name    string `json:"name"`
			Content string `json:"content"`
		}
		if err := c.ShouldBindJSON(&req); err == nil && req.Name != "" && req.Content != "" {
			ss, token, err := ShareSvc.Create(req.Name, req.Content)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "share": ss, "token": token})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and content are required"})
		return
	}

	// Multipart file upload: name from form field, content from file
	content, err := readUploadContent(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	name := c.PostForm("name")
	if name == "" {
		name = c.Query("name")
	}
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	ss, token, err := ShareSvc.Create(name, content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "share": ss, "token": token})
}

func GetShare(c *gin.Context) {
	ss, err := ShareSvc.Get(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Share subscription not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"share": ss})
}

func UpdateShare(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	ss := &models.ShareSubscription{ID: c.Param("id"), Name: req.Name}
	if err := ShareSvc.Update(ss); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteShare(c *gin.Context) {
	if err := ShareSvc.Delete(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UploadShareVersion(c *gin.Context) {
	content, err := readUploadContent(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ss, err := ShareSvc.UploadVersion(c.Param("id"), content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "share": ss})
}

func SwitchShareVersion(c *gin.Context) {
	versionNum, err := parseVersionParam(c.Param("versionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid versionId"})
		return
	}
	ss, err := ShareSvc.SwitchVersion(c.Param("id"), versionNum)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "share": ss})
}

func GetShareVersion(c *gin.Context) {
	versionNum, err := parseVersionParam(c.Param("versionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid versionId"})
		return
	}
	content, ver, err := ShareSvc.GetVersionContent(c.Param("id"), versionNum)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"version": ver, "content": content})
}

func DeleteShareVersion(c *gin.Context) {
	versionNum, err := parseVersionParam(c.Param("versionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid versionId"})
		return
	}
	ss, err := ShareSvc.DeleteVersion(c.Param("id"), versionNum)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "share": ss})
}

func RefreshShareToken(c *gin.Context) {
	token, err := ShareSvc.RefreshToken(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "token": token})
}

func RevokeShareToken(c *gin.Context) {
	if err := ShareSvc.RevokeToken(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ============================================================================
// Admin: Platforms
// ============================================================================

func ListPlatforms(c *gin.Context) {
	platforms, err := PlatformSvc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"platforms": platforms})
}

func CreatePlatform(c *gin.Context) {
	var p models.Platform
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	if err := PlatformSvc.Create(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "platform": p})
}

func GetPlatform(c *gin.Context) {
	p, err := PlatformSvc.Get(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Platform not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"platform": p})
}

func UpdatePlatform(c *gin.Context) {
	var p models.Platform
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	p.ID = c.Param("id")
	if err := PlatformSvc.Update(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeletePlatform(c *gin.Context) {
	if err := PlatformSvc.Delete(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ============================================================================
// Admin: Rules
// ============================================================================

func ListAdminRules(c *gin.Context) {
	rules, err := RuleSvc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Enrich with token
	type ruleWithToken struct {
		models.Rule
		Token string `json:"token,omitempty"`
	}
	result := make([]ruleWithToken, 0, len(rules))
	for _, r := range rules {
		tok, _ := RuleSvc.GetToken(r.ID)
		result = append(result, ruleWithToken{Rule: r, Token: tok})
	}
	c.JSON(http.StatusOK, gin.H{"rules": result})
}

func CreateRule(c *gin.Context) {
	// Try JSON body first (id + name + client_type + optional content)
	if strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		var req struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			ClientType string `json:"client_type"`
			Content    string `json:"content"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}
		if req.ID == "" || req.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id and name are required"})
			return
		}
		if req.Content != "" {
			// One-step creation with first version
			rule, token, err := createRuleWithFirstVersion(req.ID, req.Name, req.ClientType, req.Content)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "rule": rule, "token": token})
			return
		}
		// Backward compatible: JSON without content — create empty rule record
		rule := &models.Rule{
			ID:         req.ID,
			Name:       req.Name,
			ClientType: req.ClientType,
		}
		if err := RuleSvc.Create(rule); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		token, _ := RuleSvc.RefreshToken(rule.ID)
		c.JSON(http.StatusOK, gin.H{"success": true, "rule": rule, "token": token})
		return
	}

	// Multipart file upload: id/name/client_type from form fields, content from file
	content, err := readUploadContent(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	id := c.PostForm("id")
	if id == "" {
		id = c.Query("id")
	}
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	name := c.PostForm("name")
	if name == "" {
		name = c.Query("name")
	}
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	clientType := c.PostForm("client_type")
	if clientType == "" {
		clientType = c.Query("client_type")
	}

	rule, token, err := createRuleWithFirstVersion(id, name, clientType, content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "rule": rule, "token": token})
}

// createRuleWithFirstVersion creates a rule record and uploads its first version.
// On failure after DB record creation, cleans up the rule record.
func createRuleWithFirstVersion(id, name, clientType, content string) (*models.Rule, string, error) {
	rule := &models.Rule{
		ID:         id,
		Name:       name,
		ClientType: clientType,
	}
	if err := RuleSvc.Create(rule); err != nil {
		return nil, "", err
	}

	// Upload first version; cleanup DB record on failure
	if _, err := RuleSvc.UploadVersion(rule.ID, content); err != nil {
		RuleSvc.Delete(rule.ID)
		return nil, "", fmt.Errorf("failed to create first version: %w", err)
	}

	// Generate rule token; cleanup DB record + version files on failure
	token, err := RuleSvc.RefreshToken(rule.ID)
	if err != nil {
		RuleSvc.Delete(rule.ID)
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	return rule, token, nil
}

func GetAdminRule(c *gin.Context) {
	rule, err := RuleSvc.Get(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Rule not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rule": rule})
}

func UpdateAdminRule(c *gin.Context) {
	var rule models.Rule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	rule.ID = c.Param("id")
	if err := RuleSvc.Update(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteAdminRule(c *gin.Context) {
	if err := RuleSvc.Delete(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UploadRuleVersion(c *gin.Context) {
	content, err := readUploadContent(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule, err := RuleSvc.UploadVersion(c.Param("id"), content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "rule": rule})
}

func SwitchRuleVersion(c *gin.Context) {
	versionNum, err := parseVersionParam(c.Param("versionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid versionId"})
		return
	}
	rule, err := RuleSvc.SwitchVersion(c.Param("id"), versionNum)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "rule": rule})
}

func GetRuleVersion(c *gin.Context) {
	versionNum, err := parseVersionParam(c.Param("versionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid versionId"})
		return
	}
	content, ver, err := RuleSvc.GetVersionContent(c.Param("id"), versionNum)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"version": ver, "content": content})
}

func DeleteRuleVersion(c *gin.Context) {
	versionNum, err := parseVersionParam(c.Param("versionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid versionId"})
		return
	}
	rule, err := RuleSvc.DeleteVersion(c.Param("id"), versionNum)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "rule": rule})
}

func RefreshRuleToken(c *gin.Context) {
	token, err := RuleSvc.RefreshToken(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "token": token})
}

// ============================================================================
// Admin: System Config
// ============================================================================

func GetRateLimit(c *gin.Context) {
	loginLimit, downloadLimit, err := SystemSvc.GetRateLimit()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rate_limit_login": loginLimit, "rate_limit_download": downloadLimit})
}

func UpdateRateLimit(c *gin.Context) {
	var req struct {
		RateLimitLogin    int `json:"rate_limit_login"`
		RateLimitDownload int `json:"rate_limit_download"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	if err := SystemSvc.UpdateRateLimit(req.RateLimitLogin, req.RateLimitDownload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ============================================================================
// Admin: Logs
// ============================================================================

func GetLogs(c *gin.Context) {
	date := c.Query("date")

	repo := repository.NewAccessLogRepo()
	var logs []repository.AccessLogRecord
	var err error

	if date != "" {
		// Specific date requested — filter by date(created_at)
		logs, err = repo.ListByDate(date)
	} else {
		// No date — return last 24 hours to avoid UTC/local timezone boundary issues
		logs, err = repo.ListRecent()
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query logs: " + err.Error()})
		return
	}

	// Ensure we never return null — always an empty array
	if logs == nil {
		logs = make([]repository.AccessLogRecord, 0)
	}

	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

// ============================================================================
// Helper functions
// ============================================================================

// readUploadContent reads file content from multipart upload or JSON text body.
func readUploadContent(c *gin.Context) (string, error) {
	contentType := c.GetHeader("Content-Type")

	// Handle multipart file upload
	if strings.HasPrefix(contentType, "multipart/form-data") {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 50*1024*1024) // 50MB limit
		file, _, err := c.Request.FormFile("file")
		if err != nil {
			return "", err
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	// Handle JSON text body
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 50*1024*1024) // 50MB limit
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		return "", err
	}
	if req.Content == "" {
		return "", fmt.Errorf("content is required")
	}
	return req.Content, nil
}

// parseVersionParam parses a version ID string to an integer.
func parseVersionParam(v string) (int, error) {
	return strconv.Atoi(v)
}

// setDownloadHeaders adds Clash Verge-compatible response headers to download
// endpoints. These headers are harmless for non-Clash clients (they ignore
// unrecognized headers).
func setDownloadHeaders(c *gin.Context) {
	c.Header("Content-Disposition", `attachment; filename*=UTF-8''Luneflare%20VPN%20Clash.yaml`)
	c.Header("profile-update-interval", "300")
	cfgRepo := repository.NewSystemConfigRepo()
	if frontendURL, err := cfgRepo.Get("frontend_url"); err == nil && frontendURL != "" {
		c.Header("profile-web-page-url", frontendURL)
	}
}

// logAccess records a download access log entry. This is a best-effort helper
// — failures are silently ignored so they never affect the download response.
// When userID is non-empty, the user's email is looked up and stored instead of
// the raw UUID for human readability in the logs page.
func logAccess(userID, ip, downloadType, platform, shareSubID, ruleID, status, errorReason string) {
	identifier := userID
	if userID != "" {
		userRepo := repository.NewUserRepo()
		if u, err := userRepo.FindByID(userID); err == nil && u.Email != "" {
			identifier = u.Email
		} else if err != nil {
			log.Printf("[DEBUG] logAccess: FindByID(%q) failed: %v", userID, err)
		} else {
			log.Printf("[DEBUG] logAccess: FindByID(%q) OK but email empty (username=%q)", userID, u.Username)
		}
	}
	repo := repository.NewAccessLogRepo()
	_ = repo.Insert(&repository.AccessLogRecord{
		UserID:              identifier,
		IP:                  ip,
		DownloadType:        downloadType,
		Platform:            platform,
		ShareSubscriptionID: shareSubID,
		RuleID:              ruleID,
		Status:              status,
		ErrorReason:         errorReason,
	})
}

// errorReasonFromErr maps a service-layer error to an access_log error_reason.
// Distinguishes "no versions configured" (version_not_found) from other errors
// like resource-not-found or file-read failures (file_not_found).
func errorReasonFromErr(err error) string {
	if err == nil {
		return ""
	}
	if strings.Contains(err.Error(), "no versions") {
		return "version_not_found"
	}
	return "file_not_found"
}
