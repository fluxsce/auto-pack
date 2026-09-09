-- auto-pack 平台库 DDL（mysql）
-- MySQL / MariaDB 平台库。字符集跟随数据库，脚本不写 CHARSET / COLLATE。
-- 改字段请改 script/db/gen_ddl.py 后重新生成。
-- 禁止 FOREIGN KEY / REFERENCES。密钥只存密文。

-- 已有表补列 / 补索引。可重复执行（已存在则跳过）。
-- 启动时在 init.sql 之后执行。
CREATE UNIQUE INDEX `UK_AP_USER_LOGIN` ON `AP_USER` (`login_name`);
CREATE UNIQUE INDEX `UK_AP_WS_CODE` ON `AP_WORKSPACE` (`workspace_code`);
CREATE INDEX `IDX_AP_WS_OWNER` ON `AP_WORKSPACE` (`owner_user_id`);
CREATE UNIQUE INDEX `UK_AP_WSM` ON `AP_WORKSPACE_MEMBER` (`workspace_rec_id`, `user_rec_id`);
CREATE INDEX `IDX_AP_WSM_USER` ON `AP_WORKSPACE_MEMBER` (`user_rec_id`);
CREATE UNIQUE INDEX `UK_AP_RA` ON `AP_ROLE_ACTION` (`role_code`, `action_code`);
CREATE UNIQUE INDEX `UK_AP_PRJ_NAME` ON `AP_PROJECT` (`workspace_rec_id`, `project_name`);
CREATE INDEX `IDX_AP_PRJ_WS` ON `AP_PROJECT` (`workspace_rec_id`);
CREATE INDEX `IDX_AP_ENV_WS` ON `AP_ENV` (`workspace_rec_id`, `env_kind`);
CREATE INDEX `IDX_AP_JOB_WS` ON `AP_JOB` (`workspace_rec_id`, `created_at`);
CREATE INDEX `IDX_AP_JOB_PRJ` ON `AP_JOB` (`project_rec_id`, `job_status`);
CREATE INDEX `IDX_AP_JOB_ST` ON `AP_JOB` (`job_status`);
CREATE INDEX `IDX_AP_STEP_JOB` ON `AP_JOB_STEP` (`job_rec_id`, `sort_num`);
CREATE INDEX `IDX_AP_AUD_WS` ON `AP_AUDIT_LOG` (`workspace_rec_id`, `created_at`);
CREATE INDEX `IDX_AP_AUD_ACTOR` ON `AP_AUDIT_LOG` (`actor_user_id`);
CREATE UNIQUE INDEX `UK_AP_SET_KEY` ON `AP_SETTING` (`setting_key`);
CREATE UNIQUE INDEX `UK_AP_GVAR_KEY` ON `AP_GLOBAL_VAR` (`var_key`);

CREATE TABLE IF NOT EXISTS `AP_WORKSPACE_RELEASE` (
  `rec_id` VARCHAR(36) NOT NULL,
  `workspace_rec_id` VARCHAR(36) NOT NULL,
  `release_num` INT NOT NULL DEFAULT 0,
  `status` VARCHAR(64) NOT NULL,
  `release_branch` VARCHAR(128),
  `tag_branch` VARCHAR(128),
  `target_branch` VARCHAR(128),
  `released` INT NOT NULL DEFAULT 0,
  `skipped` INT NOT NULL DEFAULT 0,
  `failed` INT NOT NULL DEFAULT 0,
  `message` VARCHAR(500),
  `log_path` VARCHAR(1024),
  `post_options_json` LONGTEXT,
  `started_at` DATETIME,
  `finished_at` DATETIME,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT PK_AP_WSREL PRIMARY KEY (`rec_id`)
);
CREATE INDEX `IDX_AP_WSREL_WS` ON `AP_WORKSPACE_RELEASE` (`workspace_rec_id`, `release_num`);
