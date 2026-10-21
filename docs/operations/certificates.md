---
title: Certificate Monitoring
icon: certificate
order: 41
category:
  - Operations Manual
tag:
  - Security
---

# Certificate Monitoring

"Ops Center → Certificate Monitoring" tracks the expiry and status of TLS certificates related to the platform — **primarily manual registration** (the platform cannot auto-discover certificates signed by your internal CA), combined with expiry alert rules and integration with [notification channels](./notifications).

## Registering Certificates

Two ways:

1. **Form registration**: domain + port (default 443) + optional PEM certificate (+ private key, used only for expiry parsing, never for traffic handling)
2. **TLS dial helper** (`GET /api/v1/certificates/domain-info?domain=xxx`): performs a live TLS handshake against any domain (10s timeout) to read certificate info and prefill the form — good for probing before registering

> For configuring the platform's internal link certificates (transport/dispatch), see [TLS and Certificates](./tls-certificates); this page is an **expiry ledger** and is not responsible for issuing them.

## State Machine and Re-checks

| Status     | Criterion                                     |
| ---------- | --------------------------------------------- |
| `active`   | More than the threshold until expiry          |
| `expiring` | Inside the threshold window (default 30 days) |
| `expired`  | Already expired                               |
| `unknown`  | Never parsed successfully                     |

- Re-check a single certificate `POST /api/v1/certificates/:id/check` (parses the stored PEM and updates expiry/status)
- Re-check all `POST /api/v1/certificates/check-all` — schedule it regularly (crontab calling the API, or hosted by the platform's scheduled scheduler)
- `GET /api/v1/certificates/expiring` fetches the "expiring soon" list directly
- `GET /api/v1/certificates/stats` gives overview counts (active/expiring/expired)

## Expiry Alerts

Alert rules (`certificate_alerts` table, page form or `POST /api/v1/certificates/alerts`):

| Field             | Description                              |
| ----------------- | ---------------------------------------- |
| `domain`          | Certificate domain                       |
| `thresholdDays`   | Days before expiry to alert (default 30) |
| `active`          | Rule on/off switch                       |
| `lastTriggeredAt` | Last triggered (prevents repeated spam)  |

Fires `certificate_expiring` / `certificate_expired` events → dispatched per the ops notification routing (e.g. "expiry events only go to the DingTalk on-call group").

## Operational Recommendations

- **Managed inventory**: public domain certificates + transport CA/server certificates + dependent upstreams (database/Redis if they use TLS)
- **Cadence**: one `check-all` per day is enough; set alert thresholds by issuance cycle (use a 14-day threshold for Let's Encrypt-style 90-day certificates; for annual certificates use the 30/14/7 three-stage tiers)
- The replacement itself before expiry goes through the platform approval flow (certificate replacement is low-risk; two-person review suffices)
