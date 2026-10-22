# SDK-Agent transport redesign

## Status

- Status: Accepted Target Design
- Scope: `SDK <-> Agent`
- Out of scope: `Agent <-> Server`

## Design conclusion

This design unifies the future direction of SDK integration. The core conclusions:

1. The default SDK-Agent transport migrates from the SDK-side `legacy transport` to a standalone `tcp` transport, with `tls` enabled as needed.
2. The SDK no longer opens a local listen port and no longer exposes `rpc_addr` / `local_listen` to the Agent.
3. The SDK and the Agent use **a single SDK-initiated long connection**, over which registration, heartbeats, function invocations, job control, and response returns all complete.
4. The `Agent <-> Server` link is not in this redesign's implementation scope, but its design goal is already unified onto the shared session runtime.
5. The SDK-side legacy `NNGServer` / `LocalControl` / "local server callback" models enter deprecated status.

## The shared session runtime and the subprotocol

Two terms need pinning down:

- `shared session runtime`
  - the transport and session foundation shared by the two main links
  - covering `tcp/tls + framing + request/response mux + reconnect + heartbeat + drain`
- `subprotocol`
  - the concrete application-layer subprotocol running on that foundation
  - for `SDK <-> Agent`, that is the `sdk-agent subprotocol`

The key characteristics of the `sdk-agent subprotocol`:

- the first frame must be `ProviderConnectRequest`
- `tls` is not enabled by default
- it targets the provider session, not callback-address registration

## Background

The current multi-language SDKs have these problems at the SDK-Agent connection layer:

- Java / C# / JS / Python are highly sensitive to the legacy transport's binding quality and local runtime dispatch.
- When an SDK is embedded as a dependency of a business program, pulling in a native legacy-transport runtime raises integration complexity, artifact size, and CI flakiness.
- The repo still carries the historical design of "the SDK opens a local listen port and the Agent calls back into the SDK", which contradicts the current goal.
- For SDK users, the most stable integration model is:
  - the SDK connects to the Agent proactively
  - the SDK exposes no listen port
  - the Agent maintains only the session with the SDK and the dispatching

This design does not discuss the Agent-Server interaction protocol; it converges only the SDK-Agent boundary.

## Scope

### In Scope

- The connection model between the SDK and the Agent
- The transport type between the SDK and the Agent
- How the SDK Provider / Invoker carry messages across the SDK-Agent boundary
- Reconnection, heartbeats, backpressure, and overload handling
- A unified recommendation for the SDK configuration surface

### Out Of Scope

- The Agent-Server communication protocol
- Server-side function routing, job scheduling, and the control-plane protocol
- Dashboard / HTTP API / gRPC APIs
- The concrete code implementation details of each language SDK

## Target architecture

### Target topology

```text
+----------------------+             +----------------------+
| Business Process     |             | Agent                |
|  - Game Server       |             |  - Session manager   |
|  - Embedded SDK      |             |  - Dispatch / queue  |
|                      |             |  - Backpressure      |
|  SDK opens one       | TCP (+TLS)  |                      |
|  outbound connection +-----------> |  accepts SDK session |
|  and keeps it alive  |             |                      |
+----------------------+             +----------------------+
```

### Key requirements

- Only the SDK may initiate the connection; the business program must never be asked to open a listen port.
- The Agent must support sending and receiving requests and responses in both directions over the same connection.
- The SDK and the Agent must support automatic reconnection and re-registration after a disconnect.
- The SDK and the Agent must support overload feedback and backpressure control.

## Transport layer design

### Default transport

- Default transport: `tcp`
- Default security mode: `tls.optional`
- `tls` is an optional enhancement, not a default requirement

### TLS policy

The typical SDK-embedded-in-a-business-process scenario is same-host, same-datacenter, same-intranet, so not forcing `tls` by default best serves minimal integration cost.

Recommended rules:

