// Package croupier provides a Go SDK for Croupier game function registration and execution.
package croupier

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"
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

	// StartTask starts an asynchronous task
	StartTask(ctx context.Context, functionID, payload string, options InvokeOptions) (string, error)

	// StreamTask streams events from a running task
	StreamTask(ctx context.Context, taskID string) (<-chan TaskEvent, error)

	// CancelTask cancels a running task
	CancelTask(ctx context.Context, taskID string) error

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

	// Reconnection
	disconnectCh chan struct{}

	// Logging
	logger Logger
}

// NewClient creates a new Croupier client
func NewClient(config *ClientConfig) Client {
	if config == nil {
		config = DefaultClientConfig()
	}

	// Default reconnection config so direct ClientConfig construction still
	// benefits from automatic reconnection (DefaultClientConfig sets this,
	// but callers that build ClientConfig literals skip it).
	if config.Reconnect == nil {
		config.Reconnect = DefaultReconnectConfig()
	}

	// Set up logger based on config
	var l Logger
	if config.DisableLogging {
		l = &NoOpLogger{}
	} else {
		l = NewDefaultLogger(config.DebugLogging, os.Stdout)
	}

	return &client{
		config:       config,
		handlers:     make(map[string]FunctionHandler),
		descriptors:  make(map[string]FunctionDescriptor),
		stopCh:       make(chan struct{}),
		disconnectCh: make(chan struct{}, 1),
		logger:       l,
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

	// Set up disconnect notification for automatic reconnection
	c.manager.SetOnDisconnect(func() {
		c.connected.Store(false)
		select {
		case c.disconnectCh <- struct{}{}:
		default:
		}
	})

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

	for {
		select {
		case <-c.stopCh:
			c.logger.Infof("Service stopped by Stop() call")
			c.running.Store(false)
			return nil
		case <-ctx.Done():
			c.logger.Infof("Service stopped by context cancellation")
			c.running.Store(false)
			return nil
		case <-c.disconnectCh:
			if !c.running.Load() {
				return nil
			}
			c.logger.Warnf("Connection lost, attempting reconnection...")
			if err := c.reconnectWithBackoff(ctx); err != nil {
				c.logger.Errorf("Reconnection failed: %v", err)
				c.running.Store(false)
				return fmt.Errorf("reconnection failed: %w", err)
			}
			c.logger.Infof("Reconnection successful, resuming service")
		}
	}
}

// reconnectWithBackoff attempts to reconnect using exponential backoff.
func (c *client) reconnectWithBackoff(ctx context.Context) error {
	rc := c.config.Reconnect
	if rc == nil || !rc.Enabled {
		return fmt.Errorf("reconnection is disabled")
	}

	delay := time.Duration(rc.InitialDelayMs) * time.Millisecond
	maxDelay := time.Duration(rc.MaxDelayMs) * time.Millisecond
	attempt := 0

	for {
		attempt++
		if rc.MaxAttempts > 0 && attempt > rc.MaxAttempts {
			return fmt.Errorf("max reconnection attempts (%d) exceeded", rc.MaxAttempts)
		}

		select {
		case <-c.stopCh:
			return fmt.Errorf("stopped during reconnection")
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}

		c.logger.Infof("Reconnection attempt %d...", attempt)

		// Disconnect old manager cleanly
		c.manager.Disconnect()

		// Create new manager and connect
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
			c.logger.Warnf("Reconnect create manager failed: %v", err)
			delay = c.nextBackoffDelay(delay, maxDelay, rc)
			continue
		}

		connectCtx, connectCancel := context.WithTimeout(ctx, 30*time.Second)
		err = c.manager.Connect(connectCtx)
		connectCancel()
		if err != nil {
			c.logger.Warnf("Reconnect dial failed: %v", err)
			delay = c.nextBackoffDelay(delay, maxDelay, rc)
			continue
		}

		// Re-register functions
		localFunctions := c.convertToLocalFunctions()
		registerCtx, registerCancel := context.WithTimeout(ctx, 30*time.Second)
		sessionID, err := c.manager.RegisterWithAgent(registerCtx, c.config.ServiceID, c.config.ServiceVersion, localFunctions)
		registerCancel()
		if err != nil {
			c.logger.Warnf("Reconnect register failed: %v", err)
			c.manager.Disconnect()
			delay = c.nextBackoffDelay(delay, maxDelay, rc)
			continue
		}

		c.sessionID = sessionID
		c.connected.Store(true)

		// Re-set disconnect callback
		c.manager.SetOnDisconnect(func() {
			c.connected.Store(false)
			select {
			case c.disconnectCh <- struct{}{}:
			default:
			}
		})

		return nil
	}
}

// nextBackoffDelay calculates the next delay with exponential backoff and jitter.
func (c *client) nextBackoffDelay(current, max time.Duration, rc *ReconnectConfig) time.Duration {
	next := time.Duration(float64(current) * rc.BackoffMultiplier)
	if next > max {
		next = max
	}
	if rc.JitterFactor > 0 {
		jitter := time.Duration(float64(next) * rc.JitterFactor)
		// Simple jitter: add random [0, 2*jitter) - jitter
		if span := int64(2 * jitter); span > 0 {
			next = next + time.Duration(rand.Int63n(span)) - jitter
		}
	}
	if next < time.Millisecond {
		next = time.Millisecond
	}
	return next
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

// convertToLocalFunctions converts FunctionDescriptors to ProviderFunctionDescriptors
// Note: This method must be called while holding c.mu (either read or write lock)
func (c *client) convertToLocalFunctions() []ProviderFunctionDescriptor {
	var localFuncs []ProviderFunctionDescriptor
	for funcID := range c.handlers {
		desc, ok := c.descriptors[funcID]
		if !ok {
			continue
		}
		version := desc.Version
		if version == "" {
			version = "1.0.0"
		}

		// Convert FunctionDescriptor to ProviderFunctionDescriptor with OpenAPI 3.0.3 fields
		localDesc := ProviderFunctionDescriptor{
			ID:                funcID,
			Version:           version,
			Tags:              desc.Tags,
			Summary:           firstNonEmpty(desc.Summary, funcID),
			Description:       firstNonEmpty(desc.Description, fmt.Sprintf("Function: %s", funcID)),
			OperationID:       firstNonEmpty(desc.OperationID, funcID),
			Deprecated:        desc.Deprecated,
			InputSchema:       firstNonEmpty(desc.InputSchema, generateBasicInputSchema()),
			OutputSchema:      firstNonEmpty(desc.OutputSchema, generateBasicOutputSchema()),
			Resource:          desc.Resource,
			Operation:         desc.Operation,
			Capability:        desc.Capability,
			Execution:         desc.Execution,
			ApprovalRequired:  desc.ApprovalRequired,
			ApprovalPolicyKey: desc.ApprovalPolicyKey,
			Risk:              desc.Risk,
			Enabled:           desc.Enabled,
			Permission:        desc.Permission,
		}

		localFuncs = append(localFuncs, localDesc)
	}
	return localFuncs
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptySlice(values []string, fallback []string) []string {
	if len(values) > 0 {
		return values
	}
	return fallback
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
