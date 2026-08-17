// Package croupier provides a Go SDK for Croupier game function registration and execution.
package croupier

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	defaultServerAPIURL      = "http://127.0.0.1:18780/api/v1"
	defaultTaskPollInterval  = 500 * time.Millisecond
	maxServerErrorBodyLength = 64 << 10
)

// httpInvoker is the L3 invoker. It is deliberately independent of the
// Provider's TCP session and only talks to the Server HTTP API.
type httpInvoker struct {
	config     *InvokerConfig
	httpClient *http.Client
	baseURL    *url.URL
	mu         sync.RWMutex

	schemas map[string]map[string]interface{}

	connected bool

	defaultGameID string
	defaultEnv    string
}

// NewHTTPInvoker creates an invoker for the Server HTTP API. Address may be a
// complete API base URL (recommended), a Server root URL, or host:port. A root
// URL and host:port are normalized to /api/v1.
func NewHTTPInvoker(config *InvokerConfig) Invoker {
	config = normalizeHTTPInvokerConfig(config)
	baseURL := parseServerAPIURL(config.Address)

	return &httpInvoker{
		config:        config,
		httpClient:    newHTTPClient(config),
		baseURL:       baseURL,
		schemas:       make(map[string]map[string]interface{}),
		defaultGameID: strings.TrimSpace(config.GameID),
		defaultEnv:    strings.TrimSpace(config.Env),
	}
}

// Connect records that the independent HTTP invoker is ready for use. HTTP is
// request based, so it intentionally does not open a Provider-like session.
func (i *httpInvoker) Connect(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.connected = true
	return nil
}

// Close closes idle HTTP connections and drops local validation schemas.
func (i *httpInvoker) Close() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.connected = false
	i.schemas = make(map[string]map[string]interface{})
	i.httpClient.CloseIdleConnections()
	return nil
}

// Invoke synchronously calls POST /api/v1/functions/:id/invoke and returns the
// Server result JSON. The Server remains responsible for authorization, audit,
// scope validation, routing and dispatch.
func (i *httpInvoker) Invoke(ctx context.Context, functionID, payload string, options InvokeOptions) (string, error) {
	if err := validateFunctionID(functionID); err != nil {
		return "", err
	}
	if err := i.validateConfiguredPayload(functionID, payload); err != nil {
		return "", err
	}

	params, err := parseJSONPayload(payload)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(struct {
		Params interface{} `json:"params"`
	}{Params: params})
	if err != nil {
		return "", fmt.Errorf("marshal invoke request: %w", err)
	}

	response, err := i.doJSON(ctx, http.MethodPost, []string{"functions", functionID, "invoke"}, "", body, options)
	if err != nil {
		return "", err
	}
	var result struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(response, &result); err != nil {
		return "", fmt.Errorf("decode invoke response: %w", err)
	}
	if result.Result == nil {
		return "", fmt.Errorf("server invoke response does not contain result")
	}
	return string(result.Result), nil
}

// StartTask starts an asynchronous Server task and returns the Server-issued
// task ID. It never fabricates a local task ID.
func (i *httpInvoker) StartTask(ctx context.Context, functionID, payload string, options InvokeOptions) (string, error) {
	if err := validateFunctionID(functionID); err != nil {
		return "", err
	}
	if err := i.validateConfiguredPayload(functionID, payload); err != nil {
		return "", err
	}

	params, err := parseJSONPayload(payload)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(struct {
		FunctionID string      `json:"functionId"`
		Params     interface{} `json:"params"`
	}{FunctionID: functionID, Params: params})
	if err != nil {
		return "", fmt.Errorf("marshal start task request: %w", err)
	}

	response, err := i.doJSON(ctx, http.MethodPost, []string{"tasks"}, "", body, options)
	if err != nil {
		return "", err
	}
	var task struct {
		TaskID string `json:"taskId"`
	}
	if err := json.Unmarshal(response, &task); err != nil {
		return "", fmt.Errorf("decode start task response: %w", err)
	}
	if strings.TrimSpace(task.TaskID) == "" {
		return "", fmt.Errorf("server start task response does not contain taskId")
	}
	return task.TaskID, nil
}

