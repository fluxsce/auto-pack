auto-pack 控制台发行目录
========================

bin/        服务进程（autopack / autopack.exe）
web/        控制台静态页，地址 /autopack/
configs/    app.yaml.example；首次复制为 app.yaml 后改密钥与 DSN
script/db/  启动迁移（sqlite / mysql / postgres）
start / stop / restart
            启停脚本（Windows 为 .bat，Linux 为 .sh）

运行时（启动时自动创建，勿当制品提交）：
  data/     平台库、工作空间检出、作业日志
  logs/     进程日志（stdout/stderr 与 app.yaml 的 log.file）
  run/      pid

首次
----
1. 复制 configs/app.yaml.example 为 configs/app.yaml
2. 改 web.jwt_secret、crypt.passphrase；生产勿用示例值
3. 需要 PostgreSQL / MySQL 时改 database.driver 与 dsn
4. Windows：start.bat
   Linux：chmod +x start.sh stop.sh restart.sh bin/autopack && ./start.sh

浏览器 http://127.0.0.1:8080/autopack/
默认账号 admin / 123456

应用根是 configs 的上一级。相对路径（data、logs、web）都相对应用根。
静态页路径已写成 ./web。启动请用本目录的 start 脚本，以便传入 -config configs。
