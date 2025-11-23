package errors

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponseBuilder_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	builder := NewResponseBuilder[string]()
	response := builder.Success("test-data").Build()

	assert.True(t, response.Success)
	assert.Equal(t, "test-data", response.Data)
	assert.Nil(t, response.Error)
	assert.Equal(t, ResponseTypeSuccess, response.Type)
}

func TestResponseBuilder_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)

	factory := NewErrorFactory("test-service")
	appErr := factory.New(ErrCodeGameNotFound, "test-operation", nil)

	builder := NewResponseBuilder[any]()
	response := builder.Error(appErr).Build()

	assert.False(t, response.Success)
	assert.Nil(t, response.Data)
	assert.NotNil(t, response.Error)
	assert.Equal(t, ResponseTypeError, response.Type)
	assert.Equal(t, ErrCodeGameNotFound, response.Error.Code)
}

func TestResponseBuilder_WithMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)

	metadata := &Metadata{
		Version:   "1.0.0",
		Duration:  100 * time.Millisecond,
		Timestamp: time.Now(),
	}

	builder := NewResponseBuilder[string]()
	response := builder.Success("test-data").WithMetadata(metadata).Build()

	assert.NotNil(t, response.Metadata)
	assert.Equal(t, "1.0.0", response.Metadata.Version)
	assert.Equal(t, 100*time.Millisecond, response.Metadata.Duration)
}

func TestResponseBuilder_WithPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)

	pagination := &Pagination{
		Page:       1,
		PageSize:   10,
		Total:      100,
		TotalPages: 10,
		HasNext:    true,
		HasPrev:    false,
	}

	builder := NewResponseBuilder[[]string]()
	response := builder.Success([]string{"item1", "item2"}).WithPagination(pagination).Build()

	assert.NotNil(t, response.Metadata)
	assert.NotNil(t, response.Metadata.Pagination)
	assert.Equal(t, 1, response.Metadata.Pagination.Page)
	assert.Equal(t, 10, response.Metadata.Pagination.PageSize)
	assert.Equal(t, int64(100), response.Metadata.Pagination.Total)
}

func TestResponseBuilder_WithPerformance(t *testing.T) {
	gin.SetMode(gin.TestMode)

	performance := &Performance{
		DatabaseQueries: 5,
		DatabaseTime:    50 * time.Millisecond,
		CacheHits:       3,
		CacheMisses:     2,
		ExternalCalls:   1,
		ExternalTime:    200 * time.Millisecond,
	}

	builder := NewResponseBuilder[string]()
	response := builder.Success("test-data").WithPerformance(performance).Build()

	assert.NotNil(t, response.Metadata)
	assert.NotNil(t, response.Metadata.Performance)
	assert.Equal(t, 5, response.Metadata.Performance.DatabaseQueries)
	assert.Equal(t, 3, response.Metadata.Performance.CacheHits)
}

func TestResponseBuilder_WriteJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建Gin上下文
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)

	// 测试成功响应
	builder := NewResponseBuilder[string]()
	builder.Success("test-data").WriteJSON(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))

	// 验证响应内容
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response["success"].(bool))
	assert.Equal(t, "test-data", response["data"])
}

func TestResponseBuilder_WriteJSONError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建Gin上下文
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)

	// 创建AppError
	factory := NewErrorFactory("test-service")
	appErr := factory.New(ErrCodeGameNotFound, "test-operation", nil)

	// 测试错误响应
	builder := NewResponseBuilder[any]()
	builder.Error(appErr).WriteJSON(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

	// 验证响应内容
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.False(t, response["success"].(bool))
	assert.NotNil(t, response["error"])
}

