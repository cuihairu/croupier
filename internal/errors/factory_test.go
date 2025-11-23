package errors

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestErrorFactory_New(t *testing.T) {
	factory := NewErrorFactory("test-service")

	err := factory.New(ErrCodeInternal, "test-operation", errors.New("test cause"))

	assert.NotNil(t, err)
	assert.Equal(t, ErrCodeInternal, err.Code)
	assert.Equal(t, "test-operation", err.Operation)
	assert.Equal(t, "test-service", err.Context["service"])
	assert.NotEmpty(t, err.TraceID)
	assert.Equal(t, errors.New("test cause"), err.Cause)
}

func TestErrorFactory_Newf(t *testing.T) {
	factory := NewErrorFactory("test-service")

	err := factory.Newf(ErrCodeNotFound, "test-operation", nil, "User %s not found", "john")

	assert.NotNil(t, err)
	assert.Equal(t, "User john not found", err.Message)
	assert.Equal(t, ErrCodeNotFound, err.Code)
}

func TestErrorFactory_Wrap(t *testing.T) {
	factory := NewErrorFactory("test-service")

	originalErr := errors.New("original error")
	wrappedErr := factory.Wrap(originalErr, "test-operation")

	assert.NotNil(t, wrappedErr)
	assert.Equal(t, originalErr, wrappedErr.Cause)
	assert.Equal(t, "test-operation", wrappedErr.Operation)
}

func TestErrorFactory_WrapAppError(t *testing.T) {
	factory := NewErrorFactory("test-service")

	appErr := factory.New(ErrCodeGameNotFound, "test-operation", nil)
	wrappedErr := factory.Wrap(appErr, "another-operation")

	// AppError不应该被包装
	assert.Equal(t, appErr, wrappedErr)
}

func TestErrorFactory_InvalidInputError(t *testing.T) {
	factory := NewErrorFactory("test-service")

	err := factory.InvalidInputError("test-operation", "email", "invalid-email", nil)

	assert.NotNil(t, err)
	assert.Equal(t, ErrCodeInvalidInput, err.Code)
	assert.Contains(t, err.Details, "email")
	assert.Contains(t, err.Details, "invalid-email")
}

func TestErrorFactory_NotFoundError(t *testing.T) {
	factory := NewErrorFactory("test-service")

	err := factory.NotFoundError("test-operation", "User", "123")

	assert.NotNil(t, err)
	assert.Equal(t, ErrCodeNotFound, err.Code)
	assert.Contains(t, err.Details, "User")
	assert.Contains(t, err.Details, "123")
}

func TestErrorFactory_GameNotFoundError(t *testing.T) {
	factory := NewErrorFactory("test-service")

	err := factory.GameNotFoundError("test-operation", "game-123")

	assert.NotNil(t, err)
	assert.Equal(t, ErrCodeGameNotFound, err.Code)
	assert.Contains(t, err.Details, "game-123")
}

func TestErrorFactory_TimeoutError(t *testing.T) {
	factory := NewErrorFactory("test-service")

	timeout := 5 * time.Second
	err := factory.TimeoutError("test-operation", timeout, errors.New("timeout"))

	assert.NotNil(t, err)
	assert.Equal(t, ErrCodeTimeout, err.Code)
	assert.True(t, err.Retryable)
	assert.Equal(t, timeout, err.RetryDelay)
	assert.Equal(t, 1, err.RetryCount)
}

func TestErrorFactory_RateLimitError(t *testing.T) {
	factory := NewErrorFactory("test-service")

	limit := 100
	window := 1 * time.Minute
	err := factory.RateLimitError("test-operation", limit, window)

	assert.NotNil(t, err)
	assert.Equal(t, ErrCodeRateLimit, err.Code)
	assert.True(t, err.Retryable)
	assert.Equal(t, window, err.RetryDelay)
	assert.NotNil(t, err.HTTPHeaders)
	assert.Equal(t, "Retry-After", err.HTTPHeaders["Retry-After"])
}

func TestErrorFactory_DatabaseError(t *testing.T) {
	factory := NewErrorFactory("test-service")

	err := factory.DatabaseError("test-operation", "SELECT * FROM users", errors.New("connection failed"))

	assert.NotNil(t, err)
	assert.Equal(t, ErrCodeDatabaseError, err.Code)
	assert.True(t, err.Retryable)
	assert.Equal(t, 1*time.Second, err.RetryDelay)
	assert.Equal(t, 3, err.RetryCount)
}

func TestErrorFactory_NetworkError(t *testing.T) {
	factory := NewErrorFactory("test-service")

	err := factory.NetworkError("test-operation", "https://api.example.com", errors.New("connection refused"))

	assert.NotNil(t, err)
	assert.Equal(t, ErrCodeNetworkError, err.Code)
	assert.True(t, err.Retryable)
	assert.Equal(t, 2*time.Second, err.RetryDelay)
	assert.Equal(t, 2, err.RetryCount)
}

func TestErrorFactory_ServiceUnavailableError(t *testing.T) {
	factory := NewErrorFactory("test-service")

	err := factory.ServiceUnavailableError("test-operation", "payment-service", errors.New("service down"))

	assert.NotNil(t, err)
	assert.Equal(t, ErrCodeServiceUnavailable, err.Code)
	assert.True(t, err.Retryable)
	assert.Equal(t, 5*time.Second, err.RetryDelay)
	assert.Equal(t, 3, err.RetryCount)
}

func TestErrorFactory_GameAlreadyExistError(t *testing.T) {
	factory := NewErrorFactory("test-service")

	err := factory.GameAlreadyExistError("test-operation", "My Game")

	assert.NotNil(t, err)
	assert.Equal(t, ErrCodeGameAlreadyExist, err.Code)
	assert.Contains(t, err.Details, "My Game")
}

