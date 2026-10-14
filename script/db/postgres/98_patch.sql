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

-- 外部平台打包通知。按 task_id 去重，不参与作业步骤。
CREATE TABLE IF NOT EXISTS "AP_INTAKE_TASK" (
  "task_id" VARCHAR(64) NOT NULL,
  "task_no" VARCHAR(64),
  "pack_task_id" VARCHAR(128),
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
  CONSTRAINT PK_AP_INTAKE PRIMARY KEY ("task_id")
);
