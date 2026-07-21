package service

import (
	"encoding/json"
	"fmt"
	"time"

	"vpn-sub/internal/models"
	"vpn-sub/internal/repository"
	"vpn-sub/internal/utils"
)

// SubscriptionService handles subscription business logic.
type SubscriptionService struct {
	repo       *repository.SubscriptionRepo
	tokenRepo  *repository.DownloadTokenRepo
	versionSvc *VersionService
}

func NewSubscriptionService(versionSvc *VersionService) *SubscriptionService {
	return &SubscriptionService{
		repo:       repository.NewSubscriptionRepo(),
		tokenRepo:  repository.NewDownloadTokenRepo(),
		versionSvc: versionSvc,
	}
}

func (s *SubscriptionService) List() ([]models.Subscription, error) {
	return s.repo.List()
}

func (s *SubscriptionService) Get(id string) (*models.Subscription, error) {
	return s.repo.FindByID(id)
}

func (s *SubscriptionService) Create(sub *models.Subscription) error {
	// Auto-generate ID if not provided
	if sub.ID == "" {
		id, err := utils.GenerateUUID()
		if err != nil {
			return fmt.Errorf("failed to generate ID: %w", err)
		}
		sub.ID = id[:12]
	}
	if !utils.IsValidID(sub.ID) {
		return fmt.Errorf("invalid subscription ID: must be [a-z0-9-]+")
	}
	if sub.Name == "" {
		return fmt.Errorf("name is required")
	}
	if sub.Platform == "" {
		return fmt.Errorf("platform is required")
	}
	if sub.Type != "default" && sub.Type != "advanced" {
		return fmt.Errorf("type must be 'default' or 'advanced'")
	}
	if existing, _ := s.repo.FindByPlatformAndType(sub.Platform, sub.Type); existing != nil {
		return fmt.Errorf("subscription with platform=%q and type=%q already exists", sub.Platform, sub.Type)
	}
	sub.Versions = []models.Version{}
	return s.repo.Create(sub)
}

func (s *SubscriptionService) Update(sub *models.Subscription) error {
	if sub.Name == "" {
		return fmt.Errorf("name is required")
	}
	// Check uniqueness of (platform, type) excluding self
	existing, _ := s.repo.FindByPlatformAndType(sub.Platform, sub.Type)
	if existing != nil && existing.ID != sub.ID {
		return fmt.Errorf("subscription with platform=%q and type=%q already exists", sub.Platform, sub.Type)
	}
	return s.repo.Update(sub)
}

func (s *SubscriptionService) Delete(id string) error {
	sub, err := s.repo.FindByID(id)
	if err != nil {
		return fmt.Errorf("subscription not found")
	}

	tx, err := repository.DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM download_tokens WHERE platform = ? AND type = ? AND custom_sub_id IS NULL`, sub.Platform, sub.Type); err != nil {
		return fmt.Errorf("failed to delete download tokens: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM subscriptions WHERE id = ?`, id); err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	s.versionSvc.RemoveVersionDir("subscriptions/" + id)
	return nil
}

