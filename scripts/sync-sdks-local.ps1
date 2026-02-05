# PowerShell SDK 同步脚本
# 用法: .\sync-sdks-local.ps1 [all|go|python|cpp|js|csharp]
#       .\sync-sdks-local.ps1 -DryRun    # dry-run 模式

param(
    [Parameter(Position=0)]
    [string[]]$SdkList = @("all"),

    [switch]$DryRun,

    [string]$BasePath = ".."
)

function Log-Info {
    param([string]$Message)
    Write-Host "[INFO] $Message" -ForegroundColor Cyan
}

function Log-Success {
    param([string]$Message)
    Write-Host "[SUCCESS] $Message" -ForegroundColor Green
}

function Log-Warn {
    param([string]$Message)
    Write-Host "[WARN] $Message" -ForegroundColor Yellow
}

function Log-Error {
    param([string]$Message)
    Write-Host "[ERROR] $Message" -ForegroundColor Red
}

# 检查 buf
if (-not (Get-Command buf -ErrorAction SilentlyContinue)) {
    Log-Error "buf 命令未找到，请安装 Buf"
    Log-Info "下载地址: https://docs.buf.build/installation"
    exit 1
}

# 获取脚本目录
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Split-Path -Parent $ScriptDir

Log-Info "=== Croupier SDK 本地同步脚本 ==="
Log-Info "基础路径: $BasePath"
Log-Info "预演模式: $DryRun"
Write-Host ""

# 解析 SDK 列表
$TargetSdks = @()
foreach ($item in $SdkList) {
    if ($item -eq "all") {
        $TargetSdks = @("go", "python", "cpp", "js", "csharp")
        break
    } elseif ($item -match ',') {
        $TargetSdks += $item -split ','
    } else {
        $TargetSdks += $item
    }
}

# SDK 配置
$SdkMapping = @{
    "go"     = @{ Repo = "croupier-sdk-go";     Name = "Go";         ProtoDir = "proto" }
    "python" = @{ Repo = "croupier-sdk-python"; Name = "Python";     ProtoDir = "proto" }
    "cpp"    = @{ Repo = "croupier-sdk-cpp";    Name = "C++";        ProtoDir = "proto" }
    "js"     = @{ Repo = "croupier-sdk-js";     Name = "JavaScript"; ProtoDir = "proto" }
    "csharp" = @{ Repo = "croupier-sdk-csharp"; Name = "C#";         ProtoDir = "proto" }
}

# 处理每个 SDK
foreach ($sdkId in $TargetSdks) {
    if (-not $SdkMapping.ContainsKey($sdkId)) {
        Log-Warn "未知的 SDK: $sdkId"
        continue
    }

    $sdk = $SdkMapping[$sdkId]
    $sdkPath = Join-Path $BasePath $sdk.Repo
    $sdkPath = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($sdkPath)

    if (-not (Test-Path $sdkPath)) {
        Log-Warn "跳过 $($sdk.Name): 路径不存在 ($sdkPath)"
        continue
    }

    Write-Host ""
    Log-Info "========================================"
    Log-Info "处理 $($sdk.Name) SDK"
    Log-Info "路径: $sdkPath"
    Write-Host ""

    if ($DryRun) {
        Log-Warn "[预演] 将复制 proto 文件并执行 buf generate"
        continue
    }

    Push-Location $sdkPath

    # 删除旧的 proto 目录
    Remove-Item -Path $sdk.ProtoDir -Recurse -Force -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Path $sdk.ProtoDir -Force | Out-Null

    # 复制 proto 文件
    Copy-Item -Path "$RepoRoot\proto\croupier" -Destination "$($sdk.ProtoDir)\" -Recurse

    # 复制 buf.yaml (如果存在)
    if (Test-Path "$RepoRoot\proto\buf.yaml") {
        Copy-Item -Path "$RepoRoot\proto\buf.yaml" -Destination "$($sdk.ProtoDir)\buf.yaml" -Force
    }

    # 执行 buf generate (使用 SDK 目录下的 buf.gen.yaml)
    Push-Location $sdk.ProtoDir
    buf generate
    Pop-Location

    # Go SDK 特殊处理: 修复 import 路径
    if ($sdkId -eq "go") {
        Get-ChildItem -Path "pkg\pb" -Filter "*.pb.go" -Recurse -ErrorAction SilentlyContinue | ForEach-Object {
            $content = Get-Content $_.FullName -Raw
            $content = $content -replace 'github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/', 'github.com/cuihairu/croupier/sdks/go/pkg/pb/'
            Set-Content -Path $_.FullName -Value $content -NoNewline
        }
        Get-ChildItem -Path "pkg" -Filter "*.go" -Recurse -ErrorAction SilentlyContinue | Where-Object { $_.FullName -notlike "*\pb\*" } | ForEach-Object {
            $content = Get-Content $_.FullName -Raw
            $content = $content -replace 'github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/', 'github.com/cuihairu/croupier/sdks/go/pkg/pb/'
            Set-Content -Path $_.FullName -Value $content -NoNewline
        }
    }

    Pop-Location
    Log-Success "$($sdk.Name) SDK 同步完成"
}

Write-Host ""
Log-Success "=== 所有同步操作完成 ==="

if ($DryRun) {
    Log-Warn "这是预演模式，没有实际修改文件"
}