// StreamTask polls the Server task-events endpoint from the last seen sequence
// until the Server marks the task complete or the context is cancelled.
func (i *httpInvoker) StreamTask(ctx context.Context, taskID string) (<-chan TaskEvent, error) {
	if err := validateTaskID(taskID); err != nil {
		return nil, err
	}

	events := make(chan TaskEvent, 16)
	go func() {
		defer close(events)
		var afterSeq int64
		for {
			response, err := i.doJSON(ctx, http.MethodGet, []string{"tasks", taskID, "events"}, fmt.Sprintf("after_seq=%d", afterSeq), nil, InvokeOptions{})
			if err != nil {
				sendTaskEvent(ctx, events, TaskEvent{EventType: "error", TaskID: taskID, Error: err.Error(), Done: true})
				return
			}

			var page struct {
				Items []struct {
					Seq      int64           `json:"seq"`
					Type     string          `json:"type"`
					Progress int32           `json:"progress"`
					Message  string          `json:"message"`
					Payload  json.RawMessage `json:"payload"`
				} `json:"items"`
				Done bool `json:"done"`
			}
			if err := json.Unmarshal(response, &page); err != nil {
				sendTaskEvent(ctx, events, TaskEvent{EventType: "error", TaskID: taskID, Error: fmt.Sprintf("decode task events response: %v", err), Done: true})
				return
			}

			emitted := false
			for _, item := range page.Items {
				afterSeq = maxInt64(afterSeq, item.Seq)
				event := TaskEvent{
					EventType: item.Type,
					TaskID:    taskID,
					Payload:   string(item.Payload),
				}
				if event.Payload == "" {
					event.Payload = item.Message
				}
				if item.Type == "failed" || item.Type == "error" {
					event.Error = item.Message
				}
				event.Done = isTerminalTaskEvent(item.Type)
				emitted = true
				if !sendTaskEvent(ctx, events, event) {
					return
				}
			}
			if page.Done {
				return
			}
			if !emitted {
				if !waitForContext(ctx, i.taskPollInterval()) {
					return
				}
			}
		}
	}()
	return events, nil
}

// CancelTask asks the Server to cancel an existing task.
func (i *httpInvoker) CancelTask(ctx context.Context, taskID string) error {
	if err := validateTaskID(taskID); err != nil {
		return err
	}
	_, err := i.doJSON(ctx, http.MethodPost, []string{"tasks", taskID, "cancel"}, "", []byte("{}"), InvokeOptions{})
	return err
}

// GetTaskStatus gets the current task state from GET /api/v1/tasks/:id. The
// returned result remains raw JSON so callers do not lose the Server's result
// shape while using this transport-level SDK.
func (i *httpInvoker) GetTaskStatus(ctx context.Context, taskID string) (*TaskStatus, error) {
	if err := validateTaskID(taskID); err != nil {
		return nil, err
	}

	response, err := i.doJSON(ctx, http.MethodGet, []string{"tasks", taskID}, "", nil, InvokeOptions{})
	if err != nil {
		return nil, err
	}
	var payload struct {
		ID         string          `json:"id"`
		FunctionID string          `json:"functionId"`
		Status     string          `json:"status"`
		Progress   int32           `json:"progress"`
		Message    string          `json:"message"`
		GameID     string          `json:"gameId"`
		Env        string          `json:"env"`
		AgentID    string          `json:"agentId"`
		Actor      string          `json:"actor"`
		TraceID    string          `json:"traceId"`
		Result     json.RawMessage `json:"result"`
		Error      string          `json:"error"`
		StartedAt  string          `json:"startedAt"`
		FinishedAt string          `json:"finishedAt"`
		CreatedAt  string          `json:"createdAt"`
		UpdatedAt  string          `json:"updatedAt"`
	}
	if err := json.Unmarshal(response, &payload); err != nil {
		return nil, fmt.Errorf("decode task status response: %w", err)
	}

	status := &TaskStatus{
		TaskID:     payload.ID,
		FunctionID: payload.FunctionID,
		Status:     payload.Status,
		Progress:   payload.Progress,
		Message:    payload.Message,
		GameID:     payload.GameID,
		Env:        payload.Env,
		AgentID:    payload.AgentID,
		Actor:      payload.Actor,
		TraceID:    payload.TraceID,
		Error:      payload.Error,
		StartedAt:  payload.StartedAt,
		FinishedAt: payload.FinishedAt,
		CreatedAt:  payload.CreatedAt,
		UpdatedAt:  payload.UpdatedAt,
	}
	if len(payload.Result) > 0 && string(payload.Result) != "null" {
		status.Result = string(payload.Result)
	}
	if status.TaskID == "" {
		status.TaskID = taskID
	}
	return status, nil
}

