package telemetry

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestNewMetricsRegistry 测试创建 MetricsRegistry
func TestNewMetricsRegistry(t *testing.T) {
	registry := NewMetricsRegistry()
	if registry == nil {
		t.Fatal("NewMetricsRegistry() should return non-nil registry")
	}
	if registry.counters == nil {
		t.Error("counters map should be initialized")
	}
	if registry.gauges == nil {
		t.Error("gauges map should be initialized")
	}
	if registry.histograms == nil {
		t.Error("histograms map should be initialized")
	}
	if registry.summaries == nil {
		t.Error("summaries map should be initialized")
	}
	if registry.definitions == nil {
		t.Error("definitions map should be initialized")
	}
}

// TestMetricsRegistry_Register 测试注册指标
func TestMetricsRegistry_Register(t *testing.T) {
	registry := NewMetricsRegistry()

	tests := []struct {
		name    string
		def     MetricDefinition
		wantErr bool
	}{
		{
			name: "注册 Counter",
			def: MetricDefinition{
				Name: "test_counter",
				Type: MetricTypeCounter,
				Help: "A test counter",
			},
			wantErr: false,
		},
		{
			name: "注册 Gauge",
			def: MetricDefinition{
				Name: "test_gauge",
				Type: MetricTypeGauge,
				Help: "A test gauge",
			},
			wantErr: false,
		},
		{
			name: "注册 Histogram",
			def: MetricDefinition{
				Name:    "test_histogram",
				Type:    MetricTypeHistogram,
				Help:    "A test histogram",
				Buckets: []float64{1, 5, 10},
			},
			wantErr: false,
		},
		{
			name: "注册 Summary",
			def: MetricDefinition{
				Name:       "test_summary",
				Type:       MetricTypeSummary,
				Help:       "A test summary",
				Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01},
			},
			wantErr: false,
		},
		{
			name: "重复注册",
			def: MetricDefinition{
				Name: "test_counter",
				Type: MetricTypeCounter,
				Help: "Duplicate counter",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.Register(tt.def)
			if (err != nil) != tt.wantErr {
				t.Errorf("Register() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestMetricsRegistry_CounterOperations 测试 Counter 操作
func TestMetricsRegistry_CounterOperations(t *testing.T) {
	registry := NewMetricsRegistry()
	err := registry.Register(MetricDefinition{
		Name: "test_counter",
		Type: MetricTypeCounter,
		Help: "Test counter",
	})
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	labels := map[string]string{"method": "get", "status": "200"}

	// 测试 Inc
	registry.Inc("test_counter", labels)
	registry.Inc("test_counter", labels)

	// 测试 Add
	registry.Add("test_counter", 5.0, labels)

	// 验证值
	metrics, _ := registry.Collect(context.Background())
	if len(metrics) == 0 {
		t.Fatal("Collect() should return metrics")
	}

	found := false
	for _, m := range metrics {
		if m.Name == "test_counter" {
			if m.Value != 7.0 {
				t.Errorf("Counter value = %f, want 7.0", m.Value)
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("Counter metric not found")
	}
}

// TestMetricsRegistry_GaugeOperations 测试 Gauge 操作
func TestMetricsRegistry_GaugeOperations(t *testing.T) {
	registry := NewMetricsRegistry()
	err := registry.Register(MetricDefinition{
		Name: "test_gauge",
		Type: MetricTypeGauge,
		Help: "Test gauge",
	})
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	labels := map[string]string{"service": "api"}

	// 测试 Set
	registry.Set("test_gauge", 10.5, labels)

	// 测试 Inc
	registry.GaugeInc("test_gauge", labels)

	// 测试 Dec
	registry.GaugeDec("test_gauge", labels)
	registry.GaugeDec("test_gauge", labels)

	// 验证值 (10.5 + 1 - 1 - 1 = 9.5)
	metrics, _ := registry.Collect(context.Background())
	found := false
	for _, m := range metrics {
		if m.Name == "test_gauge" {
			if m.Value != 9.5 {
				t.Errorf("Gauge value = %f, want 9.5", m.Value)
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("Gauge metric not found")
	}
}

// TestMetricsRegistry_HistogramOperations 测试 Histogram 操作
func TestMetricsRegistry_HistogramOperations(t *testing.T) {
	registry := NewMetricsRegistry()
	err := registry.Register(MetricDefinition{
		Name:    "test_histogram",
		Type:    MetricTypeHistogram,
		Help:    "Test histogram",
		Buckets: []float64{1, 5, 10},
	})
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	labels := map[string]string{"endpoint": "/api"}

	// 观察一些值
	registry.Observe("test_histogram", 0.5, labels)  // <= 1
	registry.Observe("test_histogram", 3, labels)    // <= 5
	registry.Observe("test_histogram", 7, labels)    // <= 10
	registry.Observe("test_histogram", 15, labels)   // > 10 (counted in +Inf)

	// 验证指标
	metrics, _ := registry.Collect(context.Background())
	if len(metrics) == 0 {
		t.Fatal("Collect() should return metrics")
	}

	// 应该有 sum, count, 和 buckets
	var sumFound, countFound bool
	for _, m := range metrics {
		if strings.HasPrefix(m.Name, "test_histogram") {
			if m.Name == "test_histogram_sum" {
				if m.Value != 25.5 {
					t.Errorf("Histogram sum = %f, want 25.5", m.Value)
				}
				sumFound = true
			}
			if m.Name == "test_histogram_count" {
				if m.Value != 4 {
					t.Errorf("Histogram count = %f, want 4", m.Value)
				}
				countFound = true
			}
		}
	}
	if !sumFound || !countFound {
		t.Error("Histogram sum/count not found")
	}
}

// TestMetricsRegistry_SummaryOperations 测试 Summary 操作
func TestMetricsRegistry_SummaryOperations(t *testing.T) {
	registry := NewMetricsRegistry()
	err := registry.Register(MetricDefinition{
		Name:       "test_summary",
		Type:       MetricTypeSummary,
		Help:       "Test summary",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	})
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	labels := map[string]string{"operation": "query"}

	// 直接使用 summary 对象
	s := registry.summaries["test_summary"]
	values := []float64{1, 2, 3, 4, 5, 10, 20, 50, 100}
	for _, v := range values {
		s.Observe(labels, v)
	}

	// 验证指标
	metrics, _ := registry.Collect(context.Background())
	if len(metrics) == 0 {
		t.Fatal("Collect() should return metrics")
	}

	// 验证基本指标存在
	var countFound bool
	for _, m := range metrics {
		if m.Name == "test_summary_count" {
			if m.Value != 9 {
				t.Errorf("Summary count = %f, want 9", m.Value)
			}
			countFound = true
		}
	}
	if !countFound {
		t.Error("Summary count not found")
	}
}

// TestMetricsRegistry_Collect 测试收集所有指标
func TestMetricsRegistry_Collect(t *testing.T) {
	registry := NewMetricsRegistry()

	// 注册多种类型的指标
	registry.Register(MetricDefinition{Name: "c1", Type: MetricTypeCounter, Help: "C1"})
	registry.Register(MetricDefinition{Name: "g1", Type: MetricTypeGauge, Help: "G1"})
	registry.Register(MetricDefinition{Name: "h1", Type: MetricTypeHistogram, Help: "H1"})
	registry.Register(MetricDefinition{Name: "s1", Type: MetricTypeSummary, Help: "S1"})

	registry.Inc("c1", nil)
	registry.Set("g1", 42, nil)
	registry.Observe("h1", 5, nil)
	registry.Observe("s1", 10, nil)

	metrics, err := registry.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if len(metrics) == 0 {
		t.Error("Collect() should return metrics")
	}
}

// TestMetricsRegistry_ExportPrometheus 测试 Prometheus 导出
func TestMetricsRegistry_ExportPrometheus(t *testing.T) {
	registry := NewMetricsRegistry()
	err := registry.Register(MetricDefinition{
		Name: "test_metric",
		Type: MetricTypeCounter,
		Help: "A test metric for Prometheus",
	})
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	registry.Inc("test_metric", map[string]string{"label": "value"})

	output, err := registry.ExportPrometheus()
	if err != nil {
		t.Fatalf("ExportPrometheus() error = %v", err)
	}

	if !strings.Contains(output, "# HELP test_metric") {
		t.Error("Output should contain HELP line")
	}
	if !strings.Contains(output, "# TYPE test_metric counter") {
		t.Error("Output should contain TYPE line")
	}
	if !strings.Contains(output, "test_metric") {
		t.Error("Output should contain metric name")
	}
}

// TestNewCounter 测试创建 Counter
func TestNewCounter(t *testing.T) {
	counter := NewCounter("test", []string{"label1", "label2"})
	if counter == nil {
		t.Fatal("NewCounter() should return non-nil counter")
	}
	if counter.name != "test" {
		t.Errorf("name = %s, want 'test'", counter.name)
	}
	if len(counter.labels) != 2 {
		t.Errorf("labels length = %d, want 2", len(counter.labels))
	}
}

// TestCounter_Inc 测试 Counter Inc
func TestCounter_Inc(t *testing.T) {
	counter := NewCounter("test", []string{"method"})
	labels := map[string]string{"method": "get"}

	counter.Inc(labels)
	counter.Inc(labels)

	metrics := counter.Collect(time.Now())
	if len(metrics) != 1 {
		t.Fatalf("Collect() returned %d metrics, want 1", len(metrics))
	}
	if metrics[0].Value != 2 {
		t.Errorf("Counter value = %f, want 2", metrics[0].Value)
	}
}

// TestCounter_Add 测试 Counter Add
func TestCounter_Add(t *testing.T) {
	counter := NewCounter("test", []string{"method"})
	labels := map[string]string{"method": "post"}

	counter.Add(labels, 5.5)
	counter.Add(labels, 2.5)

	metrics := counter.Collect(time.Now())
	if metrics[0].Value != 8 {
		t.Errorf("Counter value = %f, want 8", metrics[0].Value)
	}
}

// TestNewGauge 测试创建 Gauge
func TestNewGauge(t *testing.T) {
	gauge := NewGauge("test", []string{"service"})
	if gauge == nil {
		t.Fatal("NewGauge() should return non-nil gauge")
	}
	if gauge.name != "test" {
		t.Errorf("name = %s, want 'test'", gauge.name)
	}
}

// TestGauge_Set 测试 Gauge Set
func TestGauge_Set(t *testing.T) {
	gauge := NewGauge("test", []string{"host"})
	labels := map[string]string{"host": "server1"}

	gauge.Set(labels, 10.0)
	gauge.Set(labels, 20.0) // Should overwrite

	metrics := gauge.Collect(time.Now())
	if metrics[0].Value != 20 {
		t.Errorf("Gauge value = %f, want 20", metrics[0].Value)
	}
}

// TestGauge_IncDec 测试 Gauge Inc/Dec
func TestGauge_IncDec(t *testing.T) {
	gauge := NewGauge("test", []string{"host"})
	labels := map[string]string{"host": "server1"}

	gauge.Set(labels, 10)
	gauge.Inc(labels)  // 11
	gauge.Inc(labels)  // 12
	gauge.Dec(labels)  // 11
	gauge.Dec(labels)  // 10

	metrics := gauge.Collect(time.Now())
	if metrics[0].Value != 10 {
		t.Errorf("Gauge value = %f, want 10", metrics[0].Value)
	}
}

// TestNewHistogram 测试创建 Histogram
func TestNewHistogram(t *testing.T) {
	h := NewHistogram("test", []string{"endpoint"}, []float64{1, 5, 10})
	if h == nil {
		t.Fatal("NewHistogram() should return non-nil histogram")
	}
	if h.name != "test" {
		t.Errorf("name = %s, want 'test'", h.name)
	}
	if len(h.buckets) != 3 {
		t.Errorf("buckets length = %d, want 3", len(h.buckets))
	}
}

// TestHistogram_DefaultBuckets 测试默认 buckets
func TestHistogram_DefaultBuckets(t *testing.T) {
	h := NewHistogram("test", nil, nil)
	if len(h.buckets) != 11 {
		t.Errorf("default buckets length = %d, want 11", len(h.buckets))
	}
}

// TestHistogram_Observe 测试 Histogram Observe
func TestHistogram_Observe(t *testing.T) {
	h := NewHistogram("test", []string{"endpoint"}, []float64{1, 5, 10})
	labels := map[string]string{"endpoint": "/api"}

	h.Observe(labels, 0.5)  // <= 1, <= 5, <= 10
	h.Observe(labels, 3)    // <= 5, <= 10
	h.Observe(labels, 7)    // <= 10
	h.Observe(labels, 15)   // > 10

	metrics := h.Collect(time.Now())

	// 验证至少产生了一些指标
	if len(metrics) == 0 {
		t.Error("Histogram should produce bucket metrics")
	}

	// 验证基本的 sum 和 count
	var sumFound, countFound bool
	for _, m := range metrics {
		if m.Name == "test_sum" {
			sumFound = true
		}
		if m.Name == "test_count" {
			countFound = true
		}
	}
	if !sumFound {
		t.Error("Histogram sum not found")
	}
	if !countFound {
		t.Error("Histogram count not found")
	}
}

// TestNewSummary 测试创建 Summary
func TestNewSummary(t *testing.T) {
	s := NewSummary("test", []string{"operation"}, map[float64]float64{0.95: 0.01})
	if s == nil {
		t.Fatal("NewSummary() should return non-nil summary")
	}
	if s.name != "test" {
		t.Errorf("name = %s, want 'test'", s.name)
	}
}

// TestSummary_DefaultObjectives 测试默认 objectives
func TestSummary_DefaultObjectives(t *testing.T) {
	s := NewSummary("test", nil, nil)
	if len(s.objectives) != 4 {
		t.Errorf("default objectives length = %d, want 4", len(s.objectives))
	}
}

// TestSummary_Observe 测试 Summary Observe
func TestSummary_Observe(t *testing.T) {
	s := NewSummary("test", []string{"op"}, nil)
	labels := map[string]string{"op": "query"}

	values := []float64{10, 20, 30, 40, 50}
	for _, v := range values {
		s.Observe(labels, v)
	}

	metrics := s.Collect(time.Now())
	if len(metrics) == 0 {
		t.Fatal("Collect() should return metrics")
	}

	var countFound bool
	for _, m := range metrics {
		if m.Name == "test_count" {
			if m.Value != 5 {
				t.Errorf("Count = %f, want 5", m.Value)
			}
			countFound = true
		}
	}
	if !countFound {
		t.Error("Count metric not found")
	}
}

// TestNewTracerProvider 测试创建 TracerProvider
func TestNewTracerProvider(t *testing.T) {
	p := NewTracerProvider()
	if p == nil {
		t.Fatal("NewTracerProvider() should return non-nil provider")
	}
	if p.spans == nil {
		t.Error("spans map should be initialized")
	}
	if len(p.exporters) != 0 {
		t.Error("exporters should be empty initially")
	}
}

// TestTracerProvider_RegisterExporter 测试注册导出器
func TestTracerProvider_RegisterExporter(t *testing.T) {
	p := NewTracerProvider()

	exporter := &mockSpanExporter{}
	p.RegisterExporter(exporter)

	if len(p.exporters) != 1 {
		t.Errorf("exporters length = %d, want 1", len(p.exporters))
	}
}

// TestTracerProvider_StartSpan 测试创建 Span
func TestTracerProvider_StartSpan(t *testing.T) {
	p := NewTracerProvider()

	span, ctx := p.StartSpan(context.Background(), "test-operation")
	if span == nil {
		t.Fatal("StartSpan() should return non-nil span")
	}
	if ctx == nil {
		t.Fatal("StartSpan() should return non-nil context")
	}
	if span.Name != "test-operation" {
		t.Errorf("span name = %s, want 'test-operation'", span.Name)
	}
	if span.TraceID == "" {
		t.Error("TraceID should be set")
	}
	if span.SpanID == "" {
		t.Error("SpanID should be set")
	}
}

// TestTracerProvider_SpanParent 测试父子关系
func TestTracerProvider_SpanParent(t *testing.T) {
	p := NewTracerProvider()

	parent, ctx := p.StartSpan(context.Background(), "parent")
	if parent.ParentID != "" {
		t.Error("Parent span should have no parent ID")
	}
	parentTraceID := parent.TraceID

	child, _ := p.StartSpan(ctx, "child")
	if child.TraceID != parentTraceID {
		t.Error("Child should have same trace ID as parent")
	}
	if child.ParentID != parent.SpanID {
		t.Error("Child parent ID should match parent span ID")
	}
}

// TestTracerProvider_EndSpan 测试结束 Span
func TestTracerProvider_EndSpan(t *testing.T) {
	p := NewTracerProvider()

	span, _ := p.StartSpan(context.Background(), "test")
	if span.EndTime != nil {
		t.Error("EndTime should be nil before End()")
	}

	p.EndSpan(span)

	if span.EndTime == nil {
		t.Error("EndTime should be set after End()")
	}
	if span.Duration == 0 {
		t.Error("Duration should be set after End()")
	}
}

// TestSpan_AddEvent 测试添加事件
func TestSpan_AddEvent(t *testing.T) {
	p := NewTracerProvider()
	span, _ := p.StartSpan(context.Background(), "test")

	span.AddEvent("error", map[string]interface{}{
		"code": 500,
		"message": "internal error",
	})

	if len(span.Events) != 1 {
		t.Fatalf("Events length = %d, want 1", len(span.Events))
	}
	if span.Events[0].Name != "error" {
		t.Errorf("Event name = %s, want 'error'", span.Events[0].Name)
	}
}

// TestSpan_SetAttribute 测试设置属性
func TestSpan_SetAttribute(t *testing.T) {
	p := NewTracerProvider()
	span, _ := p.StartSpan(context.Background(), "test")

	span.SetAttribute("user.id", "12345")
	span.SetAttribute("http.method", "GET")

	if span.Attributes == nil {
		t.Fatal("Attributes should be initialized")
	}
	if len(span.Attributes) != 2 {
		t.Errorf("Attributes length = %d, want 2", len(span.Attributes))
	}
}

// TestWithAttributes 测试 WithAttributes 选项
func TestWithAttributes(t *testing.T) {
	p := NewTracerProvider()
	span, _ := p.StartSpan(context.Background(), "test",
		WithAttributes(map[string]interface{}{
			"key1": "value1",
			"key2": 42,
		}))

	if len(span.Attributes) != 2 {
		t.Errorf("Attributes length = %d, want 2", len(span.Attributes))
	}
}

// TestWithParent 测试 WithParent 选项
func TestWithParent(t *testing.T) {
	p := NewTracerProvider()
	parent, _ := p.StartSpan(context.Background(), "parent")
	child, _ := p.StartSpan(context.Background(), "child", WithParent(parent))

	if child.TraceID != parent.TraceID {
		t.Error("Child trace ID should match parent")
	}
	if child.ParentID != parent.SpanID {
		t.Error("Child parent ID should match parent span ID")
	}
}

// TestWithStatus 测试 WithStatus 选项
func TestWithStatus(t *testing.T) {
	p := NewTracerProvider()
	span, _ := p.StartSpan(context.Background(), "test", WithStatus("error"))

	if span.Status != "error" {
		t.Errorf("Status = %s, want 'error'", span.Status)
	}
}

// TestJSONSpanExporter 测试 JSON 导出器
func TestJSONSpanExporter(t *testing.T) {
	output := make(chan []byte, 10)
	exporter := NewJSONSpanExporter(output)

	p := NewTracerProvider()
	p.RegisterExporter(exporter)

	span, _ := p.StartSpan(context.Background(), "test")
	span.SetAttribute("test", "value")
	p.EndSpan(span)

	// 等待导出
	select {
	case data := <-output:
		if len(data) == 0 {
			t.Error("Exported data should not be empty")
		}
		// 验证是有效的 JSON
		if string(data[0]) != "{" {
			t.Error("Data should be JSON object")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for export")
	}
}

// TestLabelsKey 测试 labelsKey 函数
func TestLabelsKey(t *testing.T) {
	labels := map[string]string{"method": "get", "status": "200"}
	order := []string{"method", "status"}

	key := labelsKey(labels, order)
	if key != "method=get,status=200," {
		t.Errorf("labelsKey() = %s, want 'method=get,status=200,'", key)
	}
}

// TestParseLabelsKey 测试 parseLabelsKey 函数
func TestParseLabelsKey(t *testing.T) {
	key := "method=get,status=200,"
	labels := parseLabelsKey(key)

	if len(labels) != 2 {
		t.Fatalf("parseLabelsKey() returned %d labels, want 2", len(labels))
	}
	if labels["method"] != "get" {
		t.Errorf("method = %s, want 'get'", labels["method"])
	}
	if labels["status"] != "200" {
		t.Errorf("status = %s, want '200'", labels["status"])
	}
}

// TestSplitString 测试 splitString 函数
func TestSplitString(t *testing.T) {
	tests := []struct {
		input    string
		sep      string
		expected []string
	}{
		{"a,b,c", ",", []string{"a", "b", "c"}},
		{"key=value", "=", []string{"key", "value"}},
		{"single", ",", []string{"single"}},
		{"", ",", []string{""}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := splitString(tt.input, tt.sep)
			if len(result) != len(tt.expected) {
				t.Errorf("splitString() length = %d, want %d", len(result), len(tt.expected))
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("splitString()[%d] = %s, want %s", i, v, tt.expected[i])
				}
			}
		})
	}
}

// TestCalculateQuantile 测试 calculateQuantile 函数
func TestCalculateQuantile(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	tests := []struct {
		quantile float64
		expected int
	}{
		{0.0, 1},   // Minimum
		{0.5, 6},   // Median-ish
		{0.9, 10},  // 90th percentile
		{1.0, 10},  // Maximum
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%.1f", tt.quantile), func(t *testing.T) {
			result := calculateQuantile(values, tt.quantile)
			if int(result) != tt.expected {
				t.Errorf("calculateQuantile(%v) = %f, want %d", tt.quantile, result, tt.expected)
			}
		})
	}
}

// TestCalculateQuantile_Empty 测试空数组
func TestCalculateQuantile_Empty(t *testing.T) {
	result := calculateQuantile([]float64{}, 0.5)
	if result != 0 {
		t.Errorf("calculateQuantile(empty) = %f, want 0", result)
	}
}

// Mock span exporter for testing
type mockSpanExporter struct{}

func (m *mockSpanExporter) ExportSpans(ctx context.Context, spans []*Span) error {
	return nil
}
