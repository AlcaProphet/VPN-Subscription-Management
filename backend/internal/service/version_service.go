package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"vpn-sub/internal/models"
	"vpn-sub/internal/utils"
)

const (
	MaxVersions   = 5
	MaxUploadSize = 50 * 1024 * 1024 // 50 MB
)

// VersionService handles shared version management logic for all resource types.
type VersionService struct {
	dataDir string
}

// NewVersionService creates a new VersionService.
func NewVersionService(dataDir string) *VersionService {
	return &VersionService{dataDir: dataDir}
}

// DataDir returns the data directory path.
func (s *VersionService) DataDir() string {
	return s.dataDir
}

// nextVersion returns the next version number (max existing + 1).
func (s *VersionService) nextVersion(versions []models.Version) int {
	if len(versions) == 0 {
		return 1
	}
	maxV := 0
	for _, v := range versions {
		if v.Version > maxV {
			maxV = v.Version
		}
	}
	return maxV + 1
}

// versionFileName returns the file name for a version (e.g. "v3.conf").
func versionFileName(v int) string {
	return fmt.Sprintf("v%d.conf", v)
}

// currentFileName returns the symlink name for the current version.
func currentFileName() string {
	return "current.conf"
}

// ensureDir creates the directory for the given sub-path under dataDir.
func (s *VersionService) ensureDir(subDir string) (string, error) {
	fullDir, err := utils.SanitizePath(s.dataDir, subDir)
	if err != nil {
		return "", fmt.Errorf("invalid directory path: %w", err)
	}
	if err := os.MkdirAll(fullDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}
	return fullDir, nil
}

// CreateVersion creates a new version file with the given content,
// appends it to the versions list, updates the current symlink,
// and enforces the max versions limit.
// Returns the updated versions slice.
func (s *VersionService) CreateVersion(subDir, fileContent string, existingVersions []models.Version) ([]models.Version, error) {
	fullDir, err := s.ensureDir(subDir)
	if err != nil {
		return nil, err
	}

	newVersionNum := s.nextVersion(existingVersions)
	fileName := versionFileName(newVersionNum)
	filePath := filepath.Join(fullDir, fileName)

	// Write the content to the new version file
	if err := os.WriteFile(filePath, []byte(fileContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write version file: %w", err)
	}

	now := time.Now().UTC()
	newVer := models.Version{
		Version:   newVersionNum,
		FilePath:  filepath.Join(subDir, fileName),
		CreatedAt: now,
		UpdatedAt: now,
	}

	versions := append(existingVersions, newVer)

	// Atomically switch current symlink
	if err := s.switchSymlink(fullDir, fileName); err != nil {
		return nil, fmt.Errorf("failed to switch current symlink: %w", err)
	}

	// Enforce max versions limit
	versions, err = s.enforceMaxVersions(fullDir, versions)
	if err != nil {
		return nil, err
	}

	return versions, nil
}

// SwitchVersion switches the current symlink to the specified version.
func (s *VersionService) SwitchVersion(subDir string, versionNum int, versions []models.Version) ([]models.Version, error) {
	fullDir, err := s.ensureDir(subDir)
	if err != nil {
		return nil, err
	}

	// Verify the version exists
	found := false
	for i := range versions {
		if versions[i].Version == versionNum {
			found = true
			versions[i].UpdatedAt = time.Now().UTC()
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("version %d not found", versionNum)
	}

	fileName := versionFileName(versionNum)
	if err := s.switchSymlink(fullDir, fileName); err != nil {
		return nil, fmt.Errorf("failed to switch current symlink: %w", err)
	}

	return versions, nil
}

// DeleteVersion deletes a version file and removes it from the versions list.
// Returns error if trying to delete the last version.
func (s *VersionService) DeleteVersion(subDir string, versionNum int, versions []models.Version) ([]models.Version, error) {
	if len(versions) <= 1 {
		return nil, fmt.Errorf("cannot delete the last version")
	}

	fullDir, err := s.ensureDir(subDir)
	if err != nil {
		return nil, err
	}

	fileName := versionFileName(versionNum)
	filePath := filepath.Join(fullDir, fileName)

	// Remove the version file (ignore if not exists)
	os.Remove(filePath)

	// Check if the version being deleted is the one current.conf actually
	// points to, rather than assuming the deleted version is always current.
	currentPath := filepath.Join(fullDir, currentFileName())
	isDeletingCurrent := false
	if resolved, readlinkErr := os.Readlink(currentPath); readlinkErr == nil {
		isDeletingCurrent = (resolved == fileName)
	}

	// Remove from versions list
	newVersions := make([]models.Version, 0, len(versions)-1)
	for _, v := range versions {
		if v.Version == versionNum {
			continue
		}
		newVersions = append(newVersions, v)
	}

	// Only switch symlink if we actually deleted the current version
	if isDeletingCurrent {
		maxV := 0
		for _, v := range newVersions {
			if v.Version > maxV {
				maxV = v.Version
			}
		}
		if maxV > 0 {
			if err := s.switchSymlink(fullDir, versionFileName(maxV)); err != nil {
				return nil, fmt.Errorf("failed to switch current after delete: %w", err)
			}
		}
	}

	return newVersions, nil
}

// ReadVersionContent reads the content of a specific version file.
func (s *VersionService) ReadVersionContent(subDir string, version models.Version) (string, error) {
	fullDir, err := s.ensureDir(subDir)
	if err != nil {
		return "", err
	}
	filePath := filepath.Join(fullDir, filepath.Base(version.FilePath))
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read version file: %w", err)
	}
	return string(data), nil
}

// ReadCurrentVersion reads the content of the current version.
func (s *VersionService) ReadCurrentVersion(subDir string) (string, error) {
	fullDir, err := s.ensureDir(subDir)
	if err != nil {
		return "", err
	}

	currentPath := filepath.Join(fullDir, currentFileName())

	// Resolve symlink
	resolved, err := os.Readlink(currentPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve current symlink: %w", err)
	}

	// If the resolved path is relative, make it absolute relative to fullDir
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(fullDir, resolved)
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return "", fmt.Errorf("failed to read current version: %w", err)
	}
	return string(data), nil
}

