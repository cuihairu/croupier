@echo off
setlocal enabledelayedexpansion

REM ========== Croupier C++ SDK 测试构建脚本 ==========

set "VCPKG_ROOT=C:\Users\cui\vcpkg"
set "VCPKG_TOOLCHAIN=%VCPKG_ROOT%\scripts\buildsystems\vcpkg.cmake"
set "VCPKG_TRIPLET=x64-windows"

echo ========================================
echo  Building Croupier C++ SDK Tests
echo ========================================
echo  vcpkg: %VCPKG_ROOT%
echo  Toolchain: %VCPKG_TOOLCHAIN%
echo ========================================

REM 检查 vcpkg
if not exist "%VCPKG_TOOLCHAIN%" (
    echo [ERROR] vcpkg not found at %VCPKG_ROOT%
    echo [INFO] Please install vcpkg from https://github.com/microsoft/vcpkg
    exit /b 1
)

echo.
echo [*] Configuring CMake for tests...

cmake -B build ^
    -G "Visual Studio 17 2022" ^
    -A x64 ^
    -DCMAKE_BUILD_TYPE=Debug ^
    -DCMAKE_TOOLCHAIN_FILE="%VCPKG_TOOLCHAIN%" ^
    -DVCPKG_TARGET_TRIPLET=%VCPKG_TRIPLET% ^
    -DBUILD_SHARED_LIBS=ON ^
    -DBUILD_STATIC_LIBS=OFF ^
    -DBUILD_EXAMPLES=OFF ^
    -DBUILD_TESTS=ON ^
    -DCROUPIER_USE_STATIC_RUNTIME=OFF

if errorlevel 1 (
    echo [ERROR] CMake configuration failed
    exit /b 1
)

echo.
echo [*] Building tests...

cmake --build build --config Debug --parallel

if errorlevel 1 (
    echo [ERROR] Build failed
    exit /b 1
)

echo.
echo ========================================
echo  Build Complete!
echo ========================================
echo  Test executable: build\debug\croupier-sdk-tests.exe
echo ========================================

endlocal
