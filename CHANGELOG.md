# 变更日志

本文件记录 **auto-pack 控制台** 的 SemVer。不要和被打包工程的产品 Tag（如 `V9.0.0-P01`）混用。

格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，版本遵循 [Semantic Versioning](https://semver.org/lang/zh-CN/)。

控制台版本以根目录 `package.json` 为准。`node script/sync-version.mjs` 写入 `server/internal/config.AppVersion`；`pnpm dev` / `build` / `test:server` / 打包前会自动跑。发行包构建仍用 `-ldflags` 再注入一次。

## [Unreleased]

## [2.0.5] - 2026-09-10

### 变更

- 自动构建已在预同步或排队构建时，重开弹窗不再清空右侧日志、不再重打同步；「开始构建」保持禁用，并继续接入当前进度。

### 修复

- Node 环境探测：`node_path` 填安装根或 `bin` 目录时不再 `exec` 目录（Linux 上会报 permission denied），会解析到 `node` 再探测。

## [2.0.4] - 2026-09-10

### 变更

- 依赖顺序自动构建：预同步按工程推送进度（序号 / 当前工程 / 拉取或克隆结果 / Git 输出），解析依赖与构建步骤也会写进右侧日志，不再只停在「正在预同步」。
- 自动构建 / 发版 / 推送日志超长自动换行。同一空间的 Git 预同步互斥，避免两个对话框抢同一检出目录。
- 当前空间已有自动构建或自动发版时，工程页按钮仍可打开对应弹窗；弹窗内提示占用并禁用底部操作。
- 本地 `@niuma/ui` 改为 `link:../niuma-ui`，跟 niuma-ui 源码联调；CI 打包仍会改回 npm 版本。
- 工程详情构建日志不再把整页撑高：面板锁在视口内，日志与左侧历史各自滚动。
- 自动构建改为工作窗：左右栏互不影响；左侧整栏滚动，右侧日志单独滚动。

## [2.0.3] - 2026-09-10

### 变更

- 设置全部按平台生效，不再跟当前工作空间绑定：SDK 环境（`AP_ENV`）、系统属性、全局变量打开即可读写。
- 成员改为平台权限（`AP_WORKSPACE_MEMBER`）：有成员身份即可看到全部空间，角色对所有空间生效；删空间不撤成员。启动时为已有用户补全局成员行。
- 1.0 导入的环境只写一份全局凭据，不再按空间克隆。

## [2.0.2] - 2026-09-10


### 新增

- GitHub Actions `Pack`：只在版本发布时打 Linux / Windows amd64 包（`v*` 标签或手动触发），不跟每次提交。私人仓库只留最近一次构建产物。
- 发行压缩包文件名带版本；解压后目录固定为 `auto-pack/`，版本只写在包内 `VERSION`。

## [2.0.1] - 2026-09-10

### 新增

- `script/build` 全局构建（Windows / Linux）：前端、后端、按 `bin` / `web` / `configs` / `script/db` 组装发行目录，并带 `start` / `stop` / `restart`。

### 变更

- 控制台 SemVer 只改 `package.json`（和本 changelog）：`script/sync-version.mjs` 同步到 `AppVersion`，开发与打包都会跑。
- 自动发版运行日志：顶部状态不再把「跳过 0 / 失败 0」误标成警告或错误；复制按钮复制全文。刚开跑时回写发版后选项不再误报 404。
- 自动发版运行日志顶部始终显示本次成功 / 部分失败 / 失败状态（轮询与历史回看都保留）。
- 同步更新 POM 时去掉 Git Tag 前缀 `V`：`V9.0.0-P08` 写入 `9.0.0-P08-SNAPSHOT`。
- 自动发版左侧用 `RsFieldset` 分组；远程推送复用 `PushPanel`（环境 / 路径 / 备份 / updateNote 预览），推送工程按类型勾选。
- 自动构建预同步日志按工程打印：拉取/克隆、分支、提交短哈希、跳过原因，以及 Git 命令输出；不再只留开始/汇总两行。
- 自动构建把 npm / Node / Vue 纳入依赖顺序（读 `package.json` 的 name 与依赖，对齐 1.0 可构建这些类型）；打开弹窗也会预同步 Git。Go / 自定义仍由「其他工程」开关追加。
- 自动构建左侧用 `RsFieldset` 分组（范围 / 顺序预览）。预览即实际构建队列：停用工程不入队；POM 继承 parent.groupId、忽略 dependencyManagement / 插件依赖；无法解析的 Maven 工程追加到队尾并警告，不再静默丢弃。
- 作业日志、工程控制台、自动构建 / 发版 / 推送的运行日志改为 `RsLog`：按行虚拟滚动、级别着色、搜索与复制。表单里的说明多行输入仍用 `RsTextarea`。
- 工程目录远程推送改为独立工作窗（`components/remote/`）：左表单对齐 1.0（对象 / 环境 / 路径 / 备份 / 推送人 / 变更说明 / updateNote 预览）+ 右控制台；文件夹打成 `{目录名}.zip` 再上传；推送成功后自动弹出告知邮件窗（关掉推送窗也不中断）。推送作业不写入构建历史，完成后也不自动关窗。成功列表只显示文件名，「发送告知」用主按钮。告知邮件窗改为左字段右预览。
- 远程推送按 1.0：远端已有文件先拷到 `BAK/{时间戳}` 再覆盖；推送后追加（或新建）`updateNote.txt`。界面预览会读取远端已有说明并合并展示。

### 新增

- 作业页按空间分页列出（默认 20 条，可改页大小）；顶部进行中卡片与统计按当前筛选统计全量，不随页码截断。
- 作业保留到期后同时删除 `data/jobs/{id}` 日志目录和 `AP_JOB` / `AP_JOB_STEP` 行。清理由执行面后台线程按小时扫描，不在每次作业结束时跑；排队和运行中的作业不删。
- 空间导入遇到已有编码时不再报数据库唯一约束：所有者可覆盖该空间工程，也可一键改成副本编码再新建。
- 工程详情「工程目录」改为表格：多选、刷新、远程推送所选文件/文件夹，以及确认后清空检出目录；列出名称、类型、大小、修改时间与相对路径。`ftp_push` 作业可带 `artifacts` 显式路径。
- 工程列表右侧顶部增加「自动构建」「自动发版」「自动推送」：自动发版按 1.0 串行写 Version.txt / POM / 打 Tag（全局同时一次），发版后可按依赖顺序构建并 FTP 推送；自动推送按所选工程串行提交 `ftp_push`（FTP / SFTP / SSH）。自动发版 / 自动推送对话框为可缩放、可全屏工作窗。
- 设置模块（左右布局）：SDK 环境、系统属性（作业并发 / 日志保留 / Web 超时）、全局变量；`/envs` 改到设置下。
- `server/configs/app.yaml.example`：仓库只收示例；本机 `app.yaml`（DSN / 密钥）进 `.gitignore`。
- `app.yaml` 的 `workspace.root` 指定所有工作空间与工程的检出根目录（`{root}/{空间编码}/{工程名}`）。
- 空间编码与工程名校验为字母数字、点、下划线、连字符（如 `V9.0.0-P09`）；空间显示名称禁止路径分隔符。
- 审计日志覆盖作业提交/取消、工程与环境保存删除、用户与角色、空间成员；查询按空间隔离，支持动作/结果/关键字筛选与分页。
- 审计页展示操作人、目标与动作文案，可筛选并翻页。

### 变更

- 自动发版按功能拆目录：`release/{notes,pom,publish,suggest,order,record}`，HTTP 独立为 `/release`，前端对话框放到 `auto-build` / `auto-release`。发版协调器恢复 panic、校验分支名、错误用 sentinel，落库失败写进程日志。
- `app.yaml` 的 `log.file` 生效：进程日志同时写终端和该文件（默认 `server/logs/autopack.log`）。HTTP 4xx/5xx（除未登录）与 panic 也会写入；作业构建日志仍在 `data/jobs/{id}/job.log`。
- 空间编码与工程名目录允许点号，可用产品版本号（如 `V9.0.0-P09`）当编码或工程名，v1 导入不再被拒。
- 工程详情 Maven 依赖分析改为绑定 JDK/Maven 执行 `dependency:tree`（插件 JSON + verbose 冲突、POM 指纹缓存、默认可看直接依赖），去掉正则猜 POM。
- 文件 / 目录 / zip 收到 `internal/host/utils`；工作空间只解析检出路径并做鉴权。
- 点击工程名进入详情页：状态、控制台日志、检出目录、构建历史与依赖分析；配置仍走原编辑页。
- 工程列表展示分支、构建状态与时间（按状态着色），类型标签按工程分类区分颜色；行内提供配置 / 克隆 / 删除，并支持右键菜单。
- 空间列表支持右键导出 / 导入；可读取 v1 导出 JSON 并转成当前配方。
- 工程配置的字段帮助改为标签同行图标提示；左侧导航背景铺满卡片高度。
- 作业页展示运行中/排队作业的实时耗时、工程、当前步骤与错误；打包流程说明改到点击品牌「自动打包控制台」进入的 Dashboard。
- 空间 / 工程页改为左空间右工程列表，去掉「配方」文案。
- 新建/配置工程改为当前页嵌套路由，左侧工作空间不动，右侧为工程配置（常规 / 源码 / 环境 / 构建 / 触发 / 构建后）。
- 工程配方接入作业引擎：本地源码跳过 clone、按绑定凭据拉 Git、按绑定 JDK/Maven/Node 构建、按输出目录与制品规则发布。
- 工程配置只保留底部保存/取消；Git 地址、cron、相对路径在保存时校验。
- 工程配置按节拆到 `components/editor/`：环境可当场登记并绑定；构建含工作目录、制品预览与日志保留；触发支持 cron 与依赖工程；构建后可添加多条 FTP/邮件/脚本并随作业执行。
- 作业引擎按步骤拆文件（同步 / 构建 / 构建后 / 收尾），仍是同一个 `Engine`，不加新类型层次。
- 工作空间列表改为可搜索卡片行，支持克隆、编辑、删除（默认空间不可删）。
- 本机工具收进 `internal/host/{bash,git,npm,maven,ftp}`；作业执行面独立为 `job/runner`；JWT 从 `web/token` 挪到 `internal/token`。
- 登录校验与角色动作矩阵改走 `user.Service`，HTTP 不再直碰 `access`。
- 本机工具从 `internal/exec/{git,pom,ftp,shell}` 提到 `internal/{bash,git,npm,maven,ftp}`，作业引擎按工程类型分流。
- HTTP 按功能模块拆到 `web/{health,auth,jobs,workspaces,projects,envs,settings,users,audit}`，`web/router` 只组装。

## [2.0.0] - 2026-09-07

### 新增

- 绿地重写：作业模型（`AP_JOB` / `AP_JOB_STEP`），配置与执行分离。
- 分层：L5 niuma-ui、L4 HTTP、IAM 横切、L3 服务与作业引擎、L2 执行器、L1 三库。
- 多库：SQLite（默认）/ PostgreSQL / MySQL，DDL 由 `script/db/gen_ddl.py` 生成。
- 权限闸：身份 → 空间成员 → 动作（`iam.Allow`）；角色 owner / operator / viewer。
- 作业类型：`build` / `release` / `ftp_push` / `pom_version` / `branch_clone` / `sync_sources`；cron 入队。
- 环境凭据密文（`APENC1:`），API 只回 hint。

### 变更

- 试用版整树移入 `legacy/`，不再作为主线源码。
- UI 由 Element Plus 换为 niuma-ui。
- 控制台 SemVer 与产品线 `V*` Tag 分轨。

## [1.0.0] - 2026-09-07

试用版冻结。源码在 tag `v1.0.0` 与目录 `legacy/`。

### 说明

- 前端：Vue 3 + Element Plus；后端：Go + 单 SQLite；JWT 登录、无角色。
- 能力：工作空间 / 工程、构建与 cron、自动发版、批量 POM、分支克隆、FTP 推送、环境探测、操作审计。
- 版本号写死在 `legacy/backend/pkg/config.AppVersion`，打包不注入。
- 只修致命缺陷，不接新需求；补丁走 `release/v1`。
