-- platforms 表已在 Build1 Step 5（0003_groups_platforms.sql）建立；
-- 本迁移仅做列确认（幂等），不重复建表：
--   id / slug UNIQUE（platform-+8 短码，独立命名空间，不参与四类标识校验）/
--   name（不强制唯一）/ description / schemes JSON 有序数组（含 {url} 占位符）/
--   extra_headers JSON 键值对（值支持 {frontend_url} 占位符）/
--   installer_file（带时间戳文件名，可空）/ installer_url（可空）/
--   created_at / updated_at
-- 若未来列结构演进，在此追加 ALTER TABLE 语句（SQLite 支持 ADD COLUMN）。
SELECT 1;
