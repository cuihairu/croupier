# Agent-Server TCP session redesign

## Status

- Status: Accepted Target Design
- Scope: `Agent <-> Server`
- Out of scope: `SDK <-> Agent`

## Design conclusion

This design converges the transport model of the `Agent <-> Server` main link. The core conclusions:

1. The default `Agent <-> Server` transport migrates from the current `legacy REQ/REP` to a standalone `tcp` session, with `tls` enabled as configured.
2. The `Agent` keeps only an outbound long connection to the `Server`; the `Server` is no longer required to dial back to the `rpc_addr` the `Agent` exposes.
3. `Server -> Agent` `Invoke`, `StartTask`, `CancelTask`, and `Ops` requests are all delivered over the existing `Agent-Server` session.
4. The `Agent`'s local listen address serves only `GameServer / SDK / third-party applications` and no longer carries any `Server -> Agent` callback duty.
5. The `legacy messaging pattern` is no longer the abstraction center of the main link; everything converges on a lightweight session protocol that is "single connection, bidirectional, multiplexed, reconnectable, governable".

## The shared session runtime and the subprotocol

Two terms need to be pinned down here as well:

- `shared session runtime`
  - the session transport foundation shared by the two main links
  - covering `tcp/tls + framing + mux + reconnect + heartbeat + drain + backpressure`
- `subprotocol`
  - the application-layer subprotocol running on that foundation
  - for `Agent <-> Server`, that is the `agent-server subprotocol`

The key characteristics of the `agent-server subprotocol`:

- The first frame must be a `RegisterRequest` or its later equivalent connect message
- `tls` is enabled by default
- It targets the agent session, not an `rpc_addr` callback

## Background

In the current repository, `Agent <-> Server` actually mixes two links with different semantics:

- Control plane: the `Agent` actively connects to the `Server` over `legacy REQ/REP` and completes `Register/Heartbeat`
- Invocation plane: the `Server/Dispatcher` then dials back to the `Agent` by `rpc_addr` to execute `Invoke/Task/Ops`

This creates several problems:

- A split transport model: one connection for the control plane, another for the invocation plane
- Confused `rpc_addr` semantics: it looks like "the Agent's registration info" but is essentially "the entry point for the Server to dial back"
- Unclear cluster semantics: the `Agent` is actually connected to a specific `Server` node, yet the invocation plane tries to bypass that session and dial the `Agent` directly
- `legacy REQ/REP` is inherently unsuited to "the Server proactively issuing new requests on an existing connection"

This is not because the `legacy transport` lacks long-connection capability, but because the `REQ/REP` pattern in use does not match the target session model.

## What we actually need

What this scenario actually needs is not "some messaging-middleware pattern" but a lightweight application-layer session:

- One reliable long connection at the bottom
- Identity and capability negotiation first, right after the connection is established
- Both sides able to initiate new requests proactively over the same connection
- Multiple concurrent in-flight requests allowed on the same connection
- On disconnect, the old session invalidates immediately; after reconnection a new session is rebuilt
- Support for heartbeat, backpressure, drain, timeouts, cancellation, and graceful traffic removal

In other words, what we want is not `REQ/REP` but "a single-connection bidirectional session protocol".

## Why TCP session fits this scenario better

### The shape of the business boundary

The current `Agent <-> Server` scenario has a few very clear constraints:

- Deployment is mainly on intranets, within the same datacenter, on the same cluster network
- No need for `libp2p`-style hole punching, peer discovery, or multi-node mesh capability
- No need for a general protocol stack like `RSocket` covering many transports and interaction models
- No need to pull in `Aeron`, which leans toward high-performance messaging infrastructure, by default
- What is needed is a transport layer that is stable, simple, controllable, and evolvable across versions

So the default design should prioritize:

- simplicity
- easy troubleshooting
- easy unification across services
- easy future reuse of the same session mental model with `SDK <-> Agent`

### Why not keep the legacy messaging pattern

The `legacy transport` itself is not wrong; the problem is that it provides a "messaging-pattern abstraction" while we need a "session-protocol abstraction".

The main problems of keeping the `legacy transport` on the main link:

- `REQ/REP` does not suit bidirectional proactive requests
- Even with `PAIR` or other patterns, the session, mux, reconnection, and backpressure semantics would still have to be built by us
- The complexity lands in our own protocol layer anyway — the `legacy transport` does not solve it for us
- Since the protocol layer is ours regardless, staying bound to the `legacy transport` only makes the model more contorted

The conclusion is not "the legacy transport cannot do long connections" but "the legacy messaging pattern is not the right abstraction for our current main link".

### Why not RSocket / libp2p / Aeron / HTTP2

All of these directions are instructive, but none fits as the current default infrastructure:

- `RSocket`
  - The upside: session, bidirectionality, and multiplexing are all complete
  - The problem: the protocol layer is too heavy; headers and interaction models exceed this project's needs
