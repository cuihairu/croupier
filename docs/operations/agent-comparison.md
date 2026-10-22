---
title: The Four Agent Roles, Deployment and Build Artifacts
---

# The Four Agent Roles, Deployment and Build Artifacts

Croupier plans four agent roles; three have executable entry points today, and synthetics is still to be implemented. All four instance types are managed under a unique `gameId + env`; network connections, registration heartbeat, config sync, and the probing kernel reuse shared capabilities as much as possible, with role-specific responsibilities assembled separately.

## 1. Selection Summary

| Type       | Core responsibility                                                                                  | Deployment location                                                                               | Current entry / binary                           | Current implementation boundary                                                                                                                              |
| ---------- | ---------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- | ------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| sidecar    | Game SDK function registration, invocation/task forwarding, game-side supervision and observability  | On the game server host or a network segment that can reach the game services                     | `cmd/sidecar-agent` / `croupier-agent`           | The SDK session and invocation core exist; the supervision modules' implementation and CLI config/startup wiring still lag                                   |
| devops     | Authorized personnel operate intranet machines from the console, with allowlist and audit up front   | An intranet ops node that can reach the controlled hosts                                          | `cmd/devops-agent` / `devops-agent`              | Registration, heartbeat, broker, allowlist, audit, and session metadata exist; SSH/RDP/VNC, downstream sessions, and content replay are not wired            |
| capture    | CDC capture of business database changes, filtering, reconciliation/CEP/lineage detection, alerting  | A bypass node that can read the source database's replication log                                 | `cmd/capture-agent` / `capture-agent`            | MySQL/MariaDB, PostgreSQL, MongoDB, and the rule chain exist; position confirmation, rule-window recovery, and reliable alert delivery still need tightening |
| synthetics | Active probing of game-external and third-party services, launching developer multi-language plugins | A network position representing players or service consumers, possibly multi-region/multi-segment | To be implemented; no entry point/binary/package | Standalone carrier, go-plugin runtime, and the corresponding pages are to be implemented; the built-in Checker does not mean this Agent is done              |

`connector-agent` is only a placeholder on the external-system event-ingest roadmap and does not count as a fifth implemented Agent. For full responsibilities and page planning see the [Agent inventory](../design/agents-inventory.md) and the [Four console page sets](../design/agent-dashboard-design.md).

## 2. sidecar: game functions and the runtime sidecar

Used when a game needs to integrate Croupier function registration, GM invocations, and task execution. The SDK connects proactively to its local gateway (commonly port 19091), the sidecar connects proactively to the Server control plane (commonly port 19090), and Server invocations reuse the existing bidirectional connection; the Server is never required to reach into the game's intranet.

Functions can come from the six-language SDK inside the game process, or from existing Provider/OpenAPI integrations; the optional go-plugin external function adapter is a future design that maps functions onto the game's existing HTTP/TCP management interfaces — it does not mean any game can be operated without interfaces.

The supervision module covers start/stop/restart, backoff circuit breaking, desired state, scheduled tasks, snapshots, and managed-program updates, but the CLI does not yet pass OpsConfig through completely, nor is OpsServer.Start wired; supervision items in config examples must not be treated as production-verified. Remote control requires explicit permissions/a local switch, and the sidecar provides no bastion-style SSH/RDP/VNC access.

- Inputs: SDK registrations, Server invocations/tasks/explicitly authorized actions, config policies.
- Outputs: function and provider lists, invocation results/task events, system/supervision/probe information.

## 3. devops: intranet remote sessions

For development/ops staff to access allowlisted machines, e.g. troubleshooting an intranet build machine; it does not do game function registration or database change capture. Targets must be explicitly allowlisted by host+port+protocol, a session requires an audit record persisted first, and setup is refused when the audit record is missing.

Currently with `devops.enabled=false` it only registers and comes online; even when set to true, production wiring is still a noop Target and cannot establish real SSH/RDP/VNC sessions. The existing session start/end metadata is not terminal recording or replayable content, and the console entry ships with the real channel.

- Inputs: the allowlist and session config; later, authorized session requests.
- Outputs: online identity, session audit and metadata; real terminal/desktop streams are to be implemented.
- Config example: `configs/devops-agent.example.yaml`; after installation the config is named `devops-agent.yaml`.

## 4. capture: CDC and business-change inspection

Continuously reads database change logs, converts different sources into a common ChangeEvent, then applies event filtering and three gates: business-flow reconciliation, temporal-pattern CEP, and write-lineage detection. Supports MySQL/MariaDB binlog, PostgreSQL WAL logical replication, and MongoDB change streams; MongoDB requires a replica set/mongos-capable environment.

Rules are pulled from the platform per scope, keeping their version and appliedRevision; the capture source is read-only, and detection hits only report alerts — game assets are never modified directly. Credentials are passed via DSN environment variable references, and tables/collections use explicit allowlists.

