-- 1009_xray.sql — Design2 增量 DDL（基础模式 + 高级模式全量表结构一次落盘；全新部署口径，不迁移旧数据）
-- 0) 既有表增量列
ALTER TABLE platforms ADD COLUMN product_type TEXT NOT NULL DEFAULT 'yaml';
ALTER TABLE rules ADD COLUMN is_home_default INTEGER NOT NULL DEFAULT 0;
CREATE UNIQUE INDEX idx_rules_home_default ON rules(is_home_default) WHERE is_home_default = 1;
ALTER TABLE groups ADD COLUMN default_quota REAL;
ALTER TABLE users ADD COLUMN quota_override REAL;
ALTER TABLE users ADD COLUMN uuid_encrypted TEXT;
ALTER TABLE users ADD COLUMN expire_at TEXT;
ALTER TABLE users ADD COLUMN quota_exceeded INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN proxy_secret_encrypted TEXT;
ALTER TABLE subscriptions ADD COLUMN product_type TEXT NOT NULL DEFAULT 'yaml';
DROP INDEX idx_subscriptions_platform;
CREATE UNIQUE INDEX idx_subscriptions_platform_uniq ON subscriptions(platform_id);

-- f) 规则素材池（基础模式，第二章）
CREATE TABLE rule_pools (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    name           TEXT NOT NULL UNIQUE,
    urls_json      TEXT NOT NULL DEFAULT '[]',
    last_synced_at TIMESTAMP,
    sync_status    TEXT NOT NULL DEFAULT '',
    sync_error     TEXT NOT NULL DEFAULT '',
    auto_sync      INTEGER NOT NULL DEFAULT 0,
    sync_time      TEXT NOT NULL DEFAULT '04:00',
    created_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE pool_entries (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    pool_id     INTEGER NOT NULL REFERENCES rule_pools(id) ON DELETE CASCADE,
    rule_type   TEXT NOT NULL,
    match_value TEXT NOT NULL,
    source      TEXT NOT NULL CHECK (source IN ('url','manual')),
    sort_order  INTEGER NOT NULL,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (pool_id, rule_type, match_value)
);
CREATE INDEX idx_pool_entries_pool_order ON pool_entries(pool_id, sort_order);

CREATE TABLE pool_sync_tasks (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    pool_id      INTEGER NOT NULL REFERENCES rule_pools(id) ON DELETE CASCADE,
    status       TEXT NOT NULL CHECK (status IN ('running','succeeded','failed','partial')),
    per_url_json TEXT NOT NULL DEFAULT '[]',
    error        TEXT NOT NULL DEFAULT '',
    started_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    finished_at  TIMESTAMP,
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_pool_sync_tasks_pool ON pool_sync_tasks(pool_id, id DESC);

-- Xray 实例与统一节点表（高级模式，第三章/第五章；本 Build 仅建表）
CREATE TABLE xray_instances (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    name             TEXT NOT NULL UNIQUE,
    slug             TEXT NOT NULL UNIQUE,
    api_addr         TEXT NOT NULL,
    api_tag          TEXT NOT NULL DEFAULT '',
    enabled          INTEGER NOT NULL DEFAULT 1,
    last_collect_at  TIMESTAMP,
    collect_status   TEXT NOT NULL DEFAULT '',
    collect_error    TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE nodes (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    source        TEXT NOT NULL CHECK (source IN ('manual','xray')),
    name          TEXT NOT NULL UNIQUE,
    display_name  TEXT,
    instance_id   INTEGER REFERENCES xray_instances(id) ON DELETE CASCADE,
    tag           TEXT,
    protocol      TEXT NOT NULL,
    host          TEXT NOT NULL,
    port          INTEGER NOT NULL,
    protocol_json TEXT NOT NULL DEFAULT '{}',
    is_public     INTEGER NOT NULL DEFAULT 0,
    enabled       INTEGER NOT NULL DEFAULT 1,
    allocatable   INTEGER NOT NULL DEFAULT 1,
    last_seen_at  TIMESTAMP,
    missing       INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (instance_id, tag),
    CHECK ((source = 'xray' AND instance_id IS NOT NULL) OR (source = 'manual' AND instance_id IS NULL))
);
CREATE INDEX idx_nodes_instance ON nodes(instance_id);
-- 有效渲染名全局唯一兜底（display_name 非空则用之，否则 name）；跨表（代理组/强制组/Clash-mihomo 内建保留代理名）冲突由应用层校验
CREATE UNIQUE INDEX idx_nodes_render_name ON nodes(COALESCE(NULLIF(display_name,''), name));

CREATE TABLE proxy_groups (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL UNIQUE,
    type            TEXT NOT NULL CHECK (type IN ('preset','custom')),
    preset_key      TEXT,
    enabled         INTEGER NOT NULL DEFAULT 1,
    definition_json TEXT NOT NULL DEFAULT '{}',
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE group_nodes (
    group_id   INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    node_id    INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (group_id, node_id)
);
CREATE INDEX idx_group_nodes_node ON group_nodes(node_id);

CREATE TABLE xray_users (
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    instance_id  INTEGER NOT NULL REFERENCES xray_instances(id) ON DELETE CASCADE,
    inbound_tag  TEXT NOT NULL,
    node_id      INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    email        TEXT NOT NULL,
    sync_status  TEXT NOT NULL CHECK (sync_status IN ('pending','synced','failed')),
    last_error   TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, instance_id, inbound_tag)
);
CREATE INDEX idx_xray_users_node ON xray_users(node_id);

CREATE TABLE traffic_records (
    user_id  INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ym       TEXT NOT NULL,
    uplink   INTEGER NOT NULL DEFAULT 0,
    downlink INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, ym)
);

-- 装配快照（基础模式，第四章；version_id 与 versions 1:1）
CREATE TABLE assembly_blueprints (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    version_id        INTEGER NOT NULL UNIQUE REFERENCES versions(id) ON DELETE CASCADE,
    target_syntax     TEXT NOT NULL CHECK (target_syntax IN ('clash-yaml','sr-subs','generic-subs','sr-conf')),
    fixed_params_json TEXT NOT NULL DEFAULT '{}',
    selection_json    TEXT NOT NULL DEFAULT '{}',
    custom_rules_json TEXT NOT NULL DEFAULT '[]',
    render_plan_json  TEXT NOT NULL DEFAULT '{}',
    created_at        TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 独立 Xray 账号（高级模式，§5.11；本 Build 仅建表）
CREATE TABLE xray_ext_accounts (
    id                       INTEGER PRIMARY KEY AUTOINCREMENT,
    name                     TEXT NOT NULL UNIQUE,
    email                    TEXT NOT NULL UNIQUE,
    uuid_encrypted           TEXT,
    proxy_secret_encrypted   TEXT,
    quota                    REAL,
    quota_exceeded           INTEGER NOT NULL DEFAULT 0,
    created_at               TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at               TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE xray_ext_users (
    ext_account_id INTEGER NOT NULL REFERENCES xray_ext_accounts(id) ON DELETE CASCADE,
    instance_id    INTEGER NOT NULL REFERENCES xray_instances(id) ON DELETE CASCADE,
    inbound_tag    TEXT NOT NULL,
    node_id        INTEGER REFERENCES nodes(id) ON DELETE CASCADE,
    sync_status    TEXT NOT NULL CHECK (sync_status IN ('pending','synced','failed')),
    last_error     TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (ext_account_id, instance_id, inbound_tag)
);
CREATE INDEX idx_xray_ext_users_node ON xray_ext_users(node_id);

CREATE TABLE xray_ext_traffic (
    ext_account_id INTEGER NOT NULL REFERENCES xray_ext_accounts(id) ON DELETE CASCADE,
    ym             TEXT NOT NULL,
    uplink         INTEGER NOT NULL DEFAULT 0,
    downlink       INTEGER NOT NULL DEFAULT 0,
    updated_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (ext_account_id, ym)
);

-- 预设代理组种子（Design2 §3.3）：名称 + 组类型 + 默认成员「🚀直接连接」（强制组名与模板逐字一致）；管理员后续可编辑成员
INSERT INTO proxy_groups (name, type, preset_key, enabled, definition_json) VALUES
  ('🎬YouTube',     'preset', 'youtube',         1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}'),
  ('🍿Netflix',     'preset', 'netflix',         1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}'),
  ('🍻哔哩哔哩',    'preset', 'bilibili',        1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}'),
  ('📽️国外流媒体',  'preset', 'global-streaming', 1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}'),
  ('🍎苹果海外服务','preset', 'apple-overseas',  1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}'),
  ('🍏苹果国内服务','preset', 'apple-cn',        1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}'),
  ('🤖AI',          'preset', 'ai',              1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}'),
  ('🎮Steam',       'preset', 'steam',           1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}'),
  ('🧩Steam下载',   'preset', 'steam-download',  1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}');

-- g) 旧分发模型下线（全新部署口径；业务代码在 Build4 Step 2 全部停止引用）
DROP TABLE group_selections;
DROP TABLE subscription_group_rel;
