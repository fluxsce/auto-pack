# SQLite 脚本目录规范

本目录与 `backend` 同级，存放 **auto-pack** 离线 SQLite 的建表与迁移脚本。  
应用启动时由 `backend/pkg/db` 读取并执行（默认：`sqlite/001_init_schema.sql`）。

现阶段仅维护**认证最小集**（单表 `AP_USER_AUTH`），满足前端登录与改密；不区分管理员/普通用户角色字段。

---

## 目录结构

```text
scripts/
├── README.md                 # 本规范（脚本与库表设计的唯一说明入口）
└── sqlite/
    └── 001_init_schema.sql   # 当前全量建表脚本（可重复执行）
```

后续若增加表或字段，新增 `sqlite/002_xxx.sql`、`003_xxx.sql` … 按编号递增；**禁止**在已发布脚本中改写历史 DDL（应通过新迁移增量变更）。

---

## 方言与执行约定

| 项 | 约定 |
|----|------|
| 数据库 | SQLite 3 |
| 事务 | 每个脚本文件以 `BEGIN;` … `COMMIT;` 包裹 |
| 幂等 | 使用 `CREATE TABLE IF NOT EXISTS`、`CREATE INDEX IF NOT EXISTS` |
| 注释 | SQLite 无 `COMMENT ON`，表/字段语义以脚本内 `--` 行尾中文注释为准 |
| 外键 | 本库**不声明** `FOREIGN KEY` / 不使用 `ON DELETE CASCADE`，引用由应用保证 |
| 种子数据 | 优先在应用层 `internal/db/seed` 写入；若写在 SQL 中须 `INSERT OR IGNORE` 且可重复执行 |

---

## 命名规范

### 表名

- 前缀固定 **`AP_`**（Auto-Pack 产品域）。
- 后缀全大写、下划线分段，语义完整，例如 `AP_USER_AUTH`。
- 建议总长 **≤ 30** 字符（便于索引名拼接与日志阅读）。

### 索引名

- 前缀固定 **`IX_`**，建议含 `AP` 段，例如 `IX_AP_USER_LOGIN`。
- 建议总长 **≤ 30** 字符。
- **业务唯一**用 `CREATE UNIQUE INDEX`，不与 `rec_id` 主键混用。
- 普通查询索引用 `CREATE INDEX`，列为高频过滤/排序字段。

### 字段名

- 全小写蛇形（`login_name`、`password_hash`）。
- 优先完整词根，避免无语义缩写（用 `login_name` 不用 `ln`）。
- 约定后缀：
  - `*_rec_id`：指向他表主键副本（逻辑外键，库内不建 FK）。
  - `*_hash`：单向哈希（如 bcrypt）。
  - `*_cipher`：对称加密密文。
  - `*_json`：整段 JSON 文本。

---

## 建表与字段约束（全局）

以下规则适用于本目录下所有 `CREATE TABLE` / `CREATE INDEX`：

| 编号 | 规则 |
|------|------|
| A | **主键**：每表代理主键固定为 `rec_id TEXT NOT NULL PRIMARY KEY`，由应用生成 **UUID v4**；禁止整型自增主键。 |
| B | **非空与默认**：业务必填列一律 `NOT NULL`；空列表/空对象/无备注用 `DEFAULT ''` / `'[]'` / `'{}'` 等与类型一致的占位，避免 NULL 语义漂移。 |
| C | **布尔**：SQLite 无原生 BOOLEAN，统一 `INTEGER NOT NULL`，取值仅 `0` 或 `1`，并声明 `CHECK (col IN (0,1))`。 |
| D | **业务唯一**：自然键唯一性用 `CREATE UNIQUE INDEX` 表达（如 `login_name`），不与 `rec_id` 混用。 |
| E | **逻辑外键**：列名 `*_rec_id` 仅存对方主键副本；不声明外键约束。 |
| F | **CHECK**：仅对库内可判定的有界域添加（如 `seq_num >= 0`）；枚举长列表、JSON 语法、URL 格式由应用校验。 |
| G | **JSON**：JSON 语义列存 `TEXT`（列名建议 `*_json`），默认 `'{}'` 或 `'[]'`；合法性由写入方保证。 |
| H | **时间**：可变业务行含 `created_at`、`updated_at`，类型 `TEXT`，默认须写 **`(datetime('now'))`**（函数默认值外加一层括号，与纯 Go SQLite 驱动兼容）；应用层建议统一 UTC 或固定时区策略。 |
| I | **列注释**：`CREATE TABLE` 体内每个物理列必须有行尾 `-- …` 中文说明（存什么、对应哪块 API/界面）。 |

---

## 当前库表（认证最小集）

### `AP_USER_AUTH` — 用户认证

