// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package croupier

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ValidateClientConfig validates the client configuration
func ValidateClientConfig(config *ClientConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Required fields
	if config.ServiceID == "" {
		return fmt.Errorf("service_id is required")
	}

	if config.GameID == "" {
		return fmt.Errorf("game_id is required")
	}

	if config.AgentAddr == "" {
		return fmt.Errorf("agent_addr is required")
	}

	if config.Env == "" {
		return fmt.Errorf("env is required")
	}

	// Timeout validation
	if config.TimeoutSeconds <= 0 {
		return fmt.Errorf("timeout_seconds must be positive")
	}

	// Heartbeat validation
	if config.HeartbeatInterval > 0 && config.TimeoutSeconds > 0 {
		if config.HeartbeatInterval < config.TimeoutSeconds {
			return fmt.Errorf("heartbeat_interval (%d) must be >= timeout_seconds (%d)",
				config.HeartbeatInterval, config.TimeoutSeconds)
		}
	}

	if config.HeartbeatInterval <= 0 {
		return fmt.Errorf("heartbeat_interval must be positive")
	}

	// TLS validation
	if !config.Insecure && !config.InsecureSkipVerify && config.CAFile == "" {
		return fmt.Errorf("CA file is required when not using insecure mode")
	}

	// TLS cert/key pair validation
	if config.CertFile != "" && config.KeyFile == "" {
		return fmt.Errorf("key_file is required when cert_file is provided")
	}
	if config.KeyFile != "" && config.CertFile == "" {
		return fmt.Errorf("cert_file is required when key_file is provided")
	}

	// Log level validation
	if config.LogLevel != "" {
		validLevels := map[string]bool{
			"DEBUG": true, "INFO": true, "WARN": true,
			"ERROR": true, "OFF": true,
			"debug": true, "info": true, "warn": true,
			"error": true, "off": true,
		}
		if !validLevels[config.LogLevel] {
			return fmt.Errorf("invalid log_level: %s", config.LogLevel)
		}
	}

	// File transfer validation
	if config.EnableFileTransfer {
		if config.MaxFileSize <= 0 {
			return fmt.Errorf("max_file_size must be positive when file transfer is enabled")
		}
		if config.MaxFileSize > 100*1024*1024 {
			return fmt.Errorf("max_file_size exceeds maximum allowed (100MB)")
		}
	}

	// Validate nested configs
	if config.Reconnect != nil {
		if err := ValidateReconnectConfig(config.Reconnect); err != nil {
			return fmt.Errorf("invalid reconnect config: %w", err)
		}
	}

	if config.Retry != nil {
		if err := ValidateRetryConfig(config.Retry); err != nil {
			return fmt.Errorf("invalid retry config: %w", err)
		}
	}

	return nil
}

// ValidateReconnectConfig validates the reconnection configuration
func ValidateReconnectConfig(config *ReconnectConfig) error {
	if config == nil {
		return nil // nil is valid (use defaults)
	}

	if config.InitialDelayMs < 0 {
		return fmt.Errorf("initial_delay_ms cannot be negative")
	}

	if config.MaxDelayMs < 0 {
		return fmt.Errorf("max_delay_ms cannot be negative")
	}

	if config.InitialDelayMs > config.MaxDelayMs {
		return fmt.Errorf("initial_delay_ms (%d) cannot be greater than max_delay_ms (%d)",
			config.InitialDelayMs, config.MaxDelayMs)
	}

	if config.BackoffMultiplier < 1.0 {
		return fmt.Errorf("backoff_multiplier must be >= 1.0, got %f", config.BackoffMultiplier)
	}

	if config.JitterFactor < 0 || config.JitterFactor > 1.0 {
		return fmt.Errorf("jitter_factor must be between 0 and 1, got %f", config.JitterFactor)
	}

	return nil
}

// ValidateRetryConfig validates the retry configuration
func ValidateRetryConfig(config *RetryConfig) error {
	if config == nil {
		return nil // nil is valid (use defaults)
	}

	if config.InitialDelayMs < 0 {
		return fmt.Errorf("initial_delay_ms cannot be negative")
	}

	if config.MaxDelayMs < 0 {
		return fmt.Errorf("max_delay_ms cannot be negative")
	}

	if config.BackoffMultiplier < 1.0 {
		return fmt.Errorf("backoff_multiplier must be >= 1.0, got %f", config.BackoffMultiplier)
	}

	if config.JitterFactor < 0 || config.JitterFactor > 1.0 {
		return fmt.Errorf("jitter_factor must be between 0 and 1, got %f", config.JitterFactor)
	}

	if config.Enabled {
		if config.MaxAttempts <= 0 {
			return fmt.Errorf("max_attempts must be positive when retry is enabled")
		}
		if config.MaxAttempts > 10 {
			return fmt.Errorf("max_attempts cannot exceed 10")
		}
	}

	return nil
}

