package service

import (
	"fmt"

	"vpn-sub/internal/models"
	"vpn-sub/internal/repository"
)

// UserService handles user business logic.
type UserService struct {
	repo       *repository.UserRepo
	tokenRepo  *repository.DownloadTokenRepo
	customRepo *repository.CustomSubscriptionRepo
	versionSvc *VersionService
}

// NewUserService creates a new UserService.
func NewUserService(versionSvc *VersionService) *UserService {
	return &UserService{
		repo:       repository.NewUserRepo(),
		tokenRepo:  repository.NewDownloadTokenRepo(),
		customRepo: repository.NewCustomSubscriptionRepo(),
		versionSvc: versionSvc,
	}
}

// List returns all users.
func (s *UserService) List() ([]models.User, error) {
	return s.repo.List()
}

// Get returns a user by ID.
func (s *UserService) Get(userID string) (*models.User, error) {
	return s.repo.FindByID(userID)
}

// Update updates a user's editable fields.
// Admin self-protection: admin's is_advanced is always true.
func (s *UserService) Update(operatorID string, target *models.User) error {
	existing, err := s.repo.FindByID(target.UserID)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	// Role must not be changed via this endpoint (handler sets it to empty string).
	// Preserve the existing role from the database.
	// Admin self-protection for role changes is enforced at the handler level
	// (Role is hardcoded to "" in UpdateUser handler, preventing any role mutation).
	target.Role = existing.Role

	// Admin's is_advanced is always true
	if existing.Role == "admin" {
		target.IsAdvanced = true
	}

	// If is_advanced changed, delete all old download tokens
	if target.IsAdvanced != existing.IsAdvanced {
		if err := s.tokenRepo.DeleteByUser(target.UserID); err != nil {
			return fmt.Errorf("failed to revoke tokens: %w", err)
		}
	}

	target.UserID = existing.UserID // ensure ID doesn't change
	return s.repo.Update(target)
}

// Delete deletes a user and cascades: download tokens, custom subscriptions, version files.
// Admin self-protection: cannot delete self; cannot delete last admin.
func (s *UserService) Delete(operatorID, targetID string) error {
	// Cannot delete self
	if operatorID == targetID {
		return fmt.Errorf("cannot delete yourself")
	}

	target, err := s.repo.FindByID(targetID)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	// Cannot delete the last admin
	if target.Role == "admin" {
		count, err := s.repo.CountByRole("admin")
		if err != nil {
			return fmt.Errorf("failed to check admin count: %w", err)
		}
		if count <= 1 {
			return fmt.Errorf("cannot delete the last administrator")
		}
	}

	// Collect custom sub directories for file cleanup after commit
	type dirToClean struct {
		path string
	}
	var dirsToClean []dirToClean
	customs, _ := s.customRepo.ListByUser(targetID)
	for _, cs := range customs {
		dirsToClean = append(dirsToClean, dirToClean{path: "custom/" + cs.UserID + "/" + cs.Platform})
	}

	tx, err := repository.DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete custom subscriptions from DB
	if _, err := tx.Exec(`DELETE FROM custom_subscriptions WHERE user_id = ?`, targetID); err != nil {
		return fmt.Errorf("failed to delete custom subscriptions: %w", err)
	}

	// Delete download tokens
	if _, err := tx.Exec(`DELETE FROM download_tokens WHERE user_id = ?`, targetID); err != nil {
		return fmt.Errorf("failed to delete download tokens: %w", err)
	}

	// Delete user
	if _, err := tx.Exec(`DELETE FROM users WHERE user_id = ?`, targetID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	// Delete files only after successful commit
	for _, d := range dirsToClean {
		s.versionSvc.RemoveVersionDir(d.path)
	}

	return nil
}

// RevokeTokens revokes all download tokens for a user.
func (s *UserService) RevokeTokens(userID string) error {
	return s.tokenRepo.DeleteByUser(userID)
}

// CountAdmins returns the number of admin users.
func (s *UserService) CountAdmins() (int, error) {
	return s.repo.CountByRole("admin")
}
