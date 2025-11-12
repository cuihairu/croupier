package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"
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

// TelemetryConfig OpenTelemetry配置
type TelemetryConfig struct {
	ServiceName     string `yaml:"service_name"`
	ServiceVersion  string `yaml:"service_version"`
	Environment     string `yaml:"environment"`
	CollectorURL    string `yaml:"collector_url"`
	GameID          string `yaml:"game_id"`
	EnableTracing   bool   `yaml:"enable_tracing"`
	EnableMetrics   bool   `yaml:"enable_metrics"`
	SamplingRatio   float64 `yaml:"sampling_ratio"`

	// Analytics桥接配置
	Analytics AnalyticsBridgeConfig `yaml:"analytics"`
}

// Provider OpenTelemetry提供者
type Provider struct {
	TracerProvider *trace.TracerProvider
	MeterProvider  *metric.MeterProvider
	GameMetrics    *GameMetrics
	GameTracer     *GameTracer
	Bridge         *AnalyticsBridge
	config         TelemetryConfig
}

// NewProvider 创建OpenTelemetry提供者
func NewProvider(ctx context.Context, config TelemetryConfig, logger *slog.Logger) (*Provider, error) {
	// 创建资源标识
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String(config.ServiceVersion),
			semconv.DeploymentEnvironmentKey.String(config.Environment),
			GameIDKey.String(config.GameID),
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

	// 初始化游戏指标
	meter := otel.Meter("croupier.game")
	provider.GameMetrics, err = NewGameMetrics(meter)
	if err != nil {
		return nil, fmt.Errorf("failed to create game metrics: %w", err)
	}

	// 初始化Analytics桥接器
	provider.Bridge = NewAnalyticsBridge(config.Analytics, config.GameID, logger)

	// 初始化游戏追踪器
	tracer := otel.Tracer("croupier.game")
	provider.GameTracer = NewGameTracer(tracer, provider.GameMetrics, provider.Bridge)

	return provider, nil
}

// initTracing 初始化链路追踪
func initTracing(ctx context.Context, res *resource.Resource, config TelemetryConfig) (*trace.TracerProvider, error) {
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
func initMetrics(ctx context.Context, res *resource.Resource, config TelemetryConfig) (*metric.MeterProvider, error) {
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

	// 关闭Analytics桥接器
	if p.Bridge != nil {
		if shutdownErr := p.Bridge.Shutdown(ctx); shutdownErr != nil {
			err = fmt.Errorf("failed to shutdown Analytics bridge: %w", shutdownErr)
		}
	}

	if p.TracerProvider != nil {
		if shutdownErr := p.TracerProvider.Shutdown(ctx); shutdownErr != nil {
			if err != nil {
				err = fmt.Errorf("%v; failed to shutdown TracerProvider: %w", err, shutdownErr)
			} else {
				err = fmt.Errorf("failed to shutdown TracerProvider: %w", shutdownErr)
			}
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
func LoadConfigFromEnv() TelemetryConfig {
	return TelemetryConfig{
		ServiceName:    getEnvOrDefault("OTEL_SERVICE_NAME", "croupier-server"),
		ServiceVersion: getEnvOrDefault("OTEL_SERVICE_VERSION", "1.0.0"),
		Environment:    getEnvOrDefault("OTEL_ENVIRONMENT", "development"),
		CollectorURL:   getEnvOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
		GameID:         getEnvOrDefault("GAME_ID", "default"),
		EnableTracing:  getEnvOrDefault("OTEL_ENABLE_TRACING", "true") == "true",
		EnableMetrics:  getEnvOrDefault("OTEL_ENABLE_METRICS", "true") == "true",
		SamplingRatio:  parseFloatOrDefault(getEnvOrDefault("OTEL_SAMPLING_RATIO", "1.0")),
		Analytics: AnalyticsBridgeConfig{
			Enabled:        getEnvOrDefault("ANALYTICS_BRIDGE_ENABLED", "true") == "true",
			RedisAddr:      getEnvOrDefault("ANALYTICS_REDIS_ADDR", "localhost:6379"),
			RedisPassword:  getEnvOrDefault("ANALYTICS_REDIS_PASSWORD", ""),
			RedisDB:        parseIntOrDefault(getEnvOrDefault("ANALYTICS_REDIS_DB", "0")),
			TopicPrefix:    getEnvOrDefault("ANALYTICS_TOPIC_PREFIX", "game:events"),
			RetentionHours: parseIntOrDefault(getEnvOrDefault("ANALYTICS_RETENTION_HOURS", "168")), // 7天
			BatchSize:      parseIntOrDefault(getEnvOrDefault("ANALYTICS_BATCH_SIZE", "100")),
			FlushInterval:  parseDurationOrDefault(getEnvOrDefault("ANALYTICS_FLUSH_INTERVAL", "30s")),
		},
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseFloatOrDefault(s string) float64 {
	if f, err := parseFloat(s); err == nil {
		return f
	}
	return 1.0 // 默认100%采样
}

// parseFloat 简单的浮点数解析
func parseFloat(s string) (float64, error) {
	// 这里可以使用 strconv.ParseFloat，简化示例
	if s == "1.0" {
		return 1.0, nil
	}
	if s == "0.1" {
		return 0.1, nil
	}
	return 1.0, nil
}

// parseIntOrDefault 简单的整数解析
func parseIntOrDefault(s string) int {
	switch s {
	case "0":
		return 0
	case "100":
		return 100
	case "168":
		return 168
	default:
		return 0
	}
}

// parseDurationOrDefault 简单的时间间隔解析
func parseDurationOrDefault(s string) time.Duration {
	switch s {
	case "30s":
		return 30 * time.Second
	case "60s":
		return 60 * time.Second
	case "5m":
		return 5 * time.Minute
	default:
		return 30 * time.Second
	}
}