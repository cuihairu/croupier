package errors

import (
	"testing"
	"time"
)

func TestNewErrorLogger(t *testing.T) {
	t.Run("text format", func(t *testing.T) {
		el := NewErrorLogger("test-service", "1.0.0", false)
		if el == nil {
			t.Fatal("expected non-nil logger")
		}
		if el.serviceName != "test-service" {
			t.Errorf("expected service name 'test-service', got %q", el.serviceName)
		}
		if el.version != "1.0.0" {
			t.Errorf("expected version '1.0.0', got %q", el.version)
		}
		if el.enableJSON {
			t.Error("expected enableJSON to be false")
		}
	})

	t.Run("json format", func(t *testing.T) {
		el := NewErrorLogger("test-service", "2.0.0", true)
		if !el.enableJSON {
			t.Error("expected enableJSON to be true")
		}
	})
}

func TestErrorLogger_LogError(t *testing.T) {
	el := NewErrorLogger("test", "1.0", false)

	tests := []struct {
		name string
		err  *AppError
		ctx  map[string]interface{}
	}{
		{
			name: "basic error",
			err: &AppError{
				Code:           ErrCodeInternal,
				Message:        "test error",
				Operation:      "test-op",
				Severity:       SeverityHigh,
				Category:       CategorySystem,
				HTTPStatusCode: 500,
				Timestamp:      time.Now(),
			},
			ctx: nil,
		},
		{
			name: "with details",
			err: &AppError{
				Code:           ErrCodeInvalidInput,
				Message:        "validation failed",
				Details:        "field X is required",
				Severity:       SeverityLow,
				HTTPStatusCode: 400,
				Timestamp:      time.Now(),
			},
			ctx: map[string]interface{}{"request_id": "req-123"},
		},
		{
			name: "with retry info",
			err: &AppError{
				Code:           ErrCodeTimeout,
				Message:        "timeout",
				Severity:       SeverityMedium,
				Retryable:      true,
				RetryDelay:     time.Second,
				RetryCount:     3,
				HTTPStatusCode: 504,
				Timestamp:      time.Now(),
			},
			ctx: nil,
		},
		{
			name: "with cause",
			err: &AppError{
				Code:           ErrCodeDatabaseError,
				Message:        "db error",
				Severity:       SeverityCritical,
				Cause:          &AppError{Code: ErrCodeNetworkError, Message: "connection refused"},
				HTTPStatusCode: 500,
				Timestamp:      time.Now(),
			},
			ctx: nil,
		},
		{
			name: "with context",
			err: &AppError{
				Code:           ErrCodeInternal,
				Message:        "error with context",
				Severity:       SeverityHigh,
				Context:        map[string]interface{}{"key1": "val1", "key2": 42},
				HTTPStatusCode: 500,
				Timestamp:      time.Now(),
			},
			ctx: map[string]interface{}{"extra": "data"},
		},
		{
			name: "severity low",
			err: &AppError{
				Code:           ErrCodeInvalidInput,
				Message:        "low severity",
				Severity:       SeverityLow,
				HTTPStatusCode: 400,
				Timestamp:      time.Now(),
			},
			ctx: nil,
		},
		{
			name: "unknown severity",
			err: &AppError{
				Code:           ErrCodeInternal,
				Message:        "unknown severity",
				Severity:       ErrorSeverity("unknown"),
				HTTPStatusCode: 500,
				Timestamp:      time.Now(),
			},
			ctx: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// LogError should not panic
			el.LogError(tt.err, tt.ctx)
		})
	}
}

func TestErrorLogger_LogErrorWithMetrics(t *testing.T) {
	el := NewErrorLogger("test", "1.0", false)
	collector := NewDefaultMetricsCollector()

	err := &AppError{
		Code:           ErrCodeInternal,
		Message:        "test",
		Severity:       SeverityHigh,
		Category:       CategorySystem,
		HTTPStatusCode: 500,
		Timestamp:      time.Now(),
	}

	// Should not panic
	el.LogErrorWithMetrics(err, nil, collector)

	// With nil collector
	el.LogErrorWithMetrics(err, nil, nil)
}

