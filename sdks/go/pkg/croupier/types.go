// Package croupier provides a Go SDK for Croupier game function registration and execution.
package croupier

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"
)

// FunctionDescriptor defines the executable function capability contract.
type FunctionDescriptor struct {
	ID                string   `json:"id"`           // function id, e.g. "player.ban"
	Version           string   `json:"version"`      // semver, e.g. "1.2.0"
	Tags              []string `json:"tags"`         // tags for grouping and search
	Summary           string   `json:"summary"`      // short summary for catalogs and search
	Description       string   `json:"description"`  // detailed description, supports Markdown
	OperationID       string   `json:"operationId"`  // stable operation identifier
	Deprecated        bool     `json:"deprecated"`   // whether this function is deprecated
	InputSchema       string   `json:"inputSchema"`  // JSON Schema for request body validation
	OutputSchema      string   `json:"outputSchema"` // JSON Schema for response body validation
	Resource          string   `json:"resource"`     // business resource/capability key
	Operation         string   `json:"operation"`    // business action key, e.g. "ban", "send", "list"
	Capability        string   `json:"capability"`   // collection_query|item_query|create|update|delete|action|task|report
	Execution         string   `json:"execution"`    // sync|task
	ApprovalRequired  bool     `json:"approvalRequired"`
	ApprovalPolicyKey string   `json:"approvalPolicyKey"`
	Risk              string   `json:"risk"`       // "safe"|"warning"|"high"|"danger"
	Permission        string   `json:"permission"` // optional permission identifier
	Enabled           bool     `json:"enabled"`    // whether this function is currently enabled
}

// ProviderFunctionDescriptor defines a function descriptor for SDK->Agent registration
// Aligned with OpenAPI 3.0.3 Operation Object specification
// See: proto/croupier/sdk/v1/provider.proto
type ProviderFunctionDescriptor struct {
	// Core identity fields
	ID      string `json:"id"`      // Unique function identifier (e.g., "player.ban")
	Version string `json:"version"` // Function version (semver, e.g., "1.0.0")

	// OpenAPI 3.0.3 Operation Object fields
	Tags        []string `json:"tags"`        // Tags for grouping operations
	Summary     string   `json:"summary"`     // Short summary (1-2 sentences)
	Description string   `json:"description"` // Detailed description (supports markdown)
	OperationID string   `json:"operationId"` // Unique operation ID for OpenAPI docs
	Deprecated  bool     `json:"deprecated"`  // Whether this function is deprecated

	// OpenAPI 3.0.3 Schema fields (JSON Schema format)
	InputSchema  string `json:"inputSchema"`  // JSON Schema for request body validation
	OutputSchema string `json:"outputSchema"` // JSON Schema for response body validation

	// Croupier capability fields.
	Resource          string `json:"resource"`          // x-resource: business resource/capability key
	Operation         string `json:"operation"`         // x-operation: business action key
	Capability        string `json:"capability"`        // x-capability: collection_query|item_query|create|update|delete|action|task|report
	Execution         string `json:"execution"`         // x-execution: sync|task
	ApprovalRequired  bool   `json:"approvalRequired"`  // approval is independent of execution mode
	ApprovalPolicyKey string `json:"approvalPolicyKey"` // optional approval workflow key
	Risk              string `json:"risk"`              // x-risk: risk level ("safe", "warning", "high", "danger")
	Enabled           bool   `json:"enabled"`           // x-enabled: whether this function is enabled
	Permission        string `json:"permission"`        // x-permission: optional permission identifier
}

// FunctionHandler defines the signature for game function handlers
// Use []byte payloads to be language-agnostic and align with wire formats.
type FunctionHandler func(ctx context.Context, payload []byte) ([]byte, error)

// FunctionDescriptorProvider is an optional interface that function handlers can implement
// to provide additional metadata about themselves.
// Note: Since FunctionHandler is a function type, handlers cannot directly implement interfaces.
// Users should use RegisterFunctionWithDescriptor to register functions with descriptors.
type FunctionDescriptorProvider interface {
	GetDescriptor() *ProviderFunctionDescriptor
}

// HandlerWithDescriptor wraps a function handler with its descriptor
type HandlerWithDescriptor struct {
	Handler    FunctionHandler
	Descriptor *ProviderFunctionDescriptor
}

// Handle implements the FunctionHandler signature
func (hwd *HandlerWithDescriptor) Handle(ctx context.Context, payload []byte) ([]byte, error) {
	return hwd.Handler(ctx, payload)
}

