package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMetricsRegistry_Add tests Add method for counters
func TestMetricsRegistry_Add(t *testing.T) {
	registry := NewMetricsRegistry()
	registry.Register(MetricDefinition{
		Name:   "test_counter",
		Type:   "counter",
		Help:   "A test counter",
		Labels: []string{"method", "status"},
	})

	// Add value
	registry.Add("test_counter", 5.0, map[string]string{"method": "GET", "status": "200"})

	// Collect and verify
	metrics, err := registry.Collect(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, metrics)
}

// TestMetricsRegistry_Set tests Set method for gauges
func TestMetricsRegistry_Set(t *testing.T) {
	registry := NewMetricsRegistry()
	registry.Register(MetricDefinition{
		Name:   "test_gauge",
		Type:   "gauge",
		Help:   "A test gauge",
		Labels: []string{"host"},
	})

	// Set value
	registry.Set("test_gauge", 42.0, map[string]string{"host": "server1"})

	// Collect and verify
	metrics, err := registry.Collect(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, metrics)
}

// TestMetricsRegistry_GaugeInc tests GaugeInc method
func TestMetricsRegistry_GaugeInc(t *testing.T) {
	registry := NewMetricsRegistry()
	registry.Register(MetricDefinition{
		Name:   "active_connections",
		Type:   "gauge",
		Help:   "Active connections",
		Labels: []string{"server"},
	})

	// Set initial value
	registry.Set("active_connections", 10.0, map[string]string{"server": "s1"})

	// Increment
	registry.GaugeInc("active_connections", map[string]string{"server": "s1"})

	// Collect and verify
	metrics, err := registry.Collect(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, metrics)
}

// TestMetricsRegistry_GaugeDec tests GaugeDec method
func TestMetricsRegistry_GaugeDec(t *testing.T) {
	registry := NewMetricsRegistry()
	registry.Register(MetricDefinition{
		Name:   "queue_size",
		Type:   "gauge",
		Help:   "Queue size",
		Labels: []string{"queue"},
	})

	// Set initial value
	registry.Set("queue_size", 5.0, map[string]string{"queue": "jobs"})

	// Decrement
	registry.GaugeDec("queue_size", map[string]string{"queue": "jobs"})

	// Collect and verify
	metrics, err := registry.Collect(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, metrics)
}

// TestMetricsRegistry_Observe tests Observe method for histograms
func TestMetricsRegistry_Observe(t *testing.T) {
	registry := NewMetricsRegistry()
	registry.Register(MetricDefinition{
		Name:   "request_duration",
		Type:   "histogram",
		Help:   "Request duration",
		Labels: []string{"endpoint"},
	})

	// Observe values
	registry.Observe("request_duration", 0.1, map[string]string{"endpoint": "/api/users"})
	registry.Observe("request_duration", 0.2, map[string]string{"endpoint": "/api/users"})
	registry.Observe("request_duration", 0.15, map[string]string{"endpoint": "/api/users"})

	// Collect and verify
	metrics, err := registry.Collect(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, metrics)
}

// TestMetricsRegistry_Collect tests Collect method
func TestMetricsRegistry_Collect(t *testing.T) {
	registry := NewMetricsRegistry()

	// Register multiple metrics
	registry.Register(MetricDefinition{
		Name:   "counter1",
		Type:   "counter",
		Help:   "Counter 1",
		Labels: []string{},
	})
	registry.Register(MetricDefinition{
		Name:   "gauge1",
		Type:   "gauge",
		Help:   "Gauge 1",
		Labels: []string{},
	})

	// Inc counter
	registry.Inc("counter1", nil)

	// Set gauge
	registry.Set("gauge1", 100.0, nil)

	// Collect all metrics
	metrics, err := registry.Collect(context.Background())
	require.NoError(t, err)
	assert.Len(t, metrics, 2)
}

