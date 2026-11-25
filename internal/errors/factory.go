package errors

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ErrorFactory error factory
type ErrorFactory struct {
	serviceName    string
	defaultTimeout time.Duration
	traceGenerator TraceGenerator
}

// TraceGenerator trace ID generator interface
type TraceGenerator interface {
	GenerateTraceID() string
}

// DefaultTraceGenerator default trace ID generator
type DefaultTraceGenerator struct{}

func (d *DefaultTraceGenerator) GenerateTraceID() string {
	return uuid.New().String()
}

// NewErrorFactory creates an error factory
func NewErrorFactory(serviceName string) *ErrorFactory {
	return &ErrorFactory{
		serviceName:    serviceName,
		defaultTimeout: 30 * time.Second,
		traceGenerator: &DefaultTraceGenerator{},
	}
}

// WithTraceGenerator sets the trace ID generator
func (f *ErrorFactory) WithTraceGenerator(generator TraceGenerator) *ErrorFactory {
	f.traceGenerator = generator
	return f
}

// New creates a new application error
func (f *ErrorFactory) New(code ErrorCode, operation string, cause error) *AppError {
	config, err := GetErrorConfig(code)
	if err != nil {
		// If no config is found, use the default config
		config = ErrorConfig{
			Code:           code,
			Message:        "Unknown error",
			HTTPStatusCode: http.StatusInternalServerError,
			Severity:       SeverityMedium,
			Category:       CategorySystem,
			UserError:      false,
			Retryable:      false,
			DefaultDetails: "An unknown error occurred",
		}
	}

	return &AppError{
		Code:           config.Code,
		Message:        config.Message,
		Details:        config.DefaultDetails,
		Operation:      operation,
		Timestamp:      time.Now(),
		TraceID:        f.traceGenerator.GenerateTraceID(),
		Severity:       config.Severity,
		Category:       config.Category,
		UserError:      config.UserError,
		HTTPStatusCode: config.HTTPStatusCode,
		Retryable:      config.Retryable,
		Cause:          cause,
		Context: map[string]interface{}{
			"service": f.serviceName,
		},
	}
}

// Newf creates an application error with a formatted message
func (f *ErrorFactory) Newf(code ErrorCode, operation string, cause error, format string, args ...interface{}) *AppError {
	err := f.New(code, operation, cause)
	err.Message = fmt.Sprintf(format, args...)
	return err
}

// NewWithDetails creates an application error with details
func (f *ErrorFactory) NewWithDetails(code ErrorCode, operation string, details string, cause error) *AppError {
	err := f.New(code, operation, cause)
	err.Details = details
	return err
}

// Wrap wraps an existing error
func (f *ErrorFactory) Wrap(cause error, operation string) *AppError {
	if cause == nil {
		return nil
	}

	// If it is already an AppError, return it directly
	if appErr, ok := cause.(*AppError); ok {
		return appErr
	}

	// Infer the error code from the error type
	code := f.inferErrorCode(cause)
	return f.New(code, operation, cause)
}

// Wrapf wraps an existing error and adds a formatted message
func (f *ErrorFactory) Wrapf(cause error, operation string, format string, args ...interface{}) *AppError {
	err := f.Wrap(cause, operation)
	if err != nil {
		err.Message = fmt.Sprintf(format, args...)
	}
	return err
}

// InternalError creates an internal error
func (f *ErrorFactory) InternalError(operation string, cause error) *AppError {
	return f.New(ErrCodeInternal, operation, cause)
}

// InternalErrorf creates an internal error with a formatted message
func (f *ErrorFactory) InternalErrorf(operation string, cause error, format string, args ...interface{}) *AppError {
	return f.Newf(ErrCodeInternal, operation, cause, format, args...)
}

// InvalidInputError creates an input validation error
func (f *ErrorFactory) InvalidInputError(operation string, field, value string, cause error) *AppError {
	details := fmt.Sprintf("Invalid value for field '%s': %s", field, value)
	return f.NewWithDetails(ErrCodeInvalidInput, operation, details, cause)
}

// NotFoundError creates a resource-not-found error
func (f *ErrorFactory) NotFoundError(operation, resourceType, resourceID string) *AppError {
	details := fmt.Sprintf("%s with ID '%s' not found", resourceType, resourceID)
	return f.NewWithDetails(ErrCodeNotFound, operation, details, nil)
}

// UnauthorizedError creates an unauthorized error
func (f *ErrorFactory) UnauthorizedError(operation string, cause error) *AppError {
	return f.New(ErrCodeUnauthorized, operation, cause)
}

// ForbiddenError creates a forbidden-access error
func (f *ErrorFactory) ForbiddenError(operation string, resource string, cause error) *AppError {
	details := fmt.Sprintf("Access to '%s' is forbidden", resource)
	return f.NewWithDetails(ErrCodeForbidden, operation, details, cause)
}

// ConflictError creates a conflict error
func (f *ErrorFactory) ConflictError(operation string, resource string, cause error) *AppError {
	details := fmt.Sprintf("Conflict with existing %s", resource)
	return f.NewWithDetails(ErrCodeConflict, operation, details, cause)
}

