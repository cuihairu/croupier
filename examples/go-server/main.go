package main

import (
    "context"
    "encoding/json"
    "log"
    "time"

    sdk "github.com/cuihairu/croupier-sdk-go/sdk"
    // SDK registers its own json codec; agent/core use json too
)

func main() {
    cli := sdk.NewClient(sdk.ClientConfig{Addr: "127.0.0.1:19090", LocalListen: "127.0.0.1:0"})

    // Register a function handler: player.ban（从 descriptors 加载）
    schema := map[string]any{
        "type": "object",
        "properties": map[string]any{"player_id": map[string]any{"type":"string"}, "reason": map[string]any{"type":"string"}},
        "required": []any{"player_id"},
    }
    // 也可使用 RegisterFromDescriptor("descriptors/player.ban.json", handler)
    _ = cli.RegisterFunction(sdk.Function{ID: "player.ban", Version: "1.2.0", Schema: schema}, func(ctx context.Context, payload []byte) ([]byte, error) {
        var in struct{ PlayerID string `json:"player_id"`; Reason string `json:"reason"` }
        _ = json.Unmarshal(payload, &in)
        out := struct{ Success bool `json:"success"`; Echo any `json:"echo"` }{Success: true, Echo: in}
        return json.Marshal(out)
    })
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    if err := cli.Connect(ctx); err != nil {
        log.Fatalf("connect agent: %v", err)
    }
    defer cli.Close()

    log.Printf("example client connected to agent; function registered.")
    select {}
}
