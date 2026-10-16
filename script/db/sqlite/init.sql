-- auto-pack 平台库 DDL（sqlite）
-- SQLite 平台库。默认离线。
-- 改字段请改 script/db/gen_ddl.py 后重新生成。
-- 禁止 FOREIGN KEY / REFERENCES。密钥只存密文。

-- 登录用户。口令只存 bcrypt 哈希。
CREATE TABLE IF NOT EXISTS "AP_USER" (
  "rec_id" TEXT NOT NULL,
  "login_name" TEXT NOT NULL,
  "display_name" TEXT NOT NULL,
  "password_hash" TEXT NOT NULL,
  "is_enabled" INTEGER NOT NULL DEFAULT 1,
  "remark" TEXT,
  "created_at" TEXT NOT NULL DEFAULT (datetime('now')),
  "updated_at" TEXT NOT NULL DEFAULT (datetime('now')),
  CONSTRAINT PK_AP_USER PRIMARY KEY ("rec_id")
);

CREATE UNIQUE INDEX IF NOT EXISTS "UK_AP_USER_LOGIN" ON "AP_USER" ("login_name");

-- 工作空间。配置与作业的边界。
CREATE TABLE IF NOT EXISTS "AP_WORKSPACE" (
  "rec_id" TEXT NOT NULL,
  "workspace_code" TEXT NOT NULL,
  "workspace_name" TEXT NOT NULL,
  "owner_user_id" TEXT NOT NULL,
  "remark" TEXT,
  "created_at" TEXT NOT NULL DEFAULT (datetime('now')),
  "updated_at" TEXT NOT NULL DEFAULT (datetime('now')),
  CONSTRAINT PK_AP_WS PRIMARY KEY ("rec_id")
);

CREATE UNIQUE INDEX IF NOT EXISTS "UK_AP_WS_CODE" ON "AP_WORKSPACE" ("workspace_code");
CREATE INDEX IF NOT EXISTS "IDX_AP_WS_OWNER" ON "AP_WORKSPACE" ("owner_user_id");

-- 空间成员。③ 闸认这一行；角色 owner / operator / viewer。
CREATE TABLE IF NOT EXISTS "AP_WORKSPACE_MEMBER" (
  "rec_id" TEXT NOT NULL,
  "workspace_rec_id" TEXT NOT NULL,
  "user_rec_id" TEXT NOT NULL,
  "member_role" TEXT NOT NULL,
  "created_at" TEXT NOT NULL DEFAULT (datetime('now')),
  "updated_at" TEXT NOT NULL DEFAULT (datetime('now')),
  CONSTRAINT PK_AP_WSM PRIMARY KEY ("rec_id")
);

CREATE UNIQUE INDEX IF NOT EXISTS "UK_AP_WSM" ON "AP_WORKSPACE_MEMBER" ("workspace_rec_id", "user_rec_id");
CREATE INDEX IF NOT EXISTS "IDX_AP_WSM_USER" ON "AP_WORKSPACE_MEMBER" ("user_rec_id");

-- 角色已授权动作。无行则拒绝。
CREATE TABLE IF NOT EXISTS "AP_ROLE_ACTION" (
  "rec_id" TEXT NOT NULL,
  "role_code" TEXT NOT NULL,
  "action_code" TEXT NOT NULL,
  "created_at" TEXT NOT NULL DEFAULT (datetime('now')),
  "updated_at" TEXT NOT NULL DEFAULT (datetime('now')),
  CONSTRAINT PK_AP_RA PRIMARY KEY ("rec_id")
);

CREATE UNIQUE INDEX IF NOT EXISTS "UK_AP_RA" ON "AP_ROLE_ACTION" ("role_code", "action_code");

-- 工程配方。不直接跑命令，提交作业时读取。
CREATE TABLE IF NOT EXISTS "AP_PROJECT" (
  "rec_id" TEXT NOT NULL,
  "workspace_rec_id" TEXT NOT NULL,
  "project_name" TEXT NOT NULL,
  "project_type" TEXT NOT NULL,
  "repo_url" TEXT,
  "repo_branch" TEXT,
  "work_rel_path" TEXT,
  "recipe_json" TEXT NOT NULL,
  "cron_expr" TEXT,
  "is_enabled" INTEGER NOT NULL DEFAULT 1,
  "remark" TEXT,
  "created_at" TEXT NOT NULL DEFAULT (datetime('now')),
  "updated_at" TEXT NOT NULL DEFAULT (datetime('now')),
  CONSTRAINT PK_AP_PRJ PRIMARY KEY ("rec_id")
);

CREATE UNIQUE INDEX IF NOT EXISTS "UK_AP_PRJ_NAME" ON "AP_PROJECT" ("workspace_rec_id", "project_name");
CREATE INDEX IF NOT EXISTS "IDX_AP_PRJ_WS" ON "AP_PROJECT" ("workspace_rec_id");

