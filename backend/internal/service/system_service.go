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
