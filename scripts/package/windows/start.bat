@echo off
cd /d "%~dp0"
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0autopack-service.ps1" start
exit /b %ERRORLEVEL%
