---
title: Configuration Switch Effects
icon: toggle
order: 20
category:
  - Operations Manual
tag:
  - Configuration
  - Evidence
---

# Configuration Switch Effects

This page enumerates every configuration switch in the repository that has a
**visible effect when toggled** — the DOC-001 evidence inventory. For each
switch the table records the config key, the default state, and what changes
between off and on. The GUI-visible feature flags carry generated off/on
screenshots + GIF evidence (see below); the server/agent switches are
config-level (their effect is described in the row) since they have no
operator-console visual change.

Conventions:

- `off` / `on` describe the observable behavior change, not the raw byte.
- Defaults marked `(example)` come from `configs/*.yaml`; `(code)` marks a
  code-level default (Go zero value unless noted).
- `featureFlags.*` flags are **absent = enabled**; only an explicit `false`
  disables a domain. The control plane (API routes + dashboard menus) is
  trimmed by these flags; data-plane components are not governed by them.

## Feature Flags (dashboard menu + API route domains)

These are the most user-visible switches: each toggles a product domain's
menu entries in the operator console and the registration of its API routes.

| Key                       | Default          | Off effect                                                                                  | On effect                                                |
| ------------------------- | ---------------- | ------------------------------------------------------------------------------------------- | -------------------------------------------------------- |
| `featureFlags.dev`        | enabled (absent) | `dev` menu (Dev) hidden; routes not registered                                              | `dev` domain (defect tracking / task scheduling) visible |
| `featureFlags.support`    | enabled (absent) | `support` menu (Support) hidden                                                             | `support` domain (tickets / FAQ / feedback) visible      |
| `featureFlags.analytics`  | enabled (absent) | `analytics` menu (Analytics) hidden                                                         | `analytics` domain (data analysis) visible               |
| `featureFlags.ops`        | enabled (absent) | `ops` menus (Operations Center + Risk Control) hidden                                       | `ops` domain (operations center) visible                 |
| `featureFlags.extensions` | enabled (absent) | `extensions` routes unregistered; **no menu change** (all extension pages are `hideInMenu`) | `extensions` domain (extension center) routes registered |

### Feature flag evidence (off → on)

Each flag was toggled on a throwaway local server and the console sidebar
captured in both states by
`scripts/docs/config-switch-evidence.sh` (headless Chromium login →
screenshot → labeled comparison + crossfade GIF). Artifacts live under
`docs/public/evidence/config-effects/<flag>/`.

**`featureFlags.ops`** — Operations Center and Risk Control disappear together
(both route groups are gated by the same `canOpsRead` access key):

![featureFlags.ops off vs on](/evidence/config-effects/featureFlags.ops/ops-compare.png)

[Toggle GIF](/evidence/config-effects/featureFlags.ops/ops.gif) ·
[off](/evidence/config-effects/featureFlags.ops/ops-off.png) ·
[on](/evidence/config-effects/featureFlags.ops/ops-on.png)

**`featureFlags.support`** — Support menu (客服系统):

![featureFlags.support off vs on](/evidence/config-effects/featureFlags.support/support-compare.png)

[Toggle GIF](/evidence/config-effects/featureFlags.support/support.gif) ·
[off](/evidence/config-effects/featureFlags.support/support-off.png) ·
[on](/evidence/config-effects/featureFlags.support/support-on.png)

**`featureFlags.analytics`** — Analytics menu (分析中心):

![featureFlags.analytics off vs on](/evidence/config-effects/featureFlags.analytics/analytics-compare.png)

[Toggle GIF](/evidence/config-effects/featureFlags.analytics/analytics.gif) ·
[off](/evidence/config-effects/featureFlags.analytics/analytics-off.png) ·
[on](/evidence/config-effects/featureFlags.analytics/analytics-on.png)

**`featureFlags.dev`** — Dev menu (研发):

![featureFlags.dev off vs on](/evidence/config-effects/featureFlags.dev/dev-compare.png)

[Toggle GIF](/evidence/config-effects/featureFlags.dev/dev.gif) ·
[off](/evidence/config-effects/featureFlags.dev/dev-off.png) ·
[on](/evidence/config-effects/featureFlags.dev/dev-on.png)

## Server Configuration Switches

