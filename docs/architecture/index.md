---
title: Architecture overview
---

# Architecture overview

Croupier is a game operations control plane built around three cooperating components:

| Component  | Responsibility                                                                                                    |
| ---------- | ----------------------------------------------------------------------------------------------------------------- |
| **Server** | Owns registries, policy, approvals, audit, routing, and the operator console API.                                 |
| **Agent**  | Maintains the outbound Server session and local provider sessions for game processes and SDKs.                    |
| **SDK**    | Embeds in a game or local service, exposes functions to the Agent, and consumes SDK-facing APIs where applicable. |

The current transport baseline is a unified session model:

- Agent-to-Server uses a TLS-enabled TCP session.
- SDK-to-Agent uses a local TCP provider session.
- Requests, responses, heartbeats, draining, reconnection, and backpressure share the same session runtime.

## System diagram

Solid lines carry business requests and results; dashed lines carry registration and discovery. The mobile layout stacks the same topology vertically.

<picture>
  <source media="(max-width: 768px)" srcset="/architecture/system-en-mobile.svg">
  <img src="/architecture/system-en.svg" alt="Croupier system processes and data flow: console to Server over HTTPS, Server to Agent over outbound TCP sessions, Agent to game processes over local provider sessions" style="max-width: 100%;">
</picture>

## Business scope

The normal business boundary is `gameId + env`. A logical environment such as `dev`, `test`, `staging`, or `prod` does not imply a database, machine, or cluster. Deployment placement is expressed through targets, agents, and nodes.

## Core paths

```text
Function registration -> Function contract -> Resource semantics -> Page proposal -> Published page -> Operator execution
```

1. SDKs register functions and provider/process capabilities with an Agent.
2. The Agent forwards summaries to the Server over its session.
3. The Server normalizes contracts into resource and capability semantics.
4. Page Studio previews, validates, publishes, and rolls back operator pages.
5. The runtime console executes only published page bindings.

## Layered view

- **Protocol layer** uses protobuf envelopes for platform control fields.
- **Payload layer** uses UTF-8 JSON for application data.
- **Contract layer** uses JSON Schema and OpenAPI schemas for inputs and outputs.
- **Presentation layer** is driven by strongly typed page specifications, not ad hoc component trees.

## Reading path
