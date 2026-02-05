@echo off
REM Windows SDK 同步脚本 - 调用 bash 脚本
REM 用法: sync-sdks-local.bat [all^|go^|python^|cpp^|js^|csharp]
REM       sync-sdks-local.bat -d        # dry-run 模式

setlocal

REM 获取脚本所在目录
set "SCRIPT_DIR=%~dp0"
set "REPO_ROOT=%SCRIPT_DIR%.."

REM 检查是否在 Git Bash 环境中
where bash.exe >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] bash.exe 未找到，请安装 Git for Windows
    echo 下载地址: https://git-scm.com/download/win
    exit /b 1
)

REM 调用 bash 脚本
bash "%SCRIPT_DIR%sync-sdks-local.sh" %*

endlocal
