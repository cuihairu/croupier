---
title: API overview
---

# API overview

The REST API serves the operator console, external management systems, and compatible clients. Agent and SDK communication is a separate session protocol; see [Architecture](/architecture/) and [SDK Wire protocol](/zh/architecture/sdk-wire-protocol).

## Basics

| Item                   | Value                |
| ---------------------- | -------------------- |
| Base path              | `/api/v1/`           |
| Transport              | HTTP or HTTPS        |
| Authentication         | JWT bearer token     |
| Business scope headers | `X-Game-ID`, `X-Env` |
| Control payload        | Protobuf envelopes   |
| Business payload       | UTF-8 JSON           |

Most management operations require an authenticated user and a permission check. Risky function execution may also require an approval and is recorded in the audit stream.

## Primary domains

| Domain                  | Purpose                                                                                                          |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------- |
| Auth and profile        | Local login, external identity providers, permission checks, user profile, and administrator management.         |
| Functions and resources | Register, inspect, invoke, and manage functions; review resource semantics and page proposals.                   |
| Pages and console       | Draft, validate, publish, diff, and roll back operator pages and dynamic menus.                                  |
| Approvals and audit     | Review risky operations and retain administrative evidence.                                                      |
| Operations              | Agents, nodes, registry, monitoring, notifications, backups, certificates, rate limits, and platform operations. |
| Analytics               | Metric queries, behavior, retention, payment, and game-performance reporting.                                    |
| Site settings           | Branding, notification, security, outbound integration, and login-provider configuration.                        |

## Contract discipline

The effective route registration is the source of truth for endpoint existence. A domain should have one canonical page; compatibility pages must be marked as compatibility rather than extended with new semantics. When documentation and code disagree, update the documentation before adding another endpoint.

When a registered endpoint family does not yet have a standalone page, document its handler path in this overview and use the handler as the implementation reference.

## Discovery examples

```http
GET /healthz
GET /api/v1
GET /api/v1/monitoring/healthz
GET /api/v1/monitoring/metrics
```

Health and version endpoints are useful smoke checks. Monitoring metrics require authentication.

## Related pages

- [Function API](/api/function)
- [Resource Catalog API](/api/resource)
- [Auth API](/api/auth)
- [Architecture overview](/architecture/)
