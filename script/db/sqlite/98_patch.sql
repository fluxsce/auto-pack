-- auto-pack 平台库 DDL（sqlite）
-- SQLite 平台库。默认离线。
-- 改字段请改 script/db/gen_ddl.py 后重新生成。
-- 禁止 FOREIGN KEY / REFERENCES。密钥只存密文。

-- 已有表补列 / 补索引。可重复执行（已存在则跳过）。
-- 启动时在 init.sql 之后执行。
-- 工作空间自动发版记录。
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

UPDATE "AP_ENV" SET "workspace_rec_id" = '' WHERE "workspace_rec_id" <> '';

-- 外部平台打包通知。按 task_id 去重，不参与作业步骤。
CREATE TABLE IF NOT EXISTS "AP_INTAKE_TASK" (
  "task_id" TEXT NOT NULL,
  "task_no" TEXT,
  "pack_task_id" TEXT,
  "subsystem_type" TEXT,
  "project_rec_id" TEXT,
  "job_rec_id" TEXT,
  "task_status" TEXT NOT NULL,
  "result_status" TEXT,
  "result_message" TEXT,
  "oss_project_root" TEXT,
  "notice_json" TEXT,
  "finished_at" TEXT,
  "created_at" TEXT NOT NULL DEFAULT (datetime('now')),
  "updated_at" TEXT NOT NULL DEFAULT (datetime('now')),
  CONSTRAINT PK_AP_INTAKE PRIMARY KEY ("task_id")
);

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
ALTER TABLE "AP_MQ_MESSAGE" ADD COLUMN "env_rec_id" TEXT;
CREATE INDEX IF NOT EXISTS "IDX_AP_MQMSG_ENV" ON "AP_MQ_MESSAGE" ("env_rec_id", "seq_num");
