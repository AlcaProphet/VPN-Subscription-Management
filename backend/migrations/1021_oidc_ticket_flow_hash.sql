-- R31-07：OIDC 登录 ticket 固定签发时的流程指纹。
-- 指纹包含运行模式、流程代际、生效提供商与该提供商参数原始 JSON；
-- 停用/清空后流程代际变化，旧 ticket 的 flow_hash 不再匹配；
-- 迁移前旧 ticket 的 flow_hash 为空串，消费时按无效处理（R31-06 残余边界）。
ALTER TABLE oidc_login_tickets ADD COLUMN flow_hash TEXT NOT NULL DEFAULT '';
