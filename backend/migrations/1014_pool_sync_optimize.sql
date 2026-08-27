-- 1014_pool_sync_optimize.sql — R22-01：素材池差量同步优化
-- 联合索引：按池+来源+类型+匹配值查询删除/统计，避免大数据量下 O(N×M) 扫描。
CREATE INDEX IF NOT EXISTS idx_pool_entries_pool_source_type_value
    ON pool_entries(pool_id, source, rule_type, match_value);
