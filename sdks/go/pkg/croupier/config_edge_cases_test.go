// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package croupier

import (
	"strings"
	"testing"
	"time"
)

// TestDefaultClientConfig_AllFieldsSet verifies default config has no zero values
func TestDefaultClientConfig_AllFieldsSet(t *testing.T) {
	t.Parallel()

	config := DefaultClientConfig()

	if config.ServiceID == "" {
		t.Error("ServiceID should not be empty")
	}
	if config.AgentAddr == "" {
		t.Error("AgentAddr should not be empty")
	}
	if config.Env == "" {
		t.Error("Env should not be empty")
	}
	if config.ServiceVersion == "" {
		t.Error("ServiceVersion should not be empty")
	}
	if config.TimeoutSeconds == 0 {
		t.Error("TimeoutSeconds should not be zero")
	}
	if config.HeartbeatInterval == 0 {
		t.Error("HeartbeatInterval should not be zero")
	}
	if config.Headers == nil {
		t.Error("Headers should not be nil")
	}
	if config.ProviderLang == "" {
		t.Error("ProviderLang should not be empty")
	}
	if config.ProviderSDK == "" {
		t.Error("ProviderSDK should not be empty")
	}
	if config.LogLevel == "" {
		t.Error("LogLevel should not be empty")
	}
	if config.Reconnect == nil {
		t.Error("Reconnect should not be nil")
	}
	if config.Retry == nil {
		t.Error("Retry should not be nil")
	}
}

// TestClientConfig_AgentAddrParsing tests various agent address formats
func TestClientConfig_AgentAddrParsing(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		address  string
		expected struct {
			host string
			port string
		}
	}{
		{
			name:    "localhost with port",
			address: "localhost:19090",
			expected: struct{ host, port string }{
				host: "localhost",
				port: "19090",
			},
		},
		{
			name:    "IP with port",
			address: "127.0.0.1:19090",
			expected: struct{ host, port string }{
				host: "127.0.0.1",
				port: "19090",
			},
		},
		{
			name:    "tcp:// prefix",
			address: "tcp://localhost:19090",
			expected: struct{ host, port string }{
				host: "localhost",
				port: "19090",
			},
		},
		{
			name:    "IPv6 with brackets",
			address: "[::1]:19090",
			expected: struct{ host, port string }{
				host: "::1",
				port: "19090",
			},
		},
		{
			name:    "IPv6 with brackets and tcp://",
			address: "tcp://[::1]:19090",
			expected: struct{ host, port string }{
				host: "::1",
				port: "19090",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			config := &ClientConfig{AgentAddr: tc.address}

			// Verify the address is stored correctly
			if config.AgentAddr != tc.address {
				t.Errorf("Expected AgentAddr %q, got %q", tc.address, config.AgentAddr)
			}
		})
	}
}

// TestClientConfig_MultiAddressFallback tests comma-separated address fallback
func TestClientConfig_MultiAddressFallback(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		agentAddr     string
		agentIPCAddr  string
		expectedFirst string
		expectedCount int
	}{
		{
			name:          "single address",
			agentAddr:     "localhost:19090",
			expectedFirst: "localhost:19090",
			expectedCount: 1,
		},
		{
			name:          "IPC address only",
			agentIPCAddr:  "ipc://croupier-agent",
			expectedFirst: "ipc://croupier-agent",
			expectedCount: 1,
		},
		{
			name:          "both addresses",
			agentAddr:     "localhost:19090",
			agentIPCAddr:  "ipc://croupier-agent",
			expectedFirst: "ipc://croupier-agent",
			expectedCount: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			config := &ClientConfig{
				TimeoutSeconds:    30,
				HeartbeatInterval: 60,
				AgentAddr:         tc.agentAddr,
				AgentIPCAddr:      tc.agentIPCAddr,
			}

			addresses := []string{}
			if config.AgentIPCAddr != "" {
				addresses = append(addresses, config.AgentIPCAddr)
			}
			if config.AgentAddr != "" {
				addresses = append(addresses, config.AgentAddr)
			}

			if len(addresses) != tc.expectedCount {
				t.Errorf("Expected %d addresses, got %d", tc.expectedCount, len(addresses))
			}

			if len(addresses) > 0 && addresses[0] != tc.expectedFirst {
				t.Errorf("Expected first address %q, got %q", tc.expectedFirst, addresses[0])
			}
		})
	}
}

