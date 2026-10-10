[English](README.md) | [中文](README.zh.md)

<h1 align="center">Croupier</h1>

<p align="center">
  <img src="https://github.com/cuihairu/croupier/actions/workflows/ci.yml/badge.svg" alt="CI"/>
  <img src="https://codecov.io/gh/cuihairu/croupier/branch/main/graph/badge.svg?t=1789350127" alt="codecov"/>
  <img src="https://img.shields.io/badge/license-Apache%202.0-blue.svg" alt="License"/>
  <img src="https://img.shields.io/badge/go-1.26.6+-green.svg" alt="Go Version"/>
</p>

Croupier is a Server / Agent / SDK platform for game operations and control scenarios, serving multiple games and multiple environments within a single game company by default. The current architecture has converged on a unified session transport:

- `Agent <-> Server`: uses a `TCP session` by default, with `TLS` enabled by default
- `SDK <-> Agent`: uses a `TCP session` by default without `TLS`; TLS can be turned on when needed
- The two links share the same session transport foundation and differ only in the first handshake message and business semantics, as separate subprotocols

## Online Demo

URL: https://croupier.cuihairu.site/

| Account | Password   |
| ------- | ---------- |
| `admin` | `admin123` |

> [Demo environment. All data is fake and may be reset from time to time. Do not enter any real information.]

## Highlights

- Single-company, multi-game, multi-environment scoping model: the standard business boundary is `gameId + env`
- Business scope is separated from the runtime target: `scope` expresses ownership, `target` expresses where things are deployed and executed
- Unified model for function registration, scheduling, invocation, and jobs
- Lightweight session transport: single connection, bidirectional requests, reconnection, backpressure, and drain
- JSON payload + protobuf envelope, balancing cross-language consistency against integration cost
- JSON Schema capability contracts + a generative console UI driven by Ant Design Pro / ProComponents

## Supported Databases

| Database   | Driver                     | Use case                                |
| ---------- | -------------------------- | --------------------------------------- |
| SQLite     | `glebarez/sqlite`          | Development, testing, small deployments |
| MySQL      | `gorm.io/driver/mysql`     | Production (recommended)                |
| PostgreSQL | `gorm.io/driver/postgres`  | Production                              |
| SQL Server | `gorm.io/driver/sqlserver` | Enterprise environments                 |

For DSN configuration examples for each database, see the [server configuration guide](docs/operations/config-server.md).

## SDK Ecosystem

All official SDKs are now maintained together under the monorepo's `sdks/` directory.

### Official SDKs

