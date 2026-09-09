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
