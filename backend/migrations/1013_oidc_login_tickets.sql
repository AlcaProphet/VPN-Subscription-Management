CREATE TABLE oidc_login_tickets (
    ticket        TEXT PRIMARY KEY,
    session_token TEXT NOT NULL,
    expires_at    TIMESTAMP NOT NULL,
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_oidc_login_tickets_exp ON oidc_login_tickets(expires_at);
