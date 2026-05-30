param(
    [Parameter(Mandatory = $true)]
    [string]$VcpkgRoot,

    [Parameter(Mandatory = $true)]
    [string]$VcpkgCommit
)

$ErrorActionPreference = "Stop"

$bootstrapBat = Join-Path $VcpkgRoot "bootstrap-vcpkg.bat"
if (-not (Test-Path $bootstrapBat)) {
    if (Test-Path $VcpkgRoot) {
        Remove-Item -LiteralPath $VcpkgRoot -Recurse -Force
    }

    git clone https://github.com/microsoft/vcpkg.git $VcpkgRoot
}

git -C $VcpkgRoot fetch --depth 1 origin $VcpkgCommit
git -C $VcpkgRoot checkout --force FETCH_HEAD

$bootstrapBat = Join-Path $VcpkgRoot "bootstrap-vcpkg.bat"
& $bootstrapBat -disableMetrics

"VCPKG_ROOT=$VcpkgRoot" | Out-File -FilePath $env:GITHUB_ENV -Encoding utf8 -Append
"VCPKG_FORCE_SYSTEM_BINARIES=1" | Out-File -FilePath $env:GITHUB_ENV -Encoding utf8 -Append

foreach ($name in @("VCPKG_DEFAULT_TRIPLET", "VCPKG_BUILD_TYPE")) {
    $value = [Environment]::GetEnvironmentVariable($name)
    if (-not [string]::IsNullOrWhiteSpace($value)) {
        "$name=$value" | Out-File -FilePath $env:GITHUB_ENV -Encoding utf8 -Append
    }
}

$VcpkgRoot | Out-File -FilePath $env:GITHUB_PATH -Encoding utf8 -Append
