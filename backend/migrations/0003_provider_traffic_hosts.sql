-- 将已保存的服务商绑定迁移成独立主机。V.PS 的凭据由同一账号下的主机共享。
ALTER TABLE provider_traffic_settings RENAME TO provider_traffic_settings_v2;

CREATE TABLE provider_traffic_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    enabled INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),
    vps_credential_ciphertext TEXT,
    vps_revision TEXT,
    CHECK ((vps_credential_ciphertext IS NULL AND vps_revision IS NULL)
        OR (vps_credential_ciphertext IS NOT NULL AND vps_credential_ciphertext <> ''
            AND vps_revision IS NOT NULL AND vps_revision <> ''))
);

INSERT INTO provider_traffic_settings (id, enabled, vps_credential_ciphertext, vps_revision)
SELECT id, enabled, vps_credential_ciphertext, vps_revision FROM provider_traffic_settings_v2;

CREATE TABLE provider_traffic_hosts (
    id TEXT PRIMARY KEY NOT NULL CHECK (id <> ''),
    settings_id INTEGER NOT NULL DEFAULT 1 REFERENCES provider_traffic_settings(id) ON DELETE CASCADE,
    provider TEXT NOT NULL CHECK (provider IN ('kiwivm', 'vps')),
    instance_id TEXT NOT NULL CHECK (instance_id <> ''),
    name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 64),
    enabled INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),
    half_remaining INTEGER NOT NULL DEFAULT 0 CHECK (half_remaining IN (0, 1)),
    credential_ciphertext TEXT,
    revision TEXT NOT NULL CHECK (revision <> ''),
    edit_revision TEXT NOT NULL CHECK (edit_revision <> ''),
    UNIQUE (provider, instance_id),
    CHECK ((provider = 'kiwivm' AND credential_ciphertext IS NOT NULL AND credential_ciphertext <> '')
        OR (provider = 'vps' AND credential_ciphertext IS NULL))
);

INSERT INTO provider_traffic_hosts (id, provider, instance_id, name, enabled, credential_ciphertext, revision, edit_revision)
SELECT 'kiwivm', 'kiwivm', kiwivm_instance_id, '搬瓦工', kiwivm_enabled, kiwivm_credential_ciphertext, kiwivm_revision, kiwivm_revision
FROM provider_traffic_settings_v2 WHERE kiwivm_instance_id IS NOT NULL;

INSERT INTO provider_traffic_hosts (id, provider, instance_id, name, enabled, revision, edit_revision)
SELECT 'vps', 'vps', vps_instance_id, 'V.PS', vps_enabled, vps_revision, vps_revision
FROM provider_traffic_settings_v2 WHERE vps_instance_id IS NOT NULL;

DROP TABLE provider_traffic_settings_v2;
