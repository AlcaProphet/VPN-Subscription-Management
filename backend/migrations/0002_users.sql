CREATE TABLE users (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT, -- 内部稳定 user_id
    oidc_subject       TEXT UNIQUE,          -- OIDC 身份（本 Step 建列，Step 6 使用）
    username           TEXT NOT NULL,
    email              TEXT UNIQUE,          -- SQLite 原生允许多 NULL，NULL 不冲突
    role               TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin','user')),
    group_id           INTEGER,              -- 所属组（默认组在 Step 5 创建后回填，Build2 完整使用）
    password_hash      TEXT,                 -- 空 = 不可本地登录（OIDC-only 用户）
    user_source        TEXT NOT NULL CHECK (user_source IN ('oidc','local','selfreg')),
    status             TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','active','disabled')),
    credential_version INTEGER NOT NULL DEFAULT 0,   -- 凭据版本号（会话失效机制，Design1 §5.4）
    oidc_claims        TEXT,                 -- 待审批 OIDC 用户 claims 快照 JSON（Step 6 使用）
    created_at         TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_users_group_id ON users(group_id);
