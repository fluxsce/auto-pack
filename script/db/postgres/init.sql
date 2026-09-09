-- auto-pack 平台库 DDL（postgres）
-- PostgreSQL 平台库。
-- 改字段请改 script/db/gen_ddl.py 后重新生成。
-- 禁止 FOREIGN KEY / REFERENCES。密钥只存密文。

-- createdb autopack

-- 登录用户。口令只存 bcrypt 哈希。
CREATE TABLE IF NOT EXISTS "AP_USER" (
  "rec_id" VARCHAR(36) NOT NULL,
  "login_name" VARCHAR(128) NOT NULL,
  "display_name" VARCHAR(128) NOT NULL,
  "password_hash" VARCHAR(256) NOT NULL,
  "is_enabled" SMALLINT NOT NULL DEFAULT 1,
  "remark" VARCHAR(500),
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT PK_AP_USER PRIMARY KEY ("rec_id")
);

COMMENT ON TABLE "AP_USER" IS '登录用户。口令只存 bcrypt 哈希。';
COMMENT ON COLUMN "AP_USER"."rec_id" IS '主键 UUID';
COMMENT ON COLUMN "AP_USER"."login_name" IS '登录名，全局唯一';
COMMENT ON COLUMN "AP_USER"."display_name" IS '显示名';
COMMENT ON COLUMN "AP_USER"."password_hash" IS 'bcrypt 哈希';
COMMENT ON COLUMN "AP_USER"."is_enabled" IS '1 启用 / 0 停用';
COMMENT ON COLUMN "AP_USER"."remark" IS '备注';
COMMENT ON COLUMN "AP_USER"."created_at" IS '创建时间';
COMMENT ON COLUMN "AP_USER"."updated_at" IS '最后修改时间';

CREATE UNIQUE INDEX IF NOT EXISTS "UK_AP_USER_LOGIN" ON "AP_USER" ("login_name");

-- 工作空间。配置与作业的边界。
CREATE TABLE IF NOT EXISTS "AP_WORKSPACE" (
  "rec_id" VARCHAR(36) NOT NULL,
  "workspace_code" VARCHAR(128) NOT NULL,
  "workspace_name" VARCHAR(128) NOT NULL,
  "owner_user_id" VARCHAR(36) NOT NULL,
  "remark" VARCHAR(500),
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT PK_AP_WS PRIMARY KEY ("rec_id")
);

COMMENT ON TABLE "AP_WORKSPACE" IS '工作空间。配置与作业的边界。';
COMMENT ON COLUMN "AP_WORKSPACE"."rec_id" IS '主键 UUID';
COMMENT ON COLUMN "AP_WORKSPACE"."workspace_code" IS '空间编码，全局唯一';
COMMENT ON COLUMN "AP_WORKSPACE"."workspace_name" IS '显示名';
COMMENT ON COLUMN "AP_WORKSPACE"."owner_user_id" IS '负责人 user rec_id';
COMMENT ON COLUMN "AP_WORKSPACE"."remark" IS '备注';
COMMENT ON COLUMN "AP_WORKSPACE"."created_at" IS '创建时间';
COMMENT ON COLUMN "AP_WORKSPACE"."updated_at" IS '最后修改时间';

CREATE UNIQUE INDEX IF NOT EXISTS "UK_AP_WS_CODE" ON "AP_WORKSPACE" ("workspace_code");
CREATE INDEX IF NOT EXISTS "IDX_AP_WS_OWNER" ON "AP_WORKSPACE" ("owner_user_id");

-- 空间成员。③ 闸认这一行；角色 owner / operator / viewer。
CREATE TABLE IF NOT EXISTS "AP_WORKSPACE_MEMBER" (
  "rec_id" VARCHAR(36) NOT NULL,
  "workspace_rec_id" VARCHAR(36) NOT NULL,
  "user_rec_id" VARCHAR(36) NOT NULL,
  "member_role" VARCHAR(64) NOT NULL,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT PK_AP_WSM PRIMARY KEY ("rec_id")
);

