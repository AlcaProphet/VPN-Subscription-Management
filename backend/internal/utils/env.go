package utils

import "os"

// GetEnv reads an environment variable with a fallback default value.
func GetEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
