-- auto-pack 平台库 DDL（sqlite）
-- SQLite 平台库。默认离线。
-- 改字段请改 script/db/gen_ddl.py 后重新生成。
-- 禁止 FOREIGN KEY / REFERENCES。密钥只存密文。

-- 预置角色动作。用户由应用层 seed。
INSERT OR IGNORE INTO "AP_ROLE_ACTION" ("rec_id", "role_code", "action_code", "created_at", "updated_at") VALUES
    ('ra-owner-js', 'owner', 'job.submit', datetime('now'), datetime('now')),
    ('ra-owner-jc', 'owner', 'job.cancel', datetime('now'), datetime('now')),
    ('ra-owner-pw', 'owner', 'project.write', datetime('now'), datetime('now')),
    ('ra-owner-ew', 'owner', 'env.write', datetime('now'), datetime('now')),
    ('ra-owner-ua', 'owner', 'user.admin', datetime('now'), datetime('now')),
    ('ra-owner-sw', 'owner', 'settings.write', datetime('now'), datetime('now')),
    ('ra-owner-ar', 'owner', 'audit.read', datetime('now'), datetime('now')),
    ('ra-op-js', 'operator', 'job.submit', datetime('now'), datetime('now')),
    ('ra-op-jc', 'operator', 'job.cancel', datetime('now'), datetime('now')),
    ('ra-op-pw', 'operator', 'project.write', datetime('now'), datetime('now')),
    ('ra-op-ew', 'operator', 'env.write', datetime('now'), datetime('now')),
    ('ra-op-ar', 'operator', 'audit.read', datetime('now'), datetime('now')),
    ('ra-vw-ar', 'viewer', 'audit.read', datetime('now'), datetime('now'));