| Key                                              | Default           | Off effect                                                                        | On effect                                                                      |
| ------------------------------------------------ | ----------------- | --------------------------------------------------------------------------------- | ------------------------------------------------------------------------------ |
| `database.multiGame`                             | `false` (code)    | Single database; row-level isolation via `game_id`                                | Each `(game_id, env)` gets its own physical database                           |
| `cache.enabled`                                  | `false` (code)    | Cache disabled; direct store access                                               | Cache layer active                                                             |
| `cluster.enabled`                                | `false` (code)    | Cluster features off; standalone operation                                        | Cluster membership / owner forwarding active                                   |
| `cluster.insecureSkipTls`                        | `false` (code)    | Peer TLS verified                                                                 | Peer TLS verification skipped                                                  |
| `cluster.allowStandaloneDegrade`                 | `false` (code)    | Cluster required; standalone refused                                              | Standalone degrade allowed when cluster unavailable                            |
| `cluster.allowPlaintextInterconnect`             | `false` (code)    | Interconnect TLS fail-closed (refuses boot if certs missing)                      | Explicitly permit plaintext interconnect (trusted isolated intranet; degraded) |
| `cluster.interconnectTls.enabled`                | `false` (code)    | Plaintext only when `allowPlaintextInterconnect: true`; otherwise boot is refused | mTLS interconnect (peer-certificate identity)                                  |
| `cluster.interconnectTls.insecureSkipVerify`     | `false` (code)    | Peer cert verified                                                                | Peer cert verification skipped                                                 |
| `agentDispatch.enableHA`                         | `false` (code)    | HA features off                                                                   | HA multi-instance dispatch features active                                     |
| `agentDispatch.reconnection.enableAutoReconnect` | `false` (code)    | No auto-reconnect                                                                 | Agent auto-reconnect on drop                                                   |
| `agentDispatch.toAgentTLS.enabled`               | `false` (code)    | Plaintext to agent                                                                | TLS to agent                                                                   |
| `agentDispatch.toAgentTLS.insecureSkipVerify`    | `false` (code)    | Agent cert verified                                                               | Agent cert verification skipped                                                |
| `approval.allowSelfApprove`                      | `false` (example) | Self-approval rejected                                                            | Operator may approve their own request                                         |
| `executionLog.enabled`                           | `false` (code)    | Execution log not written                                                         | Execution log written per execution                                            |
| `descriptors.schemaDiffWarn`                     | `false` (code)    | Schema drift silent                                                               | Schema drift logged as warning                                                 |
| `storage.forcePathStyle`                         | `false` (code)    | Virtual-host style (S3)                                                           | Path-style object URLs                                                         |
| `metrics.perFunction`                            | `true` (example)  | Aggregated metrics only                                                           | Per-function metrics emitted                                                   |
| `metrics.perGameDenies`                          | `false` (example) | Per-game metrics not denied                                                       | Per-game metrics denied (off by scope)                                         |
| `serverStatus.enabled`                           | `false` (code)    | Maintenance gate bypassed                                                         | Agent-probe maintenance window gate active                                     |
| `herald.enabled`                                 | `false` (code)    | Pure croupier alert page                                                          | Alerts delivered via herald channels                                           |
| `provision.enabled`                              | `false` (code)    | Provisioning off                                                                  | Provision / destroy workflows active                                           |
| `backup.schedule.enabled`                        | `false` (code)    | No scheduled backups                                                              | Scheduled backups for registered sources                                       |
| `vault.enabled`                                  | `false` (code)    | Secrets inline in config                                                          | Sensitive credentials via vault provider                                       |

## Telemetry Switches

| Key                            | Default | Off effect                   | On effect                         |
| ------------------------------ | ------- | ---------------------------- | --------------------------------- |
| `telemetry.enabled`            | `false` | OTel off; zero-overhead path | OTel SDK active                   |
| `telemetry.enable_tracing`     | `false` | No tracing export            | Trace spans exported              |
| `telemetry.enable_metrics`     | `false` | No metric export             | Metrics exported                  |
| `telemetry.use_tls`            | `false` | Plaintext OTLP               | TLS to collector                  |
| `telemetry.prometheus.enabled` | `false` | Prometheus scrape off        | `/metrics` scrape endpoint active |

