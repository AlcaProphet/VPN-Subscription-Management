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
// Files are deleted AFTER the DB transaction commits to avoid orphaned DB records
// if the transaction rolls back.
func (s *PlatformService) Delete(id string) error {
	// Check platform exists
	if _, err := s.repo.FindByID(id); err != nil {
		return fmt.Errorf("platform not found")
	}

	// Collect version directories to delete after successful commit.
	// We delete files only after the DB transaction succeeds to avoid
	// a situation where files are gone but DB records are rolled back.
	type dirToClean struct {
		subDir string
	}
	var dirsToClean []dirToClean

	// Collect custom sub data for file deletion
	type customSubInfo struct {
		userID   string
		platform string
	}
	var customSubs []customSubInfo

	tx, err := repository.DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Collect subscriptions and delete from DB
	subs, _ := s.subRepo.ListByPlatform(id)
	for _, sub := range subs {
		dirsToClean = append(dirsToClean, dirToClean{subDir: "subscriptions/" + sub.ID})
		if _, err := tx.Exec(`DELETE FROM subscriptions WHERE id = ?`, sub.ID); err != nil {
			return fmt.Errorf("failed to delete subscription: %w", err)
		}
	}

	// Delete download tokens for this platform
	if _, err := tx.Exec(`DELETE FROM download_tokens WHERE platform = ?`, id); err != nil {
		return fmt.Errorf("failed to delete download tokens: %w", err)
	}

	// Collect and delete custom subscriptions
	rows, err := tx.Query(`SELECT user_id, platform FROM custom_subscriptions WHERE platform = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to query custom subscriptions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var userID, platform string
		if err := rows.Scan(&userID, &platform); err != nil {
			return fmt.Errorf("failed to scan custom subscription row: %w", err)
		}
		customSubs = append(customSubs, customSubInfo{userID: userID, platform: platform})
		dirsToClean = append(dirsToClean, dirToClean{subDir: "custom/" + userID + "/" + platform})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating custom subscriptions: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM custom_subscriptions WHERE platform = ?`, id); err != nil {
		return fmt.Errorf("failed to delete custom subscriptions: %w", err)
	}

	// Delete platform
	if _, err := tx.Exec(`DELETE FROM platforms WHERE id = ?`, id); err != nil {
		return fmt.Errorf("failed to delete platform: %w", err)
	}

	// Commit the transaction first
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	// Only now delete files from disk — if this fails, it only leaves orphaned
	// files (not orphaned DB records), which is the safer failure mode.
	for _, d := range dirsToClean {
		s.versionSvc.RemoveVersionDir(d.subDir)
	}

	return nil
}