- plaintext `tcp` by default
- enable `tls` across hosts, network segments, zero-trust networks, or when compliance requires
- enable `mTLS` when mutual identity verification is needed
- `tls` is a security configuration of the `tcp` transport, not a separately defined new transport kind

### Non-goals

This design's default path no longer depends on the SDK-side `legacy transport`. If the `legacy transport` is later retained for local high-performance scenarios, it may exist only as a non-default, explicitly opted-in compatibility option — never again as the SDK's default dependency.

### The TCP envelope protocol (Envelope v1)

The standalone TCP transport must implement its own frame boundaries; raw protobuf must not be written straight onto the socket.

The v1 envelope format:

```text
+--------------+------------------+-----------+
| FrameLength  | Croupier Header  | Body      |
| 4 bytes      | 8 bytes          | N bytes   |
+--------------+------------------+-----------+
```

Constraints:

- `FrameLength`: unsigned 32-bit big-endian; the total length of the following `Croupier Header + Body`, excluding its own 4 bytes
- `Croupier Header`: continues to reuse the existing 8-byte business header
- `Body`: protobuf-serialized bytes
- Single-frame length must be bounded by both the Agent's and the SDK's configured maximum frame size

This design is deliberately minimal:

- no extra `Magic`
- no separate transport version field
- framing only frames; it does no protocol detection

### First-message identification

The SDK-Agent `tcp` listener is a dedicated boundary and carries no multi-protocol muxing duty; when `tls` is enabled, its outer layer is only a security wrapper over the `tcp` connection.
v1 therefore identifies the protocol by "the first application-layer message must be the provider connect", not by an extra magic number.

The rules after the connection is established:

1. The Agent first reads the complete first frame according to `FrameLength`
2. Parses the `Croupier Header`
3. Validates that the first message is `ProviderConnectRequest` (recommended `MsgID = 0x050101`)
4. Validates that the header `Version` is supported and that the protobuf body is decodable
5. If any condition fails, the Agent closes the connection immediately and logs `protocol_mismatch`, `unsupported_protocol_version`, or an equivalent reason

Design constraints:

- a first message that is not the registration message → disconnect immediately
- illegal framing, a length overrun, or an undecodable body → disconnect immediately
- never attempt to re-synchronize later frames on the same connection
- never do "maybe it is some other protocol" probing-style compatibility on the same port

### Why v1 does not use a Magic

`Magic` is not worthless, but its benefit is very low in this boundary and it should not be a v1 default rule.

The problems `Magic` usually solves:

- multiplexing several protocols on one listener, needing fast identification
- making frame starts easier to spot by eye in packet captures
- attempting resynchronization after stream corruption

SDK-Agent v1 satisfies none of these premises:

- this is a dedicated SDK-Agent listener, not a multi-protocol muxing port
- the first `ProviderConnectRequest` already completes protocol identification
- `Croupier Header.Version` already carries the wire version evolution semantics
- once framing is corrupted, the simplest and safest policy is to close and reconnect, not to keep guessing at boundaries in a corrupted stream

So v1 adopts:

- minimal length-prefix framing
- first-registration-message protocol identification
- disconnect when unidentified

If "one port multiplexing several protocols" or "a transport preface" ever truly becomes necessary, introduce it explicitly through a new envelope/profile version — do not pre-embed the complexity now.

### The business header stays unchanged

The SDK-Agent transport redesign does not require overturning the existing business header. The header continues to be:

```text
+---------+------------+-----------------+
| Version | MsgID      | RequestID       |
| 1 byte  | 3 bytes    | 4 bytes         |
+---------+------------+-----------------+
```

This maximally reuses the existing protocol constants, the request-response matching rules, and the protobuf message bodies.

## The bidirectional connection model

### General principles

The same SDK-Agent long connection lets both sides initiate requests:

- SDK -> Agent
  - connect and register
  - heartbeat
  - capability reporting
  - invocation requests
  - job stream reads / cancellation requests
- Agent -> SDK
  - function invocation requests
  - job control requests
  - load-shedding / drain / shutdown notices

### Request-response rules

