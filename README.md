<p align="center">
  <img src="docs/public/logo.png" alt="Croupier Logo" width="64"/>
</p>

<h1 align="center">Croupier</h1>

<p align="center">
  <img src="https://github.com/cuihairu/croupier/actions/workflows/ci.yml/badge.svg" alt="CI"/>
  <img src="https://codecov.io/gh/cuihairu/croupier/branch/main/graph/badge.svg?t=1789350127" alt="codecov"/>
  <img src="https://img.shields.io/badge/license-Apache%202.0-blue.svg" alt="License"/>
  <img src="https://img.shields.io/badge/go-1.27.2+-green.svg" alt="Go Version"/>
</p>

[English](README.md) | [中文](README.zh.md)

Croupier is a Server / Agent / SDK platform for game operations and control, intended by default for multiple games and multiple environments within a single game company. The architecture has converged on a unified session transport:

- `Agent <-> Server`: TCP session by default, with TLS enabled by default
- `SDK <-> Agent`: TCP session by default, TLS off by default and enabled on demand
- Both links share the same session transport foundation and differ only in the first handshake message and business semantics

## Online Demo

URL: https://croupier.cuihairu.site/

| Account | Password  |
| ------- | --------- |
| `admin` | `admin123`|

> [Demo environment: all data is fake and is reset from time to time. Do not enter any real information.]

## Highlights

- Single-company, multi-game, multi-environment scope model: the standard business boundary is `gameId + env`
- Separation of business scope and run target: `scope` expresses ownership, `target` expresses deployment and execution location
- Unified function registration, dispatch, invocation, and job model
- Lightweight session transport: single connection, bidirectional requests, reconnect, backpressure, stream detach
- JSON payload + protobuf envelope: cross-language consistency at a reasonable integration cost
- JSON Schema capability contracts + a generated console UI driven by Ant Design Pro/ProComponents

## Supported Databases

| Database   | Driver                     | Typical use                        |
| ---------- | -------------------------- | ---------------------------------- |
| SQLite     | `glebarez/sqlite`          | Development, testing, small setups |
| MySQL      | `gorm.io/driver/mysql`     | Production (recommended)           |
| PostgreSQL | `gorm.io/driver/postgres`  | Production                         |
| SQL Server | `gorm.io/driver/sqlserver` | Enterprise environments            |

DSN configuration examples for each database are in the [server configuration guide](docs/operations/config-server.md).

## SDK Ecosystem

All official SDKs are maintained together in the `sdks/` directory of this monorepo.

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

