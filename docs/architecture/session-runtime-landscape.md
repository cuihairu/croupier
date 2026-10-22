---
title: Session runtime reference implementations
icon: compass
order: 6
category:
  - System architecture
tag:
  - Architecture
  - Reference
---

# Session runtime reference implementations

> **Status**: Reference — background material (an open-source survey), not a normative entry point.

This document answers two questions:

1. Do open-source implementations close to Croupier's current goal exist
2. Why we do not simply adopt one of them as the default infrastructure

Conclusion first:

- Close implementations exist
- None overlaps the current goal exactly
- The most reasonable path is to borrow their layering ideas and build a lightweight `session runtime` ourselves

## What we actually need right now

What Croupier actually needs is a set of:

- single connection
- bidirectional active requests
- multiplexing
- reconnection
- heartbeats
- draining
- backpressure
- easy cross-language adoption by default

It is not:

- a complete P2P network
- a kitchen-sink RPC framework
- a high-performance message-bus product
- infrastructure optimized specifically for shared memory or extreme low latency

So our target is closer to:

- a `session runtime`

than to any single transport library.

## The reference projects at a glance

| Project        | What to borrow                                        | Main reason not to adopt directly                     |
| -------------- | ------------------------------------------------------ | ------------------------------------------------------ |
| `RSocket`      | Bidirectional, multiplexed, clean interaction model     | A heavy protocol stack, beyond current minimal needs    |
| `Aeron`        | Same-host/low-latency/media-driver thinking             | More a high-performance messaging infrastructure; wrong default for a control plane |
| `eCAL`         | Multi-transport abstraction, strong engineering         | Aimed at data-distribution systems; the model does not fully overlap our session |
| `Zenoh`        | Cross-network transport and unified abstraction         | A larger system, beyond the current platform boundary   |
| `libp2p`       | secure channel + stream mux thinking                    | Leans P2P, multi-node networking capabilities           |
| `IceRPC / Ice` | Transport abstraction, mature RPC engineering           | Full RPC framework semantics are too heavy              |
| `yamux`        | Lightweight stream multiplexing                         | Solves only mux, not upper-layer session semantics      |
| `kcp-go`       | A reference for reliable UDP transport                  | One transport choice, not a complete session runtime    |

## RSocket

Official material:

- https://rsocket.io/about/protocol/
- https://rsocket.io/about/motivations/
- https://rsocket.io/about/implementations/

What is worth borrowing:

- It treats a connection explicitly as a long-lived session
- It supports bidirectional interaction models
- The request/stream/channel interaction semantics are clear
- Mature cross-language implementation experience

Why not adopt directly:

- The protocol headers, frame types, and interaction model are heavier than current needs
- The transport adaptation surface is large and tends to raise implementation complexity
- We currently need only minimal request/response + event capability, not the full RSocket model

Borrowing conclusion:

- Learn its session mindset and bidirectional model
- Do not import the whole protocol

## Aeron

Official material:

- https://aeron.io/docs/aeron/overview/
- https://aeron.io/docs/aeron/media-driver/

What is worth borrowing:

- A strong session / channel / media-driver mindset
- A very clear separation of same-host high performance, low latency, and media carrying
- Highly instructive for future `shared-memory` or higher-performance media

Why not adopt directly:

- The current primary need is not extreme throughput or ultra-low latency
- We care more about cross-language adoption cost, controllability, and troubleshooting simplicity
- Too heavy as default control-plane infrastructure

Borrowing conclusion:

- Learn its layering of `media` versus `session`
- Do not make it a current default dependency

## eCAL

Official material:

- https://eclipse-ecal.github.io/ecal/stable/advanced/transport_layers.html
- https://github.com/eclipse-ecal/ecal

What is worth borrowing:

- A fairly clear engineering expression of multiple transport layers
- Helps us think about naming `media kinds` and `transport adapters`
- Valuable reference for "future pluggable transport carriers"

Why not adopt directly:

