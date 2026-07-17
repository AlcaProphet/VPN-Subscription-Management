package service

import (
	"encoding/json"
	"fmt"
	"time"

	"vpn-sub/internal/models"
	"vpn-sub/internal/repository"
	"vpn-sub/internal/utils"
)

// ShareSubscriptionService handles share subscription business logic.
type ShareSubscriptionService struct {
	repo       *repository.ShareSubscriptionRepo
	tokenRepo  *repository.ShareTokenRepo
	versionSvc *VersionService
}

func NewShareSubscriptionService(versionSvc *VersionService) *ShareSubscriptionService {
	return &ShareSubscriptionService{
		repo:       repository.NewShareSubscriptionRepo(),
		tokenRepo:  repository.NewShareTokenRepo(),
		versionSvc: versionSvc,
	}
}

func (s *ShareSubscriptionService) List() ([]models.ShareSubscription, error) {
	return s.repo.List()
}

func (s *ShareSubscriptionService) Get(id string) (*models.ShareSubscription, error) {
	return s.repo.FindByID(id)
}

// Create creates a new share subscription with an initial version and auto-generated token.
func (s *ShareSubscriptionService) Create(name, content string) (*models.ShareSubscription, string, error) {
	if name == "" {
		return nil, "", fmt.Errorf("name is required")
	}

	id, err := utils.GenerateUUID()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate ID: %w", err)
	}
	id = id[:12]

	ss := &models.ShareSubscription{
		ID:        id,
		Name:      name,
		Versions:  []models.Version{},
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.Create(ss); err != nil {
		return nil, "", fmt.Errorf("failed to create share subscription: %w", err)
	}

	// Upload first version
	updatedVersions, err := s.versionSvc.CreateVersion("shares/"+id, content, []models.Version{})
	if err != nil {
		s.repo.Delete(id)
		return nil, "", fmt.Errorf("failed to create first version: %w", err)
	}

	if err := s.repo.UpdateVersions(id, updatedVersions); err != nil {
		s.repo.Delete(id)
		s.versionSvc.RemoveVersionDir("shares/" + id)
		return nil, "", fmt.Errorf("failed to save versions: %w", err)
	}

	// Generate share token
	token, err := utils.GenerateToken()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}
	if err := s.tokenRepo.Create(token, id); err != nil {
		// Clean up DB record + version files on failure
		s.repo.Delete(id)
		s.versionSvc.RemoveVersionDir("shares/" + id)
		return nil, "", fmt.Errorf("failed to create token: %w", err)
	}

	ss.Versions = updatedVersions
	return ss, token, nil
}

func (s *ShareSubscriptionService) Update(ss *models.ShareSubscription) error {
	if ss.Name == "" {
		return fmt.Errorf("name is required")
	}
	return s.repo.Update(ss)
}

