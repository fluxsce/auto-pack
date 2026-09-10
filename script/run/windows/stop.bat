@echo off
setlocal
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0script\run\service.ps1" -Action stop -Home "%~dp0"
exit /b %ERRORLEVEL%
