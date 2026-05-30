// Package croupier provides a Go SDK for Croupier game function registration and execution.
package croupier

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	sdkv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/sdk/v1"
	"github.com/xeipuuv/gojsonschema"
	"google.golang.org/protobuf/proto"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/protocol"
	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/transport"
)

// tcpInvoker implements Invoker using TCP transport
type tcpInvoker struct {
	config *InvokerConfig
	client *transport.TCPClient
	mu     sync.RWMutex

	// Function schemas for validation
	schemas map[string]map[string]interface{}

	// Connection state
	connected bool

	// Reconnection state
	isReconnecting      bool
	reconnectAttempts   int
	reconnectCancelCtx  context.CancelFunc
	reconnectCancelFlag bool
}

// NewInvoker creates a new Croupier invoker using TCP transport
func NewInvoker(config *InvokerConfig) Invoker {
	if config == nil {
		config = &InvokerConfig{
			Address:        "localhost:19090",
			TimeoutSeconds: 30,
			Insecure:       true,
			Reconnect:      DefaultReconnectConfig(),
			Retry:          DefaultRetryConfig(),
		}
	}

	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = time.Duration(config.TimeoutSeconds) * time.Second
	}

	// Set default reconnect config if not provided
	if config.Reconnect == nil {
		config.Reconnect = DefaultReconnectConfig()
	}

	// Set default retry config if not provided
	if config.Retry == nil {
		config.Retry = DefaultRetryConfig()
	}

	return &tcpInvoker{
		config:  config,
		schemas: make(map[string]map[string]interface{}),
	}
}

// Connect implements Invoker.Connect
func (i *tcpInvoker) Connect(ctx context.Context) error {
	err := i.connect(ctx)
	if err != nil {
		// Trigger reconnection if connection error and reconnection is enabled
		if i.isConnectionError(err) {
			i.scheduleReconnectIfNeeded()
		}
		return err
	}
	return nil
}

// connect performs the actual connection without triggering reconnection
func (i *tcpInvoker) connect(ctx context.Context) error {
	// Fast path: read lock to check if already connected
	i.mu.RLock()
	if i.connected {
		i.mu.RUnlock()
		return nil
	}
	// Check if reconnection is in progress
	if i.isReconnecting {
		i.mu.RUnlock()
		return fmt.Errorf("reconnection in progress")
	}
	i.mu.RUnlock()

	// Slow path: need to connect, acquire write lock
	i.mu.Lock()
	defer i.mu.Unlock()

	// Double-check after acquiring write lock
	if i.connected {
		return nil
	}

	// Check again if reconnection started while we were waiting for the lock
	if i.isReconnecting {
		return fmt.Errorf("reconnection in progress")
	}

	return i.connectLocked(ctx)
}

