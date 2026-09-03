package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
)

func TestGameTracerEndUserSessionNormalV9(t *testing.T) {
	gt, exporter := newTestGameTracer(t)
	ctx, _ := gt.StartUserSession(context.Background(), SessionStartRequest{
		UserID: "u1", SessionID: "s1", Platform: "ios", Region: "us",
	})
	gt.EndUserSession(ctx, SessionEndRequest{
		UserID: "u1", SessionID: "s1", DurationMs: 1200, CauseOfEnd: "normal",
	})

	span := exporter.find("session.end")
	require.NotNil(t, span, "session.end span must be exported")
	assert.Equal(t, codes.Ok, statusOf(span).code)
}

func TestGameTracerCompleteLevelWithBridgeV9(t *testing.T) {
	gt, exporter := newTestGameTracer(t)
	gt.bridge = &AnalyticsBridge{enabled: false}
	ctx, _ := gt.StartLevelPlaythrough(context.Background(), LevelStartRequest{
		UserID: "u1", SessionID: "s1", LevelID: "L1", Difficulty: "normal",
	})
	gt.CompleteLevelPlaythrough(ctx, LevelCompleteRequest{
		LevelID: "L1", DurationMs: 900, Stars: 3, Retries: 1, Difficulty: "normal",
	})

	span := exporter.find("progression.complete")
	require.NotNil(t, span, "progression.complete span must be exported")
	assert.Equal(t, codes.Ok, statusOf(span).code)
}

func TestGameTracerEconomyWithBridgeV9(t *testing.T) {
	gt, exporter := newTestGameTracer(t)
	gt.bridge = &AnalyticsBridge{enabled: false}
	ctx := context.Background()

	gt.TrackEconomyTransaction(ctx, EconomyTransaction{
		UserID: "u1", Currency: "gold", CurrencyKind: "soft", Amount: 10,
		Type: "earn", Source: "kill_enemy", BalanceAfter: 110,
	})
	gt.TrackEconomyTransaction(ctx, EconomyTransaction{
		UserID: "u1", Currency: "gem", CurrencyKind: "hard", Amount: 3,
		Type: "spend", Sink: "tower_build", BalanceAfter: 7,
	})

	assert.NotNil(t, exporter.find("economy.earn"))
	assert.NotNil(t, exporter.find("economy.spend"))
}

func TestMetricsRegistryRegisterDuplicateV9(t *testing.T) {
	r := NewMetricsRegistry()
	require.NoError(t, r.Register(MetricDefinition{Name: "dup.metric", Type: MetricTypeCounter}))
	err := r.Register(MetricDefinition{Name: "dup.metric", Type: MetricTypeGauge})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestFormatPrometheusMetricMultiLabelV9(t *testing.T) {
	out := formatPrometheusMetric(
		Metric{Name: "m", Type: MetricTypeCounter, Value: 3, Labels: map[string]string{"a": "1", "b": "2"}},
		MetricDefinition{Name: "m", Type: MetricTypeCounter, Help: "h"},
	)
	assert.Contains(t, out, "m{")
	assert.Contains(t, out, ",")
	assert.Contains(t, out, "3")
}

func TestSpanSetAttributeNilMapV9(t *testing.T) {
	s := &Span{}
	s.SetAttribute("k", "v")
	assert.Equal(t, "v", s.Attributes["k"])
}

func TestJSONSpanExporterErrorsAndNilOutputV9(t *testing.T) {
	// Unmarshalable attribute → marshal error propagated.
	bad := NewJSONSpanExporter(nil)
	err := bad.ExportSpans(context.Background(), []*Span{{
		Name:       "bad",
		Attributes: map[string]interface{}{"ch": make(chan int)},
	}})
	require.Error(t, err)

	// nil output channel: valid spans export without sending.
	ok := NewJSONSpanExporter(nil)
	require.NoError(t, ok.ExportSpans(context.Background(), []*Span{{Name: "fine"}}))
}

func TestCalculateQuantileClampsV9(t *testing.T) {
	values := []float64{1, 2, 3}
	assert.Equal(t, float64(1), calculateQuantile(values, -0.5), "negative quantile clamps to index 0")
	assert.Equal(t, float64(3), calculateQuantile(values, 2), "quantile > 1 clamps to last index")
	assert.Equal(t, float64(0), calculateQuantile(nil, 0.5), "empty input returns 0")
}
