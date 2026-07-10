package utils

import "regexp"

var validIDRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$`)

// IsValidID checks whether the given string is a valid resource ID.
// IDs must consist of lowercase letters, digits, and hyphens,
// and must not start or end with a hyphen. Maximum length 64 characters.
// This function MUST be in the utils package, NOT in handler.
func IsValidID(id string) bool {
	if len(id) == 0 || len(id) > 64 {
		return false
	}
	return validIDRegex.MatchString(id)
}
