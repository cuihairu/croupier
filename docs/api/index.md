---
title: API overview
---

# API overview

The REST API serves the operator console, external management systems, and compatible clients. Agent and SDK communication is a separate session protocol; see [Architecture](/architecture/) and SDK Wire protocol.

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

### Endpoint families without a standalone page

The route table in `internal/handler/routes.go` is the source of truth. The families below are registered but do not yet have their own API page; the route line is the implementation reference.

| Family              | Registered routes                                                                                                                                                                                                                    | Purpose                                          |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------ |
| Ops - provision     | `POST/GET /api/v1/ops/provision`, `GET /api/v1/ops/provision/:runId`, `POST /api/v1/ops/provision/:runId/{resume,destroy}`, `PUT /api/v1/ops/provision/:runId/protection`, `DELETE /api/v1/ops/provision/:runId` (routes.go:691-697) | Provision-engine run lifecycle (terraform/tofu)  |
| Ops - deployments   | `POST/GET /api/v1/ops/deployments`, `GET /api/v1/ops/deployments/:id`, `POST /api/v1/ops/deployments/:id/{start,rollback}` (routes.go:703-707)                                                                                       | Deployment engine                                |
| Ops - agent logs    | `GET /api/v1/ops/agents/:agentId/logs/search` (routes.go:629)                                                                                                                                                                        | Agent log tailing/search                         |
| Ops - agent metrics | `GET /api/v1/ops/agents/:agentId/metrics/history` (routes.go:654)                                                                                                                                                                    | Agent metrics history                            |
| Ops - supervisor    | `GET /api/v1/ops/agents/:agentId/supervisor`, `.../supervisor/events`, `.../supervisor/logs` (routes.go:576-579)                                                                                                                     | Managed-process supervision                      |
| Ops - alerts        | `GET /api/v1/ops/alerts`, `POST /api/v1/ops/alerts/silence`, `POST /api/v1/ops/alerts/:id/handle` (routes.go:592-595)                                                                                                                | Alert list / silence / handle                    |
| Ops - capture gates | `GET/PUT /api/v1/ops/capture/gates`, `GET /api/v1/ops/capture/gates/versions`, `POST /api/v1/ops/capture/gates/{rollback,validate}` (routes.go:659-664)                                                                              | CDC capture policy gates                         |
| Ops - cost control  | `POST /api/v1/ops/cost/estimate`, `GET /api/v1/ops/cost/{summary,idle,budget}`, `PUT /api/v1/ops/cost/budget` (routes.go:699-705)                                                                                                    | Cost estimation, node cost, idle reclaim, budget |
| Approvals           | `registerApprovalRoutes` (routes.go:232); list filters `state`/`functionId`/`gameId`/`env`/`page`/`pageSize` (internal/api/approval/dto.go:41-45)                                                                                    | Two-person-rule approval                         |
| Audit               | `GET/POST /api/v1/audit`, `GET /api/v1/audit/export`, `GET /api/v1/audit/chain/verify` (routes.go:745-748)                                                                                                                           | Audit chain query / export / integrity verify    |

## Discovery examples

```http
GET /healthz
GET /readyz
GET /api/v1
GET /api/v1/monitoring/healthz
GET /api/v1/monitoring/metrics
GET /api/v1/monitoring/status
```

Health, readiness, and version endpoints are useful smoke checks. Monitoring metrics and status require authentication.

## Related pages

- [Architecture overview](/architecture/)
