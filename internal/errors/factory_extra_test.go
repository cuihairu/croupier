package errors

import (
	"errors"
	"testing"
)

// MockTraceGenerator mock trace ID generator
type MockTraceGenerator struct {
	traceID string
}

func (m *MockTraceGenerator) GenerateTraceID() string {
	return m.traceID
}

// TestErrorFactory_WithTraceGenerator tests setting the trace ID generator
func TestErrorFactory_WithTraceGenerator(t *testing.T) {
	factory := NewErrorFactory("test-service")
	mockGen := &MockTraceGenerator{traceID: "mock-trace-123"}

	newFactory := factory.WithTraceGenerator(mockGen)

	if newFactory == nil {
		t.Fatal("WithTraceGenerator returned nil")
	}

	if newFactory.traceGenerator != mockGen {
		t.Error("TraceGenerator not set correctly")
	}

	// Verify that the new generator is used
	err := newFactory.New(ErrCodeInternal, "test-op", nil)
	if err.TraceID != "mock-trace-123" {
		t.Errorf("Expected TraceID 'mock-trace-123', got '%s'", err.TraceID)
	}
}

// TestErrorFactory_Wrapf_Formatted tests wrapping an error with a formatted message
func TestErrorFactory_Wrapf_Formatted(t *testing.T) {
	factory := NewErrorFactory("test-service")

	cause := errors.New("original error")
	err := factory.Wrapf(cause, "test-operation", "Formatted error: %s %d", "value", 42)

	if err == nil {
		t.Fatal("Wrapf returned nil")
	}

	expectedMsg := "Formatted error: value 42"
	if err.Message != expectedMsg {
		t.Errorf("Expected message '%s', got '%s'", expectedMsg, err.Message)
	}

	if err.Cause != cause {
		t.Error("Cause not set correctly")
	}
}

// TestErrorFactory_Wrapf_NilCause tests wrapping a nil error
func TestErrorFactory_Wrapf_NilCause(t *testing.T) {
	factory := NewErrorFactory("test-service")

	err := factory.Wrapf(nil, "test-operation", "Formatted error: %s", "value")

	// Wrapf calls Wrap internally, which should return nil
	if err != nil {
		t.Error("Expected nil when wrapping nil error")
	}
}

// TestErrorFactory_InternalErrorf tests an internal error with a formatted message
func TestErrorFactory_InternalErrorf(t *testing.T) {
	factory := NewErrorFactory("test-service")
	cause := errors.New("database connection failed")

	err := factory.InternalErrorf("test-operation", cause, "Internal error: %s failed", "database")

	if err == nil {
		t.Fatal("InternalErrorf returned nil")
	}

	if err.Code != ErrCodeInternal {
		t.Errorf("Expected code %s, got %s", ErrCodeInternal, err.Code)
	}

	expectedMsg := "Internal error: database failed"
	if err.Message != expectedMsg {
		t.Errorf("Expected message '%s', got '%s'", expectedMsg, err.Message)
	}

	if err.Cause != cause {
		t.Error("Cause not set correctly")
	}
}

// TestErrorFactory_UnauthorizedError tests the unauthorized error
func TestErrorFactory_UnauthorizedError(t *testing.T) {
	factory := NewErrorFactory("test-service")
	cause := errors.New("invalid token")

	err := factory.UnauthorizedError("authenticate", cause)

	if err == nil {
		t.Fatal("UnauthorizedError returned nil")
	}

	if err.Code != ErrCodeUnauthorized {
		t.Errorf("Expected code %s, got %s", ErrCodeUnauthorized, err.Code)
	}

	if !err.UserError {
		t.Error("UnauthorizedError should be marked as user error")
	}

	if err.HTTPStatusCode != 401 {
		t.Errorf("Expected HTTP status 401, got %d", err.HTTPStatusCode)
	}

	if err.Cause != cause {
		t.Error("Cause not set correctly")
	}
}