- It leans toward data distribution and system-integration frameworks
- Current Croupier concerns are session semantics, registration semantics, and platform governance
- Importing it would expand the problem from "a lightweight runtime" into "a full communications-framework selection"

Borrowing conclusion:

- Learn its transport layering
- Do not copy its overall system model

## Zenoh

Official material:

- https://zenoh.io/docs/manual/quic/
- https://zenoh.io/blog/2024-10-21-zenoh-firesong/

What is worth borrowing:

- Rich support for heterogeneous networks and many transport carriers
- Valuable for unified abstraction and cross-environment deployment

Why not adopt directly:

- The system is too large
- We do not need to evolve the platform into a general data grid
- Both the adoption cost and the mental burden are high

Borrowing conclusion:

- A mid/long-term reference for transport extension
- Not the default path for current v1

## libp2p

Official material:

- https://docs.libp2p.io/concepts/secure-comm/overview/
- https://docs.libp2p.io/concepts/multiplex/overview/

What is worth borrowing:

- secure channel
- stream multiplexing
- protocol negotiation

Why not adopt directly:

- We currently have no hole-punching, peer discovery, or mesh-networking needs
- Bringing libp2p in would expand the problem domain far too much

Borrowing conclusion:

- Learn its negotiation and multiplexing layering
- Do not adopt its network system boundary

## Ice / IceRPC

Official material:

- https://docs.zeroc.com/ice/3.8/cpp/transports
- https://zeroc.com/icerpc

What is worth borrowing:

- Mature engineering
- Clear transport abstraction
- Rich cross-language RPC experience

Why not adopt directly:

- We do not want full RPC framework semantics right now
- The current design leans toward a lightweight session foundation, not a full remote-object or RPC framework

Borrowing conclusion:

- Learn its abstraction boundaries
- Do not take in the whole framework

## yamux

Official material:

- https://github.com/hashicorp/yamux

What is worth borrowing:

- A very light within-connection multiplexing approach
- Instructive for "many streams over one connection"

Why not adopt directly:

- It solves only multiplexing
- It does not solve registration, sessions, heartbeats, reconnection, or business message framing

Borrowing conclusion:

- Its mux design is a valid reference
- It cannot be treated as the complete answer

## kcp-go

Official material:

- https://github.com/xtaci/kcp-go

What is worth borrowing:

- If we ever truly go UDP / KCP, such an implementation is a referenceable transport adapter

Why not adopt directly:

- It is just one carrier implementation
- `tcp` is already sufficient for the current default scenario
- It is not yet time to take on extra complexity for hypothetical scenarios

Borrowing conclusion:

- A reference if transport adapters are extended later
- Not needed for current v1

## The most reasonable engineering choice today

Given the current scenario, the most reasonable choice is:

- fix on `tcp` for v1
- one unified `shared session runtime`
- unified framing / header / request-response reuse
- `subprotocol` to distinguish `sdk-agent` from `agent-server`
- `JSON` as the fixed default payload

The advantages of this path:

- Lighter than `RSocket`
- Simpler than `libp2p`
- More general-purpose than `Aeron`
- Closer to the real need than "keep patching around the legacy messaging pattern"

## The mid/long-term borrowing route

If "pluggable transport/media" extension truly happens in the future, the recommended borrowing order:

1. Learn session and interaction models from `RSocket`
2. Learn lightweight mux from `yamux`
3. Learn media / channel layering from `Aeron`
4. Learn transport layering from `eCAL`
5. Evaluate `QUIC`, `KCP`, `shared-memory` only when necessary

That is:

- not picking one project and copying it wholesale
- but borrowing each project's strengths at its respective layer

## Final conclusion

The professional description closest to our goal is not:

- `the legacy messaging pattern`
- an `RPC transport`
- a `message bus`

but:

- a `shared session runtime`

If emphasizing future extensible carrier capability, it can be phrased as:

- a `pluggable transport session runtime`

This is also the wording this documentation set recommends adopting uniformly.
