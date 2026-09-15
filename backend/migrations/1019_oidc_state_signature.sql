-- R31-03：OIDC state 固定发起时的提供商与配置边界。
-- state 写入 provider_type 与发起时参数原始 JSON 的哈希；
-- 旧记录保留原样（两列为空），ConsumeState 在回调时拒绝，过期后由既有 TTL 清理。
ALTER TABLE oidc_states ADD COLUMN provider_type TEXT NOT NULL DEFAULT '';
ALTER TABLE oidc_states ADD COLUMN config_hash TEXT NOT NULL DEFAULT '';