// connectLocked performs the actual connection. Caller must hold i.mu.
func (i *tcpInvoker) connectLocked(ctx context.Context) error {
	logInfof("Connecting to server/agent at: %s", i.config.Address)

	// Create transport config
	transportCfg := &transport.Config{
		Address:     i.config.Address,
		Insecure:    i.config.Insecure,
		CAFile:      i.config.CAFile,
		CertFile:    i.config.CertFile,
		KeyFile:     i.config.KeyFile,
		ServerName:  i.config.Address, // Use address as server name
		SendTimeout: i.config.DefaultTimeout,
	}

	// Apply TLS config
	if !i.config.Insecure {
		tlsCfg, err := i.buildTLSConfig()
		if err != nil {
			return err
		}
		// Note: transport.Config will use these file paths to create TLS config
		_ = tlsCfg // Will be used by transport layer
	}

	client, err := transport.NewTCPClient(transportCfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	i.client = client
	i.connected = true

	// Reset reconnection attempts on successful connection
	i.reconnectAttempts = 0
	i.isReconnecting = false

	logInfof("Connected to: %s\n", i.config.Address)
	return nil
}

// Invoke implements Invoker.Invoke
func (i *tcpInvoker) Invoke(ctx context.Context, functionID, payload string, options InvokeOptions) (string, error) {
	// Ensure connected
	var err error
	for attempts := 0; attempts < 3; attempts++ {
		err = i.connect(ctx)
		if err == nil {
			break
		}
		if err.Error() == "reconnection in progress" {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		// For other errors, trigger reconnection if needed
		if i.isConnectionError(err) {
			i.scheduleReconnectIfNeeded()
		}
		return "", fmt.Errorf("not connected to server: %w", err)
	}

	// Client-side validation
	if schema, exists := i.schemas[functionID]; exists {
		if err := i.validatePayload(payload, schema); err != nil {
			return "", fmt.Errorf("payload validation failed for function %s: %w", functionID, err)
		}
	}

	// Capture client reference to avoid race during reconnection
	i.mu.RLock()
	client := i.client
	i.mu.RUnlock()

	return i.executeWithRetry(ctx, options, func() (string, error) {
		// Build InvokeRequest
		req := &sdkv1.InvokeRequest{
			FunctionId:     functionID,
			IdempotencyKey: options.IdempotencyKey,
			Payload:        []byte(payload),
			Metadata:       map[string]string{},
		}
		for k, v := range options.Headers {
			req.Metadata[k] = v
		}

		// Marshal request
		reqBytes, err := proto.Marshal(req)
		if err != nil {
			return "", fmt.Errorf("marshal request: %w", err)
		}

		// Call via transport
		callCtx := ctx
		if options.Timeout > 0 {
			var cancel context.CancelFunc
			callCtx, cancel = context.WithTimeout(ctx, options.Timeout)
			defer cancel()
		}

		_, respBody, err := client.Call(callCtx, protocol.MsgInvokeRequest, reqBytes)
		if err != nil {
			return "", err
		}

		// Parse response
		resp := &sdkv1.InvokeResponse{}
		if err := proto.Unmarshal(respBody, resp); err != nil {
			return "", fmt.Errorf("unmarshal response: %w", err)
		}

		return string(resp.Payload), nil
	})
}

// StartJob implements Invoker.StartJob
func (i *tcpInvoker) StartJob(ctx context.Context, functionID, payload string, options InvokeOptions) (string, error) {
	// Ensure connected
	var err error
	for attempts := 0; attempts < 3; attempts++ {
		err = i.connect(ctx)
		if err == nil {
			break
		}
		if err.Error() == "reconnection in progress" {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		if i.isConnectionError(err) {
			i.scheduleReconnectIfNeeded()
		}
		return "", fmt.Errorf("not connected to server: %w", err)
	}

	i.mu.RLock()
	client := i.client
	i.mu.RUnlock()

	return i.executeWithRetry(ctx, options, func() (string, error) {
		req := &sdkv1.InvokeRequest{
			FunctionId:     functionID,
			IdempotencyKey: options.IdempotencyKey,
			Payload:        []byte(payload),
			Metadata:       map[string]string{},
		}
		for k, v := range options.Headers {
			req.Metadata[k] = v
		}

		reqBytes, err := proto.Marshal(req)
		if err != nil {
			return "", fmt.Errorf("marshal request: %w", err)
		}

		// Use StartJob message type
		_, respBody, err := client.Call(ctx, protocol.MsgStartJobRequest, reqBytes)
		if err != nil {
			return "", err
		}

		// For StartJob, response would contain job ID
		// For now, return a placeholder
		return string(respBody), nil
	})
}

// StreamJob implements Invoker.StreamJob using TCP
// Note: Since TCP doesn't support true streaming, this implementation
// polls for job status updates and sends events to the channel.
func (i *tcpInvoker) StreamJob(ctx context.Context, jobID string) (<-chan JobEvent, error) {
	eventCh := make(chan JobEvent, 10)

	// Ensure connected
	var err error
	for attempts := 0; attempts < 3; attempts++ {
		err = i.connect(ctx)
		if err == nil {
			break
		}
		if err.Error() == "reconnection in progress" {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		if i.isConnectionError(err) {
			i.scheduleReconnectIfNeeded()
		}
		close(eventCh)
		return eventCh, fmt.Errorf("not connected to server: %w", err)
	}

	// Start polling goroutine for job status
	go func() {
		defer close(eventCh)

		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Poll for job status
				req := &sdkv1.TaskStreamRequest{
					TaskId: jobID,
				}

				reqBytes, err := proto.Marshal(req)
				if err != nil {
					eventCh <- JobEvent{
						EventType: "error",
						JobID:     jobID,
						Error:     fmt.Sprintf("marshal request: %v", err),
						Done:      true,
					}
					return
				}

				i.mu.RLock()
				client := i.client
				i.mu.RUnlock()

				if client == nil {
					eventCh <- JobEvent{
						EventType: "error",
						JobID:     jobID,
						Error:     "connection lost",
						Done:      true,
					}
					return
				}

				_, respBody, err := client.Call(ctx, protocol.MsgStreamJobRequest, reqBytes)
				if err != nil {
					eventCh <- JobEvent{
						EventType: "error",
						JobID:     jobID,
						Error:     fmt.Sprintf("poll job status failed: %v", err),
						Done:      true,
					}
					return
				}

				// Parse job event
				event := &sdkv1.TaskEvent{}
				if err := proto.Unmarshal(respBody, event); err != nil {
					eventCh <- JobEvent{
						EventType: "error",
						JobID:     jobID,
						Error:     fmt.Sprintf("unmarshal response: %v", err),
						Done:      true,
					}
					return
				}

				// Send event to channel
				jobEvent := JobEvent{
					EventType: event.GetType(),
					JobID:     jobID,
					Payload:   string(event.GetPayload()),
				}
				if event.GetProgress() > 0 {
					jobEvent.Payload = fmt.Sprintf("Progress: %d%%", event.GetProgress())
				}
				if event.GetMessage() != "" {
					jobEvent.Payload = event.GetMessage()
				}

				// Check if job is complete
				if event.GetType() == "done" || event.GetType() == "error" {
					jobEvent.Done = true
					eventCh <- jobEvent
					return
				}

				eventCh <- jobEvent
			}
		}
	}()

	return eventCh, nil
}

// CancelJob implements Invoker.CancelJob
func (i *tcpInvoker) CancelJob(ctx context.Context, jobID string) error {
	// Ensure connected
	if err := i.connect(ctx); err != nil {
		if i.isConnectionError(err) {
			i.scheduleReconnectIfNeeded()
		}
		return fmt.Errorf("not connected to server: %w", err)
	}

	i.mu.RLock()
	client := i.client
	i.mu.RUnlock()

	// Build CancelJobRequest
	req := &sdkv1.CancelTaskRequest{
		TaskId: jobID,
	}

	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	_, _, err = client.Call(ctx, protocol.MsgCancelJobRequest, reqBytes)
	if err != nil {
		return fmt.Errorf("cancel job failed: %w", err)
	}

	return nil
}

// SetSchema implements Invoker.SetSchema
func (i *tcpInvoker) SetSchema(functionID string, schema map[string]interface{}) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.schemas[functionID] = schema
	logDebugf("Set schema for function: %s", functionID)
	return nil
}

// Close implements Invoker.Close
func (i *tcpInvoker) Close() error {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Cancel any pending reconnection
	if i.reconnectCancelCtx != nil {
		i.reconnectCancelCtx()
		i.reconnectCancelCtx = nil
	}

	i.connected = false
	i.client = nil
	i.isReconnecting = false
	i.reconnectAttempts = 0

	if i.client != nil {
		i.client.Close()
		i.client = nil
	}

	i.schemas = make(map[string]map[string]interface{})
	logInfof("Invoker closed")
	return nil
}

// validatePayload validates payload against schema
func (i *tcpInvoker) validatePayload(payload string, schema map[string]interface{}) error {
	if len(schema) == 0 {
		if payload == "" {
			return fmt.Errorf("payload cannot be empty")
		}
		return nil
	}

	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	schemaLoader := gojsonschema.NewBytesLoader(schemaBytes)
	payloadLoader := gojsonschema.NewStringLoader(payload)

	result, err := gojsonschema.Validate(schemaLoader, payloadLoader)
	if err != nil {
		return fmt.Errorf("schema validation error: %w", err)
	}

	if result.Valid() {
		return nil
	}

	var errs []string
	for _, desc := range result.Errors() {
		errs = append(errs, desc.String())
	}

	return fmt.Errorf("payload validation failed: %s", strings.Join(errs, "; "))
}

// buildTLSConfig builds TLS configuration from InvokerConfig
func (i *tcpInvoker) buildTLSConfig() (*tls.Config, error) {
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// Root CA handling
	if i.config.CAFile != "" {
		caBytes, err := os.ReadFile(i.config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caBytes) {
			return nil, fmt.Errorf("failed to append CA certificates")
		}
		tlsCfg.RootCAs = pool
	} else {
		systemPool, err := x509.SystemCertPool()
		if err == nil {
			tlsCfg.RootCAs = systemPool
		}
	}

	// Client certificate (optional)
	if i.config.CertFile != "" && i.config.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(i.config.CertFile, i.config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	// Infer server name for TLS verification
	if i.config.Address != "" {
		host := i.config.Address
		if h, _, err := net.SplitHostPort(i.config.Address); err == nil && h != "" {
			host = h
		}
		tlsCfg.ServerName = host
	}

	return tlsCfg, nil
}

// isConnectionError determines if an error is a connection-related error
func (i *tcpInvoker) isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Check for common connection error patterns
	connectionPatterns := []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"network is unreachable",
		"no such host",
		"timeout",
		"transport is closing",
		"connection unavailable",
	}
	for _, pattern := range connectionPatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return true
		}
	}
	return false
}

