---
title: Deployment Topology Overview
icon: rocket
order: 2
category:
  - Operations Manual
tag:
  - Deployment
---

# Deployment Topology Overview

Croupier has a "single control plane + sidecar Agents" architecture: the Server has no local persistent state (all state lives in the database/Redis), so the choice of deployment topology boils down to just two questions — **how many Server instances** and **how to build the entry layer**.

## Topology Comparison

| Topology                                            | Applicability                                           | Availability             | Entry layer                                          | Reference                                                                |
| --------------------------------------------------- | ------------------------------------------------------- | ------------------------ | ---------------------------------------------------- | ------------------------------------------------------------------------ |
| Single-host Compose (`docker/docker-compose.yml`)   | Development, testing, small-team pilots                 | Single point             | None (direct to 18780/19090)                         | [Docker Compose Deployment (Chinese only)](/zh/operations/deploy-docker) |
| Single-host HA (`docker/docker-compose.deploy.yml`) | Single-host production                                  | 99.9%+                   | nginx (L7) + HAProxy (L4)                            | [Docker Compose Deployment (Chinese only)](/zh/operations/deploy-docker) |
| Multi-host                                          | Cross-machine production, LB must not be a single point | 99.99%                   | HAProxy active/standby (keepalived VIP) or cloud NLB | [Load Balancing](./load-balancing)                                       |
| Kubernetes                                          | Existing K8s infrastructure                             | Depends on replica count | Service (ClusterIP) + probes                         | [Kubernetes Deployment](./deploy-kubernetes)                             |
| Binary + systemd                                    | Environments without container constraints              | Depends on topology      | Self-managed nginx/HAProxy                           | [Binary Deployment](./deploy-binary)                                     |

## Component Checklist

The minimal viable control-plane components:

- **Server** (required): HTTP API `:18780` + transport entry point `:19090`
- **Database** (required): MySQL / PostgreSQL (recommended for production); SQLite for development only
- **Redis** (required for production): cache, cluster member table, registry shared state; single-host `memory` mode works but is not recommended
- **Agents** (deployed per game network): at least one per game network, connecting outbound to the Server
- Optional: `analytics-worker` / `ingest` / ClickHouse (data analytics domain), `dashboard` (frontend static assets)

## Decision Guide

```
Is high availability needed?
├─ No → single-host Compose (accept platform downtime during Server restarts)
└─ Yes → what is the hosting environment?
    ├─ Single host → docker-compose.deploy.yml (two Servers + two LB containers)
    ├─ Multiple hosts → HAProxy active/standby + keepalived VIP (or cloud NLB)
    └─ K8s → Deployment with 2 replicas + Service (kube-proxy spreads L4 naturally)
```

Key facts (details in [Server Multi-Instance HA (Chinese only)](/zh/architecture/server-ha-multi-instance)):

- The Server is **stateless**: no data is lost when any instance crashes; on recovery it rejoins the cluster automatically (member-table leases renew/get evicted automatically)
- An Agent that reconnects and re-registers migrates its ownership to a surviving instance; **switching the entry layer requires zero Agent changes** (only `server.addr` changes)
- HA is not "dual write": multiple instances coordinate via the shared-storage member table + owner forwarding; `cluster.enabled` is the single switch