// Delete deletes a share subscription and cascades token + version files.
func (s *ShareSubscriptionService) Delete(id string) error {
	if _, err := s.repo.FindByID(id); err != nil {
		return fmt.Errorf("share subscription not found")
	}

	tx, err := repository.DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM share_tokens WHERE share_subscription_id = ?`, id); err != nil {
		return fmt.Errorf("failed to delete share tokens: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM share_subscriptions WHERE id = ?`, id); err != nil {
		return fmt.Errorf("failed to delete share subscription: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	s.versionSvc.RemoveVersionDir("shares/" + id)
	return nil
}

func (s *ShareSubscriptionService) UploadVersion(id, content string) (*models.ShareSubscription, error) {
	ss, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("share subscription not found")
	}

	tx, err := repository.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var versionsJSON string
	err = tx.QueryRow(`SELECT versions FROM share_subscriptions WHERE id = ?`, id).Scan(&versionsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to lock: %w", err)
	}

	var currentVersions []models.Version
	if versionsJSON != "" && versionsJSON != "[]" {
		json.Unmarshal([]byte(versionsJSON), &currentVersions)
	}

	newVersions, err := s.versionSvc.CreateVersion("shares/"+id, content, currentVersions)
	if err != nil {
		return nil, err
	}

	// The newly created version is always the last element (highest number).
	newVersionNum := newVersions[len(newVersions)-1].Version

	// Ensure the version file is cleaned up if any subsequent step fails
	committed := false
	defer func() {
		if !committed {
			s.versionSvc.RemoveVersionFile("shares/"+id, newVersionNum)
		}
	}()

	newJSON, _ := json.Marshal(newVersions)
	_, err = tx.Exec(`UPDATE share_subscriptions SET versions = ? WHERE id = ?`, string(newJSON), id)
	if err != nil {
		return nil, fmt.Errorf("failed to update: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}
	committed = true

	ss.Versions = newVersions
	return ss, nil
}

func (s *ShareSubscriptionService) SwitchVersion(id string, versionNum int) (*models.ShareSubscription, error) {
	ss, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("share subscription not found")
	}

	tx, err := repository.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var versionsJSON string
	err = tx.QueryRow(`SELECT versions FROM share_subscriptions WHERE id = ?`, id).Scan(&versionsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to lock: %w", err)
	}

	var versions []models.Version
	if versionsJSON != "" && versionsJSON != "[]" {
		json.Unmarshal([]byte(versionsJSON), &versions)
	}

	newVersions, err := s.versionSvc.SwitchVersion("shares/"+id, versionNum, versions)
	if err != nil {
		return nil, err
	}

	newJSON, _ := json.Marshal(newVersions)
	_, err = tx.Exec(`UPDATE share_subscriptions SET versions = ? WHERE id = ?`, string(newJSON), id)
	if err != nil {
		return nil, fmt.Errorf("failed to update: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	ss.Versions = newVersions
	return ss, nil
}

func (s *ShareSubscriptionService) DeleteVersion(id string, versionNum int) (*models.ShareSubscription, error) {
	ss, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("share subscription not found")
	}

	tx, err := repository.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var versionsJSON string
	err = tx.QueryRow(`SELECT versions FROM share_subscriptions WHERE id = ?`, id).Scan(&versionsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to lock: %w", err)
	}

	var versions []models.Version
	if versionsJSON != "" && versionsJSON != "[]" {
		json.Unmarshal([]byte(versionsJSON), &versions)
	}

	newVersions, err := s.versionSvc.DeleteVersion("shares/"+id, versionNum, versions)
	if err != nil {
		return nil, err
	}

	newJSON, _ := json.Marshal(newVersions)
	_, err = tx.Exec(`UPDATE share_subscriptions SET versions = ? WHERE id = ?`, string(newJSON), id)
	if err != nil {
		return nil, fmt.Errorf("failed to update: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	ss.Versions = newVersions
	return ss, nil
}

func (s *ShareSubscriptionService) GetVersionContent(id string, versionNum int) (string, *models.Version, error) {
	ss, err := s.repo.FindByID(id)
	if err != nil {
		return "", nil, fmt.Errorf("share subscription not found")
	}
	for i := range ss.Versions {
		if ss.Versions[i].Version == versionNum {
			content, err := s.versionSvc.ReadVersionContent("shares/"+id, ss.Versions[i])
			return content, &ss.Versions[i], err
		}
	}
	return "", nil, fmt.Errorf("version %d not found", versionNum)
}

func (s *ShareSubscriptionService) GetCurrentContent(id string) (string, error) {
	ss, err := s.repo.FindByID(id)
	if err != nil {
		return "", fmt.Errorf("share subscription not found")
	}
	if len(ss.Versions) == 0 {
		return "", fmt.Errorf("no versions configured")
	}
	return s.versionSvc.ReadCurrentVersion("shares/" + id)
}

// RefreshToken generates a new share token, invalidating the old one.
func (s *ShareSubscriptionService) RefreshToken(shareID string) (string, error) {
	if _, err := s.repo.FindByID(shareID); err != nil {
		return "", fmt.Errorf("share subscription not found")
	}
	s.tokenRepo.DeleteByShareSubscriptionID(shareID)
	token, err := utils.GenerateToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	if err := s.tokenRepo.Create(token, shareID); err != nil {
		return "", fmt.Errorf("failed to create token: %w", err)
	}
	return token, nil
}

// RevokeToken deletes the token for a share subscription (link becomes unusable).
func (s *ShareSubscriptionService) RevokeToken(shareID string) error {
	return s.tokenRepo.DeleteByShareSubscriptionID(shareID)
}

// GetToken returns the current token for a share subscription.
func (s *ShareSubscriptionService) GetToken(shareID string) (string, error) {
	return s.tokenRepo.FindByShareSubscriptionID(shareID)
}

// ValidateToken validates a share download token and returns the share subscription ID.
func (s *ShareSubscriptionService) ValidateToken(token string) (string, error) {
	shareID, _, err := s.tokenRepo.FindByToken(token)
	return shareID, err
}
