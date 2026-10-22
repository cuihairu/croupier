---
title: Layers
---

# Layers

Croupier separates control-plane concerns from game-process integration. Each layer has an explicit owner and contract.

## Component layers

| Layer               | Owner        | Responsibility                                                                                   |
| ------------------- | ------------ | ------------------------------------------------------------------------------------------------ |
| Console and API     | Server       | Authentication, RBAC, audit, proposals, publishing, monitoring, and operator-facing execution.   |
| Registry and policy | Server       | Function contracts, resource semantics, snapshots, approvals, and execution policy.              |
| Agent session       | Agent        | One outbound Server session with heartbeats, reconnection, draining, and request multiplexing.   |
| Provider gateway    | Agent        | Local sessions for embedded SDKs and game processes, plus routing to healthy provider instances. |
| Embedded SDK        | Game process | Registers functions, invokes local handlers, and reports process/provider metadata.              |

## Technical boundaries

```text
Operator UI
  -> Server HTTP/API
    -> Server registry and policy
      -> Agent/Server session
        -> Agent local gateway
          -> SDK provider session
            -> game handler
```

- The Server never dials back into an Agent. Agents own the outbound session.
- The Agent never dials back into an SDK provider. Providers own the local session.
- Requests reuse the established session in both directions.
- protobuf carries platform envelopes; JSON carries business payloads.

## Data and page layers

| Boundary            | Model                                          |
| ------------------- | ---------------------------------------------- |
| Executable unit     | `FunctionContract`                             |
| Business capability | `ResourceCapability` and `CapabilitySemantics` |
| Editable page       | `PageProposal` and `PageSpec`                  |
| Runtime page        | `PublishedPageSpec`                            |
| Navigation          | `ConsoleMenuSpec`                              |

This prevents a single function descriptor from becoming an implicit page, menu, permission, or deployment boundary.

## Placement boundaries

`gameId + env` identifies business scope. It does not identify a database, host, or cluster. Physical placement is represented by targets, agents, and nodes so that logical environments can span or map to infrastructure independently.

## Related pages

- [Architecture overview](/architecture/)
