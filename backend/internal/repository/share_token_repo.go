package repository

import "time"

// ShareTokenRepo provides access to the share_tokens table.
type ShareTokenRepo struct{}

func NewShareTokenRepo() *ShareTokenRepo {
	return &ShareTokenRepo{}
}

func (r *ShareTokenRepo) FindByToken(token string) (string, string, error) {
	var shareSubscriptionID, createdAt string
	err := DB.QueryRow(
		`SELECT share_subscription_id, created_at FROM share_tokens WHERE token = ?`,
		token,
	).Scan(&shareSubscriptionID, &createdAt)
	if err != nil {
		return "", "", err
	}
	return shareSubscriptionID, createdAt, nil
}

func (r *ShareTokenRepo) FindByShareSubscriptionID(shareSubID string) (string, error) {
	var token string
	err := DB.QueryRow(
		`SELECT token FROM share_tokens WHERE share_subscription_id = ?`,
		shareSubID,
	).Scan(&token)
	return token, err
}

func (r *ShareTokenRepo) Create(token, shareSubscriptionID string) error {
	_, err := DB.Exec(
		`INSERT INTO share_tokens (token, share_subscription_id, created_at) VALUES (?, ?, ?)`,
		token, shareSubscriptionID, time.Now().UTC().Format("2006-01-02 15:04:05"),
	)
	return err
}

func (r *ShareTokenRepo) Delete(token string) error {
	_, err := DB.Exec(`DELETE FROM share_tokens WHERE token = ?`, token)
	return err
}

func (r *ShareTokenRepo) DeleteByShareSubscriptionID(shareSubID string) error {
	_, err := DB.Exec(`DELETE FROM share_tokens WHERE share_subscription_id = ?`, shareSubID)
	return err
}
