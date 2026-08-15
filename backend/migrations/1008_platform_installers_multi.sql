-- 多安装包/多下载链接升级：installer_file/installer_url 单值列 → installer_files/installer_urls JSON 数组列
--   installer_files: [{name: 原始文件名, file: 磁盘文件名}] / installer_urls: [{name: 展示名, url: 外部地址}]
ALTER TABLE platforms ADD COLUMN installer_files TEXT NOT NULL DEFAULT '[]';
ALTER TABLE platforms ADD COLUMN installer_urls TEXT NOT NULL DEFAULT '[]';
-- 存量数据迁移（单值 → 单元素数组；原始文件名历史未存，以磁盘文件名为展示名）
UPDATE platforms
   SET installer_files = CASE WHEN installer_file IS NOT NULL AND installer_file <> ''
         THEN json_array(json_object('name', installer_file, 'file', installer_file)) ELSE '[]' END,
       installer_urls  = CASE WHEN installer_url IS NOT NULL AND installer_url <> ''
         THEN json_array(json_object('name', '', 'url', installer_url)) ELSE '[]' END;
-- 旧单值列下线（SQLite ≥3.35 支持 DROP COLUMN；无索引/触发器引用）
ALTER TABLE platforms DROP COLUMN installer_file;
ALTER TABLE platforms DROP COLUMN installer_url;
