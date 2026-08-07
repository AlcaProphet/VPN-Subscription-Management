-- 下载 Token 体系与访问日志（Build2 Step 4）

-- 用户下载 Token（三态：custom_sub_id 与 subscription_id 互斥且可不填）
CREATE TABLE download_tokens (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    token           TEXT NOT NULL UNIQUE,    -- ≥128 位加密安全随机值（32 字节 base64url）
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform_id     INTEGER NOT NULL REFERENCES platforms(id) ON DELETE CASCADE,
    custom_sub_id   INTEGER REFERENCES custom_subscriptions(id) ON DELETE CASCADE,
    subscription_id INTEGER REFERENCES subscriptions(id) ON DELETE CASCADE,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_dt_user_platform ON download_tokens(user_id, platform_id);
-- 注：复用键唯一性由业务层事务内先查后建保证，不建唯一索引（可选标识 NULL 时索引唯一语义不生效，Design1 §4.2）

-- 分享订阅 Token（每分享至多一份有效；Step 5 使用）
CREATE TABLE share_tokens (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    token      TEXT NOT NULL UNIQUE,
    share_id   INTEGER NOT NULL REFERENCES share_subscriptions(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 规则 Token（每规则一份；Step 6 使用）
CREATE TABLE rule_tokens (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    token        TEXT NOT NULL UNIQUE,
    rule_id      INTEGER NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    refreshed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 访问日志（90 天自动清理）
CREATE TABLE access_logs (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id       INTEGER,               -- 分享/规则下载可空
    ip            TEXT NOT NULL,
    download_type TEXT NOT NULL,         -- subscription/custom/explicit/share/rule
    platform      TEXT,                  -- 平台标识
    resource_slug TEXT NOT NULL,         -- 记录口径见 Build3 Step 5
    status        TEXT NOT NULL,         -- success/fail
    fail_reason   TEXT,                  -- token_invalid/unassigned/...
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_access_logs_created ON access_logs(created_at);