// ValidateInvokerConfig validates the invoker configuration
func ValidateInvokerConfig(config *InvokerConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if config.Address == "" {
		return fmt.Errorf("address is required")
	}

	if config.TimeoutSeconds <= 0 {
		return fmt.Errorf("timeout_seconds must be positive")
	}

	// TLS validation
	if !config.Insecure && config.CAFile == "" {
		return fmt.Errorf("CA file is required when not using insecure mode")
	}

	return nil
}

// ValidateFunctionDescriptor validates a function descriptor
func ValidateFunctionDescriptor(descriptor *FunctionDescriptor) error {
	if descriptor.ID == "" {
		return fmt.Errorf("function ID is required")
	}

	if descriptor.Version == "" {
		return fmt.Errorf("function version is required")
	}

	// Validate semver format
	if !isValidSemVer(descriptor.Version) {
		return fmt.Errorf("invalid version format: %s (expected semver)", descriptor.Version)
	}

	return nil
}

// ValidateProviderFunctionDescriptor validates a local function descriptor
func ValidateProviderFunctionDescriptor(descriptor *ProviderFunctionDescriptor) error {
	if descriptor.ID == "" {
		return fmt.Errorf("function ID is required")
	}

	if descriptor.Version == "" {
		return fmt.Errorf("function version is required")
	}

	// Validate JSON schemas if provided
	if descriptor.InputSchema != "" {
		if !IsValidJSONSchema(descriptor.InputSchema) {
			return fmt.Errorf("invalid input_schema JSON")
		}
	}

	if descriptor.OutputSchema != "" {
		if !IsValidJSONSchema(descriptor.OutputSchema) {
			return fmt.Errorf("invalid output_schema JSON")
		}
	}

	return nil
}

// ValidateTaskEvent validates a task event
func ValidateTaskEvent(event *TaskEvent) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	if event.TaskID == "" {
		return fmt.Errorf("task_id is required")
	}

	validTypes := map[string]bool{
		"started":   true,
		"progress":  true,
		"completed": true,
		"error":     true,
	}

	if !validTypes[event.EventType] {
		return fmt.Errorf("invalid event_type: %s", event.EventType)
	}

	// Set Done flag based on event type
	if event.EventType == "completed" || event.EventType == "error" {
		event.Done = true
	}

	return nil
}

// IsValidJSONSchema checks if a string is valid JSON schema
func IsValidJSONSchema(schema string) bool {
	var js map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &js); err != nil {
		return false
	}
	// Basic check - must have at least a schema field or be an object
	if _, ok := js["$schema"]; ok {
		return true
	}
	if _, ok := js["type"]; ok {
		return true
	}
	return len(js) > 0
}

// isValidSemVer checks if a string is valid semver
func isValidSemVer(version string) bool {
	// Simple semver regex: X.Y.Z where X, Y, Z are numbers
	semverRegex := regexp.MustCompile(`^v?\d+\.\d+\.\d+(-[a-zA-Z0-9.-]+)?(\+[a-zA-Z0-9.-]+)?$`)
	return semverRegex.MatchString(version)
}

// CalculateReconnectDelay calculates the delay for a reconnection attempt
func CalculateReconnectDelay(config *ReconnectConfig, attempt int) time.Duration {
	if config == nil {
		return 0
	}

	// Calculate base delay with exponential backoff
	baseDelay := time.Duration(config.InitialDelayMs) * time.Millisecond
	delay := baseDelay * time.Duration(1<<uint(attempt))

	// Cap at max delay
	maxDelay := time.Duration(config.MaxDelayMs) * time.Millisecond
	if delay > maxDelay {
		delay = maxDelay
	}

	// Apply jitter if configured
	if config.JitterFactor > 0 {
		jitterRange := int64(float64(delay) * config.JitterFactor)
		if jitterRange > 0 {
			// This is a simplified jitter calculation
			// In production, use a proper random source
			jitter := time.Duration(int64(attempt*123456789) % (2 * jitterRange))
			delay = delay - time.Duration(jitterRange) + jitter
		}
	}

	return delay
}

