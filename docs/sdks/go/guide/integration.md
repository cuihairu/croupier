# Croupier Go SDK Integration Guide

## Installation

```bash
go get github.com/cuihairu/croupier/sdks/go
```

## Minimal Example

```go
package main

import (
    "context"
    "github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

func main() {
    client := croupier.NewClient(&croupier.ClientConfig{
        AgentAddr: "localhost:19091",
        GameID:    "demo-game",
        Env:       "development",
        Insecure:  true,
    })

    _ = client.RegisterFunction(croupier.FunctionDescriptor{
        ID: "hello.world", Version: "0.1.0", Enabled: true,
    }, func(ctx context.Context, payload []byte) ([]byte, error) {
        // Contract signature: FunctionHandler = func(ctx, payload []byte) ([]byte, error)
        return []byte(`{"message":"Hello from Go!"}`), nil
    })
}
```

## Security mode

`Insecure: true` keeps plaintext for local development. For a production
connection, set `InsecureMode: croupier.InsecureModeSecure` together with
`CAFile` (and optionally `CertFile`, `KeyFile`, `ServerName`). The mode
selector is authoritative: an explicit secure mode wins even if a legacy
`Insecure: true` remains in the same config. Omitting the mode but providing
TLS material also resolves to secure; with no security fields at all the
development default (`Insecure: true`) is kept.

```go
client := croupier.NewClient(&croupier.ClientConfig{
    AgentAddr:    "agent.example:19091",
    GameID:       "demo-game",
    Env:          "production",
    InsecureMode: croupier.InsecureModeSecure,
    CAFile:       "/etc/croupier/ca.pem",
    ServerName:   "agent.example",
})
```

## Instance Metadata (Provider Self-Description)

At registration you can attach **user-defined multi-KV instance metadata** (e.g. `serverId`, `pod`). It is reported via `ProviderConnectRequest.metadata` and is used by the console's "SDK Version Distribution" page to show instance information and support metadata-based search (REST `GET /api/v1/providers/sdk-stats?metaKey=&metaValue=`):

```go
client := croupier.NewClient(&croupier.ClientConfig{
    // ...
    InstanceMetadata: map[string]string{
        "serverId": "s1",
        "pod":      "game-7c4d",
    },
})
```

- Reserved keys `sdkLanguage` / `sdkVersion` / `sdkName` / `protocol_version` / `gameId` / `env` are generated from platform fixed fields; when user metadata collides with a reserved key, the agent drops that key and reports a warning in the registration response `warnings`.
- Metadata is for observability only (display/search/diagnostics); it does not participate in routing or load balancing.
- The demo (`sdks/go/examples/demo`) reads the `CROUPIER_INSTANCE_METADATA` environment variable, accepting either a JSON object or the `k=v,k=v` shorthand; an empty value means no metadata is attached:
  ```bash
  CROUPIER_INSTANCE_METADATA='serverId=s1,pod=game-7c4d'
  # Equivalent: CROUPIER_INSTANCE_METADATA='{"serverId":"s1","pod":"game-7c4d"}'
  ```
- For wire semantics and storage design decisions, see the "Instance Metadata" section of `docs/architecture/sdk-wire-protocol.md`.
