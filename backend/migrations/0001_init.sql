-- 版本化迁移登记表（与 store.Migrate 的 CREATE TABLE IF NOT EXISTS 幂等共存）
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 系统配置键值表：承载全部系统配置（签名密钥、认证参数、开关等，Design1 §5.3）
CREATE TABLE system_config (
    key        TEXT PRIMARY KEY,
    value      TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