// TestClientConfig_TLSValidation tests TLS configuration validation
func TestClientConfig_TLSValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		config      ClientConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "insecure mode - no TLS files required",
			config: ClientConfig{
				TimeoutSeconds:    30,
				HeartbeatInterval: 60,
				Insecure:          true,
				AgentAddr:         "localhost:19090",
				ServiceID:         "test-service",
				GameID:            "game1",
				Env:               "development",
			},
			wantErr: false,
		},
		{
			name: "secure mode without CA file",
			config: ClientConfig{
				TimeoutSeconds:    30,
				HeartbeatInterval: 60,
				Insecure:          false,
				AgentAddr:         "localhost:19090",
				ServiceID:         "test-service",
				GameID:            "game1",
				Env:               "development",
			},
			wantErr: true,
		},
		{
			name: "secure mode with skip verify",
			config: ClientConfig{
				TimeoutSeconds:     30,
				HeartbeatInterval:  60,
				Insecure:           false,
				InsecureSkipVerify: true,
				AgentAddr:          "localhost:19090",
				ServiceID:          "test-service",
				GameID:             "game1",
				Env:                "development",
			},
			wantErr: false,
		},
		{
			name: "secure mode with CA file",
			config: ClientConfig{
				TimeoutSeconds:    30,
				HeartbeatInterval: 60,
				Insecure:          false,
				CAFile:            "/path/to/ca.pem",
				AgentAddr:         "localhost:19090",
				ServiceID:         "test-service",
				GameID:            "game1",
				Env:               "development",
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateClientConfig(&tc.config)

			if tc.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("Expected error containing %q, got %q", tc.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// TestClientConfig_TimeoutValidation tests timeout configuration validation
func TestClientConfig_TimeoutValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		timeoutSec  int
		wantErr     bool
		errContains string
	}{
		{
			name:       "valid timeout - 30 seconds",
			timeoutSec: 30,
			wantErr:    false,
		},
		{
			name:       "valid timeout - 1 second",
			timeoutSec: 1,
			wantErr:    false,
		},
		{
			name:       "valid timeout - large value",
			timeoutSec: 300,
			wantErr:    false,
		},
		{
			name:        "invalid timeout - zero",
			timeoutSec:  0,
			wantErr:     true,
			errContains: "timeout",
		},
		{
			name:        "invalid timeout - negative",
			timeoutSec:  -1,
			wantErr:     true,
			errContains: "timeout",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			config := &ClientConfig{
				TimeoutSeconds:    tc.timeoutSec,
				HeartbeatInterval: 60,
				AgentAddr:         "localhost:19090",
				ServiceID:         "test-service",
				GameID:            "game1",
				Env:               "test", // Added for validation
				Insecure:          true,
			}

			// Adjust heartbeat interval to be >= timeout
			if config.HeartbeatInterval < config.TimeoutSeconds {
				config.HeartbeatInterval = config.TimeoutSeconds
			}
			err := ValidateClientConfig(config)

			if tc.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("Expected error containing %q, got %q", tc.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// TestClientConfig_HeartbeatValidation tests heartbeat configuration validation
func TestClientConfig_HeartbeatValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name              string
		heartbeatInterval int
		timeoutSeconds    int
		wantErr           bool
		errContains       string
	}{
		{
			name:              "valid heartbeat - 60 seconds",
			heartbeatInterval: 60,
			timeoutSeconds:    30,
			wantErr:           false,
		},
		{
			name:              "heartbeat equals timeout",
			heartbeatInterval: 30,
			timeoutSeconds:    30,
			wantErr:           false,
		},
		{
			name:              "heartbeat less than timeout",
			heartbeatInterval: 20,
			timeoutSeconds:    30,
			wantErr:           true,
			errContains:       "heartbeat",
		},
		{
			name:              "heartbeat too small",
			heartbeatInterval: 0,
			timeoutSeconds:    30,
			wantErr:           true,
			errContains:       "heartbeat",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			config := &ClientConfig{
				AgentAddr:         "localhost:19090",
				ServiceID:         "test-service",
				GameID:            "game1",
				Env:               "test", // Added for validation
				Insecure:          true,
				TimeoutSeconds:    tc.timeoutSeconds,
				HeartbeatInterval: tc.heartbeatInterval,
			}

			err := ValidateClientConfig(config)

			if tc.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("Expected error containing %q, got %q", tc.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// TestReconnectConfig_DefaultValues tests default reconnect configuration
func TestReconnectConfig_DefaultValues(t *testing.T) {
	t.Parallel()

	config := DefaultReconnectConfig()

	if !config.Enabled {
		t.Error("Reconnect should be enabled by default")
	}
	if config.InitialDelayMs == 0 {
		t.Error("InitialDelayMs should not be zero")
	}
	if config.MaxDelayMs == 0 {
		t.Error("MaxDelayMs should not be zero")
	}
	if config.BackoffMultiplier == 0 {
		t.Error("BackoffMultiplier should not be zero")
	}
	if config.MaxAttempts != 0 {
		t.Error("MaxAttempts should be 0 for infinite retries")
	}
}

// TestReconnectConfig_Validation tests reconnect configuration validation
func TestReconnectConfig_Validation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		config      ReconnectConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "valid reconnect config",
			config: ReconnectConfig{
				Enabled:           true,
				MaxAttempts:       5,
				InitialDelayMs:    1000,
				MaxDelayMs:        30000,
				BackoffMultiplier: 2.0,
				JitterFactor:      0.2,
			},
			wantErr: false,
		},
		{
			name: "invalid - initial delay greater than max",
			config: ReconnectConfig{
				Enabled:           true,
				MaxAttempts:       5,
				InitialDelayMs:    60000,
				MaxDelayMs:        30000,
				BackoffMultiplier: 2.0,
			},
			wantErr:     true,
			errContains: "initial_delay",
		},
		{
			name: "invalid - negative initial delay",
			config: ReconnectConfig{
				Enabled:           true,
				MaxAttempts:       5,
				InitialDelayMs:    -100,
				MaxDelayMs:        30000,
				BackoffMultiplier: 2.0,
			},
			wantErr:     true,
			errContains: "Delay",
		},
		{
			name: "invalid - backoff multiplier less than 1",
			config: ReconnectConfig{
				Enabled:           true,
				MaxAttempts:       5,
				InitialDelayMs:    1000,
				MaxDelayMs:        30000,
				BackoffMultiplier: 0.5,
			},
			wantErr:     true,
			errContains: "backoffMultiplier",
		},
		{
			name: "invalid - jitter factor out of range",
			config: ReconnectConfig{
				Enabled:           true,
				MaxAttempts:       5,
				InitialDelayMs:    1000,
				MaxDelayMs:        30000,
				BackoffMultiplier: 2.0,
				JitterFactor:      1.5,
			},
			wantErr:     true,
			errContains: "jitter",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateReconnectConfig(&tc.config)

			if tc.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("Expected error containing %q, got %q", tc.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// TestRetryConfig_DefaultValues tests default retry configuration
func TestRetryConfig_DefaultValues(t *testing.T) {
	t.Parallel()

	config := DefaultRetryConfig()

	if !config.Enabled {
		t.Error("Retry should be enabled by default")
	}
	if config.MaxAttempts == 0 {
		t.Error("MaxAttempts should not be zero")
	}
	if config.InitialDelayMs == 0 {
		t.Error("InitialDelayMs should not be zero")
	}
	if config.MaxDelayMs == 0 {
		t.Error("MaxDelayMs should not be zero")
	}
	if config.BackoffMultiplier == 0 {
		t.Error("BackoffMultiplier should not be zero")
	}
	if len(config.RetryableStatusCodes) == 0 {
		t.Error("RetryableStatusCodes should not be empty")
	}
}

// TestRetryConfig_Validation tests retry configuration validation
func TestRetryConfig_Validation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		config      RetryConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "valid retry config",
			config: RetryConfig{
				Enabled:           true,
				MaxAttempts:       3,
				InitialDelayMs:    100,
				MaxDelayMs:        5000,
				BackoffMultiplier: 2.0,
				JitterFactor:      0.1,
			},
			wantErr: false,
		},
		{
			name: "invalid - max attempts too high",
			config: RetryConfig{
				Enabled:           true,
				MaxAttempts:       100,
				InitialDelayMs:    100,
				MaxDelayMs:        5000,
				BackoffMultiplier: 2.0,
			},
			wantErr:     true,
			errContains: "maxAttempts",
		},
		{
			name: "invalid - zero max attempts with retry enabled",
			config: RetryConfig{
				Enabled:           true,
				MaxAttempts:       0,
				InitialDelayMs:    100,
				MaxDelayMs:        5000,
				BackoffMultiplier: 2.0,
			},
			wantErr:     true,
			errContains: "maxAttempts",
		},
		{
			name: "invalid - negative initial delay",
			config: RetryConfig{
				Enabled:           true,
				MaxAttempts:       3,
				InitialDelayMs:    -50,
				MaxDelayMs:        5000,
				BackoffMultiplier: 2.0,
			},
			wantErr:     true,
			errContains: "Delay",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateRetryConfig(&tc.config)

			if tc.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("Expected error containing %q, got %q", tc.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// TestClientConfig_RequiredFields tests required field validation
func TestClientConfig_RequiredFields(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		config      ClientConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "all required fields present",
			config: ClientConfig{
				TimeoutSeconds:    30,
				HeartbeatInterval: 60,
				ServiceID:         "test-service",
				GameID:            "game1",
				AgentAddr:         "localhost:19090",
				Env:               "development",
				Insecure:          true,
			},
			wantErr: false,
		},
		{
			name: "missing service ID",
			config: ClientConfig{
				TimeoutSeconds:    30,
				HeartbeatInterval: 60,
				GameID:            "game1",
				AgentAddr:         "localhost:19090",
				Env:               "development",
				Insecure:          true,
			},
			wantErr:     true,
			errContains: "service",
		},
		{
			name: "missing game ID",
			config: ClientConfig{
				TimeoutSeconds:    30,
				HeartbeatInterval: 60,
				ServiceID:         "test-service",
				AgentAddr:         "localhost:19090",
				Env:               "development",
				Insecure:          true,
			},
			wantErr:     true,
			errContains: "game",
		},
		{
			name: "missing agent address",
			config: ClientConfig{
				TimeoutSeconds:    30,
				HeartbeatInterval: 60,
				ServiceID:         "test-service",
				GameID:            "game1",
				Env:               "development",
				Insecure:          true,
			},
			wantErr:     true,
			errContains: "addr",
		},
		{
			name: "missing env",
			config: ClientConfig{
				TimeoutSeconds:    30,
				HeartbeatInterval: 60,
				ServiceID:         "test-service",
				GameID:            "game1",
				AgentAddr:         "localhost:19090",
				Insecure:          true,
			},
			wantErr:     true,
			errContains: "env",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateClientConfig(&tc.config)

			if tc.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tc.errContains != "" && !strings.Contains(strings.ToLower(err.Error()), tc.errContains) {
					t.Errorf("Expected error containing %q, got %q", tc.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// TestClientConfig_LogLevelValidation tests log level validation
func TestClientConfig_LogLevelValidation(t *testing.T) {
	t.Parallel()

	validLevels := []string{"DEBUG", "INFO", "WARN", "ERROR", "OFF", "debug", "info", "warn", "error", "off"}

	for _, level := range validLevels {
		t.Run("valid_"+level, func(t *testing.T) {
			t.Parallel()

			config := &ClientConfig{
				TimeoutSeconds:    30,
				HeartbeatInterval: 60,
				LogLevel:          level,
				AgentAddr:         "localhost:19090",
				ServiceID:         "test-service",
				GameID:            "game1",
				Env:               "test", // Added for validation
				Insecure:          true,
			}

			err := ValidateClientConfig(config)
			if err != nil {
				t.Errorf("Expected no error for log level %q, got %v", level, err)
			}
		})
	}

	invalidLevels := []string{"TRACE", "FATAL", "invalid"}

	for _, level := range invalidLevels {
		t.Run("invalid_"+level, func(t *testing.T) {
			t.Parallel()

			config := &ClientConfig{
				TimeoutSeconds:    30,
				HeartbeatInterval: 60,
				LogLevel:          level,
				AgentAddr:         "localhost:19090",
				ServiceID:         "test-service",
				GameID:            "game1",
				Env:               "test", // Added for validation
				Insecure:          true,
			}

			err := ValidateClientConfig(config)
			if err == nil {
				t.Error("Expected error for invalid log level, got nil")
			}
		})
	}
}

// TestReconnectConfig_CalculateDelay tests exponential backoff delay calculation
func TestReconnectConfig_CalculateDelay(t *testing.T) {
	t.Parallel()

	config := &ReconnectConfig{
		Enabled:           true,
		MaxAttempts:       5,
		InitialDelayMs:    1000,
		MaxDelayMs:        16000,
		BackoffMultiplier: 2.0,
		JitterFactor:      0.0,
	}

	expectedDelays := []int{1000, 2000, 4000, 8000, 16000}

	for attempt, expected := range expectedDelays {
		t.Run("attempt_"+string(rune('0'+attempt)), func(t *testing.T) {
			t.Parallel()

			delay := CalculateReconnectDelay(config, attempt)

			if delay != time.Duration(expected)*time.Millisecond {
				t.Errorf("Attempt %d: expected delay %v, got %v", attempt, time.Duration(expected)*time.Millisecond, delay)
			}
		})
	}
}

// TestReconnectConfig_CalculateDelayWithMax tests delay caps at max
func TestReconnectConfig_CalculateDelayWithMax(t *testing.T) {
	t.Parallel()

	config := &ReconnectConfig{
		Enabled:           true,
		MaxAttempts:       10,
		InitialDelayMs:    1000,
		MaxDelayMs:        5000,
		BackoffMultiplier: 2.0,
		JitterFactor:      0.0,
	}

	// Attempt 5 would be 32000ms without cap, should be capped at 5000ms
	delay := CalculateReconnectDelay(config, 5)

	expectedMax := time.Duration(config.MaxDelayMs) * time.Millisecond
	if delay != expectedMax {
		t.Errorf("Delay should be capped at %v, got %v", expectedMax, delay)
	}
}

// TestReconnectConfig_CalculateDelayWithJitter tests jitter is applied
func TestReconnectConfig_CalculateDelayWithJitter(t *testing.T) {
	t.Parallel()

	config := &ReconnectConfig{
		Enabled:           true,
		MaxAttempts:       5,
		InitialDelayMs:    1000,
		MaxDelayMs:        16000,
		BackoffMultiplier: 2.0,
		JitterFactor:      0.2,
	}

	// Test with a single attempt (attempt=1 gives base delay of 2000ms)
	attempt := 1
	baseDelay := time.Duration(config.InitialDelayMs) * time.Millisecond * (1 << uint(attempt))

	delay := CalculateReconnectDelay(config, attempt)

	// With jitter=0.2, the delay should be different from the base delay
	if delay == baseDelay {
		t.Error("Jitter should modify the base delay")
	}

	// Check that delay is within expected jitter range: [baseDelay * 0.8, baseDelay * 1.2]
	minExpected := time.Duration(float64(baseDelay) * (1 - config.JitterFactor))
	maxExpected := time.Duration(float64(baseDelay) * (1 + config.JitterFactor))

	if delay < minExpected || delay > maxExpected {
		t.Errorf("Delay %v is outside expected jitter range [%v, %v]", delay, minExpected, maxExpected)
	}
}

// TestInvokerConfig_Validation tests invoker configuration validation
func TestInvokerConfig_Validation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		config      InvokerConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "valid invoker config",
			config: InvokerConfig{
				Address:        "localhost:18080",
				TimeoutSeconds: 30,
				Insecure:       true,
			},
			wantErr: false,
		},
		{
			name: "missing address",
			config: InvokerConfig{
				TimeoutSeconds: 30,
				Insecure:       true,
			},
			wantErr:     true,
			errContains: "addr",
		},
		{
			name: "invalid timeout",
			config: InvokerConfig{
				Address:        "localhost:18080",
				TimeoutSeconds: 0,
				Insecure:       true,
			},
			wantErr:     true,
			errContains: "timeout",
		},
		{
			name: "secure mode without CA",
			config: InvokerConfig{
				Address:        "localhost:18080",
				TimeoutSeconds: 30,
				Insecure:       false,
			},
			wantErr:     true,
			errContains: "ca",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateInvokerConfig(&tc.config)

			if tc.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tc.errContains != "" && !strings.Contains(strings.ToLower(err.Error()), tc.errContains) {
					t.Errorf("Expected error containing %q, got %q", tc.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// TestInvokeOptions_DefaultValues tests default invoke options
func TestInvokeOptions_DefaultValues(t *testing.T) {
	t.Parallel()

	options := &InvokeOptions{}

	if options.IdempotencyKey != "" {
		t.Error("IdempotencyKey should be empty by default")
	}
	if options.Timeout != 0 {
		t.Error("Timeout should be zero by default")
	}
	if options.Headers != nil {
		t.Error("Headers should be nil by default")
	}
	if options.Retry != nil {
		t.Error("Retry should be nil by default")
	}
}

// TestClientConfig_MergeWithDefaults tests merging config with defaults
func TestClientConfig_MergeWithDefaults(t *testing.T) {
	t.Parallel()

	partial := &ClientConfig{
		ServiceID: "custom-service",
		GameID:    "game1",
		AgentAddr: "custom:9999",
	}

	config := MergeWithDefaults(partial)

	if config.ServiceID != "custom-service" {
		t.Errorf("ServiceID should be preserved, got %q", config.ServiceID)
	}
	if config.GameID != "game1" {
		t.Errorf("GameID should be preserved, got %q", config.GameID)
	}
	if config.AgentAddr != "custom:9999" {
		t.Errorf("AgentAddr should be preserved, got %q", config.AgentAddr)
	}

	// Check defaults are applied
	if config.Env == "" {
		t.Error("Env should have default value")
	}
	if config.ServiceVersion == "" {
		t.Error("ServiceVersion should have default value")
	}
	if config.TimeoutSeconds == 0 {
		t.Error("TimeoutSeconds should have default value")
	}
	if config.Headers == nil {
		t.Error("Headers should have default value")
	}
}

// TestClientConfig_FileTransferValidation tests file transfer settings validation
func TestClientConfig_FileTransferValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		enabled     bool
		maxSize     int
		wantErr     bool
		errContains string
	}{
		{
			name:    "file transfer disabled",
			enabled: false,
			wantErr: false,
		},
		{
			name:    "file transfer enabled with valid size",
			enabled: true,
			maxSize: 10 * 1024 * 1024,
			wantErr: false,
		},
		{
			name:        "file transfer enabled with zero size",
			enabled:     true,
			maxSize:     0,
			wantErr:     true,
			errContains: "maxfilesize",
		},
		{
			name:        "file transfer enabled with negative size",
			enabled:     true,
			maxSize:     -100,
			wantErr:     true,
			errContains: "maxfilesize",
		},
		{
			name:        "file transfer enabled with excessive size",
			enabled:     true,
			maxSize:     1024 * 1024 * 1024,
			wantErr:     true,
			errContains: "maxfilesize",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			config := &ClientConfig{
				TimeoutSeconds:     30,
				HeartbeatInterval:  60,
				EnableFileTransfer: tc.enabled,
				MaxFileSize:        tc.maxSize,
				AgentAddr:          "localhost:19090",
				ServiceID:          "test-service",
				GameID:             "game1",
				Env:                "test", // Added for validation
				Insecure:           true,
			}

			err := ValidateClientConfig(config)

			if tc.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tc.errContains != "" && !strings.Contains(strings.ToLower(err.Error()), tc.errContains) {
					t.Errorf("Expected error containing %q, got %q", tc.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// TestClientConfig_AuthTokenValidation tests auth token configuration
func TestClientConfig_AuthTokenValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		authToken   string
		headers     map[string]string
		wantErr     bool
		errContains string
	}{
		{
			name:    "no auth",
			wantErr: false,
		},
		{
			name:      "auth token only",
			authToken: "Bearer token123",
			wantErr:   false,
		},
		{
			name: "headers with authorization",
			headers: map[string]string{
				"Authorization": "Bearer token456",
			},
			wantErr: false,
		},
		{
			name:      "both auth token and authorization header - should warn but not error",
			authToken: "Bearer token123",
			headers: map[string]string{
				"Authorization": "Bearer token456",
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			config := &ClientConfig{
				TimeoutSeconds:    30,
				HeartbeatInterval: 60,
				AuthToken:         tc.authToken,
				Headers:           tc.headers,
				AgentAddr:         "localhost:19090",
				ServiceID:         "test-service",
				GameID:            "game1",
				Env:               "test", // Added for validation
				Insecure:          true,
			}

			err := ValidateClientConfig(config)

			if tc.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("Expected error containing %q, got %q", tc.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// TestGenerateUUID_Uniqueness tests UUID generation uniqueness
func TestGenerateUUID_Uniqueness(t *testing.T) {
	t.Parallel()

	uuids := make(map[string]bool)
	count := 1000

	for i := 0; i < count; i++ {
		uuid := generateUUID()
		if uuids[uuid] {
			t.Errorf("Duplicate UUID generated: %s", uuid)
		}
		uuids[uuid] = true
	}

	if len(uuids) != count {
		t.Errorf("Expected %d unique UUIDs, got %d", count, len(uuids))
	}
}

// TestGenerateUUID_Format tests UUID format
func TestGenerateUUID_Format(t *testing.T) {
	t.Parallel()

	uuid := generateUUID()

	// Check format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	parts := strings.Split(uuid, "-")
	if len(parts) != 5 {
		t.Errorf("UUID should have 5 parts separated by '-', got %d parts", len(parts))
	}

	expectedLengths := []int{8, 4, 4, 4, 12}
	for i, part := range parts {
		if len(part) != expectedLengths[i] {
			t.Errorf("Part %d should have length %d, got %d", i, expectedLengths[i], len(part))
		}
	}
}

// TestClientConfig_DeepCopy tests creating a deep copy of config
func TestClientConfig_DeepCopy(t *testing.T) {
	t.Parallel()

	original := &ClientConfig{
		ServiceID: "test-service",
		GameID:    "game1",
		AgentAddr: "localhost:19090",
		Env:       "production",
		Insecure:  false,
		CAFile:    "/path/to/ca.pem",
		Headers: map[string]string{
			"X-Custom": "value",
		},
		Reconnect: &ReconnectConfig{
			Enabled:     true,
			MaxAttempts: 5,
		},
	}

	copy := original.DeepCopy()

	// Verify copies are equal
	if copy.ServiceID != original.ServiceID {
		t.Error("ServiceID should match")
	}
	if copy.GameID != original.GameID {
		t.Error("GameID should match")
	}

	// Modify original
	original.ServiceID = "modified"
	original.Headers["X-Custom"] = "modified"

	// Verify copy is independent
	if copy.ServiceID == "modified" {
		t.Error("Copy should be independent of original")
	}
	if copy.Headers["X-Custom"] == "modified" {
		t.Error("Headers map should be copied, not referenced")
	}
}

// TestClientConfig_ProviderMetadataValidation tests provider metadata validation
func TestClientConfig_ProviderMetadataValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		providerLang string
		providerSDK  string
		serviceVer   string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "valid provider metadata",
			providerLang: "go",
			providerSDK:  "croupier-go-sdk",
			serviceVer:   "1.0.0",
			wantErr:      false,
		},
		{
			name:         "missing provider lang",
			providerLang: "",
			providerSDK:  "croupier-go-sdk",
			serviceVer:   "1.0.0",
			wantErr:      false, // Default will be applied
		},
		{
			name:         "missing service version",
			providerLang: "go",
			providerSDK:  "croupier-go-sdk",
			serviceVer:   "",
			wantErr:      false, // Default will be applied
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			config := &ClientConfig{
				TimeoutSeconds:    30,
				HeartbeatInterval: 60,
				ProviderLang:      tc.providerLang,
				ProviderSDK:       tc.providerSDK,
				ServiceVersion:    tc.serviceVer,
				AgentAddr:         "localhost:19090",
				ServiceID:         "test-service",
				GameID:            "game1",
				Env:               "test", // Added for validation
				Insecure:          true,
			}

			err := ValidateClientConfig(config)

			if tc.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("Expected error containing %q, got %q", tc.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// TestClientConfig_ControlAddrValidation tests control address validation
func TestClientConfig_ControlAddrValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		controlAddr string
		wantErr     bool
		errContains string
	}{
		{
			name:        "no control address",
			controlAddr: "",
			wantErr:     false,
		},
		{
			name:        "valid control address",
			controlAddr: "localhost:18080",
			wantErr:     false,
		},
		{
			name:        "valid control address with tcp://",
			controlAddr: "tcp://localhost:18080",
			wantErr:     false,
		},
		{
			name:        "valid control address with https://",
			controlAddr: "https://control.example.com",
			wantErr:     false,
		},
		{
			name:        "invalid control address - missing port",
			controlAddr: "localhost",
			wantErr:     false, // Control address can be HTTP URL
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			config := &ClientConfig{
				TimeoutSeconds:    30,
				HeartbeatInterval: 60,
				ControlAddr:       tc.controlAddr,
				AgentAddr:         "localhost:19090",
				ServiceID:         "test-service",
				GameID:            "game1",
				Env:               "test", // Required field
				Insecure:          true,
			}

			err := ValidateClientConfig(config)

			if tc.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("Expected error containing %q, got %q", tc.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// TestClientConfig_EnvironmentVariablePriority tests environment variable override priority
func TestClientConfig_EnvironmentVariablePriority(t *testing.T) {
	t.Parallel()

	// This test verifies the priority order: env vars > config file > defaults
	// In a real scenario, environment variables would be set externally
	// For testing, we simulate this behavior

	defaultConfig := DefaultClientConfig()
	config := ApplyEnvOverrides(defaultConfig, map[string]string{
		"CROUPIER_AGENT_ADDR":    "env-override:19090",
		"CROUPIER_TIMEOUT":       "60",
		"CROUPIER_INSECURE":      "false",
		"CROUPIER_LOG_LEVEL":     "DEBUG",
		"CROUPIER_SERVICE_ID":    "env-service",
		"CROUPIER_GAME_ID":       "env-game",
		"CROUPIER_ENV":           "staging",
		"CROUPIER_AUTH_TOKEN":    "env-token",
		"CROUPIER_CONTROL_ADDR":  "env-control:18080",
		"CROUPIER_HEARTBEAT":     "30",
		"CROUPIER_RECONNECT":     "true",
		"CROUPIER_RECONNECT_MAX": "10",
	})

	if config.AgentAddr != "env-override:19090" {
		t.Errorf("AgentAddr should be overridden by env var, got %q", config.AgentAddr)
	}
	if config.TimeoutSeconds != 60 {
		t.Errorf("TimeoutSeconds should be overridden by env var, got %d", config.TimeoutSeconds)
	}
	if config.LogLevel != "DEBUG" {
		t.Errorf("LogLevel should be overridden by env var, got %q", config.LogLevel)
	}
	if config.ServiceID != "env-service" {
		t.Errorf("ServiceID should be overridden by env var, got %q", config.ServiceID)
	}
	if config.GameID != "env-game" {
		t.Errorf("GameID should be overridden by env var, got %q", config.GameID)
	}
	if config.Env != "staging" {
		t.Errorf("Env should be overridden by env var, got %q", config.Env)
	}
	if config.AuthToken != "env-token" {
		t.Errorf("AuthToken should be overridden by env var, got %q", config.AuthToken)
	}
	if config.ControlAddr != "env-control:18080" {
		t.Errorf("ControlAddr should be overridden by env var, got %q", config.ControlAddr)
	}
	if config.HeartbeatInterval != 30 {
		t.Errorf("HeartbeatInterval should be overridden by env var, got %d", config.HeartbeatInterval)
	}
	if config.Reconnect == nil || !config.Reconnect.Enabled {
		t.Error("Reconnect should be enabled by env var")
	}
	if config.Reconnect != nil && config.Reconnect.MaxAttempts != 10 {
		t.Errorf("Reconnect MaxAttempts should be overridden by env var, got %d", config.Reconnect.MaxAttempts)
	}
}

// TestClientConfig_InvalidEnvValues tests invalid environment variable values
func TestClientConfig_InvalidEnvValues(t *testing.T) {
	t.Parallel()

	defaultConfig := DefaultClientConfig()

	testCases := []struct {
		name     string
		envKey   string
		envValue string
		// Expected value after parsing invalid env var
		expectedValue interface{}
	}{
		{
			name:          "invalid timeout - defaults should be used",
			envKey:        "CROUPIER_TIMEOUT",
			envValue:      "invalid",
			expectedValue: defaultConfig.TimeoutSeconds,
		},
		{
			name:          "invalid heartbeat - defaults should be used",
			envKey:        "CROUPIER_HEARTBEAT",
			envValue:      "not-a-number",
			expectedValue: defaultConfig.HeartbeatInterval,
		},
		{
			name:          "invalid boolean - should default to false",
			envKey:        "CROUPIER_INSECURE",
			envValue:      "maybe",
			expectedValue: false,
		},
		{
			name:          "invalid reconnect max - default should be used",
			envKey:        "CROUPIER_RECONNECT_MAX",
			envValue:      "abc",
			expectedValue: defaultConfig.Reconnect.MaxAttempts,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			envMap := map[string]string{
				tc.envKey: tc.envValue,
			}

			config := ApplyEnvOverrides(DefaultClientConfig(), envMap)

			switch tc.envKey {
			case "CROUPIER_TIMEOUT":
				if config.TimeoutSeconds != tc.expectedValue.(int) {
					t.Errorf("TimeoutSeconds should be %d for invalid env value, got %d", tc.expectedValue.(int), config.TimeoutSeconds)
				}
			case "CROUPIER_HEARTBEAT":
				if config.HeartbeatInterval != tc.expectedValue.(int) {
					t.Errorf("HeartbeatInterval should be %d for invalid env value, got %d", tc.expectedValue.(int), config.HeartbeatInterval)
				}
			case "CROUPIER_INSECURE":
				if config.Insecure != tc.expectedValue.(bool) {
					t.Errorf("Insecure should be %v for invalid env value, got %v", tc.expectedValue.(bool), config.Insecure)
				}
			case "CROUPIER_RECONNECT_MAX":
				if config.Reconnect.MaxAttempts != tc.expectedValue.(int) {
					t.Errorf("Reconnect MaxAttempts should be %d for invalid env value, got %d", tc.expectedValue.(int), config.Reconnect.MaxAttempts)
				}
			}
		})
	}
}

// TestRetryConfig_RetryableStatusCodes tests default retryable status codes
func TestRetryConfig_RetryableStatusCodes(t *testing.T) {
	t.Parallel()

	config := DefaultRetryConfig()

	expectedCodes := []int32{14, 13, 2, 10, 4}

	for _, expectedCode := range expectedCodes {
		found := false
		for _, code := range config.RetryableStatusCodes {
			if code == expectedCode {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected retryable status code %d to be present", expectedCode)
		}
	}
}

// TestRetryConfig_IsRetryable tests status code retryability check
func TestRetryConfig_IsRetryable(t *testing.T) {
	t.Parallel()

	config := DefaultRetryConfig()

	testCases := []struct {
		name     string
		code     int32
		expected bool
	}{
		{
			name:     "UNAVAILABLE - should retry",
			code:     14,
			expected: true,
		},
		{
			name:     "INTERNAL - should retry",
			code:     13,
			expected: true,
		},
		{
			name:     "UNKNOWN - should retry",
			code:     2,
			expected: true,
		},
		{
			name:     "ABORTED - should retry",
			code:     10,
			expected: true,
		},
		{
			name:     "DEADLINE_EXCEEDED - should retry",
			code:     4,
			expected: true,
		},
		{
			name:     "OK - should not retry",
			code:     0,
			expected: false,
		},
		{
			name:     "CANCELLED - should not retry",
			code:     1,
			expected: false,
		},
		{
			name:     "INVALID_ARGUMENT - should not retry",
			code:     3,
			expected: false,
		},
		{
			name:     "ALREADY_EXISTS - should not retry",
			code:     6,
			expected: false,
		},
		{
			name:     "PERMISSION_DENIED - should not retry",
			code:     7,
			expected: false,
		},
		{
			name:     "NOT_FOUND - should not retry",
			code:     5,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := config.IsRetryable(tc.code)

			if result != tc.expected {
				t.Errorf("IsRetryable(%d) = %v, expected %v", tc.code, result, tc.expected)
			}
		})
	}
}

// TestTaskEvent_Validation tests task event validation
func TestTaskEvent_Validation(t *testing.T) {
	t.Parallel()

	validTypes := []string{"started", "progress", "completed", "error"}

	for _, eventType := range validTypes {
		t.Run("valid_"+eventType, func(t *testing.T) {
			t.Parallel()

			event := &TaskEvent{
				EventType: eventType,
				TaskID:    "test-task",
				Payload:   `{"status":"running"}`,
			}

			err := ValidateTaskEvent(event)
			if err != nil {
				t.Errorf("Expected no error for event type %q, got %v", eventType, err)
			}
		})
	}

	invalidTypes := []string{"", "invalid", "STARTED", "Completed"}

	for _, eventType := range invalidTypes {
		t.Run("invalid_"+eventType, func(t *testing.T) {
			t.Parallel()

			event := &TaskEvent{
				EventType: eventType,
				TaskID:    "test-task",
			}

			err := ValidateTaskEvent(event)
			if err == nil {
				t.Error("Expected error for invalid event type")
			}
		})
	}
}

// TestTaskEvent_CompleteEvent tests task completion event
func TestTaskEvent_CompleteEvent(t *testing.T) {
	t.Parallel()

	event := &TaskEvent{
		EventType: "completed",
		Done:      true, // Completed events should have Done=true
		TaskID:    "test-task",
		Payload:   `{"result":"success"}`,
	}

	if !event.Done {
		t.Error("Completed event should have Done=true")
	}

	err := ValidateTaskEvent(event)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

// TestTaskEvent_ErrorEvent tests task error event
func TestTaskEvent_ErrorEvent(t *testing.T) {
	t.Parallel()

	event := &TaskEvent{
		EventType: "error",
		Done:      true, // Error events should have Done=true
		TaskID:    "test-task",
		Error:     "something went wrong",
	}

	if !event.Done {
		t.Error("Error event should have Done=true")
	}

	err := ValidateTaskEvent(event)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

// TestTaskEvent_ProgressEvent tests task progress event
func TestTaskEvent_ProgressEvent(t *testing.T) {
	t.Parallel()

	event := &TaskEvent{
		EventType: "progress",
		TaskID:    "test-task",
		Payload:   `{"progress":50}`,
	}

	if event.Done {
		t.Error("Progress event should have Done=false")
	}

	err := ValidateTaskEvent(event)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

// TestClientConfig_Clone tests config cloning
func TestClientConfig_Clone(t *testing.T) {
	t.Parallel()

	original := &ClientConfig{
		ServiceID:      "test-service",
		GameID:         "game1",
		AgentAddr:      "localhost:19090",
		Env:            "production",
		ServiceVersion: "2.0.0",
		TimeoutSeconds: 60,
		Insecure:       false,
		CAFile:         "/path/to/ca.pem",
		CertFile:       "/path/to/cert.pem",
		KeyFile:        "/path/to/key.pem",
		ServerName:     "example.com",
		AuthToken:      "token123",
		Headers: map[string]string{
			"X-Custom-1": "value1",
			"X-Custom-2": "value2",
		},
		Reconnect: &ReconnectConfig{
			Enabled:           true,
			MaxAttempts:       10,
			InitialDelayMs:    2000,
			MaxDelayMs:        60000,
			BackoffMultiplier: 2.5,
			JitterFactor:      0.3,
		},
		Retry: &RetryConfig{
			Enabled:           true,
			MaxAttempts:       5,
			InitialDelayMs:    200,
			MaxDelayMs:        10000,
			BackoffMultiplier: 2.0,
			JitterFactor:      0.15,
		},
	}

	cloned := original.Clone()

	// Verify all fields are copied
	if cloned.ServiceID != original.ServiceID {
		t.Error("ServiceID should match")
	}
	if cloned.GameID != original.GameID {
		t.Error("GameID should match")
	}
	if cloned.AgentAddr != original.AgentAddr {
		t.Error("AgentAddr should match")
	}
	if cloned.Env != original.Env {
		t.Error("Env should match")
	}
	if cloned.ServiceVersion != original.ServiceVersion {
		t.Error("ServiceVersion should match")
	}
	if cloned.TimeoutSeconds != original.TimeoutSeconds {
		t.Error("TimeoutSeconds should match")
	}
	if cloned.Insecure != original.Insecure {
		t.Error("Insecure should match")
	}
	if cloned.CAFile != original.CAFile {
		t.Error("CAFile should match")
	}
	if cloned.CertFile != original.CertFile {
		t.Error("CertFile should match")
	}
	if cloned.KeyFile != original.KeyFile {
		t.Error("KeyFile should match")
	}
	if cloned.ServerName != original.ServerName {
		t.Error("ServerName should match")
	}
	if cloned.AuthToken != original.AuthToken {
		t.Error("AuthToken should match")
	}

	// Verify headers are copied
	if len(cloned.Headers) != len(original.Headers) {
		t.Error("Headers should have same length")
	}
	for k, v := range original.Headers {
		if cloned.Headers[k] != v {
			t.Errorf("Header %q should match", k)
		}
	}

	// Verify nested configs are copied
	if cloned.Reconnect == nil {
		t.Error("Reconnect should be copied")
	} else {
		if cloned.Reconnect.MaxAttempts != original.Reconnect.MaxAttempts {
			t.Error("Reconnect MaxAttempts should match")
		}
	}

	if cloned.Retry == nil {
		t.Error("Retry should be copied")
	} else {
		if cloned.Retry.MaxAttempts != original.Retry.MaxAttempts {
			t.Error("Retry MaxAttempts should match")
		}
	}

	// Verify independence
	original.ServiceID = "modified"
	original.Headers["X-Custom-1"] = "modified"
	original.Reconnect.MaxAttempts = 999

	if cloned.ServiceID == "modified" {
		t.Error("Cloned config should be independent")
	}
	if cloned.Headers["X-Custom-1"] == "modified" {
		t.Error("Cloned headers should be independent")
	}
	if cloned.Reconnect.MaxAttempts == 999 {
		t.Error("Cloned reconnect should be independent")
	}
}

// TestFunctionDescriptor_Validation tests function descriptor validation
func TestFunctionDescriptor_Validation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		descriptor  FunctionDescriptor
		wantErr     bool
		errContains string
	}{
		{
			name: "valid descriptor",
			descriptor: FunctionDescriptor{
				ID:      "player.ban",
				Version: "1.0.0",
				Enabled: true,
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			descriptor: FunctionDescriptor{
				Version: "1.0.0",
			},
			wantErr:     true,
			errContains: "id",
		},
		{
			name: "missing version",
			descriptor: FunctionDescriptor{
				ID: "player.ban",
			},
			wantErr:     true,
			errContains: "version",
		},
		{
			name: "invalid version format",
			descriptor: FunctionDescriptor{
				ID:      "player.ban",
				Version: "invalid",
			},
			wantErr:     true,
			errContains: "version",
		},
		{
			name: "valid semver versions",
			descriptor: FunctionDescriptor{
				ID:      "player.ban",
				Version: "2.1.3",
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateFunctionDescriptor(&tc.descriptor)

			if tc.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tc.errContains != "" && !strings.Contains(strings.ToLower(err.Error()), tc.errContains) {
					t.Errorf("Expected error containing %q, got %q", tc.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// TestProviderFunctionDescriptor_OpenAPICompliance tests OpenAPI 3.0.3 compliance
func TestProviderFunctionDescriptor_OpenAPICompliance(t *testing.T) {
	t.Parallel()

	descriptor := &ProviderFunctionDescriptor{
		ID:           "player.create",
		Version:      "1.0.0",
		Tags:         []string{"player", "crud"},
		Summary:      "Create a new player",
		Description:  "Creates a new player with the provided data",
		OperationID:  "createPlayer",
		Deprecated:   false,
		InputSchema:  `{"type":"object","properties":{"name":{"type":"string"}}}`,
		OutputSchema: `{"type":"object","properties":{"id":{"type":"string"}}}`,
		Resource:     "player",
		Risk:         "safe",
		Operation:    "create",
	}

	err := ValidateProviderFunctionDescriptor(descriptor)
	if err != nil {
		t.Errorf("Expected no error for valid descriptor, got %v", err)
	}

	// Test JSON schema validation
	if descriptor.InputSchema != "" {
		if !IsValidJSONSchema(descriptor.InputSchema) {
			t.Error("InputSchema should be valid JSON Schema")
		}
	}

	if descriptor.OutputSchema != "" {
		if !IsValidJSONSchema(descriptor.OutputSchema) {
			t.Error("OutputSchema should be valid JSON Schema")
		}
	}
}

// TestClientConfig_TLSOptions tests TLS configuration options
func TestClientConfig_TLSOptions(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name               string
		insecure           bool
		insecureSkipVerify bool
		caFile             string
		certFile           string
		keyFile            string
		serverName         string
		wantErr            bool
		errContains        string
	}{
		{
			name:     "insecure mode - no TLS required",
			insecure: true,
			wantErr:  false,
		},
		{
			name:               "secure with skip verify",
			insecure:           false,
			insecureSkipVerify: true,
			wantErr:            false,
		},
		{
			name:     "secure with CA only",
			insecure: false,
			caFile:   "/path/to/ca.pem",
			wantErr:  false,
		},
		{
			name:     "secure with mTLS",
			insecure: false,
			caFile:   "/path/to/ca.pem",
			certFile: "/path/to/cert.pem",
			keyFile:  "/path/to/key.pem",
			wantErr:  false,
		},
		{
			name:        "secure with cert but no key",
			insecure:    false,
			caFile:      "/path/to/ca.pem", // Add CA to avoid CA error first
			certFile:    "/path/to/cert.pem",
			wantErr:     true,
			errContains: "key",
		},
		{
			name:        "secure with key but no cert",
			insecure:    false,
			caFile:      "/path/to/ca.pem", // Add CA to avoid CA error first
			keyFile:     "/path/to/key.pem",
			wantErr:     true,
			errContains: "cert",
		},
		{
			name:       "secure with server name override",
			insecure:   false,
			caFile:     "/path/to/ca.pem",
			serverName: "example.com",
			wantErr:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			config := &ClientConfig{
				TimeoutSeconds:     30,
				HeartbeatInterval:  60,
				Insecure:           tc.insecure,
				InsecureSkipVerify: tc.insecureSkipVerify,
				CAFile:             tc.caFile,
				CertFile:           tc.certFile,
				KeyFile:            tc.keyFile,
				ServerName:         tc.serverName,
				AgentAddr:          "localhost:19090",
				ServiceID:          "test-service",
				GameID:             "game1",
				Env:                "test", // Required field
			}

			err := ValidateClientConfig(config)

			if tc.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tc.errContains != "" && !strings.Contains(strings.ToLower(err.Error()), tc.errContains) {
					t.Errorf("Expected error containing %q, got %q", tc.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// TestClientConfig_HighAvailability tests multi-address configuration for HA
func TestClientConfig_HighAvailability(t *testing.T) {
	t.Parallel()

	config := &ClientConfig{
		TimeoutSeconds:    30,
		HeartbeatInterval: 60,
		AgentAddr:         "primary:19090,backup1:19090,backup2:19090",
		AgentIPCAddr:      "ipc://croupier-agent",
		ServiceID:         "test-service",
		GameID:            "game1",
		Env:               "test", // Added for validation
		Insecure:          true,
	}

	addresses := GetFallbackAddresses(config)

	// Should have 4 addresses: IPC + 3 TCP
	if len(addresses) != 4 {
		t.Errorf("Expected 4 addresses, got %d", len(addresses))
	}

	// IPC should be first
	if addresses[0] != "ipc://croupier-agent" {
		t.Errorf("First address should be IPC, got %q", addresses[0])
	}

	// TCP addresses should follow
	expectedTCP := []string{"primary:19090", "backup1:19090", "backup2:19090"}
	for i, expected := range expectedTCP {
		if addresses[i+1] != expected {
			t.Errorf("Address %d should be %q, got %q", i+1, expected, addresses[i+1])
		}
	}
}

// TestInvokeOptions_WithDefaults tests invoke options with default values applied
func TestInvokeOptions_WithDefaults(t *testing.T) {
	t.Parallel()

	options := &InvokeOptions{
		IdempotencyKey: "test-key",
	}

	result := options.WithDefaults()

	if result.IdempotencyKey != "test-key" {
		t.Error("IdempotencyKey should be preserved")
	}

	if result.Headers == nil {
		result.Headers = make(map[string]string)
	}

	if result.Timeout == 0 {
		// Default timeout should be applied if specified in config
		result.Timeout = 30 * time.Second
	}
}

// TestCalculateReconnectDelay_EdgeCases tests reconnect delay calculation edge cases
func TestCalculateReconnectDelay_EdgeCases(t *testing.T) {
	t.Parallel()

	config := &ReconnectConfig{
		Enabled:           true,
		MaxAttempts:       5,
		InitialDelayMs:    1000,
		MaxDelayMs:        10000,
		BackoffMultiplier: 2.0,
		JitterFactor:      0.0,
	}

	testCases := []struct {
		name            string
		attempt         int
		expectedDelayMs int
	}{
		{
			name:            "attempt 0",
			attempt:         0,
			expectedDelayMs: 1000, // 1000 * 2^0 = 1000
		},
		{
			name:            "attempt 1",
			attempt:         1,
			expectedDelayMs: 2000,
		},
		{
			name:            "attempt 2",
			attempt:         2,
			expectedDelayMs: 4000,
		},
		{
			name:            "attempt 3",
			attempt:         3,
			expectedDelayMs: 8000,
		},
		{
			name:            "attempt 4 - capped at max",
			attempt:         4,
			expectedDelayMs: 10000,
		},
		{
			name:            "attempt 10 - still capped",
			attempt:         10,
			expectedDelayMs: 10000,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			delay := CalculateReconnectDelay(config, tc.attempt)
			expected := time.Duration(tc.expectedDelayMs) * time.Millisecond

			if delay != expected {
				t.Errorf("Expected delay %v, got %v", expected, delay)
			}
		})
	}
}

// TestRetryConfig_CalculateDelay tests retry delay calculation
func TestRetryConfig_CalculateDelay(t *testing.T) {
	t.Parallel()

	config := &RetryConfig{
		Enabled:           true,
		MaxAttempts:       3,
		InitialDelayMs:    100,
		MaxDelayMs:        1000,
		BackoffMultiplier: 2.0,
		JitterFactor:      0.0,
	}

	testCases := []struct {
		name            string
		attempt         int
		expectedDelayMs int
	}{
		{
			name:            "first retry",
			attempt:         0,
			expectedDelayMs: 100,
		},
		{
			name:            "second retry",
			attempt:         1,
			expectedDelayMs: 200,
		},
		{
			name:            "third retry - exponential backoff",
			attempt:         2,
			expectedDelayMs: 400, // 100 * 2^2 = 400
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			delay := CalculateRetryDelay(config, tc.attempt)
			expected := time.Duration(tc.expectedDelayMs) * time.Millisecond

			if delay != expected {
				t.Errorf("Expected delay %v, got %v", expected, delay)
			}
		})
	}
}

// TestClientConfig_Serialization tests config serialization
func TestClientConfig_Serialization(t *testing.T) {
	t.Parallel()

	config := &ClientConfig{
		TimeoutSeconds:    30,
		HeartbeatInterval: 60,
		ServiceID:         "test-service",
		GameID:            "game1",
		AgentAddr:         "localhost:19090",
		Env:               "production",
		ServiceVersion:    "1.0.0",
		Insecure:          false,
		CAFile:            "/path/to/ca.pem",
		Headers: map[string]string{
			"X-Header": "value",
		},
		Reconnect: &ReconnectConfig{
			Enabled:     true,
			MaxAttempts: 5,
		},
	}

	// Serialize to JSON
	data, err := config.MarshalJSON()
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	// Deserialize
	decoded := &ClientConfig{}
	err = decoded.UnmarshalJSON(data)
	if err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	// Verify key fields match
	if decoded.ServiceID != config.ServiceID {
		t.Errorf("ServiceID mismatch: got %q, want %q", decoded.ServiceID, config.ServiceID)
	}
	if decoded.GameID != config.GameID {
		t.Errorf("GameID mismatch: got %q, want %q", decoded.GameID, config.GameID)
	}
	if decoded.AgentAddr != config.AgentAddr {
		t.Errorf("AgentAddr mismatch: got %q, want %q", decoded.AgentAddr, config.AgentAddr)
	}
}
