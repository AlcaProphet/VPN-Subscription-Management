-- 自定义订阅与分享订阅（Build2 Step 5）

-- 自定义订阅：管理员为「特定用户 + 特定平台」上传，覆盖组分配；每用户每平台最多一份
CREATE TABLE custom_subscriptions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    slug            TEXT NOT NULL UNIQUE,          -- custom- + 8 短码自动生成（四类命名空间）
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform_id     INTEGER NOT NULL REFERENCES platforms(id) ON DELETE CASCADE,
    current_version INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, platform_id)                  -- 每用户每平台最多一份
);
CREATE INDEX idx_custom_user ON custom_subscriptions(user_id);
CREATE INDEX idx_custom_platform ON custom_subscriptions(platform_id);

-- 分享订阅：不绑定用户的公开分享链接，支持 Token 刷新与吊销
CREATE TABLE share_subscriptions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    slug            TEXT NOT NULL UNIQUE,          -- share- + 8 短码自动生成（四类命名空间）
    name            TEXT NOT NULL,                 -- 不强制唯一；创建后仅可改名
    current_version INTEGER NOT NULL DEFAULT 0,
    token_status    TEXT NOT NULL DEFAULT 'active' CHECK (token_status IN ('active','revoked')),
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
