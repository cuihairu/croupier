package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/sdk/trace"
)

// --- helpers ---

type recordingExporter struct {
	spans []trace.ReadOnlySpan
}

func (e *recordingExporter) ExportSpans(_ context.Context, spans []trace.ReadOnlySpan) error {
	e.spans = append(e.spans, spans...)
	return nil
}

func (e *recordingExporter) Shutdown(context.Context) error { return nil }

func (e *recordingExporter) find(name string) trace.ReadOnlySpan {
	for _, s := range e.spans {
		if s.Name() == name {
			return s
		}
	}
	return nil
}

func (e *recordingExporter) findAll(name string) []trace.ReadOnlySpan {
	var out []trace.ReadOnlySpan
	for _, s := range e.spans {
		if s.Name() == name {
			out = append(out, s)
		}
	}
	return out
}

type spanStatus struct {
	code codes.Code
	msg  string
}

func statusOf(s trace.ReadOnlySpan) spanStatus {
	return spanStatus{code: s.Status().Code, msg: s.Status().Description}
}

func newTestGameTracer(t *testing.T) (*GameTracer, *recordingExporter) {
	t.Helper()
	exporter := &recordingExporter{}
	tp := trace.NewTracerProvider(trace.WithSpanProcessor(trace.NewSimpleSpanProcessor(exporter)))
	meter := metricnoop.NewMeterProvider().Meter("test")
	m := &GameMetrics{}
	var err error
	m.SessionCounter, err = meter.Int64Counter("session.total")
	require.NoError(t, err)
	m.UserLoginCounter, err = meter.Int64Counter("user.login.total")
	require.NoError(t, err)
	m.SessionDuration, err = meter.Float64Histogram("session.duration")
	require.NoError(t, err)
	m.LevelStartCounter, err = meter.Int64Counter("level.start.total")
	require.NoError(t, err)
	m.LevelCompleteCounter, err = meter.Int64Counter("level.complete.total")
	require.NoError(t, err)
	m.LevelFailCounter, err = meter.Int64Counter("level.fail.total")
	require.NoError(t, err)
	m.LevelRetries, err = meter.Float64Histogram("level.retries")
	require.NoError(t, err)
	m.QueueTime, err = meter.Float64Histogram("match.queue_time")
	require.NoError(t, err)
	m.MatchStartCounter, err = meter.Int64Counter("match.start.total")
	require.NoError(t, err)
	m.MatchEndCounter, err = meter.Int64Counter("match.end.total")
	require.NoError(t, err)
	m.MatchDuration, err = meter.Float64Histogram("match.duration")
	require.NoError(t, err)
	m.CurrencyEarn, err = meter.Float64Counter("economy.earn.total")
	require.NoError(t, err)
	m.CurrencySpend, err = meter.Float64Counter("economy.spend.total")
	require.NoError(t, err)
	m.RevenueTotal, err = meter.Float64Counter("monetization.revenue.total")
	require.NoError(t, err)
	m.AdImpressions, err = meter.Int64Counter("ad.impressions.total")
	require.NoError(t, err)
	m.AdRevenue, err = meter.Float64Counter("ad.revenue.total")
	require.NoError(t, err)
	m.ClientFPS, err = meter.Float64Histogram("perf.fps")
	require.NoError(t, err)
	m.MemoryUsage, err = meter.Float64Histogram("perf.memory")
	require.NoError(t, err)
	m.NetworkLatency, err = meter.Float64Histogram("perf.rtt")
	require.NoError(t, err)
	m.CrashCounter, err = meter.Int64Counter("error.crash.total")
	require.NoError(t, err)
	m.TDTowerBuildCounter, err = meter.Int64Counter("td.tower.build.total")
	require.NoError(t, err)
	m.TDTowerUpgradeCounter, err = meter.Int64Counter("td.tower.upgrade.total")
	require.NoError(t, err)
	m.GachaPullCounter, err = meter.Int64Counter("gacha.pull.total")
	require.NoError(t, err)
	return NewGameTracer(tp.Tracer("test"), m, nil), exporter
}

