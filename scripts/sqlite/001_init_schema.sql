-- auto-pack SQLite 初始化脚本（认证最小集）
-- 说明：现阶段仅保留用户认证表，用于前端登录。

BEGIN;

-- 用户认证表：登录凭据与启停状态
CREATE TABLE IF NOT EXISTS AP_USER_AUTH (
  rec_id TEXT NOT NULL PRIMARY KEY,                           -- 用户主键（UUID v4）
  login_name TEXT NOT NULL DEFAULT '',                        -- 登录用户名（业务唯一）
  password_hash TEXT NOT NULL DEFAULT '',                     -- bcrypt 密码哈希
  is_enabled INTEGER NOT NULL DEFAULT 1 CHECK (is_enabled IN (0,1)), -- 启停状态（1启用/0禁用）
  remark TEXT NOT NULL DEFAULT '',                            -- 备注说明
  created_at TEXT NOT NULL DEFAULT (datetime('now')),         -- 创建时间
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))          -- 更新时间
);

-- 用户名唯一索引
CREATE UNIQUE INDEX IF NOT EXISTS IX_AP_USER_LOGIN
  ON AP_USER_AUTH(login_name);

COMMIT;
