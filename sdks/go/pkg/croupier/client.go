// Package croupier provides a Go SDK for Croupier game function registration and execution.
package croupier

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
)

// Client represents a Croupier client for function registration and execution
type Client interface {
	// RegisterFunction registers a single function with the agent
	RegisterFunction(desc FunctionDescriptor, handler FunctionHandler) error

	// Connect establishes connection to the agent
	Connect(ctx context.Context) error

	// Serve starts the local server and maintains the connection
	Serve(ctx context.Context) error

	// Stop gracefully stops the client
	Stop() error

	// Close closes the client and cleans up resources
	Close() error
}

// Invoker represents a Croupier invoker for calling functions
type Invoker interface {
	// Connect establishes connection to the server/agent
	Connect(ctx context.Context) error

	// Invoke synchronously calls a function
	Invoke(ctx context.Context, functionID, payload string, options InvokeOptions) (string, error)

	// StartJob starts an asynchronous job
	StartJob(ctx context.Context, functionID, payload string, options InvokeOptions) (string, error)

	// StreamJob streams events from a running job
	StreamJob(ctx context.Context, jobID string) (<-chan JobEvent, error)

	// CancelJob cancels a running job
	CancelJob(ctx context.Context, jobID string) error

	// SetSchema sets validation schema for a function
	SetSchema(functionID string, schema map[string]interface{}) error

	// Close closes the invoker
	Close() error
}

// client implements the Client interface
type client struct {
	config      *ClientConfig
	handlers    map[string]FunctionHandler
	descriptors map[string]FunctionDescriptor
	mu          sync.RWMutex

	// Manager (TCP-based)
	manager   Manager
	sessionID string

	// State management (using atomic.Bool for concurrent access)
	connected atomic.Bool
	running   atomic.Bool
	stopCh    chan struct{}

	// Logging
	logger Logger
}

// NewClient creates a new Croupier client
func NewClient(config *ClientConfig) Client {
	if config == nil {
		config = DefaultClientConfig()
	}

	// Set up logger based on config
	var l Logger
	if config.DisableLogging {
		l = &NoOpLogger{}
	} else {
		l = NewDefaultLogger(config.DebugLogging, os.Stdout)
	}

	return &client{
		config:      config,
		handlers:    make(map[string]FunctionHandler),
		descriptors: make(map[string]FunctionDescriptor),
		stopCh:      make(chan struct{}),
		logger:      l,
	}
}

// RegisterFunction implements Client.RegisterFunction
func (c *client) RegisterFunction(desc FunctionDescriptor, handler FunctionHandler) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running.Load() {
		return fmt.Errorf("cannot register functions while client is running")
	}

	// Validate function descriptor
	if desc.ID == "" {
		return fmt.Errorf("function ID cannot be empty")
	}
	if desc.Version == "" {
		desc.Version = "1.0.0"
	}

	// Check if client has been closed
	if c.handlers == nil {
		return fmt.Errorf("client has been closed")
	}

	c.handlers[desc.ID] = handler
	c.descriptors[desc.ID] = desc
	c.logger.Infof("Registered function: %s (version: %s)", desc.ID, desc.Version)
	return nil
}

