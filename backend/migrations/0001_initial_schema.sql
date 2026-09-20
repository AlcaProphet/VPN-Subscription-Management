-- 首版数据库基线：直接建立当前最终结构，不包含开发期升级、回填或旧 ID 保留。
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE system_config (
    key        TEXT PRIMARY KEY,
    value      TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE users (
    id                     INTEGER PRIMARY KEY AUTOINCREMENT,
    oidc_subject           TEXT UNIQUE,
    username               TEXT NOT NULL,
    email                  TEXT UNIQUE,
    role                   TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin','user')),
    group_id               INTEGER,
    password_hash          TEXT,
    user_source            TEXT NOT NULL CHECK (user_source IN ('oidc','local','selfreg')),
    status                 TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','active','disabled')),
    credential_version     INTEGER NOT NULL DEFAULT 0,
    oidc_claims            TEXT,
    created_at             TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at             TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    quota_override         REAL,
    uuid_encrypted         TEXT,
    expire_at              TEXT,
    quota_exceeded         INTEGER NOT NULL DEFAULT 0,
    proxy_secret_encrypted TEXT
);
CREATE INDEX idx_users_group_id ON users(group_id);

CREATE TABLE groups (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    slug           TEXT NOT NULL UNIQUE,
    name           TEXT NOT NULL UNIQUE,
    is_default     INTEGER NOT NULL DEFAULT 0,
    needs_reselect INTEGER NOT NULL DEFAULT 0,
    created_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    default_quota  REAL
);

CREATE TABLE platforms (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    slug           TEXT NOT NULL UNIQUE,
    name           TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    schemes        TEXT NOT NULL DEFAULT '[]',
    extra_headers  TEXT NOT NULL DEFAULT '{}',
    created_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    installer_files TEXT NOT NULL DEFAULT '[]',
    installer_urls  TEXT NOT NULL DEFAULT '[]',
    product_type    TEXT NOT NULL DEFAULT 'yaml',
    is_default      INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE oidc_states (
    state         TEXT PRIMARY KEY,
    code_verifier TEXT NOT NULL,
    intent        TEXT NOT NULL CHECK (intent IN ('login','bind')),
    bind_user_id  INTEGER,
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    nonce         TEXT NOT NULL DEFAULT '',
    provider_type TEXT NOT NULL DEFAULT '',
    config_hash   TEXT NOT NULL DEFAULT '',
    redirect_uri  TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_oidc_states_created ON oidc_states(created_at);

CREATE TABLE password_reset_tokens (
    token      TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMP NOT NULL,
    used       INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_reset_tokens_user ON password_reset_tokens(user_id);

CREATE TABLE subscriptions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    slug            TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    platform_id     INTEGER NOT NULL REFERENCES platforms(id),
    current_version INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    product_type    TEXT NOT NULL DEFAULT 'yaml'
);
CREATE UNIQUE INDEX idx_subscriptions_platform_uniq ON subscriptions(platform_id);

CREATE TABLE versions (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_type TEXT NOT NULL CHECK (owner_type IN ('subscription','rule','custom','share')),
    owner_id   INTEGER NOT NULL,
    version_no INTEGER NOT NULL,
    file_path  TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    file_name  TEXT NOT NULL DEFAULT '',
    UNIQUE (owner_type, owner_id, version_no)
);
CREATE INDEX idx_versions_owner ON versions(owner_type, owner_id, version_no);

CREATE TABLE custom_subscriptions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    slug            TEXT NOT NULL UNIQUE,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform_id     INTEGER NOT NULL REFERENCES platforms(id) ON DELETE CASCADE,
    current_version INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, platform_id)
);
CREATE INDEX idx_custom_user ON custom_subscriptions(user_id);
CREATE INDEX idx_custom_platform ON custom_subscriptions(platform_id);

CREATE TABLE share_subscriptions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    slug            TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    current_version INTEGER NOT NULL DEFAULT 0,
    token_status    TEXT NOT NULL DEFAULT 'active' CHECK (token_status IN ('active','revoked')),
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE rules (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    slug            TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    client_type     TEXT NOT NULL DEFAULT 'shadowrocket',
    schemes         TEXT NOT NULL DEFAULT '[]',
    current_version INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_home_default INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX idx_rules_home_default ON rules(is_home_default) WHERE is_home_default = 1;

CREATE TABLE download_tokens (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    token           TEXT NOT NULL UNIQUE,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform_id     INTEGER NOT NULL REFERENCES platforms(id) ON DELETE CASCADE,
    custom_sub_id   INTEGER REFERENCES custom_subscriptions(id) ON DELETE CASCADE,
    subscription_id INTEGER REFERENCES subscriptions(id) ON DELETE CASCADE,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_dt_user_platform ON download_tokens(user_id, platform_id);

CREATE TABLE share_tokens (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    token      TEXT NOT NULL UNIQUE,
    share_id   INTEGER NOT NULL REFERENCES share_subscriptions(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE rule_tokens (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    token        TEXT NOT NULL UNIQUE,
    rule_id      INTEGER NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    refreshed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE access_logs (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id       INTEGER,
    ip            TEXT NOT NULL,
    download_type TEXT NOT NULL,
    platform      TEXT,
    resource_slug TEXT NOT NULL,
    status        TEXT NOT NULL,
    fail_reason   TEXT,
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_access_logs_created ON access_logs(created_at);

CREATE TABLE xray_instances (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL UNIQUE,
    slug            TEXT NOT NULL UNIQUE,
    api_addr        TEXT NOT NULL,
    api_tag         TEXT NOT NULL DEFAULT '',
    enabled         INTEGER NOT NULL DEFAULT 1,
    last_collect_at TIMESTAMP,
    collect_status  TEXT NOT NULL DEFAULT '',
    collect_error   TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE nodes (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    source               TEXT NOT NULL CHECK (source IN ('manual','xray')),
    name                 TEXT NOT NULL UNIQUE,
    display_name         TEXT,
    instance_id          INTEGER REFERENCES xray_instances(id) ON DELETE CASCADE,
    tag                  TEXT,
    protocol             TEXT NOT NULL,
    host                 TEXT NOT NULL,
    port                 INTEGER NOT NULL,
    protocol_json        TEXT NOT NULL DEFAULT '{}',
    is_public            INTEGER NOT NULL DEFAULT 0,
    enabled              INTEGER NOT NULL DEFAULT 1,
    allocatable          INTEGER NOT NULL DEFAULT 1,
    last_seen_at         TIMESTAMP,
    missing              INTEGER NOT NULL DEFAULT 0,
    created_at           TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at           TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    edit_revision        INTEGER NOT NULL DEFAULT 0,
    state_format_version INTEGER NOT NULL DEFAULT 1,
    current_state_json   TEXT NOT NULL DEFAULT '{}',
    extensions_json      TEXT NOT NULL DEFAULT '{}',
    UNIQUE (instance_id, tag),
    CHECK ((source = 'xray' AND instance_id IS NOT NULL) OR (source = 'manual' AND instance_id IS NULL))
);
CREATE INDEX idx_nodes_instance ON nodes(instance_id);
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
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ym         TEXT NOT NULL,
    uplink     INTEGER NOT NULL DEFAULT 0,
    downlink   INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, ym)
);

CREATE TABLE assembly_blueprints (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    version_id         INTEGER NOT NULL UNIQUE REFERENCES versions(id) ON DELETE CASCADE,
    target_syntax      TEXT NOT NULL CHECK (target_syntax IN ('clash-yaml','sr-subs','generic-subs','sr-conf')),
    fixed_params_json  TEXT NOT NULL DEFAULT '{}',
    selection_json     TEXT NOT NULL DEFAULT '{}',
    custom_rules_json  TEXT NOT NULL DEFAULT '[]',
    render_plan_json   TEXT NOT NULL DEFAULT '{}',
    created_at         TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    platform_id        INTEGER,
    rule_id            INTEGER
);

CREATE TABLE xray_ext_accounts (
    id                     INTEGER PRIMARY KEY AUTOINCREMENT,
    name                   TEXT NOT NULL UNIQUE,
    email                  TEXT NOT NULL UNIQUE,
    uuid_encrypted         TEXT,
    proxy_secret_encrypted TEXT,
    quota                  REAL,
    quota_exceeded         INTEGER NOT NULL DEFAULT 0,
    created_at             TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at             TIMESTAMP DEFAULT CURRENT_TIMESTAMP
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
    action         TEXT NOT NULL DEFAULT 'add' CHECK (action IN ('add','remove')),
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

CREATE TABLE oidc_login_tickets (
    ticket        TEXT PRIMARY KEY,
    session_token TEXT NOT NULL,
    expires_at    TIMESTAMP NOT NULL,
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    flow_hash     TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_oidc_login_tickets_exp ON oidc_login_tickets(expires_at);

CREATE TABLE rule_pools (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    name           TEXT NOT NULL UNIQUE,
    last_synced_at TIMESTAMP,
    sync_status    TEXT NOT NULL DEFAULT '',
    sync_error     TEXT NOT NULL DEFAULT '',
    auto_sync      INTEGER NOT NULL DEFAULT 0,
    sync_time      TEXT NOT NULL DEFAULT '04:00',
    created_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE rule_pool_sources (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    pool_id             INTEGER NOT NULL REFERENCES rule_pools(id) ON DELETE CASCADE,
    kind                TEXT NOT NULL CHECK (kind IN ('manual','url')),
    url                 TEXT,
    source_mode         TEXT NOT NULL DEFAULT 'auto' CHECK (source_mode IN ('clash','shadowrocket','auto')),
    sort_order          INTEGER NOT NULL,
    active_snapshot_id  INTEGER,
    pending_snapshot_id INTEGER,
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_pool_sources_pool ON rule_pool_sources(pool_id, sort_order);
CREATE UNIQUE INDEX idx_pool_sources_url ON rule_pool_sources(pool_id, url) WHERE url IS NOT NULL;
CREATE UNIQUE INDEX idx_pool_sources_manual ON rule_pool_sources(pool_id) WHERE kind = 'manual';

CREATE TABLE pool_source_snapshots (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    source_id        INTEGER NOT NULL REFERENCES rule_pool_sources(id) ON DELETE CASCADE,
    format           TEXT NOT NULL DEFAULT '',
    profile          TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL CHECK (status IN ('staging','active','pending','failed')),
    input_count      INTEGER NOT NULL DEFAULT 0,
    recognized_count INTEGER NOT NULL DEFAULT 0,
    accepted_count   INTEGER NOT NULL DEFAULT 0,
    excluded_count   INTEGER NOT NULL DEFAULT 0,
    rejected_count   INTEGER NOT NULL DEFAULT 0,
    duplicate_count  INTEGER NOT NULL DEFAULT 0,
    diagnostic_json  TEXT NOT NULL DEFAULT '[]',
    stats_json       TEXT NOT NULL DEFAULT '{}',
    created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    activated_at     TIMESTAMP
);
CREATE INDEX idx_pool_snapshots_source ON pool_source_snapshots(source_id, id DESC);

CREATE TABLE pool_canonical_rules (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    pool_id      INTEGER NOT NULL REFERENCES rule_pools(id) ON DELETE CASCADE,
    semantic_key TEXT NOT NULL,
    family       TEXT NOT NULL,
    matcher      TEXT NOT NULL,
    value        TEXT NOT NULL,
    options_json TEXT NOT NULL DEFAULT '{}',
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (pool_id, semantic_key)
);
CREATE INDEX idx_pool_canonical_pool ON pool_canonical_rules(pool_id);

CREATE TABLE pool_rule_origins (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    pool_id           INTEGER NOT NULL REFERENCES rule_pools(id) ON DELETE CASCADE,
    canonical_rule_id INTEGER NOT NULL REFERENCES pool_canonical_rules(id) ON DELETE CASCADE,
    source_id         INTEGER NOT NULL REFERENCES rule_pool_sources(id) ON DELETE CASCADE,
    snapshot_id       INTEGER REFERENCES pool_source_snapshots(id) ON DELETE CASCADE,
    sort_order        INTEGER NOT NULL,
    raw_line          TEXT NOT NULL DEFAULT '',
    line_no           INTEGER NOT NULL DEFAULT 0,
    created_at        TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_pool_origins_rule ON pool_rule_origins(canonical_rule_id);
CREATE INDEX idx_pool_origins_source ON pool_rule_origins(source_id, snapshot_id);

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

CREATE TABLE mail_result_logs (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    kind             TEXT NOT NULL,
    source           TEXT NOT NULL,
    user_id          INTEGER,
    recipient_masked TEXT NOT NULL,
    result           TEXT NOT NULL CHECK (result IN ('accepted', 'failed')),
    failure_stage    TEXT,
    recorded_at      TIMESTAMP NOT NULL,
    CHECK (result = 'failed' OR failure_stage IS NULL)
);
CREATE INDEX idx_mail_result_logs_recorded_at ON mail_result_logs(recorded_at);

INSERT INTO proxy_groups (name, type, preset_key, enabled, definition_json) VALUES
  ('🎬YouTube',      'preset', 'youtube',          1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}'),
  ('🍿Netflix',      'preset', 'netflix',          1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}'),
  ('🍻哔哩哔哩',     'preset', 'bilibili',         1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}'),
  ('📽️国外流媒体',   'preset', 'global-streaming', 1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}'),
  ('🍎苹果海外服务', 'preset', 'apple-overseas',   1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}'),
  ('🍏苹果国内服务', 'preset', 'apple-cn',         1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}'),
  ('🤖AI',           'preset', 'ai',               1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}'),
  ('🎮Steam',        'preset', 'steam',            1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}'),
  ('🧩Steam下载',    'preset', 'steam-download',   1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}');
