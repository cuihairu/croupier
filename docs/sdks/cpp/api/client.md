# Client API Reference

`CroupierClient` is the core entry point of the C++ SDK.

## Typical Responsibilities

- Establishing a session with the Agent
- Registering functions, resource capabilities, and components
- Maintaining heartbeats, reconnection, and the request-handling loop
- Managing shutdown and resource release

## Integration Advice

- Keep connection configuration separate from business handlers
- Complete registration during service startup
- Explicitly set risk levels and schemas for high-risk functions
