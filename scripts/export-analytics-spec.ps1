param(
  [string]$Configs = "configs/analytics",
  [string]$Out = "web/public/analytics-spec.json"
)

$env:GOFLAGS = "-mod=mod"
go run ./cmd/analytics-export --configs "$Configs" --out "$Out"
