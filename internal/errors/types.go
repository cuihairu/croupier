package errors

import (
	"fmt"
	"time"
)

// ErrorCode 错误码类型
type ErrorCode string

// 预定义错误码
const (
	// 通用错误码 (1000-1999)
	ErrCodeInternal     ErrorCode = "INTERNAL_ERROR"
	ErrCodeInvalidInput ErrorCode = "INVALID_INPUT"
	ErrCodeNotFound     ErrorCode = "NOT_FOUND"
	ErrCodeUnauthorized ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden    ErrorCode = "FORBIDDEN"
	ErrCodeConflict     ErrorCode = "CONFLICT"
	ErrCodeTimeout      ErrorCode = "TIMEOUT"
	ErrCodeRateLimit    ErrorCode = "RATE_LIMIT"

	// 业务错误码 (2000-2999)
	ErrCodeGameNotFound      ErrorCode = "GAME_NOT_FOUND"
	ErrCodeGameAlreadyExist  ErrorCode = "GAME_ALREADY_EXIST"
	ErrCodeGameDisabled      ErrorCode = "GAME_DISABLED"
	ErrCodeInvalidGameStatus ErrorCode = "INVALID_GAME_STATUS"

	// 数据库错误码 (3000-3999)
	ErrCodeDatabaseError     ErrorCode = "DATABASE_ERROR"
	ErrCodeConnectionLost    ErrorCode = "CONNECTION_LOST"
	ErrCodeTransactionFailed ErrorCode = "TRANSACTION_FAILED"

	// 网络错误码 (4000-4999)
	ErrCodeNetworkError       ErrorCode = "NETWORK_ERROR"
	ErrCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
	ErrCodeConnectionTimeout  ErrorCode = "CONNECTION_TIMEOUT"

	// 认证授权错误码 (5000-5999)
	ErrCodeTokenExpired            ErrorCode = "TOKEN_EXPIRED"
	ErrCodeInvalidToken            ErrorCode = "INVALID_TOKEN"
	ErrCodeInvalidCredentials      ErrorCode = "INVALID_CREDENTIALS"
	ErrCodeInsufficientPermissions ErrorCode = "INSUFFICIENT_PERMISSIONS"
)

// ErrorSeverity 错误严重级别
type ErrorSeverity string

const (
	SeverityLow      ErrorSeverity = "low"      // 低：不影响主要功能
	SeverityMedium   ErrorSeverity = "medium"   // 中：影响部分功能
	SeverityHigh     ErrorSeverity = "high"     // 高：影响主要功能
	SeverityCritical ErrorSeverity = "critical" // 严重：系统不可用
)

// AppError 应用程序错误
type AppError struct {
	// 基本错误信息
	Code      ErrorCode     `json:"code"`
	Message   string        `json:"message"`
	Details   string        `json:"details,omitempty"`
	Operation string        `json:"operation"`
	Timestamp time.Time     `json:"timestamp"`
	TraceID   string        `json:"traceId"`
	Severity  ErrorSeverity `json:"severity"`

	// 错误分类
	Category  ErrorCategory `json:"category"`
	UserError bool          `json:"userError"` // 是否为用户输入错误

	// HTTP相关
	HTTPStatusCode int               `json:"httpStatusCode"`
	HTTPHeaders    map[string]string `json:"httpHeaders,omitempty"`

	// 重试信息
	Retryable  bool          `json:"retryable"`
	RetryDelay time.Duration `json:"retryDelay,omitempty"`
	RetryCount int           `json:"retryCount,omitempty"`

	// 上下文信息
	Context map[string]interface{} `json:"context,omitempty"`

	// 原始错误
	Cause error `json:"-"`
}

// ErrorCategory 错误分类
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

// Error 实现error接口
func (e *AppError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("[%s] %s: %s - %s", e.Code, e.Operation, e.Message, e.Details)
	}
	return fmt.Sprintf("[%s] %s: %s", e.Code, e.Operation, e.Message)
}

// Unwrap 返回原始错误
func (e *AppError) Unwrap() error {
	return e.Cause
}

// Is 检查错误类型
func (e *AppError) Is(target error) bool {
	if t, ok := target.(*AppError); ok {
		return e.Code == t.Code
	}
	return false
}

// WithContext 添加上下文信息
func (e *AppError) WithContext(key string, value interface{}) *AppError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithDetails 添加详细信息
func (e *AppError) WithDetails(details string) *AppError {
	e.Details = details
	return e
}

// WithRetry 设置重试信息
func (e *AppError) WithRetry(delay time.Duration, count int) *AppError {
	e.Retryable = true
	e.RetryDelay = delay
	e.RetryCount = count
	return e
}

// WithHTTPHeader 添加HTTP头
func (e *AppError) WithHTTPHeader(key, value string) *AppError {
	if e.HTTPHeaders == nil {
		e.HTTPHeaders = make(map[string]string)
	}
	e.HTTPHeaders[key] = value
	return e
}

// String 返回错误的字符串表示
func (e *AppError) String() string {
	return e.Error()
}

// ToJSON 返回JSON格式的错误信息
func (e *AppError) ToJSON() map[string]interface{} {
	result := map[string]interface{}{
		"code":        e.Code,
		"message":     e.Message,
		"operation":   e.Operation,
		"timestamp":   e.Timestamp,
		"trace_id":    e.TraceID,
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

// ErrorMetrics 错误指标
type ErrorMetrics struct {
	Code      ErrorCode     `json:"code"`
	Operation string        `json:"operation"`
	Count     int64         `json:"count"`
	FirstSeen time.Time     `json:"firstSeen"`
	LastSeen  time.Time     `json:"lastSeen"`
	Duration  time.Duration `json:"duration"`
	Severity  ErrorSeverity `json:"severity"`
}

// ErrorCollection 错误集合，用于批量错误处理
type ErrorCollection struct {
	Errors []*AppError `json:"errors"`
	Total  int         `json:"total"`
}

// Add 添加错误到集合
func (ec *ErrorCollection) Add(err *AppError) {
	ec.Errors = append(ec.Errors, err)
	ec.Total++
}

// HasErrors 检查是否有错误
func (ec *ErrorCollection) HasErrors() bool {
	return ec.Total > 0
}

// GetByCode 根据错误码获取错误
func (ec *ErrorCollection) GetByCode(code ErrorCode) []*AppError {
	var errors []*AppError
	for _, err := range ec.Errors {
		if err.Code == code {
			errors = append(errors, err)
		}
	}
	return errors
}

// GetBySeverity 根据严重级别获取错误
func (ec *ErrorCollection) GetBySeverity(severity ErrorSeverity) []*AppError {
	var errors []*AppError
	for _, err := range ec.Errors {
		if err.Severity == severity {
			errors = append(errors, err)
		}
	}
	return errors
}

// First 返回第一个错误
func (ec *ErrorCollection) First() *AppError {
	if ec.Total > 0 {
		return ec.Errors[0]
	}
	return nil
}

// Error 实现error接口
func (ec *ErrorCollection) Error() string {
	if ec.Total == 0 {
		return "no errors"
	}
	if ec.Total == 1 {
		return ec.Errors[0].Error()
	}
	return fmt.Sprintf("%d errors occurred, first: %s", ec.Total, ec.Errors[0].Error())
}

// Is 检查是否包含特定错误
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
