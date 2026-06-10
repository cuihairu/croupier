package errors

import (
	"testing"
	"time"
)

func TestAppError_String(t *testing.T) {
	e := &AppError{
		Code:      ErrCodeInternal,
		Operation: "test-op",
		Message:   "something failed",
	}
	got := e.String()
	if got != e.Error() {
		t.Errorf("String() = %q, want %q", got, e.Error())
	}
}

func TestAppError_Unwrap(t *testing.T) {
	cause := &AppError{Code: ErrCodeNetworkError, Message: "connection refused"}
	e := &AppError{Code: ErrCodeDatabaseError, Message: "db error", Cause: cause}

	if e.Unwrap() != cause {
		t.Error("Unwrap should return cause")
	}

	e2 := &AppError{Code: ErrCodeInternal, Message: "no cause"}
	if e2.Unwrap() != nil {
		t.Error("Unwrap should return nil when no cause")
	}
}

func TestErrorCollection_Error(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		ec := &ErrorCollection{}
		if got := ec.Error(); got != "no errors" {
			t.Errorf("expected 'no errors', got %q", got)
		}
	})

	t.Run("single", func(t *testing.T) {
		ec := &ErrorCollection{
			Errors: []*AppError{{Code: ErrCodeInternal, Message: "err1"}},
			Total:  1,
		}
		got := ec.Error()
		if got == "" {
			t.Error("expected non-empty error string")
		}
	})

	t.Run("multiple", func(t *testing.T) {
		ec := &ErrorCollection{
			Errors: []*AppError{
				{Code: ErrCodeInternal, Message: "err1"},
				{Code: ErrCodeTimeout, Message: "err2"},
			},
			Total: 2,
		}
		got := ec.Error()
		if got == "" {
			t.Error("expected non-empty error string")
		}
	})
}

func TestErrorCollection_Is(t *testing.T) {
	ec := &ErrorCollection{
		Errors: []*AppError{
			{Code: ErrCodeInternal, Message: "err1"},
			{Code: ErrCodeTimeout, Message: "err2"},
		},
		Total: 2,
	}

	if !ec.Is(&AppError{Code: ErrCodeInternal}) {
		t.Error("expected to find ErrCodeInternal")
	}
	if !ec.Is(&AppError{Code: ErrCodeTimeout}) {
		t.Error("expected to find ErrCodeTimeout")
	}
	if ec.Is(&AppError{Code: ErrCodeNotFound}) {
		t.Error("should not find ErrCodeNotFound")
	}
	if ec.Is(nil) {
		t.Error("should not match nil")
	}
}

func TestAppError_WithContext(t *testing.T) {
	e := &AppError{Code: ErrCodeInternal}
	e.WithContext("key1", "val1")
	if e.Context["key1"] != "val1" {
		t.Error("expected context key1=val1")
	}
	// nil map should be initialized
	e2 := &AppError{Code: ErrCodeInternal}
	e2.WithContext("k", "v")
	if e2.Context == nil {
		t.Error("expected context map to be initialized")
	}
}

func TestAppError_WithDetails(t *testing.T) {
	e := &AppError{Code: ErrCodeInternal}
	e.WithDetails("extra info")
	if e.Details != "extra info" {
		t.Errorf("expected 'extra info', got %q", e.Details)
	}
}

func TestAppError_WithRetry(t *testing.T) {
	e := &AppError{Code: ErrCodeTimeout}
	e.WithRetry(time.Second, 3)
	if e.RetryCount != 3 {
		t.Errorf("expected retry count 3, got %d", e.RetryCount)
	}
	if e.RetryDelay != time.Second {
		t.Errorf("expected retry delay 1s, got %v", e.RetryDelay)
	}
	if !e.Retryable {
		t.Error("expected retryable to be true")
	}
}

func TestAppError_WithHTTPHeader(t *testing.T) {
	e := &AppError{Code: ErrCodeInternal}
	e.WithHTTPHeader("X-Request-ID", "123")
	if e.HTTPHeaders["X-Request-ID"] != "123" {
		t.Error("expected header to be set")
	}
	// nil map
	e2 := &AppError{Code: ErrCodeInternal}
	e2.WithHTTPHeader("K", "V")
	if e2.HTTPHeaders == nil {
		t.Error("expected headers map to be initialized")
	}
}
