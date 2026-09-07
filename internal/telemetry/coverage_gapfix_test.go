package telemetry

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/sdk/resource"
)

// --- provider.go 缝隙注入：不可达错误分支 ---

// resource.New 仅 WithAttributes 时不存在失败输入，错误分支只能注入触达；
// 同一失败顺带覆盖 NewGameTelemetryService 的 provider 包装错误。
func TestNewProvider_ResourceErrorSeam(t *testing.T) {
	orig := resourceNew
	resourceNew = func(ctx context.Context, opts ...resource.Option) (*resource.Resource, error) {
		return nil, errors.New("injected resource failure")
	}
	t.Cleanup(func() { resourceNew = orig })

	cfg := TelemetryConfig{ServiceName: "svc"}
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

	cfg := TelemetryConfig{ServiceName: "svc", EnableTracing: true}
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

	cfg := TelemetryConfig{ServiceName: "svc", EnableMetrics: true}
	_, err := NewProvider(context.Background(), cfg, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to init metrics")
	assert.Contains(t, err.Error(), "injected metric exporter failure")
}

// NewGameMetrics 用全局 meter 创建固定合法指标名，不存在失败输入。
func TestNewProvider_GameMetricsErrorSeam(t *testing.T) {
	orig := providerMetrics
	providerMetrics = func(meter metric.Meter) (*GameMetrics, error) {
		return nil, errors.New("injected game metrics failure")
	}
	t.Cleanup(func() { providerMetrics = orig })

	cfg := TelemetryConfig{ServiceName: "svc"}
	_, err := NewProvider(context.Background(), cfg, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create game metrics")
	assert.Contains(t, err.Error(), "injected game metrics failure")
}

// --- setMetricField 类型不匹配 panic ---

// 值类型未实现字段接口时必须 panic 并给出可定位的报错。
func TestSetMetricFieldValueNotImplementingPanics(t *testing.T) {
	defer func() {
		r := recover()
		require.NotNil(t, r, "non-implementing value must panic")
		msg, ok := r.(string)
		require.True(t, ok)
		assert.Contains(t, msg, "does not implement")
		assert.Contains(t, msg, "SessionCounter")
	}()
	setMetricField(&GameMetrics{}, "SessionCounter", "not-a-counter")
}

// --- Health 的 nil-Bridge 兜底 ---

// 直接构造（不经 NewGameTelemetryService）的零值 Provider 无 Bridge，
// Health 必须返回 nil 而不是 panic。
func TestGameTelemetryServiceHealthNilBridge(t *testing.T) {
	svc := &GameTelemetryService{provider: &Provider{}}
	require.NoError(t, svc.Health(context.Background()))
}

// --- ExportPrometheus 的 Collect 错误分支 ---

// (*MetricsRegistry).Collect 恒返回 nil error，错误分支只能注入触达。
func TestExportPrometheusCollectErrorSeam(t *testing.T) {
	orig := registryCollect
	registryCollect = func(r *MetricsRegistry, ctx context.Context) ([]Metric, error) {
		return nil, errors.New("injected collect failure")
	}
	t.Cleanup(func() { registryCollect = orig })

	out, err := NewMetricsRegistry().ExportPrometheus()
	require.Error(t, err)
	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "injected collect failure")
}

// --- JSONSpanExporter 输出通道满时丢弃 ---

func TestJSONSpanExporterChannelFullDrops(t *testing.T) {
	out := make(chan []byte, 1)
	exporter := NewJSONSpanExporter(out)

	provider := NewTracerProvider()
	spans := make([]*Span, 0, 3)
	for i := 0; i < 3; i++ {
		span, _ := provider.StartSpan(context.Background(), "drop-me")
		spans = append(spans, span)
	}

	require.NoError(t, exporter.ExportSpans(context.Background(), spans))
	assert.Len(t, out, 1, "buffer of 1 must keep first span and drop the rest without blocking")
}

// --- EndUserSession 非正常结束（crash）走 Error 状态分支 ---

// cause=crash 必须：事件落 stream（cause_end=crash）、不 panic、
// 状态分支走 codes.Error 路径（SpanFromContext 为 noop span 时同样可达）。
func TestGameTracerEndUserSessionCrashCause(t *testing.T) {
	bridge, _, client := newMiniRedisBridge(t, 100, time.Hour)
	meter := metricnoop.NewMeterProvider().Meter("test")
	metrics, err := NewGameMetrics(meter)
	require.NoError(t, err)
	tracer := NewGameTracer(otel.Tracer("test"), metrics, bridge)

	assert.NotPanics(t, func() {
		tracer.EndUserSession(context.Background(), SessionEndRequest{
			UserID:     "u-crash",
			SessionID:  "s-crash",
			DurationMs: 42000,
			CauseOfEnd: "crash",
		})
	})

	require.NoError(t, bridge.Shutdown(context.Background()))
	waitForStreamLength(t, client, analyticsEventsStream, 1)

	msgs, err := client.XRange(context.Background(), analyticsEventsStream, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	raw, ok := msgs[0].Values["data"].(string)
	require.True(t, ok, "stream entry must carry data field")
	payload := decodeWorkerPayload(t, raw)
	props, ok := payload["props"].(map[string]interface{})
	require.True(t, ok, "props must be an object: %+v", payload)
	assert.Equal(t, "crash", props["cause_end"])
	assert.Equal(t, "u-crash", payload["user_id"])
}

// --- batchProcessor 停止排空分支 ---

// Shutdown 排空期间仍在 batchChannel 中的事件必须被捞起并入批，
// 而不是丢失。外层 select 在 (事件就绪, stopCh 就绪) 同时满足时随机
// 选择，重复多轮使停止分支被选中，断言每一轮事件都恰好落 stream。
func TestBatchProcessorStopDrainPicksUpBufferedEvent(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	const rounds = 150
	for i := 0; i < rounds; i++ {
		b := &AnalyticsBridge{
			enabled:       true,
			logger:        logger,
			redisClient:   client,
			gameID:        "game-drain",
			batchSize:     10,
			flushInterval: time.Hour,
			batchChannel:  make(chan AnalyticsEvent, 4),
			stopCh:        make(chan struct{}),
		}
		b.batchChannel <- AnalyticsEvent{EventType: "session.end", GameID: "game-drain"}
		close(b.stopCh)

		done := make(chan struct{})
		go func() {
			b.batchProcessor()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("round %d: batchProcessor did not terminate", i)
		}

		assert.Empty(t, b.eventBatch, "round %d: drain must flush buffered event", i)
		n, err := client.XLen(context.Background(), analyticsEventsStream).Result()
		require.NoError(t, err)
		require.EqualValues(t, i+1, n, "round %d: exactly one event must land in stream", i)
	}
}