func TestConvenienceFunctions(t *testing.T) {
	// 测试SuccessResponse
	successResp := SuccessResponse("test-data")
	assert.True(t, successResp.Success)
	assert.Equal(t, "test-data", successResp.Data)

	// 测试ErrorResponse
	factory := NewErrorFactory("test-service")
	appErr := factory.New(ErrCodeInternal, "test-op", nil)
	errorResp := ErrorResponse(appErr)
	assert.False(t, errorResp.Success)
	assert.NotNil(t, errorResp.Error)

	// 测试PaginatedResponse
	pagination := &Pagination{Page: 1, PageSize: 10, Total: 100}
	paginatedResp := PaginatedResponse([]string{"item1"}, pagination)
	assert.True(t, paginatedResp.Success)
	assert.NotNil(t, paginatedResp.Metadata.Pagination)
}

func TestSendFunctions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("SendSuccess", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)

		SendSuccess(c, "test-data")

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("SendError", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)

		factory := NewErrorFactory("test-service")
		appErr := factory.New(ErrCodeGameNotFound, "test-op", nil)

		SendError(c, appErr)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("SendPaginated", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)

		pagination := &Pagination{Page: 1, PageSize: 10, Total: 100}
		SendPaginated(c, []string{"item1"}, pagination)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestResponseMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建Gin引擎
	router := gin.New()
	router.Use(ResponseMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	// 测试请求
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "test-request-id")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test-request-id", w.Header().Get("X-Request-ID"))
}

func TestResponseMiddleware_GenerateRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建Gin引擎
	router := gin.New()
	router.Use(ResponseMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	// 测试请求（不提供X-Request-ID）
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
}

func TestErrorMetadata_ToJSON(t *testing.T) {
	factory := NewErrorFactory("test-service")

	err := factory.New(ErrCodeGameNotFound, "test-operation", nil)
	err.WithDetails("test details").
		WithContext("user_id", 123).
		WithRetry(1*time.Second, 2).
		WithHTTPHeader("X-Custom", "value")

	jsonData := err.ToJSON()

	// 转换为JSON字符串并验证
	jsonBytes, jsonErr := json.Marshal(jsonData)
	require.NoError(t, jsonErr)
	jsonStr := string(jsonBytes)

	assert.Contains(t, jsonStr, "GAME_NOT_FOUND")
	assert.Contains(t, jsonStr, "test details")
	assert.Contains(t, jsonStr, "retryable")
}

func TestErrorInfo_JsonMarshal(t *testing.T) {
	errorInfo := &ErrorInfo{
		Code:      ErrCodeGameNotFound,
		Message:   "Game not found",
		Details:   "Additional details",
		Operation: "test-operation",
		TraceID:   "trace-123",
		Severity:  SeverityLow,
		Retryable: true,
		Context: map[string]interface{}{
			"user_id": 123,
		},
	}

	jsonBytes, err := json.Marshal(errorInfo)
	require.NoError(t, err)

	var unmarshaled ErrorInfo
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, errorInfo.Code, unmarshaled.Code)
	assert.Equal(t, errorInfo.Message, unmarshaled.Message)
	assert.Equal(t, errorInfo.Details, unmarshaled.Details)
	assert.Equal(t, errorInfo.Operation, unmarshaled.Operation)
	assert.Equal(t, errorInfo.TraceID, unmarshaled.TraceID)
	assert.Equal(t, errorInfo.Severity, unmarshaled.Severity)
	assert.Equal(t, errorInfo.Retryable, unmarshaled.Retryable)
	assert.Equal(t, 123, unmarshaled.Context["user_id"])
}

func TestAPIResponse_JsonMarshal(t *testing.T) {
	response := &APIResponse[string]{
		Type:      ResponseTypeSuccess,
		Success:   true,
		Data:      "test data",
		Timestamp: time.Now(),
		RequestID: "req-123",
	}

	jsonBytes, err := json.Marshal(response)
	require.NoError(t, err)

	var unmarshaled APIResponse[string]
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, response.Type, unmarshaled.Type)
	assert.Equal(t, response.Success, unmarshaled.Success)
	assert.Equal(t, response.Data, unmarshaled.Data)
	assert.Equal(t, response.RequestID, unmarshaled.RequestID)
}