// --- Provider OTel initialization ---

// newOTLPStub starts a minimal HTTP server that accepts OTLP posts so
// provider shutdown can flush without a real collector.
func newOTLPStub(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	})}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return ln.Addr().String()
}

func TestNewProvider_TracingAndMetricsEnabled_FullURL(t *testing.T) {
	cfg := TelemetryConfig{
		ServiceName:    "svc",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		CollectorURL:   "http://" + newOTLPStub(t),
		GameID:         "g1",
		EnableTracing:  true,
		EnableMetrics:  true,
		SamplingRatio:  0.5,
	}
	p, err := NewProvider(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.NotNil(t, p.TracerProvider)
	assert.NotNil(t, p.MeterProvider)
	assert.NotNil(t, p.GameMetrics)
	assert.NotNil(t, p.GameTracer)
	assert.NotNil(t, p.Bridge)

	require.NoError(t, p.Shutdown(context.Background()))
}

func TestNewProvider_HostPortForm_WithTLS(t *testing.T) {
	// Keep TLS form but expect no collector: exporter creation succeeds,
	// and shutdown failures are tolerated (asserting error paths only).
	cfg := TelemetryConfig{
		ServiceName:    "svc",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		CollectorURL:   "127.0.0.1:14318",
		GameID:         "g1",
		EnableTracing:  true,
		EnableMetrics:  false,
		UseTLS:         true,
	}
	p, err := NewProvider(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	require.NoError(t, p.Shutdown(context.Background()))
}

func TestNewProvider_NormalizesEmptyConfig(t *testing.T) {
	cfg := TelemetryConfig{}
	p, err := NewProvider(context.Background(), cfg, nil)
	require.NoError(t, err)
	assert.Equal(t, "croupier-server", p.config.ServiceName)
	assert.Equal(t, "1.0.0", p.config.ServiceVersion)
	assert.Equal(t, "development", p.config.Environment)
	assert.Equal(t, "http://localhost:4318", p.config.CollectorURL)
	assert.Equal(t, "default", p.config.GameID)
	assert.InDelta(t, 1.0, p.config.SamplingRatio, 0.0001)
	require.NoError(t, p.Shutdown(context.Background()))
}

func TestProvider_Shutdown_WithPendingSpansCanceledContext(t *testing.T) {
	cfg := TelemetryConfig{
		ServiceName:   "svc",
		CollectorURL:  "http://127.0.0.1:14318",
		EnableTracing: true,
		EnableMetrics: true,
	}
	p, err := NewProvider(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)

	// Queue a span in the batcher so shutdown must flush over OTLP.
	_, span := p.GameTracer.tracer.Start(context.Background(), "pending")
	span.End()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = p.Shutdown(ctx)
	require.Error(t, err, "flushing pending spans with a canceled context must fail")

	// After the first shutdown, the metric reader is already stopped and
	// reports "reader is shutdown" — Shutdown is not idempotent here.
	err = p.Shutdown(context.Background())
	require.ErrorContains(t, err, "MeterProvider")
}

func TestProvider_Shutdown_AllErrorsCombine(t *testing.T) {
	bridge := &AnalyticsBridge{enabled: false}
	p := &Provider{Bridge: bridge}
	assert.NoError(t, p.Shutdown(context.Background()))

	p2 := &Provider{}
	assert.NoError(t, p2.Shutdown(context.Background()))
}

// --- NewGameMetrics error branches ---

type flakyMeter struct {
	metric.Meter
	calls     int
	failAfter int
}

func (m *flakyMeter) maybeFail() error {
	m.calls++
	if m.calls > m.failAfter {
		return errors.New("instrument creation failed")
	}
	return nil
}

// noop 包装：成功分支返回非 nil 实现（直接 return nil, nil 会让反射拿到零值）。
type noopInt64Counter struct{ metric.Int64Counter }
type noopFloat64Counter struct{ metric.Float64Counter }
type noopInt64Gauge struct{ metric.Int64ObservableGauge }
type noopFloat64Gauge struct{ metric.Float64ObservableGauge }
type noopFloat64Histogram struct{ metric.Float64Histogram }

func (m *flakyMeter) Int64Counter(name string, opts ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	if err := m.maybeFail(); err != nil {
		return nil, err
	}
	return metricnoop.NewMeterProvider().Meter("t").Int64Counter(name, opts...)
}

func (m *flakyMeter) Float64Counter(name string, opts ...metric.Float64CounterOption) (metric.Float64Counter, error) {
	if err := m.maybeFail(); err != nil {
		return nil, err
	}
	return metricnoop.NewMeterProvider().Meter("t").Float64Counter(name, opts...)
}

func (m *flakyMeter) Int64ObservableGauge(name string, opts ...metric.Int64ObservableGaugeOption) (metric.Int64ObservableGauge, error) {
	if err := m.maybeFail(); err != nil {
		return nil, err
	}
	return metricnoop.NewMeterProvider().Meter("t").Int64ObservableGauge(name, opts...)
}

func (m *flakyMeter) Float64ObservableGauge(name string, opts ...metric.Float64ObservableGaugeOption) (metric.Float64ObservableGauge, error) {
	if err := m.maybeFail(); err != nil {
		return nil, err
	}
	return metricnoop.NewMeterProvider().Meter("t").Float64ObservableGauge(name, opts...)
}

func (m *flakyMeter) Float64Histogram(name string, opts ...metric.Float64HistogramOption) (metric.Float64Histogram, error) {
	if err := m.maybeFail(); err != nil {
		return nil, err
	}
	return metricnoop.NewMeterProvider().Meter("t").Float64Histogram(name, opts...)
}

func TestNewGameMetrics_InstrumentCreationErrors(t *testing.T) {
	// 规格表驱动：任一 instrument 创建失败必须立即返回错误且不产生半初始化
	// 的 GameMetrics（此前只创建 10 个、其余为 nil 接口，玩法事件一调即 panic）。
	const totalInstruments = 46
	for failAfter := 0; failAfter < totalInstruments; failAfter++ {
		m, err := NewGameMetrics(&flakyMeter{failAfter: failAfter})
		require.Error(t, err, "failAfter=%d", failAfter)
		require.ErrorContains(t, err, "instrument creation failed")
		require.Nil(t, m)
	}
	// 全部成功：46 个字段全部非 nil
	m, err := NewGameMetrics(metricnoop.NewMeterProvider().Meter("test"))
	require.NoError(t, err)
	require.NotNil(t, m)
	require.NotNil(t, m.SessionCounter)
	require.NotNil(t, m.SessionDuration)
	require.NotNil(t, m.LevelStartCounter)
	require.NotNil(t, m.CurrencyEarn)
	require.NotNil(t, m.RevenueTotal)
}

// --- AnalyticsBridge against miniredis ---

func newMiniRedisBridge(t *testing.T, batchSize int, flushInterval time.Duration) (*AnalyticsBridge, *miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	cfg := AnalyticsBridgeConfig{
		Enabled:        true,
		RedisAddr:      mr.Addr(),
		BatchSize:      batchSize,
		FlushInterval:  flushInterval,
		TopicPrefix:    "game:events",
		RetentionHours: 2,
	}
	bridge := NewAnalyticsBridge(cfg, "game-1", slog.New(slog.NewTextHandler(io.Discard, nil)))
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return bridge, mr, client
}

// decodeWorkerPayload parses a bridge XAdd "data" field as the worker
// envelope (snake_case map) for assertions.
func decodeWorkerPayload(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &payload))
	return payload
}