// RemoveVersionDir removes the entire version directory (for cascade deletes).
func (s *VersionService) RemoveVersionDir(subDir string) error {
	fullDir, err := utils.SanitizePath(s.dataDir, subDir)
	if err != nil {
		return nil // directory may not exist, that's fine
	}
	return os.RemoveAll(fullDir)
}

// RemoveVersionFile removes a single version file. Used to clean up orphaned files
// when a DB transaction fails after the version file has already been written.
func (s *VersionService) RemoveVersionFile(subDir string, versionNum int) error {
	fullDir, err := utils.SanitizePath(s.dataDir, subDir)
	if err != nil {
		return err
	}
	return os.Remove(filepath.Join(fullDir, versionFileName(versionNum)))
}

// NextVersion returns the next version number for the given versions slice.
func (s *VersionService) NextVersion(versions []models.Version) int {
	return s.nextVersion(versions)
}

// switchSymlink atomically switches the current symlink using rename().
func (s *VersionService) switchSymlink(fullDir, targetName string) error {
	currentPath := filepath.Join(fullDir, currentFileName())
	tmpPath := filepath.Join(fullDir, currentFileName()+".new")

	// Remove any stale temporary symlink from a previous failed attempt
	os.Remove(tmpPath)

	// Create new symlink pointing to the target version file
	if err := os.Symlink(targetName, tmpPath); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	// Atomically replace current with the new symlink (os.Rename is atomic on Linux/macOS).
	// No need to Remove currentPath first — Rename replaces the target atomically,
	// so there is never a window where current.conf does not exist.
	if err := os.Rename(tmpPath, currentPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename symlink: %w", err)
	}

	return nil
}

// enforceMaxVersions removes the oldest versions if count exceeds MaxVersions.
func (s *VersionService) enforceMaxVersions(fullDir string, versions []models.Version) ([]models.Version, error) {
	if len(versions) <= MaxVersions {
		return versions, nil
	}

	// Sort by version number ascending
	sorted := make([]models.Version, len(versions))
	copy(sorted, versions)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Version < sorted[j].Version
	})

	// Remove the oldest (lowest version numbers)
	toRemove := sorted[:len(sorted)-MaxVersions]
	for _, v := range toRemove {
		os.Remove(filepath.Join(fullDir, versionFileName(v.Version)))
	}

	return sorted[len(sorted)-MaxVersions:], nil
}
