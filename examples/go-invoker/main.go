package main

import (
    "context"
    "encoding/json"
    "log"
    "time"

    sdk "github.com/cuihairu/croupier-sdk-go/sdk"
)

// This example demonstrates using the Go SDK Invoker to call a function
// via FunctionService (Server or Agent address). It assumes a local Agent
// is running on 127.0.0.1:19090 and has a handler for "player.ban".
func main() {
    ctx := context.Background()

    inv, err := sdk.NewInvoker(ctx, sdk.InvokerConfig{
        Address:  "127.0.0.1:19090", // Agent's local FunctionService
        Insecure: true,               // dev only; use mTLS in prod
        Timeout:  3 * time.Second,
        GameID:   "default",
        Env:      "dev",
    })
    if err != nil { log.Fatalf("dial invoker: %v", err) }
    defer inv.Close()

    // Optional: client-side schema validation
    inv.SetSchema("player.ban", map[string]any{
        "type": "object",
        "required": []any{"player_id"},
        "properties": map[string]any{
            "player_id": map[string]any{"type":"string", "minLength": 1},
            "reason":    map[string]any{"type":"string"},
        },
    })

    // Successful call
    in := map[string]any{"player_id": "u-1001", "reason": "cheat"}
    payload, _ := json.Marshal(in)
    out, err := inv.Invoke(ctx, "player.ban", payload, sdk.WithIdempotency(sdk.NewIdempotencyKey()))
    if err != nil { log.Fatalf("invoke: %v", err) }
    log.Printf("invoke OK: %s", string(out))

    // Example validation failure (missing player_id)
    bad := map[string]any{"reason": "empty id"}
    b, _ := json.Marshal(bad)
    if _, err := inv.Invoke(ctx, "player.ban", b); err != nil {
        log.Printf("expected invalid payload: %v", err)
    }
}