// scheduleReconnectIfNeeded schedules a reconnection attempt if enabled
func (i *tcpInvoker) scheduleReconnectIfNeeded() {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Check if reconnection is enabled
	if !i.config.Reconnect.Enabled {
		return
	}

	// Check if already reconnecting
	if i.isReconnecting {
		return
	}

	// Check max attempts
	if i.config.Reconnect.MaxAttempts > 0 && i.reconnectAttempts >= i.config.Reconnect.MaxAttempts {
		logInfof("Max reconnection attempts (%d) reached, giving up", i.config.Reconnect.MaxAttempts)
		return
	}

	i.isReconnecting = true
	i.reconnectAttempts++
	attempts := i.reconnectAttempts

	delay := i.calculateReconnectDelay(attempts)
	logInfof("Scheduling reconnection attempt %d in %v", attempts, delay)

	// Create a cancellable context for the reconnection
	reconnectCtx, cancel := context.WithCancel(context.Background())
	i.reconnectCancelCtx = cancel
	i.reconnectCancelFlag = true

	go func() {
		defer func() {
			i.mu.Lock()
			if i.reconnectCancelFlag {
				i.reconnectCancelCtx = nil
				i.reconnectCancelFlag = false
			}
			i.mu.Unlock()
			cancel()
		}()

		select {
		case <-time.After(delay):
			logInfof("Reconnecting... (attempt %d)", attempts)
			if err := i.connect(context.Background()); err != nil {
				logInfof("Reconnection attempt %d failed: %v", attempts, err)
				i.scheduleReconnectIfNeeded()
			} else {
				logInfof("Reconnection successful")
			}
		case <-reconnectCtx.Done():
			logInfof("Reconnection cancelled")
		}
	}()
}

