package errors

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// LogLevel 日志级别
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// ErrorLogger 错误日志记录器
type ErrorLogger struct {
	logger      *slog.Logger
	serviceName string
	version     string
	enableJSON  bool
}

// NewErrorLogger 创建错误日志记录器
func NewErrorLogger(serviceName, version string, enableJSON bool) *ErrorLogger {
	var logger *slog.Logger

	if enableJSON {
		// JSON格式的结构化日志
		opts := &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}
		handler := slog.NewJSONHandler(os.Stdout, opts)
		logger = slog.New(handler)
	} else {
		// 文本格式的日志
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

// LogError 记录错误日志
func (el *ErrorLogger) LogError(err *AppError, context map[string]interface{}) {
	// 构建日志属性
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

	// 添加详细信息
	if err.Details != "" {
		attrs = append(attrs, slog.String("details", err.Details))
	}

	// 添加重试信息
	if err.Retryable && err.RetryDelay > 0 {
		attrs = append(attrs,
			slog.Duration("retry_delay", err.RetryDelay),
			slog.Int("retry_count", err.RetryCount),
		)
	}

	// 添加上下文信息
	if err.Context != nil {
		for key, value := range err.Context {
			attrs = append(attrs, slog.Any(key, value))
		}
	}

	// 添加额外上下文
	if context != nil {
		for key, value := range context {
			attrs = append(attrs, slog.Any(key, value))
		}
	}

	// 添加原始错误
	if err.Cause != nil {
		attrs = append(attrs, slog.String("cause", err.Cause.Error()))
	}

	// 根据严重级别选择日志级别
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

	el.logger.LogAttrs(nil, logLevel, "Application error occurred", attrs...)
}

// LogErrorWithMetrics 记录错误并更新指标
func (el *ErrorLogger) LogErrorWithMetrics(err *AppError, context map[string]interface{}, metricsCollector MetricsCollector) {
	// 记录日志
	el.LogError(err, context)

	// 更新指标
	if metricsCollector != nil {
		metricsCollector.IncrementErrorCount(err.Code, err.Severity, err.Category)
		metricsCollector.RecordErrorDuration(err.Code, time.Since(err.Timestamp))
	}
}

// LogPanic 记录panic错误
func (el *ErrorLogger) LogPanic(recovered interface{}, context map[string]interface{}) {
	attrs := []slog.Attr{
		slog.String("service", el.serviceName),
		slog.String("version", el.version),
		slog.String("type", "panic"),
		slog.Time("timestamp", time.Now()),
	}

	// 添加panic信息
	if recoveredStr, ok := recovered.(string); ok {
		attrs = append(attrs, slog.String("panic", recoveredStr))
	} else {
		attrs = append(attrs, slog.Any("panic", recovered))
	}

	// 添加上下文信息
	if context != nil {
		for key, value := range context {
			attrs = append(attrs, slog.Any(key, value))
		}
	}

	el.logger.LogAttrs(nil, slog.LevelError, "Application panic occurred", attrs...)
}

// MetricsCollector 指标收集器接口
type MetricsCollector interface {
	IncrementErrorCount(code ErrorCode, severity ErrorSeverity, category ErrorCategory)
	RecordErrorDuration(code ErrorCode, duration time.Duration)
	IncrementHTTPRequestCount(method, path, statusCode string)
	RecordHTTPRequestDuration(method, path string, duration time.Duration)
}

// DefaultMetricsCollector 默认指标收集器
type DefaultMetricsCollector struct {
	errorCounts     map[ErrorCode]int64
	errorDurations  map[ErrorCode]time.Duration
	requestCounts   map[string]int64
	requestDurations map[string]time.Duration
}

// NewDefaultMetricsCollector 创建默认指标收集器
func NewDefaultMetricsCollector() *DefaultMetricsCollector {
	return &DefaultMetricsCollector{
		errorCounts:      make(map[ErrorCode]int64),
		errorDurations:   make(map[ErrorCode]time.Duration),
		requestCounts:    make(map[string]int64),
		requestDurations: make(map[string]time.Duration),
	}
}

// IncrementErrorCount 增加错误计数
func (dmc *DefaultMetricsCollector) IncrementErrorCount(code ErrorCode, severity ErrorSeverity, category ErrorCategory) {
	dmc.errorCounts[code]++
}

// RecordErrorDuration 记录错误持续时间
func (dmc *DefaultMetricsCollector) RecordErrorDuration(code ErrorCode, duration time.Duration) {
	dmc.errorDurations[code] = duration
}

// IncrementHTTPRequestCount 增加HTTP请求计数
func (dmc *DefaultMetricsCollector) IncrementHTTPRequestCount(method, path, statusCode string) {
	key := fmt.Sprintf("%s %s %s", method, path, statusCode)
	dmc.requestCounts[key]++
}

// RecordHTTPRequestDuration 记录HTTP请求持续时间
func (dmc *DefaultMetricsCollector) RecordHTTPRequestDuration(method, path string, duration time.Duration) {
	key := fmt.Sprintf("%s %s", method, path)
	dmc.requestDurations[key] = duration
}

// GetErrorCounts 获取错误计数
func (dmc *DefaultMetricsCollector) GetErrorCounts() map[ErrorCode]int64 {
	result := make(map[ErrorCode]int64)
	for code, count := range dmc.errorCounts {
		result[code] = count
	}
	return result
}

// GetErrorDurations 获取错误持续时间
func (dmc *DefaultMetricsCollector) GetErrorDurations() map[ErrorCode]time.Duration {
	result := make(map[ErrorCode]time.Duration)
	for code, duration := range dmc.errorDurations {
		result[code] = duration
	}
	return result
}

// GetRequestCounts 获取请求计数
func (dmc *DefaultMetricsCollector) GetRequestCounts() map[string]int64 {
	result := make(map[string]int64)
	for key, count := range dmc.requestCounts {
		result[key] = count
	}
	return result
}

// Reset 重置所有指标
func (dmc *DefaultMetricsCollector) Reset() {
	dmc.errorCounts = make(map[ErrorCode]int64)
	dmc.errorDurations = make(map[ErrorCode]time.Duration)
	dmc.requestCounts = make(map[string]int64)
	dmc.requestDurations = make(map[string]time.Duration)
}

// ToJSON 将指标转换为JSON
func (dmc *DefaultMetricsCollector) ToJSON() []byte {
	metrics := map[string]interface{}{
		"error_counts":       dmc.GetErrorCounts(),
		"error_durations":    dmc.GetErrorDurations(),
		"request_counts":     dmc.GetRequestCounts(),
		"request_durations":  dmc.requestDurations,
		"timestamp":          time.Now(),
	}

	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return []byte("{}")
	}
	return data
}

