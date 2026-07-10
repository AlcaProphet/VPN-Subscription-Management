package utils

import (
	"os"
	"path/filepath"
	"strings"
)

// SanitizePath ensures the given relative path does not escape baseDir through
// path traversal attacks (e.g. "../", absolute paths). It resolves the absolute
// path of baseDir+relPath and verifies the result is still within baseDir.
// Returns the sanitized absolute path, or an error if traversal is detected.
func SanitizePath(baseDir, relPath string) (string, error) {
	// Clean the relPath first
	cleanRel := filepath.Clean(relPath)

	// Reject absolute paths
	if filepath.IsAbs(cleanRel) {
		return "", os.ErrPermission
	}

	// Resolve the full path
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}

	fullPath := filepath.Join(absBase, cleanRel)
	fullPath = filepath.Clean(fullPath)

	// Ensure the resolved path is within baseDir
	rel, err := filepath.Rel(absBase, fullPath)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", os.ErrPermission
	}

	return fullPath, nil
}
