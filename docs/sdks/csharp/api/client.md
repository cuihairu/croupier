# Client API Reference

`CroupierClient` is the core entry point of the C# SDK. It connects to the Agent, registers functions, and maintains the service lifecycle.

## Typical Responsibilities

- Establishing the connection to the Agent
- Registering function descriptors and handlers
- Starting the service loop
- Managing reconnection, heartbeats, and shutdown
