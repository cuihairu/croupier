package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// TelemetryConfig OpenTelemetry配置
type TelemetryConfig struct {
	Enabled        bool    `yaml:"enabled"`
	ServiceName    string  `yaml:"service_name"`
	ServiceVersion string  `yaml:"service_version"`
	Environment    string  `yaml:"environment"`
	CollectorURL   string  `yaml:"collector_url"`
	GameID         string  `yaml:"game_id"`
	EnableTracing  bool    `yaml:"enable_tracing"`
	EnableMetrics  bool    `yaml:"enable_metrics"`
	SamplingRatio  float64 `yaml:"sampling_ratio"`

	// TLS配置
	UseTLS  bool   `yaml:"use_tls"` // 是否使用TLS连接到Collector
	Headers string `yaml:"headers"` // 自定义HTTP头（如Authorization）

	// Analytics桥接配置
	Analytics AnalyticsBridgeConfig `yaml:"analytics"`
}

// shutdowner 是 Provider 持有 OTel SDK tracer/meter provider 的最小表面
// （两者都暴露 Shutdown(context.Context) error）。字段保持接口类型，
// no_otel 裁剪构建无需引入 SDK 类型即可编译本文件。
type shutdowner interface {
	Shutdown(context.Context) error
}

// Provider OpenTelemetry提供者
type Provider struct {
	TracerProvider shutdowner
	MeterProvider  shutdowner
	GameMetrics    *GameMetrics
	GameTracer     *GameTracer
	Bridge         *AnalyticsBridge
	config         TelemetryConfig
}

// 可注入缝隙：NewGameMetrics（全局 noop meter 创建固定合法指标名，不存在
// 失败输入）的错误分支仅能通过测试注入触达。OTel SDK 侧的注入缝
// （resource.New / OTLP exporter 构造）随装配移至 provider_sdk_default.go。
var providerMetrics = NewGameMetrics

// NewProvider 创建OpenTelemetry提供者
func NewProvider(ctx context.Context, config TelemetryConfig, logger *slog.Logger) (*Provider, error) {
	normalizeConfig(&config)

	provider := &Provider{config: config}

	// 总开关门控（todo.md:31，与 go-plugin 同策略）：Enabled=false 时
	// tracing/metrics 子开关一律失效——不创建 exporter、不建周期导出任务、
	// 不注册全局 SDK provider；仅保留轻量 API 面（W3C TraceContext 传播、
	// noop tracer/meter、Analytics 桥接自守门）。子开关不得越过总开关。
	if !config.Enabled {
		return provider.assembleLightweight(logger)
	}

	// 可禁用构建 fail-closed（no_otel build tag）：二进制不含 OTel
	// SDK/exporter 装配时，配置启用即显式报错，而非静默降级成无导出运行
	// （配置说开了、产物做不到，属于操作员必须看到错误）。
	if otelAssemblyTrimmed {
		return nil, fmt.Errorf("telemetry enabled but this binary was built without OTel support (no_otel build tag); disable telemetry or rebuild without the tag")
	}

	tracerProvider, meterProvider, err := assembleSDK(ctx, config, logger)
	if err != nil {
		return nil, err
	}
	provider.TracerProvider = tracerProvider
	provider.MeterProvider = meterProvider

	return provider.assembleLightweight(logger)
}

// assembleLightweight 补齐常驻的轻量 API 面：传播器、业务指标（全局 meter，
// 总关/裁剪构建下为 noop）、Analytics 桥接（自守门）与游戏追踪器。
func (p *Provider) assembleLightweight(logger *slog.Logger) (*Provider, error) {
	// 设置文本映射传播器（trace-context 保留范围：裁剪构建/总关时仍在，
	// 供跨进程 trace 串联）。
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// 初始化游戏指标（全局 meter 在总关/裁剪构建下为 noop 实现）
	meter := otel.Meter("croupier.game")
	gameMetrics, err := providerMetrics(meter)
	if err != nil {
		return nil, fmt.Errorf("failed to create game metrics: %w", err)
	}
	p.GameMetrics = gameMetrics

	// 初始化Analytics桥接器
	p.Bridge = NewAnalyticsBridge(p.config.Analytics, p.config.GameID, logger)

	// 初始化游戏追踪器
	p.GameTracer = NewGameTracer(otel.Tracer("croupier.game"), p.GameMetrics, p.Bridge)

	return p, nil
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
// 总开关只认 OTEL_ENABLED / CROUPIER_TELEMETRY_ENABLED；tracing/metrics/
// analytics 子开关不得重新点亮总开关（todo.md:31）。
func LoadConfigFromEnv() TelemetryConfig {
	enabled := envBool("OTEL_ENABLED", false) || envBool("CROUPIER_TELEMETRY_ENABLED", false)
	tracing := envBool("OTEL_ENABLE_TRACING", false)
	metrics := envBool("OTEL_ENABLE_METRICS", false)
	analytics := envBool("ANALYTICS_BRIDGE_ENABLED", false)
	return TelemetryConfig{
		Enabled:        enabled,
		ServiceName:    getEnvOrDefault("OTEL_SERVICE_NAME", "croupier-server"),
		ServiceVersion: getEnvOrDefault("OTEL_SERVICE_VERSION", "1.0.0"),
		Environment:    getEnvOrDefault("OTEL_ENVIRONMENT", "development"),
		// 未配置采集端时不填默认远端（todo.md:31）：留空=装配期跳过
		// exporter/周期导出任务，绝不擅自导出到默认地址。
		CollectorURL:  os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		GameID:        getEnvOrDefault("GAME_ID", "default"),
		EnableTracing: tracing,
		EnableMetrics: metrics,
		SamplingRatio: parseFloatOrDefault(getEnvOrDefault("OTEL_SAMPLING_RATIO", "1.0")),
		Analytics: AnalyticsBridgeConfig{
			Enabled:        analytics,
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

func envBool(key string, defaultValue bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}
	return strings.EqualFold(value, "true") || value == "1" || strings.EqualFold(value, "yes") || strings.EqualFold(value, "on")
}

func normalizeConfig(config *TelemetryConfig) {
	if config == nil {
		return
	}
	if strings.TrimSpace(config.ServiceName) == "" {
		config.ServiceName = "croupier-server"
	}
	if strings.TrimSpace(config.ServiceVersion) == "" {
		config.ServiceVersion = "1.0.0"
	}
	if strings.TrimSpace(config.Environment) == "" {
		config.Environment = "development"
	}
	// CollectorURL 不再回填默认远端（todo.md:31）：未配置采集端=留空，
	// 装配期跳过该信号的 exporter/周期导出任务。
	if strings.TrimSpace(config.GameID) == "" {
		config.GameID = "default"
	}
	if config.SamplingRatio <= 0 || config.SamplingRatio > 1 {
		config.SamplingRatio = 1.0
	}
}

func trimOTLPEndpoint(endpoint string, defaultPath string) string {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	endpoint = strings.TrimSuffix(endpoint, "/v1/traces")
	endpoint = strings.TrimSuffix(endpoint, "/v1/metrics")
	return endpoint + defaultPath
}

func parseFloatOrDefault(s string) float64 {
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		if f >= 0 && f <= 1 {
			return f
		}
	}
	return 1.0 // 默认100%采样
}

// parseIntOrDefault 整数解析，失败返回默认值
func parseIntOrDefault(s string) int {
	if i, err := strconv.Atoi(s); err == nil && i >= 0 {
		return i
	}
	return 0
}

// parseDurationOrDefault 时间间隔解析，失败返回默认值
func parseDurationOrDefault(s string) time.Duration {
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	return 30 * time.Second // 默认30秒
}
