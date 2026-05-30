# Croupier C++ SDK Build Script for Windows
# PowerShell script for building on Windows with Visual Studio

param(
    [string]$BuildType = "Release",
    [string]$Platform = "x64",
    [string]$VcpkgRoot = $env:VCPKG_ROOT,
    [string]$InstallPrefix = "",
    [switch]$Clean = $false,
    [string]$Examples = "ON",
    [string]$Tests = "OFF",
    [string]$Grpc = "ON",
    [string]$Vcpkg = "ON",
    [switch]$Help = $false
)

# ========== Help Function ==========
function Show-Help {
    Write-Host @"
Croupier C++ SDK Build Script for Windows

USAGE:
    .\build.ps1 [OPTIONS]

OPTIONS:
    -BuildType TYPE         Build type: Debug, Release, RelWithDebInfo (default: Release)
    -Platform PLATFORM      Target platform: x64, x86, arm64 (default: x64)
    -VcpkgRoot PATH         Path to vcpkg installation
    -InstallPrefix PATH     Installation prefix
    -Clean                  Clean build (remove build directory)
    -Examples BOOL          Build examples (default: ON)
    -Tests BOOL             Build tests (default: OFF)
    -Grpc BOOL              Enable gRPC integration (default: ON)
    -Vcpkg BOOL             Enable vcpkg integration (default: ON)
    -Help                   Show this help message

EXAMPLES:
    # Basic build
    .\build.ps1

    # Clean build with tests
    .\build.ps1 -Clean -Tests ON

    # x86 build with custom vcpkg
    .\build.ps1 -Platform x86 -VcpkgRoot "C:\vcpkg"

    # Debug build
    .\build.ps1 -BuildType Debug

ENVIRONMENT VARIABLES:
    VCPKG_ROOT             Path to vcpkg installation
"@
}

# ========== Main Script ==========
if ($Help) {
    Show-Help
    exit 0
}

# Script paths
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$SdkRoot = Split-Path -Parent $ScriptDir
$ProjectRoot = Split-Path -Parent (Split-Path -Parent $SdkRoot)

Write-Host "🚀 Croupier C++ SDK Build Script for Windows" -ForegroundColor Green
Write-Host "=============================================" -ForegroundColor Green

# ========== Platform Detection ==========
if (-not $Platform) {
    $Platform = "x64"
}

$VcpkgTriplet = switch ($Platform) {
    "x64" { "x64-windows" }
    "x86" { "x86-windows" }
    "arm64" { "arm64-windows" }
    default { "x64-windows" }
}

Write-Host "Platform: $Platform ($VcpkgTriplet)" -ForegroundColor Cyan

# ========== vcpkg Setup ==========
if ($Vcpkg -eq "ON") {
    if (-not $VcpkgRoot -and $env:VCPKG_ROOT) {
        $VcpkgRoot = $env:VCPKG_ROOT
    }

    if (-not $VcpkgRoot) {
        # Try common locations
        $CommonPaths = @(
            "C:\vcpkg",
            "C:\dev\vcpkg",
            "C:\tools\vcpkg",
            "$env:USERPROFILE\vcpkg"
        )

        foreach ($path in $CommonPaths) {
            if (Test-Path "$path\vcpkg.exe") {
                $VcpkgRoot = $path
                break
            }
        }
    }

    if ($VcpkgRoot -and (Test-Path "$VcpkgRoot\vcpkg.exe")) {
        Write-Host "Using vcpkg from: $VcpkgRoot" -ForegroundColor Green
        $VcpkgToolchain = "$VcpkgRoot\scripts\buildsystems\vcpkg.cmake"
    } else {
        Write-Warning "vcpkg not found. Set VCPKG_ROOT environment variable or use -VcpkgRoot"
        Write-Warning "Falling back to system packages..."
        $Vcpkg = "OFF"
    }
}

# ========== Build Directory ==========
$BuildDir = "$SdkRoot\build"

if ($Clean -and (Test-Path $BuildDir)) {
    Write-Host "Cleaning build directory..." -ForegroundColor Yellow
    Remove-Item -Recurse -Force $BuildDir
}

if (-not (Test-Path $BuildDir)) {
    New-Item -ItemType Directory -Path $BuildDir | Out-Null
}

Write-Host "Build directory: $BuildDir" -ForegroundColor Cyan

# ========== CMake Configuration ==========
$CmakeArgs = @(
    "-S", $SdkRoot
    "-B", $BuildDir
    "-A", $Platform
    "-DCMAKE_BUILD_TYPE=$BuildType"
)

# Install prefix
if ($InstallPrefix) {
    $CmakeArgs += "-DCMAKE_INSTALL_PREFIX=$InstallPrefix"
} else {
    $CmakeArgs += "-DCMAKE_INSTALL_PREFIX=$BuildDir\install"
}