// SetSchema configures optional local JSON Schema validation before a request
// is sent. Server-side schema/governance checks remain authoritative.
func (i *httpInvoker) SetSchema(functionID string, schema map[string]interface{}) error {
	if err := validateFunctionID(functionID); err != nil {
		return err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.schemas[functionID] = schema
	return nil
}

// SetDefaultGameEnv configures default Server scope headers for this invoker.
func (i *httpInvoker) SetDefaultGameEnv(gameID, env string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.defaultGameID = strings.TrimSpace(gameID)
	i.defaultEnv = strings.TrimSpace(env)
}

// IsConnected reports whether Connect has been called. It does not represent a
// persistent TCP session because the L3 transport is ordinary HTTP.
func (i *httpInvoker) IsConnected() bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.connected
}

// GetAddress returns the normalized Server API base URL.
func (i *httpInvoker) GetAddress() string {
	return i.baseURL.String()
}

func (i *httpInvoker) doJSON(ctx context.Context, method string, segments []string, rawQuery string, body []byte, options InvokeOptions) ([]byte, error) {
	requestURL := i.endpoint(segments, rawQuery)
	return i.withRetry(ctx, options, func(callCtx context.Context) ([]byte, int, error) {
		var reader io.Reader
		if body != nil {
			reader = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(callCtx, method, requestURL, reader)
		if err != nil {
			return nil, 0, fmt.Errorf("create HTTP request: %w", err)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		i.applyHeaders(req, options)

		resp, err := i.httpClient.Do(req)
		if err != nil {
			return nil, 0, fmt.Errorf("send HTTP request: %w", err)
		}
		defer resp.Body.Close()
		responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxServerErrorBodyLength))
		if err != nil {
			return nil, resp.StatusCode, fmt.Errorf("read HTTP response: %w", err)
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			return nil, resp.StatusCode, &httpInvokerError{statusCode: resp.StatusCode, message: serverErrorMessage(responseBody)}
		}
		return responseBody, resp.StatusCode, nil
	})
}

func (i *httpInvoker) endpoint(segments []string, rawQuery string) string {
	u := *i.baseURL
	parts := []string{u.Path}
	for _, segment := range segments {
		parts = append(parts, url.PathEscape(segment))
	}
	u.Path = path.Join(parts...)
	u.RawQuery = rawQuery
	return u.String()
}

func (i *httpInvoker) applyHeaders(req *http.Request, options InvokeOptions) {
	for key, value := range options.Headers {
		req.Header.Set(key, value)
	}
	if strings.TrimSpace(options.IdempotencyKey) != "" {
		req.Header.Set("Idempotency-Key", options.IdempotencyKey)
	}

	i.mu.RLock()
	gameID, env := i.defaultGameID, i.defaultEnv
	i.mu.RUnlock()
	if value := strings.TrimSpace(req.Header.Get("X-Game-ID")); value != "" {
		gameID = value
	}
	if value := strings.TrimSpace(req.Header.Get("X-Env")); value != "" {
		env = value
	}
	if gameID != "" {
		req.Header.Set("X-Game-ID", gameID)
	}
	if env != "" {
		req.Header.Set("X-Env", env)
	}
	if token := strings.TrimSpace(i.config.AuthToken); token != "" && req.Header.Get("Authorization") == "" {
		if !strings.HasPrefix(strings.ToLower(token), "bearer ") {
			token = "Bearer " + token
		}
		req.Header.Set("Authorization", token)
	}
}

