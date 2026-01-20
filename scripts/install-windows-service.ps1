#
# Croupier Server Windows 服务安装脚本
# 用法: .\scripts\install-windows-service.ps1 [Install|Uninstall|Reinstall|Start|Stop|Restart|Status]
#
# 环境变量 (可覆盖):
#   CROUPIER_SERVICE_NAME    - 服务名称 (默认: CroupierServer)
#   CROUPIER_DISPLAY_NAME    - 服务显示名称 (默认: Croupier Server)
#   CROUPIER_INSTALL_DIR     - 安装目录 (默认: C:\Program Files\Croupier)
#   CROUPIER_DATA_DIR        - 数据目录 (默认: C:\ProgramData\Croupier)
#   CROUPIER_LOG_DIR         - 日志目录 (默认: C:\ProgramData\Croupier\logs)
#   CROUPIER_CONFIG_DIR      - 配置目录 (默认: C:\ProgramData\Croupier\config)
#   CROUPIER_BIN_PATH        - 二进制文件路径 (用于安装时复制)
#

[CmdletBinding()]
param(
    [Parameter(Position=0)]
    [ValidateSet('Install', 'Uninstall', 'Reinstall', 'Start', 'Stop', 'Restart', 'Status', 'Help', 'Enable', 'Disable')]
    [string]$Command = 'Help',

    [Parameter()]
    [string]$ServiceName = $env:CROUPIER_SERVICE_NAME,

    [Parameter()]
    [string]$DisplayName = $env:CROUPIER_DISPLAY_NAME,

    [Parameter()]
    [string]$InstallDir = $env:CROUPIER_INSTALL_DIR,

    [Parameter()]
    [string]$DataDir = $env:CROUPIER_DATA_DIR,

    [Parameter()]
    [string]$LogDir = $env:CROUPIER_LOG_DIR,

    [Parameter()]
    [string]$ConfigDir = $env:CROUPIER_CONFIG_DIR,

    [Parameter()]
    [string]$BinPath = $env:CROUPIER_BIN_PATH
)

# 默认配置
if (-not $ServiceName) { $ServiceName = 'CroupierServer' }
if (-not $DisplayName) { $DisplayName = 'Croupier Server - Game Operations Control Plane' }
if (-not $InstallDir) { $InstallDir = 'C:\Program Files\Croupier' }
if (-not $DataDir) { $DataDir = 'C:\ProgramData\Croupier' }
if (-not $LogDir) { $LogDir = 'C:\ProgramData\Croupier\logs' }
if (-not $ConfigDir) { $ConfigDir = 'C:\ProgramData\Croupier\config' }

$BinaryName = 'croupier-server.exe'
$BinaryPath = Join-Path $InstallDir $BinaryName
$ConfigPath = Join-Path $ConfigDir 'server.yaml'

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

function Stop-ServiceSafely {
    param([string]$Name)
    $service = Get-Service -Name $Name -ErrorAction SilentlyContinue
    if ($service -and $service.Status -eq 'Running') {
        Write-Info "停止服务: $Name"
        Stop-Service -Name $Name -Force
        Start-Sleep -Seconds 2
    }
}

# 安装服务
function Install-Service {
    Write-Info "开始安装 Croupier Server Windows 服务"
    Write-Info "安装目录: $InstallDir"
    Write-Info "配置目录: $ConfigDir"
    Write-Info "数据目录: $DataDir"
    Write-Info "日志目录: $LogDir"

    # 检查管理员权限
    if (-not (Test-Admin)) {
        Write-Error "此脚本需要管理员权限运行"
        Write-Info "请以管理员身份打开 PowerShell，然后运行此脚本"
        exit 1
    }

    # 检查服务是否已存在
    $existingService = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($existingService) {
        Write-Warn "服务 '$ServiceName' 已存在"
        $response = Read-Host "是否要重新安装? (y/N)"
        if ($response -eq 'y' -or $response -eq 'Y') {
            Uninstall-ServiceInternal
        } else {
            Write-Info "安装已取消"
            exit 0
        }
    }

    # 创建目录
    Write-Info "创建目录结构..."
    New-Directory $InstallDir
    New-Directory $ConfigDir
    New-Directory $DataDir
    New-Directory $LogDir

    # 复制二进制文件
    Write-Info "安装二进制文件..."
    if ($BinPath -and (Test-Path $BinPath)) {
        Copy-Item -Path $BinPath -Destination $BinaryPath -Force
        Write-Success "二进制文件已复制到 $BinaryPath"
    } else {
        # 检查当前目录的 bin 文件夹
        $localBin = Join-Path $PSScriptRoot '..\bin' $BinaryName
        if (Test-Path $localBin) {
            Copy-Item -Path $localBin -Destination $BinaryPath -Force
            Write-Success "二进制文件已从开发目录复制"
        } else {
            Write-Warn "未找到二进制文件，请手动将 $BinaryName 复制到 $InstallDir"
            Write-Warn "安装将继续，但服务可能无法正常启动"
        }
    }

    # 创建配置文件
    if (-not (Test-Path $ConfigPath)) {
        Write-Info "创建配置文件..."
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
  file_path: "$LogDir\croupier.log"
"@
        $exampleConfig | Out-File -FilePath $ConfigPath -Encoding UTF8
        Write-Success "配置文件已创建: $ConfigPath"
    } else {
        Write-Info "配置文件已存在: $ConfigPath"
    }

    # 创建环境变量配置文件
    $envFilePath = Join-Path $ConfigDir 'croupier.env'
    if (-not (Test-Path $envFilePath)) {
        $envContent = @"
# Croupier Server 环境变量
# 此文件中的变量会被服务读取

# 数据库连接
# DATABASE_URL=postgres://croupier:password@localhost:5432/croupier?sslmode=disable

# 监听地址
# CROUPIER_SERVER_ADDR=:8443
# CROUPIER_SERVER_HTTP_ADDR=:8080

# 日志级别
# CROUPIER_LOG_LEVEL=info
"@
        $envContent | Out-File -FilePath $envFilePath -Encoding UTF8
    }

    # 使用 sc.exe 创建服务
    Write-Info "注册 Windows 服务..."
    $serviceCmd = "sc.exe create `"$ServiceName`" binPath=`"$BinaryPath --config `"$ConfigPath`"`" DisplayName=`"$DisplayName`" start=auto"

    $result = Invoke-Expression $serviceCmd 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Success "服务 '$ServiceName' 创建成功"
    } else {
        Write-Error "服务创建失败: $result"
        exit 1
    }

    # 设置服务描述
    sc.exe description "$ServiceName" "Croupier Server - 游戏运营控制面服务" | Out-Null

    # 配置服务恢复选项
    Write-Info "配置服务恢复选项..."
    sc.exe failure "$ServiceName" reset= 86400 actions= restart/5000/restart/10000/restart/20000 | Out-Null

    Write-Success "安装完成！"
    Write-Info "请执行以下步骤完成配置："
    Write-Info "  1. 编辑配置文件: $ConfigPath"
    Write-Info "  2. 启动服务: .\$PSCommandPath Start"
    Write-Info "  3. 查看服务状态: .\$PSCommandPath Status"
    Write-Info "  4. 查看日志: Get-Content $LogDir\croupier.log -Tail 50 -Wait"
}

# 卸载服务
function Uninstall-ServiceInternal {
    Write-Info "卸载服务: $ServiceName"

    $service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if (-not $service) {
        Write-Warn "服务 '$ServiceName' 不存在"
        return
    }

    # 停止服务
    Stop-ServiceSafely $ServiceName

    # 删除服务
    Write-Info "删除服务..."
    $result = sc.exe delete "$ServiceName" 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Success "服务 '$ServiceName' 已删除"
    } else {
        Write-Warn "服务删除可能失败: $result"
    }

    Write-Info "卸载完成"
    Write-Warn "目录 $InstallDir 和 $DataDir 保留未删除"
    Write-Info "如需完全删除，请手动删除这些目录"
}

function Uninstall-Service {
    if (-not (Test-Admin)) {
        Write-Error "此脚本需要管理员权限运行"
        exit 1
    }
    Uninstall-ServiceInternal
}

# 重装服务
function Reinstall-Service {
    if (-not (Test-Admin)) {
        Write-Error "此脚本需要管理员权限运行"
        exit 1
    }
    Uninstall-ServiceInternal
    Start-Sleep -Seconds 2
    Install-Service
}

# 启动服务
function Start-ServiceCmd {
    if (-not (Test-Admin)) {
        Write-Error "此脚本需要管理员权限运行"
        exit 1
    }

    Write-Info "启动服务: $ServiceName"
    $service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if (-not $service) {
        Write-Error "服务 '$ServiceName' 不存在"
        exit 1
    }

    Start-Service -Name $ServiceName
    Start-Sleep -Seconds 1

    $service = Get-Service -Name $ServiceName
    if ($service.Status -eq 'Running') {
        Write-Success "服务 '$ServiceName' 已启动"
    } else {
        Write-Warn "服务可能未成功启动，请检查状态"
    }
}

# 停止服务
function Stop-ServiceCmd {
    if (-not (Test-Admin)) {
        Write-Error "此脚本需要管理员权限运行"
        exit 1
    }

    Write-Info "停止服务: $ServiceName"
    Stop-ServiceSafely $ServiceName
    Write-Success "服务 '$ServiceName' 已停止"
}

# 重启服务
function Restart-ServiceCmd {
    if (-not (Test-Admin)) {
        Write-Error "此脚本需要管理员权限运行"
        exit 1
    }

    Write-Info "重启服务: $ServiceName"
    Restart-Service -Name $ServiceName
    Start-Sleep -Seconds 1

    $service = Get-Service -Name $ServiceName
    if ($service.Status -eq 'Running') {
        Write-Success "服务 '$ServiceName' 已重启"
    } else {
        Write-Warn "服务可能未成功启动，请检查状态"
    }
}

# 查看服务状态
function Show-ServiceStatus {
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

        # 获取更详细的服务信息
        try {
            $serviceInfo = sc.exe query "$ServiceName"
            Write-Host "`n详细信息:"
            $serviceInfo | ForEach-Object { Write-Host "  $_" }
        } catch {
            # 忽略错误
        }

        # 检查进程
        $process = Get-Process -Name $BinaryName.Replace('.exe', '') -ErrorAction SilentlyContinue
        if ($process) {
            Write-Host "`n进程信息:"
            Write-Host "  PID: $($process.Id)"
            Write-Host "  内存使用: $([math]::Round($process.WorkingSet64/1MB, 2)) MB"
            Write-Host "  CPU 时间: $($process.CPU)"
            Write-Host "  启动时间: $($process.StartTime)"
        }
    } else {
        Write-Warn "服务 '$ServiceName' 不存在"
    }
}

