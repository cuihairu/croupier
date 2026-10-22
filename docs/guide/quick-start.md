---
title: Quick Start
icon: lightbulb
order: 1
category:
  - Getting Started
tag:
  - Quick Start
---

# Quick Start

This guide only covers the startup methods that are still valid today:

- `Server <-> Agent` uses a single internal session link
- `SDK / GameServer / third-party local applications <-> Agent` use the Agent's local gateway
- The historical `gRPC` or `legacy REQ/REP` documentation is no longer the basis for new integrations

Besides the default sidecar agent, there are two other agent types — `devops-agent` (bastion proxy and process supervision) and `capture-agent` (CDC data capture) — deployed independently as needed.

## Environment Requirements

- Go `1.27.2`
- Node.js `24+`
- pnpm `11+`
- Docker (optional)
- `buf`

## Installation and Build

```bash
git clone https://github.com/cuihairu/croupier.git
cd croupier
go mod download
make proto
make build
```

Build artifacts are output to `bin/` by default.

## Prepare Configuration

For local development, it is recommended to use the example configurations in the repository directly:

```bash
cp configs/server.yaml configs/server.local.yaml
cp configs/agent.local.yaml configs/agent.runtime.yaml
```

Meaning of the key ports:

- `18780`: Server REST API
- `19090`: Server session/control entry point, for Agents to connect proactively
- `19091`: Agent local gateway, for SDK / GameServer / third-party local programs to connect

Notes:

- `19090` is the internal control and session link port and should no longer be understood as a `gRPC` or `legacy transport` dedicated port
- `19091` is the local access boundary, not a `Server -> Agent` callback entry point

## Start Services

### 1. Start the Server

```bash
./bin/croupier-server --config configs/server.local.yaml
```

### 2. Start the Agent

```bash
./bin/croupier-agent --config configs/agent.runtime.yaml
```

### 3. Start the Dashboard (optional)

```bash
cd web
pnpm install
pnpm dev
```

## Verification

### Check Server Health

```bash
curl http://localhost:18780/healthz
```

### Check Whether the Agent Has Connected to the Server

The Agent list requires authentication and scope headers:

```bash
curl http://localhost:18780/api/v1/ops/agents \
  -H "Authorization: Bearer ***" \
  -H "X-Game-ID: default" \
  -H "X-Env: prod"
```

### Check the Dashboard

```bash
curl http://localhost:8000
```

### Check the Local Access Boundary

If you are about to integrate the SDK, make sure the SDK target address points to the Agent's local gateway, for example:

```text
127.0.0.1:19091
```

and not the Server's `19090` port.

## Next Steps

- [Configuration Management](../operations/config-server.md)
- [SDK Documentation](../sdks/)