- `libp2p`
  - The upside: protocol negotiation and stream multiplexing are mature
  - The problem: it primarily serves peer-to-peer networking and multi-node overlays, not our current need
- `Aeron`
  - The upside: very strong for same-host and low-latency scenarios; a later reference for local optimization
  - The problem: it is more a high-performance messaging infrastructure, wrong as the default control plane
- `HTTP/2`
  - The upside: the stream/mux mental model is worth referencing
  - The problem: full HTTP semantics and its header system are not what we want

So the right landing point is:

- reference `HTTP/2`'s "multiplexing over one connection" idea
- stay lighter than `RSocket`
- stay more general and easier to adopt than `Aeron`
- implement the minimal session protocol directly on `TCP` ourselves

## Target architecture

### Target topology

```text
+----------------------+             +----------------------+
| Agent                |             | Server Node          |
|  - Session client    |             |  - Session manager   |
|  - Local dispatch    |  TCP(+TLS)  |  - Registry          |
|  - SDK/Game bridge   +-----------> |  - Invoke/Task/Ops   |
|                      |             |                      |
+----------------------+             +----------------------+
```

Key changes:

- The `Agent` connects to the `Server` proactively
- The `Server` owns the `Agent session`
- All `Server -> Agent` requests travel over this existing session
- The `Agent`'s local listener is reserved for local integrators only, never exposed to the `Server`

### Session semantics

After the connection is established, the `Agent` must complete session registration on it:

1. The `Agent` establishes a `tcp` connection, with `tls` attached per configuration
2. The `Agent` sends an `AgentConnectRequest` or equivalent registration message
3. The `Server` returns the `session_id`, the capability negotiation result, and any warnings
4. Both sides start heartbeats
5. The `Server` subsequently delivers `Invoke/Task/Ops` to the `Agent` on the same connection
6. On disconnect, the session invalidates immediately
7. After reconnecting, the `Agent` must re-register and obtain a new `session_id`

## Transport layer design

### Default transport

- Default transport: `tcp`
- Default security mode: `tls.enabled`
- `tls` is a security configuration on top of `tcp`, not a separately defined new transport

### framing

Keep the minimal framing:

```text
+--------------+------------------+-----------+
| FrameLength  | Croupier Header  | Body      |
| 4 bytes      | 8 bytes          | N bytes   |
+--------------+------------------+-----------+
```

Constraints:

- `FrameLength` is the total length of the following `Header + Body`
- `Header` continues to reuse the existing `Version + MsgID + RequestID`
- `Body` is protobuf-encoded
- Single-frame size is bounded by both sides' configured maximum frame length

## The bidirectional multiplexing model

### Basic principles

The same `Agent-Server` connection lets both sides initiate requests proactively:

- `Agent -> Server`
  - Connect/Register
  - Heartbeat
  - Capabilities update
  - Metrics / 状态上报
- `Server -> Agent`
  - Invoke
  - StartTask
  - CancelTask
  - Ops / control
  - Drain / shutdown

### Request-response rules

- Requests use odd `MsgID`s
- Responses use even `MsgID`s
- Each side maintains its own incrementing `RequestID` counter
- Responses must echo the original request's `RequestID`
- Multiple concurrent in-flight requests are allowed on the same connection

This means `Agent <-> Server` is no longer "one-way register/heartbeat plus a separate callback connection" but a genuinely bidirectional, multiplexed session.

## Clustering and routing

This is the most critical difference in the `Agent <-> Server` overhaul.

### The current problem

Today the `Dispatcher` can bypass the `Agent -> Server` control-plane session and dial the `Agent` directly by `rpc_addr`.
Once the session model lands, this practice must be retired.

### The new model

Introduce the notion of a "session's owning node":

- Every `Agent session` belongs to one `Server node`
- The registry must record:
  - `agent_id`
  - `session_id`
  - `server_node_id`
  - `expire_at`
  - capability and provider summaries
- After the `Dispatcher` selects an `Agent`, it no longer dials the `Agent` directly
- Instead it routes the request to the `Server node` holding that `session`
- That node delivers it to the `Agent` over the existing session

### Routing consequences

Once this is done:

- `rpc_addr` is no longer a runtime dependency of `Server -> Agent`
- An `Agent` needs only one outbound connection to some `Server` node
- Other `Server` nodes in the cluster only need to know "which node holds this session"

## The boundary of the Agent's local listener

After the redesign, the `Agent`'s local listen address serves local integrators only:

- `GameServer -> Agent`
- `SDK -> Agent`
- third-party local applications -> `Agent`

Explicitly no longer used for:

- `Server -> Agent` callbacks

The following concepts therefore move to deprecated:

- `rpc_addr` as the `Server -> Agent` invocation entry point
- the `Dispatcher` dialing the `Agent` directly
- any model that relies on the `Agent` exposing a control-plane callback port

