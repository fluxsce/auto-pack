#!/usr/bin/env python3
# 从 1.0 SQLite 的 AP_SYS_ENV 导出明文环境，再按原 rec_id 写入 2.0 某个工作空间。
# 2.0 不能直接打开 1.0 库；工程走空间导出/导入，环境走本脚本。
from __future__ import annotations

import argparse
import json
import sqlite3
import sys
import urllib.error
import urllib.request

KIND_ALIAS = {"nodejs": "node", "node.js": "node"}


def export_envs(db_path: str, out_path: str) -> None:
    conn = sqlite3.connect(db_path)
    conn.row_factory = sqlite3.Row
    items = load_envs(conn)
    conn.close()
    payload = {"format": "auto-pack-v1-envs", "count": len(items), "envs": items}
    with open(out_path, "w", encoding="utf-8", newline="\n") as fh:
        json.dump(payload, fh, ensure_ascii=False, indent=2)
        fh.write("\n")
    print(f"exported {len(items)} envs -> {out_path}")


def api(base: str, method: str, path: str, token: str | None, body: dict | None) -> dict:
    url = base.rstrip("/") + path
    data = None if body is None else json.dumps(body, ensure_ascii=False).encode("utf-8")
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req) as resp:
            raw = resp.read().decode("utf-8")
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode("utf-8", errors="replace")
        raise SystemExit(f"{method} {path} -> {exc.code}: {detail}") from exc
    return json.loads(raw) if raw else {}


def load_envs(conn: sqlite3.Connection) -> list[dict]:
    rows = conn.execute(
        """
        SELECT rec_id, env_name, env_type, config_json, is_enabled, remark
        FROM AP_SYS_ENV
        ORDER BY env_type, env_name
        """
    ).fetchall()
    items = []
    for row in rows:
        raw = row["config_json"] or "{}"
        try:
            config = json.loads(raw)
        except json.JSONDecodeError:
            config = {}
        if not isinstance(config, dict):
            config = {}
        items.append(
            {
                "rec_id": row["rec_id"],
                "env_name": row["env_name"],
                "env_type": row["env_type"],
                "env_kind": KIND_ALIAS.get((row["env_type"] or "").strip(), (row["env_type"] or "").strip()),
                "is_enabled": int(row["is_enabled"] or 0),
                "remark": row["remark"] or "",
                "config": config,
            }
        )
    return items


def workspace_bundle(conn: sqlite3.Connection, ws: sqlite3.Row) -> dict:
    projects = []
    for p in conn.execute(
        """
        SELECT rec_id, project_name, project_version, project_type, repo_url, repo_branch,
               work_dir, config_json, git_env_rec_id, jdk_env_rec_id, maven_env_rec_id,
               nodejs_env_rec_id, deploy_env_rec_id, is_enabled, remark
        FROM AP_PACK_PROJECT
        WHERE workspace_rec_id = ?
        ORDER BY project_name
        """,
        (ws["rec_id"],),
    ):
        projects.append(
            {
                "source_rec_id": p["rec_id"],
                "project_name": p["project_name"],
                "project_version": p["project_version"],
                "project_type": p["project_type"],
                "repo_url": p["repo_url"],
                "repo_branch": p["repo_branch"],
                "work_dir": p["work_dir"],
                "config_json": p["config_json"],
                "git_env_rec_id": p["git_env_rec_id"],
                "jdk_env_rec_id": p["jdk_env_rec_id"],
                "maven_env_rec_id": p["maven_env_rec_id"],
                "nodejs_env_rec_id": p["nodejs_env_rec_id"],
                "deploy_env_rec_id": p["deploy_env_rec_id"],
                "is_enabled": int(p["is_enabled"] or 1),
                "remark": p["remark"] or "",
            }
        )
    return {
        "format_version": 1,
        "source_workspace_rec_id": ws["rec_id"],
        "workspace": {
            "workspace_code": ws["workspace_code"],
            "workspace_name": ws["workspace_name"],
            "remark": ws["remark"] or "",
        },
        "projects": projects,
    }


