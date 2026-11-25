package errors

import (
	"fmt"
	"time"
)

// ErrorCode error code type
type ErrorCode string

// Predefined error codes
const (
	// General error codes (1000-1999)
	ErrCodeInternal     ErrorCode = "INTERNAL_ERROR"
	ErrCodeInvalidInput ErrorCode = "INVALID_INPUT"
	ErrCodeNotFound     ErrorCode = "NOT_FOUND"
	ErrCodeUnauthorized ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden    ErrorCode = "FORBIDDEN"
	ErrCodeConflict     ErrorCode = "CONFLICT"
	ErrCodeTimeout      ErrorCode = "TIMEOUT"
	ErrCodeRateLimit    ErrorCode = "RATE_LIMIT"

	// Business error codes (2000-2999)
	ErrCodeGameNotFound      ErrorCode = "GAME_NOT_FOUND"
	ErrCodeGameAlreadyExist  ErrorCode = "GAME_ALREADY_EXIST"
	ErrCodeGameDisabled      ErrorCode = "GAME_DISABLED"
	ErrCodeInvalidGameStatus ErrorCode = "INVALID_GAME_STATUS"
	// ErrCodeFunctionVersionBelowMinimum execution-side explainability (E9): the function
	// descriptor version is below the function-level minimum version floor, so it is
	// intercepted at the registration surface (not materialized) and dispatch has no
	// candidates. Distinguished by code from "the agent genuinely has no live session"
	// (SERVICE_UNAVAILABLE), to avoid misjudging a version-floor interception as an
	// agent connectivity failure.
	ErrCodeFunctionVersionBelowMinimum ErrorCode = "FUNCTION_VERSION_BELOW_MINIMUM"

	// Database error codes (3000-3999)
	ErrCodeDatabaseError     ErrorCode = "DATABASE_ERROR"
	ErrCodeConnectionLost    ErrorCode = "CONNECTION_LOST"
	ErrCodeTransactionFailed ErrorCode = "TRANSACTION_FAILED"

	// Network error codes (4000-4999)
	ErrCodeNetworkError       ErrorCode = "NETWORK_ERROR"
	ErrCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
	ErrCodeConnectionTimeout  ErrorCode = "CONNECTION_TIMEOUT"

	// Authentication/authorization error codes (5000-5999)
	ErrCodeTokenExpired            ErrorCode = "TOKEN_EXPIRED"
	ErrCodeInvalidToken            ErrorCode = "INVALID_TOKEN"
	ErrCodeInvalidCredentials      ErrorCode = "INVALID_CREDENTIALS"
	ErrCodeInsufficientPermissions ErrorCode = "INSUFFICIENT_PERMISSIONS"
)

// ErrorSeverity error severity
type ErrorSeverity string

const (
	SeverityLow      ErrorSeverity = "low"      // low: does not affect core functionality
	SeverityMedium   ErrorSeverity = "medium"   // medium: affects some functionality
	SeverityHigh     ErrorSeverity = "high"     // high: affects core functionality
	SeverityCritical ErrorSeverity = "critical" // critical: system unavailable
)

// AppError application error
type AppError struct {
	// Basic error info
	Code      ErrorCode     `json:"code"`
	Message   string        `json:"message"`
	Details   string        `json:"details,omitempty"`
	Operation string        `json:"operation"`
	Timestamp time.Time     `json:"timestamp"`
	TraceID   string        `json:"traceId"`
	Severity  ErrorSeverity `json:"severity"`

	// Error category
	Category  ErrorCategory `json:"category"`
	UserError bool          `json:"userError"` // whether this is a user-input error

	// HTTP-related
	HTTPStatusCode int               `json:"httpStatusCode"`
	HTTPHeaders    map[string]string `json:"httpHeaders,omitempty"`

	// Retry info
	Retryable  bool          `json:"retryable"`
	RetryDelay time.Duration `json:"retryDelay,omitempty"`
	RetryCount int           `json:"retryCount,omitempty"`

	// Context info
	Context map[string]interface{} `json:"context,omitempty"`

	// Original error
	Cause error `json:"-"`
}

// ErrorCategory error category
type ErrorCategory string

const (
	CategoryValidation ErrorCategory = "validation"
	CategoryBusiness   ErrorCategory = "business"
	CategoryDatabase   ErrorCategory = "database"
	CategoryNetwork    ErrorCategory = "network"
	CategoryAuth       ErrorCategory = "auth"
	CategoryPermission ErrorCategory = "permission"
	CategoryExternal   ErrorCategory = "external"
	CategorySystem     ErrorCategory = "system"
)

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("[%s] %s: %s - %s", e.Code, e.Operation, e.Message, e.Details)
	}
	return fmt.Sprintf("[%s] %s: %s", e.Code, e.Operation, e.Message)
}

