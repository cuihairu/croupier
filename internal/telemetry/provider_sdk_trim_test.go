//go:build no_otel

package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

// --- no_otel 裁剪构建（todo.md:31 可禁用构建） ---

// 裁剪构建 + 配置启用 → fail-closed 显式报错（与 no_go_plugin 插件门禁
// 同策略），报错须指明 no_otel build tag，让操作员能定位重建方式。
func TestTrimBuildEnabledConfigFailsClosed(t *testing.T) {
	cfg := TelemetryConfig{
		Enabled:       true,
		ServiceName:   "svc",
		CollectorURL:  "localhost:4318",
		EnableTracing: true,
		EnableMetrics: true,
	}
	_, err := NewProvider(context.Background(), cfg, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no_otel")
	assert.Contains(t, err.Error(), "built without OTel support")

	_, err = NewGameTelemetryService(cfg, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create telemetry provider")
}

// 裁剪构建 + 关闭配置 → 正常启动轻量面：无 provider/无导出，业务指标
// 调用点（noop meter）与 trace-context 传播照常可用。
func TestTrimBuildDisabledConfigKeepsLightweightSurface(t *testing.T) {
	assert.True(t, otelAssemblyTrimmed, "no_otel build must report trimmed assembly")

	cfg := TelemetryConfig{Enabled: false, EnableTracing: true, EnableMetrics: true, CollectorURL: "localhost:4318"}
	p, err := NewProvider(context.Background(), cfg, nil)
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Nil(t, p.TracerProvider)
	assert.Nil(t, p.MeterProvider)
	assert.NotNil(t, p.GameMetrics)
	assert.NotNil(t, p.GameTracer)
	assert.NotNil(t, p.Bridge)
	require.NoError(t, p.Shutdown(context.Background()))
}

// 轻量 API/trace-context 保留范围：裁剪构建下注入/提取传播头仍能还原
// 远端 span 上下文（跨进程 trace 串联不依赖 SDK 装配）。
func TestTrimBuildRetainsTraceContextPropagation(t *testing.T) {
	cfg := TelemetryConfig{Enabled: false}
	_, err := NewProvider(context.Background(), cfg, nil)
	require.NoError(t, err)

	remoteSC := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
		SpanID:     trace.SpanID{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18},
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	metadata := InjectContext(trace.ContextWithSpanContext(context.Background(), remoteSC), nil)
	require.NotEmpty(t, metadata[MetadataTraceparent], "traceparent must be injected in trimmed build")

	got := ExtractContext(context.Background(), metadata)
	extracted := trace.SpanContextFromContext(got)
	require.True(t, extracted.IsValid())
	assert.Equal(t, remoteSC.TraceID(), extracted.TraceID())
	assert.Equal(t, remoteSC.SpanID(), extracted.SpanID())
	assert.Equal(t, remoteSC.TraceID().String(), TraceIDFromMetadata(metadata))
}
