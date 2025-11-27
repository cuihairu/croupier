# Directory Structure (Go-Zero Multi-Process)

The repository now standardizes on [go-zero](https://go-zero.dev/) across all backend services. Each process (server, agent, edge, analytics-worker, …) manages its own generated code, GORM models, and migrations under `services/<name>/`. This document captures the conventions to keep those services consistent.

## Top-Level Layout

- `cmd/`            Legacy entrypoints and helper binaries.
- `services/`       Go-zero applications (`server`, `agent`, `edge`, `ingest`, …).
- `internal/`       Shared libraries (auth, db helpers, schedulers, etc.).
- `pkg/`            Exported helper packages (rare; only for stable APIs).
- `configs/`        Global YAML, RBAC, bootstrap data.
- `proto/` & `gen/` Protobuf definitions and generated stubs.
- `web/`            Frontend (Umi Max + Ant Design 5).
- `scripts/`, `tools/`, `docs/`, `packs/`, `data/` remain unchanged.

## Go-Zero Service Layout

Each service inside `services/<name>` follows the go-zero scaffold:

```
services/<name>/
  server.go                # main()
  etc/                     # config yaml files
  cmd/                     # optional process-specific commands (migrate, seed…)
  internal/
    config/                # goctl generated config structs
    handler/               # HTTP/gRPC handlers (wire request ↔ logic)
    logic/                 # business logic (generated skeletons + custom code)
    model/                 # GORM models + data access helpers
    svc/                   # ServiceContext (wires config, db, models, services)
    middleware/, runtime/, common/ … (service-scoped helpers)
```

Key layering inside a service:

- `handler → logic → svc → model`.
- Handlers perform decoding/encoding + auth guard only.
- Logic packages contain the use-cases and should rely on interfaces exposed through `svc.ServiceContext`.
- `svc` wires configs, DB clients, models, and auxiliary services.
- `internal/model` owns the GORM structs, migrations, and persistence helpers for **that process only**.

## Model & Migration Guidelines

- Every go-zero service keeps its database models under `services/<name>/internal/model`.
- Models use GORM and expose helper structs (e.g. `AdminModel`) instead of the old `internal/repo/gorm` adapters.
- `svc/service_context.go` must invoke `<model>.AutoMigrate(db)` so each process migrates only the tables it owns.
- Cross-process data sharing happens through APIs; do **not** import another service's `internal/model`.
- Legacy `internal/repo/gorm/*` packages are considered deprecated. Do not add new dependencies to them; migrate features into the corresponding service's `internal/model` as you touch them.

## Shared Internal Packages

The `internal/` directory (outside `services/`) now only hosts code that is safe to share across processes, such as:

- `internal/auth/*` – token helpers, permission checks (may depend on service models via interfaces).
- `internal/database/*` – helpers for opening and configuring GORM/SQL connections.
- `internal/platform/*` – integrations (object storage, TLS, packaging).
- `internal/security/*` – RBAC loaders, JWT tooling.

When a shared package needs to inspect service-specific tables, inject the required model interface from the service instead of importing the model package directly. This keeps the dependency direction from service → shared helper.

## Dependency Flow

```
handler  →  logic  →  svc.ServiceContext  →  internal/model
                    ↘ shared internal helpers (auth, db, platform)
```

- Handlers never import `internal/model`.
- Logic only touches persistence via the interfaces exposed on `svc.ServiceContext`.
- Shared internal helpers must remain infrastructure-only; no business logic there.

## Testing Guidance

- Logic: use go-zero generated mocks or hand-rolled stubs for the interfaces exposed via `svc`.
- Model: prefer sqlite-in-memory or dedicated test schemas; call `model.AutoMigrate` inside tests.
- Handler: thin HTTP tests verifying routing/middleware.
- Each service owns its own test data/migrations; do not rely on other services' fixtures.

## Migration Notes

- When moving legacy code from `internal/repo/gorm`, port the structs into the relevant `services/<name>/internal/model` package and wire them through that service's `svc`.
- Update docs and examples as soon as a service completes migration to avoid confusion between old Ports/Adapters notes and the go-zero layout.
