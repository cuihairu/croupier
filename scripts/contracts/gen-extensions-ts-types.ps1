param(
  [string]$OutFile = ".tmp/contracts/extensions.types.ts"
)

$ErrorActionPreference = "Stop"

$Root = (Resolve-Path "$PSScriptRoot/../..").Path
$InputSpec = Join-Path $Root "docs/contracts/extensions-openapi-v1.yaml"
$OutputPath = if ([System.IO.Path]::IsPathRooted($OutFile)) { $OutFile } else { Join-Path $Root $OutFile }

New-Item -ItemType Directory -Force -Path (Split-Path $OutputPath -Parent) | Out-Null

Write-Host "Generating TypeScript types from: $InputSpec"
Write-Host "Output: $OutputPath"

npx --yes openapi-typescript@7.10.1 $InputSpec -o $OutputPath

Write-Host "Done."
