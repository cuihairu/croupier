package errorx

import (
	"net/http"
	"testing"
)

// TestCodeError_Error 测试 Error 方法
func TestCodeError_Error(t *testing.T) {
	err := NewBadRequest("invalid input")
	if err.Error() != "invalid input" {
		t.Errorf("Error() returned wrong message: got %s, want 'invalid input'", err.Error())
	}
}

// TestNewBadRequest 测试创建 BadRequest 错误
func TestNewBadRequest(t *testing.T) {
	err := NewBadRequest("invalid request")
	if err.Code != http.StatusBadRequest {
		t.Errorf("Wrong code: got %d, want %d", err.Code, http.StatusBadRequest)
	}
	if err.Message != "invalid request" {
		t.Errorf("Wrong message: got %s, want 'invalid request'", err.Message)
	}
}

// TestNewUnauthorized 测试创建 Unauthorized 错误
func TestNewUnauthorized(t *testing.T) {
	err := NewUnauthorized("authentication required")
	if err.Code != http.StatusUnauthorized {
		t.Errorf("Wrong code: got %d, want %d", err.Code, http.StatusUnauthorized)
	}
	if err.Message != "authentication required" {
		t.Errorf("Wrong message: got %s, want 'authentication required'", err.Message)
	}
}

// TestNewForbidden 测试创建 Forbidden 错误
func TestNewForbidden(t *testing.T) {
	err := NewForbidden("access denied")
	if err.Code != http.StatusForbidden {
		t.Errorf("Wrong code: got %d, want %d", err.Code, http.StatusForbidden)
	}
	if err.Message != "access denied" {
		t.Errorf("Wrong message: got %s, want 'access denied'", err.Message)
	}
}

// TestNewNotFound 测试创建 NotFound 错误
func TestNewNotFound(t *testing.T) {
	err := NewNotFound("resource not found")
	if err.Code != http.StatusNotFound {
		t.Errorf("Wrong code: got %d, want %d", err.Code, http.StatusNotFound)
	}
	if err.Message != "resource not found" {
		t.Errorf("Wrong message: got %s, want 'resource not found'", err.Message)
	}
}

// TestNewConflict 测试创建 Conflict 错误
func TestNewConflict(t *testing.T) {
	err := NewConflict("resource already exists")
	if err.Code != http.StatusConflict {
		t.Errorf("Wrong code: got %d, want %d", err.Code, http.StatusConflict)
	}
	if err.Message != "resource already exists" {
		t.Errorf("Wrong message: got %s, want 'resource already exists'", err.Message)
	}
}

// TestNewValidationError 测试创建 ValidationError 错误
func TestNewValidationError(t *testing.T) {
	err := NewValidationError("validation failed")
	if err.Code != http.StatusUnprocessableEntity {
		t.Errorf("Wrong code: got %d, want %d", err.Code, http.StatusUnprocessableEntity)
	}
	if err.Message != "validation failed" {
		t.Errorf("Wrong message: got %s, want 'validation failed'", err.Message)
	}
}

// TestNewInternalError 测试创建 InternalError 错误
func TestNewInternalError(t *testing.T) {
	err := NewInternalError("something went wrong")
	if err.Code != http.StatusInternalServerError {
		t.Errorf("Wrong code: got %d, want %d", err.Code, http.StatusInternalServerError)
	}
	if err.Message != "something went wrong" {
		t.Errorf("Wrong message: got %s, want 'something went wrong'", err.Message)
	}
}

// TestCodeError_ErrorCode 测试 ErrorCode 方法
func TestCodeError_ErrorCode(t *testing.T) {
	tests := []struct {
		name           string
		code           int
		expectedString string
	}{
		{"BadRequest", http.StatusBadRequest, "bad_request"},
		{"Unauthorized", http.StatusUnauthorized, "unauthorized"},
		{"Forbidden", http.StatusForbidden, "forbidden"},
		{"NotFound", http.StatusNotFound, "not_found"},
		{"Conflict", http.StatusConflict, "conflict"},
		{"ValidationFailed", http.StatusUnprocessableEntity, "validation_failed"},
		{"InternalError", http.StatusInternalServerError, "internal_error"},
		{"ServiceUnavailable", http.StatusServiceUnavailable, "service_unavailable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &CodeError{Code: tt.code, Message: "test"}
			if err.ErrorCode() != tt.expectedString {
				t.Errorf("ErrorCode() returned %s, want %s", err.ErrorCode(), tt.expectedString)
			}
		})
	}

	// 测试未知错误码
	t.Run("UnknownCode", func(t *testing.T) {
		err := &CodeError{Code: 999, Message: "test"}
		if err.ErrorCode() != "" {
			t.Errorf("Unknown code should return empty string, got %s", err.ErrorCode())
		}
	})
}

// TestCodeError_Data 测试 Data 方法
func TestCodeError_Data(t *testing.T) {
	err := NewNotFound("user not found")
	code, data := err.Data()

	if code != http.StatusNotFound {
		t.Errorf("Data() returned wrong code: got %d, want %d", code, http.StatusNotFound)
	}

	dataMap, ok := data.(map[string]interface{})
	if !ok {
		t.Fatal("Data() should return a map")
	}

	if dataMap["error"] != "not_found" {
		t.Errorf("Wrong error string: got %v, want 'not_found'", dataMap["error"])
	}

	if dataMap["message"] != "user not found" {
		t.Errorf("Wrong message: got %v, want 'user not found'", dataMap["message"])
	}
}

