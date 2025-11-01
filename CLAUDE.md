# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Essential Development Commands

**Build System (Makefile-driven):**
```bash
make dev          # Clean build from scratch: proto + build
make build        # Build all binaries (server, agent, edge) to /bin
make proto        # Generate gRPC code using Buf (buf generate)
make pack         # Generate pack artifacts via protoc-gen-croupier
make test         # Run unit tests with race detection
make clean        # Remove build artifacts and generated code
```

**Local Development Setup:**
```bash
git clone --recursive https://github.com/cuihairu/croupier.git
go mod download && make submodules
./scripts/dev-certs.sh    # Generate self-signed TLS certs
buf lint && buf generate  # Generate proto code
make build               # Build binaries
```

**Testing:**
```bash
make test                 # All tests with race detection
go test ./internal/...    # Subset testing
./croupier config test --config configs/server.example.yaml  # Config validation
```

## Architecture Overview

Croupier implements a **three-tier distributed GM backend system**:

1. **Permission Control Layer** - RBAC/ABAC system independent of game logic
2. **Game Control Layer** - Function registration-driven game operations
3. **Observable Display Layer** - Descriptor-driven UI generation

### Core Components

**Server** (`internal/server/`)
- Central control plane with gRPC (8443) + HTTP REST (8080)
- Two main services: `ControlService` (agent registration) and `FunctionService` (invocation routing)
- Features: load balancing, RBAC, audit chain, approval workflows, multi-game scoping

**Agent** (`internal/agent/`)
- Distributed proxy in game networks, outbound mTLS to Server
- Local gRPC listener (19090) for game server function registration
- Bidirectional tunnel support for request/response multiplexing
- Job execution with async streaming, idempotency, cancellation

**Edge** (`internal/edge/`)
- DMZ proxy bridging Server (internal) and Agent (outbound)
- Tunnel switchboard for multiplexed connections

### Data Flow Pattern
```
Web UI → Server (HTTP) → Load Balancer → Agent → Game Server
                ↓
            Edge (optional tunnel)
```

## Key Development Patterns

**Protocol-First Development:**
- All APIs defined in `proto/` using Buf toolchain
- Custom protoc plugin (`protoc-gen-croupier`) generates pack artifacts
- Generated code in `gen/` (ignored in git)

**Descriptor-Driven Architecture:**
- Functions defined via protobuf + JSON Schema descriptors
- UI auto-generates forms, validation, and permission checks from single source
- Function packs (`.tgz`) bundle descriptors, schemas, and UI plugins

**Configuration Management:**
- Multi-layer: YAML → includes → profiles → env vars → CLI flags
- Environment prefixes: `CROUPIER_SERVER_*`, `CROUPIER_AGENT_*`
- Config validation: `./croupier config test`

**Idempotency & Job Model:**
- All operations support `idempotency-key` to prevent duplicate side effects
- Async jobs with event streaming (progress/logs/done/error)
- Job cancellation via `CancelJob` RPC

**Build Tags for Features:**
- `pg` tag: PostgreSQL support for approvals
- `sqlite` tag: SQLite approvals store
- Enables flexible deployment options

## Project Structure Essentials

```
cmd/                      # Binary entry points (server, agent, edge, unified CLI)
proto/                    # Protobuf definitions (Buf workspace)
internal/server/          # Server business logic (control, function, http, registry)
internal/agent/           # Agent logic (tunnel, local server, jobs)
internal/auth/            # RBAC, JWT, TOTP, user management
internal/function/        # Descriptor loading and validation
internal/jobs/            # Job state machine and execution
internal/loadbalancer/    # Load balancing strategies (RR, consistent hash, least conn)
internal/pack/            # Pack/plugin system with type registry
sdks/                     # Multi-language SDKs (submodules: go, cpp, java)
web/                      # Frontend submodule (Umi Max + Ant Design)
packs/                    # Example function packs (prom, http, player, grafana)
configs/                  # Configuration templates and examples
examples/                 # Demo game servers and invokers
```

## Important Implementation Details

**Function Packs System:**
- Functions bundled as tar.gz with manifest, descriptors, UI components
- Import/export via Server HTTP API with ETag versioning
- Example packs demonstrate Prometheus, HTTP, Grafana integrations

**Security Architecture:**
- Enforced mTLS for all inter-service communication
- Field-level masking for sensitive data in audit logs
- Two-person rule enforcement for high-risk operations
- Audit chain with hash-based integrity

**Multi-Game Scoping:**
- All operations scoped by `game_id`/`env` for tenant isolation
- Registry indexed by `(game_id, function_id)` for function routing
- HTTP headers `X-Game-ID`/`X-Env` propagated through call chain

**Load Balancing Abstraction:**
- Strategy interface with multiple implementations
- Health checking integrated with agent selection
- Supports routing modes: lb, broadcast, targeted, hash

## Testing Approach

Unit tests focus on:
- RBAC policy grant/deny logic (`internal/auth/rbac/`)
- Job executor state transitions and idempotency (`internal/agent/jobs/`)
- Sensitive field masking (`internal/server/http/`)
- Pack import/export workflows
- Registry agent session management

Integration examples in `examples/` demonstrate end-to-end flows from function registration through UI invocation.