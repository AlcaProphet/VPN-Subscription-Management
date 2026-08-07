-- 订阅池 + 四类资源共用版本表 + 订阅-组多对多关联（Build2 Step 2）

CREATE TABLE subscriptions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    slug            TEXT NOT NULL UNIQUE,   -- 手填，四类全局命名空间（小写字母数字连字符 3~64）
    name            TEXT NOT NULL,          -- 不强制唯一
    platform_id     INTEGER NOT NULL REFERENCES platforms(id),
    current_version INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_subscriptions_platform ON subscriptions(platform_id);

-- 四类资源共用版本表（owner_type：subscription/rule/custom/share；版本号 1 起递增不复用）
CREATE TABLE versions (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_type TEXT NOT NULL CHECK (owner_type IN ('subscription','rule','custom','share')),
    owner_id   INTEGER NOT NULL,
    version_no INTEGER NOT NULL,
    file_path  TEXT NOT NULL,               -- 相对内容根的文件路径
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (owner_type, owner_id, version_no)
);
CREATE INDEX idx_versions_owner ON versions(owner_type, owner_id, version_no);

-- 订阅-组多对多关联（本 Step 建表，Step 3 使用）
CREATE TABLE subscription_group_rel (
    subscription_id INTEGER NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    group_id        INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    PRIMARY KEY (subscription_id, group_id)
);
CREATE INDEX idx_sgr_group ON subscription_group_rel(group_id);
