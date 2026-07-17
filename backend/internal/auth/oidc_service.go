package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"vpn-sub/internal/models"
	"vpn-sub/internal/repository"
	"vpn-sub/internal/utils"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

// ============================================================================
// Types
// ============================================================================

type ProviderType string

const (
	ProviderKeycloak ProviderType = "keycloak"
	ProviderAuth0    ProviderType = "auth0"
	ProviderGeneric  ProviderType = "generic"
)

// OIDCConfig holds the OIDC provider configuration read from system_config.
type OIDCConfig struct {
	ProviderType                  ProviderType
	KeycloakBaseURL               string
	KeycloakRealm                 string
	Auth0Domain                   string
	GenericIssuer                 string
	ClientID                      string
	KeycloakClientSecretEncrypted string
	Auth0ClientSecretEncrypted    string
	GenericClientSecretEncrypted  string
	RedirectURI                   string
	FrontendURL                   string
}

// Service is the main OIDC authentication service.
// It implements middleware.AuthService for JWT validation.
type Service struct {
	provider     *oidc.Provider
	oauth2Config *oauth2.Config
	jwtSecret    []byte
	aesKey       []byte // derived from jwtSecret, first 32 bytes
	frontendURL  string
	providerType ProviderType
	cfgRepo      *repository.SystemConfigRepo
	userRepo     *repository.UserRepo
	stateRepo    *repository.OIDCStateRepo
}

// DefaultService is the package-level singleton set during app initialization.
var DefaultService *Service

// ============================================================================
// Constructor
// ============================================================================

// NewServiceFromDB reads OIDC config and JWT secret from the database and
// creates a new Service. Used during normal (configured) mode.
func NewServiceFromDB(cfgRepo *repository.SystemConfigRepo) (*Service, error) {
	// Read JWT_SECRET
	jwtSecret, err := cfgRepo.Get("JWT_SECRET")
	if err != nil || jwtSecret == "" {
		return nil, errors.New("JWT_SECRET not found in system_config")
	}

	// Read OIDC config
	cfg, err := readOIDCConfig(cfgRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to read OIDC config: %w", err)
	}

	return newService(cfg, jwtSecret)
}

// NewServiceFromParams creates a Service from raw OIDC parameters (not from DB).
// Used during setup/testing. clientSecret should be the raw (unencrypted) secret.
func NewServiceFromParams(providerType ProviderType, issuerURL, clientID, clientSecret, redirectURI, frontendURL string) (*Service, error) {
	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	oauth2Config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURI,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	svc := &Service{
		provider:     provider,
		oauth2Config: oauth2Config,
		frontendURL:  frontendURL,
		providerType: providerType,
		cfgRepo:      repository.NewSystemConfigRepo(),
		userRepo:     repository.NewUserRepo(),
		stateRepo:    repository.NewOIDCStateRepo(),
	}
	return svc, nil
}

// newService is the internal constructor used by both public constructors.
func newService(cfg *OIDCConfig, jwtSecret string) (*Service, error) {
	ctx := context.Background()

	issuerURL := getIssuerURL(cfg)
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider at %s: %w", issuerURL, err)
	}

	// Decrypt the appropriate client secret based on provider type
	aesKey := utils.AESKeyFromSecret(jwtSecret)
	clientSecret, err := getClientSecret(cfg, aesKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt client secret: %w", err)
	}

	oauth2Config := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: clientSecret,
		RedirectURL:  cfg.RedirectURI,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	svc := &Service{
		provider:     provider,
		oauth2Config: oauth2Config,
		jwtSecret:    []byte(jwtSecret),
		aesKey:       aesKey,
		frontendURL:  cfg.FrontendURL,
		providerType: cfg.ProviderType,
		cfgRepo:      repository.NewSystemConfigRepo(),
		userRepo:     repository.NewUserRepo(),
		stateRepo:    repository.NewOIDCStateRepo(),
	}
	return svc, nil
}

// ============================================================================
// OIDC Config Helpers
// ============================================================================

func getIssuerURL(cfg *OIDCConfig) string {
	switch cfg.ProviderType {
	case ProviderKeycloak:
		return strings.TrimRight(cfg.KeycloakBaseURL, "/") + "/realms/" + cfg.KeycloakRealm
	case ProviderAuth0:
		domain := cfg.Auth0Domain
		domain = strings.TrimPrefix(domain, "https://")
		domain = strings.TrimPrefix(domain, "http://")
		return "https://" + domain
	case ProviderGeneric:
		return cfg.GenericIssuer
	default:
		return cfg.GenericIssuer
	}
}

