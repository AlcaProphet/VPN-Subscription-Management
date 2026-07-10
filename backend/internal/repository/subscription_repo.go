package repository

import (
	"encoding/json"
	"vpn-sub/internal/models"
)

// SubscriptionRepo provides access to the subscriptions table.
type SubscriptionRepo struct{}

func NewSubscriptionRepo() *SubscriptionRepo {
	return &SubscriptionRepo{}
}

func (r *SubscriptionRepo) FindByID(id string) (*models.Subscription, error) {
	s := &models.Subscription{}
	var versionsJSON string
	err := DB.QueryRow(
		`SELECT id, name, platform, type, versions FROM subscriptions WHERE id = ?`,
		id,
	).Scan(&s.ID, &s.Name, &s.Platform, &s.Type, &versionsJSON)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(versionsJSON), &s.Versions)
	return s, nil
}

func (r *SubscriptionRepo) FindByPlatformAndType(platform, subType string) (*models.Subscription, error) {
	s := &models.Subscription{}
	var versionsJSON string
	err := DB.QueryRow(
		`SELECT id, name, platform, type, versions FROM subscriptions WHERE platform = ? AND type = ?`,
		platform, subType,
	).Scan(&s.ID, &s.Name, &s.Platform, &s.Type, &versionsJSON)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(versionsJSON), &s.Versions)
	return s, nil
}

func (r *SubscriptionRepo) List() ([]models.Subscription, error) {
	rows, err := DB.Query(`SELECT id, name, platform, type, versions FROM subscriptions ORDER BY platform, type`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []models.Subscription
	for rows.Next() {
		var s models.Subscription
		var versionsJSON string
		if err := rows.Scan(&s.ID, &s.Name, &s.Platform, &s.Type, &versionsJSON); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(versionsJSON), &s.Versions)
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

func (r *SubscriptionRepo) Create(s *models.Subscription) error {
	versionsJSON, _ := json.Marshal(s.Versions)
	_, err := DB.Exec(
		`INSERT INTO subscriptions (id, name, platform, type, versions) VALUES (?, ?, ?, ?, ?)`,
		s.ID, s.Name, s.Platform, s.Type, string(versionsJSON),
	)
	return err
}

func (r *SubscriptionRepo) UpdateVersions(id string, versions []models.Version) error {
	versionsJSON, _ := json.Marshal(versions)
	_, err := DB.Exec(`UPDATE subscriptions SET versions = ? WHERE id = ?`, string(versionsJSON), id)
	return err
}

func (r *SubscriptionRepo) Delete(id string) error {
	_, err := DB.Exec(`DELETE FROM subscriptions WHERE id = ?`, id)
	return err
}

func (r *SubscriptionRepo) ListByPlatform(platform string) ([]models.Subscription, error) {
	rows, err := DB.Query(
		`SELECT id, name, platform, type, versions FROM subscriptions WHERE platform = ? ORDER BY type`,
		platform,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []models.Subscription
	for rows.Next() {
		var s models.Subscription
		var versionsJSON string
		if err := rows.Scan(&s.ID, &s.Name, &s.Platform, &s.Type, &versionsJSON); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(versionsJSON), &s.Versions)
		subs = append(subs, s)
	}
	return subs, rows.Err()
}