COMMENT ON TABLE "AP_WORKSPACE_MEMBER" IS '空间成员。③ 闸认这一行；角色 owner / operator / viewer。';
COMMENT ON COLUMN "AP_WORKSPACE_MEMBER"."rec_id" IS '主键 UUID';
COMMENT ON COLUMN "AP_WORKSPACE_MEMBER"."workspace_rec_id" IS '空间';
COMMENT ON COLUMN "AP_WORKSPACE_MEMBER"."user_rec_id" IS '成员';
COMMENT ON COLUMN "AP_WORKSPACE_MEMBER"."member_role" IS 'owner / operator / viewer';
COMMENT ON COLUMN "AP_WORKSPACE_MEMBER"."created_at" IS '创建时间';
COMMENT ON COLUMN "AP_WORKSPACE_MEMBER"."updated_at" IS '最后修改时间';

CREATE UNIQUE INDEX IF NOT EXISTS "UK_AP_WSM" ON "AP_WORKSPACE_MEMBER" ("workspace_rec_id", "user_rec_id");
CREATE INDEX IF NOT EXISTS "IDX_AP_WSM_USER" ON "AP_WORKSPACE_MEMBER" ("user_rec_id");

-- 角色已授权动作。无行则拒绝。
CREATE TABLE IF NOT EXISTS "AP_ROLE_ACTION" (
  "rec_id" VARCHAR(36) NOT NULL,
  "role_code" VARCHAR(64) NOT NULL,
  "action_code" VARCHAR(64) NOT NULL,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT PK_AP_RA PRIMARY KEY ("rec_id")
);

COMMENT ON TABLE "AP_ROLE_ACTION" IS '角色已授权动作。无行则拒绝。';
COMMENT ON COLUMN "AP_ROLE_ACTION"."rec_id" IS '主键 UUID';
COMMENT ON COLUMN "AP_ROLE_ACTION"."role_code" IS 'owner / operator / viewer';
COMMENT ON COLUMN "AP_ROLE_ACTION"."action_code" IS 'job.submit 等';
COMMENT ON COLUMN "AP_ROLE_ACTION"."created_at" IS '创建时间';
COMMENT ON COLUMN "AP_ROLE_ACTION"."updated_at" IS '最后修改时间';

CREATE UNIQUE INDEX IF NOT EXISTS "UK_AP_RA" ON "AP_ROLE_ACTION" ("role_code", "action_code");

-- 工程配方。不直接跑命令，提交作业时读取。
CREATE TABLE IF NOT EXISTS "AP_PROJECT" (
  "rec_id" VARCHAR(36) NOT NULL,
  "workspace_rec_id" VARCHAR(36) NOT NULL,
  "project_name" VARCHAR(128) NOT NULL,
  "project_type" VARCHAR(64) NOT NULL,
  "repo_url" VARCHAR(1024),
  "repo_branch" VARCHAR(128),
  "work_rel_path" VARCHAR(1024),
  "recipe_json" TEXT NOT NULL,
  "cron_expr" VARCHAR(64),
  "is_enabled" SMALLINT NOT NULL DEFAULT 1,
  "remark" VARCHAR(500),
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT PK_AP_PRJ PRIMARY KEY ("rec_id")
);

COMMENT ON TABLE "AP_PROJECT" IS '工程配方。不直接跑命令，提交作业时读取。';
COMMENT ON COLUMN "AP_PROJECT"."rec_id" IS '主键 UUID';
COMMENT ON COLUMN "AP_PROJECT"."workspace_rec_id" IS '所属空间';
COMMENT ON COLUMN "AP_PROJECT"."project_name" IS '工程名';
COMMENT ON COLUMN "AP_PROJECT"."project_type" IS 'maven / npm / vue / go / custom';
COMMENT ON COLUMN "AP_PROJECT"."repo_url" IS 'Git 地址';
COMMENT ON COLUMN "AP_PROJECT"."repo_branch" IS '默认分支';
COMMENT ON COLUMN "AP_PROJECT"."work_rel_path" IS '空间根下相对目录';
COMMENT ON COLUMN "AP_PROJECT"."recipe_json" IS '构建配方 JSON';
COMMENT ON COLUMN "AP_PROJECT"."cron_expr" IS 'crontab，空则不调度';
COMMENT ON COLUMN "AP_PROJECT"."is_enabled" IS '1 启用';
COMMENT ON COLUMN "AP_PROJECT"."remark" IS '备注';
COMMENT ON COLUMN "AP_PROJECT"."created_at" IS '创建时间';
COMMENT ON COLUMN "AP_PROJECT"."updated_at" IS '最后修改时间';

