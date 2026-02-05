# PowerShell SDK 同步脚本
# 用法: .\sync-sdks-local.ps1 [all|go|python|cpp|js|csharp]
#       .\sync-sdks-local.ps1 -DryRun    # dry-run 模式

param(
    [Parameter(Position=0)]
    [string[]]$SdkList = @("all"),

    [switch]$DryRun,
    [switch]$SkipGenerate,

    [string]$BasePath = ".."
)

# 颜色输出函数
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

# 检查依赖
function Test-Command {
    param([string]$Name)
    $null = Get-Command $Name -ErrorVariable err -ErrorAction SilentlyContinue
    return -not $err
}

# 检查 buf
if (-not (Test-Command "buf")) {
    Log-Error "buf 命令未找到，请安装 Buf"
    Log-Info "下载地址: https://docs.buf.build/installation"
    exit 1
}

# 检查 bash（用于调用原始脚本）
$useBash = Test-Command "bash"

# SDK 配置
$AllSdks = @{
    "go" = @{ Repo = "croupier-sdk-go"; Name = "Go"; Sync = Sync-GoSdk }
    "python" = @{ Repo = "croupier-sdk-python"; Name = "Python"; Sync = Sync-PythonSdk }
    "cpp" = @{ Repo = "croupier-sdk-cpp"; Name = "C++"; Sync = Sync-CppSdk }
    "js" = @{ Repo = "croupier-sdk-js"; Name = "JavaScript"; Sync = Sync-JsSdk }
    "csharp" = @{ Repo = "croupier-sdk-csharp"; Name = "C#"; Sync = Sync-CsharpSdk }
}

# 获取脚本目录
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Split-Path -Parent $ScriptDir

Log-Info "=== Croupier SDK 本地同步脚本 (PowerShell) ==="
Log-Info "基础路径: $BasePath"
Log-Info "预演模式: $DryRun"
Log-Info "跳过生成: $SkipGenerate"
Write-Host ""

# 解析 SDK 列表
$TargetSdks = @()
foreach ($item in $SdkList) {
    if ($item -eq "all") {
        $TargetSdks = $AllSdks.Keys
        break
    } else {
        if ($item -contains ',') {
            $parts = $item -split ','
            $TargetSdks += $parts
        } else {
            $TargetSdks += $item
        }
    }
}

# 处理每个 SDK
foreach ($sdkId in $TargetSdks) {
    if (-not $AllSdks.ContainsKey($sdkId)) {
        Log-Warn "未知的 SDK: $sdkId"
        continue
    }

    $sdk = $AllSdks[$sdkId]
    $sdkPath = Join-Path $BasePath $sdk.Repo

    # 转换路径
    $sdkPath = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($sdkPath)

    if (-not (Test-Path $sdkPath)) {
        Log-Warn "跳过 $($sdk.Name): 路径不存在 ($sdkPath)"
        continue
    }

    Write-Host ""
    Log-Info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    Log-Info "处理 $($sdk.Name) SDK"
    Log-Info "路径: $sdkPath"
    Write-Host ""

    # 如果有 bash，使用原始脚本
    if ($useBash) {
        $args = @()
        if ($DryRun) { $args += "-d" }
        if ($SkipGenerate) { $args += "--skip-generate" }
        $args += $sdkId

        bash "$ScriptDir/sync-sdks-local.sh" $args
    } else {
        Log-Warn "bash 不可用，跳过 $sdkId"
    }
}

Write-Host ""
Log-Success "=== 所有同步操作完成 ==="

if ($DryRun) {
    Log-Warn "这是预演模式，没有实际修改文件"
}

