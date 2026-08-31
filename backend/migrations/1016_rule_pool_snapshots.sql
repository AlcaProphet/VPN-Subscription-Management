-- 1016_rule_pool_snapshots.sql — Build16 不兼容迁移：来源模型、Canonical Rule、快照与 origin。
-- 删除旧素材池业务数据，保留旧 ID 防复用（新池 ID 从旧最大值之后开始）。

CREATE TABLE _pool_1016_old_max (max_id INTEGER NOT NULL);
INSERT INTO _pool_1016_old_max SELECT COALESCE(MAX(id), 0) FROM rule_pools;

DROP TABLE IF EXISTS pool_sync_tasks;
DROP TABLE IF EXISTS pool_entries;
DROP TABLE IF EXISTS rule_pools;

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
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    pool_id            INTEGER NOT NULL REFERENCES rule_pools(id) ON DELETE CASCADE,
    kind               TEXT NOT NULL CHECK (kind IN ('manual','url')),
    url                TEXT,
    source_mode        TEXT NOT NULL DEFAULT 'auto' CHECK (source_mode IN ('clash','shadowrocket','auto')),
    sort_order         INTEGER NOT NULL,
    active_snapshot_id INTEGER,
    pending_snapshot_id INTEGER,
    created_at         TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_pool_sources_pool ON rule_pool_sources(pool_id, sort_order);
CREATE UNIQUE INDEX idx_pool_sources_url ON rule_pool_sources(pool_id, url) WHERE url IS NOT NULL;
CREATE UNIQUE INDEX idx_pool_sources_manual ON rule_pool_sources(pool_id) WHERE kind = 'manual';

CREATE TABLE pool_source_snapshots (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    source_id       INTEGER NOT NULL REFERENCES rule_pool_sources(id) ON DELETE CASCADE,
    format          TEXT NOT NULL DEFAULT '',
    profile         TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL CHECK (status IN ('staging','active','pending','failed')),
    input_count     INTEGER NOT NULL DEFAULT 0,
    recognized_count INTEGER NOT NULL DEFAULT 0,
    accepted_count  INTEGER NOT NULL DEFAULT 0,
    excluded_count  INTEGER NOT NULL DEFAULT 0,
    rejected_count  INTEGER NOT NULL DEFAULT 0,
    duplicate_count INTEGER NOT NULL DEFAULT 0,
    diagnostic_json TEXT NOT NULL DEFAULT '[]',
    stats_json      TEXT NOT NULL DEFAULT '{}',
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
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
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    pool_id          INTEGER NOT NULL REFERENCES rule_pools(id) ON DELETE CASCADE,
    canonical_rule_id INTEGER NOT NULL REFERENCES pool_canonical_rules(id) ON DELETE CASCADE,
    source_id        INTEGER NOT NULL REFERENCES rule_pool_sources(id) ON DELETE CASCADE,
    snapshot_id      INTEGER REFERENCES pool_source_snapshots(id) ON DELETE CASCADE,
    sort_order       INTEGER NOT NULL,
    raw_line         TEXT NOT NULL DEFAULT '',
    line_no          INTEGER NOT NULL DEFAULT 0,
    created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
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

DELETE FROM sqlite_sequence WHERE name = 'rule_pools';
INSERT INTO sqlite_sequence(name, seq)
SELECT 'rule_pools', max_id FROM _pool_1016_old_max;

DROP TABLE _pool_1016_old_max;
