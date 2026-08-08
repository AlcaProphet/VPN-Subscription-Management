-- 版本原始文件名（Build2 修复：下载时保留原始扩展名，Issue1 R03）
ALTER TABLE versions ADD COLUMN file_name TEXT NOT NULL DEFAULT '';
