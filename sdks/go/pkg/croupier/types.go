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
	ID           string   `json:"id"`            // function id, e.g. "player.ban"
	Version      string   `json:"version"`       // semver, e.g. "1.2.0"
	Tags         []string `json:"tags"`          // tags for grouping and search
	Summary      string   `json:"summary"`       // short summary for catalogs and search
	Description  string   `json:"description"`   // detailed description, supports Markdown
	OperationID  string   `json:"operation_id"`  // stable operation identifier
	Deprecated   bool     `json:"deprecated"`    // whether this function is deprecated
	InputSchema  string   `json:"input_schema"`  // JSON Schema for request body validation
	OutputSchema string   `json:"output_schema"` // JSON Schema for response body validation
	Resource     string   `json:"resource"`      // business resource/capability key
	Operation    string   `json:"operation"`     // business action key, e.g. "ban", "send", "list"
	Capability   string   `json:"capability"`    // collection_query|item_query|create|update|delete|action|task|report
	Execution    string   `json:"execution"`     // sync|task
	Risk         string   `json:"risk"`          // "safe"|"warning"|"high"|"danger"
	Permission   string   `json:"permission"`    // optional permission identifier
	Enabled      bool     `json:"enabled"`       // whether this function is currently enabled
}

// ProviderFunctionDescriptor defines a function descriptor for SDK->Agent registration
// Aligned with OpenAPI 3.0.3 Operation Object specification
// See: proto/croupier/sdk/v1/provider.proto
type ProviderFunctionDescriptor struct {
	// Core identity fields
	ID      string `json:"id"`      // Unique function identifier (e.g., "player.ban")
	Version string `json:"version"` // Function version (semver, e.g., "1.0.0")

	// OpenAPI 3.0.3 Operation Object fields
	Tags        []string `json:"tags"`         // Tags for grouping operations
	Summary     string   `json:"summary"`      // Short summary (1-2 sentences)
	Description string   `json:"description"`  // Detailed description (supports markdown)
	OperationID string   `json:"operation_id"` // Unique operation ID for OpenAPI docs
	Deprecated  bool     `json:"deprecated"`   // Whether this function is deprecated

	// OpenAPI 3.0.3 Schema fields (JSON Schema format)
	InputSchema  string `json:"input_schema"`  // JSON Schema for request body validation
	OutputSchema string `json:"output_schema"` // JSON Schema for response body validation

	// Croupier capability fields.
	Resource   string `json:"resource"`   // x-resource: business resource/capability key
	Operation  string `json:"operation"`  // x-operation: business action key
	Capability string `json:"capability"` // x-capability: collection_query|item_query|create|update|delete|action|task|report
	Execution  string `json:"execution"`  // x-execution: sync|task
	Risk       string `json:"risk"`       // x-risk: risk level ("safe", "warning", "high", "danger")
	Enabled    bool   `json:"enabled"`    // x-enabled: whether this function is enabled
	Permission string `json:"permission"` // x-permission: optional permission identifier
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
	AgentAddr    string `json:"agent_addr"`     // Agent address, e.g. "localhost:19090" or "ipc://croupier-agent,localhost:19090"
	AgentIPCAddr string `json:"agent_ipc_addr"` // IPC address for local high-performance connection (e.g., "ipc://croupier-agent")

	// Service identification (single-company, multi-game scope)
	GameID         string `json:"game_id"`         // game identifier for business scope isolation
	Env            string `json:"env"`             // logical environment: "development"|"staging"|"production"
	ServiceID      string `json:"service_id"`      // unique service identifier
	ServiceVersion string `json:"service_version"` // service version for compatibility
	AgentID        string `json:"agent_id"`        // agent identifier for load balancing
	ProviderLang   string `json:"provider_lang"`   // language reported via ProviderMeta
	ProviderSDK    string `json:"provider_sdk"`    // sdk identifier reported via ProviderMeta

	// Control plane settings
	ControlAddr string `json:"control_addr"` // optional control-plane address for manifest upload

	// Connection settings
	TimeoutSeconds    int  `json:"timeout_seconds"`    // connection timeout in seconds
	HeartbeatInterval int  `json:"heartbeat_interval"` // heartbeat interval in seconds
	Insecure          bool `json:"insecure"`           // use insecure connection (for development)

	// TLS settings (when not insecure)
	CAFile     string `json:"ca_file"`     // CA certificate file path
	CertFile   string `json:"cert_file"`   // client certificate file path
	KeyFile    string `json:"key_file"`    // client private key file path
	ServerName string `json:"server_name"` // override TLS server name verification

	// TLS verification settings (when not insecure)
	InsecureSkipVerify bool `json:"insecure_skip_verify"` // skip TLS verification (not recommended)

	// Authentication settings
	AuthToken string            `json:"auth_token"` // Bearer token for authentication
	Headers   map[string]string `json:"headers"`    // additional headers

	// Logging settings
	DisableLogging bool   `json:"disable_logging"` // Disable all logging output
	DebugLogging   bool   `json:"debug_logging"`   // Enable debug logging
	LogLevel       string `json:"log_level"`       // Log level: "DEBUG", "INFO", "WARN", "ERROR", "OFF"

	// Resiliency settings
	Reconnect *ReconnectConfig `json:"reconnect"` // reconnection configuration
	Retry     *RetryConfig     `json:"retry"`     // retry configuration

	// File transfer settings
	EnableFileTransfer bool `json:"enable_file_transfer"` // enable file transfer
	MaxFileSize        int  `json:"max_file_size"`        // max file size in bytes
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
		AgentAddr:          "localhost:19090",
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
	Enabled           bool    `json:"enabled"`            // enable automatic reconnection
	MaxAttempts       int     `json:"max_attempts"`       // max reconnection attempts (0 = infinite)
	InitialDelayMs    int     `json:"initial_delay_ms"`   // initial reconnection delay in milliseconds
	MaxDelayMs        int     `json:"max_delay_ms"`       // maximum reconnection delay in milliseconds
	BackoffMultiplier float64 `json:"backoff_multiplier"` // exponential backoff multiplier
	JitterFactor      float64 `json:"jitter_factor"`      // jitter factor (0-1) to add randomness
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
	Enabled              bool    `json:"enabled"`                // enable retry on failure
	MaxAttempts          int     `json:"max_attempts"`           // max retry attempts
	InitialDelayMs       int     `json:"initial_delay_ms"`       // initial retry delay in milliseconds
	MaxDelayMs           int     `json:"max_delay_ms"`           // maximum retry delay in milliseconds
	BackoffMultiplier    float64 `json:"backoff_multiplier"`     // exponential backoff multiplier
	JitterFactor         float64 `json:"jitter_factor"`          // jitter factor (0-1) to add randomness
	RetryableStatusCodes []int32 `json:"retryable_status_codes"` // HTTP status codes that trigger retry
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

// InvokerConfig holds configuration for the Croupier invoker
type InvokerConfig struct {
	Address        string           `json:"address"`         // server/agent address
	TimeoutSeconds int              `json:"timeout_seconds"` // request timeout in seconds
	Insecure       bool             `json:"insecure"`        // use insecure connection (skip TLS verification)
	CAFile         string           `json:"ca_file"`         // CA certificate file
	CertFile       string           `json:"cert_file"`       // client certificate file
	KeyFile        string           `json:"key_file"`        // client private key file
	DefaultTimeout time.Duration    `json:"-"`               // computed timeout duration
	Reconnect      *ReconnectConfig `json:"reconnect"`       // reconnection configuration
	Retry          *RetryConfig     `json:"retry"`           // retry configuration
}

// InvokeOptions provides options for function invocation
type InvokeOptions struct {
	IdempotencyKey string            `json:"idempotency_key"` // idempotency key to prevent duplicate execution
	Timeout        time.Duration     `json:"timeout"`         // request timeout
	Headers        map[string]string `json:"headers"`         // custom headers
	Retry          *RetryConfig      `json:"retry"`           // retry configuration override
}

// TaskEvent represents a task execution event
type TaskEvent struct {
	EventType string `json:"event_type"` // "started"|"progress"|"completed"|"error"
	TaskID    string `json:"task_id"`    // task identifier
	Payload   string `json:"payload"`    // event payload (JSON)
	Error     string `json:"error"`      // error message (if any)
	Done      bool   `json:"done"`       // whether the task is complete
}
