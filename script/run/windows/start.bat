@echo off
setlocal
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0script\run\service.ps1" -Action start -Home "%~dp0"
exit /b %ERRORLEVEL%
