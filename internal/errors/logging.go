package errors

import (
	stdctx "context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"time"
)

// LogLevel log level
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// ErrorLogger error logger
type ErrorLogger struct {
	logger      *slog.Logger
	serviceName string
	version     string
	enableJSON  bool
}

// NewErrorLogger creates an error logger
func NewErrorLogger(serviceName, version string, enableJSON bool) *ErrorLogger {
	var logger *slog.Logger

	if enableJSON {
		// Structured logging in JSON format
		opts := &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}
		handler := slog.NewJSONHandler(os.Stdout, opts)
		logger = slog.New(handler)
	} else {
		// Logging in text format
		opts := &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}
		handler := slog.NewTextHandler(os.Stdout, opts)
		logger = slog.New(handler)
	}

	return &ErrorLogger{
		logger:      logger,
		serviceName: serviceName,
		version:     version,
		enableJSON:  enableJSON,
	}
}

// LogError logs an error
func (el *ErrorLogger) LogError(err *AppError, context map[string]interface{}) {
	// Build the log attributes
	attrs := []slog.Attr{
		slog.String("service", el.serviceName),
		slog.String("version", el.version),
		slog.String("error_code", string(err.Code)),
		slog.String("operation", err.Operation),
		slog.String("message", err.Message),
		slog.String("severity", string(err.Severity)),
		slog.String("category", string(err.Category)),
		slog.Bool("user_error", err.UserError),
		slog.Bool("retryable", err.Retryable),
		slog.Time("timestamp", err.Timestamp),
		slog.String("trace_id", err.TraceID),
		slog.Int("http_status", err.HTTPStatusCode),
	}

	// Add the details
	if err.Details != "" {
		attrs = append(attrs, slog.String("details", err.Details))
	}

	// Add the retry info
	if err.Retryable && err.RetryDelay > 0 {
		attrs = append(attrs,
			slog.Duration("retry_delay", err.RetryDelay),
			slog.Int("retry_count", err.RetryCount),
		)
	}

	// Add context info
	if err.Context != nil {
		for key, value := range err.Context {
			attrs = append(attrs, slog.Any(key, value))
		}
	}

	// Add extra context (ranging over a nil map is a safe empty iteration)
	for key, value := range context {
		attrs = append(attrs, slog.Any(key, value))
	}

	// Add the original error
	if err.Cause != nil {
		attrs = append(attrs, slog.String("cause", err.Cause.Error()))
	}

	// Choose the log level based on the severity
	var logLevel slog.Level
	switch err.Severity {
	case SeverityLow:
		logLevel = slog.LevelInfo
	case SeverityMedium:
		logLevel = slog.LevelWarn
	case SeverityHigh, SeverityCritical:
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelError
	}

	el.logger.LogAttrs(stdctx.TODO(), logLevel, "Application error occurred", attrs...)
}

// LogErrorWithMetrics logs an error and updates metrics
func (el *ErrorLogger) LogErrorWithMetrics(err *AppError, context map[string]interface{}, metricsCollector MetricsCollector) {
	// Log the error
	el.LogError(err, context)

	// Update metrics
	if metricsCollector != nil {
		metricsCollector.IncrementErrorCount(err.Code, err.Severity, err.Category)
		metricsCollector.RecordErrorDuration(err.Code, time.Since(err.Timestamp))
	}
}

// LogPanic logs a panic error
func (el *ErrorLogger) LogPanic(recovered interface{}, context map[string]interface{}) {
	attrs := []slog.Attr{
		slog.String("service", el.serviceName),
		slog.String("version", el.version),
		slog.String("type", "panic"),
		slog.Time("timestamp", time.Now()),
	}

	// Add the panic info
	if recoveredStr, ok := recovered.(string); ok {
		attrs = append(attrs, slog.String("panic", recoveredStr))
	} else {
		attrs = append(attrs, slog.Any("panic", recovered))
	}

	// Add context info (ranging over a nil map is a safe empty iteration)
	for key, value := range context {
		attrs = append(attrs, slog.Any(key, value))
	}

	el.logger.LogAttrs(stdctx.TODO(), slog.LevelError, "Application panic occurred", attrs...)
}