- Requests still use odd `MsgID`s
- Responses still use even `MsgID`s
- Each initiator on the same connection maintains its own incrementing `RequestID` counter
- Responses must echo the original request's `RequestID`
- Both the SDK and the Agent must support concurrent in-flight requests

### Deprecated historical design

The following concepts are deprecated in the SDK-Agent target architecture:

- `local_listen`
- `local_addr`
- `rpc_addr`
- the SDK-side local `NNGServer` / `RequestServer`
- the model of "the Agent accesses the SDK by calling back to a local listen address"

During the compatibility window the fields and API shells may remain, but their semantics must be marked deprecated and must never again ground new implementations.

## Registration and session model

### The new semantics

After the SDK connects to the Agent, it should establish a provider session over that connection — not register a "callback address".

The recommended session flow:

1. The SDK opens the `tcp` connection, attaching `tls` per configuration
2. The SDK sends the session registration request
3. The Agent returns a `session_id`
4. The SDK starts heartbeats
5. The Agent delivers invocation requests to the SDK directly over that connection
6. On disconnect the session invalidates; after reconnecting the SDK re-registers

### MsgID direction

The current `0x05xx LocalControlService` is historical semantics; future implementations should make it "in-connection provider session semantics".

The recommendation keeps the `0x05xx` group but redefines it:

|      MsgID | Suggested name              | Notes                              |
| ---------: | ---------------------------- | ----------------------------------- |
| `0x050101` | `ProviderConnectRequest`    | The SDK establishes a provider session |
| `0x050102` | `ProviderConnectResponse`   | Returns `session_id` / the capability negotiation result |
| `0x050103` | `ProviderHeartbeatRequest`  | SDK heartbeat                        |
| `0x050104` | `ProviderHeartbeatResponse` | The Agent's heartbeat response       |
| `0x050105` | `ProviderDrainRequest`      | The Agent asks the SDK to stop taking new requests |
| `0x050106` | `ProviderDrainResponse`     | The SDK acknowledges the drain state |

Notes:

- This is the target design's recommendation, not a description of the current code state.
- The concrete protobuf message definitions and the final MsgID allocation should be written back into `docs/architecture/sdk-wire-protocol.md` before implementation.

## Agent implementation requirements

### Boundary responsibility

The Agent needs a new, maintained SDK-Agent-dedicated `tcp` listener with `tls` handshake support per configuration — but this applies only to the `SDK <-> Agent` boundary and leaves the existing `Agent <-> Server` link untouched.

### Connections and sessions

The Agent needs at least:

- accepting the SDK-initiated long connection
- establishing an independent provider session per connection
- binding the `session_id` to the underlying connection's lifecycle
- invalidating that `session_id` immediately when the connection drops
- requiring a fresh `ProviderConnectRequest` after the SDK reconnects

### Receiving and sending

The Agent needs at least:

- a read loop driven by `FrameLength`
- a write queue isolated per connection
- in-flight request tracking with timeout reclamation
- bidirectional request/response multiplexing within one connection
- maximum frame size, maximum concurrency, and maximum queue length controls

### First-frame validation

The Agent must treat first-frame validation as the protocol entry:

- the first frame must be `ProviderConnectRequest`
- an unsupported `Version` is rejected
- a mismatched `MsgID` is rejected
- an undecodable protobuf body is rejected
- on failure, close the connection directly — never enter business dispatch

### Runtime control

The Agent needs at least:

- heartbeat timeout detection
- drain / overload feedback
- bounded draining before closing a connection
- invalidation of unfinished requests on an old session
- old/new session isolation after an SDK reconnect

The unified semantics of `drain` here:

- the `Agent` marks the provider session `draining`
- no new `Invoke / Task` is assigned to that session
- in-flight requests may keep completing within the grace period
- the provider keeps heartbeats and necessary control-message handling
- the connection closes or the session invalidates only after the drain completes or the grace expires

### Explicitly out of scope for this implementation

This implementation does not require:

