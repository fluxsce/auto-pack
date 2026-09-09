# -*- coding: utf-8 -*-
"""Generate AP_* DDL for sqlite / mysql / postgres. Run: py -3 -X utf8 script/db/gen_ddl.py"""

from __future__ import annotations

from pathlib import Path

ROOT = Path(__file__).resolve().parent

COMMON: list[tuple] = [
    ("created_at", "datetime", False, "NOW", "创建时间"),
    ("updated_at", "datetime", False, "NOW", "最后修改时间"),
]

# table, pk, comment, cols, indexes
# cols: (name, type_key, nullable, default, comment)
# indexes: (name, cols, unique?)
TABLES: list[dict] = [
    {
        "table": "AP_USER",
        "pk": "PK_AP_USER",
        "pk_cols": ["rec_id"],
        "comment": "登录用户。口令只存 bcrypt 哈希。",
        "cols": [
            ("rec_id", "id", False, None, "主键 UUID"),
            ("login_name", "name", False, None, "登录名，全局唯一"),
            ("display_name", "name", False, None, "显示名"),
            ("password_hash", "varchar256", False, None, "bcrypt 哈希"),
            ("is_enabled", "flag", False, "1", "1 启用 / 0 停用"),
            ("remark", "varchar500", True, None, "备注"),
        ],
        "indexes": [
            ("UK_AP_USER_LOGIN", ["login_name"], True),
        ],
    },
    {
        "table": "AP_WORKSPACE",
        "pk": "PK_AP_WS",
        "pk_cols": ["rec_id"],
        "comment": "工作空间。配置与作业的边界。",
        "cols": [
            ("rec_id", "id", False, None, "主键 UUID"),
            ("workspace_code", "name", False, None, "空间编码，全局唯一"),
            ("workspace_name", "name", False, None, "显示名"),
            ("owner_user_id", "id", False, None, "负责人 user rec_id"),
            ("remark", "varchar500", True, None, "备注"),
        ],
        "indexes": [
            ("UK_AP_WS_CODE", ["workspace_code"], True),
            ("IDX_AP_WS_OWNER", ["owner_user_id"]),
        ],
    },
    {
        "table": "AP_WORKSPACE_MEMBER",
        "pk": "PK_AP_WSM",
        "pk_cols": ["rec_id"],
        "comment": "空间成员。③ 闸认这一行；角色 owner / operator / viewer。",
        "cols": [
            ("rec_id", "id", False, None, "主键 UUID"),
            ("workspace_rec_id", "id", False, None, "空间"),
            ("user_rec_id", "id", False, None, "成员"),
            ("member_role", "varchar64", False, None, "owner / operator / viewer"),
        ],
        "indexes": [
            ("UK_AP_WSM", ["workspace_rec_id", "user_rec_id"], True),
            ("IDX_AP_WSM_USER", ["user_rec_id"]),
        ],
    },
    {
        "table": "AP_ROLE_ACTION",
        "pk": "PK_AP_RA",
        "pk_cols": ["rec_id"],
        "comment": "角色已授权动作。无行则拒绝。",
        "cols": [
            ("rec_id", "id", False, None, "主键 UUID"),
            ("role_code", "varchar64", False, None, "owner / operator / viewer"),
            ("action_code", "varchar64", False, None, "job.submit 等"),
        ],
        "indexes": [
            ("UK_AP_RA", ["role_code", "action_code"], True),
        ],
    },
    {
        "table": "AP_PROJECT",
        "pk": "PK_AP_PRJ",
        "pk_cols": ["rec_id"],
        "comment": "工程配方。不直接跑命令，提交作业时读取。",
        "cols": [
            ("rec_id", "id", False, None, "主键 UUID"),
            ("workspace_rec_id", "id", False, None, "所属空间"),
            ("project_name", "name", False, None, "工程名"),
            ("project_type", "varchar64", False, None, "maven / npm / vue / go / custom"),
            ("repo_url", "url", True, None, "Git 地址"),
            ("repo_branch", "varchar128", True, None, "默认分支"),
            ("work_rel_path", "path", True, None, "空间根下相对目录"),
            ("recipe_json", "text", False, None, "构建配方 JSON"),
            ("cron_expr", "varchar64", True, None, "crontab，空则不调度"),
            ("is_enabled", "flag", False, "1", "1 启用"),
            ("remark", "varchar500", True, None, "备注"),
        ],
        "indexes": [
            ("UK_AP_PRJ_NAME", ["workspace_rec_id", "project_name"], True),
            ("IDX_AP_PRJ_WS", ["workspace_rec_id"]),
        ],
    },
    {
        "table": "AP_ENV",
        "pk": "PK_AP_ENV",
        "pk_cols": ["rec_id"],
        "comment": "环境凭据。密钥字段只存 APENC1 密文。",
        "cols": [
            ("rec_id", "id", False, None, "主键 UUID"),
            ("workspace_rec_id", "id", False, None, "所属空间"),
            ("env_kind", "varchar64", False, None, "git / ftp / smtp / ssh / jdk / maven / node"),
            ("env_name", "name", False, None, "显示名"),
            ("config_cipher", "text", False, None, "配置 JSON 密文"),
            ("config_hint", "varchar256", True, None, "脱敏提示，可回给前端"),
            ("is_enabled", "flag", False, "1", "1 启用"),
        ],
        "indexes": [
            ("IDX_AP_ENV_WS", ["workspace_rec_id", "env_kind"]),
        ],
    },
    {
        "table": "AP_JOB",
        "pk": "PK_AP_JOB",
        "pk_cols": ["rec_id"],
        "comment": "一次可审计作业。状态 queued/running/succeeded/failed/canceled。",
        "cols": [
            ("rec_id", "id", False, None, "主键 UUID"),
            ("workspace_rec_id", "id", False, None, "空间"),
            ("project_rec_id", "id", True, None, "工程，可空"),
            ("job_type", "varchar64", False, None, "build / release / ftp_push / pom_version / branch_clone / sync_sources"),
            ("job_status", "varchar64", False, None, "queued / running / succeeded / failed / canceled"),
            ("payload_json", "text", False, None, "作业参数 JSON"),
            ("error_text", "text", True, None, "失败原因"),
            ("created_by", "id", False, None, "提交人"),
            ("started_at", "datetime", True, None, "开始时间"),
            ("finished_at", "datetime", True, None, "结束时间"),
        ],
        "indexes": [
            ("IDX_AP_JOB_WS", ["workspace_rec_id", "created_at"]),
            ("IDX_AP_JOB_PRJ", ["project_rec_id", "job_status"]),
            ("IDX_AP_JOB_ST", ["job_status"]),
        ],
    },
    {
        "table": "AP_JOB_STEP",
        "pk": "PK_AP_STEP",
        "pk_cols": ["rec_id"],
        "comment": "作业步骤。日志在磁盘 data/jobs/{jobId}/{step}.log。",
        "cols": [
            ("rec_id", "id", False, None, "主键 UUID"),
            ("job_rec_id", "id", False, None, "作业"),
            ("step_key", "varchar64", False, None, "clone / build / push 等"),
            ("step_name", "name", False, None, "显示名"),
            ("sort_num", "int", False, "0", "顺序"),
            ("step_status", "varchar64", False, None, "pending / running / succeeded / failed / skipped / canceled"),
            ("error_text", "text", True, None, "失败原因"),
            ("started_at", "datetime", True, None, "开始时间"),
            ("finished_at", "datetime", True, None, "结束时间"),
        ],
        "indexes": [
            ("IDX_AP_STEP_JOB", ["job_rec_id", "sort_num"]),
        ],
    },
    {
        "table": "AP_AUDIT_LOG",
        "pk": "PK_AP_AUD",
        "pk_cols": ["rec_id"],
        "comment": "操作审计。",
        "cols": [
            ("rec_id", "id", False, None, "主键 UUID"),
            ("actor_user_id", "id", False, None, "操作人"),
            ("workspace_rec_id", "id", True, None, "空间，可空"),
            ("action_code", "varchar64", False, None, "动作"),
            ("target_type", "varchar64", True, None, "目标类型"),
            ("target_id", "id", True, None, "目标主键"),
            ("summary", "varchar500", False, None, "摘要"),
            ("success_flag", "flag", False, "1", "1 成功"),
        ],
        "indexes": [
            ("IDX_AP_AUD_WS", ["workspace_rec_id", "created_at"]),
            ("IDX_AP_AUD_ACTOR", ["actor_user_id"]),
        ],
    },
    {
        "table": "AP_SETTING",
        "pk": "PK_AP_SET",
        "pk_cols": ["rec_id"],
        "comment": "平台系统属性。key/value，供作业并发、日志保留、请求超时等。",
        "cols": [
            ("rec_id", "id", False, None, "主键 UUID"),
            ("setting_key", "varchar64", False, None, "job.concurrency / log.retention_days / web.timeout_sec"),
            ("setting_value", "varchar256", False, None, "属性值"),
            ("remark", "varchar500", True, None, "备注"),
        ],
        "indexes": [
            ("UK_AP_SET_KEY", ["setting_key"], True),
        ],
    },
    {
        "table": "AP_GLOBAL_VAR",
        "pk": "PK_AP_GVAR",
        "pk_cols": ["rec_id"],
        "comment": "全局环境变量。构建进程可叠加；禁止覆盖 PATH 等保留键。",
        "cols": [
            ("rec_id", "id", False, None, "主键 UUID"),
            ("var_key", "varchar64", False, None, "变量名"),
            ("var_value", "text", False, None, "变量值"),
            ("is_enabled", "flag", False, "1", "1 启用"),
            ("remark", "varchar500", True, None, "备注"),
        ],
        "indexes": [
            ("UK_AP_GVAR_KEY", ["var_key"], True),
        ],
    },
    {
        "table": "AP_WORKSPACE_RELEASE",
        "pk": "PK_AP_WSREL",
        "pk_cols": ["rec_id"],
        "comment": "工作空间自动发版记录。日志在 data/releases/{id}.log。",
        "cols": [
            ("rec_id", "id", False, None, "主键 UUID"),
            ("workspace_rec_id", "id", False, None, "空间"),
            ("release_num", "int", False, "0", "空间内序号"),
            ("status", "varchar64", False, None, "running / success / partial / failed"),
            ("release_branch", "varchar128", True, None, "发版分支"),
            ("tag_branch", "varchar128", True, None, "版本 Tag"),
            ("target_branch", "varchar128", True, None, "目标分支"),
            ("released", "int", False, "0", "成功工程数"),
            ("skipped", "int", False, "0", "跳过工程数"),
            ("failed", "int", False, "0", "失败工程数"),
            ("message", "varchar500", True, None, "摘要"),
            ("log_path", "path", True, None, "日志文件"),
            ("post_options_json", "text", True, None, "发版后构建/FTP 选项"),
            ("started_at", "datetime", True, None, "开始时间"),
            ("finished_at", "datetime", True, None, "结束时间"),
        ],
        "indexes": [
            ("IDX_AP_WSREL_WS", ["workspace_rec_id", "release_num"]),
        ],
    },
]