Currently a full alert queue or a send failure can drop alerts, and the relationship between connector checkpoints and processed-event acknowledgments still needs failure testing and tightening — reliable delivery or exactly-once under disconnection/crash cannot be promised. Source snapshots, before-images, transaction metadata, and similar capabilities are declared per connector; one source's capability must not be generalized to all sources.

- Inputs: source config, resume position, filter and three-gate policies.
- Outputs: capture statistics, positions, rule versions, and hit alerts.
- Config example: `configs/capture-agent.example.yaml`; after installation the config is named `capture-agent.yaml`.
- ARMv7 does not support this dependency combination — choose amd64/arm64; see the build platform table below.

## 5. synthetics: external game and third-party service probing (to be implemented)

Checks handshakes, logins, and game-defined business paths from outside the game process, while also checking third-party payment, account, messaging, and other dependencies; targets require `targetCategory=game/third_party`, both categories carry gameId/env, and category vs protocol/plugin capabilities are modeled separately.

The host plans to use HashiCorp go-plugin to launch persistent, reusable developer programs invoked over local gRPC IPC; game protocols are implemented by developers in the language of their choice, and results feed the shared healthprobe kernel for thresholds, failure windows, and statistics. Plugin failure, port reachability, and target business availability are surfaced separately.

The local plugin's gRPC dependency stays in a dedicated probe host/plugin; the game SDK keeps lightweight TCP access, and Agent→Server shares TCP/TLS. Plugin and OTel default integration, runtime shutoff, and compile-time trimming are later delivery requirements; today there is no synthetics install command, runnable config, or nightly placeholder package. See the [go-plugin survey](../research/synthetics-agent-go-plugin.md).

## 6. Every Instance Declares Its Game Environment

```yaml
agent:
  id: demo-prod-capture-01
  gameId: demo
  env: prod
```

This only illustrates the three identity fields shared by all roles; each binary's upstream address and business config follow its own example; the ID naming does not replace explicit fields. Run multiple independent instances across games or environments, with data directories, credentials, positions, and plugin processes isolated separately.

The sidecar currently enforces both fields; env may still be empty in devops/capture configs — a known gap to fix — so fill it explicitly at deployment. Server ownership, page labeling, and cross-scope acceptance follow the [scope spec](../architecture/game-environment-scope.md).

## 7. Nightly and Stable Downloads

After the release chain adjustment, the bundle `croupier-bin-<os>-<arch>.tar.gz` (zip on Windows) remains, alongside:

| Standalone package                    | Content                                                             |
| ------------------------------------- | ------------------------------------------------------------------- |
| `croupier-agent-<os>-<arch>.tar.gz`   | sidecar binary, `configs/agent.yaml`, AGENTS.md, INSTALL.md         |
| `devops-agent-<os>-<arch>.tar.gz`     | devops binary, `configs/devops-agent.yaml`, role and install docs   |
| `capture-agent-<os>-<arch>.tar.gz`    | capture binary, `configs/capture-agent.yaml`, role and install docs |
| `agents-<os>-<arch>.json` / `.sha256` | Manifest with roles, archive/binary SHA256, and checksum list       |

Windows uses `.zip` and `.exe`, also with configs and docs. Supported targets:

| Platform                         | sidecar | devops                 | capture                             | Server bundle             |
| -------------------------------- | ------- | ---------------------- | ----------------------------------- | ------------------------- |
| Linux amd64/arm64                | yes     | yes (session skeleton) | yes                                 | yes                       |
| Linux ARMv7 (package name armv7) | yes     | yes (session skeleton) | no, 32-bit CDC parsers incompatible | no, same dependency limit |
| macOS amd64/arm64                | yes     | yes (session skeleton) | yes                                 | yes                       |
| Windows amd64                    | yes     | yes (session skeleton) | yes                                 | yes                       |

`nightly.yml` is the only daily release entry; `release.yml` only publishes v* tags; workflow changes must merge and run successfully before they appear on GitHub Releases — this page does not claim new assets are published. Cross-platform build success is not functional acceptance in a real environment.

The Linux/macOS installer continues to support role-based installs from the bundle:

```bash
# sidecar can likewise be swapped for devops or capture (capture requires 64-bit)
curl -fsSL https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.sh | bash -s -- --agent sidecar --version nightly --game-id demo --env prod
```

For manual installs, configure the real server address, gameId/env, and credential references first — never use an example as production config; verify the archive SHA256 before extracting. The Windows install script currently covers the sidecar only; other roles are deployed manually. For details see the one-line installer.

## 8. Source Layout, Shared Capabilities and Tests

Binary entry points live uniformly under the root `cmd/`, role-specific business in `agents/<role>`, and shared pieces in `core/`; the sidecar's historical business still sits in internal. For the full directory and the MuxConn/supervision boundaries pending extraction see the [shared runtime and layout](../development/agent-runtime-layout.md).

```bash
make agents              # the three existing agents; individual builds still use agent/agent-devops/agent-capture
# shared pieces and role implementations are all in CI; environment dependencies are covered in the integration-test notes
go test ./core/... ./agents/...
```
