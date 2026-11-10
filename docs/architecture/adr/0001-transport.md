---
title: "ADR 0001: Transport selection — custom TCP + protobuf"
---

# ADR 0001: Transport selection — custom TCP + protobuf

**Status:** Accepted (2026-11-10). This is the living record of the transport
decision; it supersedes the archived note
[`docs/archive/legacy-en/architecture/transport-no-grpc.md`](../../archive/legacy-en/architecture/transport-no-grpc.md),
which is kept for history only.

**Scope:** every long-lived Croupier session link — Server ↔ Agent control
(TCP 19090) and SDK ↔ sidecar provider (TCP 19091). Not covered: browser →
Server HTTP REST/SSE, cluster interconnect (separate link), and local plugin
IPC, which has a scoped exemption (see below).

## Context

Croupier's early prototypes used gRPC as the internal transport. The project
then went through `gRPC → NNG → TCP-only` refactors and finally removed gRPC
from the game-facing chain (git history: `7b3b955d2`, `07d0f9faa`,
`573b94038`, `4d0f1ffe4`, `775398748`). The deployment topology
that drives this decision has not changed since:

- SDK (game server process) → Agent is almost always same-host or same game
  network — no public internet, no CDN/proxy chain.
- Agents dial the Server; one outbound session carries both directions.
- One game company with multiple games; no service mesh, no multi-org
  federation, no per-tenant endpoint fan-out.

