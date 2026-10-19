//go:build !no_otel

package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// otelAssemblyTrimmed 标记当前二进制是否裁掉了 OTel SDK/exporter 装配
// （no_otel build tag）。默认构建（!no_otel，todo.md:31 默认集成）恒为
// false：完整装配可用。
const otelAssemblyTrimmed = false

// 可注入缝隙：OTel SDK 的 resource.New（仅 WithAttributes 时不存在失败
// 输入）、OTLP HTTP exporter 构造（本包从不组合 WithTLSClientConfig，
// errInsecureEndpointWithTLS 不可触达）的错误分支仅能通过测试注入触达。
var (
	resourceNew    = resource.New
	newTraceExpor  = otlptracehttp.New
	newMetricExpor = otlpmetrichttp.New
)

// assembleSDK 装配 OTel SDK tracer/meter provider 并注册到全局。仅在
// 总开关开启且非裁剪构建时被调用；子开关开启但未配置采集端时跳过该
// 信号的 exporter/周期导出任务（未配置采集端不擅自向默认远端导出，
// todo.md:31），并告警让操作员知道该信号实际不导出。
func assembleSDK(ctx context.Context, config TelemetryConfig, logger *slog.Logger) (shutdowner, shutdowner, error) {
	res, err := newSDKResource(ctx, config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create resource: %w", err)
	}

	var tracerProvider shutdowner
	if config.EnableTracing {
		if strings.TrimSpace(config.CollectorURL) == "" {
			warnCollectorMissing(logger, "tracing")
		} else {
			tp, err := initTracing(ctx, res, config)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to init tracing: %w", err)
			}
			otel.SetTracerProvider(tp)
			tracerProvider = tp
		}
	}

	var meterProvider shutdowner
	if config.EnableMetrics {
		if strings.TrimSpace(config.CollectorURL) == "" {
			warnCollectorMissing(logger, "metrics")
		} else {
			mp, err := initMetrics(ctx, res, config)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to init metrics: %w", err)
			}
			otel.SetMeterProvider(mp)
			meterProvider = mp
		}
	}

	return tracerProvider, meterProvider, nil
}

func warnCollectorMissing(logger *slog.Logger, signal string) {
	msg := "telemetry signal enabled but no collector endpoint configured; export disabled " +
		"(set telemetry.collector_url / OTEL_EXPORTER_OTLP_ENDPOINT to export)"
	if logger != nil {
		logger.Warn(msg, "signal", signal)
		return
	}
	slog.Warn(msg, "signal", signal)
}

// newSDKResource 创建资源标识
func newSDKResource(ctx context.Context, config TelemetryConfig) (*resource.Resource, error) {
	return resourceNew(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String(config.ServiceVersion),
			semconv.DeploymentEnvironmentKey.String(config.Environment),
			GameIDKey.String(config.GameID),
		),
	)
}

// initTracing 初始化链路追踪
func initTracing(ctx context.Context, res *resource.Resource, config TelemetryConfig) (*trace.TracerProvider, error) {
	// 构建OTLP HTTP导出器选项
	opts := []otlptracehttp.Option{}
	if strings.Contains(config.CollectorURL, "://") {
		opts = append(opts, otlptracehttp.WithEndpointURL(trimOTLPEndpoint(config.CollectorURL, "/v1/traces")))
	} else {
		opts = append(opts,
			otlptracehttp.WithEndpoint(config.CollectorURL),
			otlptracehttp.WithURLPath("/v1/traces"),
		)
	}

	// 根据配置决定是否使用TLS
	if config.UseTLS {
		// 生产环境使用HTTPS（默认不带WithInsecure就是HTTPS）
		// 不需要添加额外选项，因为otlptracehttp默认就是secure的
	} else {
		// 开发环境使用HTTP
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	traceExporter, err := newTraceExpor(ctx, opts...)
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
	// 构建OTLP HTTP导出器选项
	opts := []otlpmetrichttp.Option{}
	if strings.Contains(config.CollectorURL, "://") {
		opts = append(opts, otlpmetrichttp.WithEndpointURL(trimOTLPEndpoint(config.CollectorURL, "/v1/metrics")))
	} else {
		opts = append(opts,
			otlpmetrichttp.WithEndpoint(config.CollectorURL),
			otlpmetrichttp.WithURLPath("/v1/metrics"),
		)
	}

	// 根据配置决定是否使用TLS
	if config.UseTLS {
		// 生产环境使用HTTPS（默认就是secure的）
	} else {
		// 开发环境使用HTTP
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}

	metricExporter, err := newMetricExpor(ctx, opts...)
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