// Connect implements Client.Connect
func (c *client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected.Load() {
		return nil
	}

	c.logger.Infof("Connecting to Croupier Agent: %s", c.config.AgentAddr)

	// Create TCP manager
	managerConfig := ManagerConfig{
		AgentAddr:          c.config.AgentAddr,
		ControlAddr:        c.config.ControlAddr,
		TimeoutSeconds:     c.config.TimeoutSeconds,
		Insecure:           c.config.Insecure,
		CAFile:             c.config.CAFile,
		CertFile:           c.config.CertFile,
		KeyFile:            c.config.KeyFile,
		ServerName:         c.config.ServerName,
		ProviderLang:       c.config.ProviderLang,
		ProviderSDK:        c.config.ProviderSDK,
		InsecureSkipVerify: c.config.InsecureSkipVerify,
	}

	var err error
	c.manager, err = NewManager(managerConfig, c.handlers)
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	// Connect to agent
	if err := c.manager.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to agent: %w", err)
	}

	// Register functions with agent
	localFunctions := c.convertToLocalFunctions()
	sessionID, err := c.manager.RegisterWithAgent(ctx, c.config.ServiceID, c.config.ServiceVersion, localFunctions)
	if err != nil {
		c.manager.Disconnect()
		return fmt.Errorf("failed to register with agent: %w", err)
	}

	c.sessionID = sessionID
	c.connected.Store(true)

	c.logger.Infof("Successfully connected and registered with Agent")
	c.logger.Infof("Session ID: %s", c.sessionID)

	return nil
}

// Serve implements Client.Serve
func (c *client) Serve(ctx context.Context) error {
	if !c.connected.Load() {
		if err := c.Connect(ctx); err != nil {
			return fmt.Errorf("connection failed, cannot start service: %w", err)
		}
	}

	c.running.Store(true)
	c.logger.Infof("Croupier client service started")
	c.logger.Infof("Registered functions: %d", len(c.handlers))
	c.logger.Infof("Use Stop() to stop the service")
	c.logger.Infof("===============================================")

	// Wait for stop signal or context cancellation
	select {
	case <-c.stopCh:
		c.logger.Infof("Service stopped by Stop() call")
	case <-ctx.Done():
		c.logger.Infof("Service stopped by context cancellation")
	}

	c.running.Store(false)
	c.logger.Infof("Service has stopped")
	return nil
}

// Stop implements Client.Stop
func (c *client) Stop() error {
	c.running.Store(false)
	c.connected.Store(false)

	c.logger.Infof("Stopping Croupier client...")

	if c.manager != nil {
		c.manager.Disconnect()
	}

	// Safely close stopCh (only close if not already closed)
	select {
	case <-c.stopCh:
		// Channel already closed
	default:
		close(c.stopCh)
	}

	c.logger.Infof("Client stopped successfully")
	return nil
}

// Close implements Client.Close
func (c *client) Close() error {
	c.Stop()
	c.handlers = nil
	return nil
}

// convertToLocalFunctions converts FunctionDescriptors to LocalFunctionDescriptors
// Note: This method must be called while holding c.mu (either read or write lock)
func (c *client) convertToLocalFunctions() []LocalFunctionDescriptor {
	var localFuncs []LocalFunctionDescriptor
	for funcID := range c.handlers {
		desc, ok := c.descriptors[funcID]
		if !ok {
			continue
		}
		version := desc.Version
		if version == "" {
			version = "1.0.0"
		}

		// Convert FunctionDescriptor to LocalFunctionDescriptor with OpenAPI 3.0.3 fields
		localDesc := LocalFunctionDescriptor{
			ID:        funcID,
			Version:   version,
			Category:  desc.Category,
			Risk:      desc.Risk,
			Entity:    desc.Entity,
			Operation: desc.Operation,
			// Generate basic JSON Schemas (can be overridden by users via custom descriptors)
			Tags:         []string{desc.Category},
			Summary:      funcID,
			Description:  fmt.Sprintf("Function: %s", funcID),
			InputSchema:  generateBasicInputSchema(),
			OutputSchema: generateBasicOutputSchema(),
		}

		localFuncs = append(localFuncs, localDesc)
	}
	return localFuncs
}

// generateBasicInputSchema creates a basic JSON Schema for request validation
func generateBasicInputSchema() string {
	return `{
		"type": "object",
		"properties": {
			"data": {"type": "string"}
		}
	}`
}

// generateBasicOutputSchema creates a basic JSON Schema for response validation
func generateBasicOutputSchema() string {
	return `{
		"type": "object",
		"properties": {
			"result": {"type": "string"}
		}
	}`
}
