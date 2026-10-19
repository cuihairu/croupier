//go:build !no_otel

package telemetry

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
)

// --- provider_sdk_default.go 缝隙注入：不可达错误分支（仅默认构建） ---

// resource.New 仅 WithAttributes 时不存在失败输入，错误分支只能注入触达；
// 同一失败顺带覆盖 NewGameTelemetryService 的 provider 包装错误。
func TestNewProvider_ResourceErrorSeam(t *testing.T) {
	orig := resourceNew
	resourceNew = func(ctx context.Context, opts ...resource.Option) (*resource.Resource, error) {
		return nil, errors.New("injected resource failure")
	}
	t.Cleanup(func() { resourceNew = orig })

	cfg := TelemetryConfig{Enabled: true, ServiceName: "svc"}
	_, err := NewProvider(context.Background(), cfg, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create resource")
	assert.Contains(t, err.Error(), "injected resource failure")

	_, err = NewGameTelemetryService(cfg, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create telemetry provider")
}

// otlptracehttp.New 在本包的选项组合下不可失败（不组合
// WithTLSClientConfig），initTracing 的错误分支只能注入触达。
func TestNewProvider_TraceExporterErrorSeam(t *testing.T) {
	orig := newTraceExpor
	newTraceExpor = func(ctx context.Context, opts ...otlptracehttp.Option) (*otlptrace.Exporter, error) {
		return nil, errors.New("injected trace exporter failure")
	}
	t.Cleanup(func() { newTraceExpor = orig })

	cfg := TelemetryConfig{Enabled: true, ServiceName: "svc", EnableTracing: true, CollectorURL: "localhost:4318"}
	_, err := NewProvider(context.Background(), cfg, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to init tracing")
	assert.Contains(t, err.Error(), "injected trace exporter failure")
}

// otlpmetrichttp.New 同理不可失败，initMetrics 错误分支只能注入触达。
func TestNewProvider_MetricExporterErrorSeam(t *testing.T) {
	orig := newMetricExpor
	newMetricExpor = func(ctx context.Context, opts ...otlpmetrichttp.Option) (*otlpmetrichttp.Exporter, error) {
		return nil, errors.New("injected metric exporter failure")
	}
	t.Cleanup(func() { newMetricExpor = orig })

	cfg := TelemetryConfig{Enabled: true, ServiceName: "svc", EnableMetrics: true, CollectorURL: "localhost:4318"}
	_, err := NewProvider(context.Background(), cfg, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to init metrics")
	assert.Contains(t, err.Error(), "injected metric exporter failure")
}

// --- 总开关/子开关装配行为（默认构建，hook 计数证明） ---

// exporterCallCount 用注入缝统计 exporter 构造次数，包装真实构造器保证
// 计数用例走的是生产同一条装配路径。
type exporterCallCount struct {
	traceCalls  int
	metricCalls int
	origTrace   func(ctx context.Context, opts ...otlptracehttp.Option) (*otlptrace.Exporter, error)
	origMetric  func(ctx context.Context, opts ...otlpmetrichttp.Option) (*otlpmetrichttp.Exporter, error)
}

func newExporterCallCount(t *testing.T) *exporterCallCount {
	t.Helper()
	c := &exporterCallCount{origTrace: newTraceExpor, origMetric: newMetricExpor}
	newTraceExpor = func(ctx context.Context, opts ...otlptracehttp.Option) (*otlptrace.Exporter, error) {
		c.traceCalls++
		return c.origTrace(ctx, opts...)
	}
	newMetricExpor = func(ctx context.Context, opts ...otlpmetrichttp.Option) (*otlpmetrichttp.Exporter, error) {
		c.metricCalls++
		return c.origMetric(ctx, opts...)
	}
	t.Cleanup(func() { newTraceExpor = c.origTrace; newMetricExpor = c.origMetric })
	return c
}

// 总开时子开关各自生效：tracing 单开→仅 trace exporter；metrics 单开→仅
// metric exporter；双开→两者；全关→零 exporter 构造。
func TestNewProvider_MasterOnSubSwitchesIndependent(t *testing.T) {
	requireFullAssembly(t)
	base := TelemetryConfig{ServiceName: "svc", CollectorURL: "http://" + newOTLPStub(t)}

	c := newExporterCallCount(t)
	p, err := NewProvider(context.Background(), TelemetryConfig{ServiceName: base.ServiceName, CollectorURL: base.CollectorURL, Enabled: true}, nil)
	require.NoError(t, err)
	require.NoError(t, p.Shutdown(context.Background()))
	assert.Equal(t, 0, c.traceCalls, "no sub-switch on must construct no exporter")
	assert.Equal(t, 0, c.metricCalls)

	p, err = NewProvider(context.Background(), TelemetryConfig{ServiceName: base.ServiceName, CollectorURL: base.CollectorURL, Enabled: true, EnableTracing: true}, nil)
	require.NoError(t, err)
	assert.NotNil(t, p.TracerProvider, "tracing sub-switch on must assemble tracer provider")
	assert.Nil(t, p.MeterProvider, "metrics sub-switch off must leave meter provider nil")
	require.NoError(t, p.Shutdown(context.Background()))
	assert.Equal(t, 1, c.traceCalls, "tracing sub-switch on must construct exactly one trace exporter")
	assert.Equal(t, 0, c.metricCalls)

	p, err = NewProvider(context.Background(), TelemetryConfig{ServiceName: base.ServiceName, CollectorURL: base.CollectorURL, Enabled: true, EnableMetrics: true}, nil)
	require.NoError(t, err)
	assert.Nil(t, p.TracerProvider, "tracing sub-switch off must leave tracer provider nil")
	assert.NotNil(t, p.MeterProvider, "metrics sub-switch on must assemble meter provider")
	require.NoError(t, p.Shutdown(context.Background()))
	assert.Equal(t, 1, c.traceCalls)
	assert.Equal(t, 1, c.metricCalls, "metrics sub-switch on must construct exactly one metric exporter")
}

// 未配置采集端不擅自向默认远端导出（todo.md:31）：总开 + 子开关全开 +
// CollectorURL 空 → 不构造任何 exporter、不建周期导出任务，轻量面照常。
func TestNewProvider_MasterOnNoCollectorNeverExports(t *testing.T) {
	requireFullAssembly(t)
	c := newExporterCallCount(t)

	cfg := TelemetryConfig{Enabled: true, ServiceName: "svc", EnableTracing: true, EnableMetrics: true, CollectorURL: ""}
	p, err := NewProvider(context.Background(), cfg, nil)
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Nil(t, p.TracerProvider, "no collector configured must not assemble tracer provider")
	assert.Nil(t, p.MeterProvider, "no collector configured must not assemble meter provider")
	assert.Equal(t, 0, c.traceCalls, "no collector configured must construct no trace exporter")
	assert.Equal(t, 0, c.metricCalls, "no collector configured must construct no metric exporter")
	require.NoError(t, p.Shutdown(context.Background()))
}

// 默认集成行为（todo.md:31）：默认构建（无 no_otel tag）恒含完整装配。
func TestDefaultBuildIncludesFullOTelAssembly(t *testing.T) {
	requireFullAssembly(t)
	assert.False(t, otelAssemblyTrimmed, "default build must include the OTel SDK/exporter assembly")

	c := newExporterCallCount(t)
	cfg := TelemetryConfig{Enabled: true, ServiceName: "svc", CollectorURL: "http://" + newOTLPStub(t), EnableTracing: true, EnableMetrics: true}
	p, err := NewProvider(context.Background(), cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, p.TracerProvider)
	assert.NotNil(t, p.MeterProvider)
	assert.Equal(t, 1, c.traceCalls)
	assert.Equal(t, 1, c.metricCalls)
	require.NoError(t, p.Shutdown(context.Background()))
}
