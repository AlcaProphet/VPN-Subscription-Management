CREATE TABLE oidc_states (
    state        TEXT PRIMARY KEY,           -- ≥128 位随机值
    code_verifier TEXT NOT NULL,             -- PKCE S256 验证器
    intent       TEXT NOT NULL CHECK (intent IN ('login','bind')),
    bind_user_id INTEGER,                    -- intent=bind 时记录目标用户
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_oidc_states_created ON oidc_states(created_at); -- TTL 过期清理用
