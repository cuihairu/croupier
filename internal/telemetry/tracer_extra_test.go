package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- TracerProvider extra coverage ---

func TestTracerProvider_RegisterExporter(t *testing.T) {
	provider := NewTracerProvider()
	exporter := &testSpanExporter{}
	provider.RegisterExporter(exporter)

	assert.Len(t, provider.exporters, 1)
}

func TestTracerProvider_StartSpan_WithParent(t *testing.T) {
	provider := NewTracerProvider()
	ctx := context.Background()

	parent, ctx2 := provider.StartSpan(ctx, "parent")
	child, _ := provider.StartSpan(ctx2, "child",
		WithParent(parent),
	)

	assert.Equal(t, parent.TraceID, child.TraceID)
	assert.Equal(t, parent.SpanID, child.ParentID)
}

func TestTracerProvider_StartSpan_WithAttributes(t *testing.T) {
	provider := NewTracerProvider()
	ctx := context.Background()

	span, _ := provider.StartSpan(ctx, "test",
		WithAttributes(map[string]interface{}{
			"key1": "value1",
			"key2": 42,
		}),
	)

	assert.Equal(t, "value1", span.Attributes["key1"])
	assert.Equal(t, 42, span.Attributes["key2"])
}

func TestTracerProvider_StartSpan_WithStatus(t *testing.T) {
	provider := NewTracerProvider()
	ctx := context.Background()

	span, _ := provider.StartSpan(ctx, "test",
		WithStatus("error"),
	)

	assert.Equal(t, "error", span.Status)
}

func TestSpan_AddEvent(t *testing.T) {
	provider := NewTracerProvider()
	ctx := context.Background()

	span, _ := provider.StartSpan(ctx, "test")
	span.AddEvent("click", map[string]interface{}{
		"button": "submit",
	})

	assert.Len(t, span.Events, 1)
	assert.Equal(t, "click", span.Events[0].Name)
	assert.Equal(t, "submit", span.Events[0].Attributes["button"])
}

func TestSpan_SetAttribute(t *testing.T) {
	provider := NewTracerProvider()
	ctx := context.Background()

	span, _ := provider.StartSpan(ctx, "test")
	span.SetAttribute("key", "value")

	assert.Equal(t, "value", span.Attributes["key"])
}

func TestTracerProvider_EndSpan_WithExporter(t *testing.T) {
	provider := NewTracerProvider()
	exporter := &testSpanExporter{}
	provider.RegisterExporter(exporter)

	ctx := context.Background()
	span, _ := provider.StartSpan(ctx, "test")
	provider.EndSpan(span)

	assert.NotNil(t, span.EndTime)
	assert.True(t, span.Duration >= 0)
}

// --- NewJSONSpanExporter ---

func TestNewJSONSpanExporter(t *testing.T) {
	ch := make(chan<- []byte, 10)
	exporter := NewJSONSpanExporter(ch)
	assert.NotNil(t, exporter)
}

func TestJSONSpanExporter_ExportSpans(t *testing.T) {
	ch := make(chan []byte, 10)
	exporter := NewJSONSpanExporter(ch)
	spans := []*Span{
		{
			TraceID:   "trace-1",
			SpanID:    "span-1",
			Name:      "test",
			StartTime: time.Now(),
		},
	}

	err := exporter.ExportSpans(context.Background(), spans)
	assert.NoError(t, err)
}

func TestJSONSpanExporter_ExportSpans_Empty(t *testing.T) {
	ch := make(chan []byte, 10)
	exporter := NewJSONSpanExporter(ch)
	err := exporter.ExportSpans(context.Background(), nil)
	assert.NoError(t, err)
}

// --- AnalyticsBridge disabled ---

func TestNewAnalyticsBridge_Disabled(t *testing.T) {
	bridge := NewAnalyticsBridge(AnalyticsBridgeConfig{Enabled: false}, "game1", nil)
	assert.NotNil(t, bridge)
	assert.False(t, bridge.enabled)
}

func TestAnalyticsBridge_Health_Disabled(t *testing.T) {
	bridge := NewAnalyticsBridge(AnalyticsBridgeConfig{Enabled: false}, "game1", nil)
	err := bridge.Health(context.Background())
	assert.NoError(t, err)
}

func TestAnalyticsBridge_Shutdown_Disabled(t *testing.T) {
	bridge := NewAnalyticsBridge(AnalyticsBridgeConfig{Enabled: false}, "game1", nil)
	err := bridge.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestAnalyticsBridge_SendSessionEvent_Disabled(t *testing.T) {
	bridge := NewAnalyticsBridge(AnalyticsBridgeConfig{Enabled: false}, "game1", nil)
	// Should not panic
	bridge.SendSessionEvent(context.Background(), "event", nil, "user1", "sess1", "ios", "us", nil)
}

func TestAnalyticsBridge_SendProgressionEvent_Disabled(t *testing.T) {
	bridge := NewAnalyticsBridge(AnalyticsBridgeConfig{Enabled: false}, "game1", nil)
	// Should not panic
	bridge.SendProgressionEvent(context.Background(), "event", nil, "user1", "sess1", "level1", nil)
}

func TestAnalyticsBridge_SendEconomyEvent_Disabled(t *testing.T) {
	bridge := NewAnalyticsBridge(AnalyticsBridgeConfig{Enabled: false}, "game1", nil)
	// Should not panic
	bridge.SendEconomyEvent(context.Background(), "event", nil, "user1", "gold", 100.0, nil)
}

// --- testSpanExporter helper ---

type testSpanExporter struct {
	exported []*Span
}

func (e *testSpanExporter) ExportSpans(ctx context.Context, spans []*Span) error {
	e.exported = append(e.exported, spans...)
	return nil
}
