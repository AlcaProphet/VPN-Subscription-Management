-- 1018_pool_snapshot_activated_at.sql — Build22 D3-7：pending 人工激活时间。
-- 为 pool_source_snapshots 增加 nullable activated_at。
-- 历史 active/failed 及尚未人工激活的 pending 不补造激活时间，保持 NULL。

ALTER TABLE pool_source_snapshots ADD COLUMN activated_at TIMESTAMP;
