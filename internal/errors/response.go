package errors

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ResponseType 响应类型
type ResponseType string

const (
	ResponseTypeSuccess ResponseType = "success"
	ResponseTypeError  ResponseType = "error"
)

// APIResponse 通用API响应结构
type APIResponse[T any] struct {
	Type      ResponseType `json:"type"`
	Success   bool         `json:"success"`
	Data      T            `json:"data,omitempty"`
	Error     *ErrorInfo   `json:"error,omitempty"`
	Metadata  *Metadata    `json:"metadata,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
	RequestID string       `json:"request_id"`
}

// ErrorInfo 错误信息
type ErrorInfo struct {
	Code       ErrorCode              `json:"code"`
	Message    string                 `json:"message"`
	Details    string                 `json:"details,omitempty"`
	Operation  string                 `json:"operation,omitempty"`
	TraceID    string                 `json:"trace_id,omitempty"`
	Severity   ErrorSeverity          `json:"severity,omitempty"`
	Retryable  bool                   `json:"retryable,omitempty"`
	RetryDelay *time.Duration         `json:"retry_delay,omitempty"`
	Context    map[string]interface{} `json:"context,omitempty"`
}

// Metadata 响应元数据
type Metadata struct {
	RequestID   string        `json:"request_id"`
	Version     string        `json:"version"`
	Duration    time.Duration `json:"duration,omitempty"`
	Timestamp   time.Time     `json:"timestamp"`
	Pagination  *Pagination   `json:"pagination,omitempty"`
	Performance *Performance  `json:"performance,omitempty"`
}

// Pagination 分页信息
type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

// Performance 性能信息
type Performance struct {
	DatabaseQueries int           `json:"database_queries,omitempty"`
	DatabaseTime    time.Duration `json:"database_time,omitempty"`
	CacheHits       int           `json:"cache_hits,omitempty"`
	CacheMisses     int           `json:"cache_misses,omitempty"`
	ExternalCalls   int           `json:"external_calls,omitempty"`
	ExternalTime    time.Duration `json:"external_time,omitempty"`
}

// ResponseBuilder 响应构建器
type ResponseBuilder[T any] struct {
	response *APIResponse[T]
}

// NewResponseBuilder 创建响应构建器
func NewResponseBuilder[T any]() *ResponseBuilder[T] {
	return &ResponseBuilder[T]{
		response: &APIResponse[T]{
			Type:      ResponseTypeSuccess,
			Success:   true,
			Timestamp: time.Now(),
			RequestID: generateRequestID(),
		},
	}
}

// Success 设置成功响应数据
func (rb *ResponseBuilder[T]) Success(data T) *ResponseBuilder[T] {
	rb.response.Type = ResponseTypeSuccess
	rb.response.Success = true
	rb.response.Data = data
	return rb
}

// Error 设置错误响应
func (rb *ResponseBuilder[T]) Error(err error) *ResponseBuilder[T] {
	rb.response.Type = ResponseTypeError
	rb.response.Success = false

	if appErr, ok := err.(*AppError); ok {
		rb.response.Error = &ErrorInfo{
			Code:       appErr.Code,
			Message:    appErr.Message,
			Details:    appErr.Details,
			Operation:  appErr.Operation,
			TraceID:    appErr.TraceID,
			Severity:   appErr.Severity,
			Retryable:  appErr.Retryable,
			Context:    appErr.Context,
		}

		if appErr.RetryDelay > 0 {
			rb.response.Error.RetryDelay = &appErr.RetryDelay
		}

		// 设置请求ID
		if appErr.TraceID != "" {
			rb.response.RequestID = appErr.TraceID
		}
	} else {
		// 处理标准错误
		rb.response.Error = &ErrorInfo{
			Code:    ErrCodeInternal,
			Message: err.Error(),
			Details: "An unexpected error occurred",
		}
	}

	return rb
}

// WithRequestID 设置请求ID
func (rb *ResponseBuilder[T]) WithRequestID(requestID string) *ResponseBuilder[T] {
	rb.response.RequestID = requestID
	return rb
}

// WithMetadata 设置元数据
func (rb *ResponseBuilder[T]) WithMetadata(metadata *Metadata) *ResponseBuilder[T] {
	rb.response.Metadata = metadata
	if metadata.RequestID == "" {
		metadata.RequestID = rb.response.RequestID
	}
	metadata.Timestamp = time.Now()
	return rb
}

// WithPagination 设置分页信息
func (rb *ResponseBuilder[T]) WithPagination(pagination *Pagination) *ResponseBuilder[T] {
	if rb.response.Metadata == nil {
		rb.response.Metadata = &Metadata{}
	}
	rb.response.Metadata.Pagination = pagination
	return rb
}

// WithPerformance 设置性能信息
func (rb *ResponseBuilder[T]) WithPerformance(performance *Performance) *ResponseBuilder[T] {
	if rb.response.Metadata == nil {
		rb.response.Metadata = &Metadata{}
	}
	rb.response.Metadata.Performance = performance
	return rb
}

// WithVersion 设置版本信息
func (rb *ResponseBuilder[T]) WithVersion(version string) *ResponseBuilder[T] {
	if rb.response.Metadata == nil {
		rb.response.Metadata = &Metadata{}
	}
	rb.response.Metadata.Version = version
	return rb
}

// Build 构建最终响应
func (rb *ResponseBuilder[T]) Build() *APIResponse[T] {
	if rb.response.Metadata != nil {
		rb.response.Metadata.Timestamp = time.Now()
		if rb.response.Metadata.Duration == 0 {
			rb.response.Metadata.Duration = time.Since(rb.response.Timestamp)
		}
	}
	return rb.response
}

// WriteJSON 写入JSON响应到Gin上下文
func (rb *ResponseBuilder[T]) WriteJSON(c *gin.Context) {
	response := rb.Build()

	// 设置状态码
	if !response.Success && response.Error != nil {
		c.Status(GetHTTPStatusCode(response.Error.Code))
	} else {
		c.Status(http.StatusOK)
	}

	// 设置请求ID到响应头
	if response.RequestID != "" {
		c.Header("X-Request-ID", response.RequestID)
	}

	// 设置内容类型
	c.Header("Content-Type", "application/json; charset=utf-8")

	c.JSON(c.Writer.Status(), response)
}

// generateRequestID 生成请求ID
func generateRequestID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString 生成随机字符串
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// SuccessResponse 创建成功响应的便捷函数
func SuccessResponse[T any](data T) *APIResponse[T] {
	return NewResponseBuilder[T]().Success(data).Build()
}

// ErrorResponse 创建错误响应的便捷函数
func ErrorResponse(err error) *APIResponse[any] {
	return NewResponseBuilder[any]().Error(err).Build()
}

// PaginatedResponse 创建分页响应的便捷函数
func PaginatedResponse[T any](data T, pagination *Pagination) *APIResponse[T] {
	return NewResponseBuilder[T]().
		Success(data).
		WithPagination(pagination).
		Build()
}

// HTTP handler helpers

// SendSuccess 发送成功响应
func SendSuccess(c *gin.Context, data interface{}) {
	NewResponseBuilder[any]().Success(data).WriteJSON(c)
}

// SendError 发送错误响应
func SendError(c *gin.Context, err error) {
	NewResponseBuilder[any]().Error(err).WriteJSON(c)
}

// SendPaginated 发送分页响应
func SendPaginated(c *gin.Context, data interface{}, pagination *Pagination) {
	NewResponseBuilder[any]().
		Success(data).
		WithPagination(pagination).
		WriteJSON(c)
}

// SendPaginatedWithMetadata 发送带元数据的分页响应
func SendPaginatedWithMetadata(c *gin.Context, data interface{}, pagination *Pagination, metadata *Metadata) {
	NewResponseBuilder[any]().
		Success(data).
		WithPagination(pagination).
		WithMetadata(metadata).
		WriteJSON(c)
}

// 响应中间件
func ResponseMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 设置请求ID
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)

		// 记录开始时间
		c.Set("start_time", time.Now())

		// 处理请求
		c.Next()

		// 如果还没有响应且发生了错误，自动发送错误响应
		if c.Writer.Status() == 0 && c.Errors.Last() != nil {
			SendError(c, c.Errors.Last().Err)
			c.Abort()
		}
	}
}

// 错误处理中间件
func ErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// 处理错误
		for _, err := range c.Errors {
			// 如果是AppError，已经处理过了
			if _, ok := err.Err.(*AppError); ok {
				continue
			}

			// 转换为AppError
			appErr := Wrap(err.Err, c.Request.URL.Path)
			if c.Writer.Status() == 0 {
				SendError(c, appErr)
			}
		}
	}
}

// 全局错误处理函数
func GlobalErrorHandler(c *gin.Context) {
	if c.Writer.Status() == 0 {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Internal server error",
		})
	}
}