// GetDescriptor returns the associated descriptor
func (hwd *HandlerWithDescriptor) GetDescriptor() *ProviderFunctionDescriptor {
	return hwd.Descriptor
}

// ClientConfig holds configuration for the Croupier client
type ClientConfig struct {
	// Agent connection settings
	AgentAddr    string `json:"agentAddr"`    // Agent local SDK gateway, e.g. "localhost:19091" or "ipc://croupier-agent,localhost:19091"
	AgentIPCAddr string `json:"agentIpcAddr"` // IPC address for local high-performance connection (e.g., "ipc://croupier-agent")

	// Service identification (single-company, multi-game scope)
	GameID         string `json:"gameId"`         // game identifier for business scope isolation
	Env            string `json:"env"`            // logical environment: "development"|"staging"|"production"
	ServiceID      string `json:"serviceId"`      // unique service identifier
	ServiceVersion string `json:"serviceVersion"` // service version for compatibility
	AgentID        string `json:"agentId"`        // agent identifier for load balancing
	ProviderLang   string `json:"providerLang"`   // language reported via ProviderMeta
	ProviderSDK    string `json:"providerSdk"`    // sdk identifier reported via ProviderMeta

	// Control plane settings
	ControlAddr string `json:"controlAddr"` // optional control-plane address for manifest upload

	// Connection settings
	TimeoutSeconds    int  `json:"timeoutSeconds"`    // connection timeout in seconds
	HeartbeatInterval int  `json:"heartbeatInterval"` // heartbeat interval in seconds
	Insecure          bool `json:"insecure"`          // use insecure connection (for development)

	// TLS settings (when not insecure)
	CAFile     string `json:"caFile"`     // CA certificate file path
	CertFile   string `json:"certFile"`   // client certificate file path
	KeyFile    string `json:"keyFile"`    // client private key file path
	ServerName string `json:"serverName"` // override TLS server name verification

	// TLS verification settings (when not insecure)
	InsecureSkipVerify bool `json:"insecureSkipVerify"` // skip TLS verification (not recommended)

	// Authentication settings
	AuthToken string            `json:"authToken"` // Bearer token for authentication
	Headers   map[string]string `json:"headers"`   // additional headers

	// Logging settings
	DisableLogging bool   `json:"disableLogging"` // Disable all logging output
	DebugLogging   bool   `json:"debugLogging"`   // Enable debug logging
	LogLevel       string `json:"logLevel"`       // Log level: "DEBUG", "INFO", "WARN", "ERROR", "OFF"

	// Resiliency settings
	Reconnect *ReconnectConfig `json:"reconnect"` // reconnection configuration
	Retry     *RetryConfig     `json:"retry"`     // retry configuration

	// File transfer settings
	EnableFileTransfer bool `json:"enableFileTransfer"` // enable file transfer
	MaxFileSize        int  `json:"maxFileSize"`        // max file size in bytes
}

// generateUUID generates a random UUID-like string using crypto/rand
func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// DefaultClientConfig returns a default client configuration
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		ServiceID:          fmt.Sprintf("go-sdk-%s", generateUUID()),
		AgentAddr:          "localhost:19091",
		Env:                "development",
		ServiceVersion:     "1.0.0",
		TimeoutSeconds:     30,
		HeartbeatInterval:  60,
		Insecure:           true, // Default to insecure for development
		Headers:            map[string]string{},
		ProviderLang:       "go",
		ProviderSDK:        "croupier-go-sdk",
		LogLevel:           "INFO",
		Reconnect:          DefaultReconnectConfig(),
		Retry:              DefaultRetryConfig(),
		EnableFileTransfer: false,
		MaxFileSize:        10 * 1024 * 1024,
	}
}

// ReconnectConfig holds configuration for automatic reconnection with exponential backoff
type ReconnectConfig struct {
	Enabled           bool    `json:"enabled"`           // enable automatic reconnection
	MaxAttempts       int     `json:"maxAttempts"`       // max reconnection attempts (0 = infinite)
	InitialDelayMs    int     `json:"initialDelayMs"`    // initial reconnection delay in milliseconds
	MaxDelayMs        int     `json:"maxDelayMs"`        // maximum reconnection delay in milliseconds
	BackoffMultiplier float64 `json:"backoffMultiplier"` // exponential backoff multiplier
	JitterFactor      float64 `json:"jitterFactor"`      // jitter factor (0-1) to add randomness
}

