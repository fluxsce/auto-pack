-- auto-pack 平台库 DDL（mysql）
-- MySQL / MariaDB 平台库。字符集跟随数据库，脚本不写 CHARSET / COLLATE。
-- 改字段请改 script/db/gen_ddl.py 后重新生成。
-- 禁止 FOREIGN KEY / REFERENCES。密钥只存密文。

-- CREATE DATABASE auto_pack;  -- 字符集用库默认，不要在表上写 CHARSET

-- 登录用户。口令只存 bcrypt 哈希。
CREATE TABLE IF NOT EXISTS `AP_USER` (
  `rec_id` VARCHAR(36) NOT NULL COMMENT '主键 UUID',
  `login_name` VARCHAR(128) NOT NULL COMMENT '登录名，全局唯一',
  `display_name` VARCHAR(128) NOT NULL COMMENT '显示名',
  `password_hash` VARCHAR(256) NOT NULL COMMENT 'bcrypt 哈希',
  `is_enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '1 启用 / 0 停用',
  `remark` VARCHAR(500) COMMENT '备注',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最后修改时间',
  CONSTRAINT PK_AP_USER PRIMARY KEY (`rec_id`),
  UNIQUE KEY `UK_AP_USER_LOGIN` (`login_name`)
) ENGINE=InnoDB COMMENT='登录用户。口令只存 bcrypt 哈希。';


-- 工作空间。配置与作业的边界。
CREATE TABLE IF NOT EXISTS `AP_WORKSPACE` (
  `rec_id` VARCHAR(36) NOT NULL COMMENT '主键 UUID',
  `workspace_code` VARCHAR(128) NOT NULL COMMENT '空间编码，全局唯一',
  `workspace_name` VARCHAR(128) NOT NULL COMMENT '显示名',
  `owner_user_id` VARCHAR(36) NOT NULL COMMENT '负责人 user rec_id',
  `remark` VARCHAR(500) COMMENT '备注',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最后修改时间',
  CONSTRAINT PK_AP_WS PRIMARY KEY (`rec_id`),
  UNIQUE KEY `UK_AP_WS_CODE` (`workspace_code`),
  KEY `IDX_AP_WS_OWNER` (`owner_user_id`)
) ENGINE=InnoDB COMMENT='工作空间。配置与作业的边界。';


-- 空间成员。③ 闸认这一行；角色 owner / operator / viewer。
CREATE TABLE IF NOT EXISTS `AP_WORKSPACE_MEMBER` (
  `rec_id` VARCHAR(36) NOT NULL COMMENT '主键 UUID',
  `workspace_rec_id` VARCHAR(36) NOT NULL COMMENT '空间',
  `user_rec_id` VARCHAR(36) NOT NULL COMMENT '成员',
  `member_role` VARCHAR(64) NOT NULL COMMENT 'owner / operator / viewer',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最后修改时间',
  CONSTRAINT PK_AP_WSM PRIMARY KEY (`rec_id`),
  UNIQUE KEY `UK_AP_WSM` (`workspace_rec_id`, `user_rec_id`),
  KEY `IDX_AP_WSM_USER` (`user_rec_id`)
) ENGINE=InnoDB COMMENT='空间成员。③ 闸认这一行；角色 owner / operator / viewer。';


-- 角色已授权动作。无行则拒绝。
CREATE TABLE IF NOT EXISTS `AP_ROLE_ACTION` (
  `rec_id` VARCHAR(36) NOT NULL COMMENT '主键 UUID',
  `role_code` VARCHAR(64) NOT NULL COMMENT 'owner / operator / viewer',
  `action_code` VARCHAR(64) NOT NULL COMMENT 'job.submit 等',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最后修改时间',
  CONSTRAINT PK_AP_RA PRIMARY KEY (`rec_id`),
  UNIQUE KEY `UK_AP_RA` (`role_code`, `action_code`)
) ENGINE=InnoDB COMMENT='角色已授权动作。无行则拒绝。';


-- 工程配方。不直接跑命令，提交作业时读取。
CREATE TABLE IF NOT EXISTS `AP_PROJECT` (
  `rec_id` VARCHAR(36) NOT NULL COMMENT '主键 UUID',
  `workspace_rec_id` VARCHAR(36) NOT NULL COMMENT '所属空间',
  `project_name` VARCHAR(128) NOT NULL COMMENT '工程名',
  `project_type` VARCHAR(64) NOT NULL COMMENT 'maven / npm / vue / go / custom',
  `repo_url` VARCHAR(1024) COMMENT 'Git 地址',
  `repo_branch` VARCHAR(128) COMMENT '默认分支',
  `work_rel_path` VARCHAR(1024) COMMENT '空间根下相对目录',
  `recipe_json` LONGTEXT NOT NULL COMMENT '构建配方 JSON',
  `cron_expr` VARCHAR(64) COMMENT 'crontab，空则不调度',
  `is_enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '1 启用',
  `remark` VARCHAR(500) COMMENT '备注',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最后修改时间',
  CONSTRAINT PK_AP_PRJ PRIMARY KEY (`rec_id`),
  UNIQUE KEY `UK_AP_PRJ_NAME` (`workspace_rec_id`, `project_name`),
  KEY `IDX_AP_PRJ_WS` (`workspace_rec_id`)
) ENGINE=InnoDB COMMENT='工程配方。不直接跑命令，提交作业时读取。';


