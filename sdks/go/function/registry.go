// Package function provides a high-level API for registering functions with Croupier.
// This package uses the new FunctionMetadata proto definition for function registration.
package function

import (
	"context"
	"fmt"
	"sync"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

// FunctionMetadata defines the core function registration contract.
// This matches the proto definition in proto/croupier/function/v1/metadata.proto
type FunctionMetadata struct {
	// Identity fields
	ID      string `json:"id"`      // Unique identifier, format: <domain>.<entity>.<action>
	Version string `json:"version"` // Semantic version (semver)

	// Classification fields
	Category string   `json:"category"` // Function category
	Tags     []string `json:"tags"`     // Tags for grouping and filtering

	// Documentation fields
	Name        string `json:"name"`        // Short display name
	Description string `json:"description"` // Detailed description

	// Parameter definitions (JSON Schema)
	InputSchema  string `json:"input_schema"`  // JSON Schema for request
	OutputSchema string `json:"output_schema"` // JSON Schema for response

	// Behavior definition
	Behavior *FunctionBehavior `json:"behavior"`

	// Risk definition (function's inherent risk level)
	Risk *FunctionRisk `json:"risk"`

	// Extension fields
	Extensions map[string]string `json:"extensions"`
}

// FunctionBehavior defines how the function executes.
type FunctionBehavior struct {
	// Execution mode
	Mode Mode `json:"mode"`

	// Idempotency
	Idempotent bool `json:"idempotent"`

	// Timeout in milliseconds
	TimeoutMs int32 `json:"timeout_ms"`

	// Routing strategy
	RouteStrategy RouteStrategy `json:"route_strategy"`

	// Caching
	Cacheable       bool  `json:"cacheable"`
	CacheTtlSeconds int32 `json:"cache_ttl_seconds"`
}

// Mode represents the execution mode.
type Mode int32

const (
	ModeUnknown Mode = iota
	ModeQuery
	ModeCommand
)

// String returns the string representation of Mode.
func (m Mode) String() string {
	switch m {
	case ModeQuery:
		return "query"
	case ModeCommand:
		return "command"
	default:
		return "unknown"
	}
}

// RouteStrategy represents the routing strategy.
type RouteStrategy int32

const (
	RouteUnknown RouteStrategy = iota
	RouteLB
	RouteBroadcast
	RouteTargeted
	RouteHash
)

// String returns the string representation of RouteStrategy.
func (r RouteStrategy) String() string {
	switch r {
	case RouteLB:
		return "lb"
	case RouteBroadcast:
		return "broadcast"
	case RouteTargeted:
		return "targeted"
	case RouteHash:
		return "hash"
	default:
		return "unknown"
	}
}

// FunctionRisk defines the function's inherent risk level.
// This is a declarative metadata field; Server-side policies determine
// the actual security requirements based on this risk level.
type FunctionRisk struct {
	Level RiskLevel `json:"level"`
}

// RiskLevel represents the risk level.
type RiskLevel int32

const (
	RiskUnknown RiskLevel = iota
	RiskLow
	RiskMedium
	RiskHigh
	RiskDanger
)

// String returns the string representation of RiskLevel.
func (r RiskLevel) String() string {
	switch r {
	case RiskLow:
		return "low"
	case RiskMedium:
		return "medium"
	case RiskHigh:
		return "high"
	case RiskDanger:
		return "danger"
	default:
		return "unknown"
	}
}

// Registry provides function registration API for SDK users.
type Registry struct {
	client   croupier.Client
	metadata map[string]*FunctionMetadata
	handlers map[string]Handler
	mu       sync.RWMutex
	logger   Logger
}

// Handler defines the signature for function handlers.
// Handler receives context and input payload, returns output payload or error.
type Handler func(ctx context.Context, input []byte) ([]byte, error)

// Logger is a simple logging interface.
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// DefaultLogger implements Logger using stdout.
type DefaultLogger struct{}

// Debug logs debug messages.
func (l *DefaultLogger) Debug(msg string, args ...interface{}) {
	fmt.Printf("[DEBUG] "+msg+"\n", args...)
}

// Info logs info messages.
func (l *DefaultLogger) Info(msg string, args ...interface{}) {
	fmt.Printf("[INFO] "+msg+"\n", args...)
}

// Warn logs warning messages.
func (l *DefaultLogger) Warn(msg string, args ...interface{}) {
	fmt.Printf("[WARN] "+msg+"\n", args...)
}

// Error logs error messages.
func (l *DefaultLogger) Error(msg string, args ...interface{}) {
	fmt.Printf("[ERROR] "+msg+"\n", args...)
}

// NoOpLogger is a logger that discards all messages.
type NoOpLogger struct{}

// Debug is a no-op.
func (l *NoOpLogger) Debug(msg string, args ...interface{}) {}

// Info is a no-op.
func (l *NoOpLogger) Info(msg string, args ...interface{}) {}

// Warn is a no-op.
func (l *NoOpLogger) Warn(msg string, args ...interface{}) {}

// Error is a no-op.
func (l *NoOpLogger) Error(msg string, args ...interface{}) {}

// NewRegistry creates a new function registry with the given Croupier client.
func NewRegistry(client croupier.Client) *Registry {
	var logger Logger = &DefaultLogger{}
	if cfg := getConfig(client); cfg != nil && cfg.DisableLogging {
		logger = &NoOpLogger{}
	}

	return &Registry{
		client:   client,
		metadata: make(map[string]*FunctionMetadata),
		handlers: make(map[string]Handler),
		logger:   logger,
	}
}

// NewRegistryWithLogger creates a new function registry with a custom logger.
func NewRegistryWithLogger(client croupier.Client, logger Logger) *Registry {
	return &Registry{
		client:   client,
		metadata: make(map[string]*FunctionMetadata),
		handlers: make(map[string]Handler),
		logger:   logger,
	}
}

// Register registers a function with its metadata and handler.
func (r *Registry) Register(metadata *FunctionMetadata, handler Handler) error {
	if metadata == nil {
		return fmt.Errorf("metadata is required")
	}
	if handler == nil {
		return fmt.Errorf("handler is required")
	}
	if metadata.ID == "" {
		return fmt.Errorf("metadata.ID is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Convert to FunctionDescriptor for client registration
	desc := r.toFunctionDescriptor(metadata)
	legacyHandler := croupier.FunctionHandler(handler)

	// Register with client
	if err := r.client.RegisterFunction(desc, croupier.FunctionHandler(legacyHandler)); err != nil {
		return fmt.Errorf("client registration failed: %w", err)
	}

	// Clone metadata for storage
	cloned := r.cloneMetadata(metadata)
	r.metadata[cloned.ID] = cloned
	r.handlers[cloned.ID] = handler

	r.logger.Debug("Registered function", "id", cloned.ID, "category", cloned.Category)
	return nil
}

// RegisterFromBuilder registers a function using a MetadataBuilder.
func (r *Registry) RegisterFromBuilder(builder *MetadataBuilder, handler Handler) error {
	if builder == nil {
		return fmt.Errorf("builder is required")
	}

	metadata, err := builder.Build()
	if err != nil {
		return fmt.Errorf("build metadata failed: %w", err)
	}

	return r.Register(metadata, handler)
}

// GetHandler retrieves a handler by function ID.
func (r *Registry) GetHandler(functionID string) (Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, exists := r.handlers[functionID]
	return handler, exists
}

// GetMetadata retrieves metadata by function ID.
func (r *Registry) GetMetadata(functionID string) (*FunctionMetadata, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metadata, exists := r.metadata[functionID]
	if !exists {
		return nil, false
	}

	// Return a clone to avoid external modifications
	return r.cloneMetadata(metadata), true
}

// ListMetadata returns all registered function metadata.
func (r *Registry) ListMetadata() []*FunctionMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*FunctionMetadata, 0, len(r.metadata))
	for _, metadata := range r.metadata {
		result = append(result, r.cloneMetadata(metadata))
	}

	return result
}

// Unregister removes a function from the registry.
func (r *Registry) Unregister(functionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.metadata[functionID]; !exists {
		return fmt.Errorf("function not found: %s", functionID)
	}

	delete(r.metadata, functionID)
	delete(r.handlers, functionID)

	r.logger.Debug("Unregistered function", "id", functionID)
	return nil
}

// Count returns the number of registered functions.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.metadata)
}

