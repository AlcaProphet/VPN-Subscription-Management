package service

import (
	"encoding/json"
	"fmt"
	"time"

	"vpn-sub/internal/models"
	"vpn-sub/internal/repository"
	"vpn-sub/internal/utils"
)

// CustomSubscriptionService handles custom subscription business logic.
type CustomSubscriptionService struct {
	repo       *repository.CustomSubscriptionRepo
	tokenRepo  *repository.DownloadTokenRepo
	versionSvc *VersionService
}

func NewCustomSubscriptionService(versionSvc *VersionService) *CustomSubscriptionService {
	return &CustomSubscriptionService{
		repo:       repository.NewCustomSubscriptionRepo(),
		tokenRepo:  repository.NewDownloadTokenRepo(),
		versionSvc: versionSvc,
	}
}

// Upload uploads a custom subscription for a user+platform (creates or overwrites).
func (s *CustomSubscriptionService) Upload(userID, platform, content string) (*models.CustomSubscription, error) {
	// Check if custom sub already exists for this user+platform
	existing, _ := s.repo.FindByUserAndPlatform(userID, platform)

	if existing != nil {
		// Overwrite: upload new version (do NOT delete tokens — the user's
		// existing download link should continue working with the new version)
		cs, err := s.UploadVersion(existing.ID, content)
		if err != nil {
			return nil, err
		}
		return cs, nil
	}

	// Create new custom subscription
	id, err := utils.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate ID: %w", err)
	}
	// Use a shorter ID format for custom subs
	id = id[:12]

	cs := &models.CustomSubscription{
		ID:        id,
		UserID:    userID,
		Platform:  platform,
		Versions:  []models.Version{},
		CreatedAt: time.Now().UTC().Format("2006-01-02 15:04:05"),
	}

	if err := s.repo.Create(cs); err != nil {
		return nil, fmt.Errorf("failed to create custom subscription: %w", err)
	}

	// Upload the first version; cleanup DB record on failure
	result, err := s.UploadVersion(id, content)
	if err != nil {
		s.repo.Delete(id)
		return nil, err
	}
	return result, nil
}

