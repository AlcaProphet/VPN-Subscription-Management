package repository

import (
	"encoding/json"
	"vpn-sub/internal/models"
)

// RuleRepo provides access to the rules table.
type RuleRepo struct{}

func NewRuleRepo() *RuleRepo {
	return &RuleRepo{}
}

func (r *RuleRepo) FindByID(id string) (*models.Rule, error) {
	rule := &models.Rule{}
	var versionsJSON string
	err := DB.QueryRow(
		`SELECT id, name, client_type, versions, created_at FROM rules WHERE id = ?`,
		id,
	).Scan(&rule.ID, &rule.Name, &rule.ClientType, &versionsJSON, &rule.CreatedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(versionsJSON), &rule.Versions); err != nil {
		rule.Versions = []models.Version{}
	}
	return rule, nil
}

func (r *RuleRepo) List() ([]models.Rule, error) {
	rows, err := DB.Query(`SELECT id, name, client_type, versions, created_at FROM rules ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.Rule
	for rows.Next() {
		var rule models.Rule
		var versionsJSON string
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.ClientType, &versionsJSON, &rule.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(versionsJSON), &rule.Versions); err != nil {
			rule.Versions = []models.Version{}
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (r *RuleRepo) Create(rule *models.Rule) error {
	versionsJSON, _ := json.Marshal(rule.Versions)
	_, err := DB.Exec(
		`INSERT INTO rules (id, name, client_type, versions, created_at) VALUES (?, ?, ?, ?, datetime('now'))`,
		rule.ID, rule.Name, rule.ClientType, string(versionsJSON),
	)
	return err
}

func (r *RuleRepo) Update(rule *models.Rule) error {
	_, err := DB.Exec(
		`UPDATE rules SET name = ?, client_type = ? WHERE id = ?`,
		rule.Name, rule.ClientType, rule.ID,
	)
	return err
}

func (r *RuleRepo) UpdateVersions(id string, versions []models.Version) error {
	versionsJSON, _ := json.Marshal(versions)
	_, err := DB.Exec(`UPDATE rules SET versions = ? WHERE id = ?`, string(versionsJSON), id)
	return err
}

func (r *RuleRepo) Delete(id string) error {
	_, err := DB.Exec(`DELETE FROM rules WHERE id = ?`, id)
	return err
}
