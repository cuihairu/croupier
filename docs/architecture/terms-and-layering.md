---
title: Terms and layering
---

# Terms and layering

These terms distinguish business scope, runtime connectivity, executable capability, and operator presentation.

| Term           | Meaning                                                       | Common misuse                                          |
| -------------- | ------------------------------------------------------------- | ------------------------------------------------------ |
| `gameId`       | Game ownership boundary.                                      | Treating it as a database, host, or release artifact.  |
| `env`          | Logical environment such as `dev` or `prod`.                  | Treating it as infrastructure placement.               |
| Provider       | A game process or SDK-backed service connected to an Agent.   | Treating every local process as an Agent.              |
| Agent          | Control-plane runtime that owns Server and provider sessions. | Treating it as a passive callback server.              |
| Function       | Smallest addressable business operation.                      | Treating it as an operator page or menu item.          |
| Resource       | A grouped business capability such as `player` or `mail`.     | Treating it as a database table automatically.         |
| PageSpec       | Strongly typed operator page DSL.                             | Storing a React component tree or static locale entry. |
| Published page | Immutable runtime page available to the console.              | Mixing drafts into operator navigation.                |

## Session vocabulary

- **Shared session runtime** provides framing, multiplexing, heartbeat, reconnection, draining, and backpressure.
- **Subprotocol** identifies the message contract on a boundary, such as SDK-Agent or Agent-Server.
- **Provider session** is the local connection created when an SDK or local service registers functions.
- **Agent session** is the outbound control-plane connection from Agent to Server.

## Contract vocabulary

- `FunctionContract` describes invocation, schema, risk, and identity.
- `CapabilitySemantics` describes collection, identity, CRUD, action, task, or report behavior.
- `PageProposal` is generated from verified capability semantics.
- `PageSpec` is edited, validated, and published.
- `PublishedPageSpec` freezes execution and presentation facts.

## Placement vocabulary

Use target, agent, and node fields for deployment placement. Keep environment semantics stable when infrastructure changes: one logical environment may use different clusters during migration or high availability operations.

## Related pages

- [Layers](/architecture/layers)
- [Architecture overview](/architecture/)
- [Data flow](/architecture/data-flow)