// TestErrorFactory_ForbiddenError tests the forbidden-access error
func TestErrorFactory_ForbiddenError(t *testing.T) {
	factory := NewErrorFactory("test-service")
	cause := errors.New("insufficient permissions")

	err := factory.ForbiddenError("access-resource", "admin-panel", cause)

	if err == nil {
		t.Fatal("ForbiddenError returned nil")
	}

	if err.Code != ErrCodeForbidden {
		t.Errorf("Expected code %s, got %s", ErrCodeForbidden, err.Code)
	}

	expectedDetails := "Access to 'admin-panel' is forbidden"
	if err.Details != expectedDetails {
		t.Errorf("Expected details '%s', got '%s'", expectedDetails, err.Details)
	}

	if !err.UserError {
		t.Error("ForbiddenError should be marked as user error")
	}

	if err.HTTPStatusCode != 403 {
		t.Errorf("Expected HTTP status 403, got %d", err.HTTPStatusCode)
	}
}

// TestErrorFactory_ConflictError tests the conflict error
func TestErrorFactory_ConflictError(t *testing.T) {
	factory := NewErrorFactory("test-service")
	cause := errors.New("duplicate key")

	err := factory.ConflictError("create-user", "user", cause)

	if err == nil {
		t.Fatal("ConflictError returned nil")
	}

	if err.Code != ErrCodeConflict {
		t.Errorf("Expected code %s, got %s", ErrCodeConflict, err.Code)
	}

	expectedDetails := "Conflict with existing user"
	if err.Details != expectedDetails {
		t.Errorf("Expected details '%s', got '%s'", expectedDetails, err.Details)
	}

	if !err.UserError {
		t.Error("ConflictError should be marked as user error")
	}

	if err.HTTPStatusCode != 409 {
		t.Errorf("Expected HTTP status 409, got %d", err.HTTPStatusCode)
	}
}

// TestErrorFactory_New_NilCause tests creating an error with a nil cause
func TestErrorFactory_New_NilCause(t *testing.T) {
	factory := NewErrorFactory("test-service")

	err := factory.New(ErrCodeInternal, "test-operation", nil)

	if err == nil {
		t.Fatal("New returned nil")
	}

	if err.Cause != nil {
		t.Error("Cause should be nil when nil is passed")
	}
}

// TestErrorFactory_New_UnknownCode tests creating an error with an unknown code
func TestErrorFactory_New_UnknownCode(t *testing.T) {
	factory := NewErrorFactory("test-service")
	unknownCode := ErrorCode("UNKNOWN_ERROR")

	err := factory.New(unknownCode, "test-operation", nil)

	if err == nil {
		t.Fatal("New returned nil for unknown code")
	}

	// Verify that the default config is used
	if err.Code != unknownCode {
		t.Errorf("Expected code %s, got %s", unknownCode, err.Code)
	}

	if err.HTTPStatusCode != 500 {
		t.Errorf("Expected default HTTP status 500, got %d", err.HTTPStatusCode)
	}

	if err.Severity != SeverityMedium {
		t.Errorf("Expected default severity Medium, got %v", err.Severity)
	}
}

// TestErrorFactory_Wrap_AlreadyAppError tests wrapping an error that is already an AppError
func TestErrorFactory_Wrap_AlreadyAppError(t *testing.T) {
	factory := NewErrorFactory("test-service")
	originalErr := factory.New(ErrCodeDatabaseError, "db-query", errors.New("SQL error"))

	wrappedErr := factory.Wrap(originalErr, "wrap-operation")

	// It should return the same AppError instance
	if wrappedErr != originalErr {
		t.Error("Wrap should return the same AppError instance")
	}
}