// TimeoutError creates a timeout error
func (f *ErrorFactory) TimeoutError(operation string, timeout time.Duration, cause error) *AppError {
	details := fmt.Sprintf("Operation timed out after %v", timeout)
	return f.NewWithDetails(ErrCodeTimeout, operation, details, cause).
		WithRetry(timeout, 1) // suggest one retry
}

// RateLimitError creates a rate limit error
func (f *ErrorFactory) RateLimitError(operation string, limit int, window time.Duration) *AppError {
	details := fmt.Sprintf("Rate limit exceeded: %d requests per %v", limit, window)
	return f.NewWithDetails(ErrCodeRateLimit, operation, details, nil).
		WithHTTPHeader("Retry-After", window.String()).
		WithRetry(window, 1)
}

// DatabaseError creates a database error
func (f *ErrorFactory) DatabaseError(operation string, query string, cause error) *AppError {
	details := fmt.Sprintf("Database error in query: %s", query)
	return f.NewWithDetails(ErrCodeDatabaseError, operation, details, cause).
		WithRetry(1*time.Second, 3) // suggest 3 retries, 1 second apart
}

// NetworkError creates a network error
func (f *ErrorFactory) NetworkError(operation string, endpoint string, cause error) *AppError {
	details := fmt.Sprintf("Network error when calling %s", endpoint)
	return f.NewWithDetails(ErrCodeNetworkError, operation, details, cause).
		WithRetry(2*time.Second, 2) // suggest 2 retries, 2 seconds apart
}

// ServiceUnavailableError creates a service-unavailable error
func (f *ErrorFactory) ServiceUnavailableError(operation, service string, cause error) *AppError {
	details := fmt.Sprintf("Service '%s' is currently unavailable", service)
	return f.NewWithDetails(ErrCodeServiceUnavailable, operation, details, cause).
		WithRetry(5*time.Second, 3) // suggest 3 retries, 5 seconds apart
}

// GameNotFoundError creates a game-not-found error
func (f *ErrorFactory) GameNotFoundError(operation, gameID string) *AppError {
	details := fmt.Sprintf("Game with ID '%s' not found", gameID)
	return f.NewWithDetails(ErrCodeGameNotFound, operation, details, nil)
}

// GameAlreadyExistError creates a game-already-exists error
func (f *ErrorFactory) GameAlreadyExistError(operation, gameName string) *AppError {
	details := fmt.Sprintf("Game with name '%s' already exists", gameName)
	return f.NewWithDetails(ErrCodeGameAlreadyExist, operation, details, nil)
}

// GameDisabledError creates a game-disabled error
func (f *ErrorFactory) GameDisabledError(operation, gameID string) *AppError {
	details := fmt.Sprintf("Game '%s' is currently disabled", gameID)
	return f.NewWithDetails(ErrCodeGameDisabled, operation, details, nil)
}

// inferErrorCode infers the error code from the error type
func (f *ErrorFactory) inferErrorCode(err error) ErrorCode {
	errStr := err.Error()

	// Infer the error type from the error string
	switch {
	case containsAny(errStr, "timeout", "deadline exceeded"):
		return ErrCodeTimeout
	case containsAny(errStr, "connection refused", "network unreachable"):
		return ErrCodeNetworkError
	case containsAny(errStr, "not found", "does not exist"):
		return ErrCodeNotFound
	case containsAny(errStr, "permission denied", "access denied"):
		return ErrCodeForbidden
	case containsAny(errStr, "unauthorized", "authentication"):
		return ErrCodeUnauthorized
	case containsAny(errStr, "invalid input", "malformed", "bad request"):
		return ErrCodeInvalidInput
	case containsAny(errStr, "conflict", "already exists"):
		return ErrCodeConflict
	case containsAny(errStr, "database", "sql", "query"):
		return ErrCodeDatabaseError
	case containsAny(errStr, "rate limit", "too many requests"):
		return ErrCodeRateLimit
	default:
		return ErrCodeInternal
	}
}

// containsAny checks whether the string contains any one of the keywords
func containsAny(s string, keywords ...string) bool {
	for _, keyword := range keywords {
		if len(s) >= len(keyword) {
			for i := 0; i <= len(s)-len(keyword); i++ {
				if s[i:i+len(keyword)] == keyword {
					return true
				}
			}
		}
	}
	return false
}

// Global default error factory
var DefaultFactory = NewErrorFactory("croupier")

// Global convenience functions
func New(code ErrorCode, operation string, cause error) *AppError {
	return DefaultFactory.New(code, operation, cause)
}

func Newf(code ErrorCode, operation string, cause error, format string, args ...interface{}) *AppError {
	return DefaultFactory.Newf(code, operation, cause, format, args...)
}

func Wrap(cause error, operation string) *AppError {
	return DefaultFactory.Wrap(cause, operation)
}

func InternalError(operation string, cause error) *AppError {
	return DefaultFactory.InternalError(operation, cause)
}

func InvalidInputError(operation string, field, value string, cause error) *AppError {
	return DefaultFactory.InvalidInputError(operation, field, value, cause)
}

func NotFoundError(operation, resourceType, resourceID string) *AppError {
	return DefaultFactory.NotFoundError(operation, resourceType, resourceID)
}

func GameNotFoundError(operation, gameID string) *AppError {
	return DefaultFactory.GameNotFoundError(operation, gameID)
}
