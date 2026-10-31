# Agent Configuration: Field-by-Field Complete Reference (sidecar / devops / capture)

This document covers only the three existing entry points; for the fourth category, synthetics, see the Four-Category Agent Guide (this section is Chinese-only) for its responsibilities and unimplemented boundaries — it currently has no runnable configuration or release package. The fact that `env` may be empty for devops/capture below is the current state of the code; deployments must fill in gameId/env explicitly, and enforced validation is on the backlog.

Each of the three agent categories has a single YAML configuration file, and the loading chain is minimal: **read file → yaml.Unmarshal →
validate → run**. This document lists, field by field, the field names, types, defaults, meanings, and code read sites
(`file:line`, aligned with the current mainline; every field has a real read point in the code — no invented fields).
For selection see agent-comparison.md (this section is Chinese-only); for installation see
agent-install.md.

**Three common conventions (identical across all three agents):**

1. **No `${ENV}` expansion layer**: config loading is pure `yaml.Unmarshal`; environment variable placeholders
   are **not** expanded. Configuration files containing passwords write the values directly and keep `root:0600`,
   or are rendered from templates on the deployment side before delivery. **Secrets never enter the repository** —
   passwords are left empty in all examples and certificates are referenced by file path only.
2. **Key names are lowerCamelCase** (`serverAddr`, `heartbeatFile`); the sidecar additionally accepts
   legacy PascalCase/snake_case compatibility keys (see the table notes), but new configurations must always use the canonical
   keys — compatibility keys are not the documented standard.
3. **Outbound TLS in three stages**: with `enabled=false` all certificate fields are ignored (plaintext connection);
   with `enabled=true` the fields are passed to the TLS client configuration as given. Enabling it + `caFile` is recommended for all production.

Code path convention: below, `devops/config/config.go` means
`agents/devops/config/config.go`, and `root.go` means `cmd/sidecar-agent/root.go`.

---

## sidecar (croupier-agent)

Loading chain: `--config` file → `yaml.Unmarshal` (`root.go:482-485`, with legacy-key compatibility
`root.go:173-241`) → defaults (`applyAgentConfigDefaults`, `root.go:450-466`) →
strict validation (`root.go:486-490`: `agent.gameId` and `agent.env` must be non-empty, otherwise startup fails).

### `agent.*` (root.go:301-313)

| Field                   | Type               | Default                                               | Description                                                                                    | Read at                                                                             |
| ----------------------- | ------------------ | ----------------------------------------------------- | ---------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------- |
| `agent.id`              | string             | empty → auto `<hostname>-<unixts>`                    | Unique agent id used for registration                                                          | `root.go:558` → `resolveAgentID` (root.go:662-673)                                  |
| `agent.gameId`          | string             | empty (**required**)                                  | Game domain tag; sessions bind to a unique (gameId, env)                                       | validated `root.go:486-490`; reported with registration `root.go:574`               |
| `agent.env`             | string             | empty (**required**)                                  | Environment tag (dev/prod/…)                                                                   | same, `root.go:575`                                                                 |
| `agent.localAddr`       | string             | empty → falls back to `host`/`port`, then `0.0.0.0:0` | Local TCP gateway: entry point for SDK Provider registration functions                         | `root.go:545-556` (strips the `tcp://` prefix)                                      |
| `agent.labels`          | map<string,string> | empty                                                 | Ops-defined custom labels, merged into the automatic system labels (os/arch/hostname/ip etc.)  | `root.go:563-565` → registration payload                                            |
| `agent.invokeTimeoutMs` | int                | 0 → 15000                                             | Budget (ms) for agent→Provider synchronous calls; the per-request `timeoutMs` takes precedence | `root.go:572`; clamped `internal/agent/local_handler.go:110-121` (≤0→15s, >60s→60s) |
| `agent.transport`       | string             | empty                                                 | **DEAD**: nobody reads it after parsing (root.go:305, 342-344)                                 | —                                                                                   |
| `agent.httpAddr`        | string             | empty                                                 | **DEAD (behavior surface)**: only echoed into one startup log line (root.go:494)               | —                                                                                   |

### `server.*` (root.go:243-252) — upstream control plane