func TestErrorFactory_GameDisabledError(t *testing.T) {
	factory := NewErrorFactory("test-service")

	err := factory.GameDisabledError("test-operation", "game-123")

	assert.NotNil(t, err)
	assert.Equal(t, ErrCodeGameDisabled, err.Code)
	assert.Contains(t, err.Details, "game-123")
}

func TestAppError_WithMethods(t *testing.T) {
	factory := NewErrorFactory("test-service")

	err := factory.New(ErrCodeInternal, "test-operation", errors.New("test cause"))

	// 测试WithDetails
	err.WithDetails("additional details")
	assert.Equal(t, "additional details", err.Details)

	// 测试WithContext
	err.WithContext("user_id", 123)
	assert.Equal(t, 123, err.Context["user_id"])

	// 测试WithRetry
	err.WithRetry(2*time.Second, 3)
	assert.True(t, err.Retryable)
	assert.Equal(t, 2*time.Second, err.RetryDelay)
	assert.Equal(t, 3, err.RetryCount)

	// 测试WithHTTPHeader
	err.WithHTTPHeader("X-Custom", "value")
	assert.Equal(t, "value", err.HTTPHeaders["X-Custom"])
}

func TestAppError_Error(t *testing.T) {
	factory := NewErrorFactory("test-service")

	err := factory.New(ErrCodeInternal, "test-operation", errors.New("test cause"))

	// 不带详细信息
	expected := "[INTERNAL_ERROR] test-operation: Internal server error"
	assert.Equal(t, expected, err.Error())

	// 带详细信息
	err.WithDetails("additional details")
	expected = "[INTERNAL_ERROR] test-operation: Internal server error - additional details"
	assert.Equal(t, expected, err.Error())
}

func TestAppError_Is(t *testing.T) {
	factory := NewErrorFactory("test-service")

	err1 := factory.New(ErrCodeGameNotFound, "test-operation", nil)
	err2 := factory.New(ErrCodeGameNotFound, "test-operation", nil)
	err3 := factory.New(ErrCodeInternal, "test-operation", nil)

	// 相同错误码应该相等
	assert.True(t, err1.Is(err2))

	// 不同错误码应该不相等
	assert.False(t, err1.Is(err3))

	// 与标准错误比较
	assert.False(t, err1.Is(errors.New("standard error")))
}

func TestAppError_ToJSON(t *testing.T) {
	factory := NewErrorFactory("test-service")

	err := factory.New(ErrCodeGameNotFound, "test-operation", nil)
	err.WithDetails("test details").
		WithContext("user_id", 123).
		WithRetry(1*time.Second, 2).
		WithHTTPHeader("X-Custom", "value")

	jsonData := err.ToJSON()

	assert.NotNil(t, jsonData)
	assert.Equal(t, ErrCodeGameNotFound, jsonData["code"])
	assert.Equal(t, "test-operation", jsonData["operation"])
	assert.Equal(t, "test details", jsonData["details"])
	assert.Equal(t, 123, jsonData["context"].(map[string]interface{})["user_id"])
	assert.Equal(t, true, jsonData["retryable"])
	assert.Equal(t, "1s", jsonData["retry_delay"])
	assert.Equal(t, 2, jsonData["retry_count"])
}

func TestErrorCollection(t *testing.T) {
	factory := NewErrorFactory("test-service")

	collection := &ErrorCollection{}

	assert.False(t, collection.HasErrors())
	assert.Equal(t, 0, collection.Total)
	assert.Nil(t, collection.First())

	// 添加错误
	err1 := factory.New(ErrCodeGameNotFound, "op1", nil)
	err2 := factory.New(ErrCodeInternal, "op2", nil)

	collection.Add(err1)
	collection.Add(err2)

	assert.True(t, collection.HasErrors())
	assert.Equal(t, 2, collection.Total)
	assert.Equal(t, err1, collection.First())

	// 按错误码查找
	gameErrors := collection.GetByCode(ErrCodeGameNotFound)
	assert.Len(t, gameErrors, 1)
	assert.Equal(t, err1, gameErrors[0])

	// 按严重级别查找
	lowSeverityErrors := collection.GetBySeverity(SeverityLow)
	assert.Len(t, lowSeverityErrors, 1)
}

func TestGlobalConvenienceFunctions(t *testing.T) {
	// 测试全局便捷函数
	err := New(ErrCodeGameNotFound, "test-operation", nil)
	assert.NotNil(t, err)
	assert.Equal(t, ErrCodeGameNotFound, err.Code)

	wrappedErr := Wrap(errors.New("original"), "test-operation")
	assert.NotNil(t, wrappedErr)
	assert.Equal(t, errors.New("original"), wrappedErr.Cause)

	internalErr := InternalError("test-operation", errors.New("internal"))
	assert.NotNil(t, internalErr)
	assert.Equal(t, ErrCodeInternal, internalErr.Code)

	invalidErr := InvalidInputError("test-operation", "email", "invalid", nil)
	assert.NotNil(t, invalidErr)
	assert.Equal(t, ErrCodeInvalidInput, invalidErr.Code)

	notFoundErr := NotFoundError("test-operation", "User", "123")
	assert.NotNil(t, notFoundErr)
	assert.Equal(t, ErrCodeNotFound, notFoundErr.Code)

	gameErr := GameNotFoundError("test-operation", "game-123")
	assert.NotNil(t, gameErr)
	assert.Equal(t, ErrCodeGameNotFound, gameErr.Code)
}
