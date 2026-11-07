---
title: Terms and layering
---

# Terms and layering

This glossary is the abstract-layer source-index supplement to [Layers](/architecture/layers). It was reviewed at commit `eec3592912d9c0156574495bb090caf3fd78c794` on 2026-11-07. Terms are ordered by runtime responsibility rather than by package name.

## Terms

| Term                   | English                | Definition                                                                                        | file:line                                                                                      |
| ---------------------- | ---------------------- | ------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Operator scope         | Operator scope         | Browser-selected game/environment sent with web requests.                                         | `web/src/services/core/scope.ts:56`                                                            |
| Provider token         | Provider token         | Browser-held authentication token used after sign-in.                                             | `web/src/app.tsx:113`                                                                          |
| HTTP lifecycle         | HTTP lifecycle         | Signal-driven REST start, drain, background stop, and pool close.                                 | `cmd/server/root.go:417`; `cmd/server/root.go:440`                                             |
| Scoped route           | Scoped route           | REST route group whose game database is selected by `GameDBMiddleware`.                           | `internal/handler/routes.go:150`                                                               |
| Service context        | Service context        | Server composition root for relational models, cache, registry, approvals, audit and jobs.        | `internal/svc/service_context.go:221`                                                          |
| Execution governance   | Execution governance   | Shared disable/assignment/subject/ACL/policy/input/approval chain for all execution entry points. | `internal/platform/functioninvoke/govern.go:63`                                                |
| Function contract      | Function contract      | Durable invocation, schema, risk, permission and approval metadata.                               | `internal/model/function_contract.go:15`                                                       |
| Task run               | Task run               | Persisted state machine for one asynchronous function run.                                        | `internal/platform/dispatch/dispatcher.go:54`                                                  |
| Dispatch               | Dispatch               | Agent selection plus durable run creation before an agent request.                                | `internal/platform/dispatch/dispatcher.go:336`; `internal/platform/dispatch/dispatcher.go:775` |
| Relational model store | Relational model store | MySQL/PostgreSQL/SQLite state defined by GORM models and migrations.                              | `internal/svc/migrations.go:186`; `internal/model/task.go:8`                                   |
| Cache store            | Cache store            | Redis or in-process cache selected from server configuration.                                     | `internal/cache/helper.go:15`                                                                  |
| Analytics sink         | Analytics sink         | Batched ClickHouse writes behind the analytics worker.                                            | `internal/analytics/worker/worker.go:435`                                                      |
| Cluster owner          | Cluster owner          | Shared-database or Redis lease resolver assigning scope ownership to one instance.                | `internal/cluster/owner_store.go:8`; `internal/cluster/owner_redis.go:27`                      |
| File task routing      | File task routing      | JSON-file or memory adapter mapping task IDs to agents.                                           | `internal/platform/dispatch/task_routing_store.go:28`                                          |
| Agent runtime          | Agent runtime          | Outbound process owning registration, heartbeat, local services and role runners.                 | `internal/app/agent/app.go:184`                                                                |
| Agent session          | Agent session          | Agent-initiated Server TCP session carrying control and execution traffic.                        | `internal/app/agent/app.go:190`; `internal/server/control_handler.go:128`                      |
| Capture gate           | Capture gate           | Capture policy for sources, filters and durable delivery.                                         | `agents/capture/runner/runner.go:201`                                                          |
| DevOps admission       | DevOps admission       | Audit/admission gate that must pass before a host session opens.                                  | `agents/devops/session/broker.go:126`                                                          |
| Wire frame             | Wire frame             | Shared 4-byte big-endian length-prefix framing.                                                   | `core/transport/tcp/framing.go:25`                                                             |
| Message ID             | Message ID             | 24-bit operation ID across registration, invocation, task and operations families.                | `pkg/protocol/message.go:51`; `pkg/protocol/message.go:71`                                     |
| Provider manager       | Provider manager       | Sidecar registry and health window for SDK provider sessions.                                     | `internal/agent/local_handler.go:52`                                                           |
| Provider invocation    | Provider invocation    | Local selection of a healthy provider followed by handler invocation.                             | `internal/agent/local_handler.go:290`                                                          |
| Function plugin        | Function plugin        | Sidecar plugin contract for describing and invoking plugin functions.                             | `agents/sidecar/funcplugin/protocol/funcplugin.proto:24`                                       |
| Probe plugin           | Probe plugin           | Synthetics go-plugin contract for describing and running probes.                                  | `agents/synthetics/pluginhost/protocol/probeplugin.proto:18`                                   |
| PageSpec               | PageSpec               | Typed draft page model with layout, localized text and bindings.                                  | `internal/dashboard/spec/types.go:537`; `internal/dashboard/spec/types.go:18`                  |
| Published page         | Published page         | Immutable snapshot freezing scope, version, bindings and contract digests.                        | `internal/dashboard/spec/types.go:797`; `internal/dashboard/spec/types.go:823`                 |
| Console menu           | Console menu           | Runtime navigation generated from published pages and menu records.                               | `internal/dashboard/spec/types.go:850`; `internal/api/console/service.go:62`                   |
| Binding execution      | Binding execution      | Published-snapshot execution path mapping inputs and invoking functions.                          | `internal/api/console/service.go:313`; `internal/api/console/service.go:413`                   |

## Boundaries retained

- `gameId + env` remains logical scope; physical placement belongs to target, agent and node records.
- An Agent owns its outbound Server session; an SDK provider owns its local connection to the sidecar.
- A function is executable metadata, not a page or permission. Pages execute only their frozen binding contracts.
- A published page is immutable; drafts are never injected into runtime navigation.

## Related pages

- [Layers](/architecture/layers)
- [Architecture overview](/architecture/)
- [Runtime topology](/architecture/runtime-topology)