def q(engine: str, name: str) -> str:
    if engine == "mysql":
        return f"`{name}`"
    return f'"{name}"'


def typ(engine: str, key: str) -> str:
    if engine == "mysql":
        return {
            "id": "VARCHAR(36)",
            "name": "VARCHAR(128)",
            "text": "LONGTEXT",
            "flag": "TINYINT NOT NULL",
            "int": "INT",
            "datetime": "DATETIME",
            "varchar64": "VARCHAR(64)",
            "varchar128": "VARCHAR(128)",
            "varchar256": "VARCHAR(256)",
            "varchar500": "VARCHAR(500)",
            "path": "VARCHAR(1024)",
            "url": "VARCHAR(1024)",
        }[key]
    if engine == "postgres":
        return {
            "id": "VARCHAR(36)",
            "name": "VARCHAR(128)",
            "text": "TEXT",
            "flag": "SMALLINT NOT NULL",
            "int": "INTEGER",
            "datetime": "TIMESTAMP",
            "varchar64": "VARCHAR(64)",
            "varchar128": "VARCHAR(128)",
            "varchar256": "VARCHAR(256)",
            "varchar500": "VARCHAR(500)",
            "path": "VARCHAR(1024)",
            "url": "VARCHAR(1024)",
        }[key]
    return {
        "id": "TEXT",
        "name": "TEXT",
        "text": "TEXT",
        "flag": "INTEGER NOT NULL",
        "int": "INTEGER",
        "datetime": "TEXT",
        "varchar64": "TEXT",
        "varchar128": "TEXT",
        "varchar256": "TEXT",
        "varchar500": "TEXT",
        "path": "TEXT",
        "url": "TEXT",
    }[key]


