---
layout: home
title: Home
hero:
  name: Croupier
  image:
    src: /logo.png
  tagline: Distributed game operations control plane and agent collaboration platform
  actions:
    - text: Quick Start →
      link: /guide
      theme: primary
    - text: SDK Guide
      link: /sdks
      theme: secondary
    - text: API Reference
      link: /api
      theme: secondary
features:
  - icon: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" width="24" height="24"><rect x="3" y="4.5" width="18" height="13" rx="2"/><path d="M7 9h5"/><path d="M7 13h7"/><circle cx="17" cy="9" r="1.6"/><circle cx="17" cy="13" r="1.6"/><path d="M9 17.5v2"/><path d="M15 17.5v2"/></svg>'
    title: Control plane and agent collaboration
    details: The Server owns permissions, approvals, audit, configuration and function routing; Agents connect game services and node capabilities through a unified session transport.
  - icon: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" width="24" height="24"><path d="M9.5 3.5h5v3h-5z"/><rect x="4.5" y="6.5" width="15" height="14" rx="2"/><path d="M8.5 11.5h7"/><path d="M8.5 15.5h4.5"/><path d="M15.5 15.5h1.5"/></svg>'
    title: Registration-driven functions
    details: OpenAPI/JSON Schema act as the function contract input; the Server recognizes resource capabilities and generates default business pages that can be published directly and edited in place.
  - icon: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" width="24" height="24"><path d="M12 3 5 6v5.2c0 4.4 2.9 7.4 7 8.8 4.1-1.4 7-4.4 7-8.8V6l-7-3z"/><path d="M9 10.5h6"/><path d="M9 14h6"/></svg>'
    title: Two-layer policy architecture
    details: YAML defaults combined with database override policies, supporting low / medium / high / critical four-level risk control.
  - icon: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" width="24" height="24"><path d="M9.5 14.5H7a2.5 2.5 0 0 1 0-5h2.5"/><path d="M14.5 9.5H17a2.5 2.5 0 0 1 0 5h-2.5"/><path d="M8.5 12h7"/><circle cx="12" cy="12" r="9" stroke-dasharray="2.5 3.5"/></svg>'
    title: Complete audit chain
    details: Every operation is written to the audit log, high-risk operations require dual approval, and a hash chain prevents tampering.
  - icon: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" width="24" height="24"><path d="M4 20V11"/><path d="M9.5 20V5"/><path d="M15 20v-6"/><path d="M20.5 20V8"/><path d="m3.5 8.5 5-4 5 4.5 6-5.5"/></svg>'
    title: Analytics pipeline
    details: ingest, worker, Redis Streams and ClickHouse form a standalone analytics pipeline; its docs live under the Analytics entry.
footer: Apache-2.0 License | Copyright © 2024-present Croupier
---

## What is Croupier

Croupier is a Server / Agent / SDK platform for game operations and control scenarios, serving multiple games and environments inside a single game company by default. The architecture has converged on "unified session transport":

- `Agent <-> Server`: `TCP session` by default, `TLS` enabled by default
- `SDK <-> Agent`: `TCP session` by default, `TLS` off by default and enabled on demand
- Both links share the same session transport foundation and differ only in the first handshake message and business semantics of the sub-protocol

## System Architecture

Entries are split by audience into **two independent planes** — do not merge them into one "load balancer layer":

- **Web console plane** (for operators/administrators, public or office network): L7 `dashboard nginx`,
  routing HTTP API / SSE per request to the Server's `:18780`
- **Agent plane** (intranet, outbound connections from game VPCs): L4 `haproxy` (`:19090`),
  spreading TCP long connections across Server control listeners; connection semantics are
  "reconnect equals failover", completely different from per-request HTTP routing

```mermaid
graph TB
  subgraph "Presentation"
    UI[Dashboard<br/>React + Ant Design Pro + ProComponents]
  end

  subgraph "Entry · Web console plane (L7, per-request routing)"
    NGINX[dashboard nginx<br/>HTTP API / SSE → :18780]
  end

  subgraph "Entry · Agent plane (L4, intranet, per-connection spreading)"
    HAPROXY[haproxy :19090<br/>TCP long connections / active health checks]
  end

  subgraph "Control layer (multi-instance HA)"
    ServerA["Server A<br/>:18780 HTTP · :19090 TCP<br/>Registry / Dispatch / RBAC / Audit"]
    ServerB["Server B<br/>:18780 HTTP · :19090 TCP<br/>Registry / Dispatch / RBAC / Audit"]
    ServerA <-.->|inter-instance<br/>forwarding + fencing| ServerB
  end

  subgraph "Shared state"
    Shared[(shared storage<br/>membership / registry / DB / Redis)]
  end

  subgraph "Proxy layer (game VPC, outbound only)"
    Agent1[Agent 1<br/>Session Client + Local Gateway]
    Agent2[Agent 2<br/>Session Client + Local Gateway]
  end

  subgraph "Business layer"
    GS1[Game Server A<br/>SDK / Third-party App]
    GS2[Game Server B<br/>SDK / Third-party App]
    GS3[Game Server C<br/>SDK / Third-party App]
  end

  UI -->|HTTPS · REST / SSE| NGINX
  NGINX --> ServerA
  NGINX --> ServerB
  Agent1 -->|TCP Session + TLS| HAPROXY
  Agent2 -->|TCP Session + TLS| HAPROXY
  HAPROXY --> ServerA
  HAPROXY --> ServerB
  ServerA --- Shared
  ServerB --- Shared
  GS1 -->|TCP Session| Agent1
  GS2 -->|TCP Session| Agent2
  GS3 -->|TCP Session| Agent1
```

