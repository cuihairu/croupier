---
title: Rate Limit Management
icon: throttle
order: 42
category:
  - Operations Manual
tag:
  - Security
---

# Rate Limit Management

"Ops Center → Rate Limit Management" is a workbench for **rule management and impact preview**: define function/service-dimension QPS rules and rollout percentages, preview the blast radius with real-time data, then land them on the enforcement side.

## Model (`rate_limits` table)

| Field                  | Description                                                 |
| ---------------------- | ----------------------------------------------------------- |
| `rateLimitId` / `name` | Identifier and display name                                 |
| `resource`             | `function` / `api` / `user`                                 |
| `limit` / `window`     | Requests per window / window in seconds (implied QPS)       |
| `action`               | `reject` (reject over-limit) / `throttle` (smooth slowdown) |
| `rules`                | Rule group JSON: `{scope(function                           | service), key, limitQps, match(labels), percent}` |
| `status`               | Enabled/disabled                                            |

Rule shape (`PUT /api/v1/rate-limits` writes the whole group):

```json
{
  "rateLimitId": "mail.send.guard",
  "name": "Bulk mail sending guard",
  "resource": "function",
  "limit": 100,
  "window": 1,
  "action": "reject",
  "rules": [
    {
      "scope": "function",
      "key": "mail.send",
      "limitQps": 50,
      "match": {},
      "percent": 100
    },
    {
      "scope": "service",
      "key": "game-node-1",
      "limitQps": 200,
      "match": { "env": "prod" },
      "percent": 30
    }
  ]
}
```

- `scope: function` limits by function ID; `scope: service` limits by Agent/service instance
- `match` matches on Agent labels; `percent` is the rollout percentage (30 = only 30% of traffic is subject to this rule)

## Impact Preview

`POST /api/v1/rate-limits/preview`: based on real-time registry data, returns the rule's hit surface — matched Agents and their current `qps` / `qps1m`, helping you answer before rollout "who will this rule hit, and does current traffic already exceed the limit".

## Enforcement Side (Important Boundary)

**The rule table itself is not consumed by the HTTP middleware** — the actual rate-limiting engine lives in `internal/platform/ratelimit` (token bucket / distributed limiter); the current integration point is the OpenAPI gateway side (the `openapi` config's `rateLimit.requestsPerMinute` / `burstSize`). This page's role is:

1. The central place for rule **registration, audit, and preview**
2. Taking effect requires the enforcement side (OpenAPI provider config or the game-side SDK) to reference the corresponding rule

Hard protection for function calls has its own channel: high-risk function governance in the registry/approval flow and platform-level API rate limiting; do not confuse them with this page's configuration.

## Operational Recommendations

- Preview before landing a rule: avoid applying a full `reject` directly to high-traffic functions
- Rollout: start `percent` at 10%, observe, then scale up gradually
- `throttle` suits batch operations that can queue; `reject` is the clearer semantic for public APIs