// calculateReconnectDelay calculates the delay before next reconnection attempt
func (i *tcpInvoker) calculateReconnectDelay(attempts int) time.Duration {
	cfg := i.config.Reconnect

	// Calculate base delay using exponential backoff
	baseDelay := time.Duration(cfg.InitialDelayMs) * time.Millisecond
	exponentialDelay := baseDelay * time.Duration(math.Pow(float64(cfg.BackoffMultiplier), float64(attempts-1)))

	// Cap at max delay
	maxDelay := time.Duration(cfg.MaxDelayMs) * time.Millisecond
	if exponentialDelay > maxDelay {
		exponentialDelay = maxDelay
	}

	// Add jitter to prevent thundering herd
	jitter := time.Duration(float64(exponentialDelay) * cfg.JitterFactor)
	randomJitter := time.Duration(float64(jitter) * (2*rand.Float64() - 1))

	finalDelay := exponentialDelay + randomJitter
	// Ensure final delay is within bounds
	if finalDelay < 0 {
		finalDelay = 0
	}
	if finalDelay > maxDelay {
		finalDelay = maxDelay
	}

	return finalDelay
}

// executeWithRetry executes a function with retry logic
func (i *tcpInvoker) executeWithRetry(ctx context.Context, options InvokeOptions, fn func() (string, error)) (string, error) {
	// Use options retry if provided, otherwise use config retry
	retryConfig := options.Retry
	if retryConfig == nil {
		retryConfig = i.config.Retry
	}

	// If retry is disabled, execute directly
	if !retryConfig.Enabled {
		return fn()
	}

	var lastErr error
	maxAttempts := retryConfig.MaxAttempts

	for attempt := 0; attempt < maxAttempts; attempt++ {
		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Check if this is a retryable error
		if !i.isRetryableError(err) || attempt >= maxAttempts-1 {
			return "", fmt.Errorf("invoke failed after %d attempts: %w", attempt+1, err)
		}

		// Connection errors should trigger reconnection
		if i.isConnectionError(err) && i.config.Reconnect.Enabled {
			i.mu.Lock()
			i.connected = false
			i.mu.Unlock()
			i.scheduleReconnectIfNeeded()
		}

		// Calculate delay and wait
		delay := i.calculateRetryDelay(attempt, retryConfig)
		logInfof("Invocation attempt %d failed, retrying in %v: %v", attempt+1, delay, err)

		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return "", fmt.Errorf("retry cancelled: %w", ctx.Err())
		}
	}

	return "", fmt.Errorf("invoke failed after %d attempts: %w", maxAttempts, lastErr)
}