Six SDK languages (Go / JS / Python / Java / C# / C++) must embed the
transport. Every runtime pulled in is a dependency tax paid six times, and
game studios are sensitive to binary size and dependency conflicts in ways
server-side projects are not.

## Decision

All Croupier internal RPC uses a **custom TCP transport with protobuf
payloads** (`core/transport/tcp`; `internal/transport` is a compatibility
shim). gRPC is **rejected** for these links and must not be introduced
directly or transitively into the game SDK chain. The contract is specified
in [sdk-wire-protocol.md](../sdk-wire-protocol.md) and pinned by regression
tests in both the Go agent transport and all six SDK transports.

**Scoped exemption:** local gRPC IPC via HashiCorp go-plugin is allowed
between external probe/plugin hosts and their plugins, confined to the
plugin-host subtrees (`agents/synthetics/pluginhost/`,
`agents/sidecar/funcplugin/`, `core/pluginhost/`). Its dependencies must
never enter game SDK builds, links, generated code, or distribution
artifacts, and must not be passed to games through public packages. The
Agent ↔ Server remote link remains the shared TCP/TLS session regardless.

Both optional integrations ship with escape hatches: the `no_go_plugin`
build tag trims plugin hosting, and the `no_otel` build tag trims telemetry
provider code (`internal/telemetry/provider_sdk_trim.go`).

## Why gRPC was rejected (per concern)

| gRPC strength                     | Verdict for this chain                                                                                                                                                                                                                                                |
| --------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Cross-language IDL                | The need is real, but protobuf alone provides the IDL. The wire framing is 4-byte length prefix + payload — trivially implementable in each language without a gRPC runtime, with generated messages from `proto/` into `pkg/pb`.                                     |
| HTTP/2 multiplexing               | The topology removes the payoff. Wide-area features gRPC is good at (per-stream flow control, LB-aware retry propagation, per-stream deadlines) matter across lossy high-latency networks; SDK → Agent is same-host or game LAN.                                      |
| Request concurrency on one link   | Delivered by `reqID` multiplexing over a single TCP connection; the historical bidirectional-tunnel deadlock is fixed and regression-tested.                                                                                                                          |
| Ecosystem (LB, xDS, interceptors) | Single-company deployment has no mesh to integrate with. Interceptor-class needs (auth, tracing) are implemented directly: handshake auth (`agent.providerAuth`), OTel trace-context propagation.                                                                     |
| Maturity of tooling               | The historical cost is concrete: a dependency-conflict resolution that ran over a week, and a real embed case where the gRPC debug build reached ~1.7 GB — heavier than the game itself (historical record, not a universal size claim for all versions/build modes). |
| Bidirectional streaming           | The needed patterns are request/response with progress events (tasks) and control frames — both are first-class frames in the existing protocol, including dual-lane bounded dispatch with fail-fast on saturation.                                                   |

## What the custom TCP layer provides (implemented, not aspirational)

| Requirement                   | Implementation                                                                                                                                                         |
| ----------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Frame contract                | 4-byte big-endian length prefix + payload; `MaxFrameBytes` = 32 MB (`core/transport/tcp/framing.go`)                                                                   |
| Evolution                     | Protocol version byte (`0x01`) in the session header, reserved for upgrades                                                                                            |
| Single-connection concurrency | `reqID` multiplexing; peers process callbacks while awaiting their own responses                                                                                       |
| Liveness                      | Heartbeat message family (`0x01`/`0x05`)                                                                                                                               |
| Overload protection           | Dual-lane (control/business) bounded dispatch + bounded worker pools + fail-fast on queue saturation                                                                   |
| Graceful removal              | Drain semantics defined at session level                                                                                                                               |
| Security                      | TLS enabled per configuration (`cluster.interconnectTls` analog on each link); plaintext is an acceptable default for same-host/game-LAN deployments                   |
| Reconnect                     | Exponential backoff + automatic re-registration                                                                                                                        |
| Observability                 | OTel trace-context propagation across invoker → server → agent → SDK (platform + Go SDK phase 1; see `internal/telemetry/context.go`, `sdks/go/pkg/croupier/trace.go`) |

## Enforcement

`scripts/check-no-grpc.sh` (CI) guards the boundary, not a blanket ban:

1. `google.golang.org/grpc` imports are allowed only inside the plugin-host
   subtrees (`agents/synthetics/pluginhost/`, `agents/sidecar/funcplugin/`,
   `core/pluginhost/`); game SDKs, other agents, and the server must not
   import it.
2. Top-level `proto/` must not contain `service`/`rpc` definitions — the
   session wire contract stays framing-level; plugin IPC contracts version
   separately inside each plugin host's `protocol/` directory.
3. The main code-generation chain (Makefile, `scripts/gen-proto.sh`,
   `proto/buf.yaml`) must not use gRPC codegen plugins.

A direct `google.golang.org/grpc` dependency in `go.mod` is the expected
consequence of hosting the plugin runtime and is reported fail-open, not a
violation.

## Known boundaries (honest list)

- **Frame-level head-of-line blocking:** all frames on one connection are
  serialized on the wire. A near-32 MB response being written delays small
  frames queued behind it (heartbeat replies included). Dual-lane dispatch
  fixes _dispatch_ priority, not _wire_ priority. Current GM traffic is
  small-payload/low-QPS, so this is not observable in practice; large-frame
  scenarios should tighten the business-side maximum frame size before
  re-considering the transport.
- **Flow control is connection-level:** TCP windows + application-level
  fail-fast. Per-stream flow control only becomes a requirement with
  sustained high-concurrency long-task streams over one link; that shape
  does not exist today.
- **HTTP/2-era conveniences are hand-rolled and therefore narrower** than
  what gRPC gives for free (e.g. deadline propagation across hops is
  explicit metadata, not transport-managed).

## When to re-evaluate

Any single trigger reopens the transport decision (QUIC/HTTP-2 or similar);
otherwise the existing protocol continues to be extended:

1. SDKs connect to Agents across the public internet (loss + high latency
   make QUIC's gains real).
2. Browsers or untrusted environments must connect directly (correct answer
   is the Server HTTP surface, not this link).
3. A single connection carries sustained large-volume streaming (per-stream
   flow control becomes a hard requirement).
4. Adoption of infrastructure that requires HTTP-family protocols (service
   mesh ingress on these links).

Size and dependency complexity remain hard constraints that outrank
ecosystem richness when comparing alternatives.

## References

- Wire contract: [sdk-wire-protocol.md](../sdk-wire-protocol.md)
- Superseded historical note:
  [`docs/archive/legacy-en/architecture/transport-no-grpc.md`](../../archive/legacy-en/architecture/transport-no-grpc.md)
- Transport implementation: `core/transport/tcp/` (shim: `internal/transport/`)
- Session redesign context:
  [agent-server-session-transport-redesign.md](../agent-server-session-transport-redesign.md),
  [sdk-agent-transport-redesign.md](../sdk-agent-transport-redesign.md)
- Boundary guard: `scripts/check-no-grpc.sh`