# Go SDK 同步函数
function Sync-GoSdk {
    param([string]$Path)

    Log-Info "开始同步 Go SDK..."

    if ($DryRun) {
        Log-Warn "[预演] 将删除 proto/ 并重新生成"
        return
    }

    Push-Location $Path

    # 删除旧文件
    Remove-Item -Path "proto" -Recurse -Force -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Path "proto" -Force | Out-Null

    # 复制 proto 文件
    Copy-Item -Path "$RepoRoot\proto\croupier" -Destination "proto\" -Recurse

    # 删除旧的生成代码
    Remove-Item -Path "pkg\pb" -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item -Path "proto\pkg" -Recurse -Force -ErrorAction SilentlyContinue

    # 创建 buf.gen.yaml
    @"
version: v2
plugins:
  - remote: buf.build/protocolbuffers/go:v1.36.11
    out: pkg/pb
    opt:
      - paths=source_relative
"@ | Out-File -FilePath "proto\buf.gen.yaml" -Encoding utf8

    # 创建 buf.yaml
    @"
version: v2
modules:
  - path: .
deps:
  - buf.build/protocolbuffers/wellknowntypes:v25.1
"@ | Out-File -FilePath "proto\buf.yaml" -Encoding utf8

    if (-not $SkipGenerate) {
        Push-Location "proto"
        buf generate
        Pop-Location

        # 移动生成的代码
        if (Test-Path "proto\pkg\pb") {
            Move-Item -Path "proto\pkg\pb\*" -Destination "pkg\pb\" -Recurse -Force
            Remove-Item -Path "proto\pkg" -Recurse -Force
        }

        # 修复 import 路径
        Get-ChildItem -Path "pkg\pb" -Filter "*.pb.go" -Recurse | ForEach-Object {
            (Get-Content $_.FullName -Raw) -replace 'github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/', 'github.com/cuihairu/croupier/sdks/go/pkg/pb/' | Set-Content $_.FullName -NoNewline
        }

        # 修复 SDK 代码 import
        Get-ChildItem -Path "pkg" -Filter "*.go" -Recurse | Where-Object { $_.FullName -notlike "*\pb\*" } | ForEach-Object {
            (Get-Content $_.FullName -Raw) -replace 'github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/', 'github.com/cuihairu/croupier/sdks/go/pkg/pb/' | Set-Content $_.FullName -NoNewline
        }

        $count = (Get-ChildItem -Path "pkg\pb" -Filter "*.pb.go" -Recurse).Count
        Log-Success "生成的 Go 文件: $count"
    } else {
        Log-Warn "跳过 buf generate"
    }

    Pop-Location
}

# Python SDK 同步函数
function Sync-PythonSdk {
    param([string]$Path)

    Log-Info "开始同步 Python SDK..."

    if ($DryRun) {
        Log-Warn "[预演] 将删除 proto/ 并重新生成"
        return
    }

    Push-Location $Path

    Remove-Item -Path "proto" -Recurse -Force -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Path "proto" -Force | Out-Null

    Copy-Item -Path "$RepoRoot\proto\croupier" -Destination "proto\" -Recurse
    Remove-Item -Path "generated" -Recurse -Force -ErrorAction SilentlyContinue

    @"
version: v2
plugins:
  - remote: buf.build/protocolbuffers/python:v25.1
    out: generated
"@ | Out-File -FilePath "proto\buf.gen.yaml" -Encoding utf8

    @"
version: v2
modules:
  - path: .
deps:
  - buf.build/protocolbuffers/wellknowntypes:v25.1
"@ | Out-File -FilePath "proto\buf.yaml" -Encoding utf8

    if (-not $SkipGenerate) {
        Push-Location "proto"
        buf generate
        Pop-Location

        if (Test-Path "proto\generated") {
            if (Test-Path "generated") {
                Remove-Item -Path "generated" -Recurse -Force
            }
            Move-Item -Path "proto\generated" -Destination "generated"

            # 创建 __init__.py
            Get-ChildItem -Path "generated" -Directory -Recurse | ForEach-Object {
                New-Item -Path (Join-Path $_.FullName "__init__.py") -ItemType File -Force | Out-Null
            }
        }

        $count = (Get-ChildItem -Path "generated" -Filter "*.py" -Recurse -ErrorAction SilentlyContinue).Count
        Log-Success "生成的 Python 文件: $count"
    } else {
        Log-Warn "跳过 buf generate"
    }

    Pop-Location
}

