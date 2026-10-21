---
title: Agent Configuration Guide
icon: settings
order: 11
category:
  - Operations Manual
tag:
  - configuration
  - agent
---

# Agent Configuration Guide

Corresponds to `configs/agent.yaml` (template in the repository). This binary has **no environment-variable expansion layer**: configuration is read only from YAML; runtime environment variables are limited to `CROUPIER_CONFIG_DIR` / `CROUPIER_AGENTLOCAL_*` etc. (see the environment variable table in agent-config.md), and the only CLI parameter is `--config` for the config file path.

## Complete Key Reference

### server (upstream connection)

| Key                         | Default  | Description                                                                                                        |
| --------------------------- | -------- | ------------------------------------------------------------------------------------------------------------------ |
| `server.addr`               | required | Server transport entry point. In production point it at an L4 LB (HAProxy/NLB/Service), not at a specific instance |
| `server.insecure`           | `false`  | TLS by default; set `true` for a plaintext connection (dev troubleshooting only)                                   |
| `server.serverName`         | `""`     | TLS SNI/verification name (used when it differs from the certificate CN)                                           |
| `server.insecureSkipVerify` | `false`  | Skip certificate verification (troubleshooting only; forbidden in production)                                      |

### agent (identity and reachability)

| Key                          | Default                                      | Description                                                                                                                                                    |
| ---------------------------- | -------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `agent.id`                   | `""`                                         | Auto-generated when left empty; specifying it explicitly is recommended for multi-agent deployments (readability on the topology page)                         |
| `agent.gameId` / `agent.env` | required                                     | Scope binding; startup fails when empty (one agent per game+env)                                                                                               |
| `agent.localAddr`            | empty → host:port fallback, then `0.0.0.0:0` | Local TCP listener (entry point for game-server function registration)                                                                                         |
| `agent.httpAddr`             | required                                     | **Server→Agent callback address**; must be reachable from the Server's point of view (service name in container networks, IP on bare metal)                    |
| `agent.labels`               | `{}`                                         | Custom labels (datacenter/machine type etc., used for filtering on the node page)                                                                              |
| `agent.invokeTimeoutMs`      | `15000`                                      | Default budget (ms) for Agent→game-server synchronous calls; when a request's `metadata["timeoutMs"]` declares a smaller value the smaller one wins; cap 60000 |

> `httpAddr` is the most common deployment mistake: writing `0.0.0.0` or `localhost` breaks Server callbacks — job dispatch/file transfer will not go through.
>
> Registration labels include the agent's self-reported system labels (`os`/`arch`/`hostname`/`ip` etc.) and the running-binary fingerprint `agentBinarySha256` (agent-supervisor-design §10.5 persistence anchor: self-reported at every registration, persisted together with the registration labels; after the agent goes offline, system-info uses it to answer "last known fingerprint"). The fingerprint is injected after config/dynamic label merging — explicitly writing the same key in `agent.labels` has no effect (computed facts cannot be overridden).

### upstream (connection resilience)

| Key                          | Default             | Description                                                                                                |
| ---------------------------- | ------------------- | ---------------------------------------------------------------------------------------------------------- |
| `upstream.heartbeatInterval` | `0`→core default 3s | Heartbeat in seconds; the Server declares an agent lost after 3× the interval                              |
| `upstream.retryInterval`     | `5`                 | **DEAD**: nobody reads it after parsing; reconnect backoff is fixed 5s→60s×1.5 (core/register/register.go) |
| `upstream.maxRetries`        | `3`                 | **DEAD**: nobody reads it after parsing (same as above)                                                    |
| `upstream.timeout`           | `10000`             | Request timeout (ms)                                                                                       |

### tls (inbound TLS on the local `:19091` listener)

`tls.enabled / certFile / keyFile / caFile / insecureSkipVerify` — encrypts connections where SDK Providers dial into the Agent's local gateway; TLS sits below the framing, and the wire protocol is unchanged. With `enabled: true`, `certFile` + `keyFile` are mandatory (missing → immediate startup error); configuring `caFile` enables mutual TLS (Providers must present a CA-signed client certificate). A failed handshake on a single connection only drops that connection (a warning is logged) and does not affect the Accept loop or other sessions. The SDK side needs TLS dialing options (the six language SDKs are being tracked one by one; keep `enabled: false` until then).

### outboundTLS (TLS for the Agent → local Provider outbound call chain)