// cloneMetadata creates a deep copy of FunctionMetadata.
func (r *Registry) cloneMetadata(metadata *FunctionMetadata) *FunctionMetadata {
	cloned := *metadata

	// Clone slices
	if metadata.Tags != nil {
		cloned.Tags = make([]string, len(metadata.Tags))
		copy(cloned.Tags, metadata.Tags)
	}

	if metadata.Behavior != nil {
		behaviorClone := *metadata.Behavior
		cloned.Behavior = &behaviorClone
	}

	if metadata.Risk != nil {
		riskClone := *metadata.Risk
		cloned.Risk = &riskClone
	}

	if metadata.Extensions != nil {
		cloned.Extensions = make(map[string]string)
		for k, v := range metadata.Extensions {
			cloned.Extensions[k] = v
		}
	}

	return &cloned
}

// toFunctionDescriptor converts FunctionMetadata to FunctionDescriptor for client registration.
func (r *Registry) toFunctionDescriptor(metadata *FunctionMetadata) croupier.FunctionDescriptor {
	desc := croupier.FunctionDescriptor{
		ID:       metadata.ID,
		Version:  metadata.Version,
		Category: metadata.Category,
		Enabled:  true,
	}

	if metadata.Risk != nil {
		desc.Risk = metadata.Risk.Level.String()
	}

	return desc
}

// getConfig extracts ClientConfig from a Client interface.
// This is a helper that tries to get config from the client.
func getConfig(client croupier.Client) *croupier.ClientConfig {
	// Try type assertion to access internal config
	type configProvider interface {
		Config() *croupier.ClientConfig
	}

	if cp, ok := client.(configProvider); ok {
		return cp.Config()
	}

	return nil
}

// SimpleRegister is a convenience function for quick function registration.
// Use this for simple cases where you don't need the full Registry.
func SimpleRegister(
	client croupier.Client,
	id, category, name string,
	handler Handler,
) error {
	registry := NewRegistry(client)

	metadata := &FunctionMetadata{
		ID:       id,
		Category: category,
		Name:     name,
		Behavior: &FunctionBehavior{
			Mode:      ModeQuery,
			TimeoutMs: 30000,
		},
		Risk: &FunctionRisk{
			Level: RiskLow,
		},
	}

	return registry.Register(metadata, handler)
}
