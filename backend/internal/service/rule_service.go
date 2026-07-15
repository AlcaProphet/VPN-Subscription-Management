package service

import (
	"encoding/json"
	"fmt"
	"time"

	"vpn-sub/internal/models"
	"vpn-sub/internal/repository"
	"vpn-sub/internal/utils"
)

// RuleService handles rule business logic.
type RuleService struct {
	repo       *repository.RuleRepo
	tokenRepo  *repository.RuleTokenRepo
	versionSvc *VersionService
}

func NewRuleService(versionSvc *VersionService) *RuleService {
	return &RuleService{
		repo:       repository.NewRuleRepo(),
		tokenRepo:  repository.NewRuleTokenRepo(),
		versionSvc: versionSvc,
	}
}

func (s *RuleService) List() ([]models.Rule, error) {
	return s.repo.List()
}

func (s *RuleService) Get(id string) (*models.Rule, error) {
	return s.repo.FindByID(id)
}

func (s *RuleService) Create(rule *models.Rule) error {
	if !utils.IsValidID(rule.ID) {
		return fmt.Errorf("invalid rule ID: must be [a-z0-9-]+")
	}
	if rule.Name == "" {
		return fmt.Errorf("name is required")
	}
	if rule.ClientType == "" {
		rule.ClientType = "shadowrocket"
	}
	rule.Versions = []models.Version{}
	rule.CreatedAt = time.Now().UTC()
	return s.repo.Create(rule)
}

func (s *RuleService) Update(rule *models.Rule) error {
	if rule.Name == "" {
		return fmt.Errorf("name is required")
	}
	return s.repo.Update(rule)
}

func (s *RuleService) Delete(id string) error {
	if _, err := s.repo.FindByID(id); err != nil {
		return fmt.Errorf("rule not found")
	}
	s.tokenRepo.DeleteByRuleID(id)
	s.versionSvc.RemoveVersionDir("rules/" + id)
	return s.repo.Delete(id)
}

func (s *RuleService) UploadVersion(id, content string) (*models.Rule, error) {
	rule, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("rule not found")
	}

	tx, err := repository.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var versionsJSON string
	err = tx.QueryRow(`SELECT versions FROM rules WHERE id = ?`, id).Scan(&versionsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to lock rule: %w", err)
	}

	var currentVersions []models.Version
	if versionsJSON != "" && versionsJSON != "[]" {
		json.Unmarshal([]byte(versionsJSON), &currentVersions)
	}

	newVersionNum := s.versionSvc.NextVersion(currentVersions)
	newVersions, err := s.versionSvc.CreateVersion("rules/"+id, content, currentVersions)
	if err != nil {
		return nil, err
	}

	// Ensure the version file is cleaned up if any subsequent step fails
	committed := false
	defer func() {
		if !committed {
			s.versionSvc.RemoveVersionFile("rules/"+id, newVersionNum)
		}
	}()

	newJSON, _ := json.Marshal(newVersions)
	_, err = tx.Exec(`UPDATE rules SET versions = ? WHERE id = ?`, string(newJSON), id)
	if err != nil {
		return nil, fmt.Errorf("failed to update versions: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}
	committed = true

	rule.Versions = newVersions
	return rule, nil
}

func (s *RuleService) SwitchVersion(id string, versionNum int) (*models.Rule, error) {
	rule, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("rule not found")
	}

	tx, err := repository.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var versionsJSON string
	err = tx.QueryRow(`SELECT versions FROM rules WHERE id = ?`, id).Scan(&versionsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to lock: %w", err)
	}

	var versions []models.Version
	if versionsJSON != "" && versionsJSON != "[]" {
		json.Unmarshal([]byte(versionsJSON), &versions)
	}

	newVersions, err := s.versionSvc.SwitchVersion("rules/"+id, versionNum, versions)
	if err != nil {
		return nil, err
	}

	newJSON, _ := json.Marshal(newVersions)
	_, err = tx.Exec(`UPDATE rules SET versions = ? WHERE id = ?`, string(newJSON), id)
	if err != nil {
		return nil, fmt.Errorf("failed to update: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	rule.Versions = newVersions
	return rule, nil
}

func (s *RuleService) DeleteVersion(id string, versionNum int) (*models.Rule, error) {
	rule, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("rule not found")
	}

	tx, err := repository.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var versionsJSON string
	err = tx.QueryRow(`SELECT versions FROM rules WHERE id = ?`, id).Scan(&versionsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to lock: %w", err)
	}

	var versions []models.Version
	if versionsJSON != "" && versionsJSON != "[]" {
		json.Unmarshal([]byte(versionsJSON), &versions)
	}

	newVersions, err := s.versionSvc.DeleteVersion("rules/"+id, versionNum, versions)
	if err != nil {
		return nil, err
	}

	newJSON, _ := json.Marshal(newVersions)
	_, err = tx.Exec(`UPDATE rules SET versions = ? WHERE id = ?`, string(newJSON), id)
	if err != nil {
		return nil, fmt.Errorf("failed to update: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	rule.Versions = newVersions
	return rule, nil
}

func (s *RuleService) GetVersionContent(id string, versionNum int) (string, *models.Version, error) {
	rule, err := s.repo.FindByID(id)
	if err != nil {
		return "", nil, fmt.Errorf("rule not found")
	}
	for i := range rule.Versions {
		if rule.Versions[i].Version == versionNum {
			content, err := s.versionSvc.ReadVersionContent("rules/"+id, rule.Versions[i])
			return content, &rule.Versions[i], err
		}
	}
	return "", nil, fmt.Errorf("version %d not found", versionNum)
}

// GetCurrentContent returns the current version content for a rule.
func (s *RuleService) GetCurrentContent(id string) (string, error) {
	rule, err := s.repo.FindByID(id)
	if err != nil {
		return "", fmt.Errorf("rule not found")
	}
	if len(rule.Versions) == 0 {
		return "", fmt.Errorf("no versions configured")
	}
	return s.versionSvc.ReadCurrentVersion("rules/" + id)
}

// RefreshToken generates a new rule token, invalidating the old one.
func (s *RuleService) RefreshToken(ruleID string) (string, error) {
	if _, err := s.repo.FindByID(ruleID); err != nil {
		return "", fmt.Errorf("rule not found")
	}
	// Delete old tokens
	s.tokenRepo.DeleteByRuleID(ruleID)
	// Generate new token
	token, err := utils.GenerateToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	if err := s.tokenRepo.Create(token, ruleID); err != nil {
		return "", fmt.Errorf("failed to create token: %w", err)
	}
	return token, nil
}

// GetToken returns the current token for a rule.
func (s *RuleService) GetToken(ruleID string) (string, error) {
	return s.tokenRepo.FindByRuleID(ruleID)
}

// ValidateToken validates a rule download token.
func (s *RuleService) ValidateToken(token string) (string, error) {
	return s.tokenRepo.FindByToken(token)
}