- modifying the `Agent <-> Server` transport protocol
- having the SDK open a local listen port
- extending new semantics for the historical `LocalControl` callback model

## Invocation and job semantics

### What stays unchanged

The `0x03xx InvokerService` business semantics should stay unchanged as far as possible:

- `InvokeRequest` / `InvokeResponse`
- `StartTaskRequest` / `StartTaskResponse`
- `StreamTaskRequest` / `TaskEvent`
- `CancelTaskRequest` / `CancelTaskResponse`

### What changes

The change is not in the message bodies but in the message transport direction:

- Old model: the Agent connects to the SDK's local listen address
- New model: the Agent sends requests to the SDK directly over the existing SDK session connection

So the migration focus belongs in the transport layer and the session management layer — not in reinventing a function invocation model.

### Decoupling the platform protocol from the business payload

Two data layers must be clearly distinguished:

- platform protocol messages
  - e.g. `ProviderConnectRequest`, `InvokeRequest`, `InvokeResponse`, `TaskEvent`
  - this layer keeps protobuf, for cross-language uniformity, version evolution, and fixed-header routing
- the user business payload
  - e.g. `InvokeRequest.payload`, `InvokeResponse.payload`, `TaskEvent.payload`
  - this layer must not force SDK users to define protobuf schemas

Design conclusions:

- protobuf serves only the Croupier platform protocol
- the SDK user's business payload is fixed to UTF-8 JSON
- SDKs must provide default automatic "native objects/structs <-> JSON bytes" encoding/decoding
- integrating the SDK never requires defining a `.proto` first
- any field the Agent must understand, route, govern, or negotiate must never hide inside the JSON payload

### Field boundary determination

Whether a field belongs in the protobuf protocol layer or the JSON payload layer follows these rules:

- The Agent must understand the field to do routing, sessions, authentication, backpressure, retries, idempotency, timeouts, audit, or capability negotiation
  - protobuf
- The field is a cross-function common field, not private to one concrete business function
  - protobuf
- The field matters only to the concrete function's implementation; the Agent only forwards and need not understand it
  - JSON payload

It compresses into one rule:

- any field the Agent needs to understand must never hide inside the JSON payload

### Positive and negative examples

Fields that belong in protobuf / header / metadata:

- `function_id`
- `request_id`
- `idempotency_key`
- `session_id`
- `trace_id`
- `game_id`
- `env`
- `timeoutMs` (a metadata key)
- platform governance fields such as `retry` / `priority`

Fields that stay in the JSON payload:

- `player_id`
- `ban_reason`
- `duration`
- `guild_id`
- `item_count`
- other parameters consumed only by the concrete business handler

### The default payload rules

SDK-Agent v1's default business payload rules:

- `InvokeRequest.payload` carries UTF-8 JSON bytes by default
- `InvokeResponse.payload` carries UTF-8 JSON bytes by default
- `TaskEvent.payload` carries UTF-8 JSON bytes by default
- The Agent forwards and dispatches; it does not parse business field semantics by default
- The SDK encodes language-native objects to JSON and decodes callbacks back into language-native objects

Why designed this way:

- the SDK's main integration scenario is embedding in an existing business process, not first building a full protobuf engineering setup
- JSON is the most stable, zero-extra-dependency common denominator across all target languages
- the vast majority of SDK users care about "pass an object and call", not about generating multi-language proto code first
- the Agent's duty at the SDK-Agent boundary is connections, sessions, backpressure, and dispatching — not binding to the user's business model

### Schema rules

`inputSchema` / `outputSchema` should be defined as an optional enhancement, not a default precondition.

v1 constraints:

- the business payload is fixed to `json`
- when a function provides a schema, it describes the JSON payload with JSON Schema by default
- without a schema, the function still registers and invokes
- schemas mainly serve documentation, validation, UI generation, and debugging hints
- "lacks a schema" must never mean "the SDK is unusable"

### Why JSON is fixed

v1 implements no multi-codec business payload negotiation.

That is:

