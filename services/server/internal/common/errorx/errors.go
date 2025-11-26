package errorx

import (
	"fmt"
	"net/http"
)

// CodeError 带错误码的错误
type CodeError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *CodeError) Error() string {
	return e.Message
}

// Data 返回错误的 HTTP 状态码和响应体
func (e *CodeError) Data() (int, interface{}) {
	return e.Code, map[string]interface{}{
		"error":   e.ErrorCode(),
		"message": e.Message,
	}
}

// ErrorCode 返回错误码字符串
func (e *CodeError) ErrorCode() string {
	return errorCodeMap[e.Code]
}

// 错误码映射
var errorCodeMap = map[int]string{
	http.StatusBadRequest:          "bad_request",
	http.StatusUnauthorized:         "unauthorized",
	http.StatusForbidden:            "forbidden",
	http.StatusNotFound:             "not_found",
	http.StatusConflict:             "conflict",
	http.StatusUnprocessableEntity:  "validation_failed",
	http.StatusInternalServerError:  "internal_error",
	http.StatusServiceUnavailable:   "service_unavailable",
}

// 预定义错误构造函数
func NewBadRequest(message string) *CodeError {
	return &CodeError{
		Code:    http.StatusBadRequest,
		Message: message,
	}
}

func NewUnauthorized(message string) *CodeError {
	return &CodeError{
		Code:    http.StatusUnauthorized,
		Message: message,
	}
}

func NewForbidden(message string) *CodeError {
	return &CodeError{
		Code:    http.StatusForbidden,
		Message: message,
	}
}

func NewNotFound(message string) *CodeError {
	return &CodeError{
		Code:    http.StatusNotFound,
		Message: message,
	}
}

func NewConflict(message string) *CodeError {
	return &CodeError{
		Code:    http.StatusConflict,
		Message: message,
	}
}

func NewValidationError(message string) *CodeError {
	return &CodeError{
		Code:    http.StatusUnprocessableEntity,
		Message: message,
	}
}

func NewInternalError(message string) *CodeError {
	return &CodeError{
		Code:    http.StatusInternalServerError,
		Message: message,
	}
}

// NewValidationErrorWithDetails 带详细信息的验证错误
func NewValidationErrorWithDetails(message string, details map[string]string) error {
	return &ValidationError{
		CodeError: CodeError{
			Code:    http.StatusUnprocessableEntity,
			Message: message,
		},
		Details: details,
	}
}

// ValidationError 验证错误（包含详细字段信息）
type ValidationError struct {
	CodeError
	Details map[string]string `json:"details"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %v", e.Message, e.Details)
}

func (e *ValidationError) Data() (int, interface{}) {
	return e.Code, map[string]interface{}{
		"error":   "validation_failed",
		"message": e.Message,
		"details": e.Details,
	}
}