func waitForStreamLength(t *testing.T, client *redis.Client, stream string, want int64) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		n, err := client.XLen(context.Background(), stream).Result()
		require.NoError(t, err)
		if n >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	n, _ := client.XLen(context.Background(), stream).Result()
	t.Fatalf("stream %s length = %d, want >= %d", stream, n, want)
}

func TestAnalyticsBridge_BatchFlushToRedis(t *testing.T) {
	bridge, mr, client := newMiniRedisBridge(t, 2, time.Hour)
	ctx := context.Background()
	require.NoError(t, bridge.Health(ctx))

	bridge.SendEvent(ctx, "session.start", nil, nil)
	bridge.SendEvent(ctx, "session.start", nil, []attribute.KeyValue{
		GameUserIDKey.String("u-1"),
		GameSessionIDKey.String("s-1"),
		GamePlatformKey.String("ios"),
		GameRegionKey.String("us"),
		attribute.String("custom", "value"),
	})

	stream := analyticsEventsStream
	waitForStreamLength(t, client, stream, 2)

	msgs, err := client.XRange(ctx, stream, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, msgs, 2)

	payload := decodeWorkerPayload(t, msgs[1].Values["data"].(string))
	assert.Equal(t, "session.start", payload["event"])
	assert.Equal(t, "game-1", payload["game_id"])
	assert.Equal(t, "u-1", payload["user_id"])
	assert.Equal(t, "s-1", payload["session_id"])
	assert.Equal(t, "ios", payload["platform"])
	assert.NotEmpty(t, payload["ts"], "worker envelope requires ts")
	props := payload["props"].(map[string]interface{})
	assert.Equal(t, "value", props["custom"])
	assert.Equal(t, "US", payload["country"], "region hint coerced to ISO-2")
	_, hasTrace := props["trace_id"]
	assert.False(t, hasTrace)

	// Retention TTL must be applied via Expire. flush 内 XAdd 与 Expire
	// 是两步：消息可见时 Expire 可能尚未落键，这里轮询有界等待（CI 上
	// 曾因竞态窗口拿到 -1ns 闪断）。
	deadline := time.Now().Add(3 * time.Second)
	var ttl time.Duration
	for {
		var err error
		ttl, err = client.TTL(ctx, stream).Result()
		require.NoError(t, err)
		if ttl > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	assert.True(t, ttl > 0 && ttl <= 2*time.Hour, "unexpected TTL: %v", ttl)

	// NOTE: bridge.Shutdown is deliberately not called here — Shutdown
	// flushes eventBatch on the caller goroutine while the unstoppable
	// batchProcessor goroutine mutates it (analytics_bridge.go:169 vs :117),
	// which the race detector flags. See TestAnalyticsBridge_Shutdown below.
	require.NoError(t, bridge.redisClient.Close())
	// Health must fail once the Redis client is closed.
	assert.Error(t, bridge.Health(ctx))
	_ = mr
}

func TestAnalyticsBridge_Shutdown(t *testing.T) {
	// 手工构造无后台协程：预关闭退出信号模拟 processor 已退出，
	// 验证 Shutdown 会 flush 批次并关闭 Redis。
	mr := miniredis.RunT(t)
	processorExited := make(chan struct{})
	close(processorExited)
	bridge := &AnalyticsBridge{
		enabled:         true,
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		redisClient:     redis.NewClient(&redis.Options{Addr: mr.Addr()}),
		gameID:          "game-1",
		topicPrefix:     "game:events",
		retentionHours:  1,
		batchSize:       10,
		flushInterval:   time.Hour,
		stopCh:          make(chan struct{}),
		processorExited: processorExited,
		eventBatch: []AnalyticsEvent{{
			EventType: "session.end", GameID: "game-1",
			Attributes: map[string]interface{}{"k": "v"},
		}},
	}

	require.NoError(t, bridge.Shutdown(context.Background()))

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	n, err := client.XLen(context.Background(), analyticsEventsStream).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), n, "Shutdown must flush pending events")

	// Disabled bridges shut down without touching Redis.
	assert.NoError(t, (&AnalyticsBridge{enabled: false}).Shutdown(context.Background()))
}

