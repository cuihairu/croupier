---
title: Alert Management
icon: bell
order: 21
category:
  - Operations Manual
tag:
  - Observability
  - Alerts
---

# Alert Management

The platform ships with a **built-in threshold rule engine** (not an Alertmanager forwarder): Agents periodically report system metrics → the rule evaluator judges them one by one → a hit raises an alert and dispatches it to notification channels. Entry point: "Ops Center → Alert Center" (requires the `ops` feature domain to be enabled).

## Metric Sources

System metrics reported by Agents (path-style keys):

| Metric path                      | Meaning                                             |
| -------------------------------- | --------------------------------------------------- |
| `cpu.usagePercent`               | CPU usage                                           |
| `memory.usagePercent`            | Memory usage                                        |
| `disk.<mount-point>.usedPercent` | Disk usage (per mount point)                        |
| `custom.<key>`                   | Custom metrics (injected by business via the Agent) |

## Alert Rules

Rule fields (Dashboard Rules tab or `POST /api/v1/alerts/rules`):

| Field             | Description                                                                                   |
| ----------------- | --------------------------------------------------------------------------------------------- |
| `metric`          | Metric path, e.g. `cpu.usagePercent`                                                          |
| `operator`        | `gt` / `gte` / `lt` / `lte`                                                                   |
| `threshold`       | Threshold                                                                                     |
| `forCount`        | Fires only after N consecutive hits (simulating a sustained window; a break resets the count) |
| `cooldownSeconds` | Cooldown after firing (default 300s; the same rule does not fire again during cooldown)       |
| `level`           | `info` / `warning` / `critical`                                                               |
| `agentFilter`     | Empty = all Agents; otherwise filter by Agent ID                                              |
| `gameId` / `env`  | Scope                                                                                         |

Example — alert only when CPU exceeds 90% for 3 consecutive cycles, with a 10-minute cooldown:

```json
{
  "metric": "cpu.usagePercent",
  "operator": "gt",
  "threshold": 90,
  "forCount": 3,
  "cooldownSeconds": 600,
  "level": "warning",
  "agentFilter": "",
  "gameId": "demo",
  "env": "prod"
}
```

## Alert Lifecycle

```
Rule hit → firing (write to the alerts table + dispatch to notification channels)
        → silenced (manual, minutes + reason) → no further prompts during silence
        → recovered (metric falls back; automatically resolved once the rule no longer hits)
```

- The list can be filtered by `level` / `status`; critical alerts are pinned to the top by default
- Silence expires automatically; it can also be deleted manually in the "Silence List"
- Trigger history is kept for audit (who silenced it and why)

## Integration with Notification Channels

When an alert fires, it is dispatched according to the notification channel configuration: in-app messages (on by default), DingTalk bots, custom webhooks, SMTP email. Dispatch failures are only logged and never block the alert from being persisted.

There is also the **herald external alert outlet** (`herald.enabled`, off by default): when enabled, events such as supervision circuit breaks (supervisor breaker_tripped) are delivered via herald — channels/recipients are all configured on the herald side; if herald is unreachable it only logs and counts failures, without blocking alert persistence.

## Relationship with External Alertmanager

With an existing Alertmanager setup, there are two ways to integrate:

1. **Inbound webhook (persistable)**: point Alertmanager webhook_configs at `POST /api/v1/alerts/inbound/alertmanager` (HMAC-SHA256 signature + timestamp anti-replay; the secret is referenced via the `alertInbound.sources.alertmanager.secretEnv` environment variable); alerts land in the same bucket as the ones on this page and capture/dbmon alerts; the generic source (`POST /api/v1/alerts/inbound/generic`) lets any third party push in with a unified envelope.
2. **Jump link only (current state)**: the `alertmanagerUrl` returned by `GET /api/v1/ops/config` (environment variable `CROUPIER_ALERTMANAGER_URL`) serves only as a page jump link.

croupier is not a cross-system alert deduplication/noise-reduction engine; inbound is one-way reception and does not write acknowledgment status back to the source system.
