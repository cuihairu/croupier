# Architecture

The current C++ SDK documentation treats the unified session model as the baseline; the legacy gRPC local-listener model is no longer the default assumption.

## Key Points

- The SDK connects to the Agent proactively
- Registration, heartbeats, invocations, and job control all run over a single connection
- Business payloads use JSON by default
