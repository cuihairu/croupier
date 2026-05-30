@echo off
setlocal enabledelayedexpansion

REM ========== Croupier C++ SDK 测试运行脚本 ==========

set "VCPKG_ROOT=C:\Users\cui\vcpkg"
set "BUILD_DIR=build"

echo ========================================
echo  Running Croupier C++ SDK Tests
echo ========================================

REM 检查测试可执行文件是否存在
if not exist "%BUILD_DIR%\debug\croupier-sdk-tests.exe" (
    echo [ERROR] Test executable not found
    echo [*] Expected: %BUILD_DIR%\debug\croupier-sdk-tests.exe
    echo [*] Please build tests first using: cmake -B build -DBUILD_TESTS=ON
    exit /b 1
)

echo.
echo [*] Running tests...
echo.

REM 设置 PATH 包含依赖 DLL
set "PATH=%BUILD_DIR%\vcpkg_installed\x64-windows\debug\bin;%PATH%"

REM 运行测试
"%BUILD_DIR%\debug\croupier-sdk-tests.exe" --gtest_output=xml:test_results.xml

if errorlevel 1 (
    echo.
    echo [FAILED] Tests failed
    exit /b 1
)

echo.
echo ========================================
echo  All tests passed!
echo ========================================

endlocal
