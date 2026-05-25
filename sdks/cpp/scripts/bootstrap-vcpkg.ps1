param(
    [Parameter(Mandatory = $true)]
    [string]$VcpkgRoot,

    [Parameter(Mandatory = $true)]
    [string]$VcpkgCommit
)

$ErrorActionPreference = "Stop"

$bootstrapBat = Join-Path $VcpkgRoot "bootstrap-vcpkg.bat"
$gitDir = Join-Path $VcpkgRoot ".git"

# Re-clone if directory exists but is not a proper standalone git repo
# (e.g. partially initialized by git submodule with fetch-depth=1)
if (Test-Path $VcpkgRoot) {
    $isProperRepo = $false
    if (Test-Path $gitDir) {
        $gitType = (Get-Item $gitDir -Force).Attributes
        # .git should be a directory for a standalone clone, not a file (submodule pointer)
        if ($gitType -match "Directory") {
            $isProperRepo = $true
        }
    }
    if (-not $isProperRepo) {
        Write-Host "Removing incomplete vcpkg directory (submodule residue)..."
        Remove-Item -LiteralPath $VcpkgRoot -Recurse -Force
    } elseif (-not (Test-Path $bootstrapBat)) {
        Write-Host "Removing vcpkg directory without bootstrap script..."
        Remove-Item -LiteralPath $VcpkgRoot -Recurse -Force
    }
}

if (-not (Test-Path $VcpkgRoot)) {
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
