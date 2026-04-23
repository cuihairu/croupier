@echo off
REM ========== 快捷构建 Release DLL ==========
REM 用法: build_release.bat

echo ========================================
echo  Building Croupier SDK (Release DLL)
echo ========================================

call build.bat release x64 sdk

echo.
echo [*] Done! Files are in: contrib\croupier\lib\
echo.
