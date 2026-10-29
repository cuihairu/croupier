# SDK Wire Protocol

> **Status**: Current — the authoritative specification of the current wire protocol, followed jointly by the main repo, the Agent, and every SDK.

This document defines the wire protocol conventions on top of Croupier's current `shared session runtime`, to be followed jointly by the main repository, the Agent, and the per-language SDKs.

## Document positioning

This document describes:

- framing
- headers
- request/response multiplexing rules
- `subprotocol` identification
- the boundary between the protobuf protocol layer and the JSON payload layer

This document no longer treats the historical `gRPC` or the `legacy messaging pattern` as the standard baseline.

## shared session runtime

The two internal links share one transport runtime:

- `Agent <-> Server`
- `SDK <-> Agent`

The shared capabilities include at least:

- `tcp`
- optional `tls`
- `4-byte frame length + 8-byte croupier header + protobuf body`
- bidirectional request/response on a single connection
- multiplexing of multiple in-flight requests
- heartbeat
- reconnect
- drain
- backpressure

This shared foundation is the `shared session runtime`.

## subprotocol

`subprotocol` here is not personalization configuration; it means "application-layer subprotocols that run on the same session runtime but differ in handshake and business semantics".

The main repository currently has two core `subprotocol`s:

- `sdk-agent subprotocol`
  - the first frame must be `ProviderConnectRequest`
  - `tls` is not enabled by default; the Agent side can enable local-gateway TLS via the `tls:` section of `agent.yaml` (TLS sits under the framing, the frame format is unchanged, and SDKs need a TLS dial option to connect)
  - targets the provider session
- `agent-server subprotocol`
  - the first frame must be `RegisterRequest`
  - `tls` is enabled by default
  - targets the agent session

## v1 first-frame identification rules

v1 does not introduce a separate `Magic`; the first application-layer message identifies the subprotocol directly.

Why:

- the listen-port boundary is already explicit, so no multi-protocol probing is needed
- `Version + MsgID` is enough to tell whether the first frame is legal
- once framing is corrupted, the safest policy is to disconnect and reconnect rather than keep guessing at boundaries in a broken stream

Rules:

1. read the complete first frame according to `FrameLength`
2. parse the 8-byte header
3. validate `Version`
4. use `MsgID` to check whether the first frame fits the current listen boundary
5. parse the protobuf body
6. if any condition fails, close the connection immediately

## Frame format

```text
+--------------+------------------+-----------+
| FrameLength  | Croupier Header  | Body      |
| 4 bytes      | 8 bytes          | N bytes   |
+--------------+------------------+-----------+
```

Constraints:

- `FrameLength` is an unsigned 32-bit big-endian integer
- `FrameLength` is the total length of the following `Header + Body`
- single-frame length must be bounded by both sides' maximum-frame configuration
- framing only frames; it does no business protocol detection

## Header format

The current unified 8-byte header:

```text
+---------+------------+-----------------+
| Version | MsgID      | RequestID       |
| 1 byte  | 3 bytes    | 4 bytes         |
+---------+------------+-----------------+
```

Constraints:

- `Version`: currently fixed at `0x01`
- `MsgID`: 24-bit unsigned, big-endian
- `RequestID`: 32-bit unsigned, big-endian
- `Body`: protobuf-encoded

## Request-response rules

Unified rules:

- odd `MsgID`s mean request
- even `MsgID`s mean response
- responses must echo the original request's `RequestID`
- each initiator maintains its own incrementing `RequestID` counter
- multiple concurrent in-flight requests are allowed on one connection

Notes:

- `responseMsgID = requestMsgID + 1` remains the default convention
- one-way event messages like `TaskEvent` are not part of the standard request/response pairing

### Synchronous call timeout: the metadata `timeoutMs` convention

The caller's declared budget for one synchronous call (milliseconds, decimal string), placed in the invoke request's
metadata map and propagated end to end; each hop applies **min(its own configured cap, the declared value)** (the
natural min semantics of a Go context deadline):

| Layer           | Behavior                                                                                                                          |
| --------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| HTTP API        | request-body `timeoutMs` (optional) → injected into `metadata["timeoutMs"]`                                                       |
| Server dispatch | `requestTimeoutBudget`: clamp [1000, 60000], tightens the ctx; the global default 15s serves only as a ceiling                    |
| Agent           | `providerCallDeadline`: clamp [1000, the Agent's configured cap], min'd with the default (`agent.invokeTimeoutMs`, default 15000) |

- Garbage values / missing → each hop falls back to its own default, no error (zero intrusion)
- A hard ceiling of 60s on the synchronous channel: longer operations belong in async tasks (the event-stream semantics)
- Semantic correction: the Agent's old hardcoded 10s sat inverted against the Server's 15s default; now aligned and configurable

## Overload feedback and backpressure status

Layered by connection role, the current implementation status (the dual lanes have landed; see next section):

| Path                                                                    | Mechanism                                                                                                                                                              | Backpressure semantics                                                                                                                                                                                          |
| ----------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Server → Agent/Provider (`transport/tcp/server.go`)                     | The read loop invokes the handler synchronously                                                                                                                        | Natural backpressure: a slow handler stalls reads, the TCP window tightens, pressure backs up end to end                                                                                                        |
| SDK → Agent (per-language SDK inbound; the Go baseline `tcp_client.go`) | A bounded worker pool (default `NumCPU`, queue `workers×4`) + a Go control lane                                                                                        | Fail-fast: when the business queue is full, an `inbound queue full, retry on another instance` error frame returns immediately and the Agent side fails over to another instance; nothing accumulates in memory |
| Agent↔Server, Agent↔Provider (`transport/tcp/mux_conn.go`)              | **Dual-lane bounded dispatch** (`dispatchInbound`): a dedicated system-lane queue (64) + one worker; the business lane gets NumCPU workers + a bounded workers×4 queue | When the business lane fills, a busy error frame returns inline (peer failover); the control lane never rejects (when full it blocks the read loop = natural control-plane backpressure)                        |

Known boundaries:

- Explicit overload signal fields like `overloaded` / `retry_after_ms` / `too_many_inflight` have no wire messages yet; the only overload signal today is the error payload text when the business queue is full
- The dual lanes have landed: Go (`MuxConn` + the Go SDK), Python (`transport/tcp.py`, a control-lane single worker that never rejects), JS (`tcp_transport.ts`, a control queue + a single-concurrency loop). Java/C++/C# inbound is still a single-queue bounded worker pool (heartbeats share the queue with business) pending migration to the Go baseline

### The dual lanes: control-message priority

One structural premise of overload governance: **control messages must not share a lane with business messages**.
When a business flood fills the shared queue, heartbeats get fail-fast rejected → the peer judges the session dead →
overload escalates into a connection avalanche; a drain signal gets buried → graceful shutdown becomes impossible under overload.

Inbound dispatch is therefore classified into two lanes by explicit MsgID (`protocol.IsControlRequest()`,
an explicit set rather than a raw byte prefix — within the 0x06 cluster family, Hello is control and ForwardInvoke is business):

| Lane             | Messages                                                                                                 | Queue semantics                                                              |
| ---------------- | -------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------- |
| System (control) | `0x01xx` Register/Heartbeat, `0x05xx` Provider session control (including Drain), `0x060101` ServerHello | A small dedicated queue, **never rejected**                                  |
| Business         | `0x03xx` Invoke/Task, `0x04xx` Ops/Metrics, `0x060103` ForwardInvoke                                     | A bounded queue; when full, fail-fast returns an error frame (peer failover) |
| Events           | One-way frames like `TaskEvent`/`MetricEvent`                                                            | Inline in the read loop, not queued (status quo kept)                        |

Behavioral guarantees:

- When the business lane is saturated, heartbeats/drain/register still get through — **session liveness and draining are unaffected by business overload**
- The system lane is itself bounded (so the control plane cannot be flooded), but its capacity is independent and it never returns reject to the peer
- The fail-fast reject semantics apply only to business requests; the `retry on another instance` wording is unchanged

Implementation locations: `MuxConn.dispatchInbound` (`internal/transport/tcp/mux_conn.go`,
Config `DispatchWorkers`/`BusinessQLen`/`ControlQLen`, defaults NumCPU / workers×4 / 64)
and the Go SDK's `handleInboundRequest` (`sdks/go/pkg/croupier/transport/tcp_client.go`,
`ctrlInbox` capacity 64 + one worker). Tests: `mux_conn_dual_lane_test.go`, `dual_lane_test.go`.

## drain semantics

`drain` is session-level graceful traffic removal, not an immediate-disconnect semantic.

Unified definition:

- no new business requests are assigned to the session
- in-flight requests may keep completing within the grace period
- the connection closes or the session invalidates only after the drain completes or the grace expires

Minimal behavioral constraints:

1. the session enters the `draining` state
2. arriving requests are rejected, rerouted, or answered with an explicit `draining` / `retry_after_ms`
3. in-flight requests may continue executing
4. heartbeats continue
5. once the grace expires, remaining requests may be force-closed

Boundary notes:

- `drain` is not ordinary backpressure
- `drain` does not mean canceling every in-flight request
- during `drain`, session-governance control messages can still be exchanged

Implementation status (ARCH-016, 2026-10-30):

- The Agent implements the `draining` state on `ProviderSession` (`agents/sidecar/provider_session.go`): `Draining`/`SetDraining`/`Inflight`/`AddInflight`. `callProvider` (`internal/agent/local_handler.go`) skips draining sessions for new business requests and rejects with an explicit `provider <id> is draining (retry after <N>ms)` when the targeted session or all function-matching sessions drain. Task continuations (a `StartTask` accepted before drain) are still routed to their draining provider — drain must not cut off work already assigned.
- The shared `LocalHandler.DrainProviderSession` marks targets draining, removes their agentlocal instances (legacy address-dial no longer routes), and waits for in-flight to reach zero or the grace to expire; the `ProviderDrainResponse` ACK is returned only after that point. `grace` defaults to `defaultDrainGrace` (30s) and is overridden by `retry_after_ms`.
- The Server routing gate: `reg.AgentSession.Draining` is set/cleared by `opsNodeDrain`/`opsNodeUndrain` via `SetAgentDraining`; `Dispatcher.agentCanInvoke` returns false for a draining agent, so new requests are no longer assigned to the node. `UpsertAgent` carries `Draining` over across session refresh (heartbeat/reconnect does not silently un-drain).
- Undrain is expressed with the `ProviderDrainRequest.reason` sentinel `__undrain__` (`protocol.ProviderDrainReasonUndrain`), sent by `opsNodeUndrain` and honored by both drain paths.

Known boundaries (honest):

- Force-close after the grace expires (constraint 5) is **not** implemented: remaining requests are not cancelled; a still-running task converges via the ARCH-015 D3 `unknown`/terminal-event path, and the drain ACK is returned at grace expiry.
- Undrain clears the `draining` flag but does not repopulate the agentlocal instance table; a drained provider's legacy address-dial routing resumes only after the provider re-registers or heartbeats its functions.
- The `reason == "__undrain__"` sentinel is used because the system protoc (3.21.12) produces bad gencode and no 34.x protoc is available; a dedicated `undrain` proto field is a follow-up when protoc 34.x is available.

## Message families

### `0x01xx` Agent Session Control

The session control messages of the `agent-server subprotocol`; the current proto names are still:

- `RegisterRequest`
- `RegisterResponse`
- `HeartbeatRequest`
- `HeartbeatResponse`
- `RegisterCapabilitiesRequest`
- `RegisterCapabilitiesResponse`

Semantically this group should already be read as "agent session connect/register", not the historical callback model.

The `agent-server subprotocol` should also carry session-level `drain` capability:

- the `Server` can put a specific `Agent session` into `draining`
- the `Dispatcher` no longer assigns new requests to that session
- in-flight requests keep completing within the grace period
- the connection closes or the session is removed only after the drain completes

### `0x02xx` Client / Result query

Reserved for SDK-visible control messages such as client registration and job-result queries.

### `0x03xx` Invocation / Task

For synchronous calls, async tasks, and cancellation:

