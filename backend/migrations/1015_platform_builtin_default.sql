-- 1015_platform_builtin_default.sql — R22-04：默认平台内置标记
ALTER TABLE platforms ADD COLUMN is_default INTEGER NOT NULL DEFAULT 0;

-- 按名称回填三个 Setup 预置默认平台（旧库名称被改后需管理员另行确认）
UPDATE platforms SET is_default = 1
WHERE name IN ('Clash Verge', 'v2rayNG', 'Shadowrocket');