func TestErrorLogger_LogPanic(t *testing.T) {
	el := NewErrorLogger("test", "1.0", false)

	t.Run("string panic", func(t *testing.T) {
		// Should not panic
		el.LogPanic("something went wrong", map[string]interface{}{"goroutine": "worker-1"})
	})

	t.Run("error panic", func(t *testing.T) {
		el.LogPanic(&AppError{Code: ErrCodeInternal, Message: "error"}, nil)
	})

	t.Run("nil context", func(t *testing.T) {
		el.LogPanic("panic message", nil)
	})
}

func TestNewDefaultMetricsCollector(t *testing.T) {
	dmc := NewDefaultMetricsCollector()
	if dmc == nil {
		t.Fatal("expected non-nil collector")
	}
	if dmc.errorCounts == nil || dmc.errorDurations == nil || dmc.requestCounts == nil || dmc.requestDurations == nil {
		t.Error("expected initialized maps")
	}
}

func TestDefaultMetricsCollector_IncrementErrorCount(t *testing.T) {
	dmc := NewDefaultMetricsCollector()
	dmc.IncrementErrorCount(ErrCodeInternal, SeverityHigh, CategorySystem)
	dmc.IncrementErrorCount(ErrCodeInternal, SeverityHigh, CategorySystem)
	dmc.IncrementErrorCount(ErrCodeInvalidInput, SeverityLow, CategoryValidation)

	counts := dmc.GetErrorCounts()
	if counts[ErrCodeInternal] != 2 {
		t.Errorf("expected 2 internal errors, got %d", counts[ErrCodeInternal])
	}
	if counts[ErrCodeInvalidInput] != 1 {
		t.Errorf("expected 1 validation error, got %d", counts[ErrCodeInvalidInput])
	}
}

func TestDefaultMetricsCollector_RecordErrorDuration(t *testing.T) {
	dmc := NewDefaultMetricsCollector()
	dmc.RecordErrorDuration(ErrCodeInternal, 100*time.Millisecond)

	durations := dmc.GetErrorDurations()
	if durations[ErrCodeInternal] != 100*time.Millisecond {
		t.Errorf("expected 100ms, got %v", durations[ErrCodeInternal])
	}
}

func TestDefaultMetricsCollector_IncrementHTTPRequestCount(t *testing.T) {
	dmc := NewDefaultMetricsCollector()
	dmc.IncrementHTTPRequestCount("GET", "/api/test", "200")
	dmc.IncrementHTTPRequestCount("GET", "/api/test", "200")

	counts := dmc.GetRequestCounts()
	key := "GET /api/test 200"
	if counts[key] != 2 {
		t.Errorf("expected 2 requests, got %d", counts[key])
	}
}

func TestDefaultMetricsCollector_RecordHTTPRequestDuration(t *testing.T) {
	dmc := NewDefaultMetricsCollector()
	dmc.RecordHTTPRequestDuration("GET", "/api/test", 50*time.Millisecond)

	// requestDurations is not exposed via getter, but should not panic
}

func TestDefaultMetricsCollector_Reset(t *testing.T) {
	dmc := NewDefaultMetricsCollector()
	dmc.IncrementErrorCount(ErrCodeInternal, SeverityHigh, CategorySystem)
	dmc.IncrementHTTPRequestCount("GET", "/api/test", "200")
	dmc.RecordErrorDuration(ErrCodeInternal, time.Second)

	dmc.Reset()

	if len(dmc.GetErrorCounts()) != 0 {
		t.Error("expected empty error counts after reset")
	}
	if len(dmc.GetRequestCounts()) != 0 {
		t.Error("expected empty request counts after reset")
	}
	if len(dmc.GetErrorDurations()) != 0 {
		t.Error("expected empty error durations after reset")
	}
}

func TestDefaultMetricsCollector_ToJSON(t *testing.T) {
	dmc := NewDefaultMetricsCollector()
	dmc.IncrementErrorCount(ErrCodeInternal, SeverityHigh, CategorySystem)

	data := dmc.ToJSON()
	if len(data) == 0 {
		t.Error("expected non-empty JSON")
	}
	// Should be valid JSON
	if data[0] != '{' {
		t.Error("expected JSON object")
	}
}

func TestNewErrorAnalyzer(t *testing.T) {
	ea := NewErrorAnalyzer(100, nil)
	if ea == nil {
		t.Fatal("expected non-nil analyzer")
	}
	if ea.maxErrors != 100 {
		t.Errorf("expected maxErrors 100, got %d", ea.maxErrors)
	}
}

