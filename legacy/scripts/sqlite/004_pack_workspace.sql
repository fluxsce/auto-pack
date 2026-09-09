-- 工作空间表（工程版本/空间列由 pkg/db 幂等迁移补齐）

BEGIN;

CREATE TABLE IF NOT EXISTS AP_PACK_WORKSPACE (
  rec_id TEXT NOT NULL PRIMARY KEY,                           -- 工作空间主键（UUID v4）
  workspace_code TEXT NOT NULL DEFAULT '',                      -- 空间编码（全局唯一）
  workspace_name TEXT NOT NULL DEFAULT '',                      -- 空间名称（界面展示）
  root_dir TEXT NOT NULL DEFAULT '',                            -- 工作空间根目录
  is_default INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0,1)), -- 默认空间
  is_enabled INTEGER NOT NULL DEFAULT 1 CHECK (is_enabled IN (0,1)), -- 启停
  sort_num INTEGER NOT NULL DEFAULT 0 CHECK (sort_num >= 0),    -- 排序
  remark TEXT NOT NULL DEFAULT '',                              -- 备注
  created_at TEXT NOT NULL DEFAULT (datetime('now')),           -- 创建时间
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))            -- 更新时间
);

CREATE UNIQUE INDEX IF NOT EXISTS IX_AP_PACK_WS_CODE
  ON AP_PACK_WORKSPACE(workspace_code);

INSERT OR IGNORE INTO AP_PACK_WORKSPACE(
  rec_id, workspace_code, workspace_name, root_dir, is_default, is_enabled, sort_num, remark)
VALUES(
  '00000000-0000-4000-8000-000000000010',
  'default',
  '默认工作空间',
  '',
  1,
  1,
  0,
  '系统初始化'
);

COMMIT;