CREATE UNIQUE INDEX IF NOT EXISTS "UK_AP_PRJ_NAME" ON "AP_PROJECT" ("workspace_rec_id", "project_name");
CREATE INDEX IF NOT EXISTS "IDX_AP_PRJ_WS" ON "AP_PROJECT" ("workspace_rec_id");

-- 环境凭据。密钥字段只存 APENC1 密文。
CREATE TABLE IF NOT EXISTS "AP_ENV" (
  "rec_id" VARCHAR(36) NOT NULL,
  "workspace_rec_id" VARCHAR(36) NOT NULL,
  "env_kind" VARCHAR(64) NOT NULL,
  "env_name" VARCHAR(128) NOT NULL,
  "config_cipher" TEXT NOT NULL,
  "config_hint" VARCHAR(256),
  "is_enabled" SMALLINT NOT NULL DEFAULT 1,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT PK_AP_ENV PRIMARY KEY ("rec_id")
);

COMMENT ON TABLE "AP_ENV" IS '环境凭据。密钥字段只存 APENC1 密文。';
COMMENT ON COLUMN "AP_ENV"."rec_id" IS '主键 UUID';
COMMENT ON COLUMN "AP_ENV"."workspace_rec_id" IS '所属空间';
COMMENT ON COLUMN "AP_ENV"."env_kind" IS 'git / ftp / smtp / ssh / jdk / maven / node';
COMMENT ON COLUMN "AP_ENV"."env_name" IS '显示名';
COMMENT ON COLUMN "AP_ENV"."config_cipher" IS '配置 JSON 密文';
COMMENT ON COLUMN "AP_ENV"."config_hint" IS '脱敏提示，可回给前端';
COMMENT ON COLUMN "AP_ENV"."is_enabled" IS '1 启用';
COMMENT ON COLUMN "AP_ENV"."created_at" IS '创建时间';
COMMENT ON COLUMN "AP_ENV"."updated_at" IS '最后修改时间';

CREATE INDEX IF NOT EXISTS "IDX_AP_ENV_WS" ON "AP_ENV" ("workspace_rec_id", "env_kind");

-- 一次可审计作业。状态 queued/running/succeeded/failed/canceled。
CREATE TABLE IF NOT EXISTS "AP_JOB" (
  "rec_id" VARCHAR(36) NOT NULL,
  "workspace_rec_id" VARCHAR(36) NOT NULL,
  "project_rec_id" VARCHAR(36),
  "job_type" VARCHAR(64) NOT NULL,
  "job_status" VARCHAR(64) NOT NULL,
  "payload_json" TEXT NOT NULL,
  "error_text" TEXT,
  "created_by" VARCHAR(36) NOT NULL,
  "started_at" TIMESTAMP,
  "finished_at" TIMESTAMP,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT PK_AP_JOB PRIMARY KEY ("rec_id")
);

COMMENT ON TABLE "AP_JOB" IS '一次可审计作业。状态 queued/running/succeeded/failed/canceled。';
COMMENT ON COLUMN "AP_JOB"."rec_id" IS '主键 UUID';
COMMENT ON COLUMN "AP_JOB"."workspace_rec_id" IS '空间';
COMMENT ON COLUMN "AP_JOB"."project_rec_id" IS '工程，可空';
COMMENT ON COLUMN "AP_JOB"."job_type" IS 'build / release / ftp_push / pom_version / branch_clone / sync_sources';
COMMENT ON COLUMN "AP_JOB"."job_status" IS 'queued / running / succeeded / failed / canceled';
COMMENT ON COLUMN "AP_JOB"."payload_json" IS '作业参数 JSON';
COMMENT ON COLUMN "AP_JOB"."error_text" IS '失败原因';
COMMENT ON COLUMN "AP_JOB"."created_by" IS '提交人';
COMMENT ON COLUMN "AP_JOB"."started_at" IS '开始时间';
COMMENT ON COLUMN "AP_JOB"."finished_at" IS '结束时间';
COMMENT ON COLUMN "AP_JOB"."created_at" IS '创建时间';
COMMENT ON COLUMN "AP_JOB"."updated_at" IS '最后修改时间';

