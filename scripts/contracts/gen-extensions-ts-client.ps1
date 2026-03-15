param(
  [string]$OutDir = ".tmp/contracts/client"
)

$ErrorActionPreference = "Stop"

$Root = (Resolve-Path "$PSScriptRoot/../..").Path
$InputSpec = Join-Path $Root "docs/contracts/extensions-openapi-v1.yaml"
$OutputPath = if ([System.IO.Path]::IsPathRooted($OutDir)) { $OutDir } else { Join-Path $Root $OutDir }

New-Item -ItemType Directory -Force -Path $OutputPath | Out-Null

Write-Host "Generating TypeScript client from: $InputSpec"
Write-Host "Output dir: $OutputPath"

npx --yes openapi-typescript-codegen@0.29.0 `
  --input $InputSpec `
  --output $OutputPath `
  --client fetch `
  --useUnionTypes

Write-Host "Done."
