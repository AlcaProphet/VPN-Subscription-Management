package repository

import (
	"encoding/json"
	"vpn-sub/internal/models"
)

// ShareSubscriptionRepo provides access to the share_subscriptions table.
type ShareSubscriptionRepo struct{}

func NewShareSubscriptionRepo() *ShareSubscriptionRepo {
	return &ShareSubscriptionRepo{}
}

func (r *ShareSubscriptionRepo) FindByID(id string) (*models.ShareSubscription, error) {
	ss := &models.ShareSubscription{}
	var versionsJSON string
	err := DB.QueryRow(
		`SELECT id, name, versions, created_at FROM share_subscriptions WHERE id = ?`,
		id,
	).Scan(&ss.ID, &ss.Name, &versionsJSON, &ss.CreatedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(versionsJSON), &ss.Versions); err != nil {
		ss.Versions = []models.Version{}
	}
	return ss, nil
}

func (r *ShareSubscriptionRepo) List() ([]models.ShareSubscription, error) {
	rows, err := DB.Query(`SELECT id, name, versions, created_at FROM share_subscriptions ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shares []models.ShareSubscription
	for rows.Next() {
		var ss models.ShareSubscription
		var versionsJSON string
		if err := rows.Scan(&ss.ID, &ss.Name, &versionsJSON, &ss.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(versionsJSON), &ss.Versions); err != nil {
			ss.Versions = []models.Version{}
		}
		shares = append(shares, ss)
	}
	return shares, rows.Err()
}

func (r *ShareSubscriptionRepo) Create(ss *models.ShareSubscription) error {
	versionsJSON, _ := json.Marshal(ss.Versions)
	_, err := DB.Exec(
		`INSERT INTO share_subscriptions (id, name, versions, created_at) VALUES (?, ?, ?, datetime('now'))`,
		ss.ID, ss.Name, string(versionsJSON),
	)
	return err
}

func (r *ShareSubscriptionRepo) Update(ss *models.ShareSubscription) error {
	_, err := DB.Exec(
		`UPDATE share_subscriptions SET name = ? WHERE id = ?`,
		ss.Name, ss.ID,
	)
	return err
}

func (r *ShareSubscriptionRepo) UpdateVersions(id string, versions []models.Version) error {
	versionsJSON, _ := json.Marshal(versions)
	_, err := DB.Exec(`UPDATE share_subscriptions SET versions = ? WHERE id = ?`, string(versionsJSON), id)
	return err
}

func (r *ShareSubscriptionRepo) Delete(id string) error {
	_, err := DB.Exec(`DELETE FROM share_subscriptions WHERE id = ?`, id)
	return err
}
