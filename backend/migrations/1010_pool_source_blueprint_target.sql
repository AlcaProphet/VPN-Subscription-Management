-- 1010_pool_source_blueprint_target.sql — 修复 R12-02/R12-13：
-- pool_entries 增加来源 URL，assembly_blueprints 增加装配目标 platform_id/rule_id，并用 versions 回填历史蓝图。
ALTER TABLE pool_entries ADD COLUMN source_url TEXT NOT NULL DEFAULT '';

ALTER TABLE assembly_blueprints ADD COLUMN platform_id INTEGER;
ALTER TABLE assembly_blueprints ADD COLUMN rule_id INTEGER;

-- 历史蓝图回填：subscription 类蓝图从订阅行取 platform_id
UPDATE assembly_blueprints
SET platform_id = (
  SELECT s.platform_id
  FROM versions v
  JOIN subscriptions s ON s.id = v.owner_id
  WHERE v.id = assembly_blueprints.version_id
    AND v.owner_type = 'subscription'
)
WHERE platform_id IS NULL;

-- 历史蓝图回填：rule 类蓝图从版本 owner 取 rule_id
UPDATE assembly_blueprints
SET rule_id = (
  SELECT v.owner_id
  FROM versions v
  WHERE v.id = assembly_blueprints.version_id
    AND v.owner_type = 'rule'
)
WHERE rule_id IS NULL;
