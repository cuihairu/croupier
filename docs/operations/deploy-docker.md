---
title: Deployment Guide
icon: rocket
order: 3
category:
  - Operations Manual
tag:
  - Deployment
---

# Deployment Guide

This document keeps only the deployment options matching artifacts that already exist in this repo: Docker images, `docker compose`, and binary deployment.

## Recommended Components

- Server
- Agent
- MySQL or PostgreSQL
- Redis
- Optional: analytics-worker, ingest, ClickHouse
- Optional: capture-agent (CDC change capture; enabled by default in the source-build stack, see below)

## Docker Deployment

Three orchestrations split by scenario — do not mix them:

| Orchestration                   | Image source  | Use case                                                         |
| ------------------------------- | ------------- | ---------------------------------------------------------------- |
| `docker-compose.quickstart.yml` | ghcr prebuilt | Quick trial/demo: a minimal stack in one command, no local build |
| `docker-compose.yml`            | Source build  | Local development: change code, change the container             |
| `docker-compose.deploy.yml`     | ghcr prebuilt | Production HA: dual server/agent instances + haproxy             |

### Quick Trial (quickstart, out of the box)

Prebuilt images (ghcr.io, anonymously pullable) bring up the minimal stack: postgres + redis + server + agent + dashboard.

```bash
cd docker
docker compose -f docker-compose.quickstart.yml up -d
# Verify: all five containers healthy; curl http://localhost:18780/healthz should show
# "ok":true with registry.agentsHealthy=1 (agent registered); Dashboard on :8000
```

Common operations:

```bash
# logs / stop
docker compose -f docker-compose.quickstart.yml logs -f server
docker compose -f docker-compose.quickstart.yml down

# upgrade the whole stack (pull + rebuild)
docker compose -f docker-compose.quickstart.yml pull
docker compose -f docker-compose.quickstart.yml up -d

# wipe data (⚠️ removes data volumes too: business DB, uploads, everything)
docker compose -f docker-compose.quickstart.yml down -v
```

Optional components come up via profiles and never mix into the default minimal stack:

```bash
# six-language SDK + OpenAPI examples — list service names explicitly when you only want the examples (see the "implicit full-stack update" pitfall below)
docker compose -f docker-compose.quickstart.yml --profile sdk-examples \
  up -d sdk-demo-go sdk-demo-python sdk-demo-java \
        sdk-demo-js sdk-demo-cpp sdk-demo-csharp

# analytics pipeline (ClickHouse + ingestion + worker); set ANALYTICS_BRIDGE_ENABLED=true
# before server events are written to the analytics stream
ANALYTICS_BRIDGE_ENABLED=true docker compose -f docker-compose.quickstart.yml \
  --profile analytics up -d
```

> **Implicit full-stack update pitfall**: `--profile sdk-examples pull && up -d` also
> pulls the newer main images of server/agent/dashboard and cascades recreate —
> effectively an upgrade (new features + database migrations land directly). When
> you only want to rebuild the example containers, scope both pull/up to explicit
> service names; if you accept the cascade, verify the new migrations have landed.
> For secrets/ports/multi-game-single-DB switching, see the header comment of
> `docker/docker-compose.quickstart.yml`.

### Local Development (source build)

```bash
docker compose up -d
```

Current image build entry points:

- `docker/Dockerfile.server` -> `./cmd/server`
- `docker/Dockerfile.agent` -> `./cmd/sidecar-agent` (named `sidecar-agent`; plan A has landed: only the source directory was renamed, the artifact name `croupier-agent` is unchanged — see the [Agent inventory](../design/agents-inventory.md) §9)
- `docker/Dockerfile.capture-agent` -> `./cmd/capture-agent` (CDC, see below)
- `docker/Dockerfile.analytics-worker` -> `./cmd/analytics-worker`
- `docker/Dockerfile.ingest` -> `./cmd/ingest`

#### capture agent (CDC, enabled by default in the source-build stack)

Connects outbound to `server:19090` to register and come online + subscribes to the postgres logical replication stream (`wal_level=logical` is pinned by `postgres.command`), with the table subscription whitelist `public.capture_demo` (an example table; switch to item-related tables in production). **Enabled by default**; the switch is the environment variable `CAPTURE_AGENT_REPLICAS` (`deploy.replicas`, default `1`; `0` = off).