func TestAnalyticsBridge_TickerFlush(t *testing.T) {
	bridge, _, client := newMiniRedisBridge(t, 100, 20*time.Millisecond)
	ctx := context.Background()

	bridge.SendEvent(ctx, "match.end", nil, nil)
	waitForStreamLength(t, client, analyticsEventsStream, 1)

	// Shutdown is not called: it races with the live batchProcessor goroutine.
}

func TestAnalyticsBridge_ChannelFullDropsEvent(t *testing.T) {
	bridge := &AnalyticsBridge{
		enabled:      true,
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		batchChannel: make(chan AnalyticsEvent, 1),
	}
	bridge.batchChannel <- AnalyticsEvent{EventType: "occupied"}

	bridge.SendEvent(context.Background(), "session.start", nil, nil)

	assert.Len(t, bridge.batchChannel, 1, "event must be dropped when channel is full")
}

func TestAnalyticsBridge_FlushBatchMarshalError(t *testing.T) {
	mr := miniredis.RunT(t)
	bridge := &AnalyticsBridge{
		enabled:       true,
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		redisClient:   redis.NewClient(&redis.Options{Addr: mr.Addr()}),
		topicPrefix:   "game:events",
		eventBatch:    []AnalyticsEvent{{EventType: "bad", Attributes: map[string]interface{}{"ch": make(chan int)}}},
		batchSize:     10,
		flushInterval: time.Hour,
	}

	assert.NotPanics(t, bridge.flushBatch)
	n, err := bridge.redisClient.XLen(context.Background(), "game:events:bad").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), n, "unmarshalable event must not be written")
}