// CalculateRetryDelay calculates the delay for a retry attempt
func CalculateRetryDelay(config *RetryConfig, attempt int) time.Duration {
	if config == nil {
		return 0
	}

	// Calculate base delay with exponential backoff
	baseDelay := time.Duration(config.InitialDelayMs) * time.Millisecond
	delay := baseDelay * time.Duration(1<<uint(attempt))

	// Cap at max delay
	maxDelay := time.Duration(config.MaxDelayMs) * time.Millisecond
	if delay > maxDelay {
		delay = maxDelay
	}

	// Apply jitter if configured
	if config.JitterFactor > 0 {
		jitterRange := int64(float64(delay) * config.JitterFactor)
		if jitterRange > 0 {
			jitter := time.Duration(int64(attempt*987654321) % (2 * jitterRange))
			delay = delay - time.Duration(jitterRange) + jitter
		}
	}

	return delay
}

// MergeWithDefaults merges a partial config with defaults
func MergeWithDefaults(partial *ClientConfig) *ClientConfig {
	defaultConfig := DefaultClientConfig()

	if partial == nil {
		return defaultConfig
	}

	result := *defaultConfig

	// Override with provided values
	if partial.ServiceID != "" {
		result.ServiceID = partial.ServiceID
	}
	if partial.GameID != "" {
		result.GameID = partial.GameID
	}
	if partial.AgentAddr != "" {
		result.AgentAddr = partial.AgentAddr
	}
	if partial.AgentIPCAddr != "" {
		result.AgentIPCAddr = partial.AgentIPCAddr
	}
	if partial.Env != "" {
		result.Env = partial.Env
	}
	if partial.ServiceVersion != "" {
		result.ServiceVersion = partial.ServiceVersion
	}
	if partial.TimeoutSeconds > 0 {
		result.TimeoutSeconds = partial.TimeoutSeconds
	}
	if partial.HeartbeatInterval > 0 {
		result.HeartbeatInterval = partial.HeartbeatInterval
	}
	if partial.Insecure {
		result.Insecure = true
	}
	if partial.CAFile != "" {
		result.CAFile = partial.CAFile
	}
	if partial.CertFile != "" {
		result.CertFile = partial.CertFile
	}
	if partial.KeyFile != "" {
		result.KeyFile = partial.KeyFile
	}
	if partial.ServerName != "" {
		result.ServerName = partial.ServerName
	}
	if partial.InsecureSkipVerify {
		result.InsecureSkipVerify = true
	}
	if partial.AuthToken != "" {
		result.AuthToken = partial.AuthToken
	}
	if partial.LogLevel != "" {
		result.LogLevel = partial.LogLevel
	}
	if partial.DisableLogging {
		result.DisableLogging = true
	}
	if partial.DebugLogging {
		result.DebugLogging = true
	}

	// Merge headers
	if partial.Headers != nil {
		result.Headers = make(map[string]string)
		for k, v := range defaultConfig.Headers {
			result.Headers[k] = v
		}
		for k, v := range partial.Headers {
			result.Headers[k] = v
		}
	}

	// Merge nested configs
	if partial.Reconnect != nil {
		result.Reconnect = partial.Reconnect
	}
	if partial.Retry != nil {
		result.Retry = partial.Retry
	}
	if partial.EnableFileTransfer {
		result.EnableFileTransfer = true
		if partial.MaxFileSize > 0 {
			result.MaxFileSize = partial.MaxFileSize
		}
	}

	return &result
}