// ErrorStats 错误统计信息
type ErrorStats struct {
	TotalErrors      int64                        `json:"total_errors"`
	ErrorsByCode     map[ErrorCode]int64          `json:"errors_by_code"`
	ErrorsBySeverity map[ErrorSeverity]int64      `json:"errors_by_severity"`
	ErrorsByCategory map[ErrorCategory]int64     `json:"errors_by_category"`
	RecentErrors    []*AppError                  `json:"recent_errors,omitempty"`
	TimeRange       TimeRange                    `json:"time_range"`
}

// TimeRange 时间范围
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// ErrorAnalyzer 错误分析器
type ErrorAnalyzer struct {
	errors         []*AppError
	maxErrors      int
	metricsCollector MetricsCollector
}

// NewErrorAnalyzer 创建错误分析器
func NewErrorAnalyzer(maxErrors int, collector MetricsCollector) *ErrorAnalyzer {
	return &ErrorAnalyzer{
		errors:         make([]*AppError, 0, maxErrors),
		maxErrors:      maxErrors,
		metricsCollector: collector,
	}
}

// AddError 添加错误到分析器
func (ea *ErrorAnalyzer) AddError(err *AppError) {
	if len(ea.errors) >= ea.maxErrors {
		// 移除最旧的错误
		ea.errors = ea.errors[1:]
	}
	ea.errors = append(ea.errors, err)

	// 更新指标
	if ea.metricsCollector != nil {
		ea.metricsCollector.IncrementErrorCount(err.Code, err.Severity, err.Category)
		ea.metricsCollector.RecordErrorDuration(err.Code, time.Since(err.Timestamp))
	}
}

// GetStats 获取错误统计信息
func (ea *ErrorAnalyzer) GetStats() *ErrorStats {
	stats := &ErrorStats{
		TotalErrors:      int64(len(ea.errors)),
		ErrorsByCode:     make(map[ErrorCode]int64),
		ErrorsBySeverity: make(map[ErrorSeverity]int64),
		ErrorsByCategory: make(map[ErrorCategory]int64),
		RecentErrors:    make([]*AppError, 0),
	}

	if len(ea.errors) > 0 {
		stats.TimeRange = TimeRange{
			Start: ea.errors[0].Timestamp,
			End:   ea.errors[len(ea.errors)-1].Timestamp,
		}

		// 只返回最近的10个错误
		recentCount := 10
		if len(ea.errors) < recentCount {
			recentCount = len(ea.errors)
		}
		stats.RecentErrors = ea.errors[len(ea.errors)-recentCount:]
	}

	// 统计错误分布
	for _, err := range ea.errors {
		stats.ErrorsByCode[err.Code]++
		stats.ErrorsBySeverity[err.Severity]++
		stats.ErrorsByCategory[err.Category]++
	}

	return stats
}

// GetTopErrors 获取最常见的错误
func (ea *ErrorAnalyzer) GetTopErrors(limit int) []ErrorCode {
	counts := make(map[ErrorCode]int64)
	for _, err := range ea.errors {
		counts[err.Code]++
	}

	// 排序并返回前N个
	type errorCount struct {
		code  ErrorCode
		count int64
	}

	var sortedErrors []errorCount
	for code, count := range counts {
		sortedErrors = append(sortedErrors, errorCount{code: code, count: count})
	}

	// 简单排序（按数量降序）
	for i := 0; i < len(sortedErrors); i++ {
		for j := i + 1; j < len(sortedErrors); j++ {
			if sortedErrors[j].count > sortedErrors[i].count {
				sortedErrors[i], sortedErrors[j] = sortedErrors[j], sortedErrors[i]
			}
		}
	}

	if limit > 0 && limit < len(sortedErrors) {
		sortedErrors = sortedErrors[:limit]
	}

	result := make([]ErrorCode, len(sortedErrors))
	for i, ec := range sortedErrors {
		result[i] = ec.code
	}

	return result
}

// Clear 清空错误记录
func (ea *ErrorAnalyzer) Clear() {
	ea.errors = ea.errors[:0]
}