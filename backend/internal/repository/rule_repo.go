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
	var schemesJSON string
	err := DB.QueryRow(
		`SELECT id, name, client_type, client_schemes, versions, created_at FROM rules WHERE id = ?`,
		id,
	).Scan(&rule.ID, &rule.Name, &rule.ClientType, &schemesJSON, &versionsJSON, &rule.CreatedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(versionsJSON), &rule.Versions); err != nil {
		rule.Versions = []models.Version{}
	}
	if err := json.Unmarshal([]byte(schemesJSON), &rule.ClientSchemes); err != nil {
		rule.ClientSchemes = []string{}
	}
	return rule, nil
}

func (r *RuleRepo) List() ([]models.Rule, error) {
	rows, err := DB.Query(`SELECT id, name, client_type, client_schemes, versions, created_at FROM rules ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.Rule
	for rows.Next() {
		var rule models.Rule
		var versionsJSON string
		var schemesJSON string
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.ClientType, &schemesJSON, &versionsJSON, &rule.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(versionsJSON), &rule.Versions); err != nil {
			rule.Versions = []models.Version{}
		}
		if err := json.Unmarshal([]byte(schemesJSON), &rule.ClientSchemes); err != nil {
			rule.ClientSchemes = []string{}
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (r *RuleRepo) Create(rule *models.Rule) error {
	versionsJSON, _ := json.Marshal(rule.Versions)
	schemesJSON, _ := json.Marshal(rule.ClientSchemes)
	_, err := DB.Exec(
		`INSERT INTO rules (id, name, client_type, client_schemes, versions, created_at) VALUES (?, ?, ?, ?, ?, datetime('now'))`,
		rule.ID, rule.Name, rule.ClientType, string(schemesJSON), string(versionsJSON),
	)
	return err
}

func (r *RuleRepo) Update(rule *models.Rule) error {
	schemesJSON, _ := json.Marshal(rule.ClientSchemes)
	_, err := DB.Exec(
		`UPDATE rules SET name = ?, client_type = ?, client_schemes = ? WHERE id = ?`,
		rule.Name, rule.ClientType, string(schemesJSON), rule.ID,
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