## Reconnection, backpressure, and draining

### Reconnection

Recommended defaults:

- `enabled = true`
- `initial_delay_ms = 1000`
- `max_delay_ms = 30000`
- `backoff_multiplier = 2.0`
- `jitter_factor = 0.2`
- after the cap, keep retrying at the cheap fixed cadence

### Backpressure

The `Server` should maintain at least:

- `max_inflight_requests` per agent session
- a pending-queue cap per agent session
- a maximum frame size
- the session's current `draining` state

**Implementation status (v0.1.2)**:

- `transport/tcp/server.go` (the Server's listen side): the read loop invokes the handler **synchronously** —
  a slow handler stalls the read loop, the TCP receive window tightens, and end-to-end backpressure forms naturally.
  Session-level `max_inflight_requests` / queue caps are not yet implemented (unnecessary under the synchronous
  model for now, but an overloaded session cannot be proactively drained — that relies on `drain`).
- `transport/tcp/mux_conn.go` (the bidirectional session for both Agent↔Server and Agent↔Provider):
  **dual-lane bounded dispatch has landed** (`dispatchInbound`, commit `31d7bdc0f`):
  - system/control lane (`0x01xx` heartbeat/registration, `0x05xx` session control including drain,
    `0x060101` Hello): a dedicated queue (`ControlQLen`, default 64) + one worker,
    never rejected — business overload cannot kill the session, and drain signals always get through
  - business lane (`0x03xx`/`0x04xx`/`0x060103` forwarded invokes): `DispatchWorkers`
    (default NumCPU) workers + a bounded `BusinessQLen` (default workers×4) queue;
    when full, a busy error frame is returned, triggering peer failover
  - classification is carried by the explicit MsgID set in `protocol.IsControlRequest()`
    (within the 0x06 family, Hello = control / ForwardInvoke = business; a byte-prefix rule cannot cut it)
  - tests: `mux_conn_dual_lane_test.go` (heartbeats answer promptly under business saturation / the control
    lane is never rejected / concurrent writes are safe)
- The Go SDK has landed the dual lanes in step (`ctrlInbox`); the other five languages are pending migration
  (see the dual-lane section of `sdk-wire-protocol.md`).
- The explicit overload signals (`overloaded` / `retry_after_ms` / `too_many_inflight`)
  have no wire messages defined yet.

### drain

The `Server` should support sending `drain` to a single `Agent session`:

- stop assigning it new requests
- let already-sent requests finish within a timeout window
- after the timeout, close the connection and invalidate the session

The `drain` here is session-level graceful traffic removal, not an immediate disconnect:

- the `Dispatcher` no longer routes new `Invoke / Task / Ops` to that session
- the `Agent` keeps processing in-flight requests
- heartbeats continue
- the connection closes after the drain completes, or is force-closed after the grace period

## Protocol and message evolution recommendations

The legacy-model cleanup of `agent/v1/register.proto` is partially complete:

- `rpc_addr` is reserved (the proto field is deleted), and the registry has executed the DB column drop
  (`internal/platform/registry/agent_session_db.go` DropColumn); only
  `internal/logic/utils/registry_helpers.go` keeps a compatibility alias
- `RegisterRequest`'s semantics have converged from "control-plane registration" to "session establishment +
  registration in one" (see the message families in `sdk-wire-protocol.md`)

Remaining naming evolution:

1. Keep the existing messages for transitional compatibility
2. Introduce more accurate names gradually, e.g.:
   - `AgentConnectRequest`
   - `AgentConnectResponse`
   - `AgentHeartbeatRequest`
   - `AgentDrainRequest`
3. Deprecate `rpc_addr` once the transition completes

## Implementation order

The recommended order of work:

1. Freeze this document
2. Introduce the `tcp session listener` on the `Server` side
3. Introduce the `tcp session client` on the `Agent` side
4. Get `Register/Heartbeat` running on the new session
5. Switch `Invoke/StartTask/CancelTask/Ops` to delivery over the existing session
6. Add `server_node_id + session_id` routing info to the registry
7. Rework the `Dispatcher` so it no longer dials the `Agent` directly
8. Clean up the `rpc_addr` direct-connection dependency
9. Remove the `legacy transport` from the default main link

## Final assessment

For this project, the core need of `Agent <-> Server` is not:

- many transports, kitchen-sink style
- P2P discovery and hole punching
- an extreme same-host low-latency message bus

but:

- one connection
- bidirectional proactive requests
- multiplexing
- simple and controllable
- easy to troubleshoot
- easy to unify with the `SDK <-> Agent` model

So `Agent <-> Server` is best converged into:

- **a lightweight TCP session**
- **TLS on demand**
- **the Server owns the Agent session**
- **Invoke/Task/Ops all reuse the existing session**
- **the Agent's local listener serves local integrators only**

That fits the system's real needs better than continuing to patch the current main link around the `legacy messaging pattern`.
