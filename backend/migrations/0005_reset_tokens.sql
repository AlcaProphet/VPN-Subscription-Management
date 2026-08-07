CREATE TABLE password_reset_tokens (
    token      TEXT PRIMARY KEY,             -- ≥128 位加密安全随机值
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMP NOT NULL,           -- 1 小时 TTL
    used       INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_reset_tokens_user ON password_reset_tokens(user_id);
