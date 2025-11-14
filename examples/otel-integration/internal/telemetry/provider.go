package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// Config OpenTelemetry 配置
type Config struct {
	ServiceName     string  `yaml:"service_name"`
	ServiceVersion  string  `yaml:"service_version"`
	Environment     string  `yaml:"environment"`
	CollectorURL    string  `yaml:"collector_url"`
	GameID          string  `yaml:"game_id"`
	EnableTracing   bool    `yaml:"enable_tracing"`
	EnableMetrics   bool    `yaml:"enable_metrics"`
	SamplingRatio   float64 `yaml:"sampling_ratio"`
}

// Provider OpenTelemetry 提供者
type Provider struct {
	TracerProvider *trace.TracerProvider
	MeterProvider  *metric.MeterProvider
	config         Config
}

// NewProvider 创建 OpenTelemetry 提供者
func NewProvider(ctx context.Context, config Config) (*Provider, error) {
	// 创建资源标识
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String(config.ServiceVersion),
			semconv.DeploymentEnvironmentKey.String(config.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	provider := &Provider{config: config}

	// 初始化Tracing
	if config.EnableTracing {
		provider.TracerProvider, err = initTracing(ctx, res, config)
		if err != nil {
			return nil, fmt.Errorf("failed to init tracing: %w", err)
		}
		otel.SetTracerProvider(provider.TracerProvider)
	}

	// 初始化Metrics
	if config.EnableMetrics {
		provider.MeterProvider, err = initMetrics(ctx, res, config)
		if err != nil {
			return nil, fmt.Errorf("failed to init metrics: %w", err)
		}
		otel.SetMeterProvider(provider.MeterProvider)
	}

	// 设置文本映射传播器
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return provider, nil
}

// initTracing 初始化链路追踪
func initTracing(ctx context.Context, res *resource.Resource, config Config) (*trace.TracerProvider, error) {
	// OTLP HTTP导出器
	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(config.CollectorURL),
		otlptracehttp.WithURLPath("/v1/traces"),
		otlptracehttp.WithInsecure(), // 开发环境使用，生产环境应启用TLS
	)
	if err != nil {
		return nil, err
	}

	// 创建TracerProvider
	tp := trace.NewTracerProvider(
		trace.WithResource(res),
		trace.WithBatcher(traceExporter,
			trace.WithBatchTimeout(time.Second*5),
			trace.WithMaxExportBatchSize(512),
		),
		trace.WithSampler(trace.TraceIDRatioBased(config.SamplingRatio)),
	)

	return tp, nil
}

// initMetrics 初始化指标收集
func initMetrics(ctx context.Context, res *resource.Resource, config Config) (*metric.MeterProvider, error) {
	// OTLP HTTP导出器
	metricExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(config.CollectorURL),
		otlpmetrichttp.WithURLPath("/v1/metrics"),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	// 创建MeterProvider
	mp := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(
			metricExporter,
			metric.WithInterval(time.Second*30), // 30秒推送一次指标
		)),
	)

	return mp, nil
}

// Shutdown 优雅关闭
func (p *Provider) Shutdown(ctx context.Context) error {
	var err error

	if p.TracerProvider != nil {
		if shutdownErr := p.TracerProvider.Shutdown(ctx); shutdownErr != nil {
			err = fmt.Errorf("failed to shutdown TracerProvider: %w", shutdownErr)
		}
	}

	if p.MeterProvider != nil {
		if shutdownErr := p.MeterProvider.Shutdown(ctx); shutdownErr != nil {
			if err != nil {
				err = fmt.Errorf("%v; failed to shutdown MeterProvider: %w", err, shutdownErr)
			} else {
				err = fmt.Errorf("failed to shutdown MeterProvider: %w", shutdownErr)
			}
		}
	}

	return err
}

// LoadConfigFromEnv 从环境变量加载配置
func LoadConfigFromEnv() Config {
	return Config{
		ServiceName:    getEnvOrDefault("OTEL_SERVICE_NAME", "game-example"),
		ServiceVersion: getEnvOrDefault("OTEL_SERVICE_VERSION", "1.0.0"),
		Environment:    getEnvOrDefault("OTEL_ENVIRONMENT", "development"),
		CollectorURL:   getEnvOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
		GameID:         getEnvOrDefault("GAME_ID", "example-game"),
		EnableTracing:  getEnvOrDefault("OTEL_ENABLE_TRACING", "true") == "true",
		EnableMetrics:  getEnvOrDefault("OTEL_ENABLE_METRICS", "true") == "true",
		SamplingRatio:  parseFloatOrDefault(getEnvOrDefault("OTEL_SAMPLING_RATIO", "1.0")),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseFloatOrDefault(s string) float64 {
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return 1.0 // 默认100%采样
}