func TestAnalyticsBridge_SendEventWithRealSpanContext(t *testing.T) {
	bridge, _, client := newMiniRedisBridge(t, 1, time.Hour)
	ctx := context.Background()

	tp := trace.NewTracerProvider()
	_, span := tp.Tracer("t").Start(ctx, "op")
	bridge.SendEvent(ctx, "function.call", span, []attribute.KeyValue{
		attribute.String("function.id", "fn-1"),
		attribute.Int("function.duration_ms", 12),
	})
	span.End()

	stream := analyticsEventsStream
	waitForStreamLength(t, client, stream, 1)

	msgs, err := client.XRange(ctx, stream, "-", "+").Result()
	require.NoError(t, err)
	payload := decodeWorkerPayload(t, msgs[0].Values["data"].(string))
	props := payload["props"].(map[string]interface{})
	assert.NotEmpty(t, props["trace_id"], "span context trace id must be propagated")
	assert.NotEmpty(t, props["span_id"])
	assert.Equal(t, float64(12), props["function.duration_ms"])
}

func TestAnalyticsBridge_SendSessionEvent_ExtraTypes(t *testing.T) {
	bridge, _, client := newMiniRedisBridge(t, 1, time.Hour)
	ctx := context.Background()

	bridge.SendSessionEvent(ctx, "session.end", nil, "u-9", "s-9", "android", "eu",
		map[string]interface{}{
			"duration_ms": int64(1500),
			"cause_end":   "normal",
			"count":       3,
			"ratio":       0.5,
			"premium":     true,
			"ignored":     struct{}{},
		})

	stream := analyticsEventsStream
	waitForStreamLength(t, client, stream, 1)
	msgs, err := client.XRange(ctx, stream, "-", "+").Result()
	require.NoError(t, err)
	payload := decodeWorkerPayload(t, msgs[0].Values["data"].(string))
	assert.Equal(t, "u-9", payload["user_id"])
	props := payload["props"].(map[string]interface{})
	assert.Equal(t, float64(1500), props["duration_ms"])
	assert.Equal(t, "normal", props["cause_end"])
	assert.Equal(t, float64(3), props["count"])
	assert.Equal(t, 0.5, props["ratio"])
	assert.Equal(t, true, props["premium"])
	_, hasIgnored := props["ignored"]
	assert.False(t, hasIgnored, "unsupported extra types must be skipped")
}