# C++ SDK 同步函数
function Sync-CppSdk {
    param([string]$Path)

    Log-Info "开始同步 C++ SDK..."

    if ($DryRun) {
        Log-Warn "[预演] 将删除 proto/ 并重新生成"
        return
    }

    Push-Location $Path

    Remove-Item -Path "proto" -Recurse -Force -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Path "proto" -Force | Out-Null

    Copy-Item -Path "$RepoRoot\proto\croupier" -Destination "proto\" -Recurse
    Remove-Item -Path "generated" -Recurse -Force -ErrorAction SilentlyContinue

    @"
version: v2
plugins:
  - remote: buf.build/protocolbuffers/cpp:v25.1
    out: generated
"@ | Out-File -FilePath "proto\buf.gen.yaml" -Encoding utf8

    @"
version: v2
modules:
  - path: .
deps:
  - buf.build/protocolbuffers/wellknowntypes:v25.1
"@ | Out-File -FilePath "proto\buf.yaml" -Encoding utf8

    if (-not $SkipGenerate) {
        Push-Location "proto"
        buf generate
        Pop-Location

        if (Test-Path "proto\generated") {
            if (Test-Path "generated") {
                Remove-Item -Path "generated" -Recurse -Force
            }
            Move-Item -Path "proto\generated" -Destination "generated"
        }

        $count = (Get-ChildItem -Path "generated" -Filter "*.pb.*" -Recurse -ErrorAction SilentlyContinue).Count
        Log-Success "生成的 C++ 文件: $count"
    } else {
        Log-Warn "跳过 buf generate"
    }

    Pop-Location
}

# JavaScript SDK 同步函数
function Sync-JsSdk {
    param([string]$Path)

    Log-Info "开始同步 JavaScript SDK..."

    if ($DryRun) {
        Log-Warn "[预演] 将删除 proto/ 并重新生成"
        return
    }

    Push-Location $Path

    Remove-Item -Path "proto" -Recurse -Force -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Path "proto" -Force | Out-Null

    Copy-Item -Path "$RepoRoot\proto\croupier" -Destination "proto\" -Recurse
    Remove-Item -Path "src\gen" -Recurse -Force -ErrorAction SilentlyContinue

    # 添加 node_modules/.bin 到 PATH
    $env:Path = "$PWD\node_modules\.bin;$env:Path"

    @"
version: v2
plugins:
  - local: protoc-gen-es
    out: src/gen
    opt:
      - target=ts
"@ | Out-File -FilePath "proto\buf.gen.yaml" -Encoding utf8

    @"
version: v2
modules:
  - path: .
deps:
  - buf.build/protocolbuffers/wellknowntypes:v25.1
"@ | Out-File -FilePath "proto\buf.yaml" -Encoding utf8

    if (-not $SkipGenerate) {
        Push-Location "proto"
        try {
            buf generate
            $count = (Get-ChildItem -Path "..\src\gen" -Filter "*.ts" -Recurse -ErrorAction SilentlyContinue).Count
            Log-Success "生成的 TypeScript 文件: $count"
        } catch {
            Log-Warn "buf generate 失败 (可能需要先安装 protoc-gen-es: npm install)"
        }
        Pop-Location
    } else {
        Log-Warn "跳过 buf generate"
    }

    Pop-Location
}

# C# SDK 同步函数
function Sync-CsharpSdk {
    param([string]$Path)

    Log-Info "开始同步 C# SDK..."

    if ($DryRun) {
        Log-Warn "[预演] 将删除 proto/ 并重新生成"
        return
    }

    Push-Location $Path

    Remove-Item -Path "proto" -Recurse -Force -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Path "proto" -Force | Out-Null

    Copy-Item -Path "$RepoRoot\proto\croupier" -Destination "proto\" -Recurse
    Remove-Item -Path "generated" -Recurse -Force -ErrorAction SilentlyContinue

    @"
version: v2
plugins:
  - remote: buf.build/protocolbuffers/csharp:v25.1
    out: generated
"@ | Out-File -FilePath "proto\buf.gen.yaml" -Encoding utf8

    @"
version: v2
modules:
  - path: .
deps:
  - buf.build/protocolbuffers/wellknowntypes:v25.1
"@ | Out-File -FilePath "proto\buf.yaml" -Encoding utf8

    if (-not $SkipGenerate) {
        Push-Location "proto"
        buf generate
        Pop-Location

        if (Test-Path "proto\generated") {
            if (Test-Path "generated") {
                Remove-Item -Path "generated" -Recurse -Force
            }
            Move-Item -Path "proto\generated" -Destination "generated"
        }

        $count = (Get-ChildItem -Path "generated" -Filter "*.cs" -Recurse -ErrorAction SilentlyContinue).Count
        Log-Success "生成的 C# 文件: $count"
    } else {
        Log-Warn "跳过 buf generate"
    }

    Pop-Location
}
