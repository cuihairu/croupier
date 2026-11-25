package errors

import (
	"testing"
)

// TestGetAllErrorConfigs tests getting all error configurations
func TestGetAllErrorConfigs(t *testing.T) {
	configs := GetAllErrorConfigs()

	if configs == nil {
		t.Fatal("GetAllErrorConfigs returned nil")
	}

	if len(configs) == 0 {
		t.Fatal("GetAllErrorConfigs returned empty map")
	}

	// Verify that some known error codes exist
	knownCodes := []ErrorCode{
		ErrCodeInternal,
		ErrCodeInvalidInput,
		ErrCodeNotFound,
		ErrCodeUnauthorized,
		ErrCodeForbidden,
		ErrCodeConflict,
		ErrCodeTimeout,
		ErrCodeRateLimit,
		ErrCodeDatabaseError,
		ErrCodeNetworkError,
		ErrCodeServiceUnavailable,
		ErrCodeGameNotFound,
		ErrCodeGameAlreadyExist,
		ErrCodeGameDisabled,
	}

	for _, code := range knownCodes {
		if _, exists := configs[code]; !exists {
			t.Errorf("Expected error code %s to exist in configs", code)
		}
	}
}

// TestAddCustomErrorConfig tests adding a custom error configuration
func TestAddCustomErrorConfig(t *testing.T) {
	customCode := ErrorCode("CUSTOM_ERROR")
	customConfig := ErrorConfig{
		Code:           customCode,
		Message:        "Custom error message",
		HTTPStatusCode: 418,
		Severity:       SeverityLow,
		Category:       CategorySystem,
		UserError:      true,
		Retryable:      false,
		DefaultDetails: "Custom error details",
	}

	// Add the custom configuration
	AddCustomErrorConfig(customConfig)

	// Verify that the configuration was added
	config, err := GetErrorConfig(customCode)
	if err != nil {
		t.Fatalf("Failed to get custom error config: %v", err)
	}

	if config.Message != customConfig.Message {
		t.Errorf("Expected message %s, got %s", customConfig.Message, config.Message)
	}

	if config.HTTPStatusCode != customConfig.HTTPStatusCode {
		t.Errorf("Expected HTTP status code %d, got %d", customConfig.HTTPStatusCode, config.HTTPStatusCode)
	}

	if config.Severity != customConfig.Severity {
		t.Errorf("Expected severity %v, got %v", customConfig.Severity, config.Severity)
	}

	if config.UserError != customConfig.UserError {
		t.Errorf("Expected UserError %v, got %v", customConfig.UserError, config.UserError)
	}

	if config.Retryable != customConfig.Retryable {
		t.Errorf("Expected Retryable %v, got %v", customConfig.Retryable, config.Retryable)
	}
}

// TestIsUserError tests determining whether this is a user error
func TestIsUserError(t *testing.T) {
	tests := []struct {
		name     string
		code     ErrorCode
		expected bool
	}{
		{"InvalidInput is user error", ErrCodeInvalidInput, true},
		{"NotFound is user error", ErrCodeNotFound, true},
		{"Unauthorized is user error", ErrCodeUnauthorized, true},
		{"Forbidden is user error", ErrCodeForbidden, true},
		{"Conflict is user error", ErrCodeConflict, true},
		{"Internal is not user error", ErrCodeInternal, false},
		{"DatabaseError is not user error", ErrCodeDatabaseError, false},
		{"NetworkError is not user error", ErrCodeNetworkError, false},
		{"Unknown code is not user error", ErrorCode("UNKNOWN"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsUserError(tt.code)
			if result != tt.expected {
				t.Errorf("IsUserError(%s) = %v, want %v", tt.code, result, tt.expected)
			}
		})
	}
}

// TestIsRetryable tests determining whether the error is retryable
func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		code     ErrorCode
		expected bool
	}{
		{"Timeout is retryable", ErrCodeTimeout, true},
		{"NetworkError is retryable", ErrCodeNetworkError, true},
		{"RateLimit is retryable", ErrCodeRateLimit, true},
		{"DatabaseError is retryable", ErrCodeDatabaseError, true},
		{"ServiceUnavailable is retryable", ErrCodeServiceUnavailable, true},
		{"Internal is retryable", ErrCodeInternal, true},
		{"InvalidInput is not retryable", ErrCodeInvalidInput, false},
		{"NotFound is not retryable", ErrCodeNotFound, false},
		{"Unauthorized is not retryable", ErrCodeUnauthorized, false},
		{"Unknown code is not retryable", ErrorCode("UNKNOWN"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryable(tt.code)
			if result != tt.expected {
				t.Errorf("IsRetryable(%s) = %v, want %v", tt.code, result, tt.expected)
			}
		})
	}
}

// TestGetSeverity tests getting the error severity
func TestGetSeverity(t *testing.T) {
	tests := []struct {
		name     string
		code     ErrorCode
		expected ErrorSeverity
	}{
		{"Internal has high severity", ErrCodeInternal, SeverityHigh},
		{"DatabaseError has high severity", ErrCodeDatabaseError, SeverityHigh},
		{"NetworkError has high severity", ErrCodeNetworkError, SeverityHigh},
		{"ServiceUnavailable has high severity", ErrCodeServiceUnavailable, SeverityHigh},
		{"Timeout has medium severity", ErrCodeTimeout, SeverityMedium},
		{"RateLimit has medium severity", ErrCodeRateLimit, SeverityMedium},
		{"Unauthorized has medium severity", ErrCodeUnauthorized, SeverityMedium},
		{"InvalidInput has low severity", ErrCodeInvalidInput, SeverityLow},
		{"NotFound has low severity", ErrCodeNotFound, SeverityLow},
		{"Unknown code has medium severity", ErrorCode("UNKNOWN"), SeverityMedium},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetSeverity(tt.code)
			if result != tt.expected {
				t.Errorf("GetSeverity(%s) = %v, want %v", tt.code, result, tt.expected)
			}
		})
	}
}

// TestGetHTTPStatusCode_UnknownCode tests that an unknown error code returns the default status code
func TestGetHTTPStatusCode_UnknownCode(t *testing.T) {
	unknownCode := ErrorCode("UNKNOWN_ERROR")
	statusCode := GetHTTPStatusCode(unknownCode)

	if statusCode != 500 {
		t.Errorf("Expected status code 500 for unknown error, got %d", statusCode)
	}
}

// TestGetErrorConfig_UnknownCode tests getting the configuration for an unknown error
func TestGetErrorConfig_UnknownCode(t *testing.T) {
	unknownCode := ErrorCode("UNKNOWN_ERROR")

	_, err := GetErrorConfig(unknownCode)
	if err == nil {
		t.Error("Expected error for unknown code, got nil")
	}
}