// MetricsCollector metrics collector interface
type MetricsCollector interface {
	IncrementErrorCount(code ErrorCode, severity ErrorSeverity, category ErrorCategory)
	RecordErrorDuration(code ErrorCode, duration time.Duration)
	IncrementHTTPRequestCount(method, path, statusCode string)
	RecordHTTPRequestDuration(method, path string, duration time.Duration)
}

// DefaultMetricsCollector default metrics collector
type DefaultMetricsCollector struct {
	errorCounts      map[ErrorCode]int64
	errorDurations   map[ErrorCode]time.Duration
	requestCounts    map[string]int64
	requestDurations map[string]time.Duration
}

// NewDefaultMetricsCollector creates a default metrics collector
func NewDefaultMetricsCollector() *DefaultMetricsCollector {
	return &DefaultMetricsCollector{
		errorCounts:      make(map[ErrorCode]int64),
		errorDurations:   make(map[ErrorCode]time.Duration),
		requestCounts:    make(map[string]int64),
		requestDurations: make(map[string]time.Duration),
	}
}

// IncrementErrorCount increments the error count
func (dmc *DefaultMetricsCollector) IncrementErrorCount(code ErrorCode, severity ErrorSeverity, category ErrorCategory) {
	dmc.errorCounts[code]++
}

// RecordErrorDuration records the error duration
func (dmc *DefaultMetricsCollector) RecordErrorDuration(code ErrorCode, duration time.Duration) {
	dmc.errorDurations[code] = duration
}

// IncrementHTTPRequestCount increments the HTTP request count
func (dmc *DefaultMetricsCollector) IncrementHTTPRequestCount(method, path, statusCode string) {
	key := fmt.Sprintf("%s %s %s", method, path, statusCode)
	dmc.requestCounts[key]++
}

// RecordHTTPRequestDuration records the HTTP request duration
func (dmc *DefaultMetricsCollector) RecordHTTPRequestDuration(method, path string, duration time.Duration) {
	key := fmt.Sprintf("%s %s", method, path)
	dmc.requestDurations[key] = duration
}

// GetErrorCounts gets the error counts
func (dmc *DefaultMetricsCollector) GetErrorCounts() map[ErrorCode]int64 {
	result := make(map[ErrorCode]int64)
	for code, count := range dmc.errorCounts {
		result[code] = count
	}
	return result
}

// GetErrorDurations gets the error durations
func (dmc *DefaultMetricsCollector) GetErrorDurations() map[ErrorCode]time.Duration {
	result := make(map[ErrorCode]time.Duration)
	for code, duration := range dmc.errorDurations {
		result[code] = duration
	}
	return result
}

// GetRequestCounts gets the request counts
func (dmc *DefaultMetricsCollector) GetRequestCounts() map[string]int64 {
	result := make(map[string]int64)
	for key, count := range dmc.requestCounts {
		result[key] = count
	}
	return result
}

// Reset resets all metrics
func (dmc *DefaultMetricsCollector) Reset() {
	dmc.errorCounts = make(map[ErrorCode]int64)
	dmc.errorDurations = make(map[ErrorCode]time.Duration)
	dmc.requestCounts = make(map[string]int64)
	dmc.requestDurations = make(map[string]time.Duration)
}

// ToJSON converts the metrics to JSON
func (dmc *DefaultMetricsCollector) ToJSON() []byte {
	metrics := map[string]interface{}{
		"error_counts":      dmc.GetErrorCounts(),
		"error_durations":   dmc.GetErrorDurations(),
		"request_counts":    dmc.GetRequestCounts(),
		"request_durations": dmc.requestDurations,
		"timestamp":         time.Now(),
	}

	// Every field of metrics is a type natively supported by encoding/json
	// (map[string]int64, map[ErrorCode]int64, map[...]time.Duration,
	// time.Time; ErrorCode is backed by a string), so MarshalIndent always
	// succeeds and there is no error path.
	data, _ := json.MarshalIndent(metrics, "", "  ")
	return data
}