// Unwrap returns the original error
func (e *AppError) Unwrap() error {
	return e.Cause
}

// Is checks the error type
func (e *AppError) Is(target error) bool {
	if t, ok := target.(*AppError); ok {
		return e.Code == t.Code
	}
	return false
}

// WithContext adds context info
func (e *AppError) WithContext(key string, value interface{}) *AppError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithDetails adds details
func (e *AppError) WithDetails(details string) *AppError {
	e.Details = details
	return e
}

// WithRetry sets retry info
func (e *AppError) WithRetry(delay time.Duration, count int) *AppError {
	e.Retryable = true
	e.RetryDelay = delay
	e.RetryCount = count
	return e
}

// WithHTTPHeader adds an HTTP header
func (e *AppError) WithHTTPHeader(key, value string) *AppError {
	if e.HTTPHeaders == nil {
		e.HTTPHeaders = make(map[string]string)
	}
	e.HTTPHeaders[key] = value
	return e
}

// String returns the string representation of the error
func (e *AppError) String() string {
	return e.Error()
}

// ToJSON returns the error info as JSON
func (e *AppError) ToJSON() map[string]interface{} {
	result := map[string]interface{}{
		"code":        e.Code,
		"message":     e.Message,
		"operation":   e.Operation,
		"timestamp":   e.Timestamp,
		"traceId":     e.TraceID,
		"severity":    e.Severity,
		"category":    e.Category,
		"user_error":  e.UserError,
		"http_status": e.HTTPStatusCode,
		"retryable":   e.Retryable,
	}

	if e.Details != "" {
		result["details"] = e.Details
	}

	if e.HTTPHeaders != nil {
		result["http_headers"] = e.HTTPHeaders
	}

	if e.Retryable && e.RetryDelay > 0 {
		result["retry_delay"] = e.RetryDelay.String()
	}

	if e.RetryCount > 0 {
		result["retry_count"] = e.RetryCount
	}

	if e.Context != nil {
		result["context"] = e.Context
	}

	return result
}

// ErrorMetrics error metrics
type ErrorMetrics struct {
	Code      ErrorCode     `json:"code"`
	Operation string        `json:"operation"`
	Count     int64         `json:"count"`
	FirstSeen time.Time     `json:"firstSeen"`
	LastSeen  time.Time     `json:"lastSeen"`
	Duration  time.Duration `json:"duration"`
	Severity  ErrorSeverity `json:"severity"`
}

// ErrorCollection error collection, used for batch error handling
type ErrorCollection struct {
	Errors []*AppError `json:"errors"`
	Total  int         `json:"total"`
}

// Add adds an error to the collection
func (ec *ErrorCollection) Add(err *AppError) {
	ec.Errors = append(ec.Errors, err)
	ec.Total++
}

// HasErrors checks whether any errors exist
func (ec *ErrorCollection) HasErrors() bool {
	return ec.Total > 0
}

// GetByCode gets errors by error code
func (ec *ErrorCollection) GetByCode(code ErrorCode) []*AppError {
	var errors []*AppError
	for _, err := range ec.Errors {
		if err.Code == code {
			errors = append(errors, err)
		}
	}
	return errors
}

// GetBySeverity gets errors by severity
func (ec *ErrorCollection) GetBySeverity(severity ErrorSeverity) []*AppError {
	var errors []*AppError
	for _, err := range ec.Errors {
		if err.Severity == severity {
			errors = append(errors, err)
		}
	}
	return errors
}

// First returns the first error
func (ec *ErrorCollection) First() *AppError {
	if ec.Total > 0 {
		return ec.Errors[0]
	}
	return nil
}

// Error implements the error interface
func (ec *ErrorCollection) Error() string {
	if ec.Total == 0 {
		return "no errors"
	}
	if ec.Total == 1 {
		return ec.Errors[0].Error()
	}
	return fmt.Sprintf("%d errors occurred, first: %s", ec.Total, ec.Errors[0].Error())
}

// Is checks whether a specific error is contained
func (ec *ErrorCollection) Is(target error) bool {
	if targetAppErr, ok := target.(*AppError); ok {
		for _, err := range ec.Errors {
			if err.Code == targetAppErr.Code {
				return true
			}
		}
	}
	return false
}