CREATE INDEX IF NOT EXISTS "IDX_AP_JOB_WS" ON "AP_JOB" ("workspace_rec_id", "created_at");
CREATE INDEX IF NOT EXISTS "IDX_AP_JOB_PRJ" ON "AP_JOB" ("project_rec_id", "job_status");
CREATE INDEX IF NOT EXISTS "IDX_AP_JOB_ST" ON "AP_JOB" ("job_status");

-- 作业步骤。日志在磁盘 data/jobs/{jobId}/{step}.log。
CREATE TABLE IF NOT EXISTS "AP_JOB_STEP" (
  "rec_id" VARCHAR(36) NOT NULL,
  "job_rec_id" VARCHAR(36) NOT NULL,
  "step_key" VARCHAR(64) NOT NULL,
  "step_name" VARCHAR(128) NOT NULL,
  "sort_num" INTEGER NOT NULL DEFAULT 0,
  "step_status" VARCHAR(64) NOT NULL,
  "error_text" TEXT,
  "started_at" TIMESTAMP,
  "finished_at" TIMESTAMP,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT PK_AP_STEP PRIMARY KEY ("rec_id")
);

COMMENT ON TABLE "AP_JOB_STEP" IS '作业步骤。日志在磁盘 data/jobs/{jobId}/{step}.log。';
COMMENT ON COLUMN "AP_JOB_STEP"."rec_id" IS '主键 UUID';
COMMENT ON COLUMN "AP_JOB_STEP"."job_rec_id" IS '作业';
COMMENT ON COLUMN "AP_JOB_STEP"."step_key" IS 'clone / build / push 等';
COMMENT ON COLUMN "AP_JOB_STEP"."step_name" IS '显示名';
COMMENT ON COLUMN "AP_JOB_STEP"."sort_num" IS '顺序';
COMMENT ON COLUMN "AP_JOB_STEP"."step_status" IS 'pending / running / succeeded / failed / skipped / canceled';
COMMENT ON COLUMN "AP_JOB_STEP"."error_text" IS '失败原因';
COMMENT ON COLUMN "AP_JOB_STEP"."started_at" IS '开始时间';
COMMENT ON COLUMN "AP_JOB_STEP"."finished_at" IS '结束时间';
COMMENT ON COLUMN "AP_JOB_STEP"."created_at" IS '创建时间';
COMMENT ON COLUMN "AP_JOB_STEP"."updated_at" IS '最后修改时间';

CREATE INDEX IF NOT EXISTS "IDX_AP_STEP_JOB" ON "AP_JOB_STEP" ("job_rec_id", "sort_num");

-- 操作审计。
CREATE TABLE IF NOT EXISTS "AP_AUDIT_LOG" (
  "rec_id" VARCHAR(36) NOT NULL,
  "actor_user_id" VARCHAR(36) NOT NULL,
  "workspace_rec_id" VARCHAR(36),
  "action_code" VARCHAR(64) NOT NULL,
  "target_type" VARCHAR(64),
  "target_id" VARCHAR(36),
  "summary" VARCHAR(500) NOT NULL,
  "success_flag" SMALLINT NOT NULL DEFAULT 1,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT PK_AP_AUD PRIMARY KEY ("rec_id")
);

