---
title: Database migration strategy — versioned migrations replacing the AutoMigrate evolution
---

# Database migration strategy: versioned migrations replacing the AutoMigrate evolution

## Status

Proposed (direction decided, landing in phases). **Do not let production databases run AutoMigrate at startup ever again.**

## Decision

1. Schema lifecycle is separated from ORM runtime responsibility: **GORM does CRUD only, never schema evolution**.
2. **Versioned migrations** are the single source of truth for schema changes; migration files land in the repository under review.
3. Because croupier has **game databases created dynamically at runtime**, we use a hybrid model:
   - A brand-new game database created for the first time → the runtime bootstraps the database and applies the baseline version (once only);
   - An existing database → catches up on pending migrations by version number;
   - A version upgrade → an operator-triggered batch rolling tool covers N existing game databases.
4. Startup only performs a **compatibility check** (database version ≥ the minimum version the program requires); on failure it refuses to start outright — it never silently ALTERs.

## Background and current state (evidence)

The current implementation is the "GORM AutoMigrate + hand-written compensation hooks + scattered SQL" shape, and the compensation cost has already materialized:

- `internal/model/migration.go`: before AutoMigrate, `migrateEnumColumns` must run first (the comment's own words: _AutoMigrate would alter their types which would destroy values_ — i.e. skipping it destroys data), plus `renameLegacyTables` and `dropLegacyPageUniqueIndexes`; postgres also needs a constraint self-healing retry loop; afterwards there is `CleanupAllLegacy`. All of these are **hand-written compensation layers that exist because there is no versioned migration**, and they will persist forever and keep growing.
- `internal/db/router/router.go`: lazily creates physical game databases (`game_<id>_<env>`) at runtime by `(game_id, env)`; `MigrateGame` runs on first open. singleflight only deduplicates within one process — **multiple processes starting at the same time still issue concurrent ALTERs against the same table**.
- `migrations/001_openapi_schema.sql`, `002_function_openapi_backfill.sql`: stray SQL files have already started to appear, with no version table and no unified executor.

## Options compared

| Option             | Assessment                                                                                                                                                              | Conclusion                                |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ----------------------------------------- |
| GORM AutoMigrate   | Convenient for bootstrapping new databases; but running it repeatedly against existing databases is dangerous: no audit, no rollback, unreviewable DDL, MDL locks on large tables, multi-instance races, schema/code drift | Reserved only for the **brand-new database baseline** |
| Atlas (versioned)  | Diff-based migration generation, lint, test, and a complete CI ecosystem; supports MySQL/MariaDB/Postgres/SQLite                                                         | **Preferred toolchain** (usable for generating baselines and linting) |
| Goose              | Mature, dual SQL + Go mode, well suited to complex data backfills                                                                                                       | An acceptable alternative executor        |
| golang-migrate     | The classic pure versioned executor                                                                                                                                      | Same as above                             |

The tool is not the point — **the governance model is**: migrations must be "versioned, committed to git, reviewable, and idempotently catch-up". Under that premise, choosing Atlas or Goose as the executor is equally fine.

## What makes croupier special: dynamic game databases

The generic tutorial assumes the set of schema targets is known and finite, so CI can run against the target databases directly. Croupier's game databases are **created only on first access**; future game databases do not exist today, and CI cannot run migrations against them. Hence the hybrid model:

```text
Brand-new database (first EnsureDatabase)
    └─ Runtime bootstrap: CREATE DATABASE + apply the baseline version once (CREATE semantics only)
        └─ Write the schema_migrations version record

Existing database (server startup / first access)
    └─ Catch up unapplied versions per schema_migrations
    └─ Cross-process lock: MySQL GET_LOCK / Postgres advisory lock (single-file sqlite has no such problem)

New version release
    └─ Operator triggers the fan-out tool, batch-rolling every existing game database per the game_envs registry
```

Rules:

- AutoMigrate is **only allowed** inside the "detected a brand-new empty database" branch as the baseline generation mechanism; any database with existing data always goes through versioned migrations.
- Migration files are named `NNN_<description>.sql` (paired where necessary with goose Go migrations for data backfill / dialect probing, e.g. 0002–0004), embedded into the binary so the fan-out tool works offline.
- Every migration must be idempotent, or commit atomically in-transaction together with its version write; MySQL DDL is not transactional, so probe-style idempotency (HasTable/HasColumn probe, then execute) is the required shape.
- After modifying `internal/db/migrate/migrations/*.sql`, run `make migrate-hash` to update `atlas.sum`; CI validates with `atlas migrate validate`.

## Data change discipline (Expand [Contract]

When changing a field or its meaning, a one-step DROP is forbidden:

```text
Expand      Extend: ADD COLUMN a new column (with a default value)
Migrate     Backfill: UPDATE to migrate old values (in batches for large tables, off-peak)
Switch      Switch over: the code reads and writes both during the transition
Contract    Contract: DROP the old column only after confirming all data has migrated
```

`migrateEnumColumns` is the existing hand-rolled example of this pattern; it should later be rewritten as a standard versioned migration entry instead of a startup hook.

## Startup validation

At startup the Server/Agent reads the current version from `schema_migrations`:

- Database version < the program's minimum required version → print a clear error (`database schema version 181, required >= 182`) and refuse to start;
- No component may implicitly trigger DDL on the request path.

## The boundary against "data regeneration"

**Schema migration and data regeneration are two different problems; do not mix the tools.**

- Column type / structure changes → the versioned migrations in this document.
- Business/spec content regeneration (e.g. after a page-generator upgrade, stale Title Case fallback labels remain in old PageProposals/PublishedPageSpecs) → the application layer triggers regeneration or cleanup keyed on a `generatorVersion` bump; no DDL is written.
- Published page snapshots are never changed silently; regeneration must be explicit (a generator version bump or an operator command), consistent with the existing "contract change → stale → republish" chain.

## Phased landing

1. [Introduce the `schema_migrations` version table and a unified executor (goose, `internal/db/migrate`, version table `goose_db_version`); AutoMigrate is narrowed to the brand-new empty database branch (a baseline bridge, one-shot only).]
2. [Startup compatibility validation (`MinimumRequiredVersion`; after catch-up, below the minimum refuses to start); cross-process session locks (MySQL `GET_LOCK` / PG advisory lock) cover the entire "version-table probe + baseline + catch-up" span, wired into `EnsureDatabase` (PG database-creation race tolerance) and `MigrateGame`; single-database mode (`multiGame: false`) also runs through `EnsureUpToDate` and no longer bare-runs AutoMigrate on every startup.]
3. [Existing compensation hooks (enum backfill, legacy table/column/index cleanup, openapi backfill) rewritten as numbered migrations 0002–0004 (goose Go migrations, registered in `internal/svc/migrations.go`, sharing the same implementation as the baseline bridge); the hooks narrow from "run on every startup" to "baseline one-shot only". The stray root-level SQL files (001/002) are deleted (002 folded into 0002).]
4. [CI migration tests cover all four dialects (the `migrate-matrix` job in `ci.yml`: real MySQL 8.4 + Postgres 16 + SQL Server 2022 services, including concurrent first-open lock verification; SQLite is covered by unit tests) and Atlas validation (the community-edition `atlas migrate validate` checks `atlas.sum` and file ordering; note that `atlas migrate lint` has been Pro-paid since v0.38, so rule-based linting is unavailable for now).]

   **SQL Server JSON columns**: `internal/model.JSON` (jsontype.go) is the dialect-aware replacement for `datatypes.JSON` — semantics aligned method by method (the mysql CAST in Value/Scan/MarshalJSON/GormValue), with one difference: the `GormDBDataType` default branch returns `nvarchar(max)` (datatypes.JSON returns empty for sqlserver, so the gorm driver falls back to the literal `json`, a type SQL Server does not have). `datatypes.JSONMap` natively includes the sqlserver branch and models keep using it. Session locks use `sp_getapplock` (note go-mssqldb does not rewrite `?` placeholders inside multi-statement batches; the resource name is inlined).

5. [Fan-out batch rolling tool: `croupier-server db fanout` (reads `game_envs`, catches up the meta database and each game database in turn, and prints a report table; `--dry-run` only reports versions without executing DDL; a database referenced by the registry but physically absent is marked `missing-database` — runtime lazy creation is not an error).]

## Review checklist

Check before merging:

```bash
rg -n 'AutoMigrate\(' internal cmd --glob '!**/*_test.go'
```

A new AutoMigrate call must live in the "brand-new empty database bootstrap" branch, otherwise it is a review failure; any DDL change must come with a numbered migration file — editing only the model struct tags is not acceptable.

## Known debt

Constructor-time AutoMigrate has been largely converged (the audit / metrics tables folded into the `internal/svc` baseline callback `migrateAuxModels`; constructors no longer execute DDL). The remaining AutoMigrate calls and their classification:

- `internal/model/migration*.go` — the baseline bridge itself; compliant.
- `internal/svc/service_context.go` (autoMigrate / autoMigrateMeta / migrateAuxModels) — baseline callbacks; compliant.
- `internal/platform/approvals/` — the `ApprovalModel` and workflow tables; the PG/SQLite stores use **their own independent DSN** (not the server meta database), so self-creating and self-managing is a reasonable shape.
- `internal/platform/registry/agent_session_db.go` — called explicitly by the svc baseline (the table structs live in the platform package to avoid a model→platform reverse dependency).
- `internal/platform/monitoring/certificates/` — the model is already in the `MetaModels` baseline; the `Store.AutoMigrate` method remains as a test-fixture entry point.
