module github.com/cuihairu/croupier/examples/otel-integration

go 1.24.0

require (
	github.com/cuihairu/croupier v0.0.0
	github.com/prometheus/client_golang v1.20.5
	github.com/redis/go-redis/v9 v9.16.0
	go.opentelemetry.io/otel v1.38.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.32.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.32.0
	go.opentelemetry.io/otel/exporters/prometheus v0.54.0
	go.opentelemetry.io/otel/metric v1.38.0
	go.opentelemetry.io/otel/sdk v1.37.0
	go.opentelemetry.io/otel/sdk/metric v1.37.0
	go.opentelemetry.io/otel/trace v1.38.0
	gopkg.in/yaml.v3 v3.0.1
)

// 使用本地 Croupier 模块
replace github.com/cuihairu/croupier => ../..
