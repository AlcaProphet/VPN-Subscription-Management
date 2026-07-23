package service

import (
	"fmt"
	"strconv"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/repository"
)

// SystemService handles system configuration business logic.
type SystemService struct {
	cfgRepo *repository.SystemConfigRepo
}

func NewSystemService() *SystemService {
	return &SystemService{
		cfgRepo: repository.NewSystemConfigRepo(),
	}
}

// GetRateLimit returns the current rate limit configuration.
func (s *SystemService) GetRateLimit() (loginLimit, downloadLimit int, err error) {
	loginLimit = 10
	downloadLimit = 20

	if val, err := s.cfgRepo.Get("rate_limit_login"); err == nil && val != "" {
		if n, parseErr := strconv.Atoi(val); parseErr == nil && n > 0 {
			loginLimit = n
		}
	}
	if val, err := s.cfgRepo.Get("rate_limit_download"); err == nil && val != "" {
		if n, parseErr := strconv.Atoi(val); parseErr == nil && n > 0 {
			downloadLimit = n
		}
	}
	return
}

// UpdateRateLimit updates the rate limit configuration.
func (s *SystemService) UpdateRateLimit(loginLimit, downloadLimit int) error {
	if loginLimit < 1 || downloadLimit < 1 {
		return fmt.Errorf("rate limits must be positive")
	}
	if err := s.cfgRepo.Set("rate_limit_login", strconv.Itoa(loginLimit)); err != nil {
		return err
	}
	if err := s.cfgRepo.Set("rate_limit_download", strconv.Itoa(downloadLimit)); err != nil {
		return err
	}
	return nil
}

// GetOIDCConfig returns the masked OIDC configuration.
func (s *SystemService) GetOIDCConfig() (map[string]string, error) {
	return auth.GetMaskedOIDCConfig(s.cfgRepo)
}

// GetAnnouncement returns the current announcement content.
func (s *SystemService) GetAnnouncement() (string, error) {
	val, err := s.cfgRepo.Get("announcement_content")
	if err != nil {
		return "", nil // key not found = no announcement
	}
	return val, nil
}

// SetAnnouncement updates the announcement content.
func (s *SystemService) SetAnnouncement(content string) error {
	return s.cfgRepo.Set("announcement_content", content)
}

// GetDebugMode returns whether debug mode is enabled.
// When enabled, 5xx errors include detailed internal messages in responses.
// Stored in system_config with key "debug_mode".
func (s *SystemService) GetDebugMode() bool {
	val, err := s.cfgRepo.Get("debug_mode")
	return err == nil && val == "true"
}

// SetDebugMode enables or disables debug mode.
func (s *SystemService) SetDebugMode(enabled bool) error {
	val := "false"
	if enabled {
		val = "true"
	}
	return s.cfgRepo.Set("debug_mode", val)
}