-- 环境凭据。密钥字段只存 APENC1 密文。
CREATE TABLE IF NOT EXISTS "AP_ENV" (
  "rec_id" TEXT NOT NULL,
  "workspace_rec_id" TEXT NOT NULL,
  "env_kind" TEXT NOT NULL,
  "env_name" TEXT NOT NULL,
  "config_cipher" TEXT NOT NULL,
  "config_hint" TEXT,
  "is_enabled" INTEGER NOT NULL DEFAULT 1,
  "created_at" TEXT NOT NULL DEFAULT (datetime('now')),
  "updated_at" TEXT NOT NULL DEFAULT (datetime('now')),
  CONSTRAINT PK_AP_ENV PRIMARY KEY ("rec_id")
);

CREATE INDEX IF NOT EXISTS "IDX_AP_ENV_WS" ON "AP_ENV" ("workspace_rec_id", "env_kind");

-- 一次可审计作业。状态 queued/running/succeeded/failed/canceled。
CREATE TABLE IF NOT EXISTS "AP_JOB" (
  "rec_id" TEXT NOT NULL,
  "workspace_rec_id" TEXT NOT NULL,
  "project_rec_id" TEXT,
  "job_type" TEXT NOT NULL,
  "job_status" TEXT NOT NULL,
  "payload_json" TEXT NOT NULL,
  "error_text" TEXT,
  "created_by" TEXT NOT NULL,
  "started_at" TEXT,
  "finished_at" TEXT,
  "created_at" TEXT NOT NULL DEFAULT (datetime('now')),
  "updated_at" TEXT NOT NULL DEFAULT (datetime('now')),
  CONSTRAINT PK_AP_JOB PRIMARY KEY ("rec_id")
);

CREATE INDEX IF NOT EXISTS "IDX_AP_JOB_WS" ON "AP_JOB" ("workspace_rec_id", "created_at");
CREATE INDEX IF NOT EXISTS "IDX_AP_JOB_PRJ" ON "AP_JOB" ("project_rec_id", "job_status");
CREATE INDEX IF NOT EXISTS "IDX_AP_JOB_ST" ON "AP_JOB" ("job_status");

-- 作业步骤。日志在磁盘 data/jobs/{jobId}/{step}.log。
CREATE TABLE IF NOT EXISTS "AP_JOB_STEP" (
  "rec_id" TEXT NOT NULL,
  "job_rec_id" TEXT NOT NULL,
  "step_key" TEXT NOT NULL,
  "step_name" TEXT NOT NULL,
  "sort_num" INTEGER NOT NULL DEFAULT 0,
  "step_status" TEXT NOT NULL,
  "error_text" TEXT,
  "started_at" TEXT,
  "finished_at" TEXT,
  "created_at" TEXT NOT NULL DEFAULT (datetime('now')),
  "updated_at" TEXT NOT NULL DEFAULT (datetime('now')),
  CONSTRAINT PK_AP_STEP PRIMARY KEY ("rec_id")
);

CREATE INDEX IF NOT EXISTS "IDX_AP_STEP_JOB" ON "AP_JOB_STEP" ("job_rec_id", "sort_num");

-- 操作审计。
CREATE TABLE IF NOT EXISTS "AP_AUDIT_LOG" (
  "rec_id" TEXT NOT NULL,
  "actor_user_id" TEXT NOT NULL,
  "workspace_rec_id" TEXT,
  "action_code" TEXT NOT NULL,
  "target_type" TEXT,
  "target_id" TEXT,
  "summary" TEXT NOT NULL,
  "success_flag" INTEGER NOT NULL DEFAULT 1,
  "created_at" TEXT NOT NULL DEFAULT (datetime('now')),
  "updated_at" TEXT NOT NULL DEFAULT (datetime('now')),
  CONSTRAINT PK_AP_AUD PRIMARY KEY ("rec_id")
);

CREATE INDEX IF NOT EXISTS "IDX_AP_AUD_WS" ON "AP_AUDIT_LOG" ("workspace_rec_id", "created_at");
CREATE INDEX IF NOT EXISTS "IDX_AP_AUD_ACTOR" ON "AP_AUDIT_LOG" ("actor_user_id");

-- 平台系统属性。key/value，供作业并发、日志保留、请求超时等。
CREATE TABLE IF NOT EXISTS "AP_SETTING" (
  "rec_id" TEXT NOT NULL,
  "setting_key" TEXT NOT NULL,
  "setting_value" TEXT NOT NULL,
  "remark" TEXT,
  "created_at" TEXT NOT NULL DEFAULT (datetime('now')),
  "updated_at" TEXT NOT NULL DEFAULT (datetime('now')),
  CONSTRAINT PK_AP_SET PRIMARY KEY ("rec_id")
);