| Field                                      | Type   | Default                                              | Description                                                                                                                                  | Read at                                                                                      |
| ------------------------------------------ | ------ | ---------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| `server.addr`                              | string | empty = **no upstream connection** (local-only mode) | Croupier Server control plane `host:port` (convention: 19090)                                                                                | `root.go:568`; when empty it is skipped with a log, `internal/app/agent/upstream.go:258-261` |
| `server.transport`                         | string | empty → `tcp`                                        | Transport type; **only `tcp` is legal**, any other value panics                                                                              | `root.go:570` → `upstream.go:150-160`                                                        |
| `server.insecure`                          | bool   | false (explicit true = plaintext connection)         | When false, the certificate fields below build the TLS                                                                                       | `root.go:586-608`                                                                            |
| `server.caFile`                            | string | empty                                                | CA for verifying the server certificate; if set but the file is missing, a **dev CA is auto-generated** into the config directory's `certs/` | `root.go:586-596` → `internal/devcert/devcert.go:33-58`                                      |
| `server.tlsCertFile` / `server.tlsKeyFile` | string | empty                                                | mTLS client certificate/private key                                                                                                          | `root.go:600-601`                                                                            |
| `server.serverName`                        | string | empty                                                | TLS SNI/hostname verification override                                                                                                       | `root.go:603`                                                                                |
| `server.insecureSkipVerify`                | bool   | false                                                | Skip server certificate chain verification                                                                                                   | `root.go:604`                                                                                |

### `upstream.*` (root.go:358-363) — heartbeat and timeouts

| Field                        | Type          | Default                         | Description                                                                                                        | Read at                                                                |
| ---------------------------- | ------------- | ------------------------------- | ------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------- |
| `upstream.heartbeatInterval` | int (seconds) | 0 → core 3s                     | Control-plane heartbeat period                                                                                     | `root.go:582` → `upstream.go:201-204` → `core/register/register.go:39` |
| `upstream.timeout`           | int (ms)      | 0 → requests 10s / heartbeat 3s | Upstream dial and request timeout                                                                                  | `root.go:580-581` (only >0 takes effect)                               |
| `upstream.retryInterval`     | int           | 0                               | **DEAD**: nobody reads it after parsing; reconnect backoff is fixed 5s→60s×1.5 (`core/register/register.go:44-45`) | —                                                                      |
| `upstream.maxRetries`        | int           | 0                               | **DEAD**: same as above; the core failure threshold defaults to 2 (register.go:42)                                 | —                                                                      |

### `log.*` (= core/logx.LogConfig, core/logx/logging.go:18-27)

| Field                                                | Type             | Default                  | Description                                                                                                                                                                                                                                  | Read at                                                           |
| ---------------------------------------------------- | ---------------- | ------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------- |
| `log.level`                                          | string           | empty → `info`           | `debug\|info\|warn\|error`                                                                                                                                                                                                                   | `root.go:500-506`                                                 |
| `log.format`                                         | string           | empty → `console`        | `console\|json`                                                                                                                                                                                                                              | `root.go:507`                                                     |
| `log.output`                                         | string           | empty = stdout only      | **In this binary it is the rotating log file path** (the file argument passed to `SetupLoggerWithFile`), not a `stdout/stderr` enum; the console stream is controlled by the environment variables `LOG_OUTPUT`/`CROUPIER_LOG_OUTPUT=stderr` | `root.go:509`; lumberjack file write `core/logx/logging.go:43-51` |
| `log.maxSize` / `maxBackups` / `maxAge` / `compress` | int/int/int/bool | lumberjack default 100MB | Rotation parameters                                                                                                                                                                                                                          | `root.go:510-513`                                                 |
| `log.file`                                           | string           | empty                    | **DEAD (in this binary)**: root.go:506-514 never reads `Logging.File`                                                                                                                                                                        | —                                                                 |

### `tls.*` (root.go:125-132) — local gateway TLS (SDK→agent)

| Field                          | Type   | Default                                           | Description                                                                                                    | Read at                                                        |
| ------------------------------ | ------ | ------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------- |
| `tls.enabled`                  | bool   | false (forced false when the whole block is zero) | Encrypts the `agent.localAddr` listener; TLS sits below the frame protocol, the SDK wire protocol is unchanged | `root.go:625`                                                  |
| `tls.certFile` / `tls.keyFile` | string | empty                                             | Gateway server certificate; enabled and file missing = **startup error**                                       | `root.go:627-628` → `internal/transport/tcp/server.go:266-269` |
| `tls.caFile`                   | string | empty                                             | Setting it turns on **mTLS** (enforces client certificate verification)                                        | `root.go:629` → server.go:237-248                              |
| `tls.insecureSkipVerify`       | bool   | false                                             | Server-side downgrade: removes mTLS enforcement (ClientAuth=NoClientCert)                                      | `root.go:630` → server.go:250-252                              |
| `tls.serverName`               | string | empty                                             | **DEAD**: not passed into the server TLS construction (root.go:626-631)                                        | —                                                              |

### `outboundTLS.*` (same shape as root.go:125-132) — dial-out Provider TLS

