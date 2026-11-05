//go:build !no_otel

// Package agentrec 实现 core/otelrec.Recorder 的真实 OTel 装配（agent 观测面，
// OBS-AGENT-001）。仅被 cmd/* agent 入口 import（agents/* 生产代码不得 import
// internal/，core/README.md 契约）；agent 侧只依赖 core/otelrec 接口与 noop
// 缺省。本包按 telemetry 开关构造 SDK meter provider（OTLP 周期导出）并实现
// Recorder 打点；no_otel 裁剪构建见 recorder_trim.go（fail-closed）。
//
// 指标面（与 server 同 semconv 风格）：
//   - agent.lifecycle     counter，phase=start|stop（启停）
//   - agent.connected     counter，state=1|0（连接状态事件）
//   - agent.events_total  counter，event=<名>（事件/吞吐计数）
//   - agent.observe_value histogram，event=<名>（观测值，如探测延迟）
//
// 错误率由调用方以 events_total{event="errors"} 表达（errors/events 可除）。
package agentrec

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"

	"github.com/cuihairu/croupier/core/otelrec"
)

// instrumentNames 供测试/面板引用（命名稳定契约）。
const (
	instrumentLifecycle = "agent.lifecycle"
	instrumentConnected = "agent.connected"
	instrumentEvents    = "agent.events_total"
	instrumentObserve   = "agent.observe_value"
)

const (
	meterName      = "croupier.agent"
	exportInterval = 30 * time.Second
)

// Recorder 是 otelrec.Recorder 的 OTel SDK 实现。mp==nil 时为零开销 noop
// （Enabled=false 或未配置采集端），方法与 Shutdown 均直通不 panic。
type Recorder struct {
	mp   *sdkmetric.MeterProvider
	base []attribute.KeyValue // Start 携带的 agent 标识，合并进后续打点

	mu        sync.Mutex
	lifecycle metric.Int64Counter
	connected metric.Int64Counter
	events    metric.Int64Counter
	observes  metric.Float64Histogram
	logger    *slog.Logger
}

// Build 按开关装配：Enabled=false → 零开销 noop；Enabled=true 且 endpoint
// 空 → 告警 + noop（不擅自向默认远端导出，与 server 同语义）；Enabled=true
// → 真实 OTLP recorder。no_otel 构建下 Enabled=true 返回错误（fail-closed，
// 不静默无导出）。
func Build(ctx context.Context, cfg otelrec.TelemetryConfig, logger *slog.Logger) (*Recorder, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if !cfg.Enabled {
		return &Recorder{logger: logger}, nil
	}
	if strings.TrimSpace(cfg.OTLPEndpoint) == "" {
		logger.Warn("agent telemetry enabled but no otlpEndpoint configured; export disabled",
			"serviceName", cfg.ServiceName)
		return &Recorder{logger: logger}, nil
	}
	return newOTLP(ctx, cfg, logger)
}

// newOTLP 构造真实 OTLP 周期导出 recorder。
func newOTLP(ctx context.Context, cfg otelrec.TelemetryConfig, logger *slog.Logger) (*Recorder, error) {
	opts := []otlpmetrichttp.Option{}
	if strings.Contains(cfg.OTLPEndpoint, "://") {
		opts = append(opts, otlpmetrichttp.WithEndpointURL(trimEndpoint(cfg.OTLPEndpoint, "/v1/metrics")))
	} else {
		opts = append(opts, otlpmetrichttp.WithEndpoint(cfg.OTLPEndpoint), otlpmetrichttp.WithURLPath("/v1/metrics"))
	}
	if cfg.Insecure {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}
	exp, err := otlpmetrichttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("agentrec otlp exporter: %w", err)
	}
	return newWithReader(ctx, cfg, sdkmetric.NewPeriodicReader(exp, sdkmetric.WithInterval(exportInterval)), logger)
}