// TestInferErrorCode tests the error code inference logic
func TestInferErrorCode(t *testing.T) {
	factory := NewErrorFactory("test-service")

	tests := []struct {
		name     string
		errMsg   string
		expected ErrorCode
	}{
		{"Timeout error", "operation timeout exceeded", ErrCodeTimeout},
		{"Deadline exceeded", "context deadline exceeded", ErrCodeTimeout},
		{"Connection refused", "connection refused", ErrCodeNetworkError},
		{"Network unreachable", "network unreachable", ErrCodeNetworkError},
		{"Not found", "resource not found", ErrCodeNotFound},
		{"Does not exist", "file does not exist", ErrCodeNotFound},
		{"Permission denied", "permission denied", ErrCodeForbidden},
		{"Access denied", "access denied", ErrCodeForbidden},
		{"Unauthorized", "unauthorized access", ErrCodeUnauthorized},
		{"Authentication", "authentication failed", ErrCodeUnauthorized},
		{"Invalid input", "invalid input parameter", ErrCodeInvalidInput},
		{"Malformed", "malformed request", ErrCodeInvalidInput},
		{"Bad request", "bad request", ErrCodeInvalidInput},
		{"Conflict", "version conflict", ErrCodeConflict},
		{"Already exists", "resource already exists", ErrCodeConflict},
		{"Database", "database connection failed", ErrCodeDatabaseError},
		{"SQL error", "SQL query error", ErrCodeDatabaseError},
		{"Rate limit", "rate limit exceeded", ErrCodeRateLimit},
		{"Too many requests", "too many requests", ErrCodeRateLimit},
		{"Unknown error", "something went wrong", ErrCodeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.errMsg)
			code := factory.inferErrorCode(err)
			if code != tt.expected {
				t.Errorf("inferErrorCode(%q) = %s, want %s", tt.errMsg, code, tt.expected)
			}
		})
	}
}

// TestContainsAny tests the substring-contains check
func TestContainsAny(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		keywords []string
		expected bool
	}{
		{"Contains first keyword", "hello world", []string{"hello", "bye"}, true},
		{"Contains second keyword", "hello world", []string{"bye", "world"}, true},
		{"Contains none", "hello world", []string{"foo", "bar"}, false},
		{"Empty string", "", []string{"test"}, false},
		{"Empty keywords", "hello world", []string{}, false},
		{"Case sensitive", "Hello World", []string{"hello", "world"}, false},
		{"Substring match", "testing", []string{"test"}, true},
		{"Exact match", "test", []string{"test"}, true},
		{"Keyword longer than string", "hi", []string{"hello"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsAny(tt.s, tt.keywords...)
			if result != tt.expected {
				t.Errorf("containsAny(%q, %v) = %v, want %v", tt.s, tt.keywords, result, tt.expected)
			}
		})
	}
}

// TestGlobalNewf tests the global Newf function
func TestGlobalNewf(t *testing.T) {
	cause := errors.New("original error")
	err := Newf(ErrCodeInternal, "test-op", cause, "Error: %s", "test")

	if err == nil {
		t.Fatal("Newf returned nil")
	}

	expectedMsg := "Error: test"
	if err.Message != expectedMsg {
		t.Errorf("Expected message '%s', got '%s'", expectedMsg, err.Message)
	}
}

// TestErrorFactory_New_WithAllFields tests creating an error that contains all fields
func TestErrorFactory_New_WithAllFields(t *testing.T) {
	factory := NewErrorFactory("test-service")
	factory.traceGenerator = &MockTraceGenerator{traceID: "fixed-trace-id"}

	cause := errors.New("cause error")
	err := factory.New(ErrCodeDatabaseError, "db-operation", cause)

	// Verify that all fields are set correctly
	if err.Code != ErrCodeDatabaseError {
		t.Error("Code not set correctly")
	}

	if err.Operation != "db-operation" {
		t.Error("Operation not set correctly")
	}

	if err.TraceID != "fixed-trace-id" {
		t.Error("TraceID not set correctly")
	}

	if err.Cause != cause {
		t.Error("Cause not set correctly")
	}

	if err.Context == nil {
		t.Error("Context should be initialized")
	}

	if err.Context["service"] != "test-service" {
		t.Error("Service name not in context")
	}

	if err.Timestamp.IsZero() {
		t.Error("Timestamp not set")
	}
}
