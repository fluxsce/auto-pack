# script

运维与库脚本。按职责分目录，不要把构建入口、启停脚本和 DDL 混在一层。

| 目录 | 只放什么 | 是否进发行包 |
|------|----------|--------------|
| `db/` | 三库 DDL（`init.sql` / `98_patch.sql` / `99_seed.sql`） | 是（`script/db/`） |
| `build/` | 全局构建入口：编前端、编后端、组装发行目录 | 否 |
| `run/` | 启停模板，构建时拷到发行包根目录 | 是（根上 `start` / `stop` / `restart`） |
| `package/` | 发行包说明与 `app.yaml.example`（`frontend.path: ./web`） | 是 |

日常开发仍用仓库根目录：

```powershell
pnpm dev
pnpm dev:web
pnpm dev:server
pnpm build
```

## 全局构建

在仓库根执行。产物只进 `build/dist/`（已 gitignore），不覆盖本机 `server/configs/app.yaml`。
干净 clone 没有 `node_modules` 时，脚本会先 `pnpm install`。`@niuma/ui` 从 npm 装 `niuma-ui`，不需要旁路仓库。

```powershell
# Windows 本机包（默认 windows/amd64）
powershell -NoProfile -ExecutionPolicy Bypass -File script/build/windows.ps1

# 交叉编 Linux
powershell -NoProfile -ExecutionPolicy Bypass -File script/build/windows.ps1 -OS linux -Arch amd64
```

```bash
# Linux 本机包（默认 linux/amd64）
bash script/build/linux.sh

# 交叉编 Windows
bash script/build/linux.sh --os windows --arch amd64
```

也可用 `pnpm package:windows` / `pnpm package:linux`。

GitHub Actions：只在发版时跑。推 `v*` 标签或手动跑 [`.github/workflows/pack.yml`](../.github/workflows/pack.yml)，Linux 与 Windows 各出一份发行包。

## 发行目录

```text
build/dist/auto-pack-{version}-{os}-{arch}.tar.gz | .zip   # 下载文件名带版本
  auto-pack/                                              # 解压后不带版本
    bin/autopack[.exe]
    web/
    configs/app.yaml.example
    script/db/{sqlite,mysql,postgres}/
    VERSION
    start / stop / restart
    README.txt
```

运行时目录（启动时创建，不打进压缩包）：`data/`（库、工作空间、作业日志）、`logs/`（进程日志）、`run/`（pid）。

不要把源码、`node_modules`、本机 DSN / 密钥打进包。首次把 `configs/app.yaml.example` 复制为 `app.yaml` 再改密钥。

```powershell
# 解压 auto-pack-2.0.1-windows-amd64.zip 后
cd auto-pack
.\start.bat
.\restart.bat
.\stop.bat
```

```bash
# 解压 auto-pack-2.0.1-linux-amd64.tar.gz 后
cd auto-pack
chmod +x start.sh stop.sh restart.sh bin/autopack
./start.sh
./restart.sh
./stop.sh
```

浏览器：http://127.0.0.1:8080/autopack/  
默认账号：`admin` / `123456`

初始化平台库仍可手工跑（示例，SQLite）：

```powershell
sqlite3 data/db/autopack.db ".read script/db/sqlite/init.sql"
```

进程启动时会自己执行对应引擎的 `init.sql` 与 `98_patch.sql`。
