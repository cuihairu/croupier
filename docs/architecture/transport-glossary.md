---
title: Transport glossary
---

# Transport glossary

> **Status**: Current — these terms describe the shipped session transport.
> They cover the wire/session layer only; page-model terms live in the
> [Dashboard glossary](./dashboard-glossary).

| Term                       | Definition                                                                                                                                             | What it is not                                                                        |
| -------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------- |
| Session runtime            | The shared transport foundation all sessions run on: `tcp/tls + framing + mux + reconnect + heartbeat + drain`                                         | A message history store, a queue, or a per-call connection                            |
| Subprotocol                | An application-layer protocol variant running on top of the session runtime; differs in handshake message, registration content, and routing semantics | "Custom configuration", a plugin system, or a separate transport                      |
| `sdk-agent` subprotocol    | The SDK → sidecar session (local TCP listener on port 19091) used for function registration and invocation                                             | A direct SDK → Server connection; the SDK never listens on a local port               |
| `agent-server` subprotocol | The agent → server control session (outbound dial to port 19090) carrying registration, health, tasks, and forwarded requests                          | An inbound server→agent connection; agents always dial out                            |
| Frame                      | The wire unit on a session: 4-byte frame length prefix, 8-byte header, protobuf body; maximum 32 MiB per frame                                         | A whole request (a request may span multiple frames, e.g. streams) or an HTTP payload |
| Handshake                  | The first frame of a session; negotiates identity and capabilities (for providers it carries `ProviderConnectRequest`)                                 | An HTTP upgrade, a TLS-only step, or optional — every session starts with one         |
| reqID multiplexing         | Correlating concurrent in-flight requests on one connection by request id, in both directions                                                          | One-connection-per-request, head-of-line-free ordering, or HTTP/2 streams             |
| Drain                      | Graceful session shutdown: stop accepting new work, let in-flight work finish, then close                                                              | An abort, a hard cancel, or a config reload                                           |
| L1 Core Provider           | The minimal provider surface every SDK must implement: construct / register / session / service loop / stop                                            | An optional add-on or a server-side feature                                           |
| L2 Provider extensions     | Optional provider capabilities: JSON Schema, drain, control-plane manifest, TLS, file transfer                                                         | Required for L1 compliance; caller-side features                                      |
| L3 Invoker                 | Caller-side capability (invoking registered functions), shipped as an independent module                                                               | A provider-side requirement; not needed to be invoked                                 |
| L4 language extensions     | Language/engine-specific add-ons (DI, Unity, Lua, Spring Boot starter, Skynet)                                                                         | Part of the wire protocol; guaranteed on every SDK                                    |

## Where to read more

- Wire protocol conventions: [SDK wire protocol](./sdk-wire-protocol)
- Transport decision history: [ADR 0001 — transport](./adr/0001-transport)
- SDK layer details: [SDK documentation](../sdks/)
