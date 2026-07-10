package repository

import "time"

// OIDCStateRepo provides access to the oidc_state table.
type OIDCStateRepo struct{}

func NewOIDCStateRepo() *OIDCStateRepo {
	return &OIDCStateRepo{}
}

// Create stores a new OIDC state record with code_verifier and nonce.
func (r *OIDCStateRepo) Create(state, codeVerifier, nonce string) error {
	_, err := DB.Exec(
		`INSERT INTO oidc_state (state, code_verifier, nonce, created_at) VALUES (?, ?, ?, ?)`,
		state, codeVerifier, nonce, time.Now().UTC().Format("2006-01-02 15:04:05"),
	)
	return err
}

// FindByState retrieves the code_verifier for a given state.
func (r *OIDCStateRepo) FindByState(state string) (codeVerifier string, err error) {
	err = DB.QueryRow(
		`SELECT code_verifier FROM oidc_state WHERE state = ?`, state,
	).Scan(&codeVerifier)
	return
}

// Delete removes an OIDC state record (used after successful callback to prevent replay).
func (r *OIDCStateRepo) Delete(state string) error {
	_, err := DB.Exec(`DELETE FROM oidc_state WHERE state = ?`, state)
	return err
}