def default_sql(engine: str, key: str, default) -> str:
    if default is None:
        return ""
    if default == "NOW":
        if engine == "mysql":
            return " DEFAULT CURRENT_TIMESTAMP"
        if engine == "postgres":
            return " DEFAULT CURRENT_TIMESTAMP"
        return " DEFAULT (datetime('now'))"
    if default == "1":
        return " DEFAULT 1"
    if default == "0":
        return " DEFAULT 0"
    return f" DEFAULT {default}"


def render_table(engine: str, spec: dict) -> str:
    lines = [f"-- {spec['comment']}", f"CREATE TABLE IF NOT EXISTS {q(engine, spec['table'])} ("]
    cols = spec["cols"] + COMMON
    col_sql = []
    for name, key, nullable, default, comment in cols:
        null = "" if not nullable else ""
        if not nullable and key != "flag":
            null = " NOT NULL"
        elif not nullable and key == "flag":
            null = ""
        d = default_sql(engine, key, default)
        piece = f"  {q(engine, name)} {typ(engine, key)}{null}{d}"
        if engine == "mysql":
            cmt = comment.replace("'", "''")
            piece += f" COMMENT '{cmt}'"
        col_sql.append(piece)
    pk = ", ".join(q(engine, c) for c in spec["pk_cols"])
    col_sql.append(f"  CONSTRAINT {spec['pk']} PRIMARY KEY ({pk})")
    if engine == "mysql":
        for iname, icols, unique in index_parts(spec):
            kind = "UNIQUE KEY" if unique else "KEY"
            cols_sql = ", ".join(q(engine, c) for c in icols)
            col_sql.append(f"  {kind} {q(engine, iname)} ({cols_sql})")
        cmt = spec["comment"].replace("'", "''")
        lines.append(",\n".join(col_sql))
        lines.append(f") ENGINE=InnoDB COMMENT='{cmt}';")
    else:
        lines.append(",\n".join(col_sql))
        lines.append(");")
    lines.append("")
    if engine == "postgres":
        lines.append(f"COMMENT ON TABLE {q(engine, spec['table'])} IS '{spec['comment']}';")
        for name, _k, _n, _d, comment in spec["cols"] + COMMON:
            lines.append(f"COMMENT ON COLUMN {q(engine, spec['table'])}.{q(engine, name)} IS '{comment}';")
        lines.append("")
    if engine != "mysql":
        for iname, icols, unique in index_parts(spec):
            u = "UNIQUE " if unique else ""
            cols_sql = ", ".join(q(engine, c) for c in icols)
            lines.append(
                f"CREATE {u}INDEX IF NOT EXISTS {q(engine, iname)} ON {q(engine, spec['table'])} ({cols_sql});"
            )
    lines.append("")
    return "\n".join(lines) + "\n"