COMMENT ON TABLE "AP_AUDIT_LOG" IS '操作审计。';
COMMENT ON COLUMN "AP_AUDIT_LOG"."rec_id" IS '主键 UUID';
COMMENT ON COLUMN "AP_AUDIT_LOG"."actor_user_id" IS '操作人';
COMMENT ON COLUMN "AP_AUDIT_LOG"."workspace_rec_id" IS '空间，可空';
COMMENT ON COLUMN "AP_AUDIT_LOG"."action_code" IS '动作';
COMMENT ON COLUMN "AP_AUDIT_LOG"."target_type" IS '目标类型';
COMMENT ON COLUMN "AP_AUDIT_LOG"."target_id" IS '目标主键';
COMMENT ON COLUMN "AP_AUDIT_LOG"."summary" IS '摘要';
COMMENT ON COLUMN "AP_AUDIT_LOG"."success_flag" IS '1 成功';
COMMENT ON COLUMN "AP_AUDIT_LOG"."created_at" IS '创建时间';
COMMENT ON COLUMN "AP_AUDIT_LOG"."updated_at" IS '最后修改时间';

CREATE INDEX IF NOT EXISTS "IDX_AP_AUD_WS" ON "AP_AUDIT_LOG" ("workspace_rec_id", "created_at");
CREATE INDEX IF NOT EXISTS "IDX_AP_AUD_ACTOR" ON "AP_AUDIT_LOG" ("actor_user_id");

-- 平台系统属性。key/value，供作业并发、日志保留、请求超时等。
CREATE TABLE IF NOT EXISTS "AP_SETTING" (
  "rec_id" VARCHAR(36) NOT NULL,
  "setting_key" VARCHAR(64) NOT NULL,
  "setting_value" VARCHAR(256) NOT NULL,
  "remark" VARCHAR(500),
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT PK_AP_SET PRIMARY KEY ("rec_id")
);

COMMENT ON TABLE "AP_SETTING" IS '平台系统属性。key/value，供作业并发、日志保留、请求超时等。';
COMMENT ON COLUMN "AP_SETTING"."rec_id" IS '主键 UUID';
COMMENT ON COLUMN "AP_SETTING"."setting_key" IS 'job.concurrency / log.retention_days / web.timeout_sec';
COMMENT ON COLUMN "AP_SETTING"."setting_value" IS '属性值';
COMMENT ON COLUMN "AP_SETTING"."remark" IS '备注';
COMMENT ON COLUMN "AP_SETTING"."created_at" IS '创建时间';
COMMENT ON COLUMN "AP_SETTING"."updated_at" IS '最后修改时间';

CREATE UNIQUE INDEX IF NOT EXISTS "UK_AP_SET_KEY" ON "AP_SETTING" ("setting_key");

-- 全局环境变量。构建进程可叠加；禁止覆盖 PATH 等保留键。
CREATE TABLE IF NOT EXISTS "AP_GLOBAL_VAR" (
  "rec_id" VARCHAR(36) NOT NULL,
  "var_key" VARCHAR(64) NOT NULL,
  "var_value" TEXT NOT NULL,
  "is_enabled" SMALLINT NOT NULL DEFAULT 1,
  "remark" VARCHAR(500),
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT PK_AP_GVAR PRIMARY KEY ("rec_id")
);

COMMENT ON TABLE "AP_GLOBAL_VAR" IS '全局环境变量。构建进程可叠加；禁止覆盖 PATH 等保留键。';
COMMENT ON COLUMN "AP_GLOBAL_VAR"."rec_id" IS '主键 UUID';
COMMENT ON COLUMN "AP_GLOBAL_VAR"."var_key" IS '变量名';
COMMENT ON COLUMN "AP_GLOBAL_VAR"."var_value" IS '变量值';
COMMENT ON COLUMN "AP_GLOBAL_VAR"."is_enabled" IS '1 启用';
COMMENT ON COLUMN "AP_GLOBAL_VAR"."remark" IS '备注';
COMMENT ON COLUMN "AP_GLOBAL_VAR"."created_at" IS '创建时间';
COMMENT ON COLUMN "AP_GLOBAL_VAR"."updated_at" IS '最后修改时间';

CREATE UNIQUE INDEX IF NOT EXISTS "UK_AP_GVAR_KEY" ON "AP_GLOBAL_VAR" ("var_key");

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
