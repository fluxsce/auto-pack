# auto-pack 1.0.0（试用版，已冻结）

本目录是 **v1.0.0** 试用基线：Vue 3 + Element Plus、Go + 单 SQLite、JWT 无角色。

- Git 附注 tag：`v1.0.0`（指向冻结前的仓库根布局）。
- 主线 2.0 在仓库根的 `server/`、`src/`、`script/db/`。
- **只修致命 bug，不接新需求。** 补丁从 tag 开 `release/v1`。

## 怎么跑

```bash
# 后端
cd backend
go run ./cmd

# 前端
cd frontend
npm install
npm run dev
```

默认管理员：`admin` / `123456`。控制台版本常量见 `backend/pkg/config.AppVersion`。

## 对照 2.0 时可以看的算法

| 包 | 用途 |
|----|------|
| `backend/pkg/git` | clone / fetch / tag / worktree |
| `backend/pkg/ftpclient` `sftpclient` `sshclient` | 远端传输 |
| `backend/pkg/maven` `nodejs` | 工具链 |
| `backend/pkg/projectrelease` | 发版 / POM / Version.txt（2.0 不整包搬 coordinator） |
| `backend/pkg/packrun` | 旧构建与 cron（2.0 改为作业入队） |
| `backend/pkg/crypt` | AES-GCM `APENC1:` 密文格式 |

产品线 Tag（`V9.0.0-P01`）只描述被打包工程，不是本控制台 SemVer。
