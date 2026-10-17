---
title: Agent core design
---

# Agent core design

`agent-core` is a small-parts library, not a framework. Each capability lives in a separate package, each agent binary imports only what it needs, and an unused package is not compiled into the binary. The design goal is to add capture, devops, and future agents without duplicating registration, reporting, auditing, probing, or supervision semantics.

## Design decisions

| Decision                                | Consequence                                                                                                                                |
| --------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| No god object or `core.App`             | The agent main owns assembly and imports only required packages.                                                                           |
| Business capabilities stay outside core | Capture CDC, devops sessions, and function routing remain role-specific.                                                                   |
| Shared runtime semantics move into core | Registration, heartbeat, reporting, metrics, probing, execution logs, and supervision contracts have one implementation.                   |
| Agents are supervised targets           | `supervisable.Main` installs signals, exit-code discipline, and heartbeat context; the sidecar supervisor can manage any core-based agent. |
| Audit before risky execution            | Command or mutation execution requires a persisted execution record before the operation starts.                                           |

## Package responsibilities

| Package             | Responsibility                                                                      |
| ------------------- | ----------------------------------------------------------------------------------- |
| `core/backoff`      | Shared retry/backoff policy wrapper.                                                |
| `core/logx`         | Structured logging and rotation setup.                                              |
| `core/register`     | Server registration, heartbeat, re-registration, and reconnect behavior.            |
| `core/report`       | Common upstream reporting; metrics first, with the alert wire added separately.     |
| `core/supervisable` | The supervised-process contract and thin main entry point.                          |
| `core/metrics`      | Process and host metric collection independent of the function channel.             |
| `core/configsync`   | Version polling, fetch, and hot application of configuration.                       |
| `core/healthprobe`  | Liveness, readiness, heartbeat checks, failure thresholds, and fault windows.       |
| `core/execlog`      | Execution audit records, local truth files, upload interface, and audit-first gate. |
| `core/crash`        | Snapshot profile handling and dump-directory management.                            |

Task execution and the local SDK/provider gateway remain sidecar-agent responsibilities. Omitting them from core also removes those attack surfaces from capture and devops agents.

## Health probes

| Semantic  | Question                       | Typical consumer                                        |
| --------- | ------------------------------ | ------------------------------------------------------- |
| Liveness  | Is the agent process alive?    | Sidecar game-process supervision.                       |
| Readiness | Are dependencies ready?        | Database/replication, guacd, or whitelisted host ports. |
| Heartbeat | When was the last live signal? | Supervisor context and console freshness.               |

Built-in HTTP, TCP, and process checkers use consecutive-failure thresholds. A fault is modeled as a window with a start time, recovery time, duration, and last reason. The local timeline is the durable truth; snapshots and window events can be reported to the console and forwarded to notification orchestration.

## Execution auditing

An execution record captures operator, agent type and instance, action, parameters, result, duration, timestamp, business scope, and task/command identity. Local rotating files preserve records when the server is unreachable. Server ingestion is an upload boundary, not a prerequisite for local truth.

The audit-first gate applies to command-like mutations: if the record cannot be persisted, the operation is refused. Read-only polling does not pay this gate.

## Extraction batches

The initial extraction was split into independently shippable batches:

1. Backoff, logging, and health probes.
2. Registration and supervised-process contracts.
3. Reporting, metrics, and execution auditing.
4. Configuration synchronization and crash support.

Each batch kept the existing sidecar implementation compilable, so a batch could stop without leaving a half-migrated dependency.

## Known boundaries

- Some core packages were built before every sidecar consumer was switched; consumer wiring proceeds by role and batch.
- The alert-report wire and server-side execution-log ingestion may trail the local implementation.
- Health-probe reporting and console consumption can be enabled after the local timeline is stable.
- Historical nesting under `agents/<role>/cmd` is not the current entry-point model; actual entry points remain under the repository root `cmd/`.
- Core and legacy internal paths coexist during migration; the runtime layout page is authoritative for current boundaries.
