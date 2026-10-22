---
title: Repository Guidelines
---

# Repository Guidelines

## Project Structure & Module Organization

- Go monorepo: binaries in `cmd/`, core implementation in `internal/`, stable exported helpers in `pkg/`.
- Frontend UI: `web/`.
- Configs & assets: `configs/`, `descriptors/`, `scripts/`, `docs/`, runtime data in `data/`.
- Protocol/IDL: `proto/`, generated stubs in `pkg/pb`.
- SDKs: `sdks/<lang>` for code, `docs/sdks/<lang>` for formal docs.

## Build, Test, and Development Commands

- Build all server-side binaries: `make build`
- Build a specific binary: `make server`, `make agent`, `make worker`, `make ingest`
- Generate protobuf code: `make proto`
- Run tests: `make test`
- Build docs: `cd docs && pnpm install && pnpm run build`
- Build dashboard: `cd web && pnpm install && pnpm build`

## Coding Style & Naming Conventions

- Go: `gofmt`/`goimports`; packages lowercase; exported ids `CamelCase`; use `context.Context` first; structured logs.
- New entrypoints go under `cmd/`; do not reintroduce `services/*` layout or stale document references.
- TypeScript/React: Prettier + ESLint; 2-space indent; components `PascalCase`; hooks `useX`.
- Commits: Conventional Commits (`feat(scope): ...`, `fix`, `chore`, `docs`).

## Enumeration Design (Mandatory)

Platform state fields and user-contract enumerations use two entirely different mechanisms; mixing them is forbidden:

### 1) Platform state machines [Go int enums + DB int columns]

Platform-owned status columns (vocabularies defined by the platform, converged at compile time) always use the int-backed enums of `internal/dbenum`:

- Coverage: `function_contracts.capability`/`risk`, `page_proposals.status`, `tickets.status`, `messages.status`, `feedback.status`, `extension_installation.status`, `certificate.status`.
- Go layer: `type TicketStatus int` + `iota` constants + `Parse*/String()`; comparisons and branching are compile-time safe.
- DB layer: columns stored as small ints (fast comparisons, cheap indexes); migration scripts backfill legacy strings into ints (an empty capability is inferred from the function name, e.g. `player.list → collection_query`).
- JSON/wire layer: enums implement `json.Marshaler/Unmarshaler`; REST inputs/outputs remain readable strings (`"open"`, `"accepted"`) — the external contract is unchanged.
- Writes go uniformly through `Parse*` (invalid values return 422); raw strings landing in the DB are forbidden; `Scan` accepts historical string rows for a smooth migration read path.
- proto/SDK wire layers stay string (the protocol is unaffected by DB enum-ization).

### 2) User-contract enumerations [JSON Schema `enum` arrays passed through]

Enums defined by the game side in function spec / OpenAPI (even when the field is also named status) are **user data**, not platform state:

- The platform does exactly three things: pass through (the schema stored verbatim in `inputSchema`/`outputSchema`, lossless), render (a property with `enum` → the form generates a Select), validate (illegal values rejected at runtime per the schema).
- The vocabulary drifts with the user's spec version; platform code must not anticipate it, must not convert it into Go enums, and must not add DB CHECK constraints.
- Users changing an enum requires no platform release.

### 3) Decision Rule

For each new field, decide by owner: vocabulary owned by the platform (state machines/governance semantics) → dbenum int enum; vocabulary owned by the game side (business vocabulary) → stays in the schema JSON. Creating new Go enums for user vocabularies is forbidden, and platform states must not add further raw-string columns.

Exemption records (kept as string, but must carry vocabulary validation):

- `extension_installation.status`: the state set includes `uninstalled` and interacts with `desired_state`, with EqualFold comparisons; it involves extension lifecycle semantics restructuring, so string + vocabulary validation remains for now.
- `certificate.status`: derived from expiry time (active/expiring/expired); a monitoring snapshot rather than a state machine, kept as string.

## Testing Guidelines

- Go unit tests co-locate as `*_test.go`; prefer table-driven tests.
- Frontend: `cd web && pnpm test` or `pnpm test:coverage`.
- Add tests when touching RBAC, APIs, routing, analytics processing, or descriptor resolution.

## Selective Testing (Mandatory)

Local development picks the verification scope by change surface; "recompile and retest everything for a one-line doc change" is forbidden. CI gates stay full (CI is the gatekeeper; local development is incremental). Tool: `make test-changed` (i.e. `scripts/test-changed.sh`), defaulting to comparing `@{u}`..working tree (including uncommitted changes); `make test-changed ARG=--committed` compares base..HEAD.

Classification rules:

