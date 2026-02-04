#
# Croupier Server Windows 服务管理脚本
# 用法: .\scripts\install-windows-service.ps1 [Install|Uninstall|Start|Stop|Restart|Status]
#
# 环境变量 (可覆盖):
#   CROUPIER_SERVICE_NAME    - 服务名称 (默认: CroupierServer)
#   CROUPIER_DISPLAY_NAME    - 服务显示名称
#   CROUPIER_CONFIG_DIR      - 配置目录 (默认: C:\ProgramData\Croupier\config)
#   CROUPIER_BIN_PATH        - 二进制文件路径
#

[CmdletBinding()]
param(
    [Parameter(Position=0)]
    [ValidateSet('Install', 'Uninstall', 'Start', 'Stop', 'Restart', 'Status', 'Help')]
    [string]$Command = 'Help',

    [Parameter()]
    [string]$ServiceName = $env:CROUPIER_SERVICE_NAME,

    [Parameter()]
    [string]$DisplayName = $env:CROUPIER_DISPLAY_NAME,

    [Parameter()]
    [string]$ConfigDir = $env:CROUPIER_CONFIG_DIR,

    [Parameter()]
    [string]$BinPath = $env:CROUPIER_BIN_PATH
)

# 默认配置
if (-not $ServiceName) { $ServiceName = 'CroupierServer' }
if (-not $DisplayName) { $DisplayName = 'Croupier Server - Game Operations Control Plane' }
if (-not $ConfigDir) { $ConfigDir = 'C:\ProgramData\Croupier\config' }

$ServiceDescription = 'Croupier Server - 游戏运营控制面服务'

# 辅助函数
function Write-ColorOutput {
    param(
        [string]$Message,
        [string]$Color = 'White'
    )
    Write-Host $Message -ForegroundColor $Color
}

function Write-Info {
    param([string]$Message)
    Write-ColorOutput "[INFO] $Message" Cyan
}

function Write-Success {
    param([string]$Message)
    Write-ColorOutput "[OK] $Message" Green
}

function Write-Warn {
    param([string]$Message)
    Write-ColorOutput "[WARN] $Message" Yellow
}

function Write-Error {
    param([string]$Message)
    Write-ColorOutput "[ERROR] $Message" Red
}