func (i *httpInvoker) withRetry(ctx context.Context, options InvokeOptions, request func(context.Context) ([]byte, int, error)) ([]byte, error) {
	retry := i.config.Retry
	if options.Retry != nil {
		retry = options.Retry
	}
	attempts := 1
	if retry != nil && retry.Enabled && retry.MaxAttempts > 1 {
		attempts = retry.MaxAttempts
	}

	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 && !waitForContext(ctx, retryDelay(attempt-1, retry)) {
			return nil, ctx.Err()
		}
		callCtx := ctx
		cancel := func() {}
		if options.Timeout > 0 {
			callCtx, cancel = context.WithTimeout(ctx, options.Timeout)
		}
		response, statusCode, err := request(callCtx)
		cancel()
		if err == nil {
			return response, nil
		}
		lastErr = err
		if attempt == attempts-1 || !isRetryableHTTPError(statusCode, retry) {
			break
		}
	}
	return nil, lastErr
}

func (i *httpInvoker) validateConfiguredPayload(functionID, payload string) error {
	i.mu.RLock()
	schema := i.schemas[functionID]
	i.mu.RUnlock()
	if schema == nil {
		return nil
	}
	if err := i.validatePayload(functionID, payload, schema); err != nil {
		return fmt.Errorf("payload validation failed: %w", err)
	}
	return nil
}

func (i *httpInvoker) validatePayload(_ string, payload string, schema map[string]interface{}) error {
	if len(schema) == 0 {
		return nil
	}
	value, err := parseJSONPayload(payload)
	if err != nil {
		return err
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft7)
	if err := compiler.AddResource("schema.json", schema); err != nil {
		return fmt.Errorf("create schema validator: %w", err)
	}
	compiled, err := compiler.Compile("schema.json")
	if err != nil {
		return fmt.Errorf("compile schema validator: %w", err)
	}
	if err := compiled.Validate(value); err != nil {
		return err
	}
	return nil
}

// parseJSONPayload is retained for local helper callers. Network operations use
// the strict package-level parser above and therefore never send invalid JSON
// to the Server API.
func (i *httpInvoker) parseJSONPayload(payload string) interface{} {
	value, err := parseJSONPayload(payload)
	if err != nil {
		return payload
	}
	return value
}

func (i *httpInvoker) taskPollInterval() time.Duration {
	if i.config.TaskPollInterval > 0 {
		return i.config.TaskPollInterval
	}
	return defaultTaskPollInterval
}

func normalizeHTTPInvokerConfig(config *InvokerConfig) *InvokerConfig {
	if config == nil {
		config = &InvokerConfig{}
	}
	if strings.TrimSpace(config.Address) == "" {
		config.Address = defaultServerAPIURL
	}
	if config.TimeoutSeconds <= 0 {
		config.TimeoutSeconds = 30
	}
	config.DefaultTimeout = time.Duration(config.TimeoutSeconds) * time.Second
	if config.Retry == nil {
		config.Retry = DefaultRetryConfig()
	}
	return config
}

func parseServerAPIURL(address string) *url.URL {
	address = strings.TrimSpace(address)
	if !strings.Contains(address, "://") {
		address = "http://" + address
	}
	parsed, err := url.Parse(address)
	if err != nil || parsed.Host == "" {
		parsed, _ = url.Parse(defaultServerAPIURL)
	}
	parsed.Path = normalizeServerAPIPath(parsed.Path)
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed
}

func normalizeServerAPIPath(p string) string {
	p = strings.TrimSuffix(strings.TrimSpace(p), "/")
	if p == "" {
		return "/api/v1"
	}
	if strings.HasSuffix(p, "/api/v1") {
		return p
	}
	return path.Join(p, "/api/v1")
}

