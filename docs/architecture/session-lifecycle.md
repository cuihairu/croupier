---
title: Session lifecycle
icon: timeline
order: 5
category:
  - System architecture
tag:
  - Architecture
  - session
  - lifecycle
---

# Session lifecycle

This document defines the minimal lifecycle model of the Croupier `shared session runtime`, focusing on unifying:

- session states
- state transition conditions
- the state-machine semantics of `drain`
- the behavioral constraints shared by `SDK <-> Agent` and `Agent <-> Server`

This document describes the runtime model, not the API details of any specific language SDK.

## Goals

This state-machine document solves three problems:

1. What does "the session is established" mean
2. What does "the session is draining" mean
3. When the connection drops, reconnects, or the grace closes, what does each side do

## The minimal state set

v1 recommends unifying on these core states:

- `connecting`
- `handshaking`
- `active`
- `draining`
- `closed`
- `reconnecting`

Notes:

- `connecting`
  - the underlying connection is being established
- `handshaking`
  - the connection is established, but the first-frame handshake and session registration are not complete
- `active`
  - the session can send and receive business requests normally
- `draining`
  - the session no longer accepts new business requests, but in-flight requests may drain
- `closed`
  - the current session has terminated and is no longer usable
- `reconnecting`
  - the old session has terminated; the runtime is waiting for or attempting a new connection

## State machine overview

```text
connecting
    |
    v
handshaking
    |
    v
 active  ---------> draining ---------> closed
    |                    ^                 |
    |                    |                 |
    +--------------------+                 |
             connection lost / fatal       |
                                            v
                                      reconnecting
                                            |
                                            v
                                       connecting
```

Supplementary notes:

- `active -> draining` is the graceful traffic-removal path
- `active -> closed` is the direct-close path
- `draining -> closed` is drain completion or grace expiry
- `closed -> reconnecting` applies only to the side with auto-reconnect enabled

## Allowed behaviors per state

### connecting

Allowed:

- establishing the underlying `tcp` connection
- initializing the read/write loops
- starting the connection timeout

Not allowed:

- sending business requests
- receiving business dispatches

### handshaking

Allowed:

- sending the first-frame handshake message
- negotiating `session_id`, protocol version, and capabilities
- validating the first frame's `MsgID` and protobuf body

Not allowed:

- treating the connection as a usable business session
- assigning `Invoke / Task / Ops`

The preconditions for entering `active` are:

- the handshake succeeded
- a valid `session_id` was obtained
- the local runtime finished initializing its in-flight table
- the provider's `game_id` / `env` match the Agent configuration (validated only when both sides are non-empty; a mismatch does not block the connection but is written to `ProviderConnectResponse.warnings`, easing diagnosis of scope drift when multiple services share an Agent)

### active

Allowed:

- sending request/response in both directions
- multiplexing multiple concurrent in-flight requests
- heartbeats
- backpressure and overload feedback
- receiving the `drain` signal

This is the only state allowed to accept new business request assignments.

### draining

This is the state most prone to implementation ambiguity; it is defined as:

- the session is still alive
- heartbeats continue
- control messages can still be sent and received
- already in-flight requests may continue executing
- no new business requests are assigned to the session

Allowed:

- completing in-flight requests
- returning the last batch of responses / events
- continuing to process session-level control messages
- returning `draining` / `retry_after_ms` when needed

Not allowed:

- accepting new business dispatches
- treating the session again as an ordinarily available node

### closed

Meaning:

- the current session is fully invalidated
- the old `session_id` must not be used again
- unfinished requests on that connection must be judged failed, canceled, or invalidated

### reconnecting

Allowed:

- waiting per the backoff policy
- initiating a new connection
- re-entering `connecting`

Not allowed:

- reusing the old `session_id`
- assuming old in-flight work recovers automatically

## The drain state machine

`drain` is not just a standalone control command; it corresponds to the runtime path `active -> draining -> closed`.

### Triggers

Typical triggers:

- the node is about to be released or exit
- the node is about to switch to a new session
- the node is overloaded and decides not to take new requests
- operations actively removes it from rotation

### What must happen on entering draining

When a session moves from `active` to `draining`, the runtime must at least:

1. mark `draining = true`
2. stop assigning new business requests to the session
3. keep heartbeats
4. keep in-flight tracking
5. start the optional `drain grace timeout`

### Behavior during draining

During `draining`:

- already-sent requests may continue to completion
- arriving business requests should be rejected, rerouted, or explicitly told to retry later
- the session should no longer participate in normal scheduling weights

### Conditions for leaving draining

Only two outcomes may leave `draining`:

1. drain completes and the session enters `closed`
2. the grace expires, the session is force-closed, and it enters `closed`

v1 does not recommend supporting:

- `draining -> active`

The reason is that it introduces more implementation complexity and scheduling ambiguity.
If "cancel the drain" is truly needed in the future, it should be done by rebuilding a new session, not by flipping the old session back to `active`.

## Mapping onto the two links

### SDK <-> Agent

When a provider session enters `draining`:

- the `Agent` no longer assigns new `Invoke / Task` to that provider
- the provider continues completing received requests
- `ProviderDrainResponse` only means "the drain state was accepted"
- once the drain completes, the connection can be closed gracefully

### Agent <-> Server

When an agent session enters `draining`:

- the `Dispatcher` no longer routes new `Invoke / Task / Ops` to that session
- the `Agent` continues completing received work
- the registry / scheduling layer should treat that session as "drainable, but takes no new orders"
- the session is removed only after the drain completes

## Failure and exception paths

Not every close goes through `drain`.

The following usually go straight to `closed`:

- an invalid first frame
- an incompatible protocol version
- authentication failure
- the underlying connection dropping
- a fatal codec error

These paths are not required to enter `draining` first.

## In-flight request handling rules

When a session closes, the policy for old in-flight work must be explicit:

- requests whose responses were already returned complete normally
- unanswered requests must be marked failed or canceled
- old in-flight work must not be implicitly inherited onto a new session

If idempotent recovery is ever needed, it should be implemented through an explicit request-level design — not by treating session reconnection as implicit recovery.

## Observability and metric recommendations

At minimum, expose these runtime metrics:

- current state
- `session_id`
- `draining`
- `inflight_count`
- `queued_request_count`
- `last_heartbeat_at`
- `drain_started_at`
- `drain_deadline_at`
- `reconnect_attempt`

Only then can troubleshooting distinguish:

- it is genuinely dead
- it is draining gracefully
- it is in a backoff-reconnect cycle

## Testing recommendations

The minimal test set includes at least:

1. a successful connect enters `active`
2. an invalid first frame goes straight to `closed`
3. under `active`, multiple request/response exchanges run concurrently
4. after entering `draining`, no new business requests are accepted
5. during `draining`, in-flight requests can still complete
6. after the grace, remaining requests are force-closed
7. a dropped connection enters `reconnecting`
8. after reconnecting, a new `session_id` is obtained

## Final constraints

The whole v1 lifecycle model compresses into one sentence:

- once established, a session enters `active`
- to exit gracefully, it enters `draining`
- once the connection drops or the grace expires, it enters `closed`
- with auto-reconnect enabled, a new session is established from `reconnecting`

This model must apply equally to:

- the `sdk-agent subprotocol`
- the `agent-server subprotocol`

The only differences are the handshake messages and the default TLS policy — never the lifecycle semantics.
