-- 系统环境配置表：SSH / SFTP / FTP / Git / Maven / Node.js / JDK 等工具环境

BEGIN;

CREATE TABLE IF NOT EXISTS AP_SYS_ENV (
  rec_id TEXT NOT NULL PRIMARY KEY,                           -- 配置主键（UUID v4）
  env_name TEXT NOT NULL DEFAULT '',                          -- 配置名称（界面展示，全局唯一）
  env_type TEXT NOT NULL DEFAULT '',                          -- 环境类型：ssh|sftp|ftp|git|maven|nodejs|jdk
  config_json TEXT NOT NULL DEFAULT '{}',                     -- 类型相关 JSON 配置（路径、主机、凭据等）
  is_default INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0,1)), -- 同类型默认项（每类至多一个默认）
  is_enabled INTEGER NOT NULL DEFAULT 1 CHECK (is_enabled IN (0,1)), -- 启停（1启用/0禁用）
  sort_num INTEGER NOT NULL DEFAULT 0 CHECK (sort_num >= 0),  -- 排序权重（越小越靠前）
  remark TEXT NOT NULL DEFAULT '',                            -- 备注说明
  created_at TEXT NOT NULL DEFAULT (datetime('now')),         -- 创建时间
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))          -- 更新时间
);

CREATE UNIQUE INDEX IF NOT EXISTS IX_AP_SYS_ENV_NAME
  ON AP_SYS_ENV(env_name);

CREATE INDEX IF NOT EXISTS IX_AP_SYS_ENV_TYPE
  ON AP_SYS_ENV(env_type);

COMMIT;