// TestMetricsRegistry_ExportPrometheus tests ExportPrometheus method
func TestMetricsRegistry_ExportPrometheus(t *testing.T) {
	registry := NewMetricsRegistry()
	registry.Register(MetricDefinition{
		Name:   "test_metric",
		Type:   "counter",
		Help:   "A test metric for Prometheus export",
		Labels: []string{"label1"},
	})

	registry.Inc("test_metric", map[string]string{"label1": "value1"})

	output, err := registry.ExportPrometheus()
	require.NoError(t, err)
	assert.Contains(t, output, "# HELP test_metric A test metric for Prometheus export")
	assert.Contains(t, output, "# TYPE test_metric counter")
	assert.Contains(t, output, "test_metric")
}

// TestNewCounter tests Counter constructor
func TestNewCounter(t *testing.T) {
	counter := NewCounter("test", []string{"method", "status"})
	assert.NotNil(t, counter)
	assert.Equal(t, "test", counter.name)
}

// TestCounter_Inc tests Counter Inc method
func TestCounter_Inc(t *testing.T) {
	counter := NewCounter("test", []string{"method"})
	labels := map[string]string{"method": "GET"}

	// Increment multiple times
	counter.Inc(labels)
	counter.Inc(labels)
	counter.Inc(labels)

	// Collect and check value
	metrics := counter.Collect(time.Now())
	require.Len(t, metrics, 1)
	assert.Equal(t, float64(3), metrics[0].Value)
}

// TestCounter_Add tests Counter Add method
func TestCounter_Add(t *testing.T) {
	counter := NewCounter("test", []string{"status"})
	labels := map[string]string{"status": "200"}

	// Add value
	counter.Add(labels, 5.5)

	// Collect and check value
	metrics := counter.Collect(time.Now())
	require.Len(t, metrics, 1)
	assert.Equal(t, 5.5, metrics[0].Value)
}

// TestCounter_Collect tests Counter Collect method
func TestCounter_Collect(t *testing.T) {
	counter := NewCounter("test_counter", []string{"method"})

	counter.Inc(map[string]string{"method": "GET"})
	counter.Inc(map[string]string{"method": "POST"})
	counter.Inc(map[string]string{"method": "GET"})

	metrics := counter.Collect(time.Now())
	assert.Len(t, metrics, 2)
}

// TestNewGauge tests Gauge constructor
func TestNewGauge(t *testing.T) {
	gauge := NewGauge("test", []string{"host"})
	assert.NotNil(t, gauge)
	assert.Equal(t, "test", gauge.name)
}

// TestGauge_Set tests Gauge Set method
func TestGauge_Set(t *testing.T) {
	gauge := NewGauge("temperature", []string{"sensor"})

	gauge.Set(map[string]string{"sensor": "s1"}, 23.5)
	gauge.Set(map[string]string{"sensor": "s2"}, 25.0)

	// Collect and check values
	metrics := gauge.Collect(time.Now())
	assert.Len(t, metrics, 2)

	// Find specific sensor values
	for _, m := range metrics {
		if m.Labels["sensor"] == "s1" {
			assert.Equal(t, 23.5, m.Value)
		}
		if m.Labels["sensor"] == "s2" {
			assert.Equal(t, 25.0, m.Value)
		}
	}
}

// TestGauge_Inc tests Gauge Inc method
func TestGauge_Inc(t *testing.T) {
	gauge := NewGauge("counter", []string{"id"})

	gauge.Set(map[string]string{"id": "1"}, 10.0)
	gauge.Inc(map[string]string{"id": "1"})

	// Collect and check
	metrics := gauge.Collect(time.Now())
	require.Len(t, metrics, 1)
	assert.Equal(t, 11.0, metrics[0].Value)
}

// TestGauge_Dec tests Gauge Dec method
func TestGauge_Dec(t *testing.T) {
	gauge := NewGauge("counter", []string{"id"})

	gauge.Set(map[string]string{"id": "1"}, 10.0)
	gauge.Dec(map[string]string{"id": "1"})

	// Collect and check
	metrics := gauge.Collect(time.Now())
	require.Len(t, metrics, 1)
	assert.Equal(t, 9.0, metrics[0].Value)
}

