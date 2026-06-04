-- 打包工程表：Git 仓库、构建类型、关联环境配置

BEGIN;

CREATE TABLE IF NOT EXISTS AP_PACK_PROJECT (
  rec_id TEXT NOT NULL PRIMARY KEY,                           -- 工程主键（UUID v4）
  workspace_rec_id TEXT NOT NULL DEFAULT '',                    -- 所属工作空间 rec_id
  project_name TEXT NOT NULL DEFAULT '',                        -- 工程名称（同空间+版本下唯一）
  project_version TEXT NOT NULL DEFAULT '1.0.0',                -- 工程版本
  project_type TEXT NOT NULL DEFAULT 'custom',                  -- 工程类型：maven|npm|vue|go|gradle|custom
  repo_url TEXT NOT NULL DEFAULT '',                            -- Git 仓库地址（可为空表示仅本地目录）
  repo_branch TEXT NOT NULL DEFAULT 'main',                     -- 默认分支或标签
  work_dir TEXT NOT NULL DEFAULT '',                            -- 工作空间下相对目录（服务端由名称推导）
  config_json TEXT NOT NULL DEFAULT '{}',                       -- 构建扩展 JSON（命令、产物路径等）
  git_env_rec_id TEXT NOT NULL DEFAULT '',                      -- 关联 Git 环境 rec_id（逻辑外键）
  jdk_env_rec_id TEXT NOT NULL DEFAULT '',                      -- 关联 JDK 环境 rec_id
  maven_env_rec_id TEXT NOT NULL DEFAULT '',                    -- 关联 Maven 环境 rec_id
  nodejs_env_rec_id TEXT NOT NULL DEFAULT '',                   -- 关联 Node.js 环境 rec_id
  deploy_env_rec_id TEXT NOT NULL DEFAULT '',                   -- 关联部署环境 rec_id（ssh|sftp|ftp）
  is_enabled INTEGER NOT NULL DEFAULT 1 CHECK (is_enabled IN (0,1)), -- 启停（1启用/0禁用）
  sort_num INTEGER NOT NULL DEFAULT 0 CHECK (sort_num >= 0),    -- 排序权重（越小越靠前）
  remark TEXT NOT NULL DEFAULT '',                              -- 备注说明
  created_at TEXT NOT NULL DEFAULT (datetime('now')),           -- 创建时间
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))            -- 更新时间
);

CREATE UNIQUE INDEX IF NOT EXISTS IX_AP_PACK_PROJ_WS_VER
  ON AP_PACK_PROJECT(workspace_rec_id, project_name, project_version);

CREATE INDEX IF NOT EXISTS IX_AP_PACK_PROJ_WS
  ON AP_PACK_PROJECT(workspace_rec_id);

CREATE INDEX IF NOT EXISTS IX_AP_PACK_PROJ_TYPE
  ON AP_PACK_PROJECT(project_type);

COMMIT;