// ApplyEnvOverrides applies environment variable overrides to config
func ApplyEnvOverrides(config *ClientConfig, envMap map[string]string) *ClientConfig {
	if config == nil {
		config = DefaultClientConfig()
	}

	result := *config

	// Apply overrides from envMap
	if val, ok := envMap["CROUPIER_AGENT_ADDR"]; ok {
		result.AgentAddr = val
	}
	if val, ok := envMap["CROUPIER_TIMEOUT"]; ok {
		if i, err := strconv.Atoi(val); err == nil && i > 0 {
			result.TimeoutSeconds = i
		}
	}
	if val, ok := envMap["CROUPIER_INSECURE"]; ok {
		result.Insecure = strings.ToLower(val) == "true" || val == "1"
	}
	if val, ok := envMap["CROUPIER_LOG_LEVEL"]; ok {
		result.LogLevel = strings.ToUpper(val)
	}
	if val, ok := envMap["CROUPIER_SERVICE_ID"]; ok {
		result.ServiceID = val
	}
	if val, ok := envMap["CROUPIER_GAME_ID"]; ok {
		result.GameID = val
	}
	if val, ok := envMap["CROUPIER_ENV"]; ok {
		result.Env = val
	}
	if val, ok := envMap["CROUPIER_AUTH_TOKEN"]; ok {
		result.AuthToken = val
	}
	if val, ok := envMap["CROUPIER_CONTROL_ADDR"]; ok {
		result.ControlAddr = val
	}
	if val, ok := envMap["CROUPIER_HEARTBEAT"]; ok {
		if i, err := strconv.Atoi(val); err == nil && i > 0 {
			result.HeartbeatInterval = i
		}
	}
	if val, ok := envMap["CROUPIER_RECONNECT"]; ok {
		if strings.ToLower(val) == "true" || val == "1" {
			if result.Reconnect == nil {
				result.Reconnect = DefaultReconnectConfig()
			}
			result.Reconnect.Enabled = true
		}
	}
	if val, ok := envMap["CROUPIER_RECONNECT_MAX"]; ok {
		if i, err := strconv.Atoi(val); err == nil {
			if result.Reconnect == nil {
				result.Reconnect = DefaultReconnectConfig()
			}
			result.Reconnect.MaxAttempts = i
		}
	}

	return &result
}

// GetFallbackAddresses returns a list of fallback addresses
func GetFallbackAddresses(config *ClientConfig) []string {
	var addresses []string

	// IPC address first
	if config.AgentIPCAddr != "" {
		addresses = append(addresses, config.AgentIPCAddr)
	}

	// Parse comma-separated TCP addresses
	if config.AgentAddr != "" {
		parts := strings.Split(config.AgentAddr, ",")
		for _, part := range parts {
			if addr := strings.TrimSpace(part); addr != "" {
				addresses = append(addresses, addr)
			}
		}
	}

	return addresses
}

// Clone creates a deep copy of the config
func (c *ClientConfig) Clone() *ClientConfig {
	if c == nil {
		return nil
	}

	clone := *c

	// Copy headers
	if c.Headers != nil {
		clone.Headers = make(map[string]string, len(c.Headers))
		for k, v := range c.Headers {
			clone.Headers[k] = v
		}
	}

	// Copy reconnect config
	if c.Reconnect != nil {
		reconnectClone := *c.Reconnect
		clone.Reconnect = &reconnectClone
	}

	// Copy retry config
	if c.Retry != nil {
		retryClone := *c.Retry
		retryClone.RetryableStatusCodes = make([]int32, len(c.Retry.RetryableStatusCodes))
		copy(retryClone.RetryableStatusCodes, c.Retry.RetryableStatusCodes)
		clone.Retry = &retryClone
	}

	return &clone
}

// DeepCopy is an alias for Clone for consistency
func (c *ClientConfig) DeepCopy() *ClientConfig {
	return c.Clone()
}

// MarshalJSON marshals the config to JSON
func (c *ClientConfig) MarshalJSON() ([]byte, error) {
	// Use type alias to avoid infinite recursion
	type clientConfigAlias ClientConfig
	return json.Marshal((*clientConfigAlias)(c))
}

// UnmarshalJSON unmarshals JSON into the config
func (c *ClientConfig) UnmarshalJSON(data []byte) error {
	// Use type alias to avoid infinite recursion
	type clientConfigAlias ClientConfig
	alias := (*clientConfigAlias)(c)
	return json.Unmarshal(data, alias)
}

// IsRetryable checks if a status code is retryable
func (r *RetryConfig) IsRetryable(statusCode int32) bool {
	if r == nil || !r.Enabled {
		return false
	}

	for _, code := range r.RetryableStatusCodes {
		if code == statusCode {
			return true
		}
	}
	return false
}

// WithDefaults applies default values to invoke options
func (o *InvokeOptions) WithDefaults() *InvokeOptions {
	if o == nil {
		return &InvokeOptions{}
	}

	result := *o

	if result.Headers == nil {
		result.Headers = make(map[string]string)
	}

	if result.Timeout == 0 {
		result.Timeout = 30 * time.Second
	}

	return &result
}