CREATE UNIQUE INDEX IF NOT EXISTS "UK_AP_SET_KEY" ON "AP_SETTING" ("setting_key");

-- 全局环境变量。构建进程可叠加；禁止覆盖 PATH 等保留键。
CREATE TABLE IF NOT EXISTS "AP_GLOBAL_VAR" (
  "rec_id" TEXT NOT NULL,
  "var_key" TEXT NOT NULL,
  "var_value" TEXT NOT NULL,
  "is_enabled" INTEGER NOT NULL DEFAULT 1,
  "remark" TEXT,
  "created_at" TEXT NOT NULL DEFAULT (datetime('now')),
  "updated_at" TEXT NOT NULL DEFAULT (datetime('now')),
  CONSTRAINT PK_AP_GVAR PRIMARY KEY ("rec_id")
);

CREATE UNIQUE INDEX IF NOT EXISTS "UK_AP_GVAR_KEY" ON "AP_GLOBAL_VAR" ("var_key");

CREATE TABLE IF NOT EXISTS "AP_WORKSPACE_RELEASE" (
  "rec_id" TEXT NOT NULL,
  "workspace_rec_id" TEXT NOT NULL,
  "release_num" INTEGER NOT NULL DEFAULT 0,
  "status" TEXT NOT NULL,
  "release_branch" TEXT,
  "tag_branch" TEXT,
  "target_branch" TEXT,
  "released" INTEGER NOT NULL DEFAULT 0,
  "skipped" INTEGER NOT NULL DEFAULT 0,
  "failed" INTEGER NOT NULL DEFAULT 0,
  "message" TEXT,
  "log_path" TEXT,
  "post_options_json" TEXT,
  "started_at" TEXT,
  "finished_at" TEXT,
  "created_at" TEXT NOT NULL DEFAULT (datetime('now')),
  "updated_at" TEXT NOT NULL DEFAULT (datetime('now')),
  CONSTRAINT PK_AP_WSREL PRIMARY KEY ("rec_id")
);

CREATE INDEX IF NOT EXISTS "IDX_AP_WSREL_WS" ON "AP_WORKSPACE_RELEASE" ("workspace_rec_id", "release_num");

CREATE TABLE IF NOT EXISTS "AP_WORKSPACE_BUILD" (
  "rec_id" TEXT NOT NULL,
  "workspace_rec_id" TEXT NOT NULL,
  "build_num" INTEGER NOT NULL DEFAULT 0,
  "status" TEXT NOT NULL,
  "scope" TEXT,
  "succeeded" INTEGER NOT NULL DEFAULT 0,
  "skipped" INTEGER NOT NULL DEFAULT 0,
  "failed" INTEGER NOT NULL DEFAULT 0,
  "message" TEXT,
  "log_path" TEXT,
  "filter_json" TEXT,
  "started_at" TEXT,
  "finished_at" TEXT,
  "created_at" TEXT NOT NULL DEFAULT (datetime('now')),
  "updated_at" TEXT NOT NULL DEFAULT (datetime('now')),
  CONSTRAINT PK_AP_WSBLD PRIMARY KEY ("rec_id")
);

CREATE INDEX IF NOT EXISTS "IDX_AP_WSBLD_WS" ON "AP_WORKSPACE_BUILD" ("workspace_rec_id", "build_num");

-- 队列生产与消费消息。只保留最近 300 条，供回溯。
CREATE TABLE IF NOT EXISTS "AP_MQ_MESSAGE" (
  "rec_id" TEXT NOT NULL,
  "env_rec_id" TEXT NOT NULL,
  "seq_num" INTEGER NOT NULL,
  "direction" TEXT NOT NULL,
  "queue_role" TEXT NOT NULL,
  "queue_name" TEXT NOT NULL,
  "message_id" TEXT,
  "task_id" TEXT,
  "body" TEXT,
  "source" TEXT NOT NULL,
  "deleted_flag" INTEGER NOT NULL DEFAULT 0,
  "actor_user_id" TEXT NOT NULL,
  "created_at" TEXT NOT NULL DEFAULT (datetime('now')),
  "updated_at" TEXT NOT NULL DEFAULT (datetime('now')),
  CONSTRAINT PK_AP_MQMSG PRIMARY KEY ("rec_id")
);

CREATE UNIQUE INDEX IF NOT EXISTS "UK_AP_MQMSG_SEQ" ON "AP_MQ_MESSAGE" ("seq_num");
CREATE INDEX IF NOT EXISTS "IDX_AP_MQMSG_ENV" ON "AP_MQ_MESSAGE" ("env_rec_id", "seq_num");

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