Client TLS for the Agent's outbound calls to game-side Providers (dialed by the local gateway `callLocalProvider`): `enabled / certFile / keyFile / caFile / serverName / insecureSkipVerify`. With `enabled: false` (the default) this link stays plaintext (compatibility fallback). Note the division of labor with the `server:` block: **TLS for the agent → server upstream link is driven by the `server:` block** (enabled when `server.insecure: false`; the certificate fields also live in the `server` block), while `outboundTLS` only applies to the outbound Agent→local-Provider link. In production, an internal CA is recommended; see [TLS and Certificates](./tls-certificates).

### ops (metrics reporting and process supervision)

| Key                   | Default                                                         | Description                                                            |
| --------------------- | --------------------------------------------------------------- | ---------------------------------------------------------------------- |
| `ops.enabled`         | `false`                                                         | System metrics collection and reporting (must be enabled explicitly)   |
| `ops.metricsInterval` | `30s`                                                           | Collection interval (also the sampling interval for managed processes) |
| `ops.metricsEnabled`  | `false` (YAML default is false; must be set to true explicitly) | Metrics switch (stops collection but keeps the reporting channel)      |

> Note: older deployments used the snake_case legacy keys `ops.metrics_interval` / `ops.metrics_enabled`, which are still parsed for compatibility; new configurations must use the lowerCamelCase keys in the table above.
> The remaining `ops` keys (`allowRestart` / `allowExec` / `allowUpdate`, `managedProcesses`,
> `execAllowedCommands`, `supervisorLog`, `snapshotDir`) are parsed directly by the sidecar CLI and passed through
> to agentcore's `Validate()`; invalid closed-set, cron, and allowlist configuration fails at startup and exits.

### ops.managedProcesses (managed process supervision, supervisor)

> Processes with `autoRestart: true` are brought up at boot by the sidecar's normal startup path; the desired state of a process stopped manually by operations survives sidecar restarts.

Each entry is keyed by a logical name and declares one agent-managed process. S1 (read-only monitoring) is delivered: RSS/CPU is sampled once per `ops.metricsInterval`; above threshold a flag is raised on the panel (processes are not killed automatically), and snapshots ride along with metrics reporting.

**Managed state**: the desired state of a process stopped manually by operations is persisted to `desired_state.json` in the supervisor log directory — `autoRestart: true` only restores **crashed** processes, so the boot-time autostart after an agent restart does not mistakenly start a process operations stopped on purpose; a manual start/restart restores the desired running state. A missing or corrupt file fails open (treated as no record; the existing autoRestart semantics apply).

**Scheduling state** (§10.6): `schedule[]` declares timed start/stop/restart. Timed triggering and the recovery loop are two execution paths — a timed stop uses the same signal as a manual stop (stopCh), so the recovery loop will not mistakenly bring up a process stopped by schedule; triggering goes through the same funnel as explicit actions (desired state persisted + a `schedule_fire` event riding along, typed separately from `manual_start`/`manual_stop` events). Snapshots carry the next trigger time and action (`nextScheduleAtUnix`/`nextScheduleAction`; zero values when there is no schedule). Honest boundary: the schedule is agent-local configuration intent, with the same trust root as `autoRestart`/boot autostart, and is **not gated by `allowRestart`** (`allowRestart` gates the remote operation surface) — to disable timed actions, delete the `schedule` configuration. **Pausing/resuming the schedule** (§10.2 pause and resume; the pause target = scheduling): `POST /ops/agents/:agentId/processes/:name/schedule/pause` (or `resume`) freezes/unfreezes all cron triggers for that process, the process runtime state is untouched, and slots missed during the pause are not caught up (crond semantics); **it is a remote ops action subject to the `allowRestart` double gate** (this does not contradict "the trigger itself is not gated" above — the gate covers the remote on/off switch, not local configuration intent); there is no dedicated configuration key; the pause intent is persisted to `logs/supervisor/schedule_pause_state.json` (survives agent restarts; corrupt → fail-open empty map); snapshots report `scheduleSuspended` upward, and events leave a trace as `schedule_paused`/`schedule_resumed`.

### ops.allowExec (remote command execution, §10.7 three-part boundary)

`allowExec: true` enables remote command execution (`POST /ops/agents/:agentId/exec`); all three prerequisites are mandatory:

