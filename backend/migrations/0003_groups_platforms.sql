CREATE TABLE groups (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    slug           TEXT NOT NULL UNIQUE,      -- group- + 8 位随机短码（独立命名空间）
    name           TEXT NOT NULL UNIQUE,
    is_default     INTEGER NOT NULL DEFAULT 0,
    needs_reselect INTEGER NOT NULL DEFAULT 0, -- Build2 使用
    created_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE platforms (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    slug            TEXT NOT NULL UNIQUE,     -- platform- + 8 位随机短码（独立命名空间）
    name            TEXT NOT NULL,            -- 不强制唯一
    description     TEXT NOT NULL DEFAULT '',
    schemes         TEXT NOT NULL DEFAULT '[]', -- JSON 数组，有序；含 {url} 占位符
    extra_headers   TEXT NOT NULL DEFAULT '{}', -- JSON 键值对；值支持 {frontend_url} 占位符
    installer_file  TEXT,                     -- 本地上传安装包（带时间戳文件名，Build2 使用）
    installer_url   TEXT,                     -- 外部链接
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
