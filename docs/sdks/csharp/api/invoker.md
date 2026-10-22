# Invoker API Reference

`CroupierInvoker` is a standalone caller that only talks to the Server HTTP API; it does not reuse the `CroupierClient` Provider TCP session or its configuration. Authentication, `gameId/env` scoping, auditing, and task persistence are all performed by the Server.

## Typical Capabilities

- `InvokeAsync`
- `BatchInvokeAsync`
- `StartTaskAsync`
- `CancelTaskAsync`
- `GetTaskStatusAsync`