1. **Double gate** (existing baseline): `ops.enabled: true` + `allowExec: true` both default to false and must be turned on explicitly together.
2. **Allowlist fail-closed**: `execAllowedCommands` must be non-empty — an empty list rejects every command and is **no longer equivalent to "allow everything"** (before materialization on 2026-10-15 it was fail-open; this has been corrected). `OpsConfig.Validate()` blocks the configuration surface in sync.
3. **Audit**: every attempt (success / business failure / client unavailable) writes `execution_logs` (`source=agent_exec`); operator, command, arguments, exit code, and duration go into the audit payload; a missing audit surface does not block the main execution path (fail-open).

> Note: the panel has no general-purpose Shell entry — program operations always go through the explicit actions of managed processes (start/stop/restart); this is the existing §10.7 baseline of "program operations = explicitly authorized actions".

**Managed state**: the desired state of a process stopped manually by operations is persisted to `desired_state.json` in the supervisor log directory — `autoRestart: true` only restores **crashed** processes, so the boot-time autostart after an agent restart does not mistakenly start a process operations stopped on purpose; a manual start/restart restores the desired running state. A missing or corrupt file fails open (treated as no record; the existing autoRestart semantics apply).

| Key                                       | Default   | Description                                                                                                                                                                                                                                                                                                                                                                                                                   |
| ----------------------------------------- | --------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `command` / `args` / `workingDir` / `env` | —         | Process launch parameters                                                                                                                                                                                                                                                                                                                                                                                                     |
| `kind`                                    | `program` | Managed object category (closed set `program` / `probe-adapter`, default program). `probe-adapter` = external probe adapter, going through the same lifecycle as program (§10.1)                                                                                                                                                                                                                                              |
| `adapterId` / `adapterVersion`            | empty     | Adapter identity and version (manifest metadata.id / version). With `kind=probe-adapter`, `adapterId` is **required**; missing → configuration validation fails — it is the anchor for probe-surface reference and authorization checks                                                                                                                                                                                       |
| `adapterProtocol` / `adapterCapabilities` | —         | Adapter IPC protocol name (default `croupier-probe-jsonl-v1`) and declared capability set (e.g. `login` / `handshake`). **Declared ≠ verified**: the platform still judges target state by actual probe results; game address/frequency/authorization/credentials do not go into this manifest (left to probe-surface configuration)                                                                                          |
| `autoRestart`                             | `false`   | Switch for crash auto-restart and sidecar startup bring-up (exponential backoff + circuit breaker, S2 delivered)                                                                                                                                                                                                                                                                                                              |
| `memThresholdBytes`                       | `0` (off) | Raise the `mem_over_limit` flag when RSS reaches this value                                                                                                                                                                                                                                                                                                                                                                   |
| `cpuThresholdPercent`                     | `0` (off) | Raise the `cpu_over_limit` flag when CPU% reaches this value                                                                                                                                                                                                                                                                                                                                                                  |
| `snapshotProfile`                         | `none`    | S3 crash snapshot profile, closed set `none/go/node/python/jvm` (declared explicitly, no language auto-detection): `go` injects `GOTRACEBACK=crash`; `node` prepends `--report-on-fatalerror --report-exclude-env`; `python` injects `PYTHONFAULTHANDLER=1`; `jvm` injects `JAVA_TOOL_OPTIONS` (`-XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=<snapshot dir>`)                                                            |
| `schedule[]`                              | empty     | Timed action list (§10.6 scheduling state): `action` closed set `start/stop/restart` + five-field `cron` (minute hour day month weekday, same parser as server-side task scheduling). Firing goes through the same funnel as explicit panel actions (desired state persisted + `schedule_fire` event riding along); 30s minute-slot scanning with dedup, no catch-up (slots missed while the agent was down are not re-fired) |

```yaml
ops:
  enabled: true
  metricsInterval: 30s
  # S3: root directory for crash snapshot artifacts (one subdirectory per process, permission 0700 — artifacts contain process memory).
  snapshotDir: logs/snapshots
  managedProcesses:
    demo-app:
      command: /usr/local/bin/demo-app
      args: ["--port", "8080"]
      workingDir: /srv/demo-app
      memThresholdBytes: 2147483648 # 2GiB
      cpuThresholdPercent: 90
      snapshotProfile: go # dump full goroutine stacks on crash and attempt a core dump
      schedule: # timed actions (§10.6): stop at night, start on weekday mornings
        - action: stop
          cron: "0 3 * * *"
        - action: start
          cron: "30 7 * * 1-5"
```

