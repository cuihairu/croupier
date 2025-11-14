# SDK Registration Flow Consistency

This document ensures consistent registration flow across all Croupier SDK languages (C++, Go, Java).

## Overview

All SDKs implement a standardized two-layer registration system aligned with the official Croupier proto definitions.

## Registration Architecture

```
Game Server → SDK → Agent → Croupier Server
              |       |        |
              |       |        └── ControlService (control.proto)
              |       └── LocalControlService (local.proto)
              └── Multi-language SDK
```

## Data Structure Alignment

### FunctionDescriptor (control.proto)

**All SDKs implement identical structure:**

| Field     | Type   | Description                                    | Example        |
|-----------|--------|------------------------------------------------|----------------|
| id        | string | function identifier                            | "player.ban"   |
| version   | string | semantic version                               | "1.2.0"        |
| category  | string | grouping category                              | "moderation"   |
| risk      | string | risk level: "low"\|"medium"\|"high"           | "high"         |
| entity    | string | entity type: "player", "item", etc.          | "player"       |
| operation | string | CRUD operation: "create"\|"read"\|"update"\|"delete" | "update" |
| enabled   | bool   | whether function is enabled                    | true           |

### LocalFunctionDescriptor (local.proto)

**All SDKs implement identical structure:**

| Field   | Type   | Description         | Example      |
|---------|--------|---------------------|--------------|
| id      | string | function identifier | "player.ban" |
| version | string | function version    | "1.2.0"      |

## Registration Flow

### 1. Function Registration (SDK Level)

**C++:**
```cpp
FunctionDescriptor desc;
desc.id = "player.ban";
desc.version = "1.0.0";
desc.category = "moderation";
desc.risk = "high";
desc.entity = "player";
desc.operation = "update";
desc.enabled = true;

client.RegisterFunction(desc, handler);
```

**Go:**
```go
desc := croupier.FunctionDescriptor{
    ID:        "player.ban",
    Version:   "1.0.0",
    Category:  "moderation",
    Risk:      "high",
    Entity:    "player",
    Operation: "update",
    Enabled:   true,
}

client.RegisterFunction(desc, handler)
```

**Java:**
```java
FunctionDescriptor desc = CroupierSDK.functionDescriptor("player.ban", "1.0.0")
        .category("moderation")
        .risk("high")
        .entity("player")
        .operation("update")
        .enabled(true)
        .build();

client.registerFunction(desc, handler);
```

### 2. SDK→Agent Registration (LocalControlService)

**All SDKs convert FunctionDescriptor → LocalFunctionDescriptor:**

```
RegisterLocal RPC:
- service_id: string
- service_version: string
- rpc_addr: string (local server address)
- functions: LocalFunctionDescriptor[]

Response:
- session_id: string
```

### 3. Agent→Server Registration (ControlService)

**Agent converts LocalFunctionDescriptor → FunctionDescriptor and forwards:**

```
Register RPC:
- game_id: string (from agent config)
- env: string (from agent config)
- agent_id: string
- functions: FunctionDescriptor[]

Response:
- session_id: string
```

## Configuration Consistency

### Multi-Tenant Isolation

**All SDKs support:**
- `game_id`: Game identifier for tenant isolation
- `env`: Environment ("development", "staging", "production")

### Service Identification

**All SDKs support:**
- `service_id`: Unique service identifier
- `service_version`: Service version for compatibility
- `agent_id`: Agent identifier for load balancing

### Connection Settings

**All SDKs support:**
- `agent_addr`: Agent gRPC address
- `local_listen`: Local server bind address
- `timeout_seconds`: Connection timeout
- `insecure`: Use insecure gRPC (development)
- TLS settings: `ca_file`, `cert_file`, `key_file`

## Build System Consistency

### Local Development (Mock gRPC)

**All SDKs:**
- Use mock gRPC implementations for local development
- No proto file dependencies required
- Quick build and iteration

**Build Commands:**
```bash
# C++
mkdir build && cd build
cmake .. && make

# Go
go build ./...

# Java
mvn compile
```

### CI/Production (Real gRPC)

**All SDKs:**
- Download proto files from main repository
- Generate gRPC code using language-specific tools
- Build with real gRPC implementation

**Environment Variables:**
- `CROUPIER_CI_BUILD=ON`: Enable CI mode
- `CROUPIER_PROTO_BRANCH=main`: Proto file branch

**Build Commands:**
```bash
# C++
cmake -DCROUPIER_CI_BUILD=ON ..
make

# Go
export CROUPIER_CI_BUILD=ON
go run scripts/generate_proto.go
go build -tags croupier_real_grpc ./...

# Java
export CROUPIER_CI_BUILD=ON
mvn compile -Pci-build
```

## Error Handling Consistency

### Connection Errors

**All SDKs handle:**
- Connection failures with retry logic
- gRPC communication errors
- Network timeouts
- TLS certificate errors

### Registration Errors

**All SDKs validate:**
- Function descriptor completeness
- Duplicate function registration
- Agent connectivity before registration
- Registration response handling

### Runtime Errors

**All SDKs provide:**
- Function execution error handling
- Graceful shutdown on context cancellation
- Resource cleanup on client close
- Heartbeat failure handling

## Example Implementations

### Complete Registration Flow

**C++:**
```cpp
// Configuration
ClientConfig config;
config.game_id = "example-game";
config.env = "development";
config.service_id = "game-server-1";
config.agent_addr = "localhost:19090";

// Client creation
CroupierClient client(config);

// Function registration
FunctionDescriptor desc = /* ... */;
client.RegisterFunction(desc, handler);

// Service startup
client.Connect();
client.Serve(); // Blocks
```

**Go:**
```go
// Configuration
config := &croupier.ClientConfig{
    GameID:    "example-game",
    Env:       "development",
    ServiceID: "game-server-1",
    AgentAddr: "localhost:19090",
}

// Client creation
client := croupier.NewClient(config)

// Function registration
desc := croupier.FunctionDescriptor{/* ... */}
client.RegisterFunction(desc, handler)

// Service startup
ctx := context.Background()
client.Serve(ctx) // Blocks
```

**Java:**
```java
// Configuration
ClientConfig config = new ClientConfig("example-game", "game-server-1");
config.setEnv("development");
config.setAgentAddr("localhost:19090");

// Client creation
CroupierClient client = CroupierSDK.createClient(config);

// Function registration
FunctionDescriptor desc = /* ... */;
client.registerFunction(desc, handler);

// Service startup
client.serve(); // Blocks
```

## Validation Checklist

- [ ] All SDKs use identical data structures aligned with proto
- [ ] All SDKs implement two-layer registration (SDK→Agent→Server)
- [ ] All SDKs support multi-tenant isolation (game_id/env)
- [ ] All SDKs support dual build modes (local/CI)
- [ ] All SDKs have consistent configuration options
- [ ] All SDKs handle errors in the same manner
- [ ] All SDKs provide similar API patterns in their respective languages

## Proto Alignment Verification

**Control.proto alignment:**
```protobuf
message FunctionDescriptor {
  string id = 1;
  string version = 2;
  string category = 3;
  string risk = 4;
  string entity = 5;
  string operation = 6;
  bool enabled = 7;
}
```

**Local.proto alignment:**
```protobuf
message LocalFunctionDescriptor {
  string id = 1;
  string version = 2;
}
```

All SDK implementations match these exact field names and types.

## Summary

This standardization ensures:
1. **Consistent developer experience** across all languages
2. **Interoperability** between different SDK implementations
3. **Maintainability** through shared patterns and conventions
4. **Proto compliance** ensuring compatibility with the Croupier ecosystem