# 启用服务 (自动启动)
function Enable-ServiceCmd {
    if (-not (Test-Admin)) {
        Write-Error "此脚本需要管理员权限运行"
        exit 1
    }

    Write-Info "启用服务自动启动: $ServiceName"
    Set-Service -Name $ServiceName -StartupType Automatic
    Write-Success "服务 '$ServiceName' 已设置为自动启动"
}

# 禁用服务
function Disable-ServiceCmd {
    if (-not (Test-Admin)) {
        Write-Error "此脚本需要管理员权限运行"
        exit 1
    }

    Write-Info "禁用服务自动启动: $ServiceName"
    Set-Service -Name $ServiceName -StartupType Disabled
    Write-Success "服务 '$ServiceName' 已禁用自动启动"
}

# 显示帮助
function Show-Help {
    Write-Host @"
Croupier Server Windows 服务安装脚本

用法: .\$($MyInvocation.MyCommand.Name) <命令> [选项]

命令:
  Install     安装服务 (创建目录、配置、注册服务)
  Uninstall   卸载服务 (保留数据目录)
  Reinstall   重装服务
  Start       启动服务
  Stop        停止服务
  Restart     重启服务
  Enable      启用服务 (开机自启)
  Disable     禁用服务
  Status      查看服务状态
  Help        显示此帮助信息

环境变量:
  CROUPIER_SERVICE_NAME   服务名称 (默认: CroupierServer)
  CROUPIER_DISPLAY_NAME   服务显示名称
  CROUPIER_INSTALL_DIR    安装目录 (默认: C:\Program Files\Croupier)
  CROUPIER_DATA_DIR       数据目录 (默认: C:\ProgramData\Croupier)
  CROUPIER_LOG_DIR        日志目录 (默认: C:\ProgramData\Croupier\logs)
  CROUPIER_CONFIG_DIR     配置目录 (默认: C:\ProgramData\Croupier\config)
  CROUPIER_BIN_PATH       二进制源文件路径 (可选)

示例:
  # 安装服务
  .\$($MyInvocation.MyCommand.Name) Install

  # 指定二进制源路径安装
  `$env:CROUPIER_BIN_PATH = 'C:\temp\croupier-server.exe'
  .\$($MyInvocation.MyCommand.Name) Install

  # 查看服务状态
  .\$($MyInvocation.MyCommand.Name) Status

  # 启动服务
  .\$($MyInvocation.MyCommand.Name) Start

  # 使用 PowerShell 服务管理
  Get-Service CroupierServer
  Start-Service CroupierServer
  Stop-Service CroupierServer

  # 查看日志
  Get-Content 'C:\ProgramData\Croupier\logs\croupier.log' -Tail 50 -Wait

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
    'Reinstall' {
        Reinstall-Service
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
    'Enable' {
        Enable-ServiceCmd
    }
    'Disable' {
        Disable-ServiceCmd
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