func (s *SubscriptionService) UploadVersion(id, content string) (*models.Subscription, error) {
	sub, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("subscription not found")
	}

	tx, err := repository.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var versionsJSON string
	err = tx.QueryRow(`SELECT versions FROM subscriptions WHERE id = ?`, id).Scan(&versionsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to lock subscription: %w", err)
	}

	var currentVersions []models.Version
	if versionsJSON != "" && versionsJSON != "[]" {
		json.Unmarshal([]byte(versionsJSON), &currentVersions)
	}

	newVersions, err := s.versionSvc.CreateVersion("subscriptions/"+id, content, currentVersions)
	if err != nil {
		return nil, err
	}

	// The newly created version is always the last element (highest number).
	newVersionNum := newVersions[len(newVersions)-1].Version

	// Ensure the version file is cleaned up if any subsequent step fails
	// (DB update or commit). The file was written outside the transaction,
	// so we must clean it up manually on failure.
	committed := false
	defer func() {
		if !committed {
			s.versionSvc.RemoveVersionFile("subscriptions/"+id, newVersionNum)
		}
	}()

	newJSON, _ := json.Marshal(newVersions)
	_, err = tx.Exec(`UPDATE subscriptions SET versions = ? WHERE id = ?`, string(newJSON), id)
	if err != nil {
		return nil, fmt.Errorf("failed to update versions: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}
	committed = true

	sub.Versions = newVersions
	return sub, nil
}

func (s *SubscriptionService) SwitchVersion(id string, versionNum int) (*models.Subscription, error) {
	sub, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("subscription not found")
	}

	tx, err := repository.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var versionsJSON string
	err = tx.QueryRow(`SELECT versions FROM subscriptions WHERE id = ?`, id).Scan(&versionsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to lock: %w", err)
	}

	var versions []models.Version
	if versionsJSON != "" && versionsJSON != "[]" {
		json.Unmarshal([]byte(versionsJSON), &versions)
	}

	newVersions, err := s.versionSvc.SwitchVersion("subscriptions/"+id, versionNum, versions)
	if err != nil {
		return nil, err
	}

	newJSON, _ := json.Marshal(newVersions)
	_, err = tx.Exec(`UPDATE subscriptions SET versions = ? WHERE id = ?`, string(newJSON), id)
	if err != nil {
		return nil, fmt.Errorf("failed to update: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	sub.Versions = newVersions
	return sub, nil
}

func (s *SubscriptionService) DeleteVersion(id string, versionNum int) (*models.Subscription, error) {
	sub, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("subscription not found")
	}

	tx, err := repository.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var versionsJSON string
	err = tx.QueryRow(`SELECT versions FROM subscriptions WHERE id = ?`, id).Scan(&versionsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to lock: %w", err)
	}

	var versions []models.Version
	if versionsJSON != "" && versionsJSON != "[]" {
		json.Unmarshal([]byte(versionsJSON), &versions)
	}

	newVersions, err := s.versionSvc.DeleteVersion("subscriptions/"+id, versionNum, versions)
	if err != nil {
		return nil, err
	}

	newJSON, _ := json.Marshal(newVersions)
	_, err = tx.Exec(`UPDATE subscriptions SET versions = ? WHERE id = ?`, string(newJSON), id)
	if err != nil {
		return nil, fmt.Errorf("failed to update: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	sub.Versions = newVersions
	return sub, nil
}

func (s *SubscriptionService) GetVersionContent(id string, versionNum int) (string, *models.Version, error) {
	sub, err := s.repo.FindByID(id)
	if err != nil {
		return "", nil, fmt.Errorf("subscription not found")
	}
	for i := range sub.Versions {
		if sub.Versions[i].Version == versionNum {
			content, err := s.versionSvc.ReadVersionContent("subscriptions/"+id, sub.Versions[i])
			return content, &sub.Versions[i], err
		}
	}
	return "", nil, fmt.Errorf("version %d not found", versionNum)
}

func (s *SubscriptionService) GetCurrentContent(platform, subType string) (string, error) {
	sub, err := s.repo.FindByPlatformAndType(platform, subType)
	if err != nil {
		return "", fmt.Errorf("subscription not found for platform=%s type=%s", platform, subType)
	}
	if len(sub.Versions) == 0 {
		return "", fmt.Errorf("no versions configured")
	}
	return s.versionSvc.ReadCurrentVersion("subscriptions/" + sub.ID)
}

func (s *SubscriptionService) GetUpdateTime() (time.Time, error) {
	subs, err := s.repo.List()
	if err != nil {
		return time.Time{}, err
	}
	var maxTime time.Time
	for _, sub := range subs {
		for _, v := range sub.Versions {
			if v.UpdatedAt.After(maxTime) {
				maxTime = v.UpdatedAt
			}
		}
	}
	return maxTime, nil
}

// ============================================================================
// Download Token helpers (used by block 4B/4C)
// ============================================================================

// GetOrCreateToken finds an existing download token for a regular subscription
// (user+platform+type) or creates a new one. Used by UserPlatforms and
// SubDownloadToken handlers.
func (s *SubscriptionService) GetOrCreateToken(userID, platform, subType string) (string, error) {
	token, err := s.tokenRepo.FindByUserAndPlatformAndType(userID, platform, subType)
	if err == nil {
		return token, nil
	}
	newToken, err := utils.GenerateToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	t := subType
	if err := s.tokenRepo.Create(newToken, userID, platform, &t, nil); err != nil {
		return "", fmt.Errorf("failed to create token: %w", err)
	}
	return newToken, nil
}

// GetOrCreateCustomToken finds an existing download token for a custom
// subscription (user+platform+custom_sub_id) or creates a new one.
func (s *SubscriptionService) GetOrCreateCustomToken(userID, platform, customSubID string) (string, error) {
	token, err := s.tokenRepo.FindByUserAndPlatformAndCustomSub(userID, platform, customSubID)
	if err == nil {
		return token, nil
	}
	newToken, err := utils.GenerateToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	if err := s.tokenRepo.Create(newToken, userID, platform, nil, &customSubID); err != nil {
		return "", fmt.Errorf("failed to create token: %w", err)
	}
	return newToken, nil
}

// RefreshToken atomically replaces the download token for a regular subscription.
// Uses UPDATE in-place instead of DELETE+INSERT to avoid a window with no token.
func (s *SubscriptionService) RefreshToken(userID, platform, subType string) (string, error) {
	newToken, err := utils.GenerateToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	// Try to update existing token in-place (atomic)
	updated, err := s.tokenRepo.ReplaceTokenForSub(userID, platform, subType, newToken)
	if err != nil {
		return "", fmt.Errorf("failed to refresh token: %w", err)
	}
	if updated {
		return newToken, nil
	}
	// No existing token — create new one
	t := subType
	if err := s.tokenRepo.Create(newToken, userID, platform, &t, nil); err != nil {
		return "", fmt.Errorf("failed to create token: %w", err)
	}
	return newToken, nil
}

// FindToken looks up a download token and returns its associated metadata.
// Returns: userID, platform, type, customSubID, error.
func (s *SubscriptionService) FindToken(token string) (userID, platform, tokType, customSubID string, err error) {
	return s.tokenRepo.FindByToken(token)
}

// SubscriptionExists checks whether a subscription for the given platform+type
// exists (regardless of whether it has any versions).
func (s *SubscriptionService) SubscriptionExists(platform, subType string) bool {
	_, err := s.repo.FindByPlatformAndType(platform, subType)
	return err == nil
}
