@echo off
setlocal enabledelayedexpansion

REM ========== Croupier C++ SDK 构建脚本 ==========
REM 用法: build.bat [配置] [平台] [目标]
REM   配置: debug | release (默认: debug)
REM   平台: x64 (默认: x64)
REM   目标: sdk | all | clean (默认: sdk)

set "BUILD_CONFIG=%~1"
if "%BUILD_CONFIG%"=="" set "BUILD_CONFIG=debug"

set "BUILD_PLATFORM=%~2"
if "%BUILD_PLATFORM%"=="" set "BUILD_PLATFORM=x64"

set "BUILD_TARGET=%~3"
if "%BUILD_TARGET%"=="" set "BUILD_TARGET=sdk"

REM 转换为 CMake 格式
if /i "%BUILD_CONFIG%"=="debug" (
    set "CMAKE_BUILD_TYPE=Debug"
    set "BUILD_CONFIG_UC=Debug"
) else (
    set "CMAKE_BUILD_TYPE=Release"
    set "BUILD_CONFIG_UC=Release"
)

echo ========================================
echo  Croupier C++ SDK Build Script
echo ========================================
echo  Configuration: %BUILD_CONFIG_UC%
echo  Platform: %BUILD_PLATFORM%
echo  Target: %BUILD_TARGET%
echo ========================================

REM 检查 vcpkg
if not exist "C:\Users\Administrator\vcpkg\vcpkg.exe" (
    echo [ERROR] vcpkg not found at C:\Users\Administrator\vcpkg
    exit /b 1
)

REM 清理目标
if /i "%BUILD_TARGET%"=="clean" (
    echo.
    echo [*] Cleaning build directory...
    if exist build rd /s /q build
    if exist install rd /s /q install
    echo [OK] Clean complete
    exit /b 0
)

REM 配置构建选项
set "VCPKG_TOOLCHAIN=C:\Users\Administrator\vcpkg\scripts\buildsystems\vcpkg.cmake"
set "VCPKG_TRIPLET=x64-windows"
set "BUILD_SHARED=ON"
set "BUILD_STATIC=OFF"
set "RUNTIME=OFF"

REM 根据配置调整选项
if /i "%BUILD_CONFIG%"=="debug" (
    set "BUILD_OUTPUT=debug"
) else (
    set "BUILD_OUTPUT=release"
)

echo.
echo [*] Configuring CMake...

cmake -B build ^
    -G "Visual Studio 17 2022" ^
    -A %BUILD_PLATFORM% ^
    -DCMAKE_BUILD_TYPE=%CMAKE_BUILD_TYPE% ^
    -DCMAKE_TOOLCHAIN_FILE="%VCPKG_TOOLCHAIN%" ^
    -DVCPKG_TARGET_TRIPLET=%VCPKG_TRIPLET% ^
    -DBUILD_SHARED_LIBS=%BUILD_SHARED% ^
    -DBUILD_STATIC_LIBS=%BUILD_STATIC% ^
    -DBUILD_EXAMPLES=OFF ^
    -DBUILD_TESTS=OFF ^
    -DCROUPIER_USE_STATIC_RUNTIME=%RUNTIME%

if errorlevel 1 (
    echo [ERROR] CMake configuration failed
    exit /b 1
)

echo.
echo [*] Building %BUILD_CONFIG_UC%...

cmake --build build --config %BUILD_CONFIG_UC% --parallel croupier-sdk-shared

if errorlevel 1 (
    echo [ERROR] Build failed
    exit /b 1
)

echo.
echo [*] Copying output files...

REM 创建输出目录
if not exist "contrib\croupier\include" mkdir "contrib\croupier\include"
if not exist "contrib\croupier\debug\lib" mkdir "contrib\croupier\debug\lib"
if not exist "contrib\croupier\debug\bin" mkdir "contrib\croupier\debug\bin"

REM 复制头文件
xcopy /y /i /q include\croupier contrib\croupier\include\ >nul

REM 复制生成的 proto 头文件
if exist "build\include\croupier" (
    xcopy /y /i /q /e build\include\croupier contrib\croupier\include\ >nul
)

REM 复制库文件和 DLL
if /i "%BUILD_CONFIG%"=="debug" (
    if exist "build\debug\lib\croupier-sdk.lib" (
        copy /y "build\debug\lib\croupier-sdk.lib" "contrib\croupier\debug\lib\" >nul
    )
    if exist "build\debug\bin\croupier-sdk.dll" (
        copy /y "build\debug\bin\croupier-sdk.dll" "contrib\croupier\debug\bin\" >nul
    )
) else (
    if exist "build\lib\croupier-sdk.lib" (
        copy /y "build\lib\croupier-sdk.lib" "contrib\croupier\lib\" >nul
    )
    if exist "build\bin\croupier-sdk.dll" (
        copy /y "build\bin\croupier-sdk.dll" "contrib\croupier\bin\" >nul
    )
)

echo.
echo ========================================
echo  Build Complete!
echo ========================================
echo  Output: contrib\croupier\%BUILD_OUTPUT%\
if /i "%BUILD_CONFIG%"=="debug" (
    echo    - croupier-sdk.lib  ^(import library^)
    echo    - croupier-sdk.dll ^(runtime library^)
) else (
    echo    - croupier-sdk.lib  ^(import library^)
    echo    - croupier-sdk.dll ^(runtime library^)
)
echo    - include/croupier/ ^(headers^)
echo ========================================

REM 复制依赖 DLL 到输出目录
echo.
echo [*] Copying dependency DLLs...
if exist "build\vcpkg_installed\x64-windows\debug\bin\*.dll" (
    copy /y "build\vcpkg_installed\x64-windows\debug\bin\*.dll" "contrib\croupier\debug\bin\" >nul 2>&1
)

echo [OK] All dependency DLLs copied
echo.
echo Total files in contrib\croupier\debug\bin\:
dir /b "contrib\croupier\debug\bin\*.dll" 2>nul | find /c /v ""
echo.

endlocal
