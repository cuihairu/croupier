package errors

import (
	"fmt"
	"net/http"
)

// ErrorConfig error configuration
type ErrorConfig struct {
	Code           ErrorCode
	Message        string
	HTTPStatusCode int
	Severity       ErrorSeverity
	Category       ErrorCategory
	UserError      bool
	Retryable      bool
	DefaultDetails string
}

// Error code configuration map
var errorConfigs = map[ErrorCode]ErrorConfig{
	// General errors
	ErrCodeInternal: {
		Code:           ErrCodeInternal,
		Message:        "Internal server error",
		HTTPStatusCode: http.StatusInternalServerError,
		Severity:       SeverityHigh,
		Category:       CategorySystem,
		UserError:      false,
		Retryable:      true,
		DefaultDetails: "An unexpected error occurred while processing your request",
	},
	ErrCodeInvalidInput: {
		Code:           ErrCodeInvalidInput,
		Message:        "Invalid input provided",
		HTTPStatusCode: http.StatusBadRequest,
		Severity:       SeverityLow,
		Category:       CategoryValidation,
		UserError:      true,
		Retryable:      false,
		DefaultDetails: "The provided input data is invalid or malformed",
	},
	ErrCodeNotFound: {
		Code:           ErrCodeNotFound,
		Message:        "Resource not found",
		HTTPStatusCode: http.StatusNotFound,
		Severity:       SeverityLow,
		Category:       CategoryBusiness,
		UserError:      true,
		Retryable:      false,
		DefaultDetails: "The requested resource could not be found",
	},
	ErrCodeUnauthorized: {
		Code:           ErrCodeUnauthorized,
		Message:        "Authentication required",
		HTTPStatusCode: http.StatusUnauthorized,
		Severity:       SeverityMedium,
		Category:       CategoryAuth,
		UserError:      true,
		Retryable:      false,
		DefaultDetails: "Authentication is required to access this resource",
	},
	ErrCodeForbidden: {
		Code:           ErrCodeForbidden,
		Message:        "Access denied",
		HTTPStatusCode: http.StatusForbidden,
		Severity:       SeverityMedium,
		Category:       CategoryPermission,
		UserError:      true,
		Retryable:      false,
		DefaultDetails: "You do not have permission to access this resource",
	},
	ErrCodeConflict: {
		Code:           ErrCodeConflict,
		Message:        "Resource conflict",
		HTTPStatusCode: http.StatusConflict,
		Severity:       SeverityMedium,
		Category:       CategoryBusiness,
		UserError:      true,
		Retryable:      false,
		DefaultDetails: "The request conflicts with the current state of the resource",
	},
	ErrCodeTimeout: {
		Code:           ErrCodeTimeout,
		Message:        "Request timeout",
		HTTPStatusCode: http.StatusRequestTimeout,
		Severity:       SeverityMedium,
		Category:       CategorySystem,
		UserError:      false,
		Retryable:      true,
		DefaultDetails: "The request timed out while processing",
	},
	ErrCodeRateLimit: {
		Code:           ErrCodeRateLimit,
		Message:        "Rate limit exceeded",
		HTTPStatusCode: http.StatusTooManyRequests,
		Severity:       SeverityMedium,
		Category:       CategorySystem,
		UserError:      true,
		Retryable:      true,
		DefaultDetails: "Too many requests, please try again later",
	},

	// Business errors
	ErrCodeGameNotFound: {
		Code:           ErrCodeGameNotFound,
		Message:        "Game not found",
		HTTPStatusCode: http.StatusNotFound,
		Severity:       SeverityLow,
		Category:       CategoryBusiness,
		UserError:      true,
		Retryable:      false,
		DefaultDetails: "The specified game does not exist or has been deleted",
	},
	ErrCodeGameAlreadyExist: {
		Code:           ErrCodeGameAlreadyExist,
		Message:        "Game already exists",
		HTTPStatusCode: http.StatusConflict,
		Severity:       SeverityMedium,
		Category:       CategoryBusiness,
		UserError:      true,
		Retryable:      false,
		DefaultDetails: "A game with the same name already exists",
	},
	ErrCodeGameDisabled: {
		Code:           ErrCodeGameDisabled,
		Message:        "Game is disabled",
		HTTPStatusCode: http.StatusForbidden,
		Severity:       SeverityMedium,
		Category:       CategoryBusiness,
		UserError:      true,
		Retryable:      false,
		DefaultDetails: "The specified game is currently disabled",
	},
	ErrCodeInvalidGameStatus: {
		Code:           ErrCodeInvalidGameStatus,
		Message:        "Invalid game status",
		HTTPStatusCode: http.StatusBadRequest,
		Severity:       SeverityMedium,
		Category:       CategoryBusiness,
		UserError:      true,
		Retryable:      false,
		DefaultDetails: "The game status transition is not valid",
	},
	ErrCodeFunctionVersionBelowMinimum: {
		Code:           ErrCodeFunctionVersionBelowMinimum,
		Message:        "Function version below configured minimum",
		HTTPStatusCode: http.StatusServiceUnavailable,
		Severity:       SeverityHigh,
		Category:       CategoryBusiness,
		UserError:      false,
		Retryable:      true,
		DefaultDetails: "The function version registered by the game server is below the configured version floor, so it was not materialized and no agent can serve it",
	},

	// Database errors
	ErrCodeDatabaseError: {
		Code:           ErrCodeDatabaseError,
		Message:        "Database operation failed",
		HTTPStatusCode: http.StatusInternalServerError,
		Severity:       SeverityHigh,
		Category:       CategoryDatabase,
		UserError:      false,
		Retryable:      true,
		DefaultDetails: "A database operation failed while processing your request",
	},
	ErrCodeConnectionLost: {
		Code:           ErrCodeConnectionLost,
		Message:        "Database connection lost",
		HTTPStatusCode: http.StatusServiceUnavailable,
		Severity:       SeverityHigh,
		Category:       CategoryDatabase,
		UserError:      false,
		Retryable:      true,
		DefaultDetails: "The connection to the database has been lost",
	},
	ErrCodeTransactionFailed: {
		Code:           ErrCodeTransactionFailed,
		Message:        "Transaction failed",
		HTTPStatusCode: http.StatusInternalServerError,
		Severity:       SeverityHigh,
		Category:       CategoryDatabase,
		UserError:      false,
		Retryable:      true,
		DefaultDetails: "The database transaction could not be completed",
	},

	// Network errors
	ErrCodeNetworkError: {
		Code:           ErrCodeNetworkError,
		Message:        "Network error",
		HTTPStatusCode: http.StatusBadGateway,
		Severity:       SeverityHigh,
		Category:       CategoryNetwork,
		UserError:      false,
		Retryable:      true,
		DefaultDetails: "A network error occurred while processing your request",
	},
	ErrCodeServiceUnavailable: {
		Code:           ErrCodeServiceUnavailable,
		Message:        "Service unavailable",
		HTTPStatusCode: http.StatusServiceUnavailable,
		Severity:       SeverityHigh,
		Category:       CategoryExternal,
		UserError:      false,
		Retryable:      true,
		DefaultDetails: "The required service is currently unavailable",
	},
	ErrCodeConnectionTimeout: {
		Code:           ErrCodeConnectionTimeout,
		Message:        "Connection timeout",
		HTTPStatusCode: http.StatusGatewayTimeout,
		Severity:       SeverityHigh,
		Category:       CategoryNetwork,
		UserError:      false,
		Retryable:      true,
		DefaultDetails: "The connection to the external service timed out",
	},

	// Authentication/authorization errors
	ErrCodeTokenExpired: {
		Code:           ErrCodeTokenExpired,
		Message:        "Token has expired",
		HTTPStatusCode: http.StatusUnauthorized,
		Severity:       SeverityMedium,
		Category:       CategoryAuth,
		UserError:      true,
		Retryable:      false,
		DefaultDetails: "Your authentication token has expired, please log in again",
	},
	ErrCodeInvalidToken: {
		Code:           ErrCodeInvalidToken,
		Message:        "Invalid token",
		HTTPStatusCode: http.StatusUnauthorized,
		Severity:       SeverityMedium,
		Category:       CategoryAuth,
		UserError:      true,
		Retryable:      false,
		DefaultDetails: "The provided authentication token is invalid",
	},
	ErrCodeInvalidCredentials: {
		Code:           ErrCodeInvalidCredentials,
		Message:        "Invalid credentials",
		HTTPStatusCode: http.StatusUnauthorized,
		Severity:       SeverityMedium,
		Category:       CategoryAuth,
		UserError:      true,
		Retryable:      false,
		DefaultDetails: "The provided username or password is incorrect",
	},
	ErrCodeInsufficientPermissions: {
		Code:           ErrCodeInsufficientPermissions,
		Message:        "Insufficient permissions",
		HTTPStatusCode: http.StatusForbidden,
		Severity:       SeverityMedium,
		Category:       CategoryPermission,
		UserError:      true,
		Retryable:      false,
		DefaultDetails: "You do not have sufficient permissions to perform this action",
	},
}

