# Ops dashboard (fleet health overview)

> OPS-011 — Frontend-only feature. Data sources are existing OPS APIs; no
> backend or PageSpec/render change is involved.

The ops dashboard (`/ops/dashboard`, page `Ops/Dashboard`) is a fleet-wide health
overview for game agents. It polls the OPS APIs every 5 seconds and renders a
best-effort, read-only snapshot. It does **not** mutate state and does **not**
pretend to be a control plane — actions (drain, etc.) stay on the dedicated
ops pages.

## Data sources

| Concern                  | API                     | Notes                                         |
| ------------------------ | ----------------------- | --------------------------------------------- |
| Node resource metrics    | `listOpsNodes`          | live CPU/memory `usagePercent`, status        |
| Alert event stream       | `fetchOpsAlerts`        | silenced alerts excluded from the ticker      |
| Provision runs (today)   | `listOpsProvisionRuns`  | counts `createdAt` in the local day           |
| Synthetics probe results | `listSyntheticsResults` | per distinct `gameId\|env` scope, last 10 min |

All four are polled with `Promise.allSettled` — a single failing source does not
take the whole page down; the page keeps the last successful snapshot and falls
back to a stale banner.

## Health tiers

A node is classified into one of five tiers:

| Tier       | Color      | Condition                                                                                      |
| ---------- | ---------- | ---------------------------------------------------------------------------------------------- |
| `normal`   | green      | CPU and memory below the pressure threshold (default 60%)                                      |
| `pressure` | amber      | CPU or memory in `[pressure, overload)` (default 60–85%)                                       |
| `overload` | red        | CPU or memory at/above overload (default 85%), **or** a synthetics probe failure in the window |
| `offline`  | gray       | status `offline` or `stale`                                                                    |
| `unknown`  | light gray | no CPU/memory sample (no false positive)                                                       |

### Debounce

`pressure`/`overload` must persist for **3 consecutive poll cycles** before the
card changes color (protects against one-off spikes). `normal`, `offline` and
`unknown` apply immediately.

### Ordering

Cards sort by tier severity — `overload` → `pressure` → `normal` → `unknown` →
`offline` (offline pinned to the bottom) — then by hostname.

## Aggregation band

Total nodes, online rate (online / total, %), average CPU and memory (only
nodes with metrics), today's provision count, and active (non-silenced) alert
count, plus per-tier count tags.

## Stale handling

If the last successful node refresh is older than 30 s (or never succeeded), a
top banner warns the data is stale; the page shows the latest snapshot but does
not present it as current.

## Drill-down

Clicking a node card opens a modal with node identity/status/metrics and an
hourly resource history sparkline (`getAgentMetricsHistory`, hourly, last 48
points). A `?agentId=` deep link preselects the drill-down.

## Known boundaries

- **Account-load view** is a placeholder: the account distribution data depends
  on the scheduling/orchestration account layout, which has no data source yet
  (OPS-002 boundary). The view is explicitly marked "pending" rather than
  fabricating numbers.
- **Thresholds** are currently the default constants (60/85); network-level
  configuration of the thresholds is registered as future work.
- The dashboard is **read-only**: it does not trigger actions, and probe-failure
  is a signal, not an enforcement.