- the business payload supports JSON only
- no `payload_codec` is defined
- no `default_payload_codec` capability negotiation
- protobuf/custom binary are not v1 business payload options
- if another business payload format is truly needed later, introduce it explicitly through a new protocol version — never as an implicit branch inside v1

## Reconnection policy

### Goals

Reconnection must recover common Agent-restart scenarios fast while avoiding thundering herds and high-frequency idle churn.

### Recommended defaults

- `enabled = true`
- `max_attempts = 0`
  - `0` means retry forever
- `initial_delay_ms = 1000`
- `max_delay_ms = 30000`
- `backoff_multiplier = 2.0`
- `jitter_factor = 0.2`

### Recommended cadence

Suggested cadence:

- an optional fast-retry phase: `0` to `1` attempts at a `200ms` delay
- the regular exponential backoff phase: `1s -> 2s -> 4s -> 8s -> 15s -> 30s`
- after reaching the cap:
  - keep retrying at a fixed `30s`
  - implementations may relax the cap to `60s`
  - **a 10-minute cap is not recommended as the default**

### Why not 10 minutes

In the SDK-Agent scenario the Agent usually sits very close to the business process:

- same host
- same datacenter
- same LAN

A 10-minute cap would stretch recovery badly after an Agent restart; the benefit is far smaller than the cost. What needs controlling is the thundering herd and log noise — not deliberately slowing recovery.

### Backoff reset

Reset the reconnection backoff counter when either condition holds:

- the connection has stayed stably alive for `60s`
- `3` consecutive heartbeats succeeded

### Error classification

#### Retryable errors

- TCP connection drops
- connection timeouts
- TLS handshake timeouts
- the Agent closing the connection
- the Agent returning a retryable overload signal

#### Non-retryable errors

- authentication failure
- incompatible protocol version
- a required capability unsupported
- an explicit configuration error

Non-retryable errors must terminate auto-reconnection immediately and surface a deterministic error reason to the upper layer.

## Backpressure and overload control

### Principles

Backpressure's primary owner is the Agent; complex scheduling policy must not be scattered across every language SDK.

### The Agent's responsibilities

The Agent should maintain at least:

- `max_inflight_requests` per SDK session
- a pending-queue cap per SDK session
- a per-message / per-frame size cap
- the connection-level drain state
- rejection and load-shedding policy under overload

### The SDK's responsibilities

The SDK needs to expose only the minimal necessary configuration:

- a local handler concurrency cap
- a local pending-queue cap
- the queue overflow policy
  - `reject`
  - `drop_oldest`
  - `caller_backoff`

**Implementation status (v0.1.2)**: the six language SDKs' inbound side is unified on the **bounded worker pool** model
(the Go baseline implementation `sdks/go/pkg/croupier/transport/tcp_client.go`):

- the read loop only enqueues, never processes; business work runs at fixed worker concurrency
  (`InboundWorkers`, default `NumCPU`, floor 2)
- the pending queue is bounded (`InboundQLen`, default `workers×4`)
- the overflow policy is **`reject` (fail-fast)**: when the queue is full, an error frame returns immediately —
  `inbound queue full, retry on another instance` — and the Agent-side failover retries on another instance;
  nothing accumulates in memory
- the `drop_oldest` / `caller_backoff` policies are not yet implemented (the current reject semantics already
  close the loop with Agent failover; no demand has driven them yet)

**The control-message dual lanes (landed in Go)**: control messages such as heartbeat requests (`HeartbeatRequest`/
`ProviderHeartbeatRequest`) no longer share a lane with business requests —
the Go SDK has implemented the dedicated `ctrlInbox` lane (`tcp_client.go`, capacity 64 + one worker,
never rejecting), so when a business flood fills the business queue the session stays alive and drain still gets through
(test `dual_lane_test.go`).

**Pending migration**: Python/Java/JS/C++/C# inbound is still a single-queue bounded worker pool
(heartbeats share the queue with business) and needs the dual lanes added per the Go baseline
(`protocol.IsControlRequest()` classification + a dedicated control queue).

