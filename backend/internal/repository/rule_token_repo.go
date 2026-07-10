package repository

import "time"

// RuleTokenRepo provides access to the rule_tokens table.
type RuleTokenRepo struct{}

func NewRuleTokenRepo() *RuleTokenRepo {
	return &RuleTokenRepo{}
}

func (r *RuleTokenRepo) FindByToken(token string) (string, error) {
	var ruleID string
	err := DB.QueryRow(`SELECT rule_id FROM rule_tokens WHERE token = ?`, token).Scan(&ruleID)
	return ruleID, err
}

func (r *RuleTokenRepo) FindByRuleID(ruleID string) (string, error) {
	var token string
	err := DB.QueryRow(`SELECT token FROM rule_tokens WHERE rule_id = ?`, ruleID).Scan(&token)
	return token, err
}

func (r *RuleTokenRepo) Create(token, ruleID string) error {
	_, err := DB.Exec(
		`INSERT INTO rule_tokens (token, rule_id, created_at) VALUES (?, ?, ?)`,
		token, ruleID, time.Now().UTC().Format("2006-01-02 15:04:05"),
	)
	return err
}

func (r *RuleTokenRepo) DeleteByToken(token string) error {
	_, err := DB.Exec(`DELETE FROM rule_tokens WHERE token = ?`, token)
	return err
}

func (r *RuleTokenRepo) DeleteByRuleID(ruleID string) error {
	_, err := DB.Exec(`DELETE FROM rule_tokens WHERE rule_id = ?`, ruleID)
	return err
}