def dump_db(db_path: str, out_dir: str) -> None:
    import os
    from datetime import datetime, timezone

    os.makedirs(out_dir, exist_ok=True)
    conn = sqlite3.connect(db_path)
    conn.row_factory = sqlite3.Row
    envs = load_envs(conn)
    env_path = os.path.join(out_dir, "envs.json")
    with open(env_path, "w", encoding="utf-8", newline="\n") as fh:
        json.dump({"format": "auto-pack-v1-envs", "count": len(envs), "envs": envs}, fh, ensure_ascii=False, indent=2)
        fh.write("\n")
    ws_dir = os.path.join(out_dir, "workspaces")
    os.makedirs(ws_dir, exist_ok=True)
    workspaces = conn.execute(
        "SELECT rec_id, workspace_code, workspace_name, remark FROM AP_PACK_WORKSPACE ORDER BY workspace_code"
    ).fetchall()
    index = []
    for ws in workspaces:
        bundle = workspace_bundle(conn, ws)
        name = (ws["workspace_code"] or ws["rec_id"]).replace("/", "-")
        path = os.path.join(ws_dir, f"{name}.json")
        with open(path, "w", encoding="utf-8", newline="\n") as fh:
            json.dump(bundle, fh, ensure_ascii=False, indent=2)
            fh.write("\n")
        index.append({"code": ws["workspace_code"], "name": ws["workspace_name"], "projects": len(bundle["projects"]), "file": path})
        print(f"workspace {ws['workspace_code']} {len(bundle['projects'])} projects -> {path}")
    conn.close()
    idx_path = os.path.join(out_dir, "index.json")
    with open(idx_path, "w", encoding="utf-8", newline="\n") as fh:
        json.dump({"exported_at": datetime.now(timezone.utc).isoformat(), "envs": len(envs), "workspaces": index}, fh, ensure_ascii=False, indent=2)
        fh.write("\n")
    print(f"exported {len(envs)} envs -> {env_path}")
    print(f"index -> {idx_path}")


def login(base: str, username: str, password: str) -> str:
    out = api(base, "POST", "/autopack/api/auth/login", None, {"username": username, "password": password})
    token = out.get("token")
    if not token:
        raise SystemExit("login did not return token")
    return token


def list_workspaces(base: str, token: str) -> list[dict]:
    out = api(base, "GET", "/autopack/api/workspaces", token, None)
    if isinstance(out, list):
        return out
    return out.get("items") or out.get("list") or []


def list_envs(base: str, token: str) -> list[dict]:
    out = api(base, "GET", "/autopack/api/envs", token, None)
    if isinstance(out, list):
        return out
    return out.get("items") or out.get("list") or []


def import_env_items(base: str, token: str, items: list[dict], keep_ids: bool = True) -> None:
    have = {(row.get("env_kind") or "", row.get("rec_id") or "") for row in list_envs(base, token)}
    for item in items:
        kind = item.get("env_kind") or item.get("env_type") or ""
        rec_id = item.get("rec_id") or ""
        if keep_ids and rec_id and (kind, rec_id) in have:
            print(f"  skip {kind} {item.get('env_name')} (already global)")
            continue
        body = {
            "env_kind": kind,
            "env_name": item.get("env_name") or "",
            "is_enabled": int(item.get("is_enabled") or 1),
            "config": item.get("config") or {},
        }
        if keep_ids:
            body["rec_id"] = rec_id
        api(base, "POST", "/autopack/api/envs", token, body)
        print(f"  env {body['env_kind']} {body['env_name']}")


def import_envs(file_path: str, base: str, workspace_id: str, username: str, password: str) -> None:
    with open(file_path, encoding="utf-8") as fh:
        payload = json.load(fh)
    items = payload.get("envs") if isinstance(payload, dict) else payload
    if not isinstance(items, list):
        raise SystemExit("invalid export file")
    token = login(base, username, password)
    import_env_items(base, token, items)
    print(f"imported {len(items)} global envs")


