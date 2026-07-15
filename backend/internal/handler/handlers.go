package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

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
	c.JSON(http.StatusOK, gin.H{"rules": rules})
}

func GetRuleDownload(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.String(http.StatusBadRequest, "missing token")
		return
	}
	ruleID, err := RuleSvc.ValidateToken(token)
	if err != nil {
		c.String(http.StatusUnauthorized, "invalid token")
		return
	}
	content, err := RuleSvc.GetCurrentContent(ruleID)
	if err != nil {
		c.String(http.StatusNotFound, "rule not found")
		return
	}
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

	result, err := svc.InitiateLogin()
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
		isSecure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
		c.SetCookie("oidc_state", "", -1, "/", "", isSecure, true)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication failed: " + err.Error()})
		return
	}

	// Clear the state cookie on success
	isSecure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
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
		issuerURL = "https://" + strings.TrimLeft(req.Auth0Domain, "https://")
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
	c.JSON(http.StatusOK, gin.H{"users": users})
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
		Username:   req.Username,
		Email:      req.Email,
		IsAdvanced: existing.IsAdvanced, // preserve if not provided
		Groups:     existing.Groups,     // preserve if not provided
		Role:       "",                  // role must not be changed via this endpoint
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
	id := c.Param("id")
	content, err := readUploadContent(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cs, err := CustomSubSvc.UploadVersion(id, content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "custom_subscription": cs})
}

func DeleteCustomSubscription(c *gin.Context) {
	if err := CustomSubSvc.Delete(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetCustomVersion(c *gin.Context) {
	versionNum, err := parseVersionParam(c.Param("versionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid versionId"})
		return
	}
	content, ver, err := CustomSubSvc.GetVersionContent(c.Param("id"), versionNum)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"version": ver, "content": content})
}

func SwitchCustomVersion(c *gin.Context) {
	versionNum, err := parseVersionParam(c.Param("versionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid versionId"})
		return
	}
	cs, err := CustomSubSvc.SwitchVersion(c.Param("id"), versionNum)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "custom_subscription": cs})
}

func DeleteCustomVersion(c *gin.Context) {
	versionNum, err := parseVersionParam(c.Param("versionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid versionId"})
		return
	}
	cs, err := CustomSubSvc.DeleteVersion(c.Param("id"), versionNum)
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
	// Enrich with token status
	type shareWithToken struct {
		models.ShareSubscription
		HasToken bool `json:"has_token"`
	}
	result := make([]shareWithToken, 0, len(shares))
	for _, s := range shares {
		_, tokErr := ShareSvc.GetToken(s.ID)
		result = append(result, shareWithToken{ShareSubscription: s, HasToken: tokErr == nil})
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
	var rule models.Rule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	if err := RuleSvc.Create(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Auto-generate rule token per AGENTS.md
	token, _ := RuleSvc.RefreshToken(rule.ID)
	c.JSON(http.StatusOK, gin.H{"success": true, "rule": rule, "token": token})
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
	c.JSON(http.StatusOK, gin.H{"logs": []interface{}{}})
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
