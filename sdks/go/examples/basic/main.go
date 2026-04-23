// Example demonstrates basic usage of the Croupier Go SDK
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

func main() {
	agentAddr := getenv("CROUPIER_AGENT_ADDR", "127.0.0.1:19091")
	gameID := getenv("CROUPIER_GAME_ID", "example-game")
	serviceID := getenv("CROUPIER_SERVICE_ID", "example-service")
	localListen := getenv("CROUPIER_LOCAL_LISTEN", "127.0.0.1:19101")

	// Create client configuration
	config := &croupier.ClientConfig{
		AgentAddr:      agentAddr,
		GameID:         gameID,
		Env:            "development",
		ServiceID:      serviceID,
		ServiceVersion: "1.0.0",
		LocalListen:    localListen,
		TimeoutSeconds: 30,
		Insecure:       true, // Use insecure gRPC for development
	}

	// Create client
	client := croupier.NewClient(config)

	// Register game functions
	if err := registerFunctions(client); err != nil {
		log.Fatalf("Failed to register functions: %v", err)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nReceived shutdown signal")
		cancel()
	}()

	// Start the client service
	fmt.Println("Starting Croupier SDK example...")
	if err := client.Serve(ctx); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}

	// Cleanup
	if err := client.Close(); err != nil {
		log.Printf("Error closing client: %v", err)
	}

	fmt.Println("Example completed")
}

func registerFunctions(client croupier.Client) error {
	// Register player ban function
	playerBanDesc := croupier.FunctionDescriptor{
		ID:        "player.ban",
		Version:   "1.0.0",
		Category:  "moderation",
		Risk:      "high",
		Entity:    "player",
		Operation: "update",
		Enabled:   true,
	}

	playerBanHandler := func(ctx context.Context, payload []byte) ([]byte, error) {
		fmt.Printf("🔨 Banning player with payload: %s\n", string(payload))

		// Simulate processing time
		time.Sleep(100 * time.Millisecond)

		result := fmt.Sprintf(`{
			"status": "success",
			"action": "ban",
			"timestamp": "%s",
			"message": "Player banned successfully"
		}`, time.Now().Format(time.RFC3339))

		return []byte(result), nil
	}

	if err := client.RegisterFunction(playerBanDesc, playerBanHandler); err != nil {
		return fmt.Errorf("failed to register player.ban: %w", err)
	}

	// Register item create function
	itemCreateDesc := croupier.FunctionDescriptor{
		ID:        "item.create",
		Version:   "1.0.0",
		Category:  "inventory",
		Risk:      "low",
		Entity:    "item",
		Operation: "create",
		Enabled:   true,
	}

	itemCreateHandler := func(ctx context.Context, payload []byte) ([]byte, error) {
		fmt.Printf("📦 Creating item with payload: %s\n", string(payload))

		result := fmt.Sprintf(`{
			"status": "success",
			"action": "create",
			"item_id": "item_%d",
			"timestamp": "%s"
		}`, time.Now().Unix(), time.Now().Format(time.RFC3339))

		return []byte(result), nil
	}

	if err := client.RegisterFunction(itemCreateDesc, itemCreateHandler); err != nil {
		return fmt.Errorf("failed to register item.create: %w", err)
	}

	fmt.Println("✅ All functions registered successfully")
	return nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
