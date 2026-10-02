<#
.SYNOPSIS
    Croupier Agent 一键安装脚本（Windows PowerShell 5.1+ / pwsh）

.DESCRIPTION
    从 GitHub Releases（匿名可下）下载对应架构的 croupier-agent 并安装到用户目录，
    自动注册用户 PATH，安装后运行 --version 校验。幂等：重复执行即覆盖升级。

    远程执行（默认装最新稳定版）：
        & ([scriptblock]::Create((irm https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.ps1)))

    带参数（或本地执行 .\scripts\install.ps1 ...）：
        -Version latest|nightly|vX.Y.Z   安装版本，默认 latest
        -BinDir <dir>                    安装目录，默认 %LOCALAPPDATA%\Programs\croupier\bin
        -WithService -ConfigPath <yaml>  注册 Windows 服务（需管理员；ConfigPath 指向 agent.yaml）
        -Uninstall                       卸载（移除服务与二进制，配置保留）

    环境变量（irm | iex 场景不便传参时）：
        CROUPIER_INSTALL_VERSION / CROUPIER_INSTALL_BIN_DIR / CROUPIER_INSTALL_WITH_SERVICE=1

.EXAMPLE
    & ([scriptblock]::Create((irm https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.ps1))) -Version nightly
#>
[CmdletBinding()]
param(
    [string]$Version = $(if ($env:CROUPIER_INSTALL_VERSION) { $env:CROUPIER_INSTALL_VERSION } else { 'latest' }),
    [string]$BinDir = $env:CROUPIER_INSTALL_BIN_DIR,
    [switch]$WithService,
    [string]$ConfigPath = $env:CROUPIER_INSTALL_CONFIG,
    [switch]$Uninstall
)

