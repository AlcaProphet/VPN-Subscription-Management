-- Build41 Step 1：基础模式服务器剩余流量。
-- 独立单行功能表（id=1）：无记录等价于总开关关闭且两家均未绑定；
-- 总开关只控制采集与展示，不改变两家绑定；单家开关关闭时保留绑定与凭据密文。
CREATE TABLE provider_traffic_settings (
    id                          INTEGER PRIMARY KEY CHECK (id = 1),
    enabled                     INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),

    kiwivm_enabled              INTEGER NOT NULL DEFAULT 0 CHECK (kiwivm_enabled IN (0, 1)),
    kiwivm_instance_id          TEXT,
    kiwivm_credential_ciphertext TEXT,
    kiwivm_revision             TEXT,

    vps_enabled                 INTEGER NOT NULL DEFAULT 0 CHECK (vps_enabled IN (0, 1)),
    vps_instance_id             TEXT,
    vps_credential_ciphertext   TEXT,
    vps_revision                TEXT,

    -- 未绑定时该家三项绑定字段必须为空且单家开关为关；绑定后实例、密文和修订必须同时非空。
    CHECK (
        (kiwivm_instance_id IS NULL AND kiwivm_credential_ciphertext IS NULL AND kiwivm_revision IS NULL AND kiwivm_enabled = 0)
        OR
        (kiwivm_instance_id IS NOT NULL AND kiwivm_instance_id <> ''
         AND kiwivm_credential_ciphertext IS NOT NULL AND kiwivm_credential_ciphertext <> ''
         AND kiwivm_revision IS NOT NULL AND kiwivm_revision <> '')
    ),
    CHECK (
        (vps_instance_id IS NULL AND vps_credential_ciphertext IS NULL AND vps_revision IS NULL AND vps_enabled = 0)
        OR
        (vps_instance_id IS NOT NULL AND vps_instance_id <> ''
         AND vps_credential_ciphertext IS NOT NULL AND vps_credential_ciphertext <> ''
         AND vps_revision IS NOT NULL AND vps_revision <> '')
    )
);