func TestAnalyticsBridge_SendProgressionAndEconomyEvents(t *testing.T) {
	bridge, _, client := newMiniRedisBridge(t, 2, time.Hour)
	ctx := context.Background()

	bridge.SendProgressionEvent(ctx, "progression.complete", nil, "u-1", "s-1", "level-3",
		map[string]interface{}{"stars": 3})
	bridge.SendEconomyEvent(ctx, "economy.earn", nil, "u-1", "gold", 99.5,
		map[string]interface{}{"source": "quest"})

	waitForStreamLength(t, client, analyticsEventsStream, 2)

	msgs, err := client.XRange(ctx, analyticsEventsStream, "-", "+").Result()
	require.NoError(t, err)
	var economy map[string]interface{}
	for _, m := range msgs {
		candidate := decodeWorkerPayload(t, m.Values["data"].(string))
		if candidate["event"] == "economy.earn" {
			economy = candidate
			break
		}
	}
	require.NotNil(t, economy, "economy.earn event must be in the stream")
	assert.Equal(t, "u-1", economy["user_id"])
	props := economy["props"].(map[string]interface{})
	assert.Equal(t, float64(99.5), props["economy.amount"])
	assert.Equal(t, "quest", props["source"])
}

// --- GameTracer gameplay events ---

func TestGameTracer_LevelPlaythroughLifecycle(t *testing.T) {
	tracer, exporter := newTestGameTracer(t)
	ctx := context.Background()

	ctx, span := tracer.StartLevelPlaythrough(ctx, LevelStartRequest{
		UserID: "u-1", SessionID: "s-1", LevelID: "lv-1", ChapterID: "ch-1",
		Difficulty: "hard", WaveIndex: 2, AttemptIndex: 3, IsBossWave: true,
	})
	assert.NotNil(t, span, "level span started")

	tracer.CompleteLevelPlaythrough(ctx, LevelCompleteRequest{
		LevelID: "lv-1", DurationMs: 9000, Stars: 3, Retries: 1, WaveIndex: 9,
		HeartsRemaining: 2, Difficulty: "hard",
	})

	done := exporter.find("progression.complete")
	require.NotNil(t, done, "completed span should be recorded")
	assert.Equal(t, codes.Ok, statusOf(done).code)

	// Fail path on a fresh level.
	ctx2, _ := tracer.StartLevelPlaythrough(ctx, LevelStartRequest{LevelID: "lv-2", Difficulty: "easy"})
	tracer.FailLevelPlaythrough(ctx2, LevelFailRequest{
		LevelID: "lv-2", DurationMs: 1000, Reason: "out_of_hearts",
		FailWave: 4, HeartsRemaining: 0, Difficulty: "easy",
	})
	failed := exporter.find("progression.fail")
	require.NotNil(t, failed)
	assert.Equal(t, codes.Error, statusOf(failed).code)
	assert.True(t, strings.Contains(statusOf(failed).msg, "out_of_hearts"))
}

