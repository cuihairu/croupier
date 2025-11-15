package croupier

import (
	"context"
	"fmt"
	"sync"
)

// Client represents a Croupier client for function registration and execution
type Client interface {
	// RegisterFunction registers a single function with the agent
	RegisterFunction(desc FunctionDescriptor, handler FunctionHandler) error

	// Connect establishes connection to the agent
	Connect(ctx context.Context) error

	// Serve starts the local gRPC server and maintains the connection
	Serve(ctx context.Context) error

	// Stop gracefully stops the client
	Stop() error

	// Close closes the client and cleans up resources
	Close() error

	// GetLocalAddress returns the local server address
	GetLocalAddress() string
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
	config   *ClientConfig
	handlers map[string]FunctionHandler
	mu       sync.RWMutex

	// gRPC related fields
	grpcManager GRPCManager
	sessionID   string
	localAddr   string

	// State management
	connected bool
	running   bool
	stopCh    chan struct{}
}

// NewClient creates a new Croupier client
func NewClient(config *ClientConfig) Client {
	if config == nil {
		config = DefaultClientConfig()
	}

	return &client{
		config:   config,
		handlers: make(map[string]FunctionHandler),
		stopCh:   make(chan struct{}),
	}
}

// RegisterFunction implements Client.RegisterFunction
func (c *client) RegisterFunction(desc FunctionDescriptor, handler FunctionHandler) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return fmt.Errorf("cannot register functions while client is running")
	}

	// Validate function descriptor
	if desc.ID == "" {
		return fmt.Errorf("function ID cannot be empty")
	}
	if desc.Version == "" {
		return fmt.Errorf("function version cannot be empty")
	}

	c.handlers[desc.ID] = handler
	fmt.Printf("Registered function: %s (version: %s)\n", desc.ID, desc.Version)
	return nil
}

// Connect implements Client.Connect
func (c *client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	fmt.Printf("🔌 Connecting to Croupier Agent: %s\n", c.config.AgentAddr)

	// Initialize gRPC manager
	grpcConfig := GRPCConfig{
		AgentAddr:      c.config.AgentAddr,
		LocalListen:    c.config.LocalListen,
		TimeoutSeconds: c.config.TimeoutSeconds,
		Insecure:       c.config.Insecure,
		CAFile:         c.config.CAFile,
		CertFile:       c.config.CertFile,
		KeyFile:        c.config.KeyFile,
	}

	var err error
	c.grpcManager, err = NewGRPCManager(grpcConfig, c.handlers)
	if err != nil {
		return fmt.Errorf("failed to create gRPC manager: %w", err)
	}

	// Connect to agent
	if err := c.grpcManager.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to agent: %w", err)
	}

	// Register functions with agent
	localFunctions := c.convertToLocalFunctions()
	sessionID, err := c.grpcManager.RegisterWithAgent(ctx, c.config.ServiceID, c.config.ServiceVersion, localFunctions)
	if err != nil {
		c.grpcManager.Disconnect()
		return fmt.Errorf("failed to register with agent: %w", err)
	}

	c.sessionID = sessionID
	c.localAddr = c.grpcManager.GetLocalAddress()
	c.connected = true

	fmt.Printf("✅ Successfully connected and registered with Agent\n")
	fmt.Printf("📍 Local service address: %s\n", c.localAddr)
	fmt.Printf("🔑 Session ID: %s\n", c.sessionID)

	return nil
}

// Serve implements Client.Serve
func (c *client) Serve(ctx context.Context) error {
	if !c.connected {
		if err := c.Connect(ctx); err != nil {
			return fmt.Errorf("connection failed, cannot start service: %w", err)
		}
	}

	c.running = true
	fmt.Printf("🚀 Croupier client service started\n")
	fmt.Printf("📍 Local service address: %s\n", c.localAddr)
	fmt.Printf("🎯 Registered functions: %d\n", len(c.handlers))
	fmt.Printf("💡 Use Stop() to stop the service\n")
	fmt.Printf("===============================================\n")

	// Start local gRPC server
	if err := c.grpcManager.StartServer(ctx); err != nil {
		return fmt.Errorf("failed to start local server: %w", err)
	}

	// Wait for stop signal or context cancellation
	select {
	case <-c.stopCh:
		fmt.Println("🛑 Service stopped by Stop() call")
	case <-ctx.Done():
		fmt.Println("🛑 Service stopped by context cancellation")
	}

	c.running = false
	fmt.Println("🛑 Service has stopped")
	return nil
}

// Stop implements Client.Stop
func (c *client) Stop() error {
	c.running = false
	c.connected = false

	fmt.Println("🛑 Stopping Croupier client...")

	if c.grpcManager != nil {
		c.grpcManager.Disconnect()
	}

	close(c.stopCh)
	fmt.Println("✅ Client stopped successfully")
	return nil
}

// Close implements Client.Close
func (c *client) Close() error {
	c.Stop()
	c.handlers = nil
	return nil
}

// GetLocalAddress implements Client.GetLocalAddress
func (c *client) GetLocalAddress() string {
	return c.localAddr
}

// convertToLocalFunctions converts FunctionDescriptors to LocalFunctionDescriptors
func (c *client) convertToLocalFunctions() []LocalFunctionDescriptor {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var localFuncs []LocalFunctionDescriptor
	for funcID := range c.handlers {
		// Note: In a real implementation, you'd store the full FunctionDescriptor
		// and convert it here. For simplicity, we're using the handler key.
		localFuncs = append(localFuncs, LocalFunctionDescriptor{
			ID:      funcID,
			Version: "1.0.0", // Default version, should come from stored descriptor
		})
	}
	return localFuncs
}