Panel entry: the node maintenance page's "Supervision" column → the process supervision drawer (status light/PID/uptime/memory/CPU/over-limit flags/snapshot profile). On a crash, a `detect_down` event records the snapshot directory and artifact filename; when the host's `core_pattern` is not in pipe mode, the agent sends a one-time read-only `snapshot_hint` tip (including suggested commands; the agent does not write host configuration). Retention/quota/artifact download permissions: see [Agent Supervisor Design Brief (this section is Chinese-only)](/zh/design/agent-supervisor-design).

### ops.allowUpdate (managed program binary update, §10.4)

`allowUpdate: true` enables remote package swap (`POST /ops/agents/:agentId/processes/:name/update`, inputs `stagedPath` + `expectedSha256`); it belongs to the double-gate family alongside allowExec (both `ops.enabled` + `allowUpdate` default to false and must be turned on explicitly together), with independent keying.

**Update state** (§10.4): the flow = content-hash pinning (a staged file whose sha256 ≠ `expectedSha256` is always rejected, fail-closed, running state untouched) → stop before swap (quiet core, no `manual_*` intermediate events) → back up the old binary as `<command>.croupier-bak` (**kept after success** as last-known-good, overwritten on each roll, for manual rollback) → bring up → exiting within the stability window (default 5s) triggers automatic rollback to the old version. The update intent lands as desired running (the same as a manual restart). The fingerprint is recorded in three places: supervisor events `update_applied` / `update_rolled_back` (message carries old→new sha256), server-side execution_logs (`source=process_update`, the primary persistent query entry, searchable by target+time+fingerprint), and the process snapshot `fingerprint` field reported with metrics.

Honest boundaries (v1): signature verification is not implemented (content hash only — the hash is a stable, queryable anchor; source trustworthiness is guaranteed by the distribution channel); database/configuration compatibility checks are not implemented (game-side knowledge, guarded by the caller); game process maintenance windows/online thresholds/traffic admission gates are unverified, to be settled with the first real game onboarding; a first installation (no old version) fails with no event (trace left in execution_logs).

### probe (declarative probe surface)

Agent-side probe configuration (R2). `targets[]` only declares **what to probe**; thresholds, semantics, sampling interval, and reporting semantics are given explicit defaults by `core/healthprobe`. **The presence of a `probe:` block enrolls the agent in the probe surface** (R4): an empty `targets` is a legal declaration (empty static + dynamic targets can be added/removed — db probe targets derived by the server from DB source ownership get pushed in); omitting the whole block = the probe surface is not assembled (zero behavioral difference). Any invalid local target (duplicate name/unknown type/missing required parameter) **fails startup immediately** — no silent skipping: duplicate names would cross-wire upstream routing.

| Key                         | Default              | Description                                                                                                              |
| --------------------------- | -------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| `probe.targets`             | empty → dynamic only | Local target list (table below); empty = only server-derived db targets (R4)                                             |
| `probe.maxDetailsPerWindow` | `10`                 | Cap on failure detail entries per target per fault window (the cap trims details only, not the total failure count fact) |

**Per-target keys** (`core/healthprobe.TargetConfig`, keys are all lowerCamelCase)