| Language | Directory      | Build                                                                                                                                                                    | Coverage                                                                                                                                    | Docs                            |
| -------- | -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------- |
| Go       | `sdks/go/`     | [![Build](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-go.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-go.yml)         | [![Coverage](https://codecov.io/gh/cuihairu/croupier/branch/main/graph/badge.svg?flag=go-sdk)](https://codecov.io/gh/cuihairu/croupier)     | [README](sdks/go/README.md)     |
| C#       | `sdks/csharp/` | [![Build](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-csharp.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-csharp.yml) | [![Coverage](https://codecov.io/gh/cuihairu/croupier/branch/main/graph/badge.svg?flag=csharp-sdk)](https://codecov.io/gh/cuihairu/croupier) | [README](sdks/csharp/README.md) |
| Java     | `sdks/java/`   | [![Build](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-java.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-java.yml)     | [![Coverage](https://codecov.io/gh/cuihairu/croupier/branch/main/graph/badge.svg?flag=java-sdk)](https://codecov.io/gh/cuihairu/croupier)   | [README](sdks/java/README.md)   |
| C++      | `sdks/cpp/`    | [![Build](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-cpp.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-cpp.yml)       | [![Coverage](https://codecov.io/gh/cuihairu/croupier/branch/main/graph/badge.svg?flag=cpp-sdk)](https://codecov.io/gh/cuihairu/croupier)    | [README](sdks/cpp/README.md)    |
| Python   | `sdks/python/` | [![Build](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-python.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-python.yml) | [![Coverage](https://codecov.io/gh/cuihairu/croupier/branch/main/graph/badge.svg?flag=python-sdk)](https://codecov.io/gh/cuihairu/croupier) | [README](sdks/python/README.md) |
| JS/TS    | `sdks/js/`     | [![Build](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-js.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-js.yml)         | [![Coverage](https://codecov.io/gh/cuihairu/croupier/branch/main/graph/badge.svg?flag=js-sdk)](https://codecov.io/gh/cuihairu/croupier)     | [README](sdks/js/README.md)     |

### Web Console (Dashboard)

| Module    | Directory | Build                                                                                                                                                                  | Coverage                                                                                                                                      |
| --------- | --------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| Dashboard | `web/`    | [![Build](https://github.com/cuihairu/croupier/actions/workflows/ci-dashboard.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-dashboard.yml) | [![Coverage](https://codecov.io/gh/cuihairu/croupier/branch/main/graph/badge.svg?flag=web-dashboard)](https://codecov.io/gh/cuihairu/croupier) |

## Architecture

Three tiers: Console (Web) → Server (control plane, multi-instance HA) → Agent (proxy inside the game VPC) → Game Server / SDK.
Entry points are separated by audience: the Web console goes over L7 (HTTP API / SSE), agents go over in-network L4 (persistent TCP connections).

See the [system architecture overview](docs/architecture/index.md) and [load balancing selection](docs/operations/load-balancing.md) for details.

## Session Model

Croupier's core transport abstraction today is not a message-history model but a lightweight application-layer session:

- One reliable long-lived connection
- The first message completes identity and capability negotiation
- New requests can be initiated from both directions on the same connection
- Multiple concurrent in-flight requests are multiplexed
- heartbeat / reconnect / drain / backpressure

This is also why two terms appear in the current documentation:

- `shared session runtime`
  - The shared transport foundation: `tcp/tls + framing + mux + reconnect + heartbeat + drain`
- `subprotocol`
  - The different subprotocols that run on top of that foundation
  - For example:
    - `sdk-agent subprotocol`
    - `agent-server subprotocol`

`subprotocol` is not a per-deployment customization; it means "application-layer protocol variants that share the same session runtime but differ in handshake messages, registration content, and routing semantics".

## Scope Model

Croupier does not adopt a SaaS multi-tenant abstraction. The standard business scope consists of:

- `gameId`: game identifier. Layered forms: REST/SDK contract key `gameId`, HTTP header `X-Game-ID`, proto field and DB column `game_id`
- `env`: logical environment identifier, such as `dev`, `staging`, `prod`

Here `env` expresses a lifecycle stage and does not directly equal a specific database, cluster, or node. Physical deployment and execution location should be expressed through separate `target`, `node`, and `agent` abstractions rather than mixed into `env`.

## Documentation

- Architecture overview: [docs/architecture/index.md](docs/architecture/index.md)
- Game and environment scoping: [docs/architecture/game-environment-scope.md](docs/architecture/game-environment-scope.md)
- SDK-Agent design: [docs/architecture/sdk-agent-transport-redesign.md](docs/architecture/sdk-agent-transport-redesign.md)
- Agent-Server design: [docs/architecture/agent-server-session-transport-redesign.md](docs/architecture/agent-server-session-transport-redesign.md)
- Wire protocol: [docs/architecture/sdk-wire-protocol.md](docs/architecture/sdk-wire-protocol.md)
- Unified SDK documentation: [docs/sdks/index.md](docs/sdks/index.md)
- SDK capability matrix: [docs/sdks/sdk-parity-matrix.md](docs/sdks/sdk-parity-matrix.md)
- SDK code entry point: [sdks/README.md](sdks/README.md)

## Release Conventions

- Server / Agent release tags use `v*`, for example `v0.2.0`
- SDK release tags use a language-prefixed format:
  - `sdk-js-v0.1.0`
  - `sdk-python-v0.1.0`
  - `sdk-go-v0.1.0`
  - `sdk-java-v0.1.0`
  - `sdk-cpp-v0.1.0`
- This prevents a single tag in the monorepo from accidentally triggering every release workflow

## Repository Layout

| Component        | Location              | Description                                              |
| ---------------- | --------------------- | -------------------------------------------------------- |
| Server / Agent   | `cmd/`, `internal/`   | Control plane, proxy, scheduling, audit, registry, jobs  |
| Proto            | `proto/`              | protobuf definitions and generation entry (single source) |
| SDKs             | `sdks/`               | Multi-language SDKs (go, js, python, java, csharp, cpp)  |
| Dashboard        | `web/`                | Web console (React + Ant Design)                         |
| Examples / Tools | `examples/`, `tools/` | Examples and helper tools                                |
| Docs             | `docs/`               | Architecture, guides, API and SDK documentation          |

## Quick Start

1. Get the code

```bash
git clone https://github.com/cuihairu/croupier.git
cd croupier
```

2. Install the toolchain

- Go 1.26.6+
- Node.js 22+ / pnpm
- `buf`
- `protoc`

3. Install the pre-commit hook (recommended)

```bash
cp scripts/pre-commit .git/hooks/pre-commit && chmod +x .git/hooks/pre-commit
```

4. Build

```bash
make proto && make build
```

5. Start

```bash
./bin/croupier-server --config configs/server.yaml
./bin/croupier-agent --config configs/agent.yaml
```

6. Open the Dashboard

```bash
cd web
pnpm install
pnpm dev
```

## Notes

Some historical documents in this repository still reference `gRPC`, `legacy REQ/REP`, `LocalControl`, `rpc_addr`, or the SDK local-listener model.
These are being cleaned up step by step under the "unified TCP session + subprotocol" design and should no longer be treated as the basis for new implementations.
