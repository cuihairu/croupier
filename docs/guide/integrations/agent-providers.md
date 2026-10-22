---
title: Agent Providers (OpenAPI Provider)
icon: plug
order: 6
category:
  - Integration Guides
tag:
  - Agent
  - providers.yaml
  - OpenAPI
---

# Agent Providers (OpenAPI Provider)

> **Status**: Current —— `providers.yaml` is the first-class configuration for wiring external HTTP APIs into Croupier on the Agent side; this page is its complete reference.

The Agent's `providers.yaml` (in the Agent config directory, overridable via `CROUPIER_CONFIG_DIR`) declares third-party HTTP APIs as **openapi providers**: at Agent startup it fetches the OpenAPI document and registers every operation as a platform function; the console catalog/invocation/OpenAPI Source binding all use them like ordinary functions, with no Croupier-specific code required from the game side.

```text
providers.yaml ──► Agent openapi provider ──► fetch openapiSpec
                       │
                       ▼
        register functions <provider>.<operationId> (service: provider:<provider>)
                       │
                       ▼
     invocation chain: console/page ─► Server ─► Agent ─► baseUrl (the real HTTP API)
```

## Configuration Structure

```yaml
providers:
  players: # provider name; function ID prefix (players.player.list)
    enabled: true
    type: openapi # the only supported type for now
    game_id: default # owning game (callable only when it matches the request scope)
    env: dev # owning environment
    metadata: # optional; user-declared instance metadata (multi-KV, reported with registration to the sdk-distribution page)
      serverId: players-svc-1
    config:
      baseUrl: "http://127.0.0.1:8091" # upstream API root address
      openapiSpec: "http://127.0.0.1:8091/openapi.json" # OpenAPI 3.0 document URL
      version: "1.2.0" # optional; this provider's contract version (semver), fallback see "Function registration rules"
      timeout: "5s" # optional; default 30s ("500ms"/"1m" both fine)
      headers: # optional; default headers attached to every request
        X-Request-Source: croupier
      auth: # optional; see the table below
        type: bearer
        token: ${PLAYERS_API_TOKEN}
```

### Field Reference

| Field                       | Required                            | Description                                                                                                                                                                                                                                                                              |
| --------------------------- | ----------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `providers.<name>.enabled`  | Yes                                 | When `false`, loading is skipped                                                                                                                                                                                                                                                         |
| `providers.<name>.type`     | Yes                                 | Only `openapi` is supported                                                                                                                                                                                                                                                              |
| `providers.<name>.game_id`  | Yes                                 | Owning game ID; must match the Agent's registered scope, otherwise `provider_scope_mismatch` is raised (no fallback inheritance of the Agent scope)                                                                                                                                      |
| `providers.<name>.env`      | Yes                                 | Owning environment (prod/stage/test/dev…); same rule when empty                                                                                                                                                                                                                          |
| `providers.<name>.metadata` | No                                  | User-declared instance metadata (multi-KV such as `serverId`); reported with registration and shown/filterable on the SDK version distribution page. Reserved keys (`gameId`/`env`/`sdkLanguage`/`sdkVersion`/`sdkName` etc.) are stripped by the Agent and raise a registration warning |
| `config.baseUrl`            | Yes                                 | Upstream API root address; paths are appended directly                                                                                                                                                                                                                                   |
| `config.openapiSpec`        | One of `openapiSpec`/`openapiSpecs` | OpenAPI document URL                                                                                                                                                                                                                                                                     |
| `config.openapiSpecs`       | One of `openapiSpec`/`openapiSpecs` | List of document URLs (merged registration)                                                                                                                                                                                                                                              |
| `config.version`            | No                                  | Contract version for this provider's functions (must be valid semver)                                                                                                                                                                                                                    |
| `config.timeout`            | No                                  | Upstream call timeout, Go duration string, default `30s`                                                                                                                                                                                                                                 |
| `config.headers`            | No                                  | Default headers attached to every request                                                                                                                                                                                                                                                |
| `config.auth`               | No                                  | Auth configuration, see below                                                                                                                                                                                                                                                            |

### Authentication (`config.auth`)

| `type`           | Fields                                                                | Behavior                                    |
| ---------------- | --------------------------------------------------------------------- | ------------------------------------------- |
| `none` (default) | —                                                                     | No auth attached                            |
| `bearer`         | `token`                                                               | `Authorization: Bearer ***`                 |
| `basic`          | `username` / `password`                                               | HTTP Basic                                  |
| `api_key`        | `api_key.name` / `api_key.value` / `api_key.in` (`header` or `query`) | Attaches the API key at the given location  |
| `custom`         | `custom_headers`                                                      | Attaches an arbitrary set of custom headers |

Config values support `${ENV_VAR}` environment variable expansion (like `token` in the example above), so secrets need not be hard-coded in the file.

## Function Registration Rules

- **Function ID**: `<provider name>.<operationId>`, e.g. provider `players` + `operationId: player.list` → `players.player.list`. The `operationId` must be stable — renaming it means a different function.
- **Contract source**: summary / description / tags / x-* governance fields come from the OpenAPI document; **inputSchema is inferred automatically** — parameters (path/query/header/cookie, all read from the caller's payload at invocation time) each become a top-level property; when an `application/json` requestBody is an object, its properties/required are merged into the top level; other shapes go under the `body` property. Known limits: only local `$ref`s are resolved (`#/components/...`; circular references are truncated to empty objects at depth 16, dangling references degrade to an empty schema, non-local references are kept verbatim); outputSchema is currently not inferred (registered empty; page detail columns depend on what the function actually returns).
- **capability inference** (method + path shape, high confidence):

  | Shape                                      | capability                |
  | ------------------------------------------ | ------------------------- |
  | `GET /{resource}`                          | `collection_query`        |
  | `GET /{resource}/{id}`                     | `item_query`              |
  | `POST /{resource}`                         | `create`                  |
  | `PUT/PATCH /{resource}/{id}`               | `update`                  |
  | `DELETE /{resource}/{id}`                  | `delete`                  |
  | Others (e.g. `POST /{resource}/{id}/kick`) | `action` (low confidence) |

  `x-capability` / `x-resource` / `x-operation` override explicitly; `x-execution: task` declares async task execution; `x-risk` / `x-permission` declare governance fields; `x-approval: required` declares an approval requirement.

- **Contract version**: resolution priority is operation-level `x-version` > `config.version` > document `info.version` > default `1.0.0`. Every level must be valid semver; invalid values raise a warning and fall back to the next level rather than dropping the function — the server-side registration gate (`invalid_version`) drops non-semver functions outright, while the OpenAPI spec allows `info.version` to be any string (e.g. `2026-09`), so the provider side normalizes before registration.

- **Pagination**: when a collection endpoint's request carries `page`/`page_size` parameters and the response is the `{items, total}` shape, the generated resource page gets pagination bindings automatically.
- **Registration timing**: the Agent loads `providers.yaml` at startup; providers distributed via extension installs go through the same registration path (`SyncExtensionProviders`), and on a name conflict with the static file the static one wins by default (`CROUPIER_EXTENSION_PROVIDER_OVERRIDE_STATIC=1` reverses this). With `CROUPIER_EXTENSION_PROVIDERS_ONLY=1`, static files are skipped.

## Relationship to OpenAPI Source

An Agent provider registers **runtime functions**; the Dashboard's "OpenAPI Source" imports **contract candidates**. The two are linked by function ID: once a Source binds `kind: provider` and `functionId: players.player.list`, the contract materializes into a FunctionContract (source=`openapi`) and page Proposals can be generated. See OpenAPI Function Registration for details.

The **runtime imports** section of the Dashboard "OpenAPI Sources" page reads the provider sessions under the current scope, with data from:

```http
GET /api/v1/openapi/runtime-sources   # auth + X-Game-ID/X-Env scope
```

Besides the source Agent, function list, registered version, and latest heartbeat, each record carries registration-chain observation fields (#27):

- `metadata`: user metadata self-reported by the provider instance (`serverId=s1` etc. key=value pairs, display/search only);
- `serviceAddr`: the listen address of the callee (provider process);
- `firstSeenUnix`: import time — when this process first observed the provider (server-side normalization: falls back to `lastSeenUnix` when no observation exists);
- `latestVersion`: the highest registration version observed in this process, monotonic upward (server-side normalization: falls back to `version` when no parseable history exists; a value different from `version` means the provider once ran at a higher version — the page highlights this for investigation).

Boundary: `firstSeenUnix`/`latestVersion` are in-memory state, lost and re-accumulated when the session expires, disconnects, or the server restarts; only "within this process's window" semantics are guaranteed (consistent with the existing `lastSeenUnix` precedent).

The binding dialog's function candidates include these runtime functions too (tagged with the importing Agent).

## Local Verification

The repo ships a runnable end-to-end example:

```bash
go run ./examples/openapi-provider -server 127.0.0.1:19090 -http 127.0.0.1:8091
```

After startup, the console function catalog (default/dev) should show `players.player.list` and 5 more functions. For the example's description see `examples/openapi-provider/README.md` in the repo.

The self-hosted deployment (docker-compose.deploy.yml) keeps the containerized version of this example running by default under the `sdk-examples` profile (image `croupier-openapi-provider-demo`, upstream via `haproxy:19090`, scope injected via `CROUPIER_SDK_EXAMPLE_GAME_ID/ENV`) — the Dashboard "OpenAPI Sources" page's runtime imports section has data out of the box, with no manual startup needed.

## Boundaries

- Document fetching supports external URLs (provider scenario); Dashboard OpenAPI Source uploads must be inline (≤2MiB, external `$ref` rejected) — the two paths have different boundaries; keep them apart.
- The auto-inferred inputSchema is a caller-perspective synthesized schema (parameters + requestBody merged) and does not necessarily map one-to-one to the API's actual body structure: operations without a requestBody get an empty object schema; at invocation time the request is still assembled by the existing ParameterMapping/RequestBodyMapping, and the schema only drives forms and contract display.
- Registration-chain semantic-equality skipping still applies: after an Agent upgrade, a function's inputSchema changing from empty to populated counts as a content change and lands as a new contract version normally; published Dashboard pages must be regenerated/republished to gain real forms.