-- 环境凭据。密钥字段只存 APENC1 密文。
CREATE TABLE IF NOT EXISTS `AP_ENV` (
  `rec_id` VARCHAR(36) NOT NULL COMMENT '主键 UUID',
  `workspace_rec_id` VARCHAR(36) NOT NULL COMMENT '历史列，SDK 环境为全局，写入空串',
  `env_kind` VARCHAR(64) NOT NULL COMMENT 'git / ftp / smtp / ssh / jdk / maven / node',
  `env_name` VARCHAR(128) NOT NULL COMMENT '显示名',
  `config_cipher` LONGTEXT NOT NULL COMMENT '配置 JSON 密文',
  `config_hint` VARCHAR(256) COMMENT '脱敏提示，可回给前端',
  `is_enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '1 启用',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最后修改时间',
  CONSTRAINT PK_AP_ENV PRIMARY KEY (`rec_id`),
  KEY `IDX_AP_ENV_WS` (`workspace_rec_id`, `env_kind`)
) ENGINE=InnoDB COMMENT='环境凭据。密钥字段只存 APENC1 密文。';


-- 一次可审计作业。状态 queued/running/succeeded/failed/canceled。
CREATE TABLE IF NOT EXISTS `AP_JOB` (
  `rec_id` VARCHAR(36) NOT NULL COMMENT '主键 UUID',
  `workspace_rec_id` VARCHAR(36) NOT NULL COMMENT '空间',
  `project_rec_id` VARCHAR(36) COMMENT '工程，可空',
  `job_type` VARCHAR(64) NOT NULL COMMENT 'build / release / ftp_push / pom_version / branch_clone / sync_sources',
  `job_status` VARCHAR(64) NOT NULL COMMENT 'queued / running / succeeded / failed / canceled',
  `payload_json` LONGTEXT NOT NULL COMMENT '作业参数 JSON',
  `error_text` LONGTEXT COMMENT '失败原因',
  `created_by` VARCHAR(36) NOT NULL COMMENT '提交人',
  `started_at` DATETIME COMMENT '开始时间',
  `finished_at` DATETIME COMMENT '结束时间',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最后修改时间',
  CONSTRAINT PK_AP_JOB PRIMARY KEY (`rec_id`),
  KEY `IDX_AP_JOB_WS` (`workspace_rec_id`, `created_at`),
  KEY `IDX_AP_JOB_PRJ` (`project_rec_id`, `job_status`),
  KEY `IDX_AP_JOB_ST` (`job_status`)
) ENGINE=InnoDB COMMENT='一次可审计作业。状态 queued/running/succeeded/failed/canceled。';


-- 作业步骤。日志在磁盘 data/jobs/{jobId}/{step}.log。
CREATE TABLE IF NOT EXISTS `AP_JOB_STEP` (
  `rec_id` VARCHAR(36) NOT NULL COMMENT '主键 UUID',
  `job_rec_id` VARCHAR(36) NOT NULL COMMENT '作业',
  `step_key` VARCHAR(64) NOT NULL COMMENT 'clone / build / push 等',
  `step_name` VARCHAR(128) NOT NULL COMMENT '显示名',
  `sort_num` INT NOT NULL DEFAULT 0 COMMENT '顺序',
  `step_status` VARCHAR(64) NOT NULL COMMENT 'pending / running / succeeded / failed / skipped / canceled',
  `error_text` LONGTEXT COMMENT '失败原因',
  `started_at` DATETIME COMMENT '开始时间',
  `finished_at` DATETIME COMMENT '结束时间',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最后修改时间',
  CONSTRAINT PK_AP_STEP PRIMARY KEY (`rec_id`),
  KEY `IDX_AP_STEP_JOB` (`job_rec_id`, `sort_num`)
) ENGINE=InnoDB COMMENT='作业步骤。日志在磁盘 data/jobs/{jobId}/{step}.log。';


-- 操作审计。
CREATE TABLE IF NOT EXISTS `AP_AUDIT_LOG` (
  `rec_id` VARCHAR(36) NOT NULL COMMENT '主键 UUID',
  `actor_user_id` VARCHAR(36) NOT NULL COMMENT '操作人',
  `workspace_rec_id` VARCHAR(36) COMMENT '空间，可空',
  `action_code` VARCHAR(64) NOT NULL COMMENT '动作',
  `target_type` VARCHAR(64) COMMENT '目标类型',
  `target_id` VARCHAR(36) COMMENT '目标主键',
  `summary` VARCHAR(500) NOT NULL COMMENT '摘要',
  `success_flag` TINYINT NOT NULL DEFAULT 1 COMMENT '1 成功',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最后修改时间',
  CONSTRAINT PK_AP_AUD PRIMARY KEY (`rec_id`),
  KEY `IDX_AP_AUD_WS` (`workspace_rec_id`, `created_at`),
  KEY `IDX_AP_AUD_ACTOR` (`actor_user_id`)
) ENGINE=InnoDB COMMENT='操作审计。';


-- 平台系统属性。key/value，供作业并发、日志保留、请求超时等。
CREATE TABLE IF NOT EXISTS `AP_SETTING` (
  `rec_id` VARCHAR(36) NOT NULL COMMENT '主键 UUID',
  `setting_key` VARCHAR(64) NOT NULL COMMENT 'job.concurrency / log.retention_days / web.timeout_sec',
  `setting_value` VARCHAR(256) NOT NULL COMMENT '属性值',
  `remark` VARCHAR(500) COMMENT '备注',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最后修改时间',
  CONSTRAINT PK_AP_SET PRIMARY KEY (`rec_id`),
  UNIQUE KEY `UK_AP_SET_KEY` (`setting_key`)
) ENGINE=InnoDB COMMENT='平台系统属性。key/value，供作业并发、日志保留、请求超时等。';


-- 全局环境变量。构建进程可叠加；禁止覆盖 PATH 等保留键。
CREATE TABLE IF NOT EXISTS `AP_GLOBAL_VAR` (
  `rec_id` VARCHAR(36) NOT NULL COMMENT '主键 UUID',
  `var_key` VARCHAR(64) NOT NULL COMMENT '变量名',
  `var_value` LONGTEXT NOT NULL COMMENT '变量值',
  `is_enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '1 启用',
  `remark` VARCHAR(500) COMMENT '备注',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最后修改时间',
  CONSTRAINT PK_AP_GVAR PRIMARY KEY (`rec_id`),
  UNIQUE KEY `UK_AP_GVAR_KEY` (`var_key`)
) ENGINE=InnoDB COMMENT='全局环境变量。构建进程可叠加；禁止覆盖 PATH 等保留键。';


-- 预置角色动作。用户由应用层 seed。
INSERT IGNORE INTO `AP_ROLE_ACTION` (`rec_id`, `role_code`, `action_code`, `created_at`, `updated_at`) VALUES
    ('ra-owner-js', 'owner', 'job.submit', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('ra-owner-jc', 'owner', 'job.cancel', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('ra-owner-pw', 'owner', 'project.write', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('ra-owner-ew', 'owner', 'env.write', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('ra-owner-ua', 'owner', 'user.admin', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('ra-owner-sw', 'owner', 'settings.write', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('ra-owner-ar', 'owner', 'audit.read', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('ra-op-js', 'operator', 'job.submit', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('ra-op-jc', 'operator', 'job.cancel', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('ra-op-pw', 'operator', 'project.write', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('ra-op-ew', 'operator', 'env.write', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('ra-op-ar', 'operator', 'audit.read', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('ra-vw-ar', 'viewer', 'audit.read', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
