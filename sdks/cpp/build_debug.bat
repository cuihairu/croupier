@echo off
REM ========== 快捷构建 Debug DLL ==========
REM 用法: build_debug.bat

echo ========================================
echo  Building Croupier SDK (Debug DLL)
echo ========================================

call build.bat debug x64 sdk

echo.
echo [*] Done! Files are in: contrib\croupier\debug\
echo.
echo [*] Copying to game server output...

xcopy /y "contrib\croupier\debug\bin\*.dll" "E:\Workspaces\server\trunk\output\Debug\GameServer\" /I >nul 2>&1
xcopy /y "contrib\croupier\debug\bin\*.dll" "E:\Workspaces\server\trunk\output\Debug\WorldServer\" /I >nul 2>&1
xcopy /y "contrib\croupier\debug\bin\*.dll" "E:\Workspaces\server\trunk\output\Debug\Mir666\" /I >nul 2>&1

echo [OK] DLLs copied to game servers
echo.