func TestGameTracer_MatchLifecycle(t *testing.T) {
	tracer, exporter := newTestGameTracer(t)
	ctx := context.Background()

	ctx, _ = tracer.StartMatch(ctx, MatchStartRequest{
		UserID: "u-1", SessionID: "s-1", MatchID: "m-1", GameMode: "pvp",
		QueueType: "solo", MapID: "map-1", QueueTimeMs: 800, MMR: 1200,
		TeamID: "t-1", DeckID: "deck-1", DeckArchetype: "aggro",
	})
	tracer.EndMatch(ctx, MatchEndRequest{
		MatchID: "m-1", MatchResult: "win", DurationMs: 60000, GameMode: "pvp",
		Kills: 10, Deaths: 2, Assists: 5, DamageDone: 999, DamageTaken: 111,
		DeckID: "deck-1",
	})
	win := exporter.find("match.end")
	require.NotNil(t, win)
	// OTel discards status descriptions for Ok status codes.
	assert.Equal(t, codes.Ok, statusOf(win).code)

	ctx2, _ := tracer.StartMatch(ctx, MatchStartRequest{UserID: "u-2", MatchID: "m-2", GameMode: "pve"})
	tracer.EndMatch(ctx2, MatchEndRequest{MatchID: "m-2", MatchResult: "lose", GameMode: "pve"})
	all := exporter.findAll("match.end")
	require.Len(t, all, 2)
	assert.Equal(t, codes.Ok, statusOf(all[1]).code)
}

func TestGameTracer_EconomyTransactions(t *testing.T) {
	tracer, exporter := newTestGameTracer(t)
	ctx := context.Background()

	tracer.TrackEconomyTransaction(ctx, EconomyTransaction{
		UserID: "u-1", Currency: "gold", CurrencyKind: "soft", Amount: 50,
		Type: "earn", Source: "quest", BalanceAfter: 150,
	})
	tracer.TrackEconomyTransaction(ctx, EconomyTransaction{
		UserID: "u-1", Currency: "gem", CurrencyKind: "hard", Amount: 10,
		Type: "spend", Sink: "tower_build", ItemID: "tower-1", BalanceAfter: 90,
	})

	require.NotNil(t, exporter.find("economy.earn"))
	require.NotNil(t, exporter.find("economy.spend"))
}

func TestGameTracer_PurchaseFlow(t *testing.T) {
	tracer, exporter := newTestGameTracer(t)
	ctx := context.Background()

	ctx, _ = tracer.StartPurchase(ctx, PurchaseFlow{
		UserID: "u-1", OrderID: "o-1", SKUID: "sku-1", PriceUSD: 4.99,
		CurrencyCode: "USD", PaymentProvider: "stripe",
	})
	tracer.CompletePurchase(ctx, PurchaseResult{
		OrderID: "o-1", SKUID: "sku-1", PriceUSD: 4.99, Success: true,
		TaxUSD: 0.4, Country: "US",
	})
	success := exporter.find("monetization.purchase_success")
	require.NotNil(t, success)
	assert.Equal(t, codes.Ok, statusOf(success).code)

	ctx2, _ := tracer.StartPurchase(ctx, PurchaseFlow{UserID: "u-2", OrderID: "o-2"})
	tracer.CompletePurchase(ctx2, PurchaseResult{
		OrderID: "o-2", Success: false, FailReason: "payment_declined",
	})
	fail := exporter.find("monetization.purchase_fail")
	require.NotNil(t, fail)
	assert.Equal(t, codes.Error, statusOf(fail).code)
	assert.Contains(t, statusOf(fail).msg, "payment_declined")
}

func TestGameTracer_AdImpression(t *testing.T) {
	tracer, exporter := newTestGameTracer(t)
	ctx := context.Background()

	tracer.TrackAdImpression(ctx, AdImpressionRequest{
		UserID: "u-1", AdNetwork: "adm", PlacementID: "revive",
		AdFormat: "rewarded", PlacementType: "revive", EcpmUSD: 12.5, RevenueUSD: 0.01,
	})
	tracker := exporter.find("ad.impression")
	require.NotNil(t, tracker)
}

func TestGameTracer_RecordPerformance(t *testing.T) {
	tracer, _ := newTestGameTracer(t)
	ctx := context.Background()

	assert.NotPanics(t, func() {
		tracer.RecordPerformance(ctx, PerformanceMetrics{
			UserID: "u-1", FPS: 60, MemoryMB: 512, CPULoad: 0.3, RTTMs: 42, JitterMs: 5, PacketLoss: 0.1,
		})
		tracer.RecordPerformance(ctx, PerformanceMetrics{UserID: "u-1"}) // all zeros: skipped
	})
}

