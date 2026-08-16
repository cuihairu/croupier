// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package croupier

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// TestIntegrationWithAgent tests real connection to croupier-agent.
// These tests run only when CROUPIER_RUN_INTEGRATION_TESTS=1. In that mode a
// missing Agent local SDK gateway is a test failure, never a false success.

const (
	defaultAgentAddr    = "localhost:19091"
	connectTimeout      = 5 * time.Second
	testServiceID       = "go-integration-test"
	testFunctionID      = "test.ping"
	testFunctionVersion = "1.0.0"
)

func getAgentAddr() string {
	if addr := os.Getenv("CROUPIER_AGENT_ADDR"); addr != "" {
		return addr
	}
	return defaultAgentAddr
}

func integrationTestsEnabled() bool {
	return os.Getenv("CROUPIER_RUN_INTEGRATION_TESTS") == "1"
}

func requireAgent(t *testing.T) {
	t.Helper()
	if !integrationTestsEnabled() {
		t.Skip("set CROUPIER_RUN_INTEGRATION_TESTS=1 to run integration tests")
	}
	if !isAgentAvailable(t) {
		t.Fatalf("croupier-agent local SDK gateway not available at %s", getAgentAddr())
	}
}

// isAgentAvailable attempts to connect to the agent and returns true if successful
func isAgentAvailable(t *testing.T) bool {
	config := &ClientConfig{
		AgentAddr:      getAgentAddr(),
		ServiceID:      "go-sdk-integration-probe",
		Insecure:       true,
		TimeoutSeconds: int(connectTimeout.Seconds()),
	}
	client := NewClient(config)

	// Register a test function
	desc := FunctionDescriptor{
		ID:      "test.probe",
		Version: "1.0.0",
	}
	handler := func(ctx context.Context, payload []byte) ([]byte, error) {
		return []byte("ok"), nil
	}

	_ = client.RegisterFunction(desc, handler)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Try to connect - if agent is not running, this will fail
	if err := client.Connect(ctx); err != nil {
		_ = client.Close()
		return false
	}
	_ = client.Close()
	return true
}

// TestIntegrationConnectToAgent tests real connection to agent
func TestIntegrationConnectToAgent(t *testing.T) {
	requireAgent(t)

	config := &ClientConfig{
		AgentAddr:         getAgentAddr(),
		ServiceID:         testServiceID,
		ServiceVersion:    "1.0.0",
		Insecure:          true,
		TimeoutSeconds:    int(connectTimeout.Seconds()),
		HeartbeatInterval: 30,
	}
	client := NewClient(config)

	// Register a test function
	desc := FunctionDescriptor{
		ID:      testFunctionID,
		Version: testFunctionVersion,
	}
	handler := func(ctx context.Context, payload []byte) ([]byte, error) {
		return []byte(fmt.Sprintf("pong: %s", string(payload))), nil
	}

	err := client.RegisterFunction(desc, handler)
	if err != nil {
		t.Fatalf("Failed to register function: %v", err)
	}

	// Connect to agent
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect to agent at %s: %v", getAgentAddr(), err)
	}

	// Clean up
	if err := client.Close(); err != nil {
		t.Errorf("Failed to close client: %v", err)
	}
}

// TestIntegrationConnectWithoutFunctions fails when no functions registered
func TestIntegrationConnectWithoutFunctions(t *testing.T) {
	requireAgent(t)

	config := &ClientConfig{
		AgentAddr:      getAgentAddr(),
		Insecure:       true,
		TimeoutSeconds: int(connectTimeout.Seconds()),
	}
	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	err := client.Connect(ctx)
	if err == nil {
		t.Error("Connect should fail when no functions are registered")
		_ = client.Close()
	}
	// Expected: error about registering at least one function
}

// TestIntegrationInvalidAddress fails with invalid agent address
func TestIntegrationInvalidAddress(t *testing.T) {
	config := &ClientConfig{
		AgentAddr:      "localhost:9999", // Non-existent port
		Insecure:       true,
		TimeoutSeconds: 5,
	}
	client := NewClient(config)

	// Register a function
	desc := FunctionDescriptor{
		ID:      testFunctionID,
		Version: testFunctionVersion,
	}
	handler := func(ctx context.Context, payload []byte) ([]byte, error) {
		return []byte("ok"), nil
	}

	_ = client.RegisterFunction(desc, handler)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err == nil {
		t.Error("Connect should fail with invalid address")
		_ = client.Close()
	}
	// Expected: connection refused or timeout error
}

