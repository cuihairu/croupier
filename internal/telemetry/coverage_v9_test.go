package telemetry

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

func TestNormalizeCountryV9(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"cN", "CN"},
		{"cn", "CN"},
		{"china", "CN"},
		{"CHINA", "CN"},
		{"us", "US"},
		{"usa", "US"},
		{"united states", "US"},
		{"jp", "JP"},
		{"japan", "JP"},
		{"kr", "KR"},
		{"korea", "KR"},
		{"de", "DE"},
		{"germany", "DE"},
		{"gb", "GB"},
		{"uk", "UK"}, // len==2 branch uppercases before the name table
		{"united kingdom", "GB"},
		{"brazil", ""},
		{"toolongcountryname", ""},
		{"   ", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := normalizeCountry(tc.in); got != tc.want {
			t.Errorf("normalizeCountry(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestWorkerPayloadFromEventCountryAndTraceV9(t *testing.T) {
	evt := AnalyticsEvent{
		EventType:  "session.start",
		GameID:     "g9",
		Region:     "japan",
		TraceID:    "tid-1",
		SpanID:     "sid-1",
		Attributes: map[string]interface{}{"k": "v"},
	}
	payload := workerPayloadFromEvent(evt)
	assert.Equal(t, "JP", payload["country"])
	props := payload["props"].(map[string]interface{})
	assert.Equal(t, "tid-1", props["trace_id"])
	assert.Equal(t, "sid-1", props["span_id"])
	assert.Equal(t, "v", props["k"])

	empty := workerPayloadFromEvent(AnalyticsEvent{})
	_, hasCountry := empty["country"]
	assert.False(t, hasCountry, "empty region must not emit country")
	_, hasProps := empty["props"]
	assert.False(t, hasProps, "no attributes must not emit props")
}

func TestNewAnalyticsBridgeDefaultsV9(t *testing.T) {
	mr := miniredis.RunT(t)
	cfg := AnalyticsBridgeConfig{
		Enabled:   true,
		RedisAddr: mr.Addr(),
		// FlushInterval / BatchSize left zero on purpose: constructor must
		// apply defensive defaults (NewTicker(0) would panic).
	}
	bridge := NewAnalyticsBridge(cfg, "game-9", nil)
	require.NotNil(t, bridge)
	assert.True(t, bridge.enabled)
	assert.Equal(t, 30*time.Second, bridge.flushInterval)
	assert.Equal(t, 100, bridge.batchSize)
	assert.NotNil(t, bridge.logger, "nil logger must fall back to default")
	require.NoError(t, bridge.Shutdown(context.Background()))
}

func TestBatchProcessorDisabledV9(t *testing.T) {
	b := &AnalyticsBridge{}
	done := make(chan struct{})
	go func() {
		b.batchProcessor()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("batchProcessor on disabled bridge must return immediately")
	}
}

func TestAnalyticsBridgeStopDrainFlushV9(t *testing.T) {
	bridge, _, client := newMiniRedisBridge(t, 100, time.Hour)
	bridge.batchChannel <- AnalyticsEvent{
		EventType: "session.end",
		GameID:    "game-1",
		Attributes: map[string]interface{}{
			"cause_end": "normal",
		},
	}
	require.NoError(t, bridge.Shutdown(context.Background()))
	waitForStreamLength(t, client, analyticsEventsStream, 1)

	// Second shutdown: stopCh already closed; idempotent guard must not
	// panic on double close.
	err := bridge.Shutdown(context.Background())
	assert.Error(t, err, "redis client is already closed on second shutdown")
}

func TestFlushBatchEmptyEarlyReturnV9(t *testing.T) {
	b := &AnalyticsBridge{enabled: true}
	assert.NotPanics(t, func() { b.flushBatch() })
}

func TestFlushBatchExecErrorV9(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	defer client.Close()
	b := &AnalyticsBridge{
		enabled:        true,
		logger:         slog.Default(),
		redisClient:    client,
		retentionHours: 1,
	}
	b.eventBatch = []AnalyticsEvent{{EventType: "x", GameID: "g"}}
	assert.NotPanics(t, func() { b.flushBatch() })
	assert.Empty(t, b.eventBatch, "batch must be cleared even when Redis is unreachable")
}

func TestSendProgressionEventExtraTypesV9(t *testing.T) {
	b := &AnalyticsBridge{}
	assert.NotPanics(t, func() {
		b.SendProgressionEvent(context.Background(), "progression.complete", nil, "u", "s", "lvl",
			map[string]interface{}{
				"str":   "x",
				"int":   1,
				"i64":   int64(2),
				"f64":   1.5,
				"bool":  true,
				"other": struct{}{},
			})
	})
}

func TestSendEconomyEventExtraTypesV9(t *testing.T) {
	b := &AnalyticsBridge{}
	assert.NotPanics(t, func() {
		b.SendEconomyEvent(context.Background(), "economy.spend", nil, "u", "gold", 9.5,
			map[string]interface{}{
				"str":   "y",
				"int":   2,
				"i64":   int64(3),
				"f64":   2.5,
				"bool":  false,
				"other": struct{}{},
			})
	})
}

func TestAnalyticsBridgeShutdownWithoutRedisV9(t *testing.T) {
	b := &AnalyticsBridge{enabled: true}
	require.NoError(t, b.Shutdown(context.Background()))
}

func TestInjectContextWithValidSpanV9(t *testing.T) {
	tid, err := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	require.NoError(t, err)
	sid, err := trace.SpanIDFromHex("0102030405060708")
	require.NoError(t, err)
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	md := InjectContext(ctx, nil)
	assert.Equal(t, tid.String(), md[MetadataTraceID])

	assert.Equal(t, tid.String(), TraceIDFromContext(ctx))
}

func TestSetMetricFieldUnknownFieldPanicsV9(t *testing.T) {
	defer func() {
		r := recover()
		require.NotNil(t, r, "unknown field must panic")
		msg, ok := r.(string)
		require.True(t, ok)
		assert.Contains(t, msg, "unknown GameMetrics field")
	}()
	setMetricField[int](&GameMetrics{}, "DefinitelyNotAField", 1)
}

func TestProviderInitMetricsHostPortFormV9(t *testing.T) {
	cfg := TelemetryConfig{
		ServiceName:   "svc",
		CollectorURL:  newOTLPStub(t), // host:port form without scheme
		EnableTracing: false,
		EnableMetrics: true,
	}
	p, err := NewProvider(context.Background(), cfg, nil)
	require.NoError(t, err)
	require.NotNil(t, p.MeterProvider)
	require.NoError(t, p.Shutdown(context.Background()))
}

func newClosedRedisBridgeV9(t *testing.T) *AnalyticsBridge {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	require.NoError(t, client.Close(), "first close succeeds")
	// Second Close inside bridge.Shutdown hits ErrClosed → bridge error.
	return &AnalyticsBridge{enabled: true, redisClient: client}
}

func TestProviderShutdownBridgeErrorV9(t *testing.T) {
	p := &Provider{Bridge: newClosedRedisBridgeV9(t)}
	err := p.Shutdown(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Analytics bridge")
}

func TestProviderShutdownBridgeAndTracerErrorsV9(t *testing.T) {
	cfg := TelemetryConfig{
		ServiceName:   "svc",
		CollectorURL:  "http://127.0.0.1:14318",
		EnableTracing: true,
		EnableMetrics: true,
	}
	p, err := NewProvider(context.Background(), cfg, nil)
	require.NoError(t, err)
	p.Bridge = newClosedRedisBridgeV9(t)

	_, span := p.GameTracer.tracer.Start(context.Background(), "pending")
	span.End()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = p.Shutdown(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Analytics bridge")
	assert.Contains(t, err.Error(), "TracerProvider")
}

func TestGameTelemetryServiceNilGuardsV9(t *testing.T) {
	ctx := context.Background()

	var nilService *GameTelemetryService
	nilService.BridgeFunctionCall(ctx, "function.call")
	_, span := nilService.StartSpan(ctx, "noop")
	require.NotNil(t, span)
	span.End()

	zero := &GameTelemetryService{}
	_, span2 := zero.StartSpan(ctx, "noop")
	require.NotNil(t, span2)
	span2.End()
	zero.BridgeFunctionCall(ctx, "function.call")
}

func TestGameTelemetryServiceProxiesAndBridgeV9(t *testing.T) {
	cfg := TelemetryConfig{ServiceName: "svc9"}
	svc, err := NewGameTelemetryService(cfg, nil)
	require.NoError(t, err)
	require.NotNil(t, svc.provider.Bridge, "bridge is always constructed")
	require.NoError(t, svc.Health(context.Background()))

	ctx := context.Background()
	levelCtx, span := svc.StartLevelPlaythrough(ctx, LevelStartRequest{LevelID: "L1", Difficulty: "hard"})
	svc.CompleteLevelPlaythrough(levelCtx, LevelCompleteRequest{LevelID: "L1", Stars: 3, Difficulty: "hard"})
	span.End()

	svc.TrackEconomyTransaction(ctx, EconomyTransaction{
		UserID: "u1", Currency: "gold", Amount: 5, Type: "earn", Source: "quest",
	})

	svc.BridgeFunctionCall(ctx, "function.call.success")
}
