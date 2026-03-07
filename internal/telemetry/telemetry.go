package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Metrics types
type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeHistogram MetricType = "histogram"
	MetricTypeSummary   MetricType = "summary"
)

// Metric represents a single metric data point
type Metric struct {
	Name      string            `json:"name"`
	Type      MetricType        `json:"type"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// MetricDefinition defines a metric
type MetricDefinition struct {
	Name       string              `json:"name"`
	Type       MetricType          `json:"type"`
	Help       string              `json:"help"`
	Labels     []string            `json:"labels,omitempty"`
	Buckets    []float64           `json:"buckets,omitempty"`    // For histograms
	Objectives map[float64]float64 `json:"objectives,omitempty"` // For summaries
}

// MetricCollector interface for collecting metrics
type MetricCollector interface {
	Collect(ctx context.Context) ([]Metric, error)
	Definitions() []MetricDefinition
}

// MetricsRegistry holds all registered metrics
type MetricsRegistry struct {
	mu          sync.RWMutex
	counters    map[string]*Counter
	gauges      map[string]*Gauge
	histograms  map[string]*Histogram
	summaries   map[string]*Summary
	definitions map[string]MetricDefinition
}

// NewMetricsRegistry creates a new metrics registry
func NewMetricsRegistry() *MetricsRegistry {
	return &MetricsRegistry{
		counters:    make(map[string]*Counter),
		gauges:      make(map[string]*Gauge),
		histograms:  make(map[string]*Histogram),
		summaries:   make(map[string]*Summary),
		definitions: make(map[string]MetricDefinition),
	}
}

// Register registers a new metric
func (r *MetricsRegistry) Register(def MetricDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.definitions[def.Name]; exists {
		return fmt.Errorf("metric %s already registered", def.Name)
	}

	r.definitions[def.Name] = def

	switch def.Type {
	case MetricTypeCounter:
		r.counters[def.Name] = NewCounter(def.Name, def.Labels)
	case MetricTypeGauge:
		r.gauges[def.Name] = NewGauge(def.Name, def.Labels)
	case MetricTypeHistogram:
		r.histograms[def.Name] = NewHistogram(def.Name, def.Labels, def.Buckets)
	case MetricTypeSummary:
		r.summaries[def.Name] = NewSummary(def.Name, def.Labels, def.Objectives)
	}

	return nil
}

// Counter operations
func (r *MetricsRegistry) Inc(name string, labels map[string]string) {
	r.mu.RLock()
	counter, exists := r.counters[name]
	r.mu.RUnlock()

	if exists {
		counter.Inc(labels)
	}
}

func (r *MetricsRegistry) Add(name string, value float64, labels map[string]string) {
	r.mu.RLock()
	counter, exists := r.counters[name]
	r.mu.RUnlock()

	if exists {
		counter.Add(labels, value)
	}
}

// Gauge operations
func (r *MetricsRegistry) Set(name string, value float64, labels map[string]string) {
	r.mu.RLock()
	gauge, exists := r.gauges[name]
	r.mu.RUnlock()

	if exists {
		gauge.Set(labels, value)
	}
}

func (r *MetricsRegistry) GaugeInc(name string, labels map[string]string) {
	r.mu.RLock()
	gauge, exists := r.gauges[name]
	r.mu.RUnlock()

	if exists {
		gauge.Inc(labels)
	}
}

func (r *MetricsRegistry) GaugeDec(name string, labels map[string]string) {
	r.mu.RLock()
	gauge, exists := r.gauges[name]
	r.mu.RUnlock()

	if exists {
		gauge.Dec(labels)
	}
}

// Histogram operations
func (r *MetricsRegistry) Observe(name string, value float64, labels map[string]string) {
	r.mu.RLock()
	histogram, exists := r.histograms[name]
	r.mu.RUnlock()

	if exists {
		histogram.Observe(labels, value)
	}
}

// Collect collects all metrics
func (r *MetricsRegistry) Collect(ctx context.Context) ([]Metric, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var metrics []Metric
	now := time.Now()

	// Collect counters
	for _, counter := range r.counters {
		for _, m := range counter.Collect(now) {
			metrics = append(metrics, m)
		}
	}

	// Collect gauges
	for _, gauge := range r.gauges {
		for _, m := range gauge.Collect(now) {
			metrics = append(metrics, m)
		}
	}

	// Collect histograms
	for _, histogram := range r.histograms {
		for _, m := range histogram.Collect(now) {
			metrics = append(metrics, m)
		}
	}

	// Collect summaries
	for _, summary := range r.summaries {
		for _, m := range summary.Collect(now) {
			metrics = append(metrics, m)
		}
	}

	return metrics, nil
}

// ExportPrometheus exports metrics in Prometheus format
func (r *MetricsRegistry) ExportPrometheus() (string, error) {
	metrics, err := r.Collect(context.Background())
	if err != nil {
		return "", err
	}

	var output string
	for _, m := range metrics {
		output += formatPrometheusMetric(m, r.definitions[m.Name])
	}

	return output, nil
}

func formatPrometheusMetric(m Metric, def MetricDefinition) string {
	var output string

	// Add help and type if not already added
	output += fmt.Sprintf("# HELP %s %s\n", m.Name, def.Help)
	output += fmt.Sprintf("# TYPE %s %s\n", m.Name, m.Type)

	// Format labels
	labelStr := ""
	if len(m.Labels) > 0 {
		first := true
		for k, v := range m.Labels {
			if !first {
				labelStr += ","
			}
			labelStr += fmt.Sprintf(`%s="%s"`, k, v)
			first = false
		}
		labelStr = "{" + labelStr + "}"
	}

	output += fmt.Sprintf("%s%s %g\n", m.Name, labelStr, m.Value)
	return output
}

// Counter metric type
type Counter struct {
	name   string
	labels []string
	mu     sync.RWMutex
	values map[string]float64
}

func NewCounter(name string, labels []string) *Counter {
	return &Counter{
		name:   name,
		labels: labels,
		values: make(map[string]float64),
	}
}

func (c *Counter) Inc(labels map[string]string) {
	c.Add(labels, 1)
}

func (c *Counter) Add(labels map[string]string, value float64) {
	key := labelsKey(labels, c.labels)
	c.mu.Lock()
	c.values[key] += value
	c.mu.Unlock()
}

func (c *Counter) Collect(timestamp time.Time) []Metric {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var metrics []Metric
	for key, value := range c.values {
		metrics = append(metrics, Metric{
			Name:      c.name,
			Type:      MetricTypeCounter,
			Value:     value,
			Labels:    parseLabelsKey(key),
			Timestamp: timestamp,
		})
	}
	return metrics
}

// Gauge metric type
type Gauge struct {
	name   string
	labels []string
	mu     sync.RWMutex
	values map[string]float64
}

func NewGauge(name string, labels []string) *Gauge {
	return &Gauge{
		name:   name,
		labels: labels,
		values: make(map[string]float64),
	}
}

func (g *Gauge) Set(labels map[string]string, value float64) {
	key := labelsKey(labels, g.labels)
	g.mu.Lock()
	g.values[key] = value
	g.mu.Unlock()
}

func (g *Gauge) Inc(labels map[string]string) {
	key := labelsKey(labels, g.labels)
	g.mu.Lock()
	g.values[key]++
	g.mu.Unlock()
}

func (g *Gauge) Dec(labels map[string]string) {
	key := labelsKey(labels, g.labels)
	g.mu.Lock()
	g.values[key]--
	g.mu.Unlock()
}

func (g *Gauge) Collect(timestamp time.Time) []Metric {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var metrics []Metric
	for key, value := range g.values {
		metrics = append(metrics, Metric{
			Name:      g.name,
			Type:      MetricTypeGauge,
			Value:     value,
			Labels:    parseLabelsKey(key),
			Timestamp: timestamp,
		})
	}
	return metrics
}

// Histogram metric type
type Histogram struct {
	name    string
	labels  []string
	buckets []float64
	mu      sync.RWMutex
	values  map[string]*histogramValue
}

type histogramValue struct {
	count   uint64
	sum     float64
	buckets map[float64]uint64
}

func NewHistogram(name string, labels []string, buckets []float64) *Histogram {
	if len(buckets) == 0 {
		buckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
	}
	return &Histogram{
		name:    name,
		labels:  labels,
		buckets: buckets,
		values:  make(map[string]*histogramValue),
	}
}

func (h *Histogram) Observe(labels map[string]string, value float64) {
	key := labelsKey(labels, h.labels)
	h.mu.Lock()
	defer h.mu.Unlock()

	hv, exists := h.values[key]
	if !exists {
		hv = &histogramValue{
			buckets: make(map[float64]uint64),
		}
		h.values[key] = hv
	}

	hv.count++
	hv.sum += value

	for _, bucket := range h.buckets {
		if value <= bucket {
			hv.buckets[bucket]++
		}
	}
}

func (h *Histogram) Collect(timestamp time.Time) []Metric {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var metrics []Metric
	for key, hv := range h.values {
		labels := parseLabelsKey(key)
		labels["le"] = "+Inf"

		// Add sum and count
		metrics = append(metrics, Metric{
			Name:      h.name + "_sum",
			Type:      MetricTypeCounter,
			Value:     hv.sum,
			Labels:    labels,
			Timestamp: timestamp,
		})
		metrics = append(metrics, Metric{
			Name:      h.name + "_count",
			Type:      MetricTypeCounter,
			Value:     float64(hv.count),
			Labels:    labels,
			Timestamp: timestamp,
		})

		// Add bucket counts
		for _, bucket := range h.buckets {
			bucketLabels := make(map[string]string)
			for k, v := range labels {
				bucketLabels[k] = v
			}
			bucketLabels["le"] = fmt.Sprintf("%g", bucket)

			metrics = append(metrics, Metric{
				Name:      h.name + "_bucket",
				Type:      MetricTypeCounter,
				Value:     float64(hv.buckets[bucket]),
				Labels:    bucketLabels,
				Timestamp: timestamp,
			})
		}
	}
	return metrics
}

// Summary metric type
type Summary struct {
	name       string
	labels     []string
	objectives map[float64]float64
	mu         sync.RWMutex
	values     map[string]*summaryValue
}

type summaryValue struct {
	observations []float64
	count        uint64
	sum          float64
}

func NewSummary(name string, labels []string, objectives map[float64]float64) *Summary {
	if objectives == nil {
		objectives = map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.95: 0.01, 0.99: 0.001}
	}
	return &Summary{
		name:       name,
		labels:     labels,
		objectives: objectives,
		values:     make(map[string]*summaryValue),
	}
}

func (s *Summary) Observe(labels map[string]string, value float64) {
	key := labelsKey(labels, s.labels)
	s.mu.Lock()
	defer s.mu.Unlock()

	sv, exists := s.values[key]
	if !exists {
		sv = &summaryValue{
			observations: make([]float64, 0),
		}
		s.values[key] = sv
	}

	sv.observations = append(sv.observations, value)
	sv.count++
	sv.sum += value
}

func (s *Summary) Collect(timestamp time.Time) []Metric {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var metrics []Metric
	for key, sv := range s.values {
		labels := parseLabelsKey(key)

		// Add sum and count
		metrics = append(metrics, Metric{
			Name:      s.name + "_sum",
			Type:      MetricTypeCounter,
			Value:     sv.sum,
			Labels:    labels,
			Timestamp: timestamp,
		})
		metrics = append(metrics, Metric{
			Name:      s.name + "_count",
			Type:      MetricTypeCounter,
			Value:     float64(sv.count),
			Labels:    labels,
			Timestamp: timestamp,
		})

		// Add quantiles
		for quantile, _ := range s.objectives {
			value := calculateQuantile(sv.observations, quantile)
			quantileLabels := make(map[string]string)
			for k, v := range labels {
				quantileLabels[k] = v
			}
			quantileLabels["quantile"] = fmt.Sprintf("%g", quantile)

			metrics = append(metrics, Metric{
				Name:      s.name,
				Type:      MetricTypeSummary,
				Value:     value,
				Labels:    quantileLabels,
				Timestamp: timestamp,
			})
		}
	}
	return metrics
}

// Tracing support
type Span struct {
	TraceID    string                 `json:"trace_id"`
	SpanID     string                 `json:"span_id"`
	ParentID   string                 `json:"parent_id,omitempty"`
	Name       string                 `json:"name"`
	StartTime  time.Time              `json:"start_time"`
	EndTime    *time.Time             `json:"end_time,omitempty"`
	Duration   time.Duration          `json:"duration,omitempty"`
	Status     string                 `json:"status"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
	Events     []SpanEvent            `json:"events,omitempty"`
}

