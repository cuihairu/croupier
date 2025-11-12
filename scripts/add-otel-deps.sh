#!/bin/bash

echo "Adding OpenTelemetry dependencies to Croupier project..."

# Core OpenTelemetry packages
go get go.opentelemetry.io/otel@latest
go get go.opentelemetry.io/otel/metric@latest
go get go.opentelemetry.io/otel/trace@latest

# OTLP Exporters
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@latest
go get go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp@latest
go get go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp@latest

# Prometheus exporter (for backward compatibility)
go get go.opentelemetry.io/otel/exporters/prometheus@latest

# SDK and instrumentation
go get go.opentelemetry.io/otel/sdk@latest
go get go.opentelemetry.io/otel/sdk/metric@latest
go get go.opentelemetry.io/otel/sdk/trace@latest
go get go.opentelemetry.io/otel/sdk/log@latest

# Auto-instrumentation packages
go get go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin@latest
go get go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@latest
go get go.opentelemetry.io/otel/semconv/v1.24.0@latest

# Redis client for analytics bridge
go get github.com/redis/go-redis/v9@latest

# Resource detection
go get go.opentelemetry.io/otel/sdk/resource@latest

echo "OpenTelemetry dependencies added successfully!"
echo "Running go mod tidy..."
go mod tidy

echo "Dependencies installation completed!"