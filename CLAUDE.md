# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Essential Development Commands

**Build System (Makefile-driven):**
```bash
make dev          # Clean build from scratch: proto + build
make build        # Build all binaries (server, agent) to /bin
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
./bin/croupier-server --config configs/server.yaml validate      # Config validation
```

**Code Style:**
```bash
gofmt -w .                # Format all Go files
gofmt -l .                # List files that need formatting
```

**SDK Conformance:**
```bash
./scripts/check-sdk-matrix.sh   # Verify each SDK exposes L1 APIs from sdks/SDK_FEATURE_MATRIX.md
```

**IMPORTANT: Before committing any changes, ALWAYS run `gofmt -w .` to ensure all Go files are properly formatted.**

## Architecture Overview

Croupier implements a **three-tier distributed GM backend system**:

1. **Permission Control Layer** - RBAC/ABAC system independent of game logic
2. **Game Control Layer** - Function registration-driven game operations
3. **Observable Display Layer** - Descriptor-driven UI generation

### Core Components

**Server** (`internal/server/`)
- Central control plane with gRPC (8443) + HTTP REST (18780)
- Two main services: `ControlService` (agent registration) and `FunctionService` (invocation routing)
- Features: load balancing, RBAC, audit chain, approval workflows, multi-game scoping

**Agent** (`internal/agent/`)
- Distributed proxy in game networks, outbound mTLS to Server
- Local gRPC listener (19090) for game server function registration
- Bidirectional tunnel support for request/response multiplexing
- Job execution with async streaming, idempotency, cancellation

### Data Flow Pattern
```
Web UI → Server (HTTP) → Load Balancer → Agent → Game Server
```

## Key Development Patterns

**Protocol-First Development:**
- All APIs defined in `proto/` using Buf toolchain
- Custom protoc plugin (`protoc-gen-croupier`) generates pack artifacts
- Generated code in `gen/` (ignored in git)

**CRITICAL: Protobuf Code Generation**
- **ALWAYS** use `make proto` to generate protobuf code
- Buf uses **remote plugins** with fixed versions (e.g., buf.build/protocolbuffers/go:v1.36.11)
- **NEVER** use local `protoc` directly - it will generate incompatible code
- Local protoc version mismatches will cause compilation failures
- Buf config (`buf.gen.yaml`) specifies exact plugin versions to match CI environment

**Descriptor-Driven Architecture:**
- Functions defined via protobuf + JSON Schema descriptors
- UI auto-generates forms, validation, and permission checks from single source
- Function packs (`.tgz`) bundle descriptors, schemas, and UI plugins

**Configuration Management:**
- Multi-layer: YAML → includes → profiles → env vars → CLI flags
- Environment prefixes: `CROUPIER_SERVER_*`, `CROUPIER_AGENT_*`
- Config validation: `./croupier config test`

**Idempotency & Task Model:**
- All operations support `idempotency-key` to prevent duplicate side effects
- Async tasks with event streaming (progress/logs/done/error)
- Task cancellation via `CancelTask` RPC

**Build Tags for Features:**
- `pg` tag: PostgreSQL support for approvals
- `sqlite` tag: SQLite approvals store
- Enables flexible deployment options

## Project Structure Essentials

```
cmd/                      # Binary entry points (server, agent, unified CLI)
proto/                    # Protobuf definitions (Buf workspace)
internal/server/          # Server business logic (control, function, http, registry)
internal/agent/           # Agent logic (tunnel, local server, jobs)
internal/auth/            # RBAC, JWT, TOTP, user management
internal/function/        # Descriptor loading and validation
internal/jobs/            # Job state machine and execution
internal/loadbalancer/    # Load balancing strategies (RR, consistent hash, least conn)
sdks/                     # Multi-language SDKs (go, python, java, js, cpp, csharp)
                          # Feature matrix: sdks/SDK_FEATURE_MATRIX.md (single source of truth)
                          # Wire protocol: docs/architecture/sdk-wire-protocol.md
web/                      # Frontend submodule (Umi Max + Ant Design)
configs/                  # Configuration templates and examples
examples/                 # Demo game servers and invokers
```

## Important Implementation Details

**Security Architecture:**
- Enforced mTLS for all inter-service communication
- Field-level masking for sensitive data in audit logs
- Two-person rule enforcement for high-risk operations
- Audit chain with hash-based integrity

**Single-Company Scope Model:**
- Croupier is not a SaaS multi-tenant platform; it assumes one game company with multiple games
- All operations are scoped by `game_id`/`env` for game/environment isolation
- Registry indexed by `(game_id, function_id)` for function routing
- HTTP headers `X-Game-ID`/`X-Env` propagated through call chain

**Database-per-Game Architecture:**
- When `database.multiGame: true` is set, each `(game_id, env)` pair gets its own physical database (e.g. `game_demo_prod`)
- The configured `database.dataSource` is the **meta database** (`croupier_meta`) holding users, roles, games registry, `game_envs` bindings, audit, extensions, etc.
- `internal/db/router.Router` lazily opens and caches per-game `*gorm.DB` connections, auto-creating and migrating each game database on first use
- `internal/db/dbctx` carries the per-request DB override via `context.Context`; game-scoped models call `dbctx.Resolve(ctx, m.db)` to pick the right connection
- `svc.GameDBMiddleware` resolves the game DB from `X-Game-ID`/`X-Env` headers and injects it into the request context
- Game-scoped models (Player, Function, Ticket, Analytics, Task, ConfigVersion, etc.) live in per-game databases; meta models (Admin, Game, Role, Audit, Extension) stay in the meta DB
- The `game_envs` table (`model.GameEnvBinding`) stores `(game_id, env, database_name)` routing records
- When `multiGame: false` (default for dev/CI), all tables coexist in a single database with `game_id` columns for row-level isolation

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