// DefaultReconnectConfig returns a default reconnection configuration
func DefaultReconnectConfig() *ReconnectConfig {
	return &ReconnectConfig{
		Enabled:           true,
		MaxAttempts:       0,     // 0 = infinite retries
		InitialDelayMs:    1000,  // 1 second
		MaxDelayMs:        30000, // 30 seconds
		BackoffMultiplier: 2.0,   // double each time
		JitterFactor:      0.2,   // 20% jitter
	}
}

// RetryConfig holds configuration for retrying failed invocations with exponential backoff
type RetryConfig struct {
	Enabled              bool    `json:"enabled"`              // enable retry on failure
	MaxAttempts          int     `json:"maxAttempts"`          // max retry attempts
	InitialDelayMs       int     `json:"initialDelayMs"`       // initial retry delay in milliseconds
	MaxDelayMs           int     `json:"maxDelayMs"`           // maximum retry delay in milliseconds
	BackoffMultiplier    float64 `json:"backoffMultiplier"`    // exponential backoff multiplier
	JitterFactor         float64 `json:"jitterFactor"`         // jitter factor (0-1) to add randomness
	RetryableStatusCodes []int32 `json:"retryableStatusCodes"` // HTTP status codes that trigger retry
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		Enabled:           true,
		MaxAttempts:       3,
		InitialDelayMs:    100,  // 100ms
		MaxDelayMs:        5000, // 5 seconds
		BackoffMultiplier: 2.0,  // double each time
		JitterFactor:      0.1,  // 10% jitter
		RetryableStatusCodes: []int32{
			14, // UNAVAILABLE
			13, // INTERNAL
			2,  // UNKNOWN
			10, // ABORTED
			4,  // DEADLINE_EXCEEDED
		},
	}
}

// InvokerConfig holds configuration for the independent Server HTTP invoker.
type InvokerConfig struct {
	Address          string           `json:"address"`          // Server HTTP API URL, e.g. https://server.example/api/v1
	AuthToken        string           `json:"authToken"`        // optional Bearer token for Server API authentication
	GameID           string           `json:"gameId"`           // optional default Server game scope
	Env              string           `json:"env"`              // optional default Server environment scope
	TaskPollInterval time.Duration    `json:"taskPollInterval"` // interval for polling Server task events
	TimeoutSeconds   int              `json:"timeoutSeconds"`   // request timeout in seconds
	Insecure         bool             `json:"insecure"`         // use insecure connection (skip TLS verification)
	CAFile           string           `json:"caFile"`           // CA certificate file
	CertFile         string           `json:"certFile"`         // client certificate file
	KeyFile          string           `json:"keyFile"`          // client private key file
	DefaultTimeout   time.Duration    `json:"-"`                // computed timeout duration
	Reconnect        *ReconnectConfig `json:"reconnect"`        // reconnection configuration
	Retry            *RetryConfig     `json:"retry"`            // retry configuration
}

// InvokeOptions provides options for function invocation
type InvokeOptions struct {
	IdempotencyKey string            `json:"idempotencyKey"` // idempotency key to prevent duplicate execution
	Timeout        time.Duration     `json:"timeout"`        // request timeout
	Headers        map[string]string `json:"headers"`        // custom headers
	Retry          *RetryConfig      `json:"retry"`          // retry configuration override
}

// TaskEvent represents a task execution event
type TaskEvent struct {
	EventType string `json:"eventType"` // "started"|"progress"|"completed"|"error"
	TaskID    string `json:"taskId"`    // task identifier
	Payload   string `json:"payload"`   // event payload (JSON)
	Error     string `json:"error"`     // error message (if any)
	Done      bool   `json:"done"`      // whether the task is complete
}

// TaskStatus is the Server-persisted state returned by GET /api/v1/tasks/:id.
// Result is the raw JSON result emitted by the task, if one is available.
type TaskStatus struct {
	TaskID     string `json:"id"`
	FunctionID string `json:"functionId,omitempty"`
	Status     string `json:"status"`
	Progress   int32  `json:"progress,omitempty"`
	Message    string `json:"message,omitempty"`
	GameID     string `json:"gameId,omitempty"`
	Env        string `json:"env,omitempty"`
	AgentID    string `json:"agentId,omitempty"`
	Actor      string `json:"actor,omitempty"`
	TraceID    string `json:"traceId,omitempty"`
	Result     string `json:"result,omitempty"`
	Error      string `json:"error,omitempty"`
	StartedAt  string `json:"startedAt,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
	CreatedAt  string `json:"createdAt,omitempty"`
	UpdatedAt  string `json:"updatedAt,omitempty"`
}