// isRetryableError checks if an error should trigger a retry
func (i *tcpInvoker) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Check for common retryable error patterns
	retryablePatterns := []string{
		"unavailable",
		"internal error",
		"deadline exceeded",
		"aborted",
		"transport is closing",
		"timeout",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// calculateRetryDelay calculates the delay before next retry attempt
func (i *tcpInvoker) calculateRetryDelay(attempt int, cfg *RetryConfig) time.Duration {
	// Calculate base delay using exponential backoff
	baseDelay := time.Duration(cfg.InitialDelayMs) * time.Millisecond
	exponentialDelay := baseDelay * time.Duration(math.Pow(float64(cfg.BackoffMultiplier), float64(attempt)))

	// Cap at max delay
	maxDelay := time.Duration(cfg.MaxDelayMs) * time.Millisecond
	if exponentialDelay > maxDelay {
		exponentialDelay = maxDelay
	}

	// Add jitter to prevent thundering herd
	jitter := time.Duration(float64(exponentialDelay) * cfg.JitterFactor)
	randomJitter := time.Duration(float64(jitter) * (2*rand.Float64() - 1))

	finalDelay := exponentialDelay + randomJitter
	// Ensure final delay is within bounds
	if finalDelay < 0 {
		finalDelay = 0
	}
	if finalDelay > maxDelay {
		finalDelay = maxDelay
	}

	return finalDelay
}