- `InvokeRequest`
- `InvokeResponse`
- `StartTaskRequest`
- `StartTaskResponse`
- `StreamTaskRequest`
- `TaskEvent`
- `CancelTaskRequest`
- `CancelTaskResponse`

### `0x04xx` Ops / Telemetry

The message family for system info, metrics, and operational control.

### `0x05xx` Provider Session

The provider session control messages of the `sdk-agent subprotocol`:

|      MsgID | Name                        | Notes                                                    |
| ---------: | --------------------------- | -------------------------------------------------------- |
| `0x050101` | `ProviderConnectRequest`    | Establish a provider session                             |
| `0x050102` | `ProviderConnectResponse`   | Return `session_id`, negotiation results, and `warnings` |
| `0x050103` | `ProviderHeartbeatRequest`  | Provider heartbeat                                       |
| `0x050104` | `ProviderHeartbeatResponse` | Heartbeat response                                       |
| `0x050105` | `ProviderDrainRequest`      | The Agent puts the provider session into `draining`      |
| `0x050106` | `ProviderDrainResponse`     | The provider acknowledges entering drain                 |

`ProviderConnectResponse` field semantics:

- `session_id`: the provider session identifier, reused by later heartbeats and invokes
- `accepted_capabilities`: the capability list the Agent accepted
- `warnings`: a non-blocking list of warning strings. Two current sources — scope-drift detection (when the provider-reported `game_id` / `env` disagrees with the Agent configuration, `game_id mismatch` / `env mismatch` is written, easing diagnosis of scope misconfiguration when multiple services share an Agent), and instance-metadata reserved-key conflicts (next section; writes `metadata key %q is reserved and dropped`)

### Instance metadata: `ProviderConnectRequest.metadata`

At registration a provider may attach **user-defined multi-KV instance metadata** to the first frame (e.g. `serverId`, `pod`), used by the SDK version distribution page and metadata-based instance search (the REST endpoint `GET /api/v1/providers/sdk-stats?metaKey=&metaValue=`).

The data path:

```
SDK 配置（Go InstanceMetadata / JS providerMetadata）
  → ProviderConnectRequest.metadata（map<string,string>, field 13）
  → Agent 合并：保留键冲突丢弃 + warnings 告警；空键跳过
  → agentlocal.Instance.Metadata
  → AgentProcess.metadata（field 11）随 RegisterRequest 上报 server
  → registry ProviderSession / ProviderSessionSnapshot
  → sdk-stats REST API（服务端子串过滤）→ web SDK 版本分布页
```