| 列名 | 类型 | 说明 |
|------|------|------|
| `rec_id` | TEXT PK | 用户主键（UUID v4） |
| `login_name` | TEXT | 登录名；业务唯一（见索引） |
| `password_hash` | TEXT | bcrypt 口令哈希（成本建议 10） |
| `is_enabled` | INTEGER | 启停：`1` 启用 / `0` 禁用 |
| `remark` | TEXT | 备注 |
| `created_at` | TEXT | 创建时间 |
| `updated_at` | TEXT | 更新时间 |

**索引**

| 索引名 | 类型 | 列 |
|--------|------|-----|
| `IX_AP_USER_LOGIN` | UNIQUE | `login_name` |

**与后端/前端的对应关系**

- 登录：`POST /autopack/api/auth/login`，按 `login_name` + 口令校验 `password_hash`。
- 当前用户：`GET /autopack/api/auth/me`（JWT 内携带用户标识，与 `rec_id` 或 `login_name` 映射由应用约定）。
- 改密 / 改用户名：需校验旧口令后更新本表（应用层实现）。
- **无** `account_kind`、`realm`、角色分级等字段；权限若后续需要应通过新表或新迁移增量添加。

**初始化账号（应用种子，非 SQL 脚本内置）**

- 默认由 `backend/internal/db/seed` 在库空时写入（待与 `AP_USER_AUTH` 对齐）。
- 开发默认：`login_name = admin`，初始口令 `123456`（仅存哈希，禁止明文落库）。

---

## 编写新脚本检查清单

提交 `sqlite/00N_*.sql` 前请自检：

1. 文件头注释说明本脚本用途（新建表 / 加列 / 加索引）。
2. 使用 `BEGIN` / `COMMIT`，且语句可重复执行。
3. 新表符合上表 **A–I** 规则，表名 `AP_*`。
4. 业务唯一键已建 `IX_AP_*` 的 `UNIQUE INDEX`。
5. 每个列均有中文行尾注释。
6. 未引入未使用的表；认证阶段不提前建“未来可能用到”的宽表。
7. 若含种子 `INSERT`，使用 `INSERT OR IGNORE` 并注明幂等条件。

---

## 与 backend 的集成

- 入口：`backend/pkg/db.Open` → `migrate()` 读取 `scripts/sqlite/001_init_schema.sql` 并执行。
- 路径解析：优先可执行文件同目录下的 `scripts/`，其次当前目录与 `../scripts/`（兼容在 `backend` 目录开发启动）。
- 种子数据：`backend/internal/repository/seed.go` 负责首启默认用户等；**须与脚本表结构一致**。

---

## 全量打包（输出到 `build/`）

应用名称：**autopack**。脚本会构建前端、嵌入静态资源、编译 Go 后端（**纯 Go**，使用 `modernc.org/sqlite`），并打成压缩包。

| 平台 | 脚本 | 输出 |
|------|------|------|
| Linux amd64 | `sh scripts/package-linux.sh` | `build/autopack-linux-amd64/`、`build/autopack-linux-amd64.tar.gz` |
| Windows amd64 | `powershell -File scripts/package-windows.ps1` | `build/autopack-windows-amd64/`、`build/autopack-windows-amd64.zip` |

**依赖**：Go 1.25+、Node.js 20+、npm（无需 C 编译器 / CGO）。

发布包目录结构：

```text
autopack-{os}-amd64/
├── autopack(.exe)
├── configs/app.yaml  # 服务端口、数据库路径、JWT 等
├── web/              # frontend dist（index.html、assets/*）
├── start.sh / stop.sh / restart.sh / status.sh   # Linux 后台启停
├── start.bat / stop.bat / restart.bat / status.bat # Windows 后台启停
├── README.txt
├── scripts/sqlite/*.sql
├── data/             # 数据库与 autopack.pid
└── logs/
```

在解压目录内执行 `start.sh` 或 `start.bat`（后台运行），浏览器访问 `http://localhost:8080/autopack/`。前端产物位于发布包内的 **`web/`** 目录，由 `autopack` 进程代理访问（勿删除该目录）。

**打包注意**：Linux 脚本**每次**在 `frontend` 下执行 `npm ci`（勿从 Windows 拷贝 `node_modules`）。Windows 若 `npm ci` 报 EPERM，先停 `npm run dev`；有 `node_modules` 时会跳过 `npm ci` 以规避文件占用。

---

## 禁止事项

- 禁止在脚本中使用整型自增主键替代 `rec_id`。
- 禁止复制其他项目表名前缀（本工程统一 `AP_`）。
- 禁止在 `001_init_schema.sql` 中堆叠与当前阶段无关的多张业务表。
- 禁止在 SQL 中写明文密码；仅允许 `password_hash`。
- 禁止依赖 `FOREIGN KEY CASCADE` 做级联删除。
