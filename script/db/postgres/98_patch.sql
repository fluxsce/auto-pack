-- auto-pack 平台库 DDL（postgres）
-- PostgreSQL 平台库。
-- 改字段请改 script/db/gen_ddl.py 后重新生成。
-- 禁止 FOREIGN KEY / REFERENCES。密钥只存密文。

-- 已有表补列 / 补索引。可重复执行（已存在则跳过）。
-- 启动时在 init.sql 之后执行。
CREATE TABLE IF NOT EXISTS "AP_WORKSPACE_RELEASE" (
  "rec_id" VARCHAR(36) NOT NULL,
  "workspace_rec_id" VARCHAR(36) NOT NULL,
  "release_num" INTEGER NOT NULL DEFAULT 0,
  "status" VARCHAR(64) NOT NULL,
  "release_branch" VARCHAR(128),
  "tag_branch" VARCHAR(128),
  "target_branch" VARCHAR(128),
  "released" INTEGER NOT NULL DEFAULT 0,
  "skipped" INTEGER NOT NULL DEFAULT 0,
  "failed" INTEGER NOT NULL DEFAULT 0,
  "message" VARCHAR(500),
  "log_path" VARCHAR(1024),
  "post_options_json" TEXT,
  "started_at" TIMESTAMP,
  "finished_at" TIMESTAMP,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT PK_AP_WSREL PRIMARY KEY ("rec_id")
);
CREATE INDEX IF NOT EXISTS "IDX_AP_WSREL_WS" ON "AP_WORKSPACE_RELEASE" ("workspace_rec_id", "release_num");

CREATE TABLE IF NOT EXISTS "AP_WORKSPACE_BUILD" (
  "rec_id" VARCHAR(36) NOT NULL,
  "workspace_rec_id" VARCHAR(36) NOT NULL,
  "build_num" INTEGER NOT NULL DEFAULT 0,
  "status" VARCHAR(64) NOT NULL,
  "scope" VARCHAR(64),
  "succeeded" INTEGER NOT NULL DEFAULT 0,
  "skipped" INTEGER NOT NULL DEFAULT 0,
  "failed" INTEGER NOT NULL DEFAULT 0,
  "message" VARCHAR(500),
  "log_path" VARCHAR(1024),
  "filter_json" TEXT,
  "started_at" TIMESTAMP,
  "finished_at" TIMESTAMP,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT PK_AP_WSBLD PRIMARY KEY ("rec_id")
);
CREATE INDEX IF NOT EXISTS "IDX_AP_WSBLD_WS" ON "AP_WORKSPACE_BUILD" ("workspace_rec_id", "build_num");

UPDATE "AP_ENV" SET "workspace_rec_id" = '' WHERE "workspace_rec_id" <> '';

-- 外部平台打包通知。每条投递一行，主键是 rec_id。同一 pack_task_id 再次投递会再打包。不参与作业步骤。
CREATE TABLE IF NOT EXISTS "AP_INTAKE_TASK" (
  "rec_id" VARCHAR(36) NOT NULL,
  "task_id" VARCHAR(64) NOT NULL,
  "task_no" VARCHAR(64),
  "pack_task_id" VARCHAR(128) NOT NULL,
  "subsystem_type" VARCHAR(32),
  "project_rec_id" VARCHAR(36),
  "job_rec_id" VARCHAR(36),
  "task_status" VARCHAR(32) NOT NULL,
  "result_status" VARCHAR(16),
  "result_message" VARCHAR(500),
  "oss_project_root" VARCHAR(1024),
  "notice_json" TEXT,
  "finished_at" TIMESTAMP,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT PK_AP_INTAKE PRIMARY KEY ("rec_id")
);
CREATE INDEX IF NOT EXISTS "IDX_AP_INTAKE_TASK" ON "AP_INTAKE_TASK" ("task_id");
CREATE INDEX IF NOT EXISTS "IDX_AP_INTAKE_PACK" ON "AP_INTAKE_TASK" ("pack_task_id");

CREATE TABLE IF NOT EXISTS "AP_MQ_MESSAGE" (
  "rec_id" VARCHAR(36) NOT NULL,
  "env_rec_id" VARCHAR(36) NOT NULL,
  "seq_num" INTEGER NOT NULL,
  "direction" VARCHAR(64) NOT NULL,
  "queue_role" VARCHAR(64) NOT NULL,
  "queue_name" VARCHAR(128) NOT NULL,
  "message_id" VARCHAR(128),
  "task_id" VARCHAR(64),
  "body" TEXT,
  "source" VARCHAR(64) NOT NULL,
  "deleted_flag" SMALLINT NOT NULL DEFAULT 0,
  "actor_user_id" VARCHAR(36) NOT NULL,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT PK_AP_MQMSG PRIMARY KEY ("rec_id")
);
CREATE UNIQUE INDEX IF NOT EXISTS "UK_AP_MQMSG_SEQ" ON "AP_MQ_MESSAGE" ("seq_num");
ALTER TABLE "AP_MQ_MESSAGE" ADD COLUMN "env_rec_id" VARCHAR(36);
CREATE INDEX IF NOT EXISTS "IDX_AP_MQMSG_ENV" ON "AP_MQ_MESSAGE" ("env_rec_id", "seq_num");
