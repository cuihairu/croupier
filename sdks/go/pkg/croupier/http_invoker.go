package croupier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	"github.com/xeipuuv/gojsonschema"
)

// httpInvoker implements Invoker interface using HTTP REST API
// Note: Async job operations (StartJob, StreamJob, CancelJob) are not fully supported
type httpInvoker struct {
	config     *InvokerConfig
	httpClient *http.Client
	baseURL    *url.URL
	mu         sync.RWMutex

	// Function schemas for validation
	schemas map[string]map[string]interface{}

	// Connection state
	connected bool

	// Default game/env for invocations
	defaultGameID string
	defaultEnv    string
}

// NewHTTPInvoker creates a new HTTP-based Croupier invoker
// Use this instead of NewInvoker when connecting via HTTP REST API
func NewHTTPInvoker(config *InvokerConfig) Invoker {
	if config == nil {
		config = &InvokerConfig{
			Address:        "localhost:18780", // Default HTTP REST port
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

	// Parse base URL
	baseURL, err := url.Parse(config.Address)
	if err != nil {
		// Fallback to http://localhost:port if address is just a host:port
		baseURL, err = url.Parse("http://" + config.Address)
		if err != nil {
			baseURL = &url.URL{Scheme: "http", Host: "localhost:18780"}
		}
	}

	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: config.DefaultTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	return &httpInvoker{
		config:     config,
		httpClient: httpClient,
		baseURL:    baseURL,
		schemas:    make(map[string]map[string]interface{}),
	}
}

// Connect implements Invoker.Connect
func (i *httpInvoker) Connect(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.connected {
		return nil
	}

	logInfof("Connecting to HTTP server at: %s", i.baseURL.String())
	i.connected = true
	logInfof("Connected to HTTP server successfully")
	return nil
}

// Close implements Invoker.Close
func (i *httpInvoker) Close() error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if !i.connected {
		return nil
	}

	i.connected = false
	i.httpClient.CloseIdleConnections()
	logInfof("HTTP connection closed")
	return nil
}

// Invoke implements Invoker.Invoke using HTTP REST API
func (i *httpInvoker) Invoke(ctx context.Context, functionID, payload string, options InvokeOptions) (string, error) {
	// Validate against schema if set
	i.mu.RLock()
	schema, hasSchema := i.schemas[functionID]
	i.mu.RUnlock()

	if hasSchema {
		if err := i.validatePayload(functionID, payload, schema); err != nil {
			return "", fmt.Errorf("payload validation failed: %w", err)
		}
	}

	// Prepare request body
	reqBody := map[string]interface{}{
		"payload": i.parseJSONPayload(payload),
		"mode":    "sync", // HTTP invoke is synchronous
	}

	// Add optional fields
	if options.IdempotencyKey != "" {
		reqBody["idempotencyKey"] = options.IdempotencyKey
	}

	// Add game/env from options or defaults
	gameID := i.defaultGameID
	env := i.defaultEnv
	// Extract game/env from options.Headers if present
	if options.Headers != nil {
		if g, ok := options.Headers["X-Game-ID"]; ok {
			gameID = g
		}
		if e, ok := options.Headers["X-Env"]; ok {
			env = e
		}
	}
	if gameID != "" {
		reqBody["gameId"] = gameID
	}
	if env != "" {
		reqBody["env"] = env
	}

	// Marshal request body
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build URL
	invokerURL := *i.baseURL
	invokerURL.Path = path.Join(invokerURL.Path, "/api/function", functionID, "invoke")

	// Retry logic
	var lastErr error
	maxAttempts := 1
	if i.config.Retry != nil && i.config.Retry.Enabled && i.config.Retry.MaxAttempts > 0 {
		maxAttempts = i.config.Retry.MaxAttempts
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// Calculate delay with exponential backoff + jitter
			backoffMs := i.config.Retry.InitialDelayMs * int(math.Pow(2, float64(attempt-1)))
			if i.config.Retry.JitterFactor > 0 {
				jitterRange := int(float64(backoffMs) * i.config.Retry.JitterFactor)
				backoffMs += rand.Intn(jitterRange*2) - jitterRange
			}
			delay := time.Duration(backoffMs) * time.Millisecond
			if i.config.Retry.MaxDelayMs > 0 && delay > time.Duration(i.config.Retry.MaxDelayMs)*time.Millisecond {
				delay = time.Duration(i.config.Retry.MaxDelayMs) * time.Millisecond
			}

			logInfof("Invocation attempt %d failed, retrying in %v: %v", attempt+1, delay, lastErr)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
			}
		}

		// Create HTTP request
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, invokerURL.String(), bytes.NewReader(bodyBytes))
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		req.Header.Set("Content-Type", "application/json")

		// Add custom headers from options
		for k, v := range options.Headers {
			req.Header.Set(k, v)
		}

		// Send request
		resp, err := i.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)
			continue
		}

		// Read response body
		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		// Check status code
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("HTTP error: %d - %s", resp.StatusCode, string(respBody))
			continue
		}

		// Parse response - server returns {code: 0, message: "", data: {...}}
		var result struct {
			Code    int         `json:"code"`
			Message string      `json:"message"`
			Data    interface{} `json:"data"`
		}

		if err := json.Unmarshal(respBody, &result); err != nil {
			lastErr = fmt.Errorf("failed to parse response: %w", err)
			continue
		}

		if result.Code != 0 {
			lastErr = fmt.Errorf("server error: %s", result.Message)
			continue
		}

		// Convert data to JSON string
		resultJSON, err := json.Marshal(result.Data)
		if err != nil {
			return "", fmt.Errorf("failed to marshal result: %w", err)
		}

		return string(resultJSON), nil
	}

	return "", fmt.Errorf("invoke failed after %d attempts: %w", maxAttempts, lastErr)
}