function Test-Admin {
    $currentUser = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($currentUser)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function New-Directory {
    param([string]$Path)
    if (-not (Test-Path $Path)) {
        New-Item -Path $Path -ItemType Directory -Force | Out-Null
        Write-Info "创建目录: $Path"
    }
}

function Get-BinaryPath {
    if ($BinPath -and (Test-Path $BinPath)) {
        return $BinPath
    }

    # 检查 bin 目录
    $localBin = Join-Path $PSScriptRoot '..\bin\croupier-server.exe'
    if (Test-Path $localBin) {
        return $localBin
    }

    # 检查当前目录
    $currentBin = Join-Path $PSScriptRoot 'croupier-server.exe'
    if (Test-Path $currentBin) {
        return $currentBin
    }

    return $null
}

# 安装服务
function Install-Service {
    Write-Info "开始安装 Croupier Server Windows 服务"

    # 检查管理员权限
    if (-not (Test-Admin)) {
        Write-Error "此脚本需要管理员权限运行"
        Write-Info "请以管理员身份打开 PowerShell，然后运行此脚本"
        exit 1
    }

    # 获取二进制文件路径
    $binary = Get-BinaryPath
    if (-not $binary) {
        Write-Error "未找到 croupier-server.exe"
        Write-Info "请设置环境变量 CROUPIER_BIN_PATH 指向二进制文件路径"
        Write-Info "或将编译后的二进制文件放在 bin 目录下"
        exit 1
    }

    Write-Info "二进制文件: $binary"
    Write-Info "配置目录: $ConfigDir"

    # 创建配置目录
    New-Directory $ConfigDir

    # 创建配置文件（如果不存在）
    $configPath = Join-Path $ConfigDir 'server.yaml'
    if (-not (Test-Path $configPath)) {
        Write-Info "创建默认配置文件: $configPath"
        $logDir = Join-Path $PSScriptRoot '..\logs'
        $exampleConfig = @"
# Croupier Server 配置文件
# 请根据实际环境修改

server:
  addr: ":8443"
  http_addr: ":8080"

database:
  driver: postgres
  dsn: "postgres://croupier:password@localhost:5432/croupier?sslmode=disable"

jwt:
  secret: "change-me-to-random-string"

log:
  level: info
  format: json
  output: "file"
  file_path: "$logDir\croupier.log"
"@
        $exampleConfig | Out-File -FilePath $configPath -Encoding UTF8
    }

    # 构建 CLI 命令
    $installArgs = @(
        "service"
        "install"
        "--config", "`"$configPath`""
        "--name", "`"$ServiceName`""
        "--display-name", "`"$DisplayName`""
        "--description", "`"$ServiceDescription`""
        "--config-dir", "`"$ConfigDir`""
    )

    Write-Info "执行命令: $binary $($installArgs -join ' ')"

    $process = Start-Process -FilePath $binary -ArgumentList $installArgs -Wait -PassThru -NoNewWindow

    if ($process.ExitCode -eq 0) {
        Write-Success "服务 '$ServiceName' 安装成功"
        Write-Info ""
        Write-Info "后续操作:"
        Write-Info "  启动服务:   .\$PSCommandPath Start"
        Write-Info "  查看状态:   .\$PSCommandPath Status"
        Write-Info "  停止服务:   .\$PSCommandPath Stop"
        Write-Info "  卸载服务:   .\$PSCommandPath Uninstall"
    } else {
        Write-Error "服务安装失败，退出代码: $($process.ExitCode)"
        exit 1
    }
}

# 卸载服务
function Uninstall-Service {
    if (-not (Test-Admin)) {
        Write-Error "此脚本需要管理员权限运行"
        exit 1
    }

    Write-Info "卸载服务: $ServiceName"

    $binary = Get-BinaryPath
    if (-not $binary) {
        Write-Error "未找到 croupier-server.exe"
        exit 1
    }

    $uninstallArgs = @("service", "uninstall", "--name", "`"$ServiceName`"")

    $process = Start-Process -FilePath $binary -ArgumentList $uninstallArgs -Wait -PassThru -NoNewWindow

    if ($process.ExitCode -eq 0) {
        Write-Success "服务 '$ServiceName' 已卸载"
    } else {
        Write-Warn "卸载可能失败，退出代码: $($process.ExitCode)"
    }
}

# 启动服务
function Start-ServiceCmd {
    if (-not (Test-Admin)) {
        Write-Error "此脚本需要管理员权限运行"
        exit 1
    }

    Write-Info "启动服务: $ServiceName"

    $binary = Get-BinaryPath
    if (-not $binary) {
        Write-Error "未找到 croupier-server.exe"
        exit 1
    }

    $startArgs = @("service", "start", "--name", "`"$ServiceName`"")

    $process = Start-Process -FilePath $binary -ArgumentList $startArgs -Wait -PassThru -NoNewWindow

    if ($process.ExitCode -eq 0) {
        Write-Success "服务 '$ServiceName' 已启动"
    } else {
        Write-Error "启动失败，退出代码: $($process.ExitCode)"
        exit 1
    }
}

# 停止服务
function Stop-ServiceCmd {
    if (-not (Test-Admin)) {
        Write-Error "此脚本需要管理员权限运行"
        exit 1
    }

    Write-Info "停止服务: $ServiceName"

    $binary = Get-BinaryPath
    if (-not $binary) {
        Write-Error "未找到 croupier-server.exe"
        exit 1
    }

    $stopArgs = @("service", "stop", "--name", "`"$ServiceName`"")

    $process = Start-Process -FilePath $binary -ArgumentList $stopArgs -Wait -PassThru -NoNewWindow

    if ($process.ExitCode -eq 0) {
        Write-Success "服务 '$ServiceName' 已停止"
    } else {
        Write-Error "停止失败，退出代码: $($process.ExitCode)"
        exit 1
    }
}

# 重启服务
function Restart-ServiceCmd {
    if (-not (Test-Admin)) {
        Write-Error "此脚本需要管理员权限运行"
        exit 1
    }

    Write-Info "重启服务: $ServiceName"

    $binary = Get-BinaryPath
    if (-not $binary) {
        Write-Error "未找到 croupier-server.exe"
        exit 1
    }

    $restartArgs = @("service", "restart", "--name", "`"$ServiceName`"")

    $process = Start-Process -FilePath $binary -ArgumentList $restartArgs -Wait -PassThru -NoNewWindow

    if ($process.ExitCode -eq 0) {
        Write-Success "服务 '$ServiceName' 已重启"
    } else {
        Write-Error "重启失败，退出代码: $($process.ExitCode)"
        exit 1
    }
}

# 查看服务状态
function Show-ServiceStatus {
    $binary = Get-BinaryPath
    if ($binary) {
        $statusArgs = @("service", "status", "--name", "`"$ServiceName`"")

        $process = Start-Process -FilePath $binary -ArgumentList $statusArgs -Wait -PassThru -NoNewWindow -RedirectStandardOutput
        # Output is captured by the CLI
    } else {
        # Fallback to PowerShell Get-Service
        $service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
        if ($service) {
            Write-Host "服务名称: $($service.Name)" -ForegroundColor Cyan
            Write-Host "显示名称: $($service.DisplayName)" -ForegroundColor Cyan
            Write-Host "状态: " -NoNewline
            switch ($service.Status) {
                'Running' { Write-Host "$($service.Status)" -ForegroundColor Green }
                'Stopped' { Write-Host "$($service.Status)" -ForegroundColor Red }
                default { Write-Host "$($service.Status)" -ForegroundColor Yellow }
            }
            Write-Host "启动类型: $($service.StartType)"
        } else {
            Write-Warn "服务 '$ServiceName' 不存在"
        }
    }
}

# 显示帮助
function Show-Help {
    Write-Host @"
Croupier Server Windows 服务管理脚本

用法: .\$($MyInvocation.MyCommand.Name) <命令> [选项]

命令:
  Install     安装服务 (使用内置 CLI)
  Uninstall   卸载服务
  Start       启动服务
  Stop        停止服务
  Restart     重启服务
  Status      查看服务状态
  Help        显示此帮助信息

环境变量:
  CROUPIER_SERVICE_NAME   服务名称 (默认: CroupierServer)
  CROUPIER_DISPLAY_NAME   服务显示名称
  CROUPIER_CONFIG_DIR     配置目录 (默认: C:\ProgramData\Croupier\config)
  CROUPIER_BIN_PATH       二进制文件路径 (可选)

示例:
  # 安装服务
  .\$($MyInvocation.MyCommand.Name) Install

  # 指定二进制路径安装
  `$env:CROUPIER_BIN_PATH = 'C:\temp\croupier-server.exe'
  .\$($MyInvocation.MyCommand.Name) Install

  # 查看服务状态
  .\$($MyInvocation.MyCommand.Name) Status

  # 启动服务
  .\$($MyInvocation.MyCommand.Name) Start

  # 直接使用 CLI 命令
  C:\path\to\croupier-server.exe service install
  C:\path\to\croupier-server.exe service status

注意:
  - 此脚本需要管理员权限运行
  - 建议以管理员身份打开 PowerShell 执行
"@ -ForegroundColor Cyan
}

# 主入口
switch ($Command) {
    'Install' {
        Install-Service
    }
    'Uninstall' {
        Uninstall-Service
    }
    'Start' {
        Start-ServiceCmd
    }
    'Stop' {
        Stop-ServiceCmd
    }
    'Restart' {
        Restart-ServiceCmd
    }
    'Status' {
        Show-ServiceStatus
    }
    'Help' {
        Show-Help
    }
    default {
        Write-Error "未知命令: $Command"
        Show-Help
        exit 1
    }
}
