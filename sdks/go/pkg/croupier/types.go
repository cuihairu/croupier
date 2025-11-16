// Package croupier provides a Go SDK for Croupier game function registration and execution.
package croupier

import (
	"context"
	"time"
)

// FunctionDescriptor defines a game function descriptor aligned with control.proto
type FunctionDescriptor struct {
	ID        string `json:"id"`        // function id, e.g. "player.ban"
	Version   string `json:"version"`   // semver, e.g. "1.2.0"
	Category  string `json:"category"`  // grouping category
	Risk      string `json:"risk"`      // "low"|"medium"|"high"
	Entity    string `json:"entity"`    // entity type, e.g. "item", "player"
	Operation string `json:"operation"` // operation type, e.g. "create", "read", "update", "delete"
	Enabled   bool   `json:"enabled"`   // whether this function is currently enabled
}

// LocalFunctionDescriptor defines a local function descriptor for SDK->Agent registration
// Aligned with agent/local/v1/local.proto
type LocalFunctionDescriptor struct {
	ID      string `json:"id"`      // function id
	Version string `json:"version"` // function version
}

// FunctionHandler defines the signature for game function handlers
// Use []byte payloads to be language-agnostic and align with wire formats.
type FunctionHandler func(ctx context.Context, payload []byte) ([]byte, error)

// ClientConfig holds configuration for the Croupier client
type ClientConfig struct {
	// Agent connection settings
	AgentAddr string `json:"agent_addr"` // Agent gRPC address, e.g. "localhost:19090"

	// Service identification (multi-tenant support)
	GameID         string `json:"game_id"`          // game identifier for tenant isolation
	Env            string `json:"env"`              // environment: "development"|"staging"|"production"
	ServiceID      string `json:"service_id"`       // unique service identifier
	ServiceVersion string `json:"service_version"`  // service version for compatibility
	AgentID        string `json:"agent_id"`         // agent identifier for load balancing

	// Local server settings
	LocalListen string `json:"local_listen"` // local gRPC listener address

	// Connection settings
	TimeoutSeconds int  `json:"timeout_seconds"` // connection timeout in seconds
	Insecure       bool `json:"insecure"`        // use insecure gRPC (for development)

	// TLS settings (when not insecure)
	CAFile   string `json:"ca_file"`   // CA certificate file path
	CertFile string `json:"cert_file"` // client certificate file path
	KeyFile  string `json:"key_file"`  // client private key file path
}

// DefaultClientConfig returns a default client configuration
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		AgentAddr:      "localhost:19090",
		Env:            "development",
		ServiceVersion: "1.0.0",
		TimeoutSeconds: 30,
		Insecure:       true, // Default to insecure for development
	}
}

// InvokerConfig holds configuration for the Croupier invoker
type InvokerConfig struct {
	Address        string        `json:"address"`         // server/agent address
	TimeoutSeconds int           `json:"timeout_seconds"` // request timeout in seconds
	Insecure       bool          `json:"insecure"`        // use insecure gRPC
	CAFile         string        `json:"ca_file"`         // CA certificate file
	CertFile       string        `json:"cert_file"`       // client certificate file
	KeyFile        string        `json:"key_file"`        // client private key file
	DefaultTimeout time.Duration `json:"-"`               // computed timeout duration
}

// InvokeOptions provides options for function invocation
type InvokeOptions struct {
	IdempotencyKey string            `json:"idempotency_key"` // idempotency key to prevent duplicate execution
	Timeout        time.Duration     `json:"timeout"`         // request timeout
	Headers        map[string]string `json:"headers"`         // custom headers
}

// JobEvent represents a job execution event
type JobEvent struct {
	EventType string `json:"event_type"` // "started"|"progress"|"completed"|"error"
	JobID     string `json:"job_id"`     // job identifier
	Payload   string `json:"payload"`    // event payload (JSON)
	Error     string `json:"error"`      // error message (if any)
	Done      bool   `json:"done"`       // whether the job is complete
}
