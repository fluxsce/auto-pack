-- 工程构建记录：触发打包、日志路径与状态

BEGIN;

CREATE TABLE IF NOT EXISTS AP_PACK_BUILD (
  rec_id TEXT NOT NULL PRIMARY KEY,
  project_rec_id TEXT NOT NULL,
  build_num INTEGER NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  message TEXT NOT NULL DEFAULT '',
  log_path TEXT NOT NULL DEFAULT '',
  started_at TEXT,
  finished_at TEXT,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS IX_AP_PACK_BUILD_NUM
  ON AP_PACK_BUILD(project_rec_id, build_num);

CREATE INDEX IF NOT EXISTS IX_AP_PACK_BUILD_PROJECT
  ON AP_PACK_BUILD(project_rec_id, created_at DESC);

COMMIT;