def index_parts(spec: dict) -> list[tuple[str, list[str], bool]]:
    out = []
    for idx in spec["indexes"]:
        out.append((idx[0], idx[1], len(idx) > 2 and bool(idx[2])))
    return out


def seed_sql(engine: str) -> str:
    now = "CURRENT_TIMESTAMP" if engine != "sqlite" else "datetime('now')"
    rows = [
        ("ra-owner-js", "owner", "job.submit"),
        ("ra-owner-jc", "owner", "job.cancel"),
        ("ra-owner-pw", "owner", "project.write"),
        ("ra-owner-ew", "owner", "env.write"),
        ("ra-owner-ua", "owner", "user.admin"),
        ("ra-owner-sw", "owner", "settings.write"),
        ("ra-owner-ar", "owner", "audit.read"),
        ("ra-op-js", "operator", "job.submit"),
        ("ra-op-jc", "operator", "job.cancel"),
        ("ra-op-pw", "operator", "project.write"),
        ("ra-op-ew", "operator", "env.write"),
        ("ra-op-ar", "operator", "audit.read"),
        ("ra-vw-ar", "viewer", "audit.read"),
    ]
    t = q(engine, "AP_ROLE_ACTION")
    cols = ", ".join(
        q(engine, c)
        for c in ("rec_id", "role_code", "action_code", "created_at", "updated_at")
    )
    vals = []
    for rec, role, action in rows:
        vals.append(f"('{rec}', '{role}', '{action}', {now}, {now})")
    body = ",\n    ".join(vals)
    if engine == "sqlite":
        ins = f"INSERT OR IGNORE INTO {t} ({cols}) VALUES\n    {body};\n"
    elif engine == "mysql":
        ins = f"INSERT IGNORE INTO {t} ({cols}) VALUES\n    {body};\n"
    else:
        ins = (
            f"INSERT INTO {t} ({cols}) VALUES\n    {body}\n"
            f"ON CONFLICT ({q(engine, 'role_code')}, {q(engine, 'action_code')}) DO NOTHING;\n"
        )
    return "-- 预置角色动作。用户由应用层 seed。\n" + ins