type SpanEvent struct {
	Name       string                 `json:"name"`
	Timestamp  time.Time              `json:"timestamp"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// TracerProvider provides tracing capabilities
type TracerProvider struct {
	exporters []SpanExporter
	mu        sync.RWMutex
	spans     map[string]*Span
}

// SpanExporter interface for exporting spans
type SpanExporter interface {
	ExportSpans(ctx context.Context, spans []*Span) error
}

// NewTracerProvider creates a new tracer provider
func NewTracerProvider() *TracerProvider {
	return &TracerProvider{
		spans: make(map[string]*Span),
	}
}

// RegisterExporter registers a span exporter
func (p *TracerProvider) RegisterExporter(exporter SpanExporter) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.exporters = append(p.exporters, exporter)
}

// StartSpan starts a new span
func (p *TracerProvider) StartSpan(ctx context.Context, name string, opts ...SpanOption) (*Span, context.Context) {
	span := &Span{
		TraceID:    generateTraceID(),
		SpanID:     generateSpanID(),
		Name:       name,
		StartTime:  time.Now(),
		Status:     "ok",
		Attributes: make(map[string]interface{}),
	}

	// Check for parent span in context
	if parentSpan, ok := ctx.Value(spanContextKey).(*Span); ok {
		span.TraceID = parentSpan.TraceID
		span.ParentID = parentSpan.SpanID
	}

	// Apply options
	for _, opt := range opts {
		opt(span)
	}

	p.mu.Lock()
	p.spans[span.SpanID] = span
	p.mu.Unlock()

	// Add span to context
	ctx = context.WithValue(ctx, spanContextKey, span)

	return span, ctx
}

// EndSpan ends a span
func (p *TracerProvider) EndSpan(span *Span) {
	now := time.Now()
	span.EndTime = &now
	span.Duration = now.Sub(span.StartTime)

	// Export span
	p.mu.RLock()
	exporters := p.exporters
	p.mu.RUnlock()

	for _, exporter := range exporters {
		go func(e SpanExporter) {
			e.ExportSpans(context.Background(), []*Span{span})
		}(exporter)
	}
}

// SpanOption configures a span
type SpanOption func(*Span)

// WithAttributes sets span attributes
func WithAttributes(attrs map[string]interface{}) SpanOption {
	return func(s *Span) {
		for k, v := range attrs {
			s.Attributes[k] = v
		}
	}
}

// WithParent sets the parent span
func WithParent(parent *Span) SpanOption {
	return func(s *Span) {
		if parent != nil {
			s.TraceID = parent.TraceID
			s.ParentID = parent.SpanID
		}
	}
}

// WithStatus sets the span status
func WithStatus(status string) SpanOption {
	return func(s *Span) {
		s.Status = status
	}
}

// AddEvent adds an event to a span
func (s *Span) AddEvent(name string, attrs map[string]interface{}) {
	event := SpanEvent{
		Name:       name,
		Timestamp:  time.Now(),
		Attributes: attrs,
	}
	s.Events = append(s.Events, event)
}

// SetAttribute sets a single attribute
func (s *Span) SetAttribute(key string, value interface{}) {
	if s.Attributes == nil {
		s.Attributes = make(map[string]interface{})
	}
	s.Attributes[key] = value
}

// JSONSpanExporter exports spans as JSON
type JSONSpanExporter struct {
	output chan<- []byte
}

// NewJSONSpanExporter creates a new JSON exporter
func NewJSONSpanExporter(output chan<- []byte) *JSONSpanExporter {
	return &JSONSpanExporter{output: output}
}

func (e *JSONSpanExporter) ExportSpans(ctx context.Context, spans []*Span) error {
	for _, span := range spans {
		data, err := json.Marshal(span)
		if err != nil {
			return err
		}
		if e.output != nil {
			select {
			case e.output <- data:
			default:
				// Channel full, drop
			}
		}
	}
	return nil
}

// Helper types and functions
type contextKey string

const spanContextKey contextKey = "span"

func labelsKey(labels map[string]string, order []string) string {
	key := ""
	for _, k := range order {
		if v, ok := labels[k]; ok {
			key += k + "=" + v + ","
		}
	}
	return key
}

func parseLabelsKey(key string) map[string]string {
	labels := make(map[string]string)
	if key == "" {
		return labels
	}
	// Simple parsing - in production would be more robust
	pairs := splitString(key, ",")
	for _, pair := range pairs {
		kv := splitString(pair, "=")
		if len(kv) == 2 {
			labels[kv[0]] = kv[1]
		}
	}
	return labels
}

func splitString(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i:i+1] == sep {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

func generateTraceID() string {
	return fmt.Sprintf("%016x", time.Now().UnixNano())
}

func generateSpanID() string {
	return fmt.Sprintf("%016x", time.Now().UnixNano()^0x12345678)
}

func calculateQuantile(values []float64, q float64) float64 {
	if len(values) == 0 {
		return 0
	}
	// Simple quantile calculation
	// In production, use a proper algorithm
	index := int(float64(len(values)-1) * q)
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}
