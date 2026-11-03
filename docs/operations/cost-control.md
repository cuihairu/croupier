---
title: Cost Control
icon: dollar
order: 60
category:
  - Operations Manual
tag:
  - Cost
  - Ops
---

# Cost Control (OPS-007)

Cost control provides a pre-creation estimate, per-node cost tagging, idle reclamation and monthly budget alerting. It is surfaced in the console under `/ops/cost`.

## Pre-creation estimate

`POST /api/v1/ops/cost/estimate` takes a provider, instance type, count and expected runtime hours and returns the projected USD cost plus the known-rate flag.

The price catalog is a **built-in reference table** (`internal/ops/costcontrol/costcontrol.go` `HourlyRates`) normalized across providers. Unknown instance types are reported with `knownRate: false` and are **not** priced (the engine refuses to fabricate a cost). Production pricing must be connected to a real billing / price API; estimate accuracy is a post-deployment reconciliation item.

## Node cost and summary

`GET /api/v1/ops/cost/summary` (scoped by `X-Game-ID` / `X-Env`) aggregates the cost of adopted provision runs from the run spec + uptime and returns per-provider totals plus per-node rows with an idle flag.

Idle evaluation (`GET /api/v1/ops/cost/idle`) relies on the agent resource metrics (OPS-010): a node is considered idle when all recent CPU samples are below the configured threshold for at least `idleHours`. No recent samples or no metrics source is treated as **unknown** (never mis-classified as idle).

## Budget

`GET` / `PUT /api/v1/ops/cost/budget` manages the per-scope monthly budget. When spend crosses the threshold (default 80%), a notification is dispatched through the configured notification channels (`notification.*`), reusing the same dispatch as approvals and alerts.

## Idle reclamation policy

The policy is safe by default: **remind only**. Automatic destruction requires an explicit `enabled: true` plus `autoDestroy: true` in the `IdlePolicy`. Automatic destruction is a post-deployment item because it needs a real cloud / multi-node stack to verify.

## Panel

The console page (`web/src/pages/Ops/Cost`) shows the estimate form, cost summary, per-provider totals, node table and budget editor. It requires `canOpsManage`.