// GetErrorConfig gets the error configuration
func GetErrorConfig(code ErrorCode) (ErrorConfig, error) {
	config, exists := errorConfigs[code]
	if !exists {
		return ErrorConfig{}, fmt.Errorf("unknown error code: %s", code)
	}
	return config, nil
}

// GetAllErrorConfigs gets all error configurations
func GetAllErrorConfigs() map[ErrorCode]ErrorConfig {
	return errorConfigs
}

// AddCustomErrorConfig adds a custom error configuration
func AddCustomErrorConfig(config ErrorConfig) {
	errorConfigs[config.Code] = config
}

// GetHTTPStatusCode gets the HTTP status code from the error code
func GetHTTPStatusCode(code ErrorCode) int {
	if config, exists := errorConfigs[code]; exists {
		return config.HTTPStatusCode
	}
	return http.StatusInternalServerError
}

// GetUserError reports whether this is a user error
func IsUserError(code ErrorCode) bool {
	if config, exists := errorConfigs[code]; exists {
		return config.UserError
	}
	return false
}

// GetRetryable reports whether the error is retryable
func IsRetryable(code ErrorCode) bool {
	if config, exists := errorConfigs[code]; exists {
		return config.Retryable
	}
	return false
}

// GetSeverity gets the error severity
func GetSeverity(code ErrorCode) ErrorSeverity {
	if config, exists := errorConfigs[code]; exists {
		return config.Severity
	}
	return SeverityMedium
}
