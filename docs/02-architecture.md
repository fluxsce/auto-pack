# auto-pack 2.0 分层

控制台 SemVer 与产品 Tag（`V9.0.0-P01`）分轨。试用版见 `legacy/` 与 tag `v1.0.0`。

```text
L5  src/                    niuma-ui 作业台
L4  server/internal/web     HTTP：按功能模块挂路由
    iam                     ①身份 ③成员 ⑤动作
L3  job / workspace / project / env / settings / user / audit / token / release
    job/runner              执行面：跑已入队作业，无 HTTP、无 iam
    release/{notes,pom,publish,suggest,order,record}   自动发版：说明 / POM / Git / Tag / 顺序 / 落库
L2  host/{bash,git,npm,maven,ftp,ssh,smtp,utils}   本机工具族，无 HTTP、无权限表
L1  store + script/db/{sqlite,mysql,postgres}
```

写操作必须经过 `iam.Allow`。Runner 不查用户表。环境密码只存 `APENC1:` 密文。工作空间与工程检出落在 `app.yaml` 的 `workspace.root`（默认 `./data/workspaces/{空间编码}/{工程名}`）。作业日志在 `data/jobs/{作业ID}/job.log`；作业列表按空间分页。保留天数由执行面后台线程按小时扫描，到期删除日志目录和 `AP_JOB` / `AP_JOB_STEP` 行（排队和运行中的不删）。

`internal/env` 是全局凭据仓（`AP_ENV`），维护 SDK / Git / FTP 等，不按工作空间隔离，也不是进程环境变量工具。`internal/settings` 存平台系统属性（作业并发、日志保留、Web 超时）与全局变量（`AP_GLOBAL_VAR`），构建时叠加到进程环境。`AP_WORKSPACE_MEMBER` 是平台成员权限（owner / operator / viewer），不跟某个工作空间绑定；删空间不删成员。本机命令走 `host/bash`，包管理走 `host/npm` / `host/maven`。新的本机能力只许在 `internal/host/` 开子包。

HTTP 对齐 gateway / luma：每个功能一块，`route` 自注册，`control` 只做参数与状态码。L3 服务不进 `web/`。`web/router` 只 Apply + 静态页。API 前缀仍是 `/autopack/api`。
