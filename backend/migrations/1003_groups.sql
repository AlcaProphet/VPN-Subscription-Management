-- 用户组与订阅分发机制（Build2 Step 3）
-- groups 表已在 Build1 Step 5（0003_groups_platforms.sql）建立：
--   id / slug UNIQUE（group-+8 短码）/ name UNIQUE / is_default / needs_reselect 默认 0 / created_at / updated_at
-- 本迁移仅补充组选定表。

-- 组选定：每组每平台至多一份；订阅被删时置空（不回退，由业务层置 needs_reselect）
CREATE TABLE group_selections (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id        INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    platform_id     INTEGER NOT NULL REFERENCES platforms(id) ON DELETE CASCADE,
    subscription_id INTEGER REFERENCES subscriptions(id) ON DELETE SET NULL, -- 订阅被删时置空（不回退）
    UNIQUE (group_id, platform_id)
);
CREATE INDEX idx_gs_platform ON group_selections(platform_id);
CREATE INDEX idx_gs_subscription ON group_selections(subscription_id);