> The two entry planes do not overlap: the web plane's nginx never touches Agent traffic, and the
> Agent plane's haproxy never touches HTTP. The L4/L7 selection rationale and removing the LB
> single point via VRRP are covered in [Operations](/operations).

> The Server supports multi-instance high-availability deployment (`cluster.enabled`): member
> discovery goes through shared storage (no seeds, no static peers), and requests landing on a
> non-owner instance are forwarded over the inter-instance link (one-hop limit + epoch fencing).
> Single-instance deployments have clustering off by default with zero overhead. See
> [Server multi-instance HA design](/zh/architecture/server-ha-multi-instance).

Key boundary notes:

- The `Server` no longer depends on reverse connections to the `rpc_addr` exposed by `Agent`
- The `Agent`'s local listener only serves `GameServer / SDK / third-party applications`
- `Server -> Agent` calls (`Invoke / StartTask / CancelTask / Ops`) reuse the existing `Agent-Server` session

## Core Capabilities

### Function management

- **OpenAPI-driven registration**: register functions quickly via the OpenAPI specification
- **JSON Schema form rendering**: fields and validation derived from the function contract; a JSON Schema form adapter uniformly renders call, query, create, edit and action forms
- **Resource/PageSpec orchestration**: CRUD resources, standalone operations, paginated tables, reports and task pages are composed from strongly-typed PageSpecs binding multiple functions
- **Unified invocation path**: the control plane routes calls through the Agent session; local access goes through the Agent gateway

### Permissions and security

Authorization is based on a Casbin RBAC/ABAC hybrid model — beyond roles, requests can be allowed or denied by attributes. Policies are two-layer: YAML provides defaults, and database override policies tune them online. Operations carry low / medium / high / critical risk levels; high-risk operations automatically enter the approval flow, and dual-approval rules require multiple confirmations before release.

### Scope model

- **Single company, multiple games**: no SaaS multi-tenant abstraction; the standard business boundary is `game_id`
- **Multi-environment governance**: `env` expresses logical environments such as `dev/test/staging/prod`
- **Ownership vs. deployment separated**: `scope` expresses business ownership, `target` expresses runtime location

### Observability

Every operation is written to the audit log; records are linked by a hash chain — altering one entry breaks the chain and can be detected afterwards. Sensitive fields such as passwords and tokens are automatically masked in logs and responses.

### Operations tooling

- **Ticketing**: player issue ticket workflows
- **Feedback management**: collect player feedback
- **Announcements**: publish game announcements
- **Analytics**: player behavior, retention, payment and other data analysis

## Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/cuihairu/croupier.git
cd croupier

# Download dependencies
go mod download

# Build the services
make build
```

### Start the services

```bash
# Start the Server
./bin/croupier-server --config configs/server.yaml

# Start the Agent
./bin/croupier-agent --config configs/agent.yaml
```

### Verify the installation

```bash
# Check Server health
curl http://localhost:18780/healthz

# Inspect API meta information (version / capability declarations)
curl http://localhost:18780/api/v1/
```

## Documentation Navigation

Organized by usage path:

| Path             | Entry                                                                                                                                                             | Description                                                                                                       |
| ---------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| **Quick Start**  | [Guide](/guide/) · [Guide](/guide)                                                                                                                                | Installation, configuration, Server + Agent startup, minimal loop                                                 |
| **Page product** | [Function management](/zh/guide/concepts/function-management) · [Page Studio](/zh/guide/concepts/function-registration-ui) · [Resource Catalog](/zh/api/resource) | Register functions → pages generated automatically → publish via Proposal Inbox → controlled execution in Console |
| **SDK**          | [SDK overview](/sdks)                                                                                                                                             | Provider / Invoker / configuration / capability matrix, 6 languages                                               |
| **API**          | [API reference](/api)                                                                                                                                             | REST contracts, authentication, game/env scope, per-resource APIs                                                 |
| **Operations**   | [Operations](/operations) · [Analytics](/analytics)                                                                                                               | Deployment, monitoring, backup, analysis, troubleshooting                                                         |
| **Development**  | [Architecture](/architecture)                                                                                                                                     | Architecture, code conventions, extension strategy, release rules                                                 |

Historical design and migration documents live under [Architecture — proposals and migrations](/architecture/) (collapsed in the sidebar) and are not part of the main onboarding path.

## Tech Stack

- Backend: Go 1.27, Gin for HTTP, GORM for ORM (MySQL / PostgreSQL / SQLite / SQL Server)
- Authorization: Casbin (RBAC/ABAC)
- Internal transport: custom TCP session, length-prefixed framing with a protobuf envelope
- Console: Umi Max + React + Ant Design Pro / ProComponents
- Documentation site: VitePress

## Roadmap

- [x] Basic function registration and invocation
- [x] RBAC permission control
- [x] Approval workflow
- [x] Audit log
- [x] Two-layer policy architecture
- [x] Dashboard vNext: function registration generates pages automatically (Proposal Inbox / Page Studio / Resource Catalog / Console dynamic menus)
- [ ] Plugin marketplace
- [ ] More SDK languages

## License

Apache-2.0 License
