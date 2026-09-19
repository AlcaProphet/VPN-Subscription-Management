-- 邮件终态结果日志：best-effort 运维记录，不参与邮件派发或恢复。
CREATE TABLE mail_result_logs (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    kind             TEXT NOT NULL,
    source           TEXT NOT NULL,
    user_id          INTEGER,
    recipient_masked TEXT NOT NULL,
    result           TEXT NOT NULL CHECK (result IN ('accepted', 'failed')),
    failure_stage    TEXT,
    recorded_at      TIMESTAMP NOT NULL,
    CHECK (result = 'failed' OR failure_stage IS NULL)
);

CREATE INDEX idx_mail_result_logs_recorded_at
    ON mail_result_logs(recorded_at);