| Environment variable         | Default                                                          | Description                                                                                                                                                                                                          |
| ---------------------------- | ---------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `CAPTURE_AGENT_REPLICAS`     | `1`                                                              | Number of capture-agent replicas. Only `0` or `1` allowed — `capture_data` is a shared position volume; multiple replicas would trample each other's positions. `0` = service off                                    |
| `CROUPIER_CAPTURE_PG_DSN`    | `postgres://croupier:***@postgres:5432/croupier?sslmode=disable` | Source database connection string. **Credential red line**: the plaintext DSN only travels via environment variables; the config file `docker/configs/capture-agent.yaml` only names the variable (`capture.dsnEnv`) |
| `POSTGRES_IMAGE`             | `postgres:15-alpine`                                             | Shared with the whole stack; CDC needs pgoutput decoding (PG 10+, logical replication)                                                                                                                               |
| `GO_IMAGE` / `RUNTIME_IMAGE` | `golang:1.27.3` / `gcr.io/distroless/static:nonroot`             | Shared with other services                                                                                                                                                                                           |

Prerequisites (postgres side):

- **`wal_level=logical`**: pinned by compose's `postgres.command`; existing stacks must add the same `command` to postgres and recreate the container (a `command` change triggers recreate).
- **The publication must be pre-created** (the connector creates slots, not publications): `docker/postgres-init/01-capture-publication.sql` creates the `public.capture_demo` table + the `croupier_capture` publication when **initializing an empty volume for the first time**. Existing volumes do not replay initdb scripts; execute manually:

```bash
docker exec croupier-postgres psql -U croupier -d croupier <<'SQL'
CREATE TABLE IF NOT EXISTS public.capture_demo (id int PRIMARY KEY, note text, updated_at timestamptz NOT NULL DEFAULT now());
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_publication WHERE pubname = 'croupier_capture') THEN
    CREATE PUBLICATION croupier_capture FOR TABLE public.capture_demo;
  END IF;
END $$;
SQL
```

- **REPLICATION privilege**: this stack's `POSTGRES_USER=croupier` is a superuser and has it naturally; in production use a dedicated replication account and grant `REPLICATION`.

One-shot verification (enabled state):

```bash
docker compose up -d                          # capture-agent starts by default
docker compose ps capture-agent               # STATUS should become healthy
docker compose logs --tail=30 capture-agent | grep -iE 'registered upstream|postgres|event'
# create a real change → decoded events appear in the log
docker exec croupier-postgres psql -U croupier -d croupier \
  -c "INSERT INTO public.capture_demo VALUES (1, 'compose-verify', now())"
```

Off-state verification:

```bash
CAPTURE_AGENT_REPLICAS=0 docker compose up -d capture-agent   # scale to 0 containers
docker compose ps capture-agent                                # no output
```

The health check uses **heartbeat file freshness** rather than a port: the capture agent has no local listening port (outbound-only), so the healthcheck is `find /app/data/capture -name agent-heartbeat -mmin -2` (supervisable heartbeat stamping, with `restart: unless-stopped` as the fallback).

Known boundaries:

- `docker-compose.deploy.yml` (ghcr prebuilt, production HA stack) does not yet include the capture-agent: CI only builds the server/agent/ingest/analytics-worker/openapi-demo images and has not published a `croupier-capture-agent` image. Enabling it in the HA stack requires first adding a build job to `.github/workflows/docker.yml` and adding the `x-capture-agent-common` anchor.
- `docker-compose.quickstart.yml` (the minimal trial stack) excludes the capture-agent to keep CDC prerequisites (wal_level / publication) out of the demo path.
- When the capture agent cannot reach the server, **only registration and policy pulls are affected, not capture** (the runner's registration loop and capture loop are independent); conversely, with `capture.enabled=false` the process still registers and heartbeats, it just captures nothing (registration only).

### HA Dual-Instance Deployment (docker-compose.deploy.yml)

The production orchestration `docker/docker-compose.deploy.yml` deploys the dual-instance topology per the [Server multi-instance HA architecture](../architecture/server-ha-multi-instance.md):

```
                          ┌─► croupier-server  ── HTTP API 18780 ─┐
Browser ──► dashboard nginx ┤                                       ├─► shared postgres/redis
(L7 split)                └─► croupier-server2 ── HTTP API 18780 ─┘

agent/agent2 ── TCP ──► haproxy :19090 (L4: leastconn + tcp-check + runtime re-resolution)
                          ├─► croupier-server  :19090 ──┐
                          └─► croupier-server2 :19090 ──┴─► cluster-interconnect forwarding + shared directory

sdk-examples ── HTTP ──► agent:19091      haproxy stats ──► :8404 (connection-distribution triage)

openapi-provider-demo ── TCP ──► haproxy :19090 (embedded Agent, registers functions via providers.yaml)
```

- **Dual Server** (`server`/`server2`, YAML anchor shares config): cluster membership table + owner forwarding coordinate automatically; if either instance fails, the other takes over invocations (agents disconnect and reconnect through the LB to the surviving instance — architecture doc §6 failure semantics)
- **Dual Agent** (`agent`/`agent2`): upstream uniformly via the `haproxy:19090` L4 LB; `configs/agent2.yaml` differs in Agent ID and httpAddr (that file is not committed; at deployment, copy `configs/agent.yaml` and modify the differing entries)
- **Two LB layers with separate duties** (nginx for people, HAProxy for machines):
  - dashboard nginx (L7): `split_clients` hashes requests across both instances' 18780 + docker DNS resolver for runtime resolution (10s, so instance rebuilds changing IPs never 502); SSE buffering disabled
  - haproxy (L4): agents' self-developed transport TCP long connections spread by `leastconn` + `tcp-check` active health checks + `resolvers` runtime re-resolution (follows instance rebuilds automatically) + stats page (:8404)
- Host ports are published only by instance 1 of each pair (server: 8443→19090 transport / 18780 HTTP, agent: 19091); instance 2 is reachable only inside the cluster
- **openapi-provider-demo** (`sdk-examples` profile, deployed with `enable_sdk_examples=true`): the resident container of `examples/openapi-provider` (image `croupier-openapi-provider-demo`); its embedded Agent registers the players API into the server via providers.yaml — the data source for the Dashboard's "OpenAPI Sources → runtime imports" section, with scope following `CROUPIER_SDK_EXAMPLE_GAME_ID/ENV`

```bash
cd docker
docker compose -f docker-compose.deploy.yml up -d
# Verify: Dashboard "Ops center → cluster topology" should show both instances online + agent ownership distribution
```

> **Historical note**: the dashboard nginx's stream block once carried the Agent L4 (upstream resolved only at startup, so instance rebuilds changing IP required `--force-recreate dashboard`); an independent haproxy service now carries it (runtime re-resolution + active health checks), and that limitation no longer exists.

### Load-Balancer Option Selection

Both the Agent's L4 entry and the Dashboard's L7 entry can be swapped per deployment environment. For the detailed comparison (nginx stream vs HAProxy vs keepalived+LVS), config examples, and migration paths, see the standalone **load balancing** page. Quick reference:

| Option                             | Layer     | Use case                                                                                                                                                         |
| ---------------------------------- | --------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **HAProxy** (this repo's default)  | L4        | Agent ingress LB: runtime DNS re-resolution + active health checks + observable stats                                                                            |
| nginx stream                       | L4 TCP    | Alternative: reuses the dashboard image with zero new components, but upstream resolves only at startup (rebuilds need a reload) and has no active health checks |
| **keepalived + LVS/nginx/HAProxy** | VRRP + L4 | LB self-high-availability (VIP failover); the public-cloud equivalent is a cloud NLB                                                                             |
| Kubernetes Service                 | L4        | K8s environments need no self-built LB layer                                                                                                                     |

Whichever option, **the Agent side needs no changes at all**: single-address connection + reconnect + re-register (architecture doc §6.2 failover timeline).

### Building Images Individually

```bash
docker build -t croupier-server:latest -f docker/Dockerfile.server .
docker build -t croupier-agent:latest -f docker/Dockerfile.agent .
```

## Binary Deployment

### Server

```bash
make server
./bin/croupier-server --config configs/server.yaml
```

### Agent

```bash
make agent
./bin/croupier-agent --config configs/agent.yaml
```

### Analytics Chain

```bash
make worker
make ingest
./bin/analytics-worker
ANALYTICS_INGEST_SECRET=dev-secret ./bin/ingest --addr :8088 --secret dev-secret
```

## Service Install Scripts

The repo provides:

- `scripts/install-systemd.sh`
- `scripts/install-launchd.sh`
- `scripts/install-windows-service.ps1`

## Health Checks

```bash
curl http://localhost:18780/healthz
curl http://localhost:8088/healthz
```

## Deployment Recommendations

1. Use externally managed instances for the database and Redis; do not co-locate storage with the application containers.
2. Do not use the repo's default `configs/*.yaml` directly in production; copy and adapt per environment.
3. Inject sensitive values like `DATABASE_URL`, `JWT_SECRET`, and object-storage keys via environment variables.
4. The Server exposes the HTTP entry; agents connect through the L4 load-balanced entry (see "Load-balancer option selection" above) — never straight to a single Server instance.

## Next Steps

- [Security Configuration](./security.md)
