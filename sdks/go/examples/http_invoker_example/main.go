// Simple example demonstrating HTTP invoker usage
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

func main() {
	fmt.Println("=== Croupier Go SDK - HTTP Invoker Example ===")

	// Create HTTP invoker configuration
	invokerConfig := &croupier.InvokerConfig{
		Address:        "localhost:18780", // HTTP REST API port
		TimeoutSeconds: 30,
		Insecure:       true,
	}

	// Create HTTP invoker (instead of gRPC invoker)
	invoker := croupier.NewHTTPInvoker(invokerConfig)
	defer invoker.Close()

	// Connect to server
	ctx := context.Background()
	if err := invoker.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	fmt.Println("✅ Connected to server via HTTP REST API")

	// Set validation schema (optional)
	banSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"player_id": map[string]interface{}{"type": "string"},
			"reason":    map[string]interface{}{"type": "string"},
			"duration":  map[string]interface{}{"type": "integer"},
		},
		"required": []string{"player_id", "reason"},
	}

	if err := invoker.SetSchema("player.ban", banSchema); err != nil {
		log.Printf("Warning: Failed to set schema: %v", err)
	}

	// Prepare invocation payload
	payload := map[string]interface{}{
		"player_id": "player_12345",
		"reason":    "违规操作",
		"duration":  3600, // seconds
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("Failed to marshal payload: %v", err)
	}

	// Invoke options
	options := croupier.InvokeOptions{
		IdempotencyKey: fmt.Sprintf("key-%d", time.Now().UnixNano()),
		Timeout:        30 * time.Second,
		Headers: map[string]string{
			"X-Game-ID": "example-game",
			"X-Env":     "development",
		},
	}

	// Invoke function
	fmt.Println("\n📞 Invoking player.ban function...")
	fmt.Printf("Payload: %s\n\n", string(payloadJSON))

	result, err := invoker.Invoke(ctx, "player.ban", string(payloadJSON), options)
	if err != nil {
		log.Fatalf("❌ Invoke failed: %v", err)
	}

	fmt.Printf("✅ Invoke succeeded!\n")
	fmt.Printf("Result: %s\n", result)

	fmt.Println("\n=== Example completed successfully ===")
}