The sdk-stats instance detail (`SdkInstanceItem`) exposes both `firstSeenUnix` (registration time, #44; server-normalized: falls back to `lastSeenUnix` when no observation exists) and `lastSeenUnix`. `firstSeenUnix` has process-window semantics — registry sessions are in-memory, so they re-accumulate from zero after session expiry or a server restart (consistent with the runtime-sources boundary below).

Rules:

- **Reserved keys**: `sdkLanguage` / `sdkVersion` / `sdkName` / `protocol_version` / `gameId` / `env` are platform-fixed fields (the Agent derives them from the request's fixed fields). When user metadata collides, the Agent **drops the key and writes a warning** — it must not overwrite platform semantics; empty keys are skipped
- **Observation only, never routing**: metadata is used only for display/search/diagnostics; load balancing and function routing must not read it (matching the proto comment "observability only, never routing")
- Older SDKs that never send the field are fully compatible; when unconfigured, an empty map is not sent on the wire (nil/omitted on both Go/JS)

Storage and query design conclusions (persisted; landed in #11):

- Online provider sessions are **ephemeral in-memory data** (kept alive by heartbeat TTLs); instance metadata is additionally **persisted in an EAV table** (migration 0033, `provider_metadata`): `(game_id, env, service_id, meta_key, meta_value)` + `UNIQUE(game_id, env, service_id, meta_key)` + `INDEX(meta_key, meta_value)` — portable across the sqlite/mysql/postgres dialects, dependent on no JSON query extensions. It refreshes wholesale after every successful agent registration (upsert current values + delete rows for services that vanished this round; an unchanged duplicate registration skips the write amplification), and session-expiry cleanup deletes the corresponding rows; the table's row set follows the registered sessions, so dropdown options and aggregate queries still have a source after a restart
- The aggregate query = `GET /api/v1/providers/meta-options` (scoped group): the server returns distinct keys → value sets (including per-value instance counts, values sorted by count descending) for the current game scope, with a 30s in-process cache invalidated on registration — the filter dropdown's options are server-provided (#2); the frontend must not derive them from the (already-filtered) list data
- Search = the server applies an in-memory linear filter over the snapshot (`metaKey`/`metaValue` substrings, case-insensitive; a value condition matches both the key name and the whole `k=v` pair) + frontend instance-keyword matching; the scale (instances per agent session) is far below anything that needs an index
- DB-less (in-memory registry) degradation: with no database, aggregation computes on the fly from the online session snapshot and is lost on restart — see the deployment-shape differences

Historical aliases such as `RegisterLocalRequest`, `RegisterLocalResponse`, and `HeartbeatLocalRequest` belong to compatibility semantics only and should not appear in new design documents.

The semantics of `ProviderDrainRequest / Response` should unify as:

- the `Agent` no longer assigns new requests to that provider session
- the provider keeps completing the in-flight requests it has received
- the provider's `ProviderDrainResponse` means only "the drain state was accepted"
- the actual connection close happens after the drain completes or the grace expires

## Business payload rules

Two layers must be clearly distinguished:

- the protocol message layer
  - protobuf
  - for session, routing, capability negotiation, and job control
- the business payload layer
  - UTF-8 JSON by default
  - generally carried in `bytes payload`

The v1 default rules:

- `InvokeRequest.payload` is JSON bytes
- `InvokeResponse.payload` is JSON bytes
- `TaskEvent.payload` is JSON bytes
- SDK users do not need to define a `.proto` first to integrate
- SDKs should provide automatic native-object ↔ JSON bytes encoding/decoding by default

## Field layering rules

Which layer a field belongs to is not judged by "does it look like a business field" but by "who needs to understand it".

Rules:

- Fields the Agent must understand, route, govern, or negotiate go in the protobuf protocol layer
- Fields consumed only by the concrete business function go in the JSON payload layer
- Hiding platform control fields inside the JSON payload and asking the Agent to guess is forbidden

Typical protobuf fields:

- `function_id`
- `session_id`
- `idempotency_key`
- `traceId`
  - W3C trace propagation is detailed in `docs/architecture/sdk-otel-propagation.md` (SDKs propagate only, never export)
- `gameId`
- `env`
- `timeoutMs`
- `priority`
- capability-negotiation, rate-limiting, retry, and audit-related fields

Typical JSON payload fields:

- `player_id`
- `ban_reason`
- `duration`
- `guild_id`
- `item_count`

## Schema rules

`inputSchema` / `outputSchema` (proto field names `input_schema` / `output_schema`) describe, on the default path, the JSON Schema of the JSON payload.

Rules:

- schemas are an optional enhancement
- functions still register and invoke without schemas
- schemas mainly serve validation, documentation, UI generation, and debugging hints
- v1 does not negotiate multiple payload codecs

## Compatibility rules

- An unsupported `Version` disconnects immediately
- A first frame that does not fit the `subprotocol` disconnects immediately
- An undecodable protobuf body disconnects immediately
- Once disconnected, the old `session_id` must invalidate immediately
- After reconnecting, a new session must be established; never assume the old session recovered automatically

## Explicitly deprecated legacy concepts

The following must no longer serve as the basis of new protocol implementations:

- `LocalControlService` as the main semantic entry point
- `RegisterLocalRequest` as the main semantic entry point
- `rpc_addr`
- an SDK-side local listen server
- the `Server -> Agent` callback model
- the claim that "the legacy message envelope over TCP equals a separate TCP transport"

## Related documents

- [Architecture overview](./)
- [SDK-Agent transport redesign](./sdk-agent-transport-redesign.md)
- [Agent-Server TCP session redesign](./agent-server-session-transport-redesign.md)
- [SDK documentation](../sdks/)