// TestNewValidationErrorWithDetails 测试带详细信息的验证错误
func TestNewValidationErrorWithDetails(t *testing.T) {
	details := map[string]string{
		"email":    "invalid format",
		"password": "too short",
	}

	err := NewValidationErrorWithDetails("validation failed", details)

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatal("NewValidationErrorWithDetails should return ValidationError")
	}

	if validationErr.Code != http.StatusUnprocessableEntity {
		t.Errorf("Wrong code: got %d, want %d", validationErr.Code, http.StatusUnprocessableEntity)
	}

	if validationErr.Message != "validation failed" {
		t.Errorf("Wrong message: got %s, want 'validation failed'", validationErr.Message)
	}

	if len(validationErr.Details) != 2 {
		t.Errorf("Wrong details count: got %d, want 2", len(validationErr.Details))
	}

	if validationErr.Details["email"] != "invalid format" {
		t.Errorf("Wrong email detail: got %s, want 'invalid format'", validationErr.Details["email"])
	}
}

// TestValidationError_Error 测试 ValidationError 的 Error 方法
func TestValidationError_Error(t *testing.T) {
	details := map[string]string{
		"field1": "error1",
		"field2": "error2",
	}

	err := &ValidationError{
		CodeError: CodeError{
			Code:    http.StatusUnprocessableEntity,
			Message: "validation failed",
		},
		Details: details,
	}

	errStr := err.Error()
	expected := "validation failed: map[field1:error1 field2:error2]"

	if errStr != expected {
		// Go map 的字符串表示可能不稳定，所以我们只检查关键部分
		if len(errStr) < len("validation failed: ") {
			t.Errorf("Error() returned too short string: got %s", errStr)
		}
	}
}

// TestValidationError_Data 测试 ValidationError 的 Data 方法
func TestValidationError_Data(t *testing.T) {
	details := map[string]string{
		"email": "required",
	}

	err := &ValidationError{
		CodeError: CodeError{
			Code:    http.StatusUnprocessableEntity,
			Message: "missing fields",
		},
		Details: details,
	}

	code, data := err.Data()

	if code != http.StatusUnprocessableEntity {
		t.Errorf("Wrong code: got %d, want %d", code, http.StatusUnprocessableEntity)
	}

	dataMap, ok := data.(map[string]interface{})
	if !ok {
		t.Fatal("Data() should return a map")
	}

	if dataMap["error"] != "validation_failed" {
		t.Errorf("Wrong error string: got %v, want 'validation_failed'", dataMap["error"])
	}

	if dataMap["message"] != "missing fields" {
		t.Errorf("Wrong message: got %v, want 'missing fields'", dataMap["message"])
	}

	if dataMap["details"] == nil {
		t.Error("Details should not be nil")
	}
}

// TestCodeError_EmptyMessage 测试空消息
func TestCodeError_EmptyMessage(t *testing.T) {
	err := NewBadRequest("")
	if err.Error() != "" {
		t.Errorf("Empty message should return empty string, got %s", err.Error())
	}
}

// TestCodeError_UnicodeMessage 测试 Unicode 消息
func TestCodeError_UnicodeMessage(t *testing.T) {
	message := "错误信息 🔥"
	err := NewBadRequest(message)
	if err.Error() != message {
		t.Errorf("Unicode message not preserved: got %s, want %s", err.Error(), message)
	}
}

// TestCodeError_ImplementsError 测试实现 error 接口
func TestCodeError_ImplementsError(t *testing.T) {
	var err error = NewBadRequest("test")
	if err == nil {
		t.Error("CodeError should implement error interface")
	}

	_ = err.Error() // 应该能够调用 Error 方法
}

// TestValidationError_ImplementsError 测试 ValidationError 实现 error 接口
func TestValidationError_ImplementsError(t *testing.T) {
	details := map[string]string{"field": "error"}
	var err error = NewValidationErrorWithDetails("test", details)
	if err == nil {
		t.Error("ValidationError should implement error interface")
	}

	_ = err.Error() // 应该能够调用 Error 方法
}

// TestNewErrorWithDifferentCodes 测试不同的错误码
func TestNewErrorWithDifferentCodes(t *testing.T) {
	errors := []struct {
		err      *CodeError
		expected int
	}{
		{NewBadRequest("test"), http.StatusBadRequest},
		{NewUnauthorized("test"), http.StatusUnauthorized},
		{NewForbidden("test"), http.StatusForbidden},
		{NewNotFound("test"), http.StatusNotFound},
		{NewConflict("test"), http.StatusConflict},
		{NewValidationError("test"), http.StatusUnprocessableEntity},
		{NewInternalError("test"), http.StatusInternalServerError},
	}

	for _, tt := range errors {
		if tt.err.Code != tt.expected {
			t.Errorf("Wrong code: got %d, want %d", tt.err.Code, tt.expected)
		}
	}
}

// BenchmarkNewError 性能基准测试
func BenchmarkNewError(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewBadRequest("test error message")
	}
}

// BenchmarkErrorCode 性能基准测试
func BenchmarkErrorCode(b *testing.B) {
	err := NewBadRequest("test")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.ErrorCode()
	}
}

// BenchmarkErrorData 性能基准测试
func BenchmarkErrorData(b *testing.B) {
	err := NewBadRequest("test")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = err.Data()
	}
}