## API Response Contract (Mandatory)

All HTTP APIs must follow one response contract. Do not mix envelope and non-envelope styles.

### 1) Success Response

- Use standard HTTP status code (`200/201/204`).
- Return business payload directly as JSON object/array.
- Do **not** wrap success payload in `{ "code": ..., "message": ..., "data": ... }`.

Examples:
- `200 OK` + `{ "id": 1, "name": "admin" }`
- `200 OK` + `{ "items": [...], "total": 10 }`
- `204 No Content` with empty body

### 2) Error Response

- Use standard HTTP error status (`400/401/403/404/409/422/500`).
- Body uses unified error object:
  - `error`: stable machine-readable code (snake_case)
  - `message`: human-readable message
  - `details` (optional): structured validation/business detail

Example:
- `401 Unauthorized` + `{ "error": "unauthorized", "message": "未授权" }`

### 3) Frontend Error Handling Contract

HTTP status and response body have different responsibilities. Do not duplicate them.

- HTTP status expresses the error class:
  - `400`: invalid request / validation error
  - `401`: unauthenticated / token invalid
  - `403`: authenticated but forbidden
  - `404`: resource not found
  - `409`: conflict
  - `422`: semantically invalid but well-formed request
  - `500`: internal server error
- Response body expresses the readable and structured error detail:
  - `error`: stable code for frontend branching
  - `message`: user-facing error text
  - `details`: optional structured payload for form fields or extra diagnostics

Recommended examples:

- `400 Bad Request`

```json
{
  "error": "validation_failed",
  "message": "请求参数无效",
  "details": {
    "gameId": "不能为空"
  }
}
```

- `401 Unauthorized`

```json
{
  "error": "unauthorized",
  "message": "未授权"
}
```

- `409 Conflict`

```json
{
  "error": "conflict",
  "message": "资源状态冲突"
}
```

Frontend rules:

- Route and global handling should primarily branch on HTTP status.
- UI text should come from `message`.
- Stable client logic should branch on `error`, not on localized `message`.
- Form-level rendering should read `details` when present.
- Do not require frontend to read a business `code` field when HTTP status already exists.

### 4) Explicit Exceptions

- SSE endpoints must return `text/event-stream` (not JSON envelope).
- Health/readiness probes may return minimal payload (for example `{ "status": "ok" }`).
- File/binary download endpoints follow content-type requirements instead of JSON.

### 5) Implementation Rules

- Prefer `internal/common/response` for API handlers.
- Do not introduce new handlers based on `internal/pkg2/response` envelope style.
- Middleware auth failures must keep the same JSON error shape as above.
- New APIs must keep response shape consistent with existing REST handlers in `internal/api`.

### 6) CI/Review Checklist

Before merge, scan for direct ad-hoc response writes:

```bash
rg -n "\bc\.(JSON|IndentedJSON|String|Data|PureJSON|XML|YAML|ProtoBuf)\(" internal --glob "!**/*_test.go"
```

Any new direct write must be justified as an exception (SSE/health/binary), otherwise refactor to unified response helpers.

## Configuration Contract (Mandatory)

Configuration naming must be standardized. Do not invent new casing per file or per module.

This repository previously drifted into mixed styles such as `Server` vs `storage`, `Driver` vs `driver`, `Dir` vs `dir`, `Enabled` vs `enabled`. That is invalid going forward.

### 1) Canonical Naming Rules

- YAML keys: `lowerCamelCase`
- JSON keys: `lowerCamelCase`
- Go struct fields: `PascalCase`
- Environment variables: `UPPER_SNAKE_CASE`
- CLI flags: `kebab-case`

Examples:

```yaml
server:
  host: 0.0.0.0
  port: 18780
  timeout: 600000
  mode: dev

database:
  driver: mysql
  dataSource: xxx

storage:
  driver: file
  baseDir: data/uploads

cache:
  enabled: true
  type: redis
```

### 2) Implementation Rules

- New config structs must use `yaml:"lowerCamelCase"` and `json:"lowerCamelCase"`.
- Do not add new config tags using `Server`, `Driver`, `Dir`, `Enabled`, `BaseDir` style keys.
- Do not mix uppercase and lowercase siblings in the same config object.
- Config examples under `configs/*.yaml` must use canonical keys only.
- If old configs already exist, backward compatibility must be handled in code, not by continuing to write new examples in legacy style.

### 3) Backward Compatibility Rules

- Legacy config keys may be tolerated only in compatibility parsing code.
- Compatibility exists only to avoid breaking old deployments.
- Compatibility code must not become the new documentation standard.
- All docs, examples, generated templates, and new tests must use canonical `lowerCamelCase`.

### 4) Review Checklist

Before merge, check for mixed or legacy config naming:

```bash
rg -n 'yaml:"[A-Z]' internal/config
rg -n '^\s+[A-Z][A-Za-z0-9]*:' configs/*.yaml configs/**/*.yaml
```

If a change introduces new uppercase YAML keys, it is a review failure unless the file is explicitly a legacy compatibility fixture.
