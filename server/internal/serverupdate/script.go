package serverupdate

import (
	"fmt"
	"os"
)

// unixScript 等本进程退出后换 bin/autopack，再用 restart.sh / start.sh 拉起。
func unixScript(root, dest, staged string) string {
	backup := backupPath(root)
	lock := lockPath(root)
	return fmt.Sprintf(`#!/bin/sh
ROOT=%q
DEST=%q
STAGED=%q
BACKUP=%q
LOCK=%q
PID=%d
LOG="$ROOT/data/server-update.log"
log() { echo "$(date '+%%Y-%%m-%%d %%H:%%M:%%S') $1" >>"$LOG"; }

sleep 2
i=0
while kill -0 "$PID" 2>/dev/null; do
  i=$((i+1))
  if [ "$i" -gt 150 ]; then
    kill -9 "$PID" 2>/dev/null || true
    break
  fi
  sleep 0.2
done
mkdir -p "$(dirname "$DEST")"
if [ -f "$DEST" ]; then
  mv "$DEST" "$DEST.bak" || { log "backup binary failed"; exit 1; }
fi
mv "$STAGED" "$DEST" || { log "install binary failed"; exit 1; }
chmod 755 "$DEST" 2>/dev/null || true

start_server() {
  if [ -x "$ROOT/restart.sh" ]; then
    "$ROOT/restart.sh" >>"$LOG" 2>&1
    return $?
  fi
  if [ -x "$ROOT/start.sh" ]; then
    "$ROOT/start.sh" >>"$LOG" 2>&1
    return $?
  fi
  log "no start/restart script"
  return 1
}

if start_server; then
  rm -f "$DEST.bak"
  rm -rf "$BACKUP" "$LOCK"
  rm -f "$0"
  log "upgraded"
  exit 0
fi
log "start failed; restoring"
if [ -f "$DEST.bak" ]; then
  mv "$DEST.bak" "$DEST" || true
  chmod 755 "$DEST" 2>/dev/null || true
fi
if [ -d "$BACKUP/web" ]; then
  rm -rf "$ROOT/web"
  cp -a "$BACKUP/web" "$ROOT/web" || true
fi
if [ -d "$BACKUP/script" ]; then
  rm -rf "$ROOT/script"
  cp -a "$BACKUP/script" "$ROOT/script" || true
fi
start_server || true
log "restored"
rm -f "$LOCK" "$0"
exit 1
`, root, dest, staged, backup, lock, os.Getpid())
}

// windowsScript 等进程退出后换 exe，再用 restart.bat / start.bat 拉起。
func windowsScript(root, dest, staged string) string {
	backup := backupPath(root)
	lock := lockPath(root)
	return fmt.Sprintf(`@echo off
setlocal
set "ROOT=%s"
set "DEST=%s"
set "STAGED=%s"
set "BACKUP=%s"
set "LOCK=%s"
set "LOG=%%ROOT%%\data\server-update.log"
powershell -NoProfile -Command "Start-Sleep -Seconds 2"
taskkill /F /IM autopack.exe >NUL 2>&1
powershell -NoProfile -Command "Start-Sleep -Seconds 2"
if not exist "%%ROOT%%\bin" mkdir "%%ROOT%%\bin" >NUL 2>&1
if exist "%%DEST%%" move /Y "%%DEST%%" "%%DEST%%.bak" >NUL
move /Y "%%STAGED%%" "%%DEST%%" >NUL
if errorlevel 1 goto restore
set "OK=0"
if exist "%%ROOT%%\restart.bat" (
  call "%%ROOT%%\restart.bat" >>"%%LOG%%" 2>&1
  if not errorlevel 1 set "OK=1"
)
if "%%OK%%"=="0" if exist "%%ROOT%%\start.bat" (
  call "%%ROOT%%\start.bat" >>"%%LOG%%" 2>&1
  if not errorlevel 1 set "OK=1"
)
if "%%OK%%"=="0" goto restore
powershell -NoProfile -Command "Start-Sleep -Seconds 2; if (-not (Get-Process -Name autopack -ErrorAction SilentlyContinue)) { exit 1 }"
if errorlevel 1 goto restore
echo upgraded>>"%%LOG%%"
if exist "%%DEST%%.bak" del /F /Q "%%DEST%%.bak" >NUL 2>&1
if exist "%%BACKUP%%" rmdir /S /Q "%%BACKUP%%" >NUL 2>&1
if exist "%%LOCK%%" del /F /Q "%%LOCK%%" >NUL 2>&1
del "%%~f0" >NUL 2>&1
exit /b 0
:restore
echo start failed; restoring>>"%%LOG%%"
if exist "%%DEST%%.bak" move /Y "%%DEST%%.bak" "%%DEST%%" >NUL
if exist "%%BACKUP%%\web" (
  if exist "%%ROOT%%\web" rmdir /S /Q "%%ROOT%%\web"
  xcopy /E /I /Y "%%BACKUP%%\web" "%%ROOT%%\web\" >NUL
)
if exist "%%BACKUP%%\script" (
  if exist "%%ROOT%%\script" rmdir /S /Q "%%ROOT%%\script"
  xcopy /E /I /Y "%%BACKUP%%\script" "%%ROOT%%\script\" >NUL
)
if exist "%%ROOT%%\start.bat" call "%%ROOT%%\start.bat" >>"%%LOG%%" 2>&1
echo restored>>"%%LOG%%"
if exist "%%LOCK%%" del /F /Q "%%LOCK%%" >NUL 2>&1
del "%%~f0" >NUL 2>&1
exit /b 1
`, root, dest, staged, backup, lock)
}
