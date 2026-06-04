autopack - auto-pack release bundle
====================================

Contents:
  autopack(.exe)   API server
  web/             Frontend static files (dist); served by autopack at /autopack/
  configs/         app.yaml (server addr, database path, jwt, frontend path)
  scripts/sqlite/  Database schema (applied on first start)
  data/            SQLite database + autopack.pid (runtime)
  logs/            Application and service stdout logs

All HTTP routes use prefix /autopack (for reverse-proxy without trailing-slash issues).

Edit configs/app.yaml before deploy (server.addr, database.path, web.jwt_secret).
Optional: autopack(.exe) -config /path/to/configs

First start creates the database and default admin:
  username: admin
  password: 123456

Linux (background):
  chmod +x autopack *.sh
  ./start.sh      # start in background
  ./stop.sh       # stop
  ./restart.sh    # restart
  ./status.sh     # check pid

Windows (background):
  start.bat       # start in background
  stop.bat
  restart.bat
  status.bat

Open http://localhost:8080/autopack/ in your browser.
Keep the web/ folder next to the binary when deploying.

Packaging (from repo):
  Windows:  scripts\package-windows.ps1
  Linux:    scripts/package-linux.sh
  Tip: stop local "npm run dev" before Windows pack to avoid npm ci EPERM on node_modules.
