package repository

import (
	"encoding/json"
	"vpn-sub/internal/models"
)

// CustomSubscriptionRepo provides access to the custom_subscriptions table.
type CustomSubscriptionRepo struct{}

func NewCustomSubscriptionRepo() *CustomSubscriptionRepo {
	return &CustomSubscriptionRepo{}
}

func (r *CustomSubscriptionRepo) FindByID(id string) (*models.CustomSubscription, error) {
	cs := &models.CustomSubscription{}
	var versionsJSON string
	err := DB.QueryRow(
		`SELECT id, user_id, platform, versions, created_at FROM custom_subscriptions WHERE id = ?`,
		id,
	).Scan(&cs.ID, &cs.UserID, &cs.Platform, &versionsJSON, &cs.CreatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(versionsJSON), &cs.Versions)
	return cs, nil
}

func (r *CustomSubscriptionRepo) FindByUserAndPlatform(userID, platform string) (*models.CustomSubscription, error) {
	cs := &models.CustomSubscription{}
	var versionsJSON string
	err := DB.QueryRow(
		`SELECT id, user_id, platform, versions, created_at FROM custom_subscriptions WHERE user_id = ? AND platform = ?`,
		userID, platform,
	).Scan(&cs.ID, &cs.UserID, &cs.Platform, &versionsJSON, &cs.CreatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(versionsJSON), &cs.Versions)
	return cs, nil
}

func (r *CustomSubscriptionRepo) ListByUser(userID string) ([]models.CustomSubscription, error) {
	rows, err := DB.Query(
		`SELECT id, user_id, platform, versions, created_at FROM custom_subscriptions WHERE user_id = ? ORDER BY platform`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []models.CustomSubscription
	for rows.Next() {
		var cs models.CustomSubscription
		var versionsJSON string
		if err := rows.Scan(&cs.ID, &cs.UserID, &cs.Platform, &versionsJSON, &cs.CreatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(versionsJSON), &cs.Versions)
		subs = append(subs, cs)
	}
	return subs, rows.Err()
}

func (r *CustomSubscriptionRepo) Create(cs *models.CustomSubscription) error {
	versionsJSON, _ := json.Marshal(cs.Versions)
	_, err := DB.Exec(
		`INSERT INTO custom_subscriptions (id, user_id, platform, versions, created_at) VALUES (?, ?, ?, ?, datetime('now'))`,
		cs.ID, cs.UserID, cs.Platform, string(versionsJSON),
	)
	return err
}

func (r *CustomSubscriptionRepo) UpdateVersions(id string, versions []models.Version) error {
	versionsJSON, _ := json.Marshal(versions)
	_, err := DB.Exec(`UPDATE custom_subscriptions SET versions = ? WHERE id = ?`, string(versionsJSON), id)
	return err
}

func (r *CustomSubscriptionRepo) Delete(id string) error {
	_, err := DB.Exec(`DELETE FROM custom_subscriptions WHERE id = ?`, id)
	return err
}

func (r *CustomSubscriptionRepo) DeleteByUser(userID string) error {
	_, err := DB.Exec(`DELETE FROM custom_subscriptions WHERE user_id = ?`, userID)
	return err
}
