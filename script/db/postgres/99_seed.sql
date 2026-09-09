-- auto-pack 平台库 DDL（postgres）
-- PostgreSQL 平台库。
-- 改字段请改 script/db/gen_ddl.py 后重新生成。
-- 禁止 FOREIGN KEY / REFERENCES。密钥只存密文。

-- 预置角色动作。用户由应用层 seed。
INSERT INTO "AP_ROLE_ACTION" ("rec_id", "role_code", "action_code", "created_at", "updated_at") VALUES
    ('ra-owner-js', 'owner', 'job.submit', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('ra-owner-jc', 'owner', 'job.cancel', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('ra-owner-pw', 'owner', 'project.write', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('ra-owner-ew', 'owner', 'env.write', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('ra-owner-ua', 'owner', 'user.admin', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('ra-owner-sw', 'owner', 'settings.write', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('ra-owner-ar', 'owner', 'audit.read', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('ra-op-js', 'operator', 'job.submit', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('ra-op-jc', 'operator', 'job.cancel', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('ra-op-pw', 'operator', 'project.write', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('ra-op-ew', 'operator', 'env.write', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('ra-op-ar', 'operator', 'audit.read', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('ra-vw-ar', 'viewer', 'audit.read', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("role_code", "action_code") DO NOTHING;