def push_db(db_path: str, base: str, username: str, password: str) -> None:
    conn = sqlite3.connect(db_path)
    conn.row_factory = sqlite3.Row
    envs = load_envs(conn)
    workspaces = conn.execute(
        "SELECT rec_id, workspace_code, workspace_name, remark FROM AP_PACK_WORKSPACE ORDER BY workspace_code"
    ).fetchall()
    token = login(base, username, password)
    import_env_items(base, token, envs)
    print(f"imported {len(envs)} global envs")
    existing = {str(w.get("workspace_code") or ""): w for w in list_workspaces(base, token)}
    for ws in workspaces:
        bundle = workspace_bundle(conn, ws)
        code = ws["workspace_code"]
        hit = existing.get(code)
        if hit:
            body = {
                "mode": "replace",
                "target_workspace_rec_id": hit.get("rec_id"),
                "bundle": bundle,
            }
        else:
            body = {
                "mode": "new",
                "workspace_code": code,
                "workspace_name": ws["workspace_name"] or code,
                "bundle": bundle,
            }
        out = api(base, "POST", "/autopack/api/workspaces/import", token, body)
        wid = out.get("rec_id") or (hit.get("rec_id") if hit else "")
        print(f"workspace {code} -> {wid} projects={out.get('projects_imported')}")
    conn.close()


def main() -> None:
    parser = argparse.ArgumentParser(description="Migrate auto-pack 1.0 AP_SYS_ENV into 2.0 AP_ENV")
    sub = parser.add_subparsers(dest="cmd", required=True)

    exp = sub.add_parser("export", help="dump 1.0 sqlite AP_SYS_ENV to JSON")
    exp.add_argument("--db", required=True, help="1.0 sqlite path, e.g. legacy/data/db/autopack.db")
    exp.add_argument("--out", required=True, help="output JSON")

    dump = sub.add_parser("dump", help="dump envs + each workspace v1 bundle")
    dump.add_argument("--db", required=True)
    dump.add_argument("--out-dir", required=True)

    imp = sub.add_parser("import", help="POST dumped envs into a running 2.0 workspace")
    imp.add_argument("--file", required=True, help="JSON from export")
    imp.add_argument("--base", default="http://127.0.0.1:8080", help="2.0 origin")
    imp.add_argument("--workspace", required=True, help="2.0 workspace_rec_id")
    imp.add_argument("--user", default="admin")
    imp.add_argument("--password", default="123456")

    push = sub.add_parser("push-db", help="import all 1.0 workspaces + global envs into running 2.0")
    push.add_argument("--db", required=True)
    push.add_argument("--base", default="http://127.0.0.1:8080")
    push.add_argument("--user", default="admin")
    push.add_argument("--password", default="123456")

    att = sub.add_parser("attach", help="import 1.0 envs as global 2.0 AP_ENV")
    att.add_argument("--db", required=True)
    att.add_argument("--base", default="http://127.0.0.1:8080")
    att.add_argument("--user", default="admin")
    att.add_argument("--password", default="123456")

    args = parser.parse_args()
    if args.cmd == "export":
        export_envs(args.db, args.out)
        return
    if args.cmd == "dump":
        dump_db(args.db, args.out_dir)
        return
    if args.cmd == "push-db":
        push_db(args.db, args.base, args.user, args.password)
        return
    if args.cmd == "attach":
        conn = sqlite3.connect(args.db)
        conn.row_factory = sqlite3.Row
        envs = load_envs(conn)
        conn.close()
        token = login(args.base, args.user, args.password)
        import_env_items(args.base, token, envs)
        return
    import_envs(args.file, args.base, args.workspace, args.user, args.password)


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        sys.exit(130)