## Authentication Provider Switches

Each provider's `enabled` gate adds/removes that identity source from the
login flow. `register.enabled` gates self-registration.

| Key                                      | Default        | Off effect                 | On effect                             |
| ---------------------------------------- | -------------- | -------------------------- | ------------------------------------- |
| `auth.providers.local.enabled`           | `false` (code) | Local login disabled       | Local username/password login enabled |
| `auth.providers.ldap.enabled`            | `false` (code) | LDAP login disabled        | LDAP directory login enabled          |
| `auth.providers.ldap.startTls`           | `false` (code) | Plain LDAP                 | STARTTLS upgrade                      |
| `auth.providers.ldap.insecureSkipVerify` | `false` (code) | LDAP cert verified         | LDAP cert verification skipped        |
| `auth.providers.oidc.enabled`            | `false` (code) | OIDC login disabled        | OIDC single sign-on enabled           |
| `auth.providers.github.enabled`          | `false` (code) | GitHub login disabled      | GitHub OAuth login enabled            |
| `auth.providers.wechat.enabled`          | `false` (code) | WeChat login disabled      | WeChat OAuth login enabled            |
| `auth.providers.genericoauth.enabled`    | `false` (code) | Generic OAuth disabled     | Generic OAuth provider enabled        |
| `auth.providers.register.enabled`        | `false` (code) | Self-registration disabled | Self-registration enabled             |

## Agent-Side Switches

These live in the agent / sidecar / capture / devops / synthetics configs
(`configs/*-agent.example.yaml`, `internal/app/agent/ops_config.go`).

| Key                            | Default           | Off effect                     | On effect                           |
| ------------------------------ | ----------------- | ------------------------------ | ----------------------------------- |
| `ops.metricsEnabled`           | `false` (example) | Ops metrics off                | Ops node metrics collected/reported |
| `ops.allowRestart`             | `false` (example) | Agent restart refused          | Agent restart permitted via ops API |
| `ops.allowExec`                | `false` (example) | Exec refused (allowlist empty) | Exec permitted within allowlist     |
| `ops.allowUpdate`              | `false` (example) | Agent update refused           | Agent update permitted              |
| `ops.autoRestart`              | `false` (example) | No auto-restart                | Auto-restart on crash               |
| `capture.telemetry.enabled`    | `false`           | Capture OTel off               | Capture telemetry exported          |
| `devops.telemetry.enabled`     | `false`           | DevOps OTel off                | DevOps telemetry exported           |
| `synthetics.telemetry.enabled` | `false`           | Synthetics OTel off            | Synthetics telemetry exported       |

> The agent-side telemetry switches reuse the shared `telemetry` block
> (`enabled` + `enableMetrics` + `collectorUrl` + `useTls` + `serviceName`);
> see [config-agent.md](config-agent.md) for the full block and the
> `no_otel` build-tag fail-closed behavior.

## Known Boundaries

- Defaults are the repository example / code fallback; a deployment may
  override any of them via YAML / env / CLI precedence.
- Switch keys use the canonical `lowerCamelCase` config contract; the shared
  analytics `telemetry` block retains its historical `snake_case` keys
  (`enable_tracing`, `enable_metrics`, `use_tls`, `collector_url`,
  `service_name`) as a compatibility surface.
- `featureFlags.extensions` is **route-only**: all extension pages are
  `hideInMenu`, so toggling it changes no sidebar entry (verified empirically
  — the off/on menu labels are identical). It only registers/unregisters the
  extension API routes. This differs from the other four flags, which each
  hide a top-level menu.
- `featureFlags.ops` hides both Operations Center and Risk Control: the
  `/risk/*` routes are gated by the same `canOpsRead` access key as `/ops/*`,
  so disabling `ops` removes both menus.
- The off/on screenshots + GIFs above are generated by
  `scripts/docs/config-switch-evidence.sh` against a throwaway local SQLite
  server; the pipeline is re-runnable for any `featureFlags.<name>` and writes
  artifacts to `docs/public/evidence/config-effects/<flag>/`.
- The remaining switches in this inventory are server/agent-side and have no
  operator-console visual change; their evidence is config-level (the switch's
  effect is described in the row), not a screenshot.