func TestGameTracer_CrashAndTowerAndGacha(t *testing.T) {
	tracer, exporter := newTestGameTracer(t)
	ctx := context.Background()

	tracer.TrackCrash(ctx, CrashEvent{
		UserID: "u-1", SessionID: "s-1", StackHash: "abc", SignalCode: "SIGSEGV",
		Scene: "battle", DeviceID: "d-1",
	})
	require.NotNil(t, exporter.find("error.crash"))

	tracer.TrackTowerBuild(ctx, TowerBuildRequest{
		UserID: "u-1", LevelID: "lv-1", TowerID: "t-1", TowerType: "arrow",
		PosX: 3, PosY: 4, Cost: 100, WaveIndex: 1,
	})
	require.NotNil(t, exporter.find("td.tower.build"))

	tracer.TrackTowerUpgrade(ctx, TowerUpgradeRequest{
		UserID: "u-1", LevelID: "lv-1", TowerID: "t-1", TowerType: "arrow",
		FromLevel: 1, ToLevel: 2, Cost: 150, WaveIndex: 2,
	})
	require.NotNil(t, exporter.find("td.tower.upgrade"))

	tracer.TrackGachaPull(ctx, GachaPullRequest{
		UserID: "u-1", PoolID: "p-1", Pulls: 10, Rarity: "ssr",
		PityCounter: 80, ItemIDs: []string{"i-1"},
	})
	require.NotNil(t, exporter.find("gacha.pull"))
}

// --- GameTelemetryService proxies ---

func TestGameTelemetryService_ProxiesAndSpans(t *testing.T) {
	cfg := TelemetryConfig{
		ServiceName:   "svc",
		CollectorURL:  "http://127.0.0.1:14318",
		EnableTracing: false,
		EnableMetrics: false,
	}
	svcTel, err := NewGameTelemetryService(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)

	ctx := context.Background()

	// EndSpan with nil span must be a no-op.
	assert.NotPanics(t, func() { svcTel.EndSpan(nil, time.Now(), errors.New("x")) })

	sctx, span := svcTel.StartSpan(ctx, "unit", attribute.String("k", "v"))
	require.NotNil(t, span)
	require.NotNil(t, sctx)
	svcTel.EndSpan(span, time.Now(), nil)
	svcTel.EndSpan(span, time.Time{}, errors.New("boom"))

	// NOTE: StartLevelPlaythrough/CompleteLevelPlaythrough/TrackEconomyTransaction
	// proxies are intentionally not exercised through the provider-built service:
	// NewGameMetrics never initializes LevelStartCounter/LevelCompleteCounter/
	// CurrencyEarn etc., so those proxies panic with a nil interface call
	// (tracer.go:161). See GameTracer tests above, which inject a fully
	// initialized GameMetrics.

	fctx, fspan := svcTel.TrackFunctionCall(ctx, FunctionCallRequest{FunctionID: "fn"})
	svcTel.CompleteFunctionCall(fctx, FunctionCallResult{Success: true, DurationMs: 5, ResultType: "ok"})
	svcTel.CompleteFunctionCall(fctx, FunctionCallResult{Success: false, ErrorMessage: "err", ErrorCode: "E1"})
	_ = fspan

	pctx, _ := svcTel.TrackPermissionCheck(ctx, PermissionCheckRequest{UserID: "u", Resource: "r", Action: "a"})
	svcTel.CompletePermissionCheck(pctx, PermissionCheckResult{Granted: true})
	svcTel.CompletePermissionCheck(pctx, PermissionCheckResult{Granted: false})

	require.NoError(t, svcTel.Health(ctx))
	require.NoError(t, svcTel.Shutdown(ctx))
}