// UploadVersion uploads a new version to an existing custom subscription.
func (s *CustomSubscriptionService) UploadVersion(id, content string) (*models.CustomSubscription, error) {
	cs, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("custom subscription not found")
	}

	subDir := "custom/" + cs.UserID + "/" + cs.Platform

	tx, err := repository.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var versionsJSON string
	err = tx.QueryRow(`SELECT versions FROM custom_subscriptions WHERE id = ?`, id).Scan(&versionsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to lock: %w", err)
	}

	var currentVersions []models.Version
	if versionsJSON != "" && versionsJSON != "[]" {
		if err := json.Unmarshal([]byte(versionsJSON), &currentVersions); err != nil {
			return nil, fmt.Errorf("failed to parse versions JSON: %w", err)
		}
	}

	newVersions, err := s.versionSvc.CreateVersion(subDir, content, currentVersions)
	if err != nil {
		return nil, err
	}

	// The newly created version is always the last element (highest number).
	newVersionNum := newVersions[len(newVersions)-1].Version

	// Ensure the version file is cleaned up if any subsequent step fails
	committed := false
	defer func() {
		if !committed {
			s.versionSvc.RemoveVersionFile(subDir, newVersionNum)
		}
	}()

	newJSON, _ := json.Marshal(newVersions)
	_, err = tx.Exec(`UPDATE custom_subscriptions SET versions = ? WHERE id = ?`, string(newJSON), id)
	if err != nil {
		return nil, fmt.Errorf("failed to update: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}
	committed = true

	cs.Versions = newVersions
	return cs, nil
}

// SwitchVersion switches the current version.
func (s *CustomSubscriptionService) SwitchVersion(id string, versionNum int) (*models.CustomSubscription, error) {
	cs, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("custom subscription not found")
	}

	subDir := "custom/" + cs.UserID + "/" + cs.Platform

	tx, err := repository.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var versionsJSON string
	err = tx.QueryRow(`SELECT versions FROM custom_subscriptions WHERE id = ?`, id).Scan(&versionsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to lock: %w", err)
	}

	var versions []models.Version
	if versionsJSON != "" && versionsJSON != "[]" {
		if err := json.Unmarshal([]byte(versionsJSON), &versions); err != nil {
			return nil, fmt.Errorf("failed to parse versions JSON: %w", err)
		}
	}

	newVersions, err := s.versionSvc.SwitchVersion(subDir, versionNum, versions)
	if err != nil {
		return nil, err
	}

	newJSON, _ := json.Marshal(newVersions)
	_, err = tx.Exec(`UPDATE custom_subscriptions SET versions = ? WHERE id = ?`, string(newJSON), id)
	if err != nil {
		return nil, fmt.Errorf("failed to update: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	cs.Versions = newVersions
	return cs, nil
}

// DeleteVersion deletes a version.
func (s *CustomSubscriptionService) DeleteVersion(id string, versionNum int) (*models.CustomSubscription, error) {
	cs, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("custom subscription not found")
	}

	subDir := "custom/" + cs.UserID + "/" + cs.Platform

	tx, err := repository.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var versionsJSON string
	err = tx.QueryRow(`SELECT versions FROM custom_subscriptions WHERE id = ?`, id).Scan(&versionsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to lock: %w", err)
	}

	var versions []models.Version
	if versionsJSON != "" && versionsJSON != "[]" {
		if err := json.Unmarshal([]byte(versionsJSON), &versions); err != nil {
			return nil, fmt.Errorf("failed to parse versions JSON: %w", err)
		}
	}

	newVersions, err := s.versionSvc.DeleteVersion(subDir, versionNum, versions)
	if err != nil {
		return nil, err
	}

	newJSON, _ := json.Marshal(newVersions)
	_, err = tx.Exec(`UPDATE custom_subscriptions SET versions = ? WHERE id = ?`, string(newJSON), id)
	if err != nil {
		return nil, fmt.Errorf("failed to update: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	cs.Versions = newVersions
	return cs, nil
}

// GetVersionContent returns a specific version's content.
func (s *CustomSubscriptionService) GetVersionContent(id string, versionNum int) (string, *models.Version, error) {
	cs, err := s.repo.FindByID(id)
	if err != nil {
		return "", nil, fmt.Errorf("custom subscription not found")
	}
	for i := range cs.Versions {
		if cs.Versions[i].Version == versionNum {
			content, err := s.versionSvc.ReadVersionContent("custom/"+cs.UserID+"/"+cs.Platform, cs.Versions[i])
			return content, &cs.Versions[i], err
		}
	}
	return "", nil, fmt.Errorf("version %d not found", versionNum)
}

// GetCurrentContent returns the current version content.
func (s *CustomSubscriptionService) GetCurrentContent(id string) (string, error) {
	cs, err := s.repo.FindByID(id)
	if err != nil {
		return "", fmt.Errorf("custom subscription not found")
	}
	if len(cs.Versions) == 0 {
		return "", fmt.Errorf("no versions configured")
	}
	return s.versionSvc.ReadCurrentVersion("custom/" + cs.UserID + "/" + cs.Platform)
}

// Delete deletes a custom subscription and cascades download tokens + version files.
func (s *CustomSubscriptionService) Delete(id string) error {
	cs, err := s.repo.FindByID(id)
	if err != nil {
		return fmt.Errorf("custom subscription not found")
	}

	subDir := "custom/" + cs.UserID + "/" + cs.Platform

	tx, err := repository.DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM download_tokens WHERE custom_sub_id = ?`, id); err != nil {
		return fmt.Errorf("failed to delete download tokens: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM custom_subscriptions WHERE id = ?`, id); err != nil {
		return fmt.Errorf("failed to delete custom subscription: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	s.versionSvc.RemoveVersionDir(subDir)
	return nil
}

// GetByUserAndPlatform returns the custom subscription for a user+platform.
func (s *CustomSubscriptionService) GetByUserAndPlatform(userID, platform string) (*models.CustomSubscription, error) {
	return s.repo.FindByUserAndPlatform(userID, platform)
}

// ListByUser returns all custom subscriptions for a user.
func (s *CustomSubscriptionService) ListByUser(userID string) ([]models.CustomSubscription, error) {
	return s.repo.ListByUser(userID)
}

// RefreshToken atomically replaces the download token for a custom subscription.
// Uses UPDATE in-place instead of DELETE+INSERT to avoid a window with no token.
func (s *CustomSubscriptionService) RefreshToken(customSubID string) error {
	oldToken, err := s.tokenRepo.FindTokenByCustomSubID(customSubID)
	if err != nil {
		// No existing token — nothing to refresh, caller will create via GetOrCreateCustomToken
		return nil
	}
	newToken, err := utils.GenerateToken()
	if err != nil {
		return err
	}
	return s.tokenRepo.ReplaceTokenValue(oldToken, newToken)
}