func TestErrorAnalyzer_AddError(t *testing.T) {
	collector := NewDefaultMetricsCollector()
	ea := NewErrorAnalyzer(3, collector)

	ea.AddError(&AppError{Code: ErrCodeInternal, Severity: SeverityHigh, Category: CategorySystem, Timestamp: time.Now()})
	ea.AddError(&AppError{Code: ErrCodeInvalidInput, Severity: SeverityLow, Category: CategoryValidation, Timestamp: time.Now()})

	if len(ea.errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(ea.errors))
	}
}

func TestErrorAnalyzer_AddError_MaxExceeded(t *testing.T) {
	ea := NewErrorAnalyzer(2, nil)

	ea.AddError(&AppError{Code: ErrCodeInternal, Timestamp: time.Now()})
	ea.AddError(&AppError{Code: ErrCodeInvalidInput, Timestamp: time.Now()})
	ea.AddError(&AppError{Code: ErrCodeTimeout, Timestamp: time.Now()}) // Should remove first

	if len(ea.errors) != 2 {
		t.Errorf("expected 2 errors (max), got %d", len(ea.errors))
	}
	if ea.errors[0].Code != ErrCodeInvalidInput {
		t.Errorf("expected first error to be Validation, got %s", ea.errors[0].Code)
	}
}

func TestErrorAnalyzer_GetStats(t *testing.T) {
	ea := NewErrorAnalyzer(100, nil)

	// Empty stats
	stats := ea.GetStats()
	if stats.TotalErrors != 0 {
		t.Errorf("expected 0 total errors, got %d", stats.TotalErrors)
	}

	// With errors
	now := time.Now()
	ea.AddError(&AppError{Code: ErrCodeInternal, Severity: SeverityHigh, Category: CategorySystem, Timestamp: now})
	ea.AddError(&AppError{Code: ErrCodeInternal, Severity: SeverityHigh, Category: CategorySystem, Timestamp: now.Add(time.Second)})
	ea.AddError(&AppError{Code: ErrCodeInvalidInput, Severity: SeverityLow, Category: CategoryValidation, Timestamp: now.Add(2 * time.Second)})

	stats = ea.GetStats()
	if stats.TotalErrors != 3 {
		t.Errorf("expected 3 total errors, got %d", stats.TotalErrors)
	}
	if stats.ErrorsByCode[ErrCodeInternal] != 2 {
		t.Errorf("expected 2 internal errors, got %d", stats.ErrorsByCode[ErrCodeInternal])
	}
	if stats.ErrorsBySeverity[SeverityHigh] != 2 {
		t.Errorf("expected 2 high severity errors, got %d", stats.ErrorsBySeverity[SeverityHigh])
	}
	if len(stats.RecentErrors) != 3 {
		t.Errorf("expected 3 recent errors, got %d", len(stats.RecentErrors))
	}
}

func TestErrorAnalyzer_GetTopErrors(t *testing.T) {
	ea := NewErrorAnalyzer(100, nil)

	ea.AddError(&AppError{Code: ErrCodeInternal, Timestamp: time.Now()})
	ea.AddError(&AppError{Code: ErrCodeInternal, Timestamp: time.Now()})
	ea.AddError(&AppError{Code: ErrCodeInvalidInput, Timestamp: time.Now()})

	top := ea.GetTopErrors(1)
	if len(top) != 1 {
		t.Fatalf("expected 1 top error, got %d", len(top))
	}
	if top[0] != ErrCodeInternal {
		t.Errorf("expected ErrCodeInternal, got %s", top[0])
	}

	// No limit
	all := ea.GetTopErrors(0)
	if len(all) != 2 {
		t.Errorf("expected 2 error codes, got %d", len(all))
	}
}

func TestErrorAnalyzer_Clear(t *testing.T) {
	ea := NewErrorAnalyzer(100, nil)
	ea.AddError(&AppError{Code: ErrCodeInternal, Timestamp: time.Now()})
	ea.Clear()

	stats := ea.GetStats()
	if stats.TotalErrors != 0 {
		t.Errorf("expected 0 errors after clear, got %d", stats.TotalErrors)
	}
}
