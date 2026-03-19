#
# Croupier Agent Windows Service Installation Script (using NSSM)
# Usage: .\scripts\install-agent.ps1 [Install|Uninstall|Reinstall|Start|Stop|Restart|Status]
#
# Environment Variables (can override):
#   CROUPIER_AGENT_VERSION     - Agent version (default: v0.1.2)
#   CROUPIER_AGENT_DIR         - Installation directory (required)
#   CROUPIER_SERVER_ADDR       - Server address (default: localhost:8443)
#   CROUPIER_GAME_ID           - Game ID (required)
#   CROUPIER_ENV               - Environment (default: dev)
#   CROUPIER_NSSM_PATH         - Path to nssm.exe (optional, auto-download if not found)
#

[CmdletBinding()]
param(
    [Parameter(Position=0)]
    [ValidateSet('Install', 'Uninstall', 'Reinstall', 'Start', 'Stop', 'Restart', 'Status', 'Help', 'Enable', 'Disable')]
    [string]$Command = 'Help',

    [Parameter()]
    [string]$Version = $env:CROUPIER_AGENT_VERSION,

    [Parameter()]
    [string]$ServerAddr = $env:CROUPIER_SERVER_ADDR,

    [Parameter()]
    [string]$GameID = $env:CROUPIER_GAME_ID,

    [Parameter()]
    [string]$Env = $env:CROUPIER_ENV,

    [Parameter()]
    [string]$InstallDir = $env:CROUPIER_AGENT_DIR
)

# Default configuration
$DefaultVersion = "v0.1.4"
$DefaultServerAddr = "localhost:8443"
$DefaultEnv = "dev"

if (-not $Version) { $Version = $DefaultVersion }
if (-not $ServerAddr) { $ServerAddr = $DefaultServerAddr }
if (-not $Env) { $Env = $DefaultEnv }

$ServiceName = "CroupierAgent"
$DisplayName = "Croupier Agent - Game Server Proxy"
$BinaryName = "croupier-agent.exe"
$ReleaseUrl = "https://github.com/cuihairu/croupier/releases/download"
$NssmUrl = "https://nssm.cc/release/nssm-2.24.zip"
$NssmDir = Join-Path $env:ProgramFiles "NSSM"
$NssmPath = Join-Path $NssmDir "nssm.exe"

# Validate InstallDir
if ($Command -eq 'Install' -and -not $InstallDir) {
    Write-Error "Installation directory is required. Use -InstallDir or set CROUPIER_AGENT_DIR environment variable."
    exit 1
}

if ($InstallDir) {
    $BinaryPath = Join-Path $InstallDir $BinaryName
    $ConfigPath = Join-Path $InstallDir "agent.yaml"
    $DataDir = Join-Path $InstallDir "data"
    $LogsDir = Join-Path $InstallDir "logs"
}

# Helper functions
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
        Write-Info "Created directory: $Path"
    }
}

function Stop-ServiceSafely {
    param([string]$Name)
    $service = Get-Service -Name $Name -ErrorAction SilentlyContinue
    if ($service -and $service.Status -eq 'Running') {
        Write-Info "Stopping service: $Name"
        Stop-Service -Name $Name -Force
        Start-Sleep -Seconds 2
    }
}

# Ensure NSSM is available
function Get-Nssm {
    if (Test-Path $NssmPath) {
        return $NssmPath
    }

    # Check in PATH
    $nssmInPath = Get-Command nssm -ErrorAction SilentlyContinue
    if ($nssmInPath) {
        return $nssmInPath.Source
    }

    # NSSM not found, show download instructions
    Write-Error "NSSM is required but not installed"
    Write-Host ""
    Write-Host "Please install NSSM first:" -ForegroundColor Cyan
    Write-Host "  1. Download from: https://nssm.cc/download" -ForegroundColor White
    Write-Host "  2. Extract and copy nssm.exe to: $NssmDir" -ForegroundColor White
    Write-Host "  3. Or use Chocolatey: choco install nssm" -ForegroundColor White
    Write-Host ""
    Write-Host "After installation, run this script again." -ForegroundColor Yellow
    exit 1
}

