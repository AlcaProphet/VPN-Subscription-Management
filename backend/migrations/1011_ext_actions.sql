-- 1011_ext_actions.sql — R15-10：独立账号推送记录增加重试动作标识，区分“待新增”与“待移除”。
ALTER TABLE xray_ext_users ADD COLUMN action TEXT NOT NULL DEFAULT 'add' CHECK (action IN ('add','remove'));