| Field                                                                       | Description                                                                                                            | Read at                                                       |
| --------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------- |
| `enabled` + `certFile`/`keyFile`/`caFile`/`serverName`/`insecureSkipVerify` | Encrypts the agent→Provider dial-out connections (the fallback dial path when no existing Provider session is present) | `root.go:610-620` → `internal/agent/local_handler.go:176-179` |

### `ops.*` (root.go:93-97) — only 3 keys reachable

| Field                 | Type        | Default                              | Description                                                                                                                                                                                                                 | Read at                                                      |
| --------------------- | ----------- | ------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------ |
| `ops.enabled`         | bool        | false (must be turned on explicitly) | Enables the ops module (system info/metrics/processes/timed jobs read-only RPC surface)                                                                                                                                     | `root.go:639`                                                |
| `ops.metricsInterval` | Go duration | empty → 30s                          | Metrics collection/report period; **parse failures are silently ignored** (root.go:643-647). Note that `OpsConfig.Validate()`'s [3s,24h] clamp is **not called by this binary**; out-of-range values pass through unchanged | `root.go:643-647` → `app.go:432-442` → `upstream.go:583-586` |
| `ops.metricsEnabled`  | bool        | false (overrides the default true)   | Master switch for periodic metrics reporting                                                                                                                                                                                | `root.go:642`; loop gate `upstream.go:277-283`               |