// ErrorStats error statistics
type ErrorStats struct {
	TotalErrors      int64                   `json:"totalErrors"`
	ErrorsByCode     map[ErrorCode]int64     `json:"errorsByCode"`
	ErrorsBySeverity map[ErrorSeverity]int64 `json:"errorsBySeverity"`
	ErrorsByCategory map[ErrorCategory]int64 `json:"errorsByCategory"`
	RecentErrors     []*AppError             `json:"recentErrors,omitempty"`
	TimeRange        TimeRange               `json:"timeRange"`
}

// TimeRange time range
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// ErrorAnalyzer error analyzer
type ErrorAnalyzer struct {
	errors           []*AppError
	maxErrors        int
	metricsCollector MetricsCollector
}

// NewErrorAnalyzer creates an error analyzer
func NewErrorAnalyzer(maxErrors int, collector MetricsCollector) *ErrorAnalyzer {
	return &ErrorAnalyzer{
		errors:           make([]*AppError, 0, maxErrors),
		maxErrors:        maxErrors,
		metricsCollector: collector,
	}
}

// AddError adds an error to the analyzer
func (ea *ErrorAnalyzer) AddError(err *AppError) {
	if len(ea.errors) >= ea.maxErrors {
		// Remove the oldest error
		ea.errors = ea.errors[1:]
	}
	ea.errors = append(ea.errors, err)

	// Update metrics
	if ea.metricsCollector != nil {
		ea.metricsCollector.IncrementErrorCount(err.Code, err.Severity, err.Category)
		ea.metricsCollector.RecordErrorDuration(err.Code, time.Since(err.Timestamp))
	}
}

// GetStats gets the error statistics
func (ea *ErrorAnalyzer) GetStats() *ErrorStats {
	stats := &ErrorStats{
		TotalErrors:      int64(len(ea.errors)),
		ErrorsByCode:     make(map[ErrorCode]int64),
		ErrorsBySeverity: make(map[ErrorSeverity]int64),
		ErrorsByCategory: make(map[ErrorCategory]int64),
		RecentErrors:     make([]*AppError, 0),
	}

	if len(ea.errors) > 0 {
		stats.TimeRange = TimeRange{
			Start: ea.errors[0].Timestamp,
			End:   ea.errors[len(ea.errors)-1].Timestamp,
		}

		// Return only the 10 most recent errors
		recentCount := 10
		if len(ea.errors) < recentCount {
			recentCount = len(ea.errors)
		}
		stats.RecentErrors = ea.errors[len(ea.errors)-recentCount:]
	}

	// Tally the error distribution
	for _, err := range ea.errors {
		stats.ErrorsByCode[err.Code]++
		stats.ErrorsBySeverity[err.Severity]++
		stats.ErrorsByCategory[err.Category]++
	}

	return stats
}

// GetTopErrors gets the most common errors
func (ea *ErrorAnalyzer) GetTopErrors(limit int) []ErrorCode {
	counts := make(map[ErrorCode]int64)
	for _, err := range ea.errors {
		counts[err.Code]++
	}

	// Sort and return the top N
	type errorCount struct {
		code  ErrorCode
		count int64
	}

	var sortedErrors []errorCount
	for code, count := range counts {
		sortedErrors = append(sortedErrors, errorCount{code: code, count: count})
	}

	// Sort by descending count, then ascending code. The previous bubble sort
	// only ordered by count, so ties followed the random map iteration order
	// (the same input could yield a different top order on two calls), and
	// covering the swap branch in tests was left to chance; the deterministic
	// sort makes the same input always produce the same output.
	sort.Slice(sortedErrors, func(i, j int) bool {
		if sortedErrors[i].count != sortedErrors[j].count {
			return sortedErrors[i].count > sortedErrors[j].count
		}
		return sortedErrors[i].code < sortedErrors[j].code
	})

	if limit > 0 && limit < len(sortedErrors) {
		sortedErrors = sortedErrors[:limit]
	}

	result := make([]ErrorCode, len(sortedErrors))
	for i, ec := range sortedErrors {
		result[i] = ec.code
	}

	return result
}

// Clear clears the recorded errors
func (ea *ErrorAnalyzer) Clear() {
	ea.errors = ea.errors[:0]
}
