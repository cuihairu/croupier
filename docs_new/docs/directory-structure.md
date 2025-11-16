# Directory Structure (Ports/Adapters + Wire)

This document defines the target repo layout based on a clear layering with Ports/Adapters and Google Wire for dependency injection. It aims to keep process boundaries explicit, business logic centralized, and infrastructure replaceable.

## Top-Level

- `cmd/`           Executables (one folder per binary)
  - `server/`     Server main()
  - `agent/`      Agent main()
  - `edge/`       Edge main()
  - `analytics-worker/` (optional) background worker
- `internal/`      Application code (non-exported)
- `pkg/`           Shared libraries (if any; stable, exported)
- `configs/`       YAML configs, RBAC, users, games defaults
- `proto/`         Protobuf definitions
- `gen/`           Generated code (from `proto/`)
- `web/`           Front-end
- `scripts/`       Dev/CI scripts
- `tools/`         Examples, utilities, adapters demo (non-critical)
- `docs/`          Documentation

## Internal Layout (C: Ports/Adapters)

### `internal/app/` (Process Assemblers)

Role: Compose the process (HTTP routes, middleware, gRPC servers), wire dependencies, read config.

- `internal/app/server/http/`
  - `routes.go`     Gin routes/handlers (no business logic; call services)
  - `middleware.go` Common middlewares (authz, scope guard, logging, cors)
  - `wire.go`       Wire sets and injectors (build: wireinject)
  - `wire_gen.go`   Wire generated (committed)
- `internal/app/agent/`  Agent wiring
- `internal/app/edge/`   Edge wiring

Rules:
- Handlers only orchestrate: decode → authz → call service → encode
- No direct SQL/GORM usage in app; no business rules here

### `internal/service/` (Application Services)

Role: Application use-cases (transactions, authorization, audit), built on Ports interfaces.

- `internal/service/games/`
  - `service.go`     Game + Envs use-cases (add/rename env, apply defaults, validate)
  - Depends on `internal/ports` interfaces
- `internal/service/users/`, `assignments/`, `analytics/` …

Rules:
- No GORM/HTTP imports. Only use Ports.
- Transaction boundaries are here (via a `UnitOfWork` port), if needed.

### `internal/ports/` (Interfaces)

Role: Abstractions for external dependencies (repositories, object storage, cache, etc.).

- `games.go`  (`GamesRepository`, `UnitOfWork`, structs for DTO-like use)
- `users.go`, `assignments.go`, …

Rules:
- No infrastructure imports; only `context`/`time`/`errors` etc.

### `internal/repo/gorm/` (Adapters: SQL via GORM)

Role: Implement Ports with a concrete technology (GORM). One folder per domain.

- `internal/repo/gorm/games/`      GORM models + repository impl for Ports
- `internal/repo/gorm/users/`
- `internal/repo/gorm/messages/`
- `internal/repo/gorm/support/`
- `internal/repo/gorm/assignments/`

Rules:
- Only persistence logic here. No business rules.
- GORM models are here; not spread in service/app.

### `internal/platform/` (3rd-party Integrations)

Role: Non-business platform code: object storage, TLS, packaging, validation, etc.

- `objstore/`, `tlsutil/`, `pack/`, `validation/` …

### `internal/security/`

Role: Security building blocks.

- `rbac/`   Casbin policy loaders, helpers
- `token/`  JWT manager
- `approvals/` (if any)

## Dependency Arrows

```
app  →  service  →  ports  ←  repo/gorm
                    ↑
                 security/platform (as needed)
```

- app depends on service & wiring only
- service depends only on ports
- repo/gorm implements ports
- security/platform are used by service or repo as needed

## Naming & Conventions

- Business models live in service (or a nested `model.go`), infra models live in repo/gorm
- Ports package exposes small, stable interfaces
- Avoid circular deps; keep packages cohesive and small
- Transactions: expose `UnitOfWork` in ports; service coordinates it
- Wire: keep 1–2 sets per process (`RepoSet`, `ServiceSet`), and a root injector per app

## Wire Setup (Example)

```go
// internal/app/server/http/wire.go
//go:build wireinject
package httpapp

import (
  "github.com/google/wire"
  gamesvc "github.com/your/module/internal/service/games"
  gamesrepo "github.com/your/module/internal/repo/gorm/games"
)

var RepoSet = wire.NewSet(ProvideGormDB, gamesrepo.NewRepo /* ports impl */)
var ServiceSet = wire.NewSet(gamesvc.New /* *gamesvc.Service */)

func InitServerApp(cfg Config) (*Server, error) {
  wire.Build(RepoSet, ServiceSet, NewHTTPServer /* handlers */)
  return &Server{}, nil
}
```

## Testing Guidance

- service: mock ports (gomock or hand-rolled), test rules/flows
- repo: sqlite-in-memory AutoMigrate + CRUD integration tests
- app: thin HTTP tests for routes/middleware

## Migration Notes

- GORM models should be unified under repo/gorm (no duplicate models elsewhere)
- Frontend & handlers should not import repo/gorm directly
- Legacy GamesMeta page/service removed; UI should call `/api/games` and `/api/me/games` via `web/src/services/croupier/games.ts`.