| Module    | Directory | Build                                                                                                                                                                  | Coverage                                                                                                                                       |
| --------- | --------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| Dashboard | `web/`    | [![Build](https://github.com/cuihairu/croupier/actions/workflows/ci-dashboard.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-dashboard.yml) | [![Coverage](https://codecov.io/gh/cuihairu/croupier/branch/main/graph/badge.svg?flag=web-dashboard)](https://codecov.io/gh/cuihairu/croupier) |

## Architecture

Three tiers: Console (Web) → Server (control plane, capable of multi-instance HA) → Agent (proxy inside the game VPC) → Game Server / SDK.
Entry points are separated by audience: the web console uses L7 (HTTP API / SSE), while agents use internal L4 (long-lived TCP connections).

See the [architecture overview](docs/architecture/index.md) and the [load balancing guide](docs/operations/load-balancing.md) for details.

## Open-Source Foundation

Croupier is not a fork. It is a control-plane business layer, and everything underneath is built on open-source components (versions as pinned in `go.mod` / `web/package.json`):

- Server: Go 1.26, [Gin](https://github.com/gin-gonic/gin) for HTTP, [GORM](https://gorm.io) as the ORM (MySQL / PostgreSQL / SQL Server / glebarez SQLite drivers), [Casbin](https://casbin.org) for authorization decisions, goose for numbered migrations
- Transport: the TCP session for Agent↔Server and SDK↔Agent is implemented on the Go standard library `net` + `crypto/tls` (length-prefix framing, protobuf envelope). gRPC is not used; the rationale is in [transport-no-grpc.md](docs/architecture/transport-no-grpc.md)
- Observability: [OpenTelemetry](https://opentelemetry.io) Go SDK with OTLP HTTP exporter
- Console: built on [Umi Max](https://umijs.org) and [Ant Design](https://ant.design) / ProComponents; JSON Schema forms are driven by [RJSF](https://rjsf.github.io/react-jsonschema-form/), and the editor is [Monaco](https://microsoft.github.io/monaco-editor/)
- Protocol and tooling: [protobuf](https://protobuf.dev) (fully local protoc generation) + [buf](https://buf.build) lint
- The six SDKs (go / js / python / java / csharp / cpp) are built on each language's standard library; the wire contract is in [sdk-wire-protocol.md](docs/architecture/sdk-wire-protocol.md)

## Session Model

The core transport abstraction in Croupier is not a "message history" model but a lightweight application-layer session:

- One reliable long connection
- The first message negotiates identity and capabilities
- New requests can be initiated in both directions on the same connection
- Multiple concurrent in-flight requests are multiplexed
- heartbeat / reconnect / drain / backpressure

This is why two terms appear in the documentation:

- `shared session runtime`
  - The shared transport foundation: `tcp/tls + framing + mux + reconnect + heartbeat + drain`
- `subprotocol`
  - A protocol variant running on top of that foundation, for example:
    - `sdk-agent subprotocol`
    - `agent-server subprotocol`

A `subprotocol` is not "custom configuration"; it is an application-layer protocol variant that shares the same session runtime but differs in handshake message, registration content, and routing semantics.

## Scope Model

Croupier does not adopt a SaaS multi-tenant abstraction. The standard business scope is:

- `gameId`: the game identifier. Layered forms: contract key `gameId` in REST/SDK, HTTP header `X-Game-ID`, proto field and DB column `game_id`
- `env`: logical environment identifier, such as `dev`, `staging`, `prod`

`env` here expresses a lifecycle stage and is not equal to a specific database, cluster, or node. Physical deployment and runtime location should be expressed through separate `target`, `node`, and `agent` abstractions, not mixed into `env`.

## Documentation

- Architecture overview: [docs/architecture/index.md](docs/architecture/index.md)
- Game and environment scope: [docs/architecture/game-environment-scope.md](docs/architecture/game-environment-scope.md)
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
- This prevents one tag in the monorepo from accidentally triggering every release workflow

## Repository Layout

| Component        | Location              | Description                                                     |
| ---------------- | --------------------- | --------------------------------------------------------------- |
| Server / Agent   | `cmd/`, `internal/`   | Control plane, proxy, dispatch, audit, registry, and jobs       |
| Proto            | `proto/`              | Protobuf definitions and generation entry point (single source) |
| SDKs             | `sdks/`               | Multi-language SDKs (go, js, python, java, csharp, cpp)         |
| Dashboard        | `web/`                | Web console (React + Ant Design)                                |
| Examples / Tools | `examples/`, `tools/` | Examples and helper tools                                       |
| Docs             | `docs/`               | Architecture, guides, API, and SDK documentation                |

## One-Line Agent Install

Install croupier-agent on a game server with a single command (detects OS and CPU architecture automatically, downloads the artifact anonymously, and rerunning upgrades in place):

Linux (x86_64 / ARM64 / ARMv7):

```bash
curl -fsSL https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.sh | bash -s --
```

macOS (Intel / Apple Silicon):

```bash
curl -fsSL https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.sh | bash -s --
```

Windows (PowerShell 5.1+, x64):

```powershell
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.ps1)))
```

The default is the latest stable release; `--version nightly` installs the daily build, `--with-service` registers autostart on boot (systemd / launchd / Windows service), and `--uninstall` removes the agent. Full usage is in [agent install](docs/operations/agent-install.md).

## Docker Compose Deployment

Pre-built images bring up a minimal stack with one command (server + agent + dashboard + postgres/redis; no local build required):

```bash
cd docker
docker compose -f docker-compose.quickstart.yml up -d
```

Common operations:

```bash
docker compose -f docker-compose.quickstart.yml logs -f server   # follow logs
docker compose -f docker-compose.quickstart.yml down             # stop
docker compose -f docker-compose.quickstart.yml pull && \
docker compose -f docker-compose.quickstart.yml up -d            # upgrade
docker compose -f docker-compose.quickstart.yml down -v          # ⚠️ wipe data (removes volumes too)
```

Optional components (six-language SDK examples / analytics pipeline) are started with `--profile` and are not part of the default stack; secrets, ports, multi-game vs. single-database switching, and the pitfall that "a `--profile` pull cascades into rebuilding the full stack" are described in the header comments of
[docker-compose.quickstart.yml](docker/docker-compose.quickstart.yml)
and the [Docker deployment guide](docs/operations/deploy-docker.md).

## Quick Start

1. Clone the repository

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

6. View the dashboard

```bash
cd web
pnpm install
pnpm dev
```

## Notes

Some historical documents in this repository still reference `gRPC`, the historical `REQ/REP`, `LocalControl`, `rpc_addr`, or the SDK local-listener model.
These are being cleaned up step by step under the unified "TCP session + subprotocol" design and should no longer be treated as the basis for new implementations.