def header(engine: str) -> str:
    titles = {
        "sqlite": "SQLite 平台库。默认离线。",
        "mysql": "MySQL / MariaDB 平台库。字符集跟随数据库，脚本不写 CHARSET / COLLATE。",
        "postgres": "PostgreSQL 平台库。",
    }
    return (
        f"-- auto-pack 平台库 DDL（{engine}）\n"
        f"-- {titles[engine]}\n"
        "-- 改字段请改 script/db/gen_ddl.py 后重新生成。\n"
        "-- 禁止 FOREIGN KEY / REFERENCES。密钥只存密文。\n\n"
    )


def render_patch(engine: str) -> str:
    lines = [
        header(engine),
        "-- 已有表补列 / 补索引。可重复执行（已存在则跳过）。\n",
        "-- 启动时在 init.sql 之后执行。\n",
    ]
    if engine == "mysql":
        # 上次失败可能已建表但没建索引；MySQL 无 CREATE INDEX IF NOT EXISTS。
        for spec in TABLES:
            for iname, icols, unique in index_parts(spec):
                u = "UNIQUE " if unique else ""
                cols_sql = ", ".join(q(engine, c) for c in icols)
                lines.append(
                    f"CREATE {u}INDEX {q(engine, iname)} ON {q(engine, spec['table'])} ({cols_sql});\n"
                )
        return "".join(lines)
    return "".join(lines) + "-- 当前无增量列。\nSELECT 1;\n"


def write_engine(engine: str) -> None:
    d = ROOT / engine
    d.mkdir(parents=True, exist_ok=True)
    for old in d.glob("*.sql"):
        old.unlink()
    prelude = header(engine)
    if engine == "mysql":
        prelude += "-- CREATE DATABASE auto_pack;  -- 字符集用库默认，不要在表上写 CHARSET\n\n"
    if engine == "postgres":
        prelude += "-- createdb autopack\n\n"
    bodies = [render_table(engine, spec) for spec in TABLES]
    seed = seed_sql(engine)
    (d / "99_seed.sql").write_text(header(engine) + seed, encoding="utf-8")
    (d / "98_patch.sql").write_text(render_patch(engine), encoding="utf-8")
    (d / "init.sql").write_text(prelude + "".join(bodies) + seed, encoding="utf-8")


def main() -> None:
    for engine in ("sqlite", "mysql", "postgres"):
        write_engine(engine)
    print("ok", ROOT)


if __name__ == "__main__":
    main()
