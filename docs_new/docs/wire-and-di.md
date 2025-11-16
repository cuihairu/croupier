Wire DI & Providers
===================

This document summarizes the dependency injection setup using Google Wire and the available providers.

Entry Points
------------

- `InitServerApp(...)`: compose a Server from explicit dependencies (audit writer, RBAC policy, JWT manager, db, repos, services, etc.)
- `InitServerAppAuto(...)`: auto-construct audit/RBAC/JWT/DB/Repos/Services from environment variables

Providers
---------

- DB: `ProvideGormDBFromEnv()`
  - `DB_DRIVER`: `postgres|mysql|sqlite|mssql|sqlserver|auto` (default `auto`)
  - `DATABASE_URL`: connection string/DSN
  - Auto fallback to SQLite: `file:data/croupier.db`
- Games defaults: `ProvideGamesDefaults()` → reads `configs/games.json` (`default_envs`)
- RBAC policy:
  - `ProvideRBACPolicyAuto()`
    - If both `RBAC_MODEL` and `RBAC_POLICY` are set, use Casbin with these paths
    - Else use `RBAC_CONFIG` (JSON or Casbin directory); JSON falls back to legacy policy
- JWT manager: `ProvideJWTManagerFromEnv()`
  - `JWT_SECRET`: HS256 secret (default `dev-secret`)
- Certificate store: `ProvideCertStore(db)` (GORM-backed)
- Object store: `ProvideObjectStoreFromEnv()`
  - `STORAGE_DRIVER`: `s3|file|oss|cos`
  - `STORAGE_BUCKET`, `STORAGE_REGION`, `STORAGE_ENDPOINT`, `STORAGE_ACCESS_KEY`, `STORAGE_SECRET_KEY`, `STORAGE_FORCE_PATH_STYLE`
  - `file` mode defaults to `data/uploads` when base dir is empty
- ClickHouse: `ProvideClickHouseFromEnv()`
  - `CLICKHOUSE_DSN`: e.g., `clickhouse://host:port/...` (optional)

Local Development
-----------------

- Install wire: `go install github.com/google/wire/cmd/wire@latest`
- Generate code: `make wire` (runs `wire` in `internal/app/server/http`)
- Commit generated file: `internal/app/server/http/wire_gen.go` (CI validates no diff)

Notes
-----

- Handlers should be thin: decode → authorize → call service → encode
- Services should depend only on ports, not GORM
- Repositories (GORM) own DB models and perform persistence only

