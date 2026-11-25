package errors

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// ResponseType response type
type ResponseType string

const (
	ResponseTypeSuccess ResponseType = "success"
	ResponseTypeError   ResponseType = "error"
)

// APIResponse generic API response structure
type APIResponse[T any] struct {
	Type      ResponseType `json:"type"`
	Success   bool         `json:"success"`
	Data      T            `json:"data,omitempty"`
	Error     *ErrorInfo   `json:"error,omitempty"`
	Metadata  *Metadata    `json:"metadata,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
	RequestID string       `json:"requestId"`
}

// ErrorInfo error information
type ErrorInfo struct {
	Code       ErrorCode              `json:"code"`
	Message    string                 `json:"message"`
	Details    string                 `json:"details,omitempty"`
	Operation  string                 `json:"operation,omitempty"`
	TraceID    string                 `json:"traceId,omitempty"`
	Severity   ErrorSeverity          `json:"severity,omitempty"`
	Retryable  bool                   `json:"retryable,omitempty"`
	RetryDelay *time.Duration         `json:"retryDelay,omitempty"`
	Context    map[string]interface{} `json:"context,omitempty"`
}

// Metadata response metadata
type Metadata struct {
	RequestID   string        `json:"requestId"`
	Version     string        `json:"version"`
	Duration    time.Duration `json:"duration,omitempty"`
	Timestamp   time.Time     `json:"timestamp"`
	Pagination  *Pagination   `json:"pagination,omitempty"`
	Performance *Performance  `json:"performance,omitempty"`
}

// Pagination pagination info
type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
	HasNext    bool  `json:"hasNext"`
	HasPrev    bool  `json:"hasPrev"`
}

// Performance performance info
type Performance struct {
	DatabaseQueries int           `json:"databaseQueries,omitempty"`
	DatabaseTime    time.Duration `json:"databaseTime,omitempty"`
	CacheHits       int           `json:"cacheHits,omitempty"`
	CacheMisses     int           `json:"cacheMisses,omitempty"`
	ExternalCalls   int           `json:"externalCalls,omitempty"`
	ExternalTime    time.Duration `json:"externalTime,omitempty"`
}

// ResponseBuilder response builder
type ResponseBuilder[T any] struct {
	response *APIResponse[T]
}

// NewResponseBuilder creates a response builder
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

// Success sets the success response data
func (rb *ResponseBuilder[T]) Success(data T) *ResponseBuilder[T] {
	rb.response.Type = ResponseTypeSuccess
	rb.response.Success = true
	rb.response.Data = data
	return rb
}

// Error sets the error response
func (rb *ResponseBuilder[T]) Error(err error) *ResponseBuilder[T] {
	rb.response.Type = ResponseTypeError
	rb.response.Success = false

	if appErr, ok := err.(*AppError); ok {
		rb.response.Error = &ErrorInfo{
			Code:      appErr.Code,
			Message:   appErr.Message,
			Details:   appErr.Details,
			Operation: appErr.Operation,
			TraceID:   appErr.TraceID,
			Severity:  appErr.Severity,
			Retryable: appErr.Retryable,
			Context:   appErr.Context,
		}

		if appErr.RetryDelay > 0 {
			rb.response.Error.RetryDelay = &appErr.RetryDelay
		}

		// Set the request ID
		if appErr.TraceID != "" {
			rb.response.RequestID = appErr.TraceID
		}
	} else {
		// Handle standard errors
		rb.response.Error = &ErrorInfo{
			Code:    ErrCodeInternal,
			Message: err.Error(),
			Details: "An unexpected error occurred",
		}
	}

	return rb
}

// WithRequestID sets the request ID
func (rb *ResponseBuilder[T]) WithRequestID(requestID string) *ResponseBuilder[T] {
	rb.response.RequestID = requestID
	return rb
}

// WithMetadata sets the metadata
func (rb *ResponseBuilder[T]) WithMetadata(metadata *Metadata) *ResponseBuilder[T] {
	rb.response.Metadata = metadata
	if metadata.RequestID == "" {
		metadata.RequestID = rb.response.RequestID
	}
	metadata.Timestamp = time.Now()
	return rb
}

// WithPagination sets the pagination info
func (rb *ResponseBuilder[T]) WithPagination(pagination *Pagination) *ResponseBuilder[T] {
	if rb.response.Metadata == nil {
		rb.response.Metadata = &Metadata{}
	}
	rb.response.Metadata.Pagination = pagination
	return rb
}

// WithPerformance sets the performance info
func (rb *ResponseBuilder[T]) WithPerformance(performance *Performance) *ResponseBuilder[T] {
	if rb.response.Metadata == nil {
		rb.response.Metadata = &Metadata{}
	}
	rb.response.Metadata.Performance = performance
	return rb
}

// WithVersion sets the version info
func (rb *ResponseBuilder[T]) WithVersion(version string) *ResponseBuilder[T] {
	if rb.response.Metadata == nil {
		rb.response.Metadata = &Metadata{}
	}
	rb.response.Metadata.Version = version
	return rb
}

// Build builds the final response
func (rb *ResponseBuilder[T]) Build() *APIResponse[T] {
	if rb.response.Metadata != nil {
		rb.response.Metadata.Timestamp = time.Now()
		if rb.response.Metadata.Duration == 0 {
			rb.response.Metadata.Duration = time.Since(rb.response.Timestamp)
		}
	}
	return rb.response
}

// WriteJSON writes JSON response to an HTTP ResponseWriter.
func (rb *ResponseBuilder[T]) WriteJSON(w http.ResponseWriter, r *http.Request) {
	response := rb.Build()

	// Set the status code
	status := http.StatusOK
	if !response.Success && response.Error != nil {
		status = GetHTTPStatusCode(response.Error.Code)
	}

	// Set the request ID in the response header
	if response.RequestID != "" {
		w.Header().Set("X-Request-ID", response.RequestID)
	}

	// Set the content type
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response)
}

// generateRequestID generates a request ID
func generateRequestID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString generates a random string
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// SuccessResponse is a convenience function that creates a success response
func SuccessResponse[T any](data T) *APIResponse[T] {
	return NewResponseBuilder[T]().Success(data).Build()
}

// ErrorResponse is a convenience function that creates an error response
func ErrorResponse(err error) *APIResponse[any] {
	return NewResponseBuilder[any]().Error(err).Build()
}

// PaginatedResponse is a convenience function that creates a paginated response
func PaginatedResponse[T any](data T, pagination *Pagination) *APIResponse[T] {
	return NewResponseBuilder[T]().
		Success(data).
		WithPagination(pagination).
		Build()
}

// HTTP handler helpers

// SendSuccess sends a success response
func SendSuccess(w http.ResponseWriter, r *http.Request, data interface{}) {
	NewResponseBuilder[any]().Success(data).WriteJSON(w, r)
}

// SendError sends an error response
func SendError(w http.ResponseWriter, r *http.Request, err error) {
	NewResponseBuilder[any]().Error(err).WriteJSON(w, r)
}

// SendPaginated sends a paginated response
func SendPaginated(w http.ResponseWriter, r *http.Request, data interface{}, pagination *Pagination) {
	NewResponseBuilder[any]().
		Success(data).
		WithPagination(pagination).
		WriteJSON(w, r)
}

// SendPaginatedWithMetadata sends a paginated response with metadata
func SendPaginatedWithMetadata(w http.ResponseWriter, r *http.Request, data interface{}, pagination *Pagination, metadata *Metadata) {
	NewResponseBuilder[any]().
		Success(data).
		WithPagination(pagination).
		WithMetadata(metadata).
		WriteJSON(w, r)
}

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	startTimeKey contextKey = "start_time"
)

// ResponseMiddleware injects request id and start time into context and response headers.
func ResponseMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		w.Header().Set("X-Request-ID", requestID)

		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		ctx = context.WithValue(ctx, startTimeKey, time.Now())

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
