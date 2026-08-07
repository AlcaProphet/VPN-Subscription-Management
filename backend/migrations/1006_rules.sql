-- 分流规则（Build2 Step 6）

CREATE TABLE rules (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    slug            TEXT NOT NULL UNIQUE,    -- 手填，四类命名空间交叉校验
    name            TEXT NOT NULL,           -- 不强制唯一
    client_type     TEXT NOT NULL DEFAULT 'shadowrocket', -- 当前仅 shadowrocket；创建后不可改
    schemes         TEXT NOT NULL DEFAULT '[]',           -- JSON 数组，含 {url} 占位符；创建后不可改
    current_version INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
-- rule_tokens 表已在 Build2 Step 4（1004）建立；slug 查重至此实现全四表生效（替换 Step 2 的跳过逻辑）
