-- R31-05：OIDC state 固定发起授权时使用的 redirect_uri。
-- 地址变化/清除只影响之后新发起的授权，进行中的 state 继续使用写入时的地址。
ALTER TABLE oidc_states ADD COLUMN redirect_uri TEXT NOT NULL DEFAULT '';