func newHTTPClient(config *InvokerConfig) *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}
	if !config.Insecure {
		tlsConfig, err := buildHTTPInvokerTLSConfig(config)
		if err != nil {
			// Keep constructor compatibility. The request will fail deterministically
			// instead of silently downgrading TLS verification.
			transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		} else {
			transport.TLSClientConfig = tlsConfig
		}
	}
	return &http.Client{Timeout: config.DefaultTimeout, Transport: transport}
}

func buildHTTPInvokerTLSConfig(config *InvokerConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if config.CAFile != "" {
		pem, err := osReadFile(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("parse CA file")
		}
		tlsConfig.RootCAs = pool
	}
	if config.CertFile != "" || config.KeyFile != "" {
		certificate, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}
	return tlsConfig, nil
}

// osReadFile is a variable solely to keep TLS file errors testable without
// changing the public invoker API.
var osReadFile = func(name string) ([]byte, error) { return os.ReadFile(name) }

func parseJSONPayload(payload string) (interface{}, error) {
	if strings.TrimSpace(payload) == "" {
		return map[string]interface{}{}, nil
	}
	var value interface{}
	if err := json.Unmarshal([]byte(payload), &value); err != nil {
		return nil, fmt.Errorf("payload must be valid JSON: %w", err)
	}
	return value, nil
}

func validateFunctionID(functionID string) error {
	if strings.TrimSpace(functionID) == "" {
		return fmt.Errorf("function ID cannot be empty")
	}
	return nil
}

func validateTaskID(taskID string) error {
	if strings.TrimSpace(taskID) == "" {
		return fmt.Errorf("task ID cannot be empty")
	}
	return nil
}

type httpInvokerError struct {
	statusCode int
	message    string
}

func (e *httpInvokerError) Error() string {
	return fmt.Sprintf("server returned HTTP %d: %s", e.statusCode, e.message)
}

func serverErrorMessage(body []byte) string {
	var response struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if json.Unmarshal(body, &response) == nil {
		if strings.TrimSpace(response.Message) != "" {
			return response.Message
		}
		if strings.TrimSpace(response.Error) != "" {
			return response.Error
		}
	}
	if message := strings.TrimSpace(string(body)); message != "" {
		return message
	}
	return "empty response body"
}

func isRetryableHTTPError(statusCode int, retry *RetryConfig) bool {
	if statusCode == 0 {
		return true
	}
	if retry != nil && len(retry.RetryableStatusCodes) > 0 {
		for _, code := range retry.RetryableStatusCodes {
			if int32(statusCode) == code {
				return true
			}
		}
	}
	return statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError
}

func retryDelay(attempt int, retry *RetryConfig) time.Duration {
	if retry == nil {
		return 0
	}
	multiplier := retry.BackoffMultiplier
	if multiplier <= 0 {
		multiplier = 2
	}
	delay := time.Duration(float64(retry.InitialDelayMs)*math.Pow(multiplier, float64(attempt))) * time.Millisecond
	if retry.MaxDelayMs > 0 && delay > time.Duration(retry.MaxDelayMs)*time.Millisecond {
		delay = time.Duration(retry.MaxDelayMs) * time.Millisecond
	}
	if retry.JitterFactor > 0 && delay > 0 {
		jitter := time.Duration(float64(delay) * retry.JitterFactor * (2*rand.Float64() - 1))
		delay += jitter
	}
	if delay < 0 {
		return 0
	}
	return delay
}

func waitForContext(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func sendTaskEvent(ctx context.Context, events chan<- TaskEvent, event TaskEvent) bool {
	select {
	case <-ctx.Done():
		return false
	case events <- event:
		return true
	}
}

func isTerminalTaskEvent(eventType string) bool {
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "completed", "succeeded", "failed", "error", "cancelled", "canceled", "timed_out", "timeout":
		return true
	default:
		return false
	}
}

func maxInt64(left, right int64) int64 {
	if right > left {
		return right
	}
	return left
}