### Overload feedback

The Agent must provide explicit feedback instead of leaving the SDK guessing blindly:

- `overloaded`
- `retry_after_ms`
- `draining`
- `too_many_inflight`

On receiving these signals the SDK may:

- pause sending new requests
- defer reconnection
- lower local concurrency
- surface "the Agent is shedding load" to the business layer

## SDK API surface recommendations

### Recommended configuration fields

All SDKs should converge on the following field semantics:

| Field                         | Notes                    |
| ------------------------------ | -------------------------- |
| `transport.kind`               | Fixed preference `tcp`     |
| `transport.address`            | The Agent address          |
| `transport.connectTimeoutMs`   | The connection timeout     |
| `transport.requestTimeoutMs`   | The request timeout        |
| `transport.tls`                | TLS configuration          |
| `reconnect.enabled`            | Auto-reconnect on/off      |
| `reconnect.initialDelayMs`     | The initial backoff        |
| `reconnect.maxDelayMs`         | The maximum backoff        |
| `reconnect.backoffMultiplier`  | The exponential backoff multiplier |
| `reconnect.jitterFactor`       | The jitter factor          |
| `backpressure.maxConcurrency`  | The local concurrency cap  |
| `backpressure.max_queue_size`  | The local queue cap        |

### Deprecated fields

Future implementations should deprecate:

- `local_listen`
- `local_addr`
- `rpc_addr`
- `ipcAddress` (when its only use was the SDK's local listening)

### Lifecycle APIs

The recommended long-term APIs:

- `connect()`
- `close()`
- `registerFunction()`
- `registerFunctions()`

Compatibility-window APIs:

- `serve()` / `Serve()`
  - may remain for now
  - but must no longer mean "start the local listen server"
  - only a compatibility wrapper for "block until the connection lifecycle ends"

## Capability negotiation

When a provider session is established, the SDK should report explicitly:

- `sdk_language`
- `sdk_version`
- `protocol_version`
- `supported_capabilities`
  - `provider_session`
  - `invoke`
  - `start_job`
  - `stream_job`
  - `cancel_job`
- `supported_transport`
  - `tcp`
- `transport_security_mode`
  - `plaintext`
  - `tls`

From these the Agent can:

- reject SDKs that fail the requirements
- disable unsupported invocation paths according to capabilities
- turn misalignment from a "runtime pitfall" into an "explicit failure at connect time"

## Migration order

The recommended order of work:

1. Freeze this design document in the main repo
2. Update `docs/architecture/sdk-wire-protocol.md`
   - make explicit the `0x05xx` migration from `LocalControl` to `ProviderSession`
3. Implement the SDK-Agent-dedicated standalone `tcp` transport in the Agent, with `tls` per configuration
4. Use Go as the reference implementation first, verifying bidirectional send/receive on one connection
5. Migrate Python / JS / C# / Java / C++ gradually
6. Clean up the SDKs' local-listening, `rpc_addr`, and `NNGServer` leftovers
7. Switch the SDKs' default transport to `tcp`

## Impact on existing documents

Once this design lands, the following need synchronized cleanup or rewrites:

- `docs/architecture/sdk-wire-protocol.md`
  - delete or rewrite the `LocalControlService` historical semantics
- `docs/sdks/sdk-parity-matrix.md`
  - stop treating "local server / LocalControl support" as a target capability
- Each language SDK's README / docs
  - remove `local_listen`, local-listen-port, and `gRPC` / `NNGServer` callback-style descriptions

## Final assessment

For the "embed the SDK in another user program" scenario, the SDK side continuing to depend on the legacy transport clearly costs more than it returns.
The future SDK-Agent direction should converge explicitly on:

- **one connection**
- **pure TCP, TLS on demand**
- **the SDK listens on no port**
- **the Agent owns sessions, backpressure, and dispatching**
- **Agent-Server and SDK-Agent share one session runtime mental model**
