@echo off
REM Wrapper deploy Windows — chạy từ cobo_iam_services\
cd /d "%~dp0"
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0deploy-dev.ps1" %*
exit /b %ERRORLEVEL%