| Key                    | Default          | Description                                                                                                                  |
| ---------------------- | ---------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| `name`                 | required         | Target identifier (unique within one agent; both the alert-line identifier and the metrics path `probe.<target>.*` use it)   |
| `type`                 | required         | Closed set `http` / `tcp` / `icmp` / `db` (`process` does not go through the factory; it is handled by the supervisor side)  |
| `url`                  | type=http        | HTTP probe address                                                                                                           |
| `address`              | type=tcp         | `host:port`                                                                                                                  |
| `address`              | type=icmp        | Hostname or IP; ICMP also supports `count` (default 1, needs `CAP_NET_RAW`)                                                  |
| `dsn`/`driver`/`query` | type=db          | Database connection string / driver name (a driver registered by the binary's dependencies) / assertion SQL                  |
| `timeout`              | per-type default | Single probe timeout                                                                                                         |
| `semantics`            | `readiness`      | Perspective declaration, closed set `readiness` (ready: is the function usable) / `liveness` (alive: is the process running) |
| `interval`             | `30s`            | Local base sampling frequency — **the fallback when a pushed policy does not cover this target**                             |
| `failThreshold`        | `3`              | How many consecutive failures open a fault window                                                                            |
| `recoverThreshold`     | `1`              | How many consecutive successes close a fault window                                                                          |

```yaml
probe:
  maxDetailsPerWindow: 10
  targets:
    - name: game-db-primary
      type: db
      dsn: "user=app@tcp(127.0.0.1:3306)/demo"
      driver: mysql
      query: "SELECT 1"
      semantics: readiness
      interval: 30s
      failThreshold: 3
    - name: platform-api
      type: http
      url: "http://127.0.0.1:8080/healthz"
      semantics: liveness
      timeout: 3s
    - name: redis
      type: tcp
      address: "127.0.0.1:6379"
    - name: gateway
      type: icmp
      address: "192.0.2.1"
      timeout: 2s
```

**The single source of truth for sampling frequency is the Server** (configuration and execution are separated): after the panel publishes a `probe.policy` sampling strategy it is pushed over the wire and takes effect on the agent hot (four stacked granularities: base/per-target/time-window/special-times); at that point **the policy takes precedence over this table's `interval`**, which serves only as the fallback for uncovered targets; when no policy is published, this table's values are used. See the [API documentation probe endpoint](/archive/legacy-en/api/ops) and the [Review Brief §13.3](/archive/legacy-en/design/agent-health-recovery-review).

**Derived db probe targets (R4)**: agents with a `probe:` block periodically pull the db target set derived by the server from `db_sources.owner_agent_id` (30s polling, target names `db:<source name>`, duplicate source names disambiguated with a `#<id>` suffix, driver mapping postgres→pgx, probe assertion `SELECT 1`). On pull failure or "empty version + empty body" (the server cannot derive anything), the existing targets are kept; **an empty set** is a real state (all ownership removed), and the agent tears down all dynamic probes; the version is the content hash of the target set — if the set is unchanged, it is skipped. If a derived target has the same name as a local target, the push is rejected wholesale (retry next round) — no silent cross-wiring. DSN plaintext is only returned over the control connection to the owning agent, with the same trust boundary as local `probe.targets`.

Probe results are visible in two places: the node maintenance page's "Probe Summary" card (a status matrix, abnormal rows first) and the alert center (window open/close drives the firing/resolved lifecycle; alert ID `probe:<agentId>:<target>`).

## Dual-Agent / Multi-Agent Deployment

Multiple agents on one host (multiple games or multiple environments): the only configuration deltas to copy are four items — `agent.id`, `agent.gameId/env`, `agent.localAddr`, `agent.httpAddr` (`configs/agent2.yaml` is not committed; operations generate it by copying `configs/agent.yaml` and changing those four items — the `agent2` service in `docker/docker-compose.deploy.yml` mounts that path). The upstream `server.addr` shares the same LB entry point.

## Environment Variables

This binary has no "env overrides YAML" layer — the table below lists runtime environment variables (not mapped to configuration keys):

| Variable                                                           | Default                    | Description                                                                |
| ------------------------------------------------------------------ | -------------------------- | -------------------------------------------------------------------------- |
| `CROUPIER_CONFIG_DIR`                                              | —                          | Configuration directory (takes priority over the `--config` path)          |
| `CROUPIER_AGENT_LOCAL_ADDR`                                        | empty → host:port fallback | Local TCP listener address (overrides the `agent.localAddr` fallback path) |
| `CROUPIER_AGENT_VERSION`                                           | —                          | Version override (for registration/reporting)                              |
| `CROUPIER_AGENTLOCAL_PROBE_INTERVAL`                               | 5s                         | Provider keepalive probe interval                                          |
| `CROUPIER_AGENTLOCAL_PRUNE_INTERVAL`                               | 30s                        | Session pruning interval                                                   |
| `CROUPIER_AGENTLOCAL_MAX_AGE`                                      | 2m                         | Maximum session lifetime                                                   |
| `CROUPIER_AGENTLOCAL_TASKRESULT_MAX_AGE`                           | 10m                        | Task-result retention                                                      |
| `CROUPIER_EXTENSION_SYNC_API` / `CROUPIER_EXTENSION_SYNC_INTERVAL` | 30s                        | Extension sync address and interval                                        |

## Verification

1. The agent log shows session establishment and successful registration
2. The Dashboard's "Ops Center → Node Maintenance" shows the agent online with correct labels
3. Trigger one function call for this agent from the Dashboard to verify the callback path (`httpAddr` correctness)
