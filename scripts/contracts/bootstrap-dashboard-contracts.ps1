param(
  [Parameter(Mandatory = $true)]
  [string]$DashboardRoot,
  [switch]$Force
)

$ErrorActionPreference = "Stop"

$Root = (Resolve-Path "$PSScriptRoot/../..").Path
$Dashboard = if ([System.IO.Path]::IsPathRooted($DashboardRoot)) { $DashboardRoot } else { Join-Path $Root $DashboardRoot }

if (!(Test-Path $Dashboard -PathType Container)) {
  throw "Dashboard root not found: $Dashboard"
}

Write-Host "Bootstrap dashboard contracts into: $Dashboard"

powershell -File (Join-Path $Root "scripts/contracts/gen-extensions-ts-types.ps1") `
  -OutFile (Join-Path $Dashboard "src/services/contracts/extensions.ts")

powershell -File (Join-Path $Root "scripts/contracts/gen-extensions-ts-client.ps1") `
  -OutDir (Join-Path $Dashboard "src/services/generated/extensions-client")

$TemplateRoot = Join-Path $Root "docs/contracts/templates/dashboard"
$files = Get-ChildItem -Path $TemplateRoot -Recurse -File

foreach ($file in $files) {
  $relative = $file.FullName.Substring($TemplateRoot.Length + 1)
  $target = Join-Path $Dashboard $relative
  New-Item -ItemType Directory -Force -Path (Split-Path $target -Parent) | Out-Null

  if ((Test-Path $target) -and -not $Force) {
    Write-Host "Skip existing: $target"
    continue
  }

  Copy-Item -Path $file.FullName -Destination $target -Force
  Write-Host "Wrote: $target"
}

Write-Host "Bootstrap done."
