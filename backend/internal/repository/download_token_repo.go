package repository

import (
	"database/sql"
	"time"
)

// DownloadTokenRepo provides access to the download_tokens table.
type DownloadTokenRepo struct{}

func NewDownloadTokenRepo() *DownloadTokenRepo {
	return &DownloadTokenRepo{}
}

// FindByToken retrieves a download token record.
func (r *DownloadTokenRepo) FindByToken(token string) (userID, platform, tokenType, customSubID string, err error) {
	var t, cs sql.NullString
	err = DB.QueryRow(
		`SELECT user_id, platform, type, custom_sub_id FROM download_tokens WHERE token = ?`,
		token,
	).Scan(&userID, &platform, &t, &cs)
	if err != nil {
		return "", "", "", "", err
	}
	if t.Valid {
		tokenType = t.String
	}
	if cs.Valid {
		customSubID = cs.String
	}
	return
}

// FindByUserAndPlatformAndType finds a token for a regular subscription (no custom_sub).
func (r *DownloadTokenRepo) FindByUserAndPlatformAndType(userID, platform, subType string) (string, error) {
	var token string
	err := DB.QueryRow(
		`SELECT token FROM download_tokens WHERE user_id = ? AND platform = ? AND type = ? AND custom_sub_id IS NULL`,
		userID, platform, subType,
	).Scan(&token)
	return token, err
}

// FindByUserAndPlatformAndCustomSub finds a token for a custom subscription.
func (r *DownloadTokenRepo) FindByUserAndPlatformAndCustomSub(userID, platform, customSubID string) (string, error) {
	var token string
	err := DB.QueryRow(
		`SELECT token FROM download_tokens WHERE user_id = ? AND platform = ? AND custom_sub_id = ?`,
		userID, platform, customSubID,
	).Scan(&token)
	return token, err
}

// Create inserts a new download token.
func (r *DownloadTokenRepo) Create(token, userID, platform string, subType *string, customSubID *string) error {
	_, err := DB.Exec(
		`INSERT INTO download_tokens (token, user_id, platform, type, custom_sub_id, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		token, userID, platform, subType, customSubID, time.Now().UTC().Format("2006-01-02 15:04:05"),
	)
	return err
}

// Delete removes a download token.
func (r *DownloadTokenRepo) Delete(token string) error {
	_, err := DB.Exec(`DELETE FROM download_tokens WHERE token = ?`, token)
	return err
}

// DeleteByUser removes all download tokens for a user.
func (r *DownloadTokenRepo) DeleteByUser(userID string) error {
	_, err := DB.Exec(`DELETE FROM download_tokens WHERE user_id = ?`, userID)
	return err
}

// DeleteByCustomSubID removes all tokens associated with a custom subscription.
func (r *DownloadTokenRepo) DeleteByCustomSubID(customSubID string) error {
	_, err := DB.Exec(`DELETE FROM download_tokens WHERE custom_sub_id = ?`, customSubID)
	return err
}

// FindTokenByCustomSubID finds any token for a custom subscription.
func (r *DownloadTokenRepo) FindTokenByCustomSubID(customSubID string) (string, error) {
	var token string
	err := DB.QueryRow(`SELECT token FROM download_tokens WHERE custom_sub_id = ? LIMIT 1`, customSubID).Scan(&token)
	return token, err
}

// ReplaceTokenValue atomically replaces an existing token value with a new one.
func (r *DownloadTokenRepo) ReplaceTokenValue(oldToken, newToken string) error {
	_, err := DB.Exec(`UPDATE download_tokens SET token = ? WHERE token = ?`, newToken, oldToken)
	return err
}

// ReplaceTokenForSub atomically replaces a regular subscription token in-place
// by its unique key (user_id + platform + type, custom_sub_id IS NULL).
// Returns true if an existing row was updated, false if none found.
func (r *DownloadTokenRepo) ReplaceTokenForSub(userID, platform, subType, newToken string) (bool, error) {
	result, err := DB.Exec(
		`UPDATE download_tokens SET token = ? WHERE user_id = ? AND platform = ? AND type = ? AND custom_sub_id IS NULL`,
		newToken, userID, platform, subType,
	)
	if err != nil {
		return false, err
	}
	n, _ := result.RowsAffected()
	return n > 0, nil
}

// DeleteByPlatformAndType removes tokens for a specific subscription (no custom_sub).
func (r *DownloadTokenRepo) DeleteByPlatformAndType(platform, subType string) error {
	_, err := DB.Exec(
		`DELETE FROM download_tokens WHERE platform = ? AND type = ? AND custom_sub_id IS NULL`,
		platform, subType,
	)
	return err
}

// DeleteByPlatform removes all tokens for a platform.
func (r *DownloadTokenRepo) DeleteByPlatform(platform string) error {
	_, err := DB.Exec(`DELETE FROM download_tokens WHERE platform = ?`, platform)
	return err
}