| Change surface                                                                                      | Verification action                                                                                                                                                                    |
| --------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `docs/**`, `*.md`, comment/whitespace-only `.go`                                                    | No compile: docs checks only (docs site build link check + prettier over changed files only)                                                                                           |
| `.go` under `internal/**`, `cmd/**`, `pkg/**`, `core/**`, `database/**`, `agents/**`, `examples/**` | `go build` + `go test` only for changed packages and their transitive reverse dependents (reverse-looked-up via `go list`, including test-import edges)                                |
| Shared layers `internal/{model,config,common,svc,db}`, `pkg/pb`                                     | Escalate to full                                                                                                                                                                       |
| `go.mod`, `go.sum`, `Makefile`, `CMakeLists.txt`, `.github/**`, `proto/**`, `buf.*`                 | Full gate                                                                                                                                                                              |
| `web/**`                                                                                            | tsc + targeted jest (changed test files + test files importing changed source files); `package.json`, jest/umi config, `src/services/api/`, `src/types/` changes escalate to full jest |
| `docker/**`                                                                                         | `docker compose config` validation / Dockerfile build (when the docker CLI exists)                                                                                                     |
| `configs/**`                                                                                        | `go test ./internal/config/...`                                                                                                                                                        |
| `scripts/**`, `tools/**`                                                                            | `bash -n` (shellcheck when available)                                                                                                                                                  |
| Everything else (`mobile/`, `packs/`, `descriptors/` etc.)                                          | Print a hint, never silently drop; verify manually                                                                                                                                     |

Full gate = `gofmt` + `go build ./...` + `go test -short ./...` + web tsc + web jest + docs build + `dashboard_vnext_guard` + `pnpm format:check`.

> Go tests run **without `-race`**: consistent with `make test` (`go test -v -short ./...`) and CI (`go test ./internal/... ./agents/...`); none of this repo's existing gates run with `-race`. Known limits (measured 2026-10-14, all pre-existing and unrelated to CI): under full `-race`, the `pkg/protocol` overflow case triggers a `checkptr` fatal, `internal/server` and others mix atomic writes with bare reads in test code, and `internal/hotreload` plugin cases do not pass `-race` through, causing `.so` ABI mismatch; until these are fixed, a full `-race` run cannot go green — a separate quality-engineering item.

When to run full: `go.mod`/CI/build-system changes, shared-layer changes, pre-release wrap-up gates. When incremental: day-to-day features, docs, single-package fix commits.

## Commit & Pull Request Guidelines

- PR should include: what changed, why, and how it was verified.
- When adding APIs or permissions, update the matching files under `configs/`.
- Keep diffs focused; ensure `make test` passes before merge.

## Security & Configuration Tips

- Secrets go through environment variables, not hardcoded YAML.
- Example local run: `./bin/croupier-server --config configs/server.yaml`.

## Release Tagging

- Server / Agent release tags use `v*`, for example `v0.2.0`.
- SDK release tags use namespaced prefixes:
  - `sdk-js-v*`
  - `sdk-python-v*`
  - `sdk-go-v*`
  - `sdk-java-v*`
  - `sdk-cpp-v*`
- Do not use plain `v*` tags for SDK-only releases, otherwise you will target the server/agent release lane.

## Compatibility and Legacy Governance (no-compat-legacy principle)

This phase does not treat backward compatibility as a constraint. New protocols, new SDKs, and new docs **do not keep old entry points by default**.

### Hard Rules

1. **New code uses canonical naming**: protocol fields, SDK APIs, and doc terminology uniformly use the current baseline naming (e.g. `Task` not `Job`, `ProviderConnect` not `RegisterLocal`, TCP session not the `rpc_addr` callback).
2. **Delete by default, don't keep by default**: when renaming or migrating, delete the old entry directly — no `@Deprecated` aliases. Old generated code, old docs, and old examples are cleaned up or archived together.
3. **Temporary retention must be gated**: if a compatibility field truly must linger (e.g. DB column removal needs a migration, proto field removal needs SDK regeneration), all of the following must hold:
   - Annotate the **removal condition** at the code site (what event triggers removal) and the **owner/deadline**.
   - Register it in `todo.md` as a standalone item with the affected paths.
   - Provide a detection script (e.g. `scripts/check-sdk-matrix.sh`) that stays visible until complete removal.
4. **Historical archives go only into `docs/archive/`**: migration notes and old design docs move into `docs/archive/`, never left in main-path docs as "current pipeline" descriptions.

### Check Tooling

- `scripts/check-sdk-matrix.sh`: SDK missing capabilities, old wire names, and old README terminology are all **failures** (exit 1), blocking CI. The allowlist covers only protocol-alias modules, and those modules themselves must also use canonical naming.
- `rg "StartJob|RegisterLocal|HeartbeatLocal|rpc_addr|LocalControl"` may only appear in `docs/archive/`, in gated notes inside `todo.md`, or in generated-code regeneration scripts.

### Violation Examples

- [Adding `startJob` as an alias of `startTask` "to ease migration".]
- [Describing `rpc_addr` / the gRPC callback as the current pipeline in the README.]
- [Deleting an old field without writing a removal condition or registering a todo.]
- [Renaming `startJob` to `startTask` directly, updating all call sites and tests.]

## Transport Decision (No gRPC)

- Internal RPC (Server ↔ Agent ↔ SDK) **does not use gRPC**; it uses the self-developed TCP transport + protobuf. This conclusion came out of refactoring away from gRPC's pitfalls and is irreversible.
- Hard constraint: **no new direct gRPC usage**; the residual `google.golang.org/grpc` in `go.mod` is an indirect dependency only, to be periodically evaluated for removal.
- New RPC needs go through the self-developed TCP+proto; capabilities like a unified error protocol are built in the self-developed RPC layer (proto `Status`/`RpcError`), not by introducing gRPC.