// StartJob implements Invoker.StartJob
// Note: HTTP REST API doesn't fully support async jobs yet
func (i *httpInvoker) StartJob(ctx context.Context, functionID, payload string, options InvokeOptions) (string, error) {
	// For HTTP API, we can simulate async jobs by invoking synchronously
	// but this is not true async execution
	_, err := i.Invoke(ctx, functionID, payload, options)
	if err != nil {
		return "", err
	}

	// Return a fake job ID (in real implementation, server should return job ID)
	return fmt.Sprintf("http-job-%d", time.Now().UnixNano()), nil
}

// StreamJob implements Invoker.StreamJob
// Note: Not supported for HTTP REST API
func (i *httpInvoker) StreamJob(ctx context.Context, jobID string) (<-chan JobEvent, error) {
	return nil, fmt.Errorf("StreamJob is not supported for HTTP invoker")
}

// CancelJob implements Invoker.CancelJob
// Note: Not supported for HTTP REST API
func (i *httpInvoker) CancelJob(ctx context.Context, jobID string) error {
	return fmt.Errorf("CancelJob is not supported for HTTP invoker")
}

// SetSchema implements Invoker.SetSchema
func (i *httpInvoker) SetSchema(functionID string, schema map[string]interface{}) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.schemas[functionID] = schema
	logInfof("Schema set for function: %s", functionID)
	return nil
}

// SetDefaultGameEnv sets default game ID and env for invocations
func (i *httpInvoker) SetDefaultGameEnv(gameID, env string) {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.defaultGameID = gameID
	i.defaultEnv = env
}

// parseJSONPayload safely parses JSON or returns the raw string
func (i *httpInvoker) parseJSONPayload(payload string) interface{} {
	var jsonPayload interface{}
	if err := json.Unmarshal([]byte(payload), &jsonPayload); err != nil {
		// If not valid JSON, return as string
		return payload
	}
	return jsonPayload
}

// validatePayload validates the payload against a JSON schema
func (i *httpInvoker) validatePayload(functionID, payload string, schema map[string]interface{}) error {
	// Parse payload
	var payloadJSON interface{}
	if err := json.Unmarshal([]byte(payload), &payloadJSON); err != nil {
		return fmt.Errorf("invalid JSON payload: %w", err)
	}

	// Create schema loader
	schemaLoader := gojsonschema.NewGoLoader(schema)
	documentLoader := gojsonschema.NewGoLoader(payloadJSON)

	// Validate
	schemaVal, err := gojsonschema.NewSchema(schemaLoader)
	if err != nil {
		return fmt.Errorf("failed to create schema validator: %w", err)
	}

	result, err := schemaVal.Validate(documentLoader)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if !result.Valid() {
		var errs []string
		for _, desc := range result.Errors() {
			errs = append(errs, desc.Field()+": "+desc.Description())
		}
		return fmt.Errorf("payload validation failed: %v", errs)
	}

	return nil
}

// IsConnected returns whether the invoker is connected
func (i *httpInvoker) IsConnected() bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.connected
}

// GetAddress returns the server address
func (i *httpInvoker) GetAddress() string {
	return i.baseURL.String()
}
