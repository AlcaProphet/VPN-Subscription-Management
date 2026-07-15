package service

import (
	"fmt"

	"vpn-sub/internal/models"
	"vpn-sub/internal/repository"
	"vpn-sub/internal/utils"
)

// PlatformService handles platform business logic.
type PlatformService struct {
	repo       *repository.PlatformRepo
	subRepo    *repository.SubscriptionRepo
	tokenRepo  *repository.DownloadTokenRepo
	customRepo *repository.CustomSubscriptionRepo
	versionSvc *VersionService
}

// NewPlatformService creates a new PlatformService.
func NewPlatformService(versionSvc *VersionService) *PlatformService {
	return &PlatformService{
		repo:       repository.NewPlatformRepo(),
		subRepo:    repository.NewSubscriptionRepo(),
		tokenRepo:  repository.NewDownloadTokenRepo(),
		customRepo: repository.NewCustomSubscriptionRepo(),
		versionSvc: versionSvc,
	}
}

// List returns all platforms.
func (s *PlatformService) List() ([]models.Platform, error) {
	return s.repo.List()
}

// Get returns a platform by ID.
func (s *PlatformService) Get(id string) (*models.Platform, error) {
	return s.repo.FindByID(id)
}

// Create creates a new platform.
func (s *PlatformService) Create(p *models.Platform) error {
	if !utils.IsValidID(p.ID) {
		return fmt.Errorf("invalid platform ID: must be [a-z0-9-]+")
	}
	if p.Name == "" {
		return fmt.Errorf("name is required")
	}
	// Check for duplicate
	if existing, _ := s.repo.FindByID(p.ID); existing != nil {
		return fmt.Errorf("platform with ID %q already exists", p.ID)
	}
	return s.repo.Create(p)
}

// Update updates a platform's fields.
func (s *PlatformService) Update(p *models.Platform) error {
	if p.Name == "" {
		return fmt.Errorf("name is required")
	}
	return s.repo.Update(p)
}

// Delete deletes a platform and cascades: subscriptions, download tokens, custom subscriptions.
func (s *PlatformService) Delete(id string) error {
	// Check platform exists
	if _, err := s.repo.FindByID(id); err != nil {
		return fmt.Errorf("platform not found")
	}

	// Delete subscriptions and their version files
	subs, _ := s.subRepo.ListByPlatform(id)
	for _, sub := range subs {
		s.versionSvc.RemoveVersionDir("subscriptions/" + sub.ID)
		s.subRepo.Delete(sub.ID)
	}

	// Delete download tokens for this platform
	s.tokenRepo.DeleteByPlatform(id)

	// Delete custom subscriptions for this platform and their version files
	// Query all custom subs for this platform, delete version files, then DB records
	rows, err := repository.DB.Query(
		`SELECT user_id, platform FROM custom_subscriptions WHERE platform = ?`, id,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var userID, platform string
			if err := rows.Scan(&userID, &platform); err == nil {
				s.versionSvc.RemoveVersionDir("custom/" + userID + "/" + platform)
			}
		}
	}
	repository.DB.Exec(`DELETE FROM custom_subscriptions WHERE platform = ?`, id)

	// Delete platform
	return s.repo.Delete(id)
}
