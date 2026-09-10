# auto-pack

作业型自动打包控制台（2.0）。配置（空间 / 工程 / 环境）与执行（作业）分开；UI 用 [niuma-ui](https://github.com/Blair-Shang/niuma-ui)；平台库支持 SQLite / PostgreSQL / MySQL。

试用版 1.0.0 已冻结：tag `v1.0.0`，源码在 [legacy/](legacy/)。

## 版本

| 名称 | 示例 | 管什么 |
|------|------|--------|
| 控制台 SemVer | `v2.0.0` | 本仓库；见 [CHANGELOG.md](CHANGELOG.md) |
| 产品发版 Tag | `V9.0.0-P01` | 被打包工程，只出现在作业参数里 |

发版只改根目录 `package.json` 的 `version` 和 [CHANGELOG.md](CHANGELOG.md)。`pnpm dev` / 打包会跑 `node script/sync-version.mjs`，把号写入 `server/internal/config.AppVersion`；发行包还会用 `-ldflags` 再注入一次。

## 目录

```
server/                 # Go 控制面
  cmd/                  # 入口
  configs/app.yaml.example  # 复制为 app.yaml；本机 yaml 不入库
  internal/
    config/             # YAML + AppVersion
    store/              # 三库连接与事务
    migrate/            # init.sql + 98_patch.sql
    iam/                # 身份 / 空间成员 / 动作
    crypt/              # 环境密文
    token/              # JWT，无 HTTP
    web/                # L4 HTTP，按功能模块划分
      mod/ router/      # 前缀与组装；router 不写业务路径
      health/ auth/ jobs/ workspaces/ projects/ envs/ users/ audit/
    host/               # 本机工具族（bash git npm maven ftp）
    job/                # 作业控制面（入队 / 查询 / 取消 / cron）
      runner/           # 执行面
    workspace/ project/ env/ user/ audit/
src/                    # Vue 3 + niuma-ui
script/
  db/                   # sqlite / mysql / postgres DDL
  build/                # Windows / Linux 全局构建
  run/                  # 启停模板（打进发行包根目录）
  package/              # 发行包说明
legacy/                 # v1.0.0 试用版
```

## 启动

```bash
# 需 Node >= 20、pnpm、Go；UI 从 npm 装 niuma-ui
# 首次：复制示例配置，再按本机改 DSN / 密钥（app.yaml 不进 git）
#   Copy-Item server/configs/app.yaml.example server/configs/app.yaml
pnpm install
pnpm dev          # Vite :5173 + 后端 :8080
# pnpm dev:web    # 只前端
# pnpm dev:server # 只后端
```

浏览器：http://localhost:5173/autopack/  
API：`/autopack/api/...`  
默认账号：`admin` / `123456`

`app.yaml` 里 `database.driver` 可改为 `postgres` 或 `mysql`。`workspace.root` 是所有空间与工程的检出根目录，默认 `./data/workspaces/{空间编码}/{工程名}`。

## 发行包

全局构建把前端、后端和运维脚本打进 `build/dist/auto-pack-{版本}-{os}-{arch}/`，按 `bin` / `web` / `configs` / `script/db` 分开，不带源码和本机密钥。说明见 [script/README.md](script/README.md)。

```powershell
pnpm package:windows
# 或交叉编 Linux：
# powershell -NoProfile -ExecutionPolicy Bypass -File script/build/windows.ps1 -OS linux
```

```bash
pnpm package:linux
# 或：bash script/build/linux.sh --os windows
```

到发行目录后：`start.bat` / `./start.sh`。浏览器 http://127.0.0.1:8080/autopack/

## 权限（2.0）

JWT 只证明身份。写操作走 `iam.Allow(actor, workspace, action)`。

| 角色 | 动作 |
|------|------|
| owner | 全部 |
| operator | `job.submit` `job.cancel` `project.write` `env.write` `audit.read` |
| viewer | 只读作业与日志 |

`user.admin` 管成员与动作矩阵。完整 RBAC UI 在用户页；闸在 service，不在 Runner。

## 作业类型

`build` · `release` · `ftp_push` · `pom_version` · `branch_clone` · `sync_sources`

cron 只负责入队，不直接跑构建。