$ErrorActionPreference = 'Stop'
# Windows PowerShell 5.1 默认协议栈可能不含 TLS 1.2，显式启用
try { [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12 } catch {}

$Repo = 'cuihairu/croupier'
$ReleaseBase = "https://github.com/$Repo/releases"
$BinName = 'croupier-agent.exe'
$ServiceName = 'croupier-agent'

function Write-Info { param([string]$Message) Write-Host "[INFO] $Message" -ForegroundColor Green }
function Write-Warn2 { param([string]$Message) Write-Host "[WARN] $Message" -ForegroundColor Yellow }
function Die {
    param([string]$Message)
    Write-Host "[ERROR] $Message" -ForegroundColor Red
    exit 1
}

function Get-LatestTag {
    # 匿名 GitHub API（releases/latest 不含 prerelease，nightly 不会被选中）
    try {
        $rel = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing
        return $rel.tag_name
    } catch {
        Die "无法解析最新稳定版 tag（$($_.Exception.Message)）。可显式指定 -Version nightly 或 -Version vX.Y.Z"
    }
}

function Test-IsAdmin {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    (New-Object Security.Principal.WindowsPrincipal($identity)).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

# ---- 服务卸载 / 停启辅助 ----

function Remove-AgentService {
    $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($svc) {
        if ($svc.Status -ne 'Stopped') { Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue }
        sc.exe delete $ServiceName | Out-Null
        Write-Info "已移除 Windows 服务: $ServiceName"
    }
}

# ---- 卸载 ----

if ($Uninstall) {
    Remove-AgentService
    if (-not $BinDir) { $BinDir = Join-Path $env:LOCALAPPDATA 'Programs\croupier\bin' }
    $target = Join-Path $BinDir $BinName
    if (Test-Path $target) { Remove-Item $target -Force; Write-Info "已删除二进制: $target" }
    Write-Info "卸载完成（agent.yaml 配置保留，如需清理请手动删除）"
    exit 0
}

# ---- 平台识别 ----

$arch = $env:PROCESSOR_ARCHITECTURE
switch ($arch) {
    'AMD64' { $goarch = 'amd64' }
    'ARM64' { Die "当前发布面暂无 windows-arm64 产物（已发布: linux-amd64/arm64/armv7、darwin-amd64/arm64、windows-amd64）。可自行交叉构建: `$env:GOOS='windows'; `$env:GOARCH='arm64'; go build ./cmd/agent" }
    default { Die "不支持的 CPU 架构: $arch（已发布: linux-amd64/arm64/armv7、darwin-amd64/arm64、windows-amd64）" }
}

# ---- 解析版本与下载 ----

if ($Version -eq 'latest') {
    $tag = Get-LatestTag
    Write-Info "最新稳定版: $tag"
} else {
    $tag = $Version
}

$url = "$ReleaseBase/download/$tag/croupier-bin-windows-$goarch.zip"
Write-Info "目标产物: $url"

$temp = Join-Path ([IO.Path]::GetTempPath()) ("croupier-install-" + [Guid]::NewGuid().ToString('N').Substring(0, 8))
New-Item -ItemType Directory -Force -Path $temp | Out-Null
try {
    $zipPath = Join-Path $temp 'croupier-bin.zip'
    try {
        Invoke-WebRequest -Uri $url -OutFile $zipPath -UseBasicParsing
    } catch {
        Die "下载失败: $url`n$($_.Exception.Message)`n检查网络连通性，或该平台产物是否已发布: $ReleaseBase"
    }

    Expand-Archive -Path $zipPath -DestinationPath $temp -Force
    $agentSrc = Get-ChildItem -Path $temp -Recurse -Filter $BinName | Select-Object -First 1
    if (-not $agentSrc) { Die "归档中未找到 $BinName，产物内容异常" }

    # ---- 安装 ----

    if (-not $BinDir) { $BinDir = Join-Path $env:LOCALAPPDATA 'Programs\croupier\bin' }
    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
    $target = Join-Path $BinDir $BinName

    $oldVersion = $null
    if (Test-Path $target) {
        try { $oldVersion = (& $target --version 2>$null | Select-Object -First 1) } catch {}
    }

    Copy-Item -Path $agentSrc.FullName -Destination $target -Force

    # ---- 用户 PATH 注册 ----

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if ([string]::IsNullOrEmpty($userPath)) {
        [Environment]::SetEnvironmentVariable('Path', $BinDir, 'User')
        Write-Info "已设置用户 PATH: $BinDir（新开终端生效）"
    } elseif ($userPath -notlike "*$BinDir*") {
        [Environment]::SetEnvironmentVariable('Path', ($userPath.TrimEnd(';') + ';' + $BinDir), 'User')
        Write-Info "已加入用户 PATH: $BinDir（新开终端生效）"
    }
    if (($env:Path -split ';') -notcontains $BinDir) { $env:Path = $env:Path + ';' + $BinDir }

    # ---- 校验 ----

    try {
        $newVersion = (& $target --version 2>$null | Select-Object -First 1)
    } catch {
        Die "安装后校验失败: $target --version 未正常输出"
    }
    if (-not $newVersion) { Die "安装后校验失败: $target --version 输出为空" }

    if ($oldVersion -and $oldVersion -ne $newVersion) {
        Write-Info "已升级: $oldVersion -> $newVersion"
    } elseif ($oldVersion) {
        Write-Info "已重装（版本不变）: $newVersion"
    } else {
        Write-Info "已安装: $newVersion"
    }
    Write-Info "二进制位置: $target"

    # ---- 可选：注册 Windows 服务 ----

    if ($WithService) {
        if (-not (Test-IsAdmin)) { Die "-WithService 需要管理员 PowerShell。请以管理员重开并携带 -ConfigPath 参数重试" }
        if (-not $ConfigPath -or -not (Test-Path $ConfigPath)) {
            Die "-WithService 需要已有配置：-ConfigPath <agent.yaml 路径>`n配置模板见仓库 configs/agent.yaml，配置项详见 docs/operations/config-agent.md"
        }
        Remove-AgentService
        New-Service -Name $ServiceName -DisplayName 'Croupier Agent' `
            -BinaryPathName ('"{0}" --config "{1}"' -f $target, (Resolve-Path $ConfigPath).Path) `
            -StartupType Automatic | Out-Null
        Start-Service -Name $ServiceName
        Write-Info "服务已注册并启动: Get-Service $ServiceName"
    }

    Write-Info "完成。下一步：编辑 agent.yaml 连上 Server（上游地址见 docs/operations/config-agent.md）"
} finally {
    Remove-Item -Recurse -Force $temp -ErrorAction SilentlyContinue
}
