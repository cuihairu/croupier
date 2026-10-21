---
title: Operations Manual
icon: tool
order: 1
category:
  - Operations Manual
tag:
  - Operations
  - Deployment
  - SRE
---

# Operations Manual

Operational documentation for on-call and SRE roles: it answers "how to deploy, how to upgrade, how to configure, how to troubleshoot" — for design motivation see the [Architecture section](/architecture/), and for endpoint semantics see the [API Reference](/api/).

## Content Map

| Group         | Coverage                                                                                                                                                                                           |
| ------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Deployment    | Overview of topology choices, Docker Compose (including HA orchestration), binary + systemd, Kubernetes, version upgrades and rollbacks, load balancing                                            |
| Configuration | Full Server Configuration Reference, Full Agent Configuration Reference, TLS and Certificates; for the layering model and hot reload see the Architecture section (Config Layering, Feature Flags) |
| Observability | Metrics, health checks, Prometheus integration                                                                                                                                                     |
| Reliability   | Backup and Restore, Troubleshooting, Database Migration Strategy (Chinese only)                                                                                                                    |
| Security      | TLS, secrets, RBAC audit baseline                                                                                                                                                                  |
| Release       | SDK Package Release: release workflows for the official repositories of six languages (manual), required Secrets configuration, per-language release mechanics and common issues                   |

## Port and Endpoint Quick Reference

| Port/Path      | Owner   | Purpose                                                                    |
| -------------- | ------- | -------------------------------------------------------------------------- |
| `:18780`       | Server  | HTTP API + Dashboard backend                                               |
| `:19090`       | Server  | Self-developed transport, Agent/SDK access entry point (via L4 LB)         |
| `:19091`       | Agent   | Local TCP, game-server function registration                               |
| `:8404`        | HAProxy | stats page (for troubleshooting Agent connection distribution)             |
| `GET /healthz` | Server  | Liveness probe (both the root path and `/api/v1/monitoring/healthz` work)  |
| `GET /metrics` | Server  | Prometheus metrics (authentication required, `/api/v1/monitoring/metrics`) |
| `GET /api/v1`  | Server  | Version and feature-domain metadata (basis for frontend menu trimming)     |
