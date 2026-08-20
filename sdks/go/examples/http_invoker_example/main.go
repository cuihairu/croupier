// Simple example demonstrating HTTP invoker usage.
//
// runScenario keeps the example testable: main() only wires configuration,
// while the invocation flow lives in an exported function covered by
// package tests against an httptest server.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

func main() {
	fmt.Println("=== Croupier Go SDK - HTTP Invoker Example ===")

	// L3 调用方仅经由 Server HTTP API；Provider TCP 注册链路不用于此处。
	invokerConfig := &croupier.InvokerConfig{
		Address:        "http://127.0.0.1:18780/api/v1",
		TimeoutSeconds: 30,
		Insecure:       true,
	}

	if err := runScenario(invokerConfig); err != nil {
		log.Fatalf("❌ example failed: %v", err)
	}
	fmt.Println("\n=== Example completed successfully ===")
}

// runScenario executes the full L3 invoke flow against the configured server
// base URL: connect, optional schema validation, and one idempotent invoke.
func runScenario(cfg *croupier.InvokerConfig) error {
	// NewInvoker 是公共 L3 入口。
	invoker := croupier.NewInvoker(cfg)
	defer invoker.Close()

	ctx := context.Background()
	if err := invoker.Connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	fmt.Println("✅ Connected to server via HTTP REST API")

	// Set validation schema (optional)
	banSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"playerId": map[string]interface{}{"type": "string"},
			"reason":   map[string]interface{}{"type": "string"},
			"duration": map[string]interface{}{"type": "integer"},
		},
		"required": []interface{}{"playerId", "reason"},
	}
	if err := invoker.SetSchema("player.ban", banSchema); err != nil {
		log.Printf("Warning: Failed to set schema: %v", err)
	}

	payload := map[string]interface{}{
		"playerId": "player_12345",
		"reason":   "违规操作",
		"duration": 3600,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	options := croupier.InvokeOptions{
		IdempotencyKey: fmt.Sprintf("key-%d", time.Now().UnixNano()),
		Timeout:        30 * time.Second,
		Headers: map[string]string{
			"X-Game-ID": "example-game",
			"X-Env":     "development",
		},
	}

	fmt.Println("\n📞 Invoking player.ban function...")
	fmt.Printf("Payload: %s\n\n", string(payloadJSON))

	result, err := invoker.Invoke(ctx, "player.ban", string(payloadJSON), options)
	if err != nil {
		return fmt.Errorf("invoke: %w", err)
	}
	fmt.Printf("✅ Invoke succeeded!\nResult: %s\n", result)
	return nil
}

// errServerUnreachable marks the failure path used by tests.
var errServerUnreachable = errors.New("server unreachable")