# vcpkg configuration
if ($Vcpkg -eq "ON" -and $VcpkgToolchain) {
    $CmakeArgs += "-DCMAKE_TOOLCHAIN_FILE=$VcpkgToolchain"
    $CmakeArgs += "-DVCPKG_TARGET_TRIPLET=$VcpkgTriplet"
}

# Build options
$CmakeArgs += "-DENABLE_VCPKG=$Vcpkg"
$CmakeArgs += "-DENABLE_GRPC=$Grpc"
$CmakeArgs += "-DBUILD_EXAMPLES=$Examples"
$CmakeArgs += "-DBUILD_TESTS=$Tests"
$CmakeArgs += "-DBUILD_SHARED_LIBS=ON"
$CmakeArgs += "-DBUILD_STATIC_LIBS=ON"

Write-Host ""
Write-Host "Configuration:" -ForegroundColor Cyan
Write-Host "  Platform: $Platform" -ForegroundColor White
Write-Host "  Build Type: $BuildType" -ForegroundColor White
Write-Host "  vcpkg: $Vcpkg" -ForegroundColor White
Write-Host "  gRPC: $Grpc" -ForegroundColor White
Write-Host "  Examples: $Examples" -ForegroundColor White
Write-Host "  Tests: $Tests" -ForegroundColor White
Write-Host ""

Write-Host "Configuring CMake..." -ForegroundColor Yellow
Write-Host "cmake $($CmakeArgs -join ' ')" -ForegroundColor Gray

try {
    & cmake $CmakeArgs
    if ($LASTEXITCODE -ne 0) {
        throw "CMake configuration failed"
    }
} catch {
    Write-Error "❌ CMake configuration failed: $($_.Exception.Message)"
    exit 1
}

# ========== Build ==========
Write-Host ""
Write-Host "Building..." -ForegroundColor Yellow

$ParallelJobs = $env:NUMBER_OF_PROCESSORS
if (-not $ParallelJobs) { $ParallelJobs = 4 }

Write-Host "Using $ParallelJobs parallel jobs" -ForegroundColor Gray

try {
    & cmake --build $BuildDir --config $BuildType --parallel $ParallelJobs
    if ($LASTEXITCODE -ne 0) {
        throw "Build failed"
    }
    Write-Host "✅ Build completed successfully!" -ForegroundColor Green
} catch {
    Write-Error "❌ Build failed: $($_.Exception.Message)"
    exit 1
}

# ========== Run Tests ==========
if ($Tests -eq "ON") {
    Write-Host ""
    Write-Host "Running tests..." -ForegroundColor Yellow

    try {
        Set-Location $BuildDir
        & ctest --build-config $BuildType --output-on-failure
        if ($LASTEXITCODE -ne 0) {
            throw "Tests failed"
        }
        Write-Host "✅ All tests passed!" -ForegroundColor Green
    } catch {
        Write-Error "❌ Some tests failed: $($_.Exception.Message)"
        exit 1
    } finally {
        Set-Location $SdkRoot
    }
}

# ========== Create Packages ==========
Write-Host ""
Write-Host "Creating packages..." -ForegroundColor Yellow

try {
    # Build packages
    & cmake --build $BuildDir --config $BuildType --target package
    if ($LASTEXITCODE -ne 0) {
        throw "Package creation failed"
    }

    # Install to staging directory
    & cmake --install $BuildDir --prefix "$BuildDir\package"
    if ($LASTEXITCODE -ne 0) {
        throw "Installation failed"
    }

    # Create additional ZIP archive
    $PackageName = "croupier-cpp-sdk-$(Get-Date -Format 'yyyyMMdd')-windows-$Platform"
    $ZipPath = "$BuildDir\$PackageName.zip"

    Compress-Archive -Path "$BuildDir\package\*" -DestinationPath $ZipPath -Force

    Write-Host "✅ Packages created in $BuildDir\" -ForegroundColor Green
} catch {
    Write-Error "❌ Package creation failed: $($_.Exception.Message)"
    exit 1
}

# ========== Summary ==========
Write-Host ""
Write-Host "🎉 Build completed successfully!" -ForegroundColor Green
Write-Host "📦 Packages available in: $BuildDir\" -ForegroundColor Cyan

Write-Host ""
Write-Host "📖 Next steps:" -ForegroundColor Cyan
Write-Host "  • Run examples: $BuildDir\bin\$BuildType\virtual-object-demo.exe" -ForegroundColor White
Write-Host "  • Install SDK: cmake --install $BuildDir" -ForegroundColor White
Write-Host "  • Package distribution: $BuildDir\*.zip" -ForegroundColor White