// TestIntegrationReconnect tests reconnecting after disconnect
func TestIntegrationReconnect(t *testing.T) {
	requireAgent(t)

	config := &ClientConfig{
		AgentAddr:         getAgentAddr(),
		ServiceID:         testServiceID + "-reconnect",
		Insecure:          true,
		TimeoutSeconds:    int(connectTimeout.Seconds()),
		HeartbeatInterval: 30,
	}
	client := NewClient(config)

	desc := FunctionDescriptor{
		ID:      testFunctionID,
		Version: testFunctionVersion,
	}
	handler := func(ctx context.Context, payload []byte) ([]byte, error) {
		return []byte("ok"), nil
	}

	_ = client.RegisterFunction(desc, handler)

	// First connection
	ctx1, cancel1 := context.WithTimeout(context.Background(), connectTimeout)
	if err := client.Connect(ctx1); err != nil {
		t.Fatalf("First connect failed: %v", err)
	}
	cancel1()

	// Disconnect
	if err := client.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Second connection
	ctx2, cancel2 := context.WithTimeout(context.Background(), connectTimeout)
	if err := client.Connect(ctx2); err != nil {
		t.Fatalf("Second connect failed: %v", err)
	}
	cancel2()

	// Clean up
	_ = client.Close()
}

// TestIntegrationHeartbeat verifies heartbeat is sent
func TestIntegrationHeartbeat(t *testing.T) {
	requireAgent(t)

	config := &ClientConfig{
		AgentAddr:         getAgentAddr(),
		ServiceID:         testServiceID + "-heartbeat",
		Insecure:          true,
		TimeoutSeconds:    int(connectTimeout.Seconds()),
		HeartbeatInterval: 2, // Short interval for testing
	}
	client := NewClient(config)

	desc := FunctionDescriptor{
		ID:      testFunctionID,
		Version: testFunctionVersion,
	}
	handler := func(ctx context.Context, payload []byte) ([]byte, error) {
		return []byte("ok"), nil
	}

	_ = client.RegisterFunction(desc, handler)

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Wait for a few heartbeats
	time.Sleep(5 * time.Second)

	// Clean up
	if err := client.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

// TestIntegrationMultipleFunctions tests registering multiple functions
func TestIntegrationMultipleFunctions(t *testing.T) {
	requireAgent(t)

	config := &ClientConfig{
		AgentAddr:         getAgentAddr(),
		ServiceID:         testServiceID + "-multi",
		Insecure:          true,
		TimeoutSeconds:    int(connectTimeout.Seconds()),
		HeartbeatInterval: 30,
	}
	client := NewClient(config)

	// Register multiple functions
	functions := []struct {
		id      string
		version string
		handler func(context.Context, []byte) ([]byte, error)
	}{
		{
			id:      "test.ping",
			version: "1.0.0",
			handler: func(_ context.Context, payload []byte) ([]byte, error) {
				return []byte(fmt.Sprintf("pong: %s", string(payload))), nil
			},
		},
		{
			id:      "test.echo",
			version: "1.0.0",
			handler: func(_ context.Context, payload []byte) ([]byte, error) {
				return payload, nil
			},
		},
		{
			id:      "test.upper",
			version: "1.0.0",
			handler: func(_ context.Context, payload []byte) ([]byte, error) {
				return []byte(string(payload)), nil
			},
		},
	}

	for _, fn := range functions {
		desc := FunctionDescriptor{
			ID:      fn.id,
			Version: fn.version,
		}
		err := client.RegisterFunction(desc, fn.handler)
		if err != nil {
			t.Fatalf("Failed to register function %s: %v", fn.id, err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Clean up
	if err := client.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

// TestIntegrationIdempotentConnect tests that multiple connect calls are safe
func TestIntegrationIdempotentConnect(t *testing.T) {
	requireAgent(t)

	config := &ClientConfig{
		AgentAddr:         getAgentAddr(),
		ServiceID:         testServiceID + "-idempotent",
		Insecure:          true,
		TimeoutSeconds:    int(connectTimeout.Seconds()),
		HeartbeatInterval: 30,
	}
	client := NewClient(config)

	desc := FunctionDescriptor{
		ID:      testFunctionID,
		Version: testFunctionVersion,
	}
	handler := func(ctx context.Context, payload []byte) ([]byte, error) {
		return []byte("ok"), nil
	}

	_ = client.RegisterFunction(desc, handler)

	// First connect
	ctx1, cancel1 := context.WithTimeout(context.Background(), connectTimeout)
	if err := client.Connect(ctx1); err != nil {
		t.Fatalf("First connect failed: %v", err)
	}
	cancel1()

	// Second connect should be safe (idempotent)
	ctx2, cancel2 := context.WithTimeout(context.Background(), connectTimeout)
	if err := client.Connect(ctx2); err != nil {
		t.Errorf("Second connect should not fail: %v", err)
	}
	cancel2()

	// Clean up
	if err := client.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}
