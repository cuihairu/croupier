---
title: Current architecture
---

# Current architecture

> **Game-operations re-review, 2026-10-29:** unified sessions do not imply unified governance. Raw task entry has partial gates; scheduled/manual-trigger execution goes directly to Dispatcher. Policy/schema lookup failures can skip checks, and writing the unique schedule log after dispatch does not prevent duplicate execution. ARCH-023–027 in the root TODO track these gaps; evidence is in `docs/review/code-todo-alignment-2026-10-29.md`. Production stability for high-risk game writes is not established.

Croupier is a game operations control plane for one company with multiple games and environments. The implementation combines a **modular Server, outbound Agents with distinct roles, and game SDKs**, plus a separate analytics pipeline. This overview follows source wiring reviewed on 2026-10-29; design goals are not implied runtime guarantees.

## System diagram

Arrows identify the labeled flow or connection initiator. Agents dial the Server and SDKs dial the sidecar; established sessions carry requests and results in both directions. Dashed edges are optional connections, including cluster coordination. L7/L4 proxies are deployment options; a single instance may be reached directly.

<ArchitectureDiagram lang="en" />

## Processes and responsibilities

| Component                 | Current responsibility                                                                          | Boundary                                                                                                                                                                                                                    |
| ------------------------- | ----------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Web console               | React/Umi/Ant Design, resources, Page Studio and operator actions                               | HTTP API/SSE; no direct Agent sessions                                                                                                                                                                                      |
| Server                    | Gin API, identity, scope, policy, approvals, audit, contracts, pages, registry/dispatch and Ops | One modular deployment unit                                                                                                                                                                                                 |
| sidecar-agent             | SDK provider sessions, game function dispatch, tasks and managed process/plugin wiring          | Built as croupier-agent; some wiring remains in internal/app/agent                                                                                                                                                          |
| capture-agent             | Database change capture, checkpoints, filtering and rules                                       | Separate from SDK function routing                                                                                                                                                                                          |
| synthetics-agent          | Role-specific probes, results and probe plugins                                                 | Role-specific application lifecycle                                                                                                                                                                                         |
| devops-agent              | Registration, allowlist sync, Broker/Hub session foundations                                    | Console-to-Agent relay is incomplete; RDP/VNC unavailable                                                                                                                                                                   |
| ingest / analytics-worker | Event publishing, consumption and ClickHouse aggregation                                        | Built-in worker consumes Redis Streams; Kafka publishing alone is not an end-to-end pipeline. The worker refuses to start under `ANALYTICS_MQ_TYPE=kafka` (ARCH-013 preflight) so the pair cannot be silently misconfigured |

## Execution and publication

1. HTTP authentication and scope middleware resolve and authorize gameId + env.
2. Console execution checks the selected PageSpec snapshot and binding. Function calls converge in internal/platform/functioninvoke.
3. Disabled/assignment gates, policy, approval and input-contract checks run before dispatch as applicable.
4. Dispatch selects a scoped Agent, using its local session or forwarding to the Server instance that owns that session.
5. The sidecar invokes the game SDK over its provider session. task_runs/task_events persist asynchronous progress for queries and SSE.
6. Audit is recorded according to the relevant path and policy. Execution logs are a separate mechanism, not an audit transaction guarantee.

Publication follows **registration/OpenAPI → FunctionContract → resource semantics → proposal/draft → validation and publication → Console binding execution**. The Console uses published snapshots; Page Studio also has a controlled draft execution path. Database-backed page idempotency does not make all external side effects exactly-once.

## State and availability

| State                                            | Current location                                            | Implication                                                             |
| ------------------------------------------------ | ----------------------------------------------------------- | ----------------------------------------------------------------------- |
| Live Agent/SDK connections                       | Owning process memory                                       | Access through owner forwarding, not shared net.Conn                    |
| Registry                                         | Local registry and database agent_sessions snapshots        | Discovery snapshots do not replace live sessions                        |
| Membership and owner leases                      | Shared relational database or configured Redis              | One-hop forwarding and epoch fencing                                    |
| Business data, pages, tasks and page idempotency | Relational database; some game models support dbctx routing | Verify scope behavior for each model                                    |
| Task routing                                     | Local cache/file with task_runs lookup fallback             | Cross-instance cancellation has a database fallback                     |
| Function approvals                               | MemStore in current ServiceContext wiring                   | SQLStore exists but is not wired for restart/cross-instance persistence |
| Audit                                            | SQLAuditStore and AuditService                              | Hash-chain validation; no signer wired by default                       |
| Analytics                                        | Redis Streams → worker → ClickHouse                         | Separate from the control database                                      |

HA routing exists, but not every business module has shared state. Some cluster initialization failures fall back to standalone; healthz does not fully represent cluster readiness.

## Transport and trust

- Server HTTP: 18780 by default; HTTPS commonly terminates at the reverse proxy.
- Agents dial Server control on 19090. SDKs dial the sidecar provider on 19091. Ports are configurable.
- Custom TCP sessions use length-prefixed framing and protobuf platform messages; function payloads are typically UTF-8 JSON.
- core/transport/tcp owns MuxConn. Roles explicitly compose registration, reconnect and configuration packages.
- Agent-to-Server TLS is supported but **disabled in repository development templates**: Server cert/key are empty and Agent insecure=true. Configure certificates explicitly for production.
- Server interconnect currently uses plaintext TCP and role-only hello messages. Keep it on a trusted isolated network; mTLS remains pending.

## Scope and databases

The authorization scope is gameId + env. Targets, Agents and nodes describe execution placement. Each Agent instance declares one scope.

Single-database mode still validates scope. multiGame separates metadata from selected game models through dbctx. Binding validation and physical database-name calculation currently follow separate paths: verify mappings before changing prefixes or migrating legacy database names. A logical environment does not automatically imply dedicated hardware, a cluster or a database.

## Source map

| Concern                 | Repository paths                                                           |
| ----------------------- | -------------------------------------------------------------------------- |
| Bootstrap               | cmd/server/root.go, cmd/server/cluster.go, internal/svc/service_context.go |
| API and scope           | internal/handler/routes.go, internal/svc/game_middleware.go                |
| Execution               | internal/platform/functioninvoke, internal/platform/dispatch               |
| Registry and clustering | internal/platform/registry, internal/cluster                               |
| Pages                   | internal/dashboard, internal/api/page, internal/api/console                |
| Governance              | internal/platform/approvals, internal/audit                                |
| Storage                 | internal/db/router, internal/db/dbctx                                      |
| Agents                  | agents, core, internal/app/agent, cmd/*-agent                              |
| Analytics               | cmd/ingest, cmd/analytics-worker, internal/analytics                       |

## Reading path

- [SDK wire protocol](./sdk-wire-protocol)
- [Session lifecycle](./session-lifecycle)
- [Multi-instance HA and limitations](./server-ha-multi-instance)
- [Agent roles](../operations/agent-comparison)
- [Analytics](../analytics/)
- [Quick start](../guide/quick-start)

## Runtime evidence and lifecycles

[Process inventory, connection matrix, lifecycle SVGs, deployment failures and official-reference checks](./runtime-topology). Diagrams show current wiring; durable approvals, interconnect mTLS, Provider draining and reliable task replay remain follow-up work.
