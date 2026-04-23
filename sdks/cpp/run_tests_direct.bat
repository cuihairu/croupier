@echo off
setlocal enabledelayedexpansion

cd /d D:\croupier\croupier-sdk-cpp\build\Debug

REM Add vcpkg DLLs to PATH
set "PATH=D:\croupier\croupier-sdk-cpp\build\vcpkg_installed\x64-windows\debug\bin;%PATH%"

echo ========================================
echo  Running Croupier C++ SDK Tests
echo ========================================
echo.

REM Run tests
croupier-sdk-tests.exe --gtest_output=xml:../../test_results.xml

if errorlevel 1 (
    echo.
    echo [FAILED] Tests failed with exit code %errorlevel%
    exit /b %errorlevel%
)

echo.
echo ========================================
echo  All tests passed!
echo ========================================

endlocal
