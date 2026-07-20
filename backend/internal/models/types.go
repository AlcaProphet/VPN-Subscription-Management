package models

import "time"

// ============================================================================
// System Config
// ============================================================================

type SystemConfig struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ============================================================================
// User
// ============================================================================

type User struct {
	UserID     string   `json:"user_id"`
	Username   string   `json:"username"`
	Email      string   `json:"email"`
	Role       string   `json:"role"` // "admin" or "user"
	IsAdvanced bool     `json:"is_advanced"`
	Groups     []string `json:"groups"` // JSON array, reserved for future use
}

// ============================================================================
// Platform
// ============================================================================

type Platform struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	ClientSchemes []string `json:"client_schemes"` // JSON array, e.g. ["clash://install-config?url="]
	DownloadURL   string   `json:"download_url"`   // optional, nullable
}

// ============================================================================
// Subscription
// ============================================================================

type Subscription struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Platform string    `json:"platform"`
	Type     string    `json:"type"`     // "default" or "advanced", UNIQUE(platform, type)
	Versions []Version `json:"versions"` // JSON array of version objects
}

// ============================================================================
// Version (used by subscriptions, rules, custom_subscriptions, share_subscriptions)
// ============================================================================

type Version struct {
	Version   int       `json:"version"`
	FilePath  string    `json:"file_path"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ============================================================================
// Rule
// ============================================================================

type Rule struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	ClientType string    `json:"client_type"` // reserved for future expansion, e.g. "shadowrocket"
	Versions   []Version `json:"versions"`    // JSON array
	CreatedAt  string    `json:"created_at"`  // SQLite TEXT, e.g. "2026-07-20 06:31:59"
}

// ============================================================================
// Access Log
// ============================================================================

type AccessLog struct {
	ID                  int64     `json:"id"`
	UserID              string    `json:"user_id"` // nullable
	IP                  string    `json:"ip"`
	DownloadType        string    `json:"download_type"`         // "subscription", "share", "custom", "rule"
	Platform            string    `json:"platform"`              // nullable
	ShareSubscriptionID string    `json:"share_subscription_id"` // nullable
	RuleID              string    `json:"rule_id"`               // nullable
	Status              string    `json:"status"`                // "success" or "failed"
	ErrorReason         string    `json:"error_reason"`          // nullable, e.g. "token_invalid"
	CreatedAt           time.Time `json:"created_at"`
}

// ============================================================================
// OIDC State
// ============================================================================

type OIDCState struct {
	State        string    `json:"state"`
	CodeVerifier string    `json:"code_verifier"`
	Nonce        string    `json:"nonce"`
	CreatedAt    time.Time `json:"created_at"`
}

// ============================================================================
// Download Token
// ============================================================================

type DownloadToken struct {
	Token       string    `json:"token"`
	UserID      string    `json:"user_id"`
	Platform    string    `json:"platform"`
	Type        *string   `json:"type"`          // "default" or "advanced"; NULL when custom_sub_id is non-null
	CustomSubID *string   `json:"custom_sub_id"` // nullable, FK to custom_subscriptions
	CreatedAt   time.Time `json:"created_at"`
}

// ============================================================================
// Custom Subscription
// ============================================================================

type CustomSubscription struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Platform  string    `json:"platform"`
	Versions  []Version `json:"versions"`   // JSON array
	CreatedAt string    `json:"created_at"` // SQLite TEXT
}

// ============================================================================
// Share Subscription
// ============================================================================

type ShareSubscription struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Versions  []Version `json:"versions"`   // JSON array
	CreatedAt string    `json:"created_at"` // SQLite TEXT
}

// ============================================================================
// Share Token
// ============================================================================

type ShareToken struct {
	Token               string `json:"token"`
	ShareSubscriptionID string `json:"share_subscription_id"`
	CreatedAt           string `json:"created_at"` // SQLite TEXT
}

// ============================================================================
// Rule Token
// ============================================================================

type RuleToken struct {
	Token     string `json:"token"`
	RuleID    string `json:"rule_id"`
	CreatedAt string `json:"created_at"` // SQLite TEXT
}

// ============================================================================
// Request / Response helpers
// ============================================================================

// UserPlatformInfo is returned by GET /user/platforms — platform info
// plus the user's download token and custom subscription status.
type UserPlatformInfo struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	ClientSchemes      []string `json:"client_schemes"`
	DownloadURL        string   `json:"download_url"`
	HasCustomSub       bool     `json:"has_custom_sub"`
	CustomSubID        string   `json:"custom_sub_id,omitempty"`
	DownloadToken      string   `json:"download_token,omitempty"`    // user's primary token
	PreviewToken       string   `json:"preview_token,omitempty"`     // admin: token for the first preview type
	PreviewSubType     string   `json:"preview_sub_type,omitempty"`  // "default" or "advanced" for first preview token
	PreviewToken2      string   `json:"preview_token2,omitempty"`    // admin: token for the second preview type (when custom sub exists)
	PreviewSubType2    string   `json:"preview_sub_type2,omitempty"` // "default" or "advanced" for second preview token
	SubType            string   `json:"sub_type,omitempty"`          // "default", "advanced", or empty for custom-only
	DefaultConfigured  bool     `json:"default_configured"`          // whether a default sub exists for this platform
	AdvancedConfigured bool     `json:"advanced_configured"`         // whether an advanced sub exists for this platform
}
