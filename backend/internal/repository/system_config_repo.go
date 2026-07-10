package repository

// SystemConfigRepo provides access to the system_config table.
type SystemConfigRepo struct{}

func NewSystemConfigRepo() *SystemConfigRepo {
	return &SystemConfigRepo{}
}

// Get retrieves a config value by key.
func (r *SystemConfigRepo) Get(key string) (string, error) {
	var value string
	err := DB.QueryRow(`SELECT value FROM system_config WHERE key = ?`, key).Scan(&value)
	return value, err
}

// Set inserts or updates a config key-value pair.
func (r *SystemConfigRepo) Set(key, value string) error {
	_, err := DB.Exec(
		`INSERT INTO system_config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?`,
		key, value, value,
	)
	return err
}

// Exists checks if a config key exists.
func (r *SystemConfigRepo) Exists(key string) (bool, error) {
	var count int
	err := DB.QueryRow(`SELECT COUNT(*) FROM system_config WHERE key = ?`, key).Scan(&count)
	return count > 0, err
}
