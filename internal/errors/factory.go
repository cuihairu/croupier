package errors

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ErrorFactory 错误工厂
type ErrorFactory struct {
	serviceName    string
	defaultTimeout time.Duration
	traceGenerator TraceGenerator
}

// TraceGenerator 追踪ID生成器接口
type TraceGenerator interface {
	GenerateTraceID() string
}

// DefaultTraceGenerator 默认追踪ID生成器
type DefaultTraceGenerator struct{}

func (d *DefaultTraceGenerator) GenerateTraceID() string {
	return uuid.New().String()
}

// NewErrorFactory 创建错误工厂
func NewErrorFactory(serviceName string) *ErrorFactory {
	return &ErrorFactory{
		serviceName:    serviceName,
		defaultTimeout: 30 * time.Second,
		traceGenerator: &DefaultTraceGenerator{},
	}
}

// WithTraceGenerator 设置追踪ID生成器
func (f *ErrorFactory) WithTraceGenerator(generator TraceGenerator) *ErrorFactory {
	f.traceGenerator = generator
	return f
}

// New 创建新的应用错误
func (f *ErrorFactory) New(code ErrorCode, operation string, cause error) *AppError {
	config, err := GetErrorConfig(code)
	if err != nil {
		// 如果找不到配置，使用默认配置
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

// Newf 创建带格式化消息的应用错误
func (f *ErrorFactory) Newf(code ErrorCode, operation string, cause error, format string, args ...interface{}) *AppError {
	err := f.New(code, operation, cause)
	err.Message = fmt.Sprintf(format, args...)
	return err
}

// NewWithDetails 创建带详细信息的应用错误
func (f *ErrorFactory) NewWithDetails(code ErrorCode, operation string, details string, cause error) *AppError {
	err := f.New(code, operation, cause)
	err.Details = details
	return err
}

// Wrap 包装现有错误
func (f *ErrorFactory) Wrap(cause error, operation string) *AppError {
	if cause == nil {
		return nil
	}

	// 如果已经是AppError，直接返回
	if appErr, ok := cause.(*AppError); ok {
		return appErr
	}

	// 根据错误类型推断错误码
	code := f.inferErrorCode(cause)
	return f.New(code, operation, cause)
}

// Wrapf 包装现有错误并添加格式化消息
func (f *ErrorFactory) Wrapf(cause error, operation string, format string, args ...interface{}) *AppError {
	err := f.Wrap(cause, operation)
	if err != nil {
		err.Message = fmt.Sprintf(format, args...)
	}
	return err
}

// InternalError 创建内部错误
func (f *ErrorFactory) InternalError(operation string, cause error) *AppError {
	return f.New(ErrCodeInternal, operation, cause)
}

// InternalErrorf 创建带格式化消息的内部错误
func (f *ErrorFactory) InternalErrorf(operation string, cause error, format string, args ...interface{}) *AppError {
	return f.Newf(ErrCodeInternal, operation, cause, format, args...)
}

// InvalidInputError 创建输入验证错误
func (f *ErrorFactory) InvalidInputError(operation string, field, value string, cause error) *AppError {
	details := fmt.Sprintf("Invalid value for field '%s': %s", field, value)
	return f.NewWithDetails(ErrCodeInvalidInput, operation, details, cause)
}

// NotFoundError 创建资源未找到错误
func (f *ErrorFactory) NotFoundError(operation, resourceType, resourceID string) *AppError {
	details := fmt.Sprintf("%s with ID '%s' not found", resourceType, resourceID)
	return f.NewWithDetails(ErrCodeNotFound, operation, details, nil)
}

// UnauthorizedError 创建未授权错误
func (f *ErrorFactory) UnauthorizedError(operation string, cause error) *AppError {
	return f.New(ErrCodeUnauthorized, operation, cause)
}

// ForbiddenError 创建禁止访问错误
func (f *ErrorFactory) ForbiddenError(operation string, resource string, cause error) *AppError {
	details := fmt.Sprintf("Access to '%s' is forbidden", resource)
	return f.NewWithDetails(ErrCodeForbidden, operation, details, cause)
}

// ConflictError 创建冲突错误
func (f *ErrorFactory) ConflictError(operation string, resource string, cause error) *AppError {
	details := fmt.Sprintf("Conflict with existing %s", resource)
	return f.NewWithDetails(ErrCodeConflict, operation, details, cause)
}

// TimeoutError 创建超时错误
func (f *ErrorFactory) TimeoutError(operation string, timeout time.Duration, cause error) *AppError {
	details := fmt.Sprintf("Operation timed out after %v", timeout)
	return f.NewWithDetails(ErrCodeTimeout, operation, details, cause).
		WithRetry(timeout, 1) // 建议重试一次
}

// RateLimitError 创建速率限制错误
func (f *ErrorFactory) RateLimitError(operation string, limit int, window time.Duration) *AppError {
	details := fmt.Sprintf("Rate limit exceeded: %d requests per %v", limit, window)
	return f.NewWithDetails(ErrCodeRateLimit, operation, details, nil).
		WithHTTPHeader("Retry-After", window.String()).
		WithRetry(window, 1)
}

// DatabaseError 创建数据库错误
func (f *ErrorFactory) DatabaseError(operation string, query string, cause error) *AppError {
	details := fmt.Sprintf("Database error in query: %s", query)
	return f.NewWithDetails(ErrCodeDatabaseError, operation, details, cause).
		WithRetry(1*time.Second, 3) // 建议重试3次，每次间隔1秒
}

// NetworkError 创建网络错误
func (f *ErrorFactory) NetworkError(operation string, endpoint string, cause error) *AppError {
	details := fmt.Sprintf("Network error when calling %s", endpoint)
	return f.NewWithDetails(ErrCodeNetworkError, operation, details, cause).
		WithRetry(2*time.Second, 2) // 建议重试2次，每次间隔2秒
}

// ServiceUnavailableError 创建服务不可用错误
func (f *ErrorFactory) ServiceUnavailableError(operation, service string, cause error) *AppError {
	details := fmt.Sprintf("Service '%s' is currently unavailable", service)
	return f.NewWithDetails(ErrCodeServiceUnavailable, operation, details, cause).
		WithRetry(5*time.Second, 3) // 建议重试3次，每次间隔5秒
}

// GameNotFoundError 创建游戏未找到错误
func (f *ErrorFactory) GameNotFoundError(operation, gameID string) *AppError {
	details := fmt.Sprintf("Game with ID '%s' not found", gameID)
	return f.NewWithDetails(ErrCodeGameNotFound, operation, details, nil)
}

// GameAlreadyExistError 创建游戏已存在错误
func (f *ErrorFactory) GameAlreadyExistError(operation, gameName string) *AppError {
	details := fmt.Sprintf("Game with name '%s' already exists", gameName)
	return f.NewWithDetails(ErrCodeGameAlreadyExist, operation, details, nil)
}

// GameDisabledError 创建游戏已禁用错误
func (f *ErrorFactory) GameDisabledError(operation, gameID string) *AppError {
	details := fmt.Sprintf("Game '%s' is currently disabled", gameID)
	return f.NewWithDetails(ErrCodeGameDisabled, operation, details, nil)
}

// inferErrorCode 根据错误类型推断错误码
func (f *ErrorFactory) inferErrorCode(err error) ErrorCode {
	errStr := err.Error()

	// 根据错误字符串推断错误类型
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

// containsAny 检查字符串是否包含任意一个关键词
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

// 全局默认错误工厂
var DefaultFactory = NewErrorFactory("croupier")

// 全局便捷函数
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
