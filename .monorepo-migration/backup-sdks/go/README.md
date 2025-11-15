# Croupier Go SDK

Go SDK for Croupier game function registration and execution system.

## Overview

The Croupier Go SDK enables game servers to register functions with the Croupier system and handle incoming function calls through gRPC communication. This SDK is aligned with the official Croupier proto definitions.

## Features

- **Proto-aligned data structures**: All types match the official Croupier proto definitions
- **Dual build system**: Mock implementation for local development, real gRPC for CI/production
- **Multi-tenant support**: Built-in support for game_id/env isolation
- **Function registration**: Register game functions with descriptors and handlers
- **gRPC communication**: Efficient bi-directional communication with agents
- **Error handling**: Comprehensive error handling and connection management

## Quick Start

### Installation

```bash
go get github.com/cuihairu/croupier-sdk-go
```

### Basic Usage

```go
package main

import (
    "context"
    "log"

    "github.com/cuihairu/croupier-sdk-go/pkg/croupier"
)

func main() {
    // Create client configuration
    config := &croupier.ClientConfig{
        AgentAddr:      "localhost:19090",
        GameID:         "my-game",
        Env:            "development",
        ServiceID:      "my-service",
        ServiceVersion: "1.0.0",
        Insecure:       true, // For development
    }

    // Create client
    client := croupier.NewClient(config)

    // Register a function
    desc := croupier.FunctionDescriptor{
        ID:        "player.ban",
        Version:   "1.0.0",
        Category:  "moderation",
        Risk:      "high",
        Entity:    "player",
        Operation: "update",
        Enabled:   true,
    }

    handler := func(ctx context.Context, payload string) (string, error) {
        // Handle the function call
        return `{"status":"success"}`, nil
    }

    if err := client.RegisterFunction(desc, handler); err != nil {
        log.Fatal(err)
    }

    // Start serving
    ctx := context.Background()
    if err := client.Serve(ctx); err != nil {
        log.Fatal(err)
    }
}
```

## Data Types

### FunctionDescriptor

Aligned with `control.proto`:

```go
type FunctionDescriptor struct {
    ID        string // function id, e.g. "player.ban"
    Version   string // semver, e.g. "1.2.0"
    Category  string // grouping category
    Risk      string // "low"|"medium"|"high"
    Entity    string // entity type, e.g. "player"
    Operation string // "create"|"read"|"update"|"delete"
    Enabled   bool   // whether enabled
}
```

### LocalFunctionDescriptor

Aligned with `agent/local/v1/local.proto`:

```go
type LocalFunctionDescriptor struct {
    ID      string // function id
    Version string // function version
}
```

## Configuration

### ClientConfig

```go
type ClientConfig struct {
    // Connection
    AgentAddr      string // Agent gRPC address
    LocalListen    string // Local server address
    TimeoutSeconds int    // Connection timeout
    Insecure       bool   // Use insecure gRPC

    // Multi-tenant isolation
    GameID         string // Game identifier
    Env            string // Environment (dev/staging/prod)
    ServiceID      string // Service identifier
    ServiceVersion string // Service version
    AgentID        string // Agent identifier

    // TLS (when not insecure)
    CAFile   string // CA certificate
    CertFile string // Client certificate
    KeyFile  string // Private key
}
```

## Build Modes

### Local Development (Mock gRPC)

For local development, the SDK uses mock implementations:

```bash
go build ./...
go run examples/basic/main.go
```

### CI/Production (Real gRPC)

For CI builds with real proto-generated code:

```bash
export CROUPIER_CI_BUILD=ON
go run scripts/generate_proto.go
go build -tags croupier_real_grpc ./...
```

The CI system automatically:
1. Downloads proto files from main repository
2. Generates gRPC Go code using protoc
3. Builds with real gRPC implementation
4. Runs tests and examples

## Architecture

```
Game Server → Go SDK → Agent → Croupier Server
```

The SDK implements a two-layer registration system:
1. **SDK → Agent**: Uses `LocalControlService` (from `local.proto`)
2. **Agent → Server**: Uses `ControlService` (from `control.proto`)

## Examples

See the `examples/` directory for complete usage examples:

- `examples/basic/`: Basic function registration and serving
- More examples coming soon...

## Development

### Prerequisites

- Go 1.20 or later
- Protocol Buffers compiler (protoc)
- Go protoc plugins:
  ```bash
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
  ```

### Building

```bash
# Local development (mock)
make build

# CI build (real gRPC)
make ci-build

# Run tests
make test

# Generate proto code manually
go run scripts/generate_proto.go
```

## Error Handling

The SDK provides comprehensive error handling:

- Connection failures with automatic retry
- Function registration validation
- gRPC communication errors
- Graceful shutdown on context cancellation

## Contributing

1. Ensure all types align with proto definitions
2. Add tests for new functionality
3. Update documentation for API changes
4. Test both local and CI build modes

## License

See LICENSE file for details.