# Download Agent binary
function Download-AgentBinary {
    Write-Info "Downloading Croupier Agent $Version ..."

    $tempDir = Join-Path $env:TEMP "croupier-agent-$Version"
    New-Directory $tempDir

    $zipFile = Join-Path $tempDir "croupier-windows.zip"
    $url = "$ReleaseUrl/$($Version.TrimStart('v'))/croupier-bin-windows-amd64.zip"

    try {
        Invoke-WebRequest -Uri $url -OutFile $zipFile -UseBasicParsing
        Write-Success "Download complete"

        # Extract
        Write-Info "Extracting files..."
        Expand-Archive -Path $zipFile -DestinationPath $tempDir -Force

        # Copy agent
        $sourceAgent = Join-Path $tempDir $BinaryName
        if (Test-Path $sourceAgent) {
            Copy-Item -Path $sourceAgent -Destination $BinaryPath -Force
            Write-Success "Binary installed to: $BinaryPath"
        } else {
            Write-Error "Could not find $BinaryName after extraction"
            exit 1
        }

        # Cleanup temp files
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
    catch {
        Write-Error "Download failed: $_"
        exit 1
    }
}

# Generate configuration file
function New-AgentConfig {
    Write-Info "Generating configuration file..."

    if (-not $GameID) {
        Write-Warn "GAME_ID not specified, using default 'my-game'"
        $GameID = "my-game"
    }

    $configContent = @"
# Croupier Agent Configuration
# Generated: $(Get-Date -Format "yyyy-MM-dd HH:mm:ss")

name: croupier-agent
host: 0.0.0.0
port: 18888

# Server configuration
server:
  addr: $ServerAddr
  insecure: true                # Disable mTLS for development
  tlsCertFile: ""
  tlsKeyFile: ""
  caFile: ""
  serverName: ""
  insecureSkipVerify: true

# Agent configuration
agent:
  id: ""                        # Auto-generate if empty
  gameId: "$GameID"
  env: "$Env"
  localAddr: "127.0.0.1:19090"
  httpAddr: "127.0.0.1:19091"
  labels: {}

# Upstream configuration
upstream:
  heartbeatInterval: 30
  retryInterval: 5
  maxRetries: 3
  timeout: 10000

# TLS configuration for game servers connecting to this Agent
tls:
  enabled: false
  certFile: ""
  keyFile: ""
  caFile: ""
  insecureSkipVerify: false
"@

    $configContent | Out-File -FilePath $ConfigPath -Encoding UTF8
    Write-Success "Configuration created: $ConfigPath"
}

# Install service using NSSM
function Install-Service {
    Write-Info "Installing Croupier Agent Windows Service"
    Write-Info "Install directory: $InstallDir"
    Write-Info "Config file: $ConfigPath"
    Write-Info "Server address: $ServerAddr"
    Write-Info "Game ID: $GameID"

    # Check admin privileges
    if (-not (Test-Admin)) {
        Write-Error "This script requires administrator privileges"
        Write-Info "Please run PowerShell as Administrator"
        exit 1
    }

    # Ensure NSSM is available
    $nssm = Get-Nssm
    if (-not $nssm) {
        Write-Error "NSSM is required but not available"
        exit 1
    }

    # Check if service already exists
    $existingService = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($existingService) {
        Write-Warn "Service '$ServiceName' already exists"
        $response = Read-Host "Reinstall? (y/N)"
        if ($response -eq 'y' -or $response -eq 'Y') {
            Uninstall-ServiceInternal
        } else {
            Write-Info "Installation cancelled"
            exit 0
        }
    }

    # Check GameID
    if (-not $GameID) {
        Write-Error "GameID is required"
        Write-Info "Usage: .\$PSCommandPath Install -GameID 'your-game-id' -InstallDir 'path'"
        exit 1
    }

    # Create directories
    Write-Info "Creating directory structure..."
    New-Directory $InstallDir
    New-Directory $DataDir
    New-Directory $LogsDir

    # Download or use local binary
    if (Test-Path $BinaryPath) {
        Write-Info "Binary already exists, skipping download"
    } else {
        Download-AgentBinary
    }

    # Generate config
    if (Test-Path $ConfigPath) {
        Write-Warn "Configuration already exists: $ConfigPath"
        $response = Read-Host "Overwrite? (y/N)"
        if ($response -eq 'y' -or $response -eq 'Y') {
            New-AgentConfig
        }
    } else {
        New-AgentConfig
    }

    # Install service using NSSM
    Write-Info "Installing service with NSSM..."

    # Remove existing service with NSSM if it exists
    & $nssm remove $ServiceName confirm
    Start-Sleep -Seconds 1

    # Install service
    & $nssm install $ServiceName $BinaryPath "--config" $ConfigPath
    if ($LASTEXITCODE -eq 0) {
        Write-Success "Service '$ServiceName' installed with NSSM"
    } else {
        Write-Error "Service installation failed"
        exit 1
    }

    # Set application directory (working directory)
    Write-Info "Setting working directory..."
    & $nssm set $ServiceName AppDirectory $InstallDir

    # Set display name
    & $nssm set $ServiceName DisplayName $DisplayName

    # Set description
    & $nssm set $ServiceName Description "Croupier Agent - Game Server Proxy Service"

    # Configure stdout/stderr redirection
    & $nssm set $ServiceName StdoutLogfile "$LogsDir\stdout.log"
    & $nssm set $ServiceName StderrLogfile "$LogsDir\stderr.log"

    # Configure service recovery (restart on failure)
    & $nssm set $ServiceName AppRestartDelay 5000
    & $nssm set $ServiceName AppThrottle 1500
    & $nssm set $ServiceName AppExit Default Restart
    & $nssm set $ServiceName AppRestart 5000

    # Set to auto-start
    & $nssm set $ServiceName Start SERVICE_AUTO_START

    Write-Success "Installation complete!"
    Write-Info ""
    Write-Info "Next steps:"
    Write-Info "  1. Review config: $ConfigPath"
    Write-Info "  2. Start service: .\$PSCommandPath Start"
    Write-Info "  3. Check status: .\$PSCommandPath Status"
    Write-Info "  4. View logs: Get-Content $LogsDir\*.log -Tail 50"
}

# Uninstall service
function Uninstall-ServiceInternal {
    Write-Info "Uninstalling service: $ServiceName"

    $nssm = Get-Nssm
    if (-not $nssm) {
        Write-Warn "NSSM not found, using sc.exe"
        $service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
        if ($service) {
            Stop-ServiceSafely $ServiceName
            sc.exe delete "$ServiceName" 2>&1 | Out-Null
        }
        return
    }

    # Stop service with NSSM
    & $nssm stop $ServiceName
    Start-Sleep -Seconds 2

    # Remove service with NSSM
    & $nssm remove $ServiceName confirm

    Write-Success "Service '$ServiceName' removed"
    Write-Warn "Directory $InstallDir was not deleted"
    Write-Info "To remove completely: Remove-Item -Recurse -Force $InstallDir"
}

function Uninstall-Service {
    if (-not (Test-Admin)) {
        Write-Error "This script requires administrator privileges"
        exit 1
    }
    Uninstall-ServiceInternal
}

# Reinstall service
function Reinstall-Service {
    if (-not (Test-Admin)) {
        Write-Error "This script requires administrator privileges"
        exit 1
    }
    Uninstall-ServiceInternal
    Start-Sleep -Seconds 2
    Install-Service
}

# Start service
function Start-ServiceCmd {
    if (-not (Test-Admin)) {
        Write-Error "This script requires administrator privileges"
        exit 1
    }

    Write-Info "Starting service: $ServiceName"

    $nssm = Get-Nssm
    if ($nssm) {
        & $nssm start $ServiceName
    } else {
        $service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
        if (-not $service) {
            Write-Error "Service '$ServiceName' does not exist"
            exit 1
        }
        Start-Service -Name $ServiceName
    }

    Start-Sleep -Seconds 2

    $service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($service -and $service.Status -eq 'Running') {
        Write-Success "Service '$ServiceName' started"
    } else {
        Write-Warn "Service may not have started successfully"
    }
}

# Stop service
function Stop-ServiceCmd {
    if (-not (Test-Admin)) {
        Write-Error "This script requires administrator privileges"
        exit 1
    }

    Write-Info "Stopping service: $ServiceName"

    $nssm = Get-Nssm
    if ($nssm) {
        & $nssm stop $ServiceName
    } else {
        Stop-ServiceSafely $ServiceName
    }

    Write-Success "Service '$ServiceName' stopped"
}

# Restart service
function Restart-ServiceCmd {
    if (-not (Test-Admin)) {
        Write-Error "This script requires administrator privileges"
        exit 1
    }

    Write-Info "Restarting service: $ServiceName"

    $nssm = Get-Nssm
    if ($nssm) {
        & $nssm restart $ServiceName
    } else {
        Restart-Service -Name $ServiceName
    }

    Start-Sleep -Seconds 2

    $service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($service -and $service.Status -eq 'Running') {
        Write-Success "Service '$ServiceName' restarted"
    } else {
        Write-Warn "Service may not have started successfully"
    }
}

# Show service status
function Show-ServiceStatus {
    $service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($service) {
        Write-Host "Service Name: $($service.Name)" -ForegroundColor Cyan
        Write-Host "Display Name: $($service.DisplayName)" -ForegroundColor Cyan
        Write-Host "Status: " -NoNewline
        switch ($service.Status) {
            'Running' { Write-Host "$($service.Status)" -ForegroundColor Green }
            'Stopped' { Write-Host "$($service.Status)" -ForegroundColor Red }
            default { Write-Host "$($service.Status)" -ForegroundColor Yellow }
        }
        Write-Host "Startup Type: $($service.StartType)"

        # Check process
        $process = Get-Process -Name $BinaryName.Replace('.exe', '') -ErrorAction SilentlyContinue
        if ($process) {
            Write-Host "`nProcess Info:"
            Write-Host "  PID: $($process.Id)"
            Write-Host "  Memory: $([math]::Round($process.WorkingSet64/1MB, 2)) MB"
            Write-Host "  Start Time: $($process.StartTime)"
        }

        # Show config info
        if ($InstallDir) {
            Write-Host "`nConfiguration:"
            Write-Host "  Install Dir: $InstallDir"
            Write-Host "  Config File: $ConfigPath"
            Write-Host "  Logs Dir: $LogsDir"
        }

        # Show NSSM specific info
        $nssm = Get-Nssm
        if ($nssm) {
            Write-Host "`nNSSM Status:"
            & $nssm status $ServiceName
        }
    } else {
        Write-Warn "Service '$ServiceName' does not exist"
    }
}

# Enable service (auto start)
function Enable-ServiceCmd {
    if (-not (Test-Admin)) {
        Write-Error "This script requires administrator privileges"
        exit 1
    }

    Write-Info "Enabling service auto-start: $ServiceName"

    $nssm = Get-Nssm
    if ($nssm) {
        & $nssm set $ServiceName Start SERVICE_AUTO_START
    } else {
        Set-Service -Name $ServiceName -StartupType Automatic
    }

    Write-Success "Service '$ServiceName' set to auto-start"
}

# Disable service
function Disable-ServiceCmd {
    if (-not (Test-Admin)) {
        Write-Error "This script requires administrator privileges"
        exit 1
    }

    Write-Info "Disabling service auto-start: $ServiceName"

    $nssm = Get-Nssm
    if ($nssm) {
        & $nssm set $ServiceName Start SERVICE_DEMAND_START
    } else {
        Set-Service -Name $ServiceName -StartupType Manual
    }

    Write-Success "Service '$ServiceName' disabled"
}

# Show help
function Show-Help {
    Write-Host @"
Croupier Agent Windows Service Installation Script (using NSSM)

Usage: .\$($MyInvocation.MyCommand.Name) <command> [options]

Commands:
  Install     Install service (download binary, generate config, register service)
  Uninstall   Uninstall service (keep data directory)
  Reinstall   Reinstall service
  Start       Start service
  Stop        Stop service
  Restart     Restart service
  Enable      Enable service (auto-start on boot)
  Disable     Disable service
  Status      Show service status and configuration
  Help        Show this help message

Options:
  -Version      Agent version (default: $DefaultVersion)
  -ServerAddr   Server address (default: $DefaultServerAddr)
  -GameID       Game ID (required)
  -Env          Environment (default: $DefaultEnv)
  -InstallDir   Installation directory (required)

Environment Variables:
  CROUPIER_AGENT_VERSION   Agent version
  CROUPIER_SERVER_ADDR     Server address
  CROUPIER_GAME_ID         Game ID
  CROUPIER_ENV             Environment
  CROUPIER_AGENT_DIR       Installation directory
  CROUPIER_NSSM_PATH       Path to nssm.exe

Examples:
  # Install service (specify GameID and InstallDir)
  .\$($MyInvocation.MyCommand.Name) Install -GameID "my-game" -InstallDir "C:\croupier\agent"

  # Install service (specify Server address)
  .\$($MyInvocation.MyCommand.Name) Install -GameID "my-game" -InstallDir "C:\croupier\agent" -ServerAddr "192.168.1.100:8443"

  # Install service (specify environment)
  .\$($MyInvocation.MyCommand.Name) Install -GameID "my-game" -InstallDir "~\croupier\agent" -Env "prod"

  # Check service status
  .\$($MyInvocation.MyCommand.Name) Status

  # Start service
  .\$($MyInvocation.MyCommand.Name) Start

  # Using NSSM directly
  nssm start CroupierAgent
  nssm stop CroupierAgent
  nssm restart CroupierAgent
  nssm status CroupierAgent

Notes:
  - This script requires administrator privileges
  - Run PowerShell as Administrator to execute
  - InstallDir parameter is required for installation
  - NSSM is required (see https://nssm.cc/download)
    * Download: https://nssm.cc/release/nssm-2.24.zip
    * Or use: choco install nssm
"@ -ForegroundColor Cyan
}

# Main entry point
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
        Write-Error "Unknown command: $Command"
        Show-Help
        exit 1
    }
}