// TestGauge_Collect tests Gauge Collect method
func TestGauge_Collect(t *testing.T) {
	gauge := NewGauge("test_gauge", []string{"host"})

	gauge.Set(map[string]string{"host": "h1"}, 100.0)
	gauge.Set(map[string]string{"host": "h2"}, 200.0)

	metrics := gauge.Collect(time.Now())
	assert.Len(t, metrics, 2)
}

// TestNewHistogram tests Histogram constructor
func TestNewHistogram(t *testing.T) {
	histogram := NewHistogram("test", []string{"endpoint"}, []float64{0.1, 0.5, 1.0})
	assert.NotNil(t, histogram)
	assert.Equal(t, "test", histogram.name)
}

// TestHistogram_Observe tests Histogram Observe method
func TestHistogram_Observe(t *testing.T) {
	histogram := NewHistogram("latency", []string{"api"}, []float64{0.1, 0.5, 1.0})

	histogram.Observe(map[string]string{"api": "users"}, 0.05)
	histogram.Observe(map[string]string{"api": "users"}, 0.3)
	histogram.Observe(map[string]string{"api": "users"}, 0.8)

	// Collect and verify
	metrics := histogram.Collect(time.Now())
	assert.NotEmpty(t, metrics)
}

// TestHistogram_Collect tests Histogram Collect method
func TestHistogram_Collect(t *testing.T) {
	histogram := NewHistogram("test_histogram", []string{"status"}, []float64{1.0, 5.0, 10.0})

	histogram.Observe(map[string]string{"status": "ok"}, 2.0)
	histogram.Observe(map[string]string{"status": "ok"}, 7.0)
	histogram.Observe(map[string]string{"status": "error"}, 0.5)

	metrics := histogram.Collect(time.Now())
	assert.NotEmpty(t, metrics)
}

// TestNewSummary tests Summary constructor
func TestNewSummary(t *testing.T) {
	objectives := map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001}
	summary := NewSummary("test", []string{"operation"}, objectives)
	assert.NotNil(t, summary)
	assert.Equal(t, "test", summary.name)
}

// TestSummary_Observe tests Summary Observe method
func TestSummary_Observe(t *testing.T) {
	objectives := map[float64]float64{0.5: 0.05, 0.9: 0.01}
	summary := NewSummary("response_size", []string{"endpoint"}, objectives)

	for i := 1; i <= 100; i++ {
		summary.Observe(map[string]string{"endpoint": "/api"}, float64(i*10))
	}

	// Collect and verify
	metrics := summary.Collect(time.Now())
	assert.NotEmpty(t, metrics)
}

// TestSummary_Collect tests Summary Collect method
func TestSummary_Collect(t *testing.T) {
	objectives := map[float64]float64{0.5: 0.05, 0.9: 0.01}
	summary := NewSummary("test_summary", []string{"op"}, objectives)

	summary.Observe(map[string]string{"op": "read"}, 10.0)
	summary.Observe(map[string]string{"op": "read"}, 20.0)
	summary.Observe(map[string]string{"op": "read"}, 30.0)

	metrics := summary.Collect(time.Now())
	assert.NotEmpty(t, metrics)
}

// TestNewTracerProvider tests TracerProvider constructor
func TestNewTracerProvider(t *testing.T) {
	provider := NewTracerProvider()
	assert.NotNil(t, provider)
}

// TestTracerProvider_StartSpan tests StartSpan
func TestTracerProvider_StartSpan(t *testing.T) {
	provider := NewTracerProvider()
	ctx := context.Background()

	span, ctx2 := provider.StartSpan(ctx, "test-span")
	assert.NotNil(t, span)
	assert.Equal(t, "test-span", span.Name)
	assert.NotEqual(t, ctx, ctx2) // Context should contain span
}

// TestTracerProvider_EndSpan tests EndSpan
func TestTracerProvider_EndSpan(t *testing.T) {
	provider := NewTracerProvider()
	ctx := context.Background()

	span, _ := provider.StartSpan(ctx, "span1")
	// EndSpan should not panic
	provider.EndSpan(span)

	assert.NotNil(t, span.EndTime)
}