// newWithReader 以注入 reader 构造 recorder（测试经 ManualReader 断言，
// 生产经 OTLP PeriodicReader）。不校验 reader 是否已带 resource——构造处
// 统一挂 resource。
func newWithReader(ctx context.Context, cfg otelrec.TelemetryConfig, reader sdkmetric.Reader, logger *slog.Logger) (*Recorder, error) {
	serviceName := cfg.ServiceName
	if strings.TrimSpace(serviceName) == "" {
		serviceName = "croupier-agent"
	}
	res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceNameKey.String(serviceName)))
	if err != nil {
		return nil, fmt.Errorf("agentrec resource: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(reader),
	)
	meter := mp.Meter(meterName)

	lifecycle, err := meter.Int64Counter(instrumentLifecycle, metric.WithDescription("agent lifecycle events"))
	if err != nil {
		return nil, fmt.Errorf("agentrec instrument %s: %w", instrumentLifecycle, err)
	}
	connected, err := meter.Int64Counter(instrumentConnected, metric.WithDescription("agent upstream connection state events"))
	if err != nil {
		return nil, fmt.Errorf("agentrec instrument %s: %w", instrumentConnected, err)
	}
	events, err := meter.Int64Counter(instrumentEvents, metric.WithDescription("agent event throughput counts"))
	if err != nil {
		return nil, fmt.Errorf("agentrec instrument %s: %w", instrumentEvents, err)
	}
	observes, err := meter.Float64Histogram(instrumentObserve, metric.WithDescription("agent observed values"))
	if err != nil {
		return nil, fmt.Errorf("agentrec instrument %s: %w", instrumentObserve, err)
	}

	return &Recorder{mp: mp, logger: logger, lifecycle: lifecycle, connected: connected, events: events, observes: observes}, nil
}

// Shutdown 停止周期导出（幂等；noop 直通）。
func (r *Recorder) Shutdown(ctx context.Context) error {
	if r.mp == nil {
		return nil
	}
	return r.mp.Shutdown(ctx)
}

func (r *Recorder) Start(ctx context.Context, attrs map[string]string) {
	r.mu.Lock()
	r.base = toAttributes(attrs)
	r.mu.Unlock()
	if r.mp == nil {
		return
	}
	r.lifecycle.Add(ctx, 1, metric.WithAttributes(r.withBase("phase", "start")...))
}

func (r *Recorder) Stop(ctx context.Context, err error) {
	if r.mp == nil {
		return
	}
	attrs := r.withBase("phase", "stop")
	if err != nil {
		attrs = append(attrs, attribute.String("error", err.Error()))
	}
	r.lifecycle.Add(ctx, 1, metric.WithAttributes(attrs...))
}

func (r *Recorder) Connected(ctx context.Context) {
	if r.mp == nil {
		return
	}
	r.connected.Add(ctx, 1, metric.WithAttributes(r.withBase("state", "1")...))
}

func (r *Recorder) Disconnected(ctx context.Context, err error) {
	if r.mp == nil {
		return
	}
	attrs := r.withBase("state", "0")
	if err != nil {
		attrs = append(attrs, attribute.String("error", err.Error()))
	}
	r.connected.Add(ctx, 1, metric.WithAttributes(attrs...))
	// 断连同时计一次错误（错误率分子），调用方亦可显式 Count("errors")。
	r.events.Add(ctx, 1, metric.WithAttributes(r.withBase("event", "errors")...))
}

func (r *Recorder) Count(ctx context.Context, event string, n int64, attrs map[string]string) {
	if r.mp == nil {
		return
	}
	r.events.Add(ctx, n, metric.WithAttributes(r.withAttrs(event, attrs)...))
}

func (r *Recorder) Observe(ctx context.Context, event string, value float64, attrs map[string]string) {
	if r.mp == nil {
		return
	}
	r.observes.Record(ctx, value, metric.WithAttributes(r.withAttrs(event, attrs)...))
}

// withBase 追加单个 attr 到 Start 基座。
func (r *Recorder) withBase(key, val string) []attribute.KeyValue {
	r.mu.Lock()
	base := make([]attribute.KeyValue, len(r.base))
	copy(base, r.base)
	r.mu.Unlock()
	return append(base, attribute.String(key, val))
}

// withAttrs 合并 Start 基座 + event 维度 + 调用方 attrs。
func (r *Recorder) withAttrs(event string, attrs map[string]string) []attribute.KeyValue {
	out := r.withBase("event", event)
	return append(out, toAttributes(attrs)...)
}

func toAttributes(m map[string]string) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(m))
	for k, v := range m {
		out = append(out, attribute.String(k, v))
	}
	return out
}

// trimEndpoint 与 server telemetry.trimOTLPEndpoint 同口径（带 scheme 的
// endpoint 按 path 归一，缺省拼 defaultPath）。
func trimEndpoint(endpoint, defaultPath string) string {
	e := strings.TrimRight(endpoint, "/")
	if !strings.Contains(e, "/v1/") {
		return e + defaultPath
	}
	return e
}