func getClientSecret(cfg *OIDCConfig, aesKey []byte) (string, error) {
	var encrypted string
	switch cfg.ProviderType {
	case ProviderKeycloak:
		encrypted = cfg.KeycloakClientSecretEncrypted
	case ProviderAuth0:
		encrypted = cfg.Auth0ClientSecretEncrypted
	case ProviderGeneric:
		encrypted = cfg.GenericClientSecretEncrypted
	default:
		return "", errors.New("unknown provider type")
	}
	if encrypted == "" {
		return "", errors.New("client secret not configured")
	}
	return utils.DecryptAES(encrypted, aesKey)
}

// readOIDCConfig reads all OIDC-related keys from system_config.
func readOIDCConfig(cfgRepo *repository.SystemConfigRepo) (*OIDCConfig, error) {
	get := func(key string) string {
		v, _ := cfgRepo.Get(key)
		return v
	}

	cfg := &OIDCConfig{
		ProviderType:                  ProviderType(get("provider_type")),
		KeycloakBaseURL:               get("keycloak_base_url"),
		KeycloakRealm:                 get("keycloak_realm"),
		Auth0Domain:                   get("auth0_domain"),
		GenericIssuer:                 get("generic_issuer"),
		ClientID:                      get("client_id"),
		KeycloakClientSecretEncrypted: get("keycloak_client_secret_encrypted"),
		Auth0ClientSecretEncrypted:    get("auth0_client_secret_encrypted"),
		GenericClientSecretEncrypted:  get("generic_client_secret_encrypted"),
		RedirectURI:                   get("redirect_uri"),
		FrontendURL:                   get("frontend_url"),
	}

	if cfg.ProviderType == "" {
		return nil, errors.New("provider_type not configured")
	}
	return cfg, nil
}

// ============================================================================
// PKCE
// ============================================================================

// GeneratePKCE creates a new code_verifier and its S256 code_challenge.
func GeneratePKCE() (codeVerifier string, codeChallenge string, err error) {
	// Generate 32 random bytes, base64url-encode (without padding)
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	codeVerifier = base64.RawURLEncoding.EncodeToString(b)

	// SHA256 of code_verifier
	h := sha256.Sum256([]byte(codeVerifier))
	codeChallenge = base64.RawURLEncoding.EncodeToString(h[:])
	return codeVerifier, codeChallenge, nil
}

// GenerateState creates a random state string for CSRF protection.
func GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ============================================================================
// Auth Flow
// ============================================================================

// InitiateLoginResult contains the values needed by the handler after
// initiating an OIDC login.
type InitiateLoginResult struct {
	RedirectURL string // full OIDC provider auth URL
	State       string // state value (also stored in cookie and DB)
}

// InitiateLogin generates the OIDC authorization URL with PKCE.
// It stores state + code_verifier in the oidc_state table.
func (s *Service) InitiateLogin() (*InitiateLoginResult, error) {
	state, err := GenerateState()
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}

	codeVerifier, codeChallenge, err := GeneratePKCE()
	if err != nil {
		return nil, fmt.Errorf("failed to generate PKCE: %w", err)
	}

	// Store state + code_verifier in DB (10-minute TTL via periodic cleanup)
	if err := s.stateRepo.Create(state, codeVerifier, ""); err != nil {
		return nil, fmt.Errorf("failed to store OIDC state: %w", err)
	}

	authURL := s.oauth2Config.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)

	return &InitiateLoginResult{
		RedirectURL: authURL,
		State:       state,
	}, nil
}

// CallbackResult contains the result of a successful OIDC callback.
type CallbackResult struct {
	UserID      string
	JWT         string
	FrontendURL string
}

