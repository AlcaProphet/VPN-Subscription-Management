package service

import (
	"database/sql"
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
	return s.repo.Update(sub)
}

func (s *SubscriptionService) Delete(id string) error {
	sub, err := s.repo.FindByID(id)
	if err != nil {
		return fmt.Errorf("subscription not found")
	}
	s.tokenRepo.DeleteByPlatformAndType(sub.Platform, sub.Type)
	s.versionSvc.RemoveVersionDir("subscriptions/" + id)
	return s.repo.Delete(id)
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

	newJSON, _ := json.Marshal(newVersions)
	_, err = tx.Exec(`UPDATE subscriptions SET versions = ? WHERE id = ?`, string(newJSON), id)
	if err != nil {
		return nil, fmt.Errorf("failed to update versions: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

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

var _ = sql.LevelDefault
