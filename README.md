# auto-pack

自动打包工程：

- 前端：Vue 3 + Vite + Element Plus
- 后端：Go + SQLite（离线数据库）

## 目录结构

```
backend/
  cmd/                 # 程序入口
  configs/             # app.yaml 配置
  internal/
    app/               # 依赖装配与 HTTP 根 Handler
    config/            # 加载 YAML 配置
    db/                # SQLite 初始化与种子数据
    repository/        # 数据库访问（模型、仓储、种子数据）
    httpx/             # JSON 响应、CORS 等 HTTP 工具
    web/               # HTTP 层：auth、assets、handler、静态资源
      auth/            # 登录、JWT、鉴权
      assets/          # 前端产物目录常量（打包脚本用）
      handler/         # 按 API 模块拆分（login、me、password、username）
      middleware/      # 鉴权中间件
      static/          # 磁盘静态资源代理（发布包 web/ 目录）
frontend/
```

## 发布打包

在仓库根目录执行（输出到 `build/`，应用名 `autopack`）：

```bash
# Linux（需在 Linux 下执行）
bash scripts/package-linux.sh

# Windows（PowerShell）
powershell -File scripts/package-windows.ps1
```

详见 [scripts/README.md](scripts/README.md)。

## 启动

### 后端

```bash
cd backend
go run ./cmd
# 或指定配置目录：go run ./cmd -config ./configs
```

配置见 `backend/configs/app.yaml`：

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `server.addr` | `:8080` | 监听地址 |
| `database.path` | `./data/autopack.db` | SQLite 文件路径（相对应用根目录） |
| `web.jwt_secret` | 内置开发密钥 | JWT 签名密钥，生产环境务必修改 |
| `web.frontend.path` | 空 | 前端 dist；空则自动找 `web/` 或 `../frontend/dist` |

首次启动会自动创建数据库，并初始化默认管理员：

- 用户名：`admin`
- 密码：`123456`

### 前端

```bash
cd frontend
npm install
npm run dev
```

## 认证接口

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| POST | `/autopack/api/auth/login` | 登录，返回 JWT | 否 |
| GET | `/autopack/api/auth/me` | 当前用户信息 | Bearer Token |
| PUT | `/autopack/api/auth/password` | 修改密码 | Bearer Token |
| PUT | `/autopack/api/auth/username` | 修改用户名 | Bearer Token |

### 登录

```json
POST /autopack/api/auth/login
{ "username": "admin", "password": "123456" }
```

响应示例：

```json
{
  "token": "...",
  "expires_at": "2026-06-03T10:00:00+08:00",
  "user": { "id": 1, "username": "admin" }
}
```

后续请求在 Header 中携带：`Authorization: Bearer <token>`

### 修改密码

```json
PUT /autopack/api/auth/password
{ "old_password": "123456", "new_password": "newpass" }
```

### 修改用户名

```json
PUT /autopack/api/auth/username
{ "password": "123456", "new_username": "admin2" }
```

## 其他接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/autopack/api/health` | 健康检查 |
| GET/POST | `/autopack/api/tasks` | 打包任务（需登录） |

前端与静态资源统一前缀 **`/autopack`**（生产环境由后端从发布包 `web/` 目录代理，访问 http://localhost:8080/autopack/ ）。