// HandleCallback processes the OIDC callback.
// It performs triple verification (cookie state == query state == DB record),
// exchanges the code for tokens, creates or finds the user, handles first-admin
// logic, and generates a JWT.
func (s *Service) HandleCallback(ctx context.Context, queryState, cookieState, code string) (*CallbackResult, error) {
	// Triple verification
	if queryState == "" || cookieState == "" {
		return nil, errors.New("missing state parameter")
	}
	if queryState != cookieState {
		return nil, errors.New("state mismatch: cookie != query")
	}

	// Look up state in DB and get code_verifier
	codeVerifier, _, err := s.stateRepo.FindByState(queryState)
	if err != nil {
		return nil, fmt.Errorf("state not found or expired: %w", err)
	}

	// Delete state record immediately to prevent replay
	if err := s.stateRepo.Delete(queryState); err != nil {
		return nil, fmt.Errorf("failed to delete state: %w", err)
	}

	// Exchange authorization code for tokens, using PKCE code_verifier
	oauth2Token, err := s.oauth2Config.Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", codeVerifier),
	)
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %w", err)
	}

	// Extract and verify the ID token
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("no id_token in response")
	}

	verifier := s.provider.Verifier(&oidc.Config{ClientID: s.oauth2Config.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("id_token verification failed: %w", err)
	}

	// Extract claims
	var claims struct {
		Sub      string `json:"sub"`
		Email    string `json:"email"`
		Name     string `json:"name"`
		Username string `json:"preferred_username"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse ID token claims: %w", err)
	}

	if claims.Sub == "" {
		return nil, errors.New("missing sub claim in ID token")
	}

	// Fallback for username
	username := claims.Username
	if username == "" {
		username = claims.Name
	}
	if username == "" {
		username = claims.Email
	}

	// Find or create user
	user, err := s.userRepo.FindByID(claims.Sub)
	isNewUser := false
	if err != nil {
		// User doesn't exist — create new
		isAdvanced := false
		role := "user"

		// Check if this is the first admin
		adminInit, _ := s.cfgRepo.Get("admin_initialized")
		if adminInit != "true" {
			role = "admin"
			isAdvanced = true
			// Set admin_initialized to true
			if err := s.cfgRepo.Set("admin_initialized", "true"); err != nil {
				return nil, fmt.Errorf("failed to set admin_initialized: %w", err)
			}
		}

		user = &models.User{
			UserID:     claims.Sub,
			Username:   username,
			Email:      claims.Email,
			Role:       role,
			IsAdvanced: isAdvanced,
			Groups:     []string{},
		}
		if err := s.userRepo.Create(user); err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
		isNewUser = true
		_ = isNewUser // reserved for logging
	}

	// Generate JWT
	jwtToken, err := s.GenerateJWT(user.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate JWT: %w", err)
	}

	return &CallbackResult{
		UserID:      user.UserID,
		JWT:         jwtToken,
		FrontendURL: s.frontendURL,
	}, nil
}

// ============================================================================
// JWT
// ============================================================================

// GenerateJWT creates a signed JWT for the given user_id.
// The token contains only user_id in claims + standard exp/iat.
// Validity: 7 days.
func (s *Service) GenerateJWT(userID string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"user_id": userID,
		"iat":     now.Unix(),
		"exp":     now.Add(7 * 24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// ValidateJWT parses and validates a JWT token string, returning the user_id.
// This implements the middleware.AuthService interface.
func (s *Service) ValidateJWT(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return "", fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", errors.New("invalid token claims")
	}

	userID, ok := claims["user_id"].(string)
	if !ok || userID == "" {
		return "", errors.New("missing user_id in token")
	}

	return userID, nil
}

// ============================================================================
// Test OIDC Connection
// ============================================================================

// TestConnection attempts to create an OIDC provider and verify connectivity.
// Returns an error if the provider is unreachable or misconfigured.
func TestConnection(providerType ProviderType, issuerURL, clientID, clientSecret, redirectURI string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return fmt.Errorf("failed to discover OIDC provider at %s: %w", issuerURL, err)
	}

	// Just verifying we can create the provider is enough for a connection test
	_ = provider
	_ = clientID
	_ = clientSecret
	_ = redirectURI

	return nil
}

// ============================================================================
// Setup: Configure System
// ============================================================================

// ConfigureSystemResult holds the result of configuring the system.
type ConfigureSystemResult struct {
	JWTSecret string // the generated JWT secret (may be needed)
}

// ConfigureSystem stores OIDC configuration and ensures JWT_SECRET exists.
// On first setup (JWT_SECRET not yet set), a new secret is generated.
// On subsequent calls (e.g. OIDC config page save), the existing JWT_SECRET is
// reused so that existing JWTs and encrypted provider secrets remain valid.
// When rawClientSecret is empty (admin chose not to modify the secret),
// the existing encrypted secret is preserved unchanged.
func ConfigureSystem(cfgRepo *repository.SystemConfigRepo, cfg *OIDCConfig, rawClientSecret string) (*ConfigureSystemResult, error) {
	// Check if JWT_SECRET already exists — reuse it if so, generate only on first setup
	jwtSecret, err := cfgRepo.Get("JWT_SECRET")
	if err != nil || jwtSecret == "" {
		// First-time setup: generate a new JWT_SECRET (at least 32 bytes = 256 bits)
		jwtSecretBytes, genErr := utils.GenerateRandomBytes(32)
		if genErr != nil {
			return nil, fmt.Errorf("failed to generate JWT_SECRET: %w", genErr)
		}
		jwtSecret = base64.RawURLEncoding.EncodeToString(jwtSecretBytes)
	}

	// Derive AES key from JWT_SECRET
	aesKey := utils.AESKeyFromSecret(jwtSecret)

	// Determine which provider-specific key to use for the encrypted secret
	secretKey := getSecretKeyForProvider(cfg.ProviderType)

	// Write all config to system_config
	configs := map[string]string{
		"JWT_SECRET":    jwtSecret,
		"provider_type": string(cfg.ProviderType),
		"client_id":     cfg.ClientID,
		"redirect_uri":  cfg.RedirectURI,
		"frontend_url":  cfg.FrontendURL,
		"configured":    "true",
		// admin_initialized is NOT set here — it's set by the first user login
	}

	// Only encrypt and store a new client secret if one was provided.
	// An empty rawClientSecret means the admin chose not to modify it (Normal mode),
	// so we skip the secret fields entirely and preserve the existing encrypted value.
	if rawClientSecret != "" {
		encryptedSecret, encErr := utils.EncryptAES(rawClientSecret, aesKey)
		if encErr != nil {
			return nil, fmt.Errorf("failed to encrypt client secret: %w", encErr)
		}
		configs[secretKey] = encryptedSecret
	}

	switch cfg.ProviderType {
	case ProviderKeycloak:
		configs["keycloak_base_url"] = cfg.KeycloakBaseURL
		configs["keycloak_realm"] = cfg.KeycloakRealm
	case ProviderAuth0:
		configs["auth0_domain"] = cfg.Auth0Domain
	case ProviderGeneric:
		configs["generic_issuer"] = cfg.GenericIssuer
	}

	for k, v := range configs {
		if err := cfgRepo.Set(k, v); err != nil {
			return nil, fmt.Errorf("failed to set %s: %w", k, err)
		}
	}

	return &ConfigureSystemResult{JWTSecret: jwtSecret}, nil
}

// SwitchProvider updates only the provider_type in system_config.
// Existing provider-specific fields are preserved.
func SwitchProvider(cfgRepo *repository.SystemConfigRepo, newProviderType ProviderType) error {
	return cfgRepo.Set("provider_type", string(newProviderType))
}

// getSecretKeyForProvider returns the system_config key for the encrypted
// client secret based on the provider type.
func getSecretKeyForProvider(pt ProviderType) string {
	switch pt {
	case ProviderKeycloak:
		return "keycloak_client_secret_encrypted"
	case ProviderAuth0:
		return "auth0_client_secret_encrypted"
	case ProviderGeneric:
		return "generic_client_secret_encrypted"
	default:
		return "generic_client_secret_encrypted"
	}
}

// ============================================================================
// GetCurrentUser
// ============================================================================

// GetCurrentUser returns the user model for the given user ID.
// Always reads from the database (never uses JWT claims).
func GetCurrentUser(userID string) (*models.User, error) {
	repo := repository.NewUserRepo()
	return repo.FindByID(userID)
}

// GetMaskedOIDCConfig returns the OIDC config with client secret masked for UI display.
func GetMaskedOIDCConfig(cfgRepo *repository.SystemConfigRepo) (map[string]string, error) {
	result := make(map[string]string)
	keys := []string{
		"provider_type", "keycloak_base_url", "keycloak_realm",
		"auth0_domain", "generic_issuer", "client_id",
		"redirect_uri", "frontend_url",
	}
	for _, k := range keys {
		v, _ := cfgRepo.Get(k)
		result[k] = v
	}
	// Mask the secret — show "••••••" if configured.
	// Return both the provider-specific key AND a generic "client_secret" key
	// so the frontend can read it without knowing the provider type.
	secretKey := getSecretKeyForProvider(ProviderType(result["provider_type"]))
	if v, err := cfgRepo.Get(secretKey); err == nil && v != "" {
		result[secretKey] = "••••••"
		result["client_secret"] = "••••••"
	} else {
		result[secretKey] = ""
		result["client_secret"] = ""
	}
	return result, nil
}
