package croupier

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// invoker implements the Invoker interface
type invoker struct {
	config *InvokerConfig
	conn   *grpc.ClientConn
	mu     sync.RWMutex

	// Function schemas for validation
	schemas map[string]map[string]interface{}

	// Connection state
	connected bool
}

// NewInvoker creates a new Croupier invoker
func NewInvoker(config *InvokerConfig) Invoker {
	if config == nil {
		config = &InvokerConfig{
			Address:        "localhost:8080",
			TimeoutSeconds: 30,
			Insecure:       true,
		}
	}

	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = time.Duration(config.TimeoutSeconds) * time.Second
	}

	return &invoker{
		config:  config,
		schemas: make(map[string]map[string]interface{}),
	}
}

// Connect implements Invoker.Connect
func (i *invoker) Connect(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.connected {
		return nil
	}

	fmt.Printf("Connecting to server/agent at: %s\n", i.config.Address)

	// Setup connection options
	var opts []grpc.DialOption

	if i.config.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		// TODO: Implement TLS credentials
		return fmt.Errorf("TLS not implemented yet")
	}

	// Create connection with timeout
	ctx, cancel := context.WithTimeout(ctx, i.config.DefaultTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, i.config.Address, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", i.config.Address, err)
	}

	i.conn = conn
	i.connected = true

	fmt.Printf("Connected to: %s\n", i.config.Address)
	return nil
}

// Invoke implements Invoker.Invoke
func (i *invoker) Invoke(ctx context.Context, functionID, payload string, options InvokeOptions) (string, error) {
	if !i.connected {
		if err := i.Connect(ctx); err != nil {
			return "", fmt.Errorf("not connected to server: %w", err)
		}
	}

	// Client-side validation
	if schema, exists := i.schemas[functionID]; exists {
		if err := i.validatePayload(payload, schema); err != nil {
			return "", fmt.Errorf("payload validation failed for function %s: %w", functionID, err)
		}
	}

	fmt.Printf("Invoking function: %s\n", functionID)
	fmt.Printf("Payload: %s\n", payload)

	// TODO: Implement actual gRPC call using FunctionService
	// This would use the generated gRPC client from function.proto

	// Mock response for now
	response := fmt.Sprintf(`{"status":"success","function_id":"%s","timestamp":"%s"}`,
		functionID, time.Now().Format(time.RFC3339))

	fmt.Printf("Response: %s\n", response)
	return response, nil
}

// StartJob implements Invoker.StartJob
func (i *invoker) StartJob(ctx context.Context, functionID, payload string, options InvokeOptions) (string, error) {
	if !i.connected {
		if err := i.Connect(ctx); err != nil {
			return "", fmt.Errorf("not connected to server: %w", err)
		}
	}

	fmt.Printf("Starting job for function: %s\n", functionID)

	// TODO: Implement actual gRPC call for job execution
	// This would use the JobService from the proto definitions

	// Generate mock job ID
	jobID := fmt.Sprintf("job-%s-%d", functionID, time.Now().Unix())
	fmt.Printf("Started job: %s\n", jobID)

	return jobID, nil
}

// StreamJob implements Invoker.StreamJob
func (i *invoker) StreamJob(ctx context.Context, jobID string) (<-chan JobEvent, error) {
	eventCh := make(chan JobEvent, 10)

	if !i.connected {
		if err := i.Connect(ctx); err != nil {
			close(eventCh)
			return eventCh, fmt.Errorf("not connected to server: %w", err)
		}
	}

	fmt.Printf("Streaming job events for: %s\n", jobID)

	// Start goroutine to simulate job events
	go func() {
		defer close(eventCh)

		// Mock job events
		events := []JobEvent{
			{
				EventType: "started",
				JobID:     jobID,
				Payload:   `{"status":"job_started"}`,
				Done:      false,
			},
			{
				EventType: "progress",
				JobID:     jobID,
				Payload:   `{"progress":50}`,
				Done:      false,
			},
			{
				EventType: "completed",
				JobID:     jobID,
				Payload:   `{"result":"success"}`,
				Done:      true,
			},
		}

		for _, event := range events {
			select {
			case eventCh <- event:
				time.Sleep(500 * time.Millisecond) // Simulate processing time
			case <-ctx.Done():
				return
			}
		}
	}()

	return eventCh, nil
}

// CancelJob implements Invoker.CancelJob
func (i *invoker) CancelJob(ctx context.Context, jobID string) error {
	if !i.connected {
		if err := i.Connect(ctx); err != nil {
			return fmt.Errorf("not connected to server: %w", err)
		}
	}

	fmt.Printf("Cancelling job: %s\n", jobID)

	// TODO: Implement actual gRPC call for job cancellation

	return nil
}

// SetSchema implements Invoker.SetSchema
func (i *invoker) SetSchema(functionID string, schema map[string]interface{}) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.schemas[functionID] = schema
	fmt.Printf("Set schema for function: %s\n", functionID)
	return nil
}

// Close implements Invoker.Close
func (i *invoker) Close() error {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.connected = false

	if i.conn != nil {
		i.conn.Close()
		i.conn = nil
	}

	i.schemas = make(map[string]map[string]interface{})
	fmt.Println("Invoker closed")
	return nil
}

// validatePayload validates payload against schema
func (i *invoker) validatePayload(payload string, schema map[string]interface{}) error {
	// TODO: Implement proper JSON schema validation
	// For now, just check if payload is not empty
	if payload == "" {
		return fmt.Errorf("payload cannot be empty")
	}
	return nil
}