---
title: C++ SDK
icon: file-code
order: 1
category:
  - SDK Documentation
tag:
  - C++
  - SDK
---

# Croupier C++ SDK

This document describes the unified design baseline for the C++ SDK in the main repository. It no longer follows the legacy `gRPC`, SDK-local-listener, or default native transport dependency patterns.

## Current Positioning

The C++ SDK has a clear target role:

- An embedded dependency inside business applications
- Actively connects to the Agent's local gateway
- Handles registration, heartbeats, invocations, and job control over a single session
- Uses JSON payloads by default
- Enables TLS on demand

This means:

- The SDK no longer needs to open local ports
- Integrators no longer need to maintain `rpc_addr`
- `gRPC` is no longer the default link
- The legacy transport is no longer a mandatory default dependency

## Current Design Baseline

### Transport

- Default transport: an independent `tcp session`
- Default security policy: plaintext is acceptable on intranets; enable `tls` for cross-host or compliance-sensitive scenarios
- First frame: `ProviderConnectRequest`
- The same connection supports bidirectional request/response multiplexing

### Session

- The SDK establishes a `provider session` after connecting
- The Agent dispatches invocation requests to the SDK over the existing session
- Once disconnected, the old session becomes invalid immediately
- After automatic reconnection, the SDK sends `ProviderConnectRequest` again

### Data

- Platform protocol: protobuf
- Business payload: UTF-8 JSON
- `input_schema` / `output_schema` (C++ member names; corresponding contract keys `inputSchema` / `outputSchema`): JSON Schema, optional

## Core Capabilities the C++ SDK Should Expose

- Connection management
  - `address`
  - `connect timeout`
  - `request timeout`
  - `tls`
- Session management
  - `ProviderConnect`
  - `heartbeat`
  - `drain`
  - Reconnection
- Registration and invocation
  - Registering function descriptors
  - Handling `InvokeRequest` dispatched by the Agent
  - Handling job-style requests
- Business encoding/decoding
  - Native objects <-> JSON

## Legacy Models No Longer Recommended

If any of the following still appear in documentation, examples, or READMEs, they are outdated designs:

- A local `gRPC server` inside the SDK
- A local `NNGServer` inside the SDK
- `LocalControlService`
- `RegisterLocal`
- `rpc_addr`
- `local_listen`
- "Agent calls back into the SDK"

## Conceptual Onboarding Flow

The flow below is conceptual; concrete API names do not have to match it exactly:

```cpp
ClientOptions options;
options.address = "127.0.0.1:19091";
options.tls.enabled = false;
options.reconnect.initialDelayMs = 1000;
options.reconnect.maxDelayMs = 30000;
options.reconnect.steadyStateDelayMs = 30000;

Provider provider(options);
provider.registerFunction(descriptor, handler);
provider.serve();
```

Semantic notes:

- `serve()` means maintaining the session lifecycle and handling downstream requests
- `serve()` no longer means "start a local listener service"

## Recommended Documentation Boundaries

The C++ SDK documentation in the main repository should only cover:

- Protocol baseline
- Transport baseline
- Configuration semantics
- Prohibited items

Concrete API names, build scripts, release artifacts, and example code are owned by the `croupier-sdk-cpp` repository.

## Documentation Entry Points

- [Guide](./guide/)
- [API](./api/)
- [Configuration](./configuration/)
- [Examples](./examples/)

## Related Documents

- [SDK Documentation](../../sdks/)
- [SDK Wire Protocol](../../zh/architecture/sdk-wire-protocol.md)