**Honest disclosure (DEAD surface)**: the rich fields of agentcore's `OpsConfig` —
`allowRestart`/`allowExec`/`managedProcesses`/`execAllowedCommands`/`execTimeout`/
`supervisorLog.*`/`snapshotDir` (`internal/app/agent/ops_config.go:11-55`) —
are **not parsed by this binary** (the sidecar's local OpsConfig has only the 3 keys in the table above; unknown keys are silently dropped).
In this binary, the restart/stop-start/execute-command RPCs always refuse (`ops_server.go:137,185,218,290`).
The commented-out `managedProcesses:`/`supervisorLog:` blocks in `configs/agent.yaml` have no effect on this
binary.

### Environment variables (runtime behavior, not a YAML override layer)

| Variable                                                                                      | Default                                                           | Purpose                                                                                                        | Read at                                                       |
| --------------------------------------------------------------------------------------------- | ----------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------- |
| `CROUPIER_CONFIG_DIR`                                                                         | `<exe dir>/etc` if it exists, otherwise `/etc/croupier` (service) | Config directory for providers.yaml and service installation                                                   | `internal/app/agent/app.go:112`; `service.go:519`             |
| `CROUPIER_AGENTLOCAL_PROBE_INTERVAL`                                                          | `5s` (≤0 disables keepalive probing)                              | Provider keepalive probe period                                                                                | `provider_keepalive.go:25-31`                                 |
| `CROUPIER_AGENTLOCAL_PRUNE_INTERVAL` / `_MAX_AGE` / `_TASKRESULT_MAX_AGE`                     | `30s` / `2m` / `10m`                                              | Local storage pruning cadence                                                                                  | `app.go:261-263`                                              |
| `CROUPIER_AGENTLOCAL_PROVIDER_HEARTBEAT`                                                      | `30s`                                                             | Static Provider session heartbeat                                                                              | `internal/app/agent/provider.go:314`                          |
| `CROUPIER_EXTENSION_SYNC_API` / `_INTERVAL` / `_PROVIDERS_ONLY` / `_PROVIDER_OVERRIDE_STATIC` | off / `30s`                                                       | Extension sync pull surface                                                                                    | `app.go:297-310`; `provider.go:49-50,304-310`                 |
| `CROUPIER_EXTENSION_SYNC_TOKEN`                                                               | empty (server rejects with 401)                                   | ARCH-021 restricted machine JWT sent as `X-Croupier-Agent-Token`; issue via `croupier-server token agent-sync` | `app.go:startExtensionSync`; `extension_sync_puller.go:93-95` |
| `LOG_OUTPUT` / `CROUPIER_LOG_OUTPUT`                                                          | stdout; `stderr` redirects the console stream                     | Where console logs go                                                                                          | `core/logx/logging.go:35-40`                                  |

### Command line and subcommands (root.go:64-66, 37)

- `--config/-f` (default `etc/agent.yaml`, **required**, error if missing), `--mode/-m`,
  `--debug/-d` (both are only echoed into the startup log, **changing no behavior**, root.go:530).
- Subcommands: `agent` (equivalent to the root command), `version`, `service install|uninstall|start|stop|restart|status|run`
  (kardianos service management, service.go:37-94).

### sidecar Minimal Runnable Config

```yaml
agent:
  gameId: default # required
  env: dev # required
```

`server.addr` omitted = no upstream connection, local-only operation (upstream.go:258-261); the gateway binds
the ephemeral port `0.0.0.0:0` (add `port: 19091` for a fixed one).

### sidecar Recommended Production Config

```yaml
host: 0.0.0.0
port: 19091

server:
  addr: "croupier-server.internal:19090"
  transport: tcp
  insecure: false
  tlsCertFile: /etc/croupier/certs/agent.crt # path reference only; values never enter the repo
  tlsKeyFile: /etc/croupier/certs/agent.key
  caFile: /etc/croupier/certs/ca.crt
  serverName: croupier-server.internal
  insecureSkipVerify: false

agent:
  id: game-a-node-01
  gameId: game-a
  env: prod
  localAddr: "0.0.0.0:19091"
  labels: { tier: game-logic }
  invokeTimeoutMs: 15000

upstream:
  heartbeatInterval: 10
  timeout: 10000

log:
  level: info
  format: json
  output: /var/log/croupier/agent.log # note: in this binary, output is the log file path
  maxSize: 100
  maxBackups: 5
  maxAge: 14
  compress: true

ops:
  enabled: true
  metricsInterval: 30s
  metricsEnabled: true
```

---

## devops-agent

Loading chain: `agents/devops/config/config.go:75-88` (ReadFile → Unmarshal →
Validate), **no compatibility keys, no env expansion layer**; validation failure exits with 2
(`cmd/devops-agent/main.go:29-34`).

### `agent.*` (devops/config/config.go:26-37)

| Field                 | Type   | Default                      | Description                                                                                                    | Read at                                                           |
| --------------------- | ------ | ---------------------------- | -------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------- |
| `agent.id`            | string | empty (**required**)         | Unique id for registration                                                                                     | validated `config.go:92-94`; registration payload `runner.go:109` |
| `agent.serverAddr`    | string | empty (**required**)         | Upstream control plane `host:port` (`tcp://` prefix optional)                                                  | validated `config.go:95-97`; `runner.go:286` (controlconn.Dial)   |
| `agent.gameId`        | string | empty (**required**)         | Game domain tag                                                                                                | validated `config.go:98-100`; `runner.go:110`                     |
| `agent.env`           | string | empty                        | Environment tag                                                                                                | `runner.go:111`                                                   |
| `agent.heartbeatFile` | string | empty (= heartbeat file off) | Supervisable heartbeat mtime stamp file (the supervisor samples last_heartbeat_unix); interval defaults to 10s | `main.go:37`; `core/supervisable/options.go:29-31`                |

### `outboundTLS.*` (devops/config/config.go:16-23)

| Field                  | Type   | Default | Description                                                                  | Read at                                          |
| ---------------------- | ------ | ------- | ---------------------------------------------------------------------------- | ------------------------------------------------ |
| `enabled`              | bool   | false   | false = plaintext connection to upstream                                     | `runner.go:124-126` (tlsFromCfg; nil → Insecure) |
| `certFile` / `keyFile` | string | empty   | mTLS client certificate/private key                                          | `runner.go:128-129`                              |
| `caFile`               | string | empty   | CA for verifying the server certificate (strongly recommended in production) | `runner.go:130`                                  |
| `serverName`           | string | empty   | TLS SNI override                                                             | `runner.go:131`                                  |
| `insecureSkipVerify`   | bool   | false   | Skip server certificate verification                                         | `runner.go:132`                                  |

(`controlconn.Dial` side constants: connect 5s / receive 30s / send 10s, `agents/controlconn/controlconn.go:42-44`, not configurable.)

### `devops.*` (devops/config/config.go:52-65)

| Field               | Type                         | Default                | Description                                                                                                                        | Read at                                                                                                     |
| ------------------- | ---------------------------- | ---------------------- | ---------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| `devops.enabled`    | bool                         | **false**              | false = registers online only and takes no sessions; true = enables the session surface (an allowlist must be configured first)    | `runner.go:68`                                                                                              |
| `devops.allowlist`  | `[]{host, ports, protocols}` | empty                  | **Host allowlist (hard boundary)**: required and non-empty when enabled=true (config.go:102-119); host non-empty, port ∈ (0,65535] | validated `config.go:102-119`; matching `agents/devops/session/allowlist.go:23-49`; assembly `runner.go:90` |
| `devops.sessionDir` | string                       | `data/devops/sessions` | Session audit record directory (session recorder; assembly failure = exit)                                                         | default `config.go:131-136`; `runner.go:83`                                                                 |
| `devops.execlogDir` | string                       | `data/devops/execlog`  | Execution audit log directory (execlog lumberjack, default 100MB × 10 backups × 30 days)                                           | default `config.go:138-143`; `runner.go:76`                                                                 |
| `devops.guacdAddr`  | string                       | empty                  | **DEAD (reserved)**: the declaring comment says "not wired in the skeleton batch"; the code never reads it (config.go:62-64)       | —                                                                                                           |

The `AllowHost` struct (config.go:42-48): `host` string, `ports` []int,
`protocols` []string.

### Command line (cmd/devops-agent/main.go)

- `-config` (default `configs/devops-agent.yaml`), `-version`.
- Exit code contract: 2 = configuration error; 0 = normal stop; 1 = runtime error
  (main.go:29-34 + supervisable).

### devops Minimal Runnable Config

```yaml
agent:
  id: devops-bastion-01
  serverAddr: 10.0.0.5:19090
  gameId: demo
```

(Registered online with a visible heartbeat; `devops.enabled` defaults to false, so no remote sessions are accepted.)

### devops Recommended Production Config

```yaml
agent:
  id: devops-bastion-01
  serverAddr: 10.0.0.5:19090
  gameId: demo
  env: prod
  heartbeatFile: /var/lib/croupier/devops/agent-heartbeat

outboundTLS:
  enabled: true
  caFile: /etc/croupier/certs/ca.pem
  certFile: /etc/croupier/certs/devops.pem
  keyFile: /etc/croupier/certs/devops-key.pem
  serverName: croupier-control.internal

devops:
  enabled: false # always false until the session channel is wired; enable per allowlist after wiring
  allowlist:
    - host: 10.0.0.21
      ports: [22]
      protocols: [ssh]
  sessionDir: /var/lib/croupier/devops/sessions
  execlogDir: /var/lib/croupier/devops/execlog
```

**Known boundary**: with `devops.enabled=true`, session establishment gets a
`session channel ... not implemented yet (skeleton)` rejection
(`agents/devops/session/session.go:71-73`) — the protocol channel is wired in a separate batch; the meaning of the
current installation = joining the registration/heartbeat/audit supervision surface.

---

## capture-agent

Loading chain: `agents/capture/config/config.go:90-103` (pure Unmarshal, no env expansion) →
`Validate()` (config.go:106-135); validation failure exits with 2
(`cmd/capture-agent/main.go:29-34`).

### `agent.*` (capture/config/config.go:28-39)

| Field                 | Type   | Default              | Description                                                   | Read at                                             |
| --------------------- | ------ | -------------------- | ------------------------------------------------------------- | --------------------------------------------------- |
| `agent.id`            | string | empty (**required**) | Unique id for registration (gates locate session pulls by it) | validated `config.go:107-109`; `runner.go:139` etc. |
| `agent.serverAddr`    | string | empty (**required**) | Upstream control plane `host:port`                            | validated `config.go:110-112`; `runner.go:286`      |
| `agent.gameId`        | string | empty (**required**) | Game domain tag (gates/alerts are scoped by it)               | validated `config.go:113-115`; `runner.go:296`      |
| `agent.env`           | string | empty                | Environment tag                                               | `runner.go:297`                                     |
| `agent.heartbeatFile` | string | empty (= off)        | Same as devops (supervisable heartbeat file)                  | `main.go:37`                                        |

### `outboundTLS.*` (capture/config/config.go:18-25)

The six fields are the same as devops's in both name and meaning; read at `runner.go:563-576` (tlsFromCfg) →
`agents/controlconn/controlconn.go:36-60`. No Validate constraints.

### `capture.*` (capture/config/config.go:69-80)

| Field                              | Type   | Default                              | Description                                                                                                                                                                                                                                                       | Read at                                                      |
| ---------------------------------- | ------ | ------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------ |
| `capture.enabled`                  | bool   | **false (deliberately default-off)** | CDC master switch; false = registers online only and consumes nothing                                                                                                                                                                                             | validated `config.go:116`; `runner.go:304`                   |
| `capture.bookmarkDir`              | string | empty → `data/capture/bookmarks`     | Bookmark file directory (one file per source, `mysql.json`, atomic write via tmp+rename)                                                                                                                                                                          | `config.go:138-144` (BookmarkPath); `runner.go:122`          |
| `capture.bookmarkFlushIntervalSec` | int    | 0 → 5s                               | Periodic bookmark flush interval (≤0 always falls back to 5s); also flushed unconditionally on ctx cancellation                                                                                                                                                   | `config.go:147-152` (FlushInterval); `runner.go:361,498-510` |
| `capture.sourceType`               | string | empty                                | Data source type (`mysql` \| `postgres` \| `mongodb`); required when enabled=true. mongodb requires a source replica set (change streams are a hard prerequisite; standalone is rejected at `Open`), bookmark file `mongodb.json`, allowlist form `db.collection` | validated `config.go:175-182`                                |
| `capture.dsnEnv`                   | string | empty                                | Name of the environment variable holding the connection string (the plaintext DSN is taken only from an environment variable — never into the config file, never into logs); required when enabled=true                                                           | validated `config.go:175-182`                                |
| `capture.options`                  | map    | empty                                | Non-secret connection parameters (serverId/flavor/tables/slotName/publication/database…); `options.tables` is required (comma-separated `db.table`/`db.collection` allowlist)                                                                                     | validated `config.go:175-182`                                |
| `capture.mysql`                    | object | empty                                | **Legacy compatibility, do not use in new deployments**: tolerated only by the compat parsing code; no longer written into any example/documentation                                                                                                              | `runner.go:150,521`                                          |
| `capture.gates`                    | object | empty = capture only, no judgments   | Gate-layer rules; **file rules are only a fallback** — once the panel has published `capture.gates`, they are fully overridden                                                                                                                                    | validated `config.go:125-133`; hot push `runner.go:212-244`  |

### `capture.mysql.*` (agents/capture/source/mysql.go:16-24)

| Field               | Type             | Default       | Description                                                                                                                                                  | Read at                                                                           |
| ------------------- | ---------------- | ------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------- |
| `host` / `port`     | string / uint16  | empty / 0     | Source database address                                                                                                                                      | `mysql.go:78-79`; GTID probe `mysql_probe.go:16`; readiness probe `runner.go:521` |
| `user` / `password` | string           | empty / empty | Replication account (**plaintext field**: config file root:0600; least privilege = REPLICATION SLAVE + REPLICATION CLIENT + SELECT on the subscribed tables) | `mysql.go:80-81`                                                                  |
| `serverId`          | uint32           | 0             | binlog subscription slave server_id; **must not** clash with the source instance or other subscribers (a conflict = the server breaks the stream)            | `mysql.go:77`                                                                     |
| `tables`            | map<string,bool> | empty         | **required and non-empty when enabled=true** (config.go:120-122); subscription allowlist, keys `"db.table"` matched exactly, a `false` value = exclude       | `mysql_pump.go:77-80` (MatchesTable → source/event.go:58-64)                      |

**Source database prerequisites** (source-side ops items, not part of this configuration): `binlog_format=ROW`,
`binlog_row_image=FULL`, `gtid_mode=ON` (probe-verified, `mysql_probe.go:29-31`).

### `capture.gates.*` (capture/config/config.go:45-66)

Rule templates are code, rule bodies are pure data (no DSL). Everything is validated fully at startup; **`event.ops`
is the only section not validated inside Validate** — an invalid value makes buildGateSet in `runner.New`
degrade to pass-all with a warning `capture gates disabled, invalid file rules`
(`runner.go:152-157`), rather than refusing to start.

#### `event.ops` (closed set insert/update/delete)

| Value                      | Description                                                                                           | Read at                                                |
| -------------------------- | ----------------------------------------------------------------------------------------------------- | ------------------------------------------------------ |
| `[]` (omitted)             | Allow all events (zero-cost pass-through)                                                             | `agents/capture/filter/event.go:36-38`                 |
| `insert`/`update`/`delete` | Allowlist; values outside the closed set produce a build error `event filter op %q not in closed set` | `filter/event.go:25-28`; evaluated `runner.go:406-415` |

#### `reconcile.rules[*]` (reconciliation gate; agents/capture/gate/reconcile.go:49-59)

| Field                       | Type     | Default        | Constraint                                                                                                                                                       | Read at                                                     |
| --------------------------- | -------- | -------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------- |
| `name`                      | string   | —              | Required                                                                                                                                                         | `reconcile.go:63-65`                                        |
| `itemTable` / `ledgerTable` | string   | —              | Must be a two-segment `"db.table"` (the table name contains no dot); the two must differ; **the same table must not be reused across reconcile rules**           | `reconcile.go:66-74`; uniqueness `reconcile.go:110-124`     |
| `itemKey` / `ledgerKey`     | string   | —              | Required; key form: an integer string = positional row-image index (the C1 row image is a positional-array JSON), a field name = object key (forward compatible) | `reconcile.go:75-77`; key extraction `reconcile.go:288-310` |
| `windowSec`                 | int      | 0 → **300**    | ≥0; pairing window (design default ±5 minutes)                                                                                                                   | `reconcile.go:78-80`; `Window()` :93-98                     |
| `severity`                  | string   | empty → `warn` | Closed set `warn\|critical`                                                                                                                                      | `reconcile.go:81-85`; default :380-383                      |
| `exemptKeys`                | []string | `[]`           | Exact-match key exemptions (batch pipeline flows etc.)                                                                                                           | `reconcile.go:198-204,284-286`                              |
| `maxPending`                | int      | 0 → **10000**  | ≥0; per-rule in-window entry cap (ring FIFO: evicted debit = alert, credit = silent)                                                                             | `reconcile.go:86-88`; `cap()` :101-106                      |

Hit ID: `reconcile.missing_ledger` (reconcile.go:27,389).

#### `cep.offHours[*]` (time-window gate; agents/capture/gate/cep.go:36-43)

| Field         | Type                  | Default        | Constraint                                                                                                                  | Read at                                              |
| ------------- | --------------------- | -------------- | --------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------- |
| `name`        | string                | —              | Required                                                                                                                    | `cep.go:83-85`                                       |
| `table`       | string                | —              | Two-segment `"db.table"`                                                                                                    | `cep.go:86-88`                                       |
| `ops`         | []string              | `[]` = all     | Closed three-value set                                                                                                      | `cep.go:89-91` (checkOps :459-468); runtime :446-456 |
| `windows[]`   | `[]{startMin,endMin}` | —              | **Required and non-empty**; startMin ∈ [0,1439], endMin ∈ [1,1440]; left-closed right-open; `start>end` = crossing midnight | `cep.go:92-98`; midnight crossing :481-494           |
| `tzOffsetMin` | int                   | 0 = UTC        | ∈ [-720, 840] (UTC+8 = 480)                                                                                                 | `cep.go:99-101`                                      |
| `severity`    | string                | empty → `warn` | Closed set                                                                                                                  | `cep.go:102` (:471-478)                              |

Hit ID: `cep.off_hours` (cep.go:15); judged by event time (the binlog truth), falling back to the wall clock when there is no timestamp (cep.go:293-296).

#### `cep.burst[*]` (quantity-spike gate; cep.go:50-60)

| Field                    | Type    | Default                 | Constraint                                                                                         | Read at                            |
| ------------------------ | ------- | ----------------------- | -------------------------------------------------------------------------------------------------- | ---------------------------------- |
| `name` / `table` / `ops` | —       | —                       | Same as offHours                                                                                   | `cep.go:108-116`                   |
| `deltaField`             | string  | —                       | Required; numeric delta field (key form same as itemKey); missing/non-numeric → skip and count     | `cep.go:117-119`; :313-315         |
| `threshold`              | float64 | —                       | >0; single = per-event \|delta\| threshold; window = sliding-window cumulative \|delta\| threshold | `cep.go:120-122`                   |
| `mode`                   | string  | —                       | Closed set `single\|window`; window requires windowSec>0                                           | `cep.go:123-130`                   |
| `windowSec`              | int     | — (required for window) | Sliding window seconds                                                                             | `cep.go:125-127`                   |
| `maxEvents`              | int     | 0 → **10000**           | ≥0; cumulative bucket ring capacity (FIFO evicts oldest)                                           | `cep.go:131-133`; `capOf` :409-414 |
| `severity`               | string  | empty → `warn`          | Closed set                                                                                         | `cep.go:134`                       |

Hit ID: `cep.burst_delta` (cep.go:16); after a window hit the bucket is cleared to prevent alert spam (cep.go:332).

#### `cep.probing[*]` (continuous-probing gate; cep.go:67-78)

| Field                    | Type    | Default        | Constraint                                                                                 | Read at                    |
| ------------------------ | ------- | -------------- | ------------------------------------------------------------------------------------------ | -------------------------- |
| `name` / `table` / `ops` | —       | —              | Same as offHours                                                                           | `cep.go:140-148`           |
| `keyField`               | string  | —              | Required; row key (C1 degradation: key = row key rather than account)                      | `cep.go:149-151`; :349-351 |
| `deltaField`             | string  | —              | Required; numeric delta field                                                              | `cep.go:149-151`           |
| `smallBelow`             | float64 | —              | >0; a single \|delta\| below this = small change; ≥ this resets that key's sequence        | `cep.go:152-154`; :353-359 |
| `count`                  | int     | —              | ≥2; fires on the Nth small change within the window (at most one alert per key per window) | `cep.go:155-157`; :372-374 |
| `windowSec`              | int     | —              | >0                                                                                         | `cep.go:158-160`           |
| `maxTracked`             | int     | 0 → **10000**  | ≥0; tracked-key cap (FIFO deterministic eviction)                                          | `cep.go:161-163`; :361     |
| `severity`               | string  | empty → `warn` | Closed set                                                                                 | `cep.go:163`               |

Hit ID: `cep.probing` (cep.go:17).

#### `lineage.rules[*]` (lineage gate; agents/capture/gate/lineage.go:19-25)

| Field                    | Type     | Default        | Constraint                                                                         | Read at            |
| ------------------------ | -------- | -------------- | ---------------------------------------------------------------------------------- | ------------------ |
| `name` / `table` / `ops` | —        | —              | Same as above (two-segment table)                                                  | `lineage.go:30-38` |
| `allowAccounts`          | []string | —              | **Required and non-empty** (must at least include the application's write account) | `lineage.go:39-41` |
| `severity`               | string   | empty → `warn` | Closed set                                                                         | `lineage.go:42`    |

Hit ID: `lineage.whitelist_violation` (lineage.go:10). **Degraded state**: a binlog
ROW without a session account → every hit is skipped and counted (`Degraded()`, lineage.go:89-92);
registration label `captureLineage=degraded` (`runner.go:535-548`); the panel renders a degraded badge.

### Hot Push and Bookmarks (ops essentials)

- **Gate rule hot push**: the panel publishes `capture.gates` (config_versions) → the agent polls every
  **30s** (`core/configsync/configsync.go:15`, **not configurable via YAML**) →
  full validation (the same implementation as the panel's dry-run) → atomic pointer swap; on failure the old version is kept, the cursor does not advance,
  and the next round retries (`runner.go:212-244`; `core/configsync/configsync.go:19-56`).
  File rules apply only when nothing has ever been published (runner.go:159-167).
- **Bookmarks**: `<bookmarkDir>/mysql.json` (GTID set + updatedAt). Resume = `StartSyncGTID`
  from the bookmark; no bookmark = probe `gtid_executed` and start from now (no history replay).
  **A corrupt bookmark file = refuse to start** (exit 1, manual intervention, no silent rescan;
  `runner.go:324-350`; `source/source.go:50-53`).

### capture Minimal Runnable Config

```yaml
agent:
  id: capture-01
  serverAddr: 127.0.0.1:19090
  gameId: demo
```

(`capture.enabled` defaults to false: registered online, heartbeat visible, never touches the database;
`agents/capture/config/config_test.go:73-140` proves the same shape.)

### capture Recommended Production Config

```yaml
agent:
  id: capture-01
  serverAddr: 10.0.0.5:19090
  gameId: demo
  env: prod
  heartbeatFile: /var/lib/croupier/capture/agent-heartbeat

outboundTLS:
  enabled: true
  caFile: /etc/croupier/certs/ca.pem
  certFile: /etc/croupier/certs/capture.pem
  keyFile: /etc/croupier/certs/capture-key.pem
  serverName: croupier-control.internal

capture:
  enabled: true
  bookmarkDir: /var/lib/croupier/capture/bookmarks
  bookmarkFlushIntervalSec: 5
  mysql:
    host: 10.0.1.20
    port: 3306
    user: repl
    password: "" # plaintext field: this file is root:0600; written via deployment-side template rendering
    serverId: 4242
    tables:
      game.items: true
      game.player_wallet: true
  gates:
    event:
      ops: [insert, update, delete]
    reconcile:
      rules:
        - name: item-ledger
          itemTable: game.items
          ledgerTable: game.ledger
          itemKey: "1"
          ledgerKey: "0"
          windowSec: 300
          severity: critical
    cep:
      burst:
        - name: wallet-drain
          table: game.player_wallet
          deltaField: "2"
          threshold: 100000
          mode: single
          severity: critical
    lineage:
      rules:
        - name: items-accounts
          table: game.items
          allowAccounts: ["game-app"] # in production, prefer panel hot push over file rules
          severity: critical
```

Production recommendation: publish rule bodies via the panel's "Risk Control → Rules" (config_versions hot push +
version rollback); the file `gates` serves only as a fallback skeleton; put `mysql.password` in a root:0600 configuration
file or render it from deployment-side templates — never into the repository or logs.

---

## Appendix: Quick Three-Agent Comparison (configuration surface)

|                          | sidecar                                                                                               | devops                            | capture                          |
| ------------------------ | ----------------------------------------------------------------------------------------------------- | --------------------------------- | -------------------------------- |
| Config file              | `agent.yaml`                                                                                          | `devops-agent.yaml`               | `capture-agent.yaml`             |
| Required                 | `agent.gameId`+`agent.env`                                                                            | `agent.id`+`serverAddr`+`gameId`  | same as devops                   |
| Legacy key compatibility | yes (PascalCase/snake_case)                                                                           | none                              | none                             |
| Default posture          | starts even when the upstream is unreachable (local-only)                                             | registers only, takes no sessions | registers only, consumes nothing |
| Dead fields              | `name`/`agent.transport`/`httpAddr`/`upstream.retryInterval`/`maxRetries`/`log.file`/`tls.serverName` | `devops.guacdAddr`